package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg/booklet"
)

func safeUint64[T ~int | ~int64](v T) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func safeInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func safeDuration(v uint64) time.Duration {
	if v > uint64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(v)
}

type Status string

const (
	dummyPipeCollectorSize = 1024
)

const (
	StatusOK     Status = "OK"
	StatusWA     Status = "WA"
	StatusPE     Status = "PE"
	StatusTLE    Status = "TLE"
	StatusMLE    Status = "MLE"
	StatusOLE    Status = "OLE"
	StatusRE     Status = "RE"
	StatusCE     Status = "CE"
	StatusFail   Status = "FAIL"
	StatusPoints Status = "POINTS"
)

type RunResult struct {
	Status     Status
	ExitStatus int
	Time       time.Duration
	Memory     int64 // in bytes
	Stdout     []byte
	Stderr     []byte
}

type CheckerResult struct {
	Status     Status
	ExitStatus int
	Message    string
	Score      *float64
}

type InteractiveResult struct {
	Status           Status
	SolutionResult   RunResult
	InteractorResult RunResult
	OutputID         string // set only by InteractFiles
	Message          string
	Score            *float64
}

type Executable struct {
	PrimaryFileID string
	Dependencies  map[string]string // filename -> sandbox file ID
}

type Sandbox struct {
	client *Client
	config *Config
}

func NewSandbox(client *Client, config *Config) *Sandbox {
	return &Sandbox{
		client: client,
		config: config,
	}
}

// inline sends text in the JSON request; other bytes would be mangled there, so they go through the file store.
func (s *Sandbox) inline(ctx context.Context, data []byte, uploaded *[]string) (cmdFile, error) {
	if utf8.Valid(data) {
		return *memoryFile(string(data)), nil
	}
	id, err := s.client.upload(ctx, data)
	if err != nil {
		return cmdFile{}, err
	}
	*uploaded = append(*uploaded, id)
	return *cachedFile(id), nil
}

func (s *Sandbox) CompileWithLimits(ctx context.Context, sourceCode []byte, langKey string, dependencies map[string][]byte, limits FileRunLimits) (Executable, error) {
	if limits.CPU <= 0 || limits.Wall <= 0 || limits.Memory == 0 || limits.Processes == 0 || limits.Output <= 0 || limits.Diagnostic <= 0 || limits.Diagnostic > 64*1024 {
		return Executable{}, fmt.Errorf("invalid compilation limits")
	}

	lang, ok := s.config.Langs[langKey]
	if !ok {
		return Executable{}, fmt.Errorf("%w: %s", ErrUnknownLanguage, langKey)
	}

	var uploaded []string
	defer func() { s.client.release(uploaded...) }()
	copyIn := make(map[string]cmdFile)
	var err error
	if copyIn[lang.CodeFile], err = s.inline(ctx, sourceCode, &uploaded); err != nil {
		return Executable{}, err
	}
	for depName, depContent := range dependencies {
		if copyIn[depName], err = s.inline(ctx, depContent, &uploaded); err != nil {
			return Executable{}, err
		}
	}

	if lang.Type == "interpreter" || lang.Compile == "" {
		copyOutCached := []string{lang.CodeFile}
		for depName := range dependencies {
			copyOutCached = append(copyOutCached, depName)
		}
		results, err := s.client.run(ctx, request{Cmd: []cmd{{
			Args:          []string{"/bin/true"},
			Files:         []*cmdFile{memoryFile(""), collector("stdout", dummyPipeCollectorSize, false), collector("stderr", dummyPipeCollectorSize, false)},
			CopyIn:        copyIn,
			CopyOutCached: copyOutCached,
			ClockLimit:    uint64(limits.Wall),
			CopyOutMax:    uint64(limits.Output),
			CPULimit:      uint64(limits.CPU),
			MemoryLimit:   limits.Memory.Uint64(),
			ProcLimit:     limits.Processes,
			Env:           []string{"PATH=/usr/bin:/bin", "GOCACHE=/var/cache/go", "CGO_ENABLED=0", "HOME=/tmp"},
		}}})
		if err != nil {
			return Executable{}, fmt.Errorf("failed to cache script: %w", err)
		}
		res := results[0]
		if res.Status != resultAccepted {
			return Executable{}, fmt.Errorf("caching dummy execution failed: status=%v, exitStatus=%v, stderr=%s",
				res.Status, res.ExitStatus, res.Files["stderr"])
		}

		fileID, ok := res.FileIDs[lang.CodeFile]
		if !ok {
			return Executable{}, fmt.Errorf("fileID not found for cached script %s", lang.CodeFile)
		}

		cachedDeps := make(map[string]string)
		for depName := range dependencies {
			depFileID, ok := res.FileIDs[depName]
			if !ok {
				return Executable{}, fmt.Errorf("fileID not found for dependency %s", depName)
			}
			cachedDeps[depName] = depFileID
		}

		return Executable{
			PrimaryFileID: fileID,
			Dependencies:  cachedDeps,
		}, nil
	}

	outputName := lang.CompileFile
	if outputName == "" {
		outputName = "solution"
	}

	compileCmd := strings.ReplaceAll(lang.Compile, "${name}", outputName)
	results, err := s.client.run(ctx, request{Cmd: []cmd{{
		Args:          strings.Fields(compileCmd),
		Files:         []*cmdFile{memoryFile(""), collector("stdout", int64(limits.Diagnostic), false), collector("stderr", int64(limits.Diagnostic), false)},
		CopyIn:        copyIn,
		CopyOutCached: []string{outputName},
		ClockLimit:    uint64(limits.Wall),
		CopyOutMax:    uint64(limits.Output),
		CPULimit:      uint64(limits.CPU),
		MemoryLimit:   limits.Memory.Uint64(),
		ProcLimit:     limits.Processes,
		Env:           []string{"PATH=/usr/bin:/bin", "GOCACHE=/var/cache/go", "CGO_ENABLED=0", "HOME=/tmp"},
	}}})
	if err != nil {
		return Executable{}, fmt.Errorf("compilation failed to dispatch: %w", err)
	}

	res := results[0]
	if res.Status != resultAccepted {
		return Executable{}, fmt.Errorf("compilation failed:\nstatus: %v\nexit code: %d\nstdout: %s\nstderr: %s",
			res.Status, res.ExitStatus, res.Files["stdout"], res.Files["stderr"])
	}

	fileID, ok := res.FileIDs[outputName]
	if !ok {
		return Executable{}, fmt.Errorf("fileID not found in compilation results for %s", outputName)
	}
	return Executable{
		PrimaryFileID: fileID,
		Dependencies:  nil,
	}, nil
}

