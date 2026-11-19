package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
	"unicode/utf8"
)

// ErrUnavailable marks a run that failed in transport, so retrying may succeed.
var ErrUnavailable = errors.New("go-judge unavailable")

// Client runs commands through go-judge's HTTP API.
type Client struct{ endpoint }

// The HTTP client should have no timeout: runs are bounded by their limits and ctx.
func NewClient(base string, client *http.Client) (*Client, error) {
	e, err := newEndpoint(base, client)
	if err != nil {
		return nil, err
	}
	return &Client{e}, nil
}

// cmdFile mirrors go-judge's JSON CmdFile; exactly one source is set.
type cmdFile struct {
	Src     *string `json:"src,omitempty"`
	Content *string `json:"content,omitempty"`
	FileID  *string `json:"fileId,omitempty"`
	Name    *string `json:"name,omitempty"`
	Max     *int64  `json:"max,omitempty"`
	Pipe    bool    `json:"pipe,omitempty"`
}

type cmd struct {
	Args          []string           `json:"args"`
	Env           []string           `json:"env,omitempty"`
	Files         []*cmdFile         `json:"files,omitempty"`
	CPULimit      uint64             `json:"cpuLimit"`
	ClockLimit    uint64             `json:"clockLimit"`
	MemoryLimit   uint64             `json:"memoryLimit"`
	ProcLimit     uint64             `json:"procLimit"`
	CopyIn        map[string]cmdFile `json:"copyIn,omitempty"`
	CopyOut       []string           `json:"copyOut,omitempty"`
	CopyOutCached []string           `json:"copyOutCached,omitempty"`
	CopyOutMax    uint64             `json:"copyOutMax,omitempty"`
}

type pipeIndex struct {
	Index int `json:"index"`
	Fd    int `json:"fd"`
}

type pipeMap struct {
	In    pipeIndex `json:"in"`
	Out   pipeIndex `json:"out"`
	Max   int64     `json:"max"`
	Proxy bool      `json:"proxy"`
}

type request struct {
	Cmd         []cmd     `json:"cmd"`
	PipeMapping []pipeMap `json:"pipeMapping,omitempty"`
}

type result struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error"`
	Time       uint64            `json:"time"`
	Memory     uint64            `json:"memory"`
	Files      map[string]string `json:"files"`
	FileIDs    map[string]string `json:"fileIds"`
}

// go-judge status strings (envexec.Status).
const (
	resultAccepted       = "Accepted"
	resultMemoryLimit    = "Memory Limit Exceeded"
	resultTimeLimit      = "Time Limit Exceeded"
	resultOutputLimit    = "Output Limit Exceeded"
	resultNonzeroExit    = "Nonzero Exit Status"
	resultSignalled      = "Signalled"
)

func memoryFile(content string) *cmdFile { return &cmdFile{Content: &content} }

func cachedFile(id string) *cmdFile { return &cmdFile{FileID: &id} }

func localFile(src string) *cmdFile { return &cmdFile{Src: &src} }

func collector(name string, max int64, pipe bool) *cmdFile {
	return &cmdFile{Name: &name, Max: &max, Pipe: pipe}
}

func (c *Client) run(ctx context.Context, req request) ([]result, error) {
	for _, cmd := range req.Cmd {
		for _, f := range cmd.Files {
			if f != nil && f.Content != nil && !utf8.ValidString(*f.Content) {
				return nil, errors.New("inline sandbox content must be UTF-8")
			}
		}
		for _, f := range cmd.CopyIn {
			if f.Content != nil && !utf8.ValidString(*f.Content) {
				return nil, errors.New("inline sandbox content must be UTF-8")
			}
		}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode sandbox request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sandbox request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sandbox run: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("sandbox run: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}
	var results []result
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		// go-judge sends the status before the body, so a crash mid-run truncates a 200 response.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: truncated response: %w", ErrUnavailable, err)
		}
		return nil, fmt.Errorf("decode sandbox response: %w", err)
	}
	if len(results) != len(req.Cmd) {
		return nil, fmt.Errorf("sandbox returned %d results for %d commands", len(results), len(req.Cmd))
	}
	return results, nil
}

// upload stores trusted bytes that cannot travel as JSON text.
func (c *Client) upload(ctx context.Context, data []byte) (string, error) {
	return c.post(ctx, bytes.NewReader(data), int64(len(data)))
}

func (c *Client) release(ids ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		if id == "" {
			continue
		}
		if err := c.delete(ctx, id); err != nil {
			slog.WarnContext(ctx, "release sandbox file", "error", err)
		}
	}
}

func unavailable(err error) bool {
	var n net.Error
	return errors.Is(err, ErrUnavailable) || errors.As(err, &n)
}
