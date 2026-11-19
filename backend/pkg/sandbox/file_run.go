package sandbox

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg/bytesize"
)

type FileRunLimits struct {
	CPU        time.Duration `yaml:"cpu" json:"cpu"`
	Wall       time.Duration `yaml:"wall" json:"wall"`
	Memory     bytesize.Size `yaml:"memory" json:"memory"`
	Processes  uint64        `yaml:"processes" json:"processes"`
	Output     bytesize.Size `yaml:"output" json:"output"`
	Diagnostic bytesize.Size `yaml:"diagnostic" json:"diagnostic"`
}

type FileRunResult struct {
	Status     Status
	ExitStatus int
	Time       time.Duration
	Memory     int64
	OutputID   string
	Stderr     []byte
}

func (s *Sandbox) fileCommand(executable Executable, language string, input File, args []string, extras []File, limits FileRunLimits) (*cmd, error) {
	if limits.CPU <= 0 || limits.Wall <= 0 || limits.Memory == 0 || limits.Processes == 0 || limits.Output <= 0 || limits.Diagnostic <= 0 || limits.Diagnostic > 64*1024 {
		return nil, fmt.Errorf("invalid sandbox file execution limits")
	}
	lang, ok := s.config.Langs[language]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownLanguage, language)
	}
	name := lang.CodeFile
	if lang.Type == "compiler" {
		name = lang.CompileFile
	}
	if !fs.ValidPath(name) || !validFileID(executable.PrimaryFileID) {
		return nil, fmt.Errorf("invalid executable")
	}
	copyIn := map[string]cmdFile{name: *cachedFile(executable.PrimaryFileID)}
	for path, id := range executable.Dependencies {
		if !fs.ValidPath(path) || path == name || !validFileID(id) {
			return nil, fmt.Errorf("invalid executable dependency")
		}
		copyIn[path] = *cachedFile(id)
	}
	stdin := memoryFile("")
	if input != (File{}) {
		var err error
		if stdin, err = input.request(); err != nil {
			return nil, err
		}
	}
	// The pipe charges output page-cache memory to the trusted transport instead of the contestant.
	files := []*cmdFile{stdin, collector("stdout", int64(limits.Output), true), collector("stderr", int64(limits.Diagnostic), false)}
	for _, f := range extras {
		r, err := f.request()
		if err != nil {
			return nil, err
		}
		files = append(files, r)
	}
	argv := strings.Fields(strings.NewReplacer("${name}", name, "${dir}", ".").Replace(lang.Execute))
	argv = append(argv, args...)
	return &cmd{
		Args: argv, Env: []string{"PATH=/usr/bin:/bin"}, Files: files, CopyIn: copyIn,
		CopyOutCached: []string{"stdout"},
		CPULimit:      uint64(limits.CPU), ClockLimit: uint64(limits.Wall), MemoryLimit: limits.Memory.Uint64(), ProcLimit: limits.Processes,
		CopyOutMax: uint64(limits.Output),
	}, nil
}

// RunFiles exposes extra files as /dev/fd/3 onward without copying them into tmpfs.
// The caller owns the returned output ID even when the command fails.
func (s *Sandbox) RunFiles(ctx context.Context, executable Executable, language string, input File, args []string, extras []File, limits FileRunLimits) (*FileRunResult, error) {
	c, err := s.fileCommand(executable, language, input, args, extras, limits)
	if err != nil {
		return nil, err
	}
	results, err := s.client.run(ctx, request{Cmd: []cmd{*c}})
	if err != nil {
		return nil, fmt.Errorf("execute sandbox files: %w", err)
	}
	r := results[0]
	status := statusToString(r.Status)
	if status == StatusOK && r.ExitStatus != 0 {
		status = StatusRE
	}
	return &FileRunResult{
		Status: status, ExitStatus: r.ExitStatus, Time: safeDuration(r.Time), Memory: safeInt64(r.Memory),
		OutputID: r.FileIDs["stdout"], Stderr: []byte(r.Files["stderr"]),
	}, nil
}

