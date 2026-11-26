package sandbox

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Typesetter runs TeX Live, pandoc and rsvg-convert in the typeset go-judge, never in a plain container: both read files by name.
type Typesetter struct {
	client *Client
	limits FileRunLimits
}

func NewTypesetter(client *Client, limits FileRunLimits) *Typesetter {
	return &Typesetter{client: client, limits: limits}
}

// TeXError is the first LaTeX error; Line counts lines of the compiled document, 0 when the log names none.
type TeXError struct {
	Line    int
	Message string
}

func (e *TeXError) Error() string {
	if e.Line == 0 {
		return "LaTeX error: " + e.Message
	}
	return fmt.Sprintf("LaTeX error at line %d: %s", e.Line, e.Message)
}

// PandocContents is the text of a log entry, or compact JSON when pandoc logs an AST node.
type PandocContents string

func (c *PandocContents) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		*c = PandocContents(s)
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return fmt.Errorf("compact contents: %w", err)
	}
	*c = PandocContents(buf.String())
	return nil
}

// PandocMessage is an entry of pandoc --log.
type PandocMessage struct {
	Type     string         `json:"type"`
	Contents PandocContents `json:"contents"`
	Line     int            `json:"line"`
	Column   int            `json:"column"`
	Pretty   string         `json:"pretty"`
}

var texErrorLine = regexp.MustCompile(`(?m)^\./doc\.tex:(\d+): (.*)$`)

// PDF compiles doc.tex twice, for references, next to files; SVG images become PDF under their own name.
func (t *Typesetter) PDF(ctx context.Context, doc string, files map[string][]byte) ([]byte, error) {
	copyIn := map[string]cmdFile{"doc.tex": *memoryFile(doc)}
	var uploaded []string
	defer func() { t.client.release(uploaded...) }()
	for _, name := range slices.Sorted(maps.Keys(files)) {
		data := files[name]
		if utf8.Valid(data) {
			copyIn[name] = *memoryFile(string(data))
			continue
		}
		id, err := t.client.upload(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("%w: upload %s: %w", ErrUnavailable, name, err)
		}
		uploaded = append(uploaded, id)
		copyIn[name] = *cachedFile(id)
	}
	latex := "pdflatex -no-shell-escape -file-line-error -interaction=nonstopmode -halt-on-error doc.tex >/dev/null"
	script := `shopt -s globstar nullglob
for f in **/*.svg; do rsvg-convert -f pdf -o "$f.pdf" "$f" && mv "$f.pdf" "$f" || exit 2; done
{ ` + latex + " && " + latex + fmt.Sprintf("; } || { tail -c %d doc.log; exit 1; }", int64(t.limits.Diagnostic))
	res, err := t.run(ctx, script, copyIn, nil, nil, []string{"doc.pdf"})
	if err != nil {
		return nil, err
	}
	id := res.FileIDs["doc.pdf"]
	defer t.client.release(id)
	switch {
	case res.Status == resultNonzeroExit && res.ExitStatus == 1:
		return nil, texError(res.Files["stdout"])
	case res.Status != resultAccepted:
		return nil, fmt.Errorf("typeset PDF: %s (exit %d): %s", res.Status, res.ExitStatus, res.Files["stderr"])
	case id == "":
		return nil, errors.New("typeset PDF: pdflatex wrote no doc.pdf")
	}
	var pdf bytes.Buffer
	if _, err := t.client.get(ctx, id, &pdf, int64(t.limits.Output)); err != nil {
		return nil, fmt.Errorf("download PDF: %w", err)
	}
	return pdf.Bytes(), nil
}

func texError(log string) *TeXError {
	if m := texErrorLine.FindStringSubmatch(log); m != nil {
		line, _ := strconv.Atoi(m[1])
		return &TeXError{Line: line, Message: m[2]}
	}
	if i := strings.Index(log, "\n! "); i >= 0 {
		message, _, _ := strings.Cut(log[i+3:], "\n")
		return &TeXError{Message: message}
	}
	return &TeXError{Message: strings.TrimSpace(log[max(0, len(log)-512):])}
}

// HTML converts a LaTeX fragment with pandoc, leaving math to KaTeX in the browser.
func (t *Typesetter) HTML(ctx context.Context, tex string) (string, []PandocMessage, error) {
	return t.pandoc(ctx, tex, "-f", "latex", "-t", "html", "--mathjax")
}

//go:embed plain.lua
var plainLaTeX string

// LaTeX converts Markdown or HTML, as pandoc names the format, into LaTeX that HTML and PDF typesetting read back.
func (t *Typesetter) LaTeX(ctx context.Context, from, text string) (string, []PandocMessage, error) {
	return t.pandoc(ctx, text, "-f", from, "-t", "latex", "--lua-filter=plain.lua")
}

func (t *Typesetter) pandoc(ctx context.Context, input string, args ...string) (string, []PandocMessage, error) {
	args = append([]string{"pandoc", "--wrap=none", "--log=log.json", "-o", "out", "in"}, args...)
	files := map[string]cmdFile{"in": *memoryFile(input), "plain.lua": *memoryFile(plainLaTeX)}
	res, err := t.run(ctx, `exec "$@"`, files, args, []string{"out?", "log.json?"}, nil)
	if err != nil {
		return "", nil, err
	}
	if res.Status != resultAccepted {
		return "", nil, fmt.Errorf("pandoc: %s (exit %d): %s", res.Status, res.ExitStatus, res.Files["stderr"])
	}
	var log []PandocMessage
	if err := json.Unmarshal([]byte(res.Files["log.json"]), &log); err != nil {
		return "", nil, fmt.Errorf("decode pandoc log: %w", err)
	}
	return res.Files["out"], log, nil
}

// run executes a bash script in the sandbox; go-judge failures, unlike the script's, are ErrUnavailable.
func (t *Typesetter) run(ctx context.Context, script string, copyIn map[string]cmdFile, args, copyOut, cached []string) (result, error) {
	l := t.limits
	results, err := t.client.run(ctx, request{Cmd: []cmd{{
		Args:          append([]string{"/bin/bash", "-c", script, "typeset"}, args...),
		Files:         []*cmdFile{memoryFile(""), collector("stdout", int64(l.Diagnostic), false), collector("stderr", int64(l.Diagnostic), false)},
		CopyIn:        copyIn,
		CopyOut:       copyOut,
		CopyOutCached: cached,
		CopyOutMax:    safeUint64(int64(l.Output)),
		CPULimit:      safeUint64(l.CPU.Nanoseconds()),
		ClockLimit:    safeUint64(l.Wall.Nanoseconds()),
		MemoryLimit:   l.Memory.Uint64(),
		ProcLimit:     l.Processes,
		Env:           []string{"PATH=/usr/bin:/bin", "HOME=/tmp", "TEXMFVAR=/tmp/texmf-var", "TEXMFCONFIG=/tmp/texmf-config"},
	}}})
	if err != nil {
		return result{}, fmt.Errorf("%w: typeset run: %w", ErrUnavailable, err)
	}
	res := results[0]
	switch res.Status {
	case resultAccepted, resultNonzeroExit, resultTimeLimit, resultMemoryLimit, resultOutputLimit:
		return res, nil
	}
	t.client.release(slices.Collect(maps.Values(res.FileIDs))...)
	return result{}, fmt.Errorf("%w: typeset run: %s: %s", ErrUnavailable, res.Status, res.Error)
}
