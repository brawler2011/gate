package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// FileTransport uses go-judge's disk file store. FileAdd/FileGet in its gRPC
// API are unary messages and must not be used for test bodies or executables.
type FileTransport struct {
	base string
	http *http.Client
}

func NewFileTransport(base string, client *http.Client) (*FileTransport, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("invalid sandbox file endpoint")
	}
	if client == nil {
		return nil, fmt.Errorf("sandbox file HTTP client is required")
	}
	return &FileTransport{base: strings.TrimRight(base, "/"), http: client}, nil
}

// Upload sends exactly the regular file's current size with bounded buffers.
// The caller owns the file; its offset is not changed.
func (t *FileTransport) Upload(ctx context.Context, file *os.File, limit int64) (string, error) {
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat sandbox upload: %w", err)
	}
	if !stat.Mode().IsRegular() || limit <= 0 || stat.Size() > limit {
		return "", fmt.Errorf("sandbox upload must be a regular file within quota")
	}
	var header bytes.Buffer
	m := multipart.NewWriter(&header)
	if _, err := m.CreateFormFile("file", "blob"); err != nil {
		return "", fmt.Errorf("create sandbox multipart: %w", err)
	}
	prefix := bytes.Clone(header.Bytes())
	header.Reset()
	if err := m.Close(); err != nil {
		return "", fmt.Errorf("close sandbox multipart: %w", err)
	}
	suffix := bytes.Clone(header.Bytes())
	body := io.MultiReader(bytes.NewReader(prefix), io.NewSectionReader(file, 0, stat.Size()), bytes.NewReader(suffix))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/file", body)
	if err != nil {
		return "", fmt.Errorf("create sandbox upload: %w", err)
	}
	req.Header.Set("Content-Type", m.FormDataContentType())
	req.ContentLength = int64(len(prefix)) + stat.Size() + int64(len(suffix))
	resp, err := t.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload sandbox file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload sandbox file: HTTP %d", resp.StatusCode)
	}
	var id string
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&id); err != nil {
		return "", fmt.Errorf("decode sandbox file ID: %w", err)
	}
	if !validFileID(id) {
		return "", fmt.Errorf("invalid sandbox file ID")
	}
	return id, nil
}

func validFileID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\\x00")
}

// Download copies at most limit bytes to a caller-owned destination. On quota
// failure the caller must discard the incomplete destination.
// ErrQuotaExceeded reports content larger than the caller-supplied limit; retrying cannot help.
var ErrQuotaExceeded = errors.New("sandbox file exceeds quota")

func (t *FileTransport) Download(ctx context.Context, id string, dst io.Writer, limit int64) (int64, error) {
	if !validFileID(id) || limit <= 0 {
		return 0, fmt.Errorf("invalid sandbox download ID or quota")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.base+"/file/"+url.PathEscape(id), nil)
	if err != nil {
		return 0, fmt.Errorf("create sandbox download: %w", err)
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download sandbox file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download sandbox file: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return 0, ErrQuotaExceeded
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, limit))
	if err != nil {
		return n, fmt.Errorf("copy sandbox file: %w", err)
	}
	var extra [1]byte
	if _, err := io.ReadFull(resp.Body, extra[:]); !errors.Is(err, io.EOF) {
		if err != nil {
			return n, fmt.Errorf("finish sandbox download: %w", err)
		}
		return n, ErrQuotaExceeded
	}
	return n, nil
}

func (t *FileTransport) Delete(ctx context.Context, id string) error {
	if !validFileID(id) {
		return fmt.Errorf("invalid sandbox file ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.base+"/file/"+url.PathEscape(id), nil)
	if err != nil {
		return fmt.Errorf("create sandbox deletion: %w", err)
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete sandbox file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete sandbox file: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (t *FileTransport) Preview(ctx context.Context, id string, limit int64) ([]byte, bool, error) {
	if !validFileID(id) || limit <= 0 || limit > 128*1024 {
		return nil, false, fmt.Errorf("invalid preview ID or quota")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.base+"/file/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, false, fmt.Errorf("newrequestwithcontext: %w", err)
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("sandbox preview: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, false, fmt.Errorf("readall: %w", err)
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}