func (s *Sandbox) CheckFiles(ctx context.Context, executable Executable, language string, input, output, answer File, limits FileRunLimits) (*CheckerResult, error) {
	limits.Output = limits.Diagnostic
	c, err := s.fileCommand(executable, language, File{}, []string{"/dev/fd/3", "/dev/fd/4", "/dev/fd/5"}, []File{input, output, answer}, limits)
	if err != nil {
		return nil, err
	}
	c.CopyOutCached = nil
	results, err := s.client.run(ctx, request{Cmd: []cmd{*c}})
	if err != nil {
		return nil, fmt.Errorf("checker file execution: %w", err)
	}
	r := results[0]
	status, score := parseTestlibVerdict(r.ExitStatus, r.Files["stdout"], r.Files["stderr"])
	if r.Status != resultAccepted && r.Status != resultNonzeroExit {
		status = StatusFail
		score = nil
	}
	return &CheckerResult{Status: status, ExitStatus: r.ExitStatus, Message: r.Files["stderr"], Score: score}, nil
}

const interactorOutput = "output.txt"

// InteractFiles allows a zero answer; OutputID is the interactor's file for the checker.
// The caller owns OutputID even on failure.
func (s *Sandbox) InteractFiles(ctx context.Context, solution Executable, solutionLanguage string, interactor Executable, interactorLanguage string, input, answer File, solutionLimits, interactorLimits FileRunLimits) (*InteractiveResult, error) {
	sol, err := s.fileCommand(solution, solutionLanguage, File{}, nil, nil, solutionLimits)
	if err != nil {
		return nil, err
	}
	args, extra := []string{"/dev/fd/3", interactorOutput}, []File{input}
	if answer != (File{}) {
		args, extra = append(args, "/dev/fd/4"), append(extra, answer)
	}
	intr, err := s.fileCommand(interactor, interactorLanguage, File{}, args, extra, interactorLimits)
	if err != nil {
		return nil, err
	}
	for _, c := range []*cmd{sol, intr} {
		c.Files[0] = nil
		c.Files[1] = nil
		c.CopyOutCached = nil
	}
	// An interactor that writes no output still yields an empty one.
	intr.CopyIn[interactorOutput] = *memoryFile("")
	intr.CopyOutCached = []string{interactorOutput}
	pipes := []pipeMap{
		{In: pipeIndex{Index: 0, Fd: 1}, Out: pipeIndex{Index: 1, Fd: 0}, Proxy: true, Max: int64(solutionLimits.Output)},
		{In: pipeIndex{Index: 1, Fd: 1}, Out: pipeIndex{Index: 0, Fd: 0}, Proxy: true, Max: int64(interactorLimits.Output)},
	}
	results, err := s.client.run(ctx, request{Cmd: []cmd{*sol, *intr}, PipeMapping: pipes})
	if err != nil {
		return nil, fmt.Errorf("interactive file execution: %w", err)
	}
	a, b := results[0], results[1]
	outputID := b.FileIDs[interactorOutput]
	status, score := parseTestlibVerdict(b.ExitStatus, "", b.Files["stderr"])
	if b.Status != resultAccepted && b.Status != resultNonzeroExit {
		status = StatusFail
		score = nil
	}
	if (status == StatusOK || status == StatusPoints) && a.Status != resultAccepted {
		status = statusToString(a.Status)
		score = nil
	}
	return &InteractiveResult{Status: status, Score: score, Message: b.Files["stderr"], OutputID: outputID,
		SolutionResult:   RunResult{Status: statusToString(a.Status), ExitStatus: a.ExitStatus, Time: safeDuration(a.Time), Memory: safeInt64(a.Memory), Stderr: []byte(a.Files["stderr"])},
		InteractorResult: RunResult{Status: statusToString(b.Status), ExitStatus: b.ExitStatus, Time: safeDuration(b.Time), Memory: safeInt64(b.Memory), Stderr: []byte(b.Files["stderr"])}}, nil
}
