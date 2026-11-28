package sandbox

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	return s.compile(ctx, sourceCode, langKey, dependencies, nil, limits)
}

// CompileWithGraders builds the source with the graders as its language's configuration says; other languages cannot.
func (s *Sandbox) CompileWithGraders(ctx context.Context, sourceCode []byte, langKey string, graders Graders, limits FileRunLimits) (Executable, error) {
	return s.compile(ctx, sourceCode, langKey, nil, &graders, limits)
}

func (s *Sandbox) compile(ctx context.Context, sourceCode []byte, langKey string, dependencies map[string][]byte, graders *Graders, limits FileRunLimits) (Executable, error) {
	if limits.CPU <= 0 || limits.Wall <= 0 || limits.Memory == 0 || limits.Processes == 0 || limits.Output <= 0 || limits.Diagnostic <= 0 || limits.Diagnostic > 64*1024 {
		return Executable{}, fmt.Errorf("invalid compilation limits")
	}

	lang, err := s.config.Lookup(langKey)
	if err != nil {
		return Executable{}, err
	}
	outputName := cmp.Or(lang.CompileFile, "solution")
	codeFile, compileCmd := lang.CodeFile, lang.Compile
	if strings.Contains(codeFile+compileCmd, classArg) {
		r := strings.NewReplacer(classArg, javaClass(sourceCode))
		codeFile, compileCmd = r.Replace(codeFile), r.Replace(compileCmd)
	}
	var linked []string
	if graders != nil {
		g, ok := lang.graded(graders.Solution)
		if !ok || !lang.Graded(slices.Collect(maps.Keys(graders.Files)), graders.Solution) {
			return Executable{}, fmt.Errorf("%w: %s", ErrGradersUnsupported, langKey)
		}
		for _, name := range slices.Sorted(maps.Keys(graders.Files)) {
			if lang.compiles(name) {
				linked = append(linked, name)
			}
		}
		dependencies = maps.Clone(graders.Files)
		codeFile, compileCmd = g.codeFile, g.compile
		if g.entry != "" && lang.Type == "interpreter" {
			// The entry grader runs under the solution's usual name, so running needs no grader settings.
			delete(dependencies, g.entry)
			dependencies[g.codeFile] = sourceCode
			codeFile, sourceCode = lang.CodeFile, graders.Files[g.entry]
		}
	}

	var uploaded []string
	defer func() { s.client.release(uploaded...) }()
	copyIn := make(map[string]cmdFile)
	if copyIn[codeFile], err = s.inline(ctx, sourceCode, &uploaded); err != nil {
		return Executable{}, err
	}
	for depName, depContent := range dependencies {
		if copyIn[depName], err = s.inline(ctx, depContent, &uploaded); err != nil {
			return Executable{}, err
		}
	}

	if lang.Type == "interpreter" || compileCmd == "" {
		copyOutCached := []string{codeFile}
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

		fileID, ok := res.FileIDs[codeFile]
		if !ok {
			return Executable{}, fmt.Errorf("fileID not found for cached script %s", codeFile)
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

	args := []string{}
	for arg := range strings.FieldsSeq(strings.ReplaceAll(compileCmd, "${name}", outputName)) {
		if arg == gradersArg {
			args = append(args, linked...)
		} else {
			args = append(args, arg)
		}
	}
	results, err := s.client.run(ctx, request{Cmd: []cmd{{
		Args:          args,
		Files:         []*cmdFile{memoryFile(""), collector("stdout", int64(limits.Diagnostic), false), collector("stderr", int64(limits.Diagnostic), false)},
		CopyIn:        copyIn,
		CopyOutCached: []string{outputName},
		ClockLimit:    uint64(limits.Wall),
		CopyOutMax:    uint64(limits.Output),
		CPULimit:      uint64(limits.CPU),
		MemoryLimit:   limits.Memory.Uint64(),
		ProcLimit:     limits.Processes,
		// javac rejects non-ASCII file names, such as a Java class named in Cyrillic, under the C locale.
		Env: []string{"PATH=/usr/bin:/bin", "GOCACHE=/var/cache/go", "CGO_ENABLED=0", "HOME=/tmp", "LANG=C.UTF-8"},
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