func statusToString(status string) Status {
	switch status {
	case resultAccepted:
		return StatusOK
	case resultMemoryLimit:
		return StatusMLE
	case resultTimeLimit:
		return StatusTLE
	case resultOutputLimit:
		return StatusOLE
	case resultNonzeroExit, resultSignalled:
		return StatusRE
	default:
		return StatusFail
	}
}

// latexError keeps the log from the first TeX error ("! ..."), which names the problem and its line.
func latexError(log string) string {
	const limit = 4096
	if i := strings.Index(log, "\n! "); i >= 0 {
		return log[i+1 : i+1+min(limit, len(log)-i-1)]
	}
	return log[max(0, len(log)-limit):]
}

// CompilePDF limits the PDF by limits.Output and the reported log tail by limits.Diagnostic.
func (s *Sandbox) CompilePDF(ctx context.Context, texSource string, limits FileRunLimits) ([]byte, error) {
	latex := "pdflatex -interaction=nonstopmode -halt-on-error booklet.tex >/dev/null"
	results, err := s.client.run(ctx, request{Cmd: []cmd{{
		// The error is at the end of the log, so only its tail leaves the sandbox.
		Args:          []string{"/bin/sh", "-c", fmt.Sprintf("{ %s && %s; } || { tail -c %d booklet.log; exit 1; }", latex, latex, int64(limits.Diagnostic))},
		Files:         []*cmdFile{memoryFile(""), collector("stdout", int64(limits.Diagnostic), false), collector("stderr", int64(limits.Diagnostic), false)},
		CopyIn:        map[string]cmdFile{"booklet.tex": *memoryFile(texSource), "olymp.sty": *memoryFile(string(booklet.OlympSty))},
		CopyOutCached: []string{"booklet.pdf"},
		CPULimit:      safeUint64(limits.CPU.Nanoseconds()),
		ClockLimit:    safeUint64(limits.Wall.Nanoseconds()),
		MemoryLimit:   limits.Memory.Uint64(),
		ProcLimit:     limits.Processes,
		Env:           []string{"PATH=/usr/bin:/bin", "HOME=/tmp", "TEXMFVAR=/tmp/texmf-var", "TEXMFCONFIG=/tmp/texmf-config"},
	}}})
	if err != nil {
		return nil, fmt.Errorf("LaTeX compilation failed to dispatch: %w", err)
	}

	res := results[0]
	pdfID := res.FileIDs["booklet.pdf"]
	defer s.client.release(pdfID)
	switch res.Status {
	case resultAccepted:
	case resultNonzeroExit, resultTimeLimit, resultMemoryLimit, resultOutputLimit:
		return nil, fmt.Errorf("LaTeX compilation error: status=%v, exitStatus=%v\n%s",
			res.Status, res.ExitStatus, latexError(res.Files["stdout"]+res.Files["stderr"]))
	default:
		// go-judge's own failures (Internal Error, File Error) say nothing about the source.
		return nil, fmt.Errorf("%w: LaTeX run: %s: %s", ErrUnavailable, res.Status, res.Error)
	}
	if pdfID == "" {
		return nil, fmt.Errorf("LaTeX compilation succeeded but booklet.pdf was not produced")
	}
	var pdf bytes.Buffer
	if _, err := s.client.get(ctx, pdfID, &pdf, int64(limits.Output)); err != nil {
		return nil, fmt.Errorf("download booklet.pdf: %w", err)
	}
	if pdf.Len() == 0 {
		return nil, fmt.Errorf("LaTeX compilation succeeded but booklet.pdf is empty")
	}
	return pdf.Bytes(), nil
}
