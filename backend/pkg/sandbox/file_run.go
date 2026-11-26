package sandbox

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg/bytesize"
)

type FileRunLimits struct {
	CPU time.Duration `yaml:"cpu" json:"cpu"`
	// Derived from CPU by problems.limits.wall_factor.
	Wall       time.Duration `yaml:"-" json:"wall"`
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
	return s.runFile(ctx, c, "stdout")
}

// Streams name the files a solution reads and writes instead of standard input and output; empty keeps the stream.
type Streams struct{ Input, Output string }

// ValidSolutionFile excludes the stream collector names, which go-judge never copies out as files.
func ValidSolutionFile(name string) bool {
	return fs.ValidPath(name) && !strings.Contains(name, "/") && name != "stdout" && name != "stderr"
}

// RunSolution reads a missing output file as empty; the caller owns the returned output ID.
func (s *Sandbox) RunSolution(ctx context.Context, executable Executable, language string, input File, streams Streams, limits FileRunLimits) (*FileRunResult, error) {
	stdin := input
	if streams.Input != "" {
		stdin = File{}
	}
	c, err := s.fileCommand(executable, language, stdin, nil, nil, limits)
	if err != nil {
		return nil, err
	}
	output := "stdout"
	for name, f := range map[string]File{streams.Input: input, streams.Output: {}} {
		if name == "" {
			continue
		}
		if _, taken := c.CopyIn[name]; !ValidSolutionFile(name) || taken {
			return nil, fmt.Errorf("solution file %q is invalid or clashes with the executable", name)
		}
		r := memoryFile("")
		if f != (File{}) {
			if r, err = f.request(); err != nil {
				return nil, err
			}
		}
		c.CopyIn[name] = *r
	}
	if streams.Output != "" {
		output = streams.Output
		c.CopyOutCached = []string{output}
	}
	return s.runFile(ctx, c, output)
}

func (s *Sandbox) runFile(ctx context.Context, c *cmd, output string) (*FileRunResult, error) {
	results, err := s.client.run(ctx, request{Cmd: []cmd{*c}})
	if err != nil {
		return nil, fmt.Errorf("execute sandbox files: %w", err)
	}
	r := results[0]
	status := statusToString(r.Status)
	if status == StatusOK && r.ExitStatus != 0 {
		status = StatusRE
	}
	if slices.Contains(r.FileError, fileError{Name: output, Type: "CopyOutSizeExceeded"}) {
		status = StatusOLE
	}
	return &FileRunResult{
		Status: status, ExitStatus: r.ExitStatus, Time: safeDuration(r.Time), Memory: safeInt64(r.Memory),
		OutputID: r.FileIDs[output], Stderr: []byte(r.Files["stderr"]),
	}, nil
}

// CheckFiles gives an ICPC checker the output on standard input.
func (s *Sandbox) CheckFiles(ctx context.Context, checker Jury, input, output, answer File, limits FileRunLimits) (*CheckerResult, error) {
	limits.Output = limits.Diagnostic
	stdin, args, extras := File{}, []string{"/dev/fd/3", "/dev/fd/4", "/dev/fd/5"}, []File{input, output, answer}
	if checker.Protocol == ProtocolICPC {
		stdin, args, extras = output, []string{"/dev/fd/3", "/dev/fd/4", feedbackDir}, []File{input, answer}
	}
	c, err := s.fileCommand(checker.Executable, checker.Language, stdin, append(args, checker.Args...), extras, limits)
	if err != nil {
		return nil, err
	}
	c.CopyOutCached = nil
	if checker.Protocol == ProtocolICPC {
		icpcFeedback(c)
	}
	results, err := s.client.run(ctx, request{Cmd: []cmd{*c}})
	if err != nil {
		return nil, fmt.Errorf("checker file execution: %w", err)
	}
	r := results[0]
	status, score, message := checker.verdict(r, r.Files["stdout"])
	return &CheckerResult{Status: status, ExitStatus: r.ExitStatus, Message: message, Score: score}, nil
}

const interactorOutput = "output.txt"

// InteractFiles allows a zero answer; OutputID is a testlib interactor's file for the checker.
// With processes > 0, the interactor manages that many solution copies as ProblemDraft.processes in the API contract describes.
// The caller owns OutputID even on failure.
func (s *Sandbox) InteractFiles(ctx context.Context, solution Executable, solutionLanguage string, interactor Jury, processes int, input, answer File, solutionLimits, interactorLimits FileRunLimits) (*InteractiveResult, error) {
	copies := max(processes, 1)
	cmds := make([]cmd, 0, copies+1)
	for i := range copies {
		var args []string
		if processes > 0 {
			args = []string{strconv.Itoa(i)}
		}
		sol, err := s.fileCommand(solution, solutionLanguage, File{}, args, nil, solutionLimits)
		if err != nil {
			return nil, err
		}
		sol.Files[0], sol.Files[1], sol.CopyOutCached = nil, nil, nil
		cmds = append(cmds, *sol)
	}
	args, extra := []string{"/dev/fd/3", interactorOutput}, []File{input}
	if answer != (File{}) {
		args, extra = append(args, "/dev/fd/4"), append(extra, answer)
	}
	if interactor.Protocol == ProtocolICPC {
		args = []string{"/dev/fd/3", "/dev/null", feedbackDir}
		if answer != (File{}) {
			args[1] = "/dev/fd/4"
		}
	}
	intr, err := s.fileCommand(interactor.Executable, interactor.Language, File{}, append(args, interactor.Args...), extra, interactorLimits)
	if err != nil {
		return nil, err
	}
	if interactor.Protocol == ProtocolICPC {
		icpcFeedback(intr)
	}
	m := len(cmds)
	var pipes []pipeMap
	if processes == 0 {
		intr.Files[0], intr.Files[1] = nil, nil
		pipes = []pipeMap{
			{In: pipeIndex{Index: 0, Fd: 1}, Out: pipeIndex{Index: m, Fd: 0}, Proxy: true, Max: int64(solutionLimits.Output)},
			{In: pipeIndex{Index: m, Fd: 1}, Out: pipeIndex{Index: 0, Fd: 0}, Proxy: true, Max: int64(interactorLimits.Output)},
		}
	} else {
		if answer == (File{}) {
			intr.Files = append(intr.Files, memoryFile(""))
		}
		for i := range copies {
			intr.Files = append(intr.Files, nil, nil)
			pipes = append(pipes,
				pipeMap{In: pipeIndex{Index: i, Fd: 1}, Out: pipeIndex{Index: m, Fd: 5 + 2*i}, Proxy: true, Max: int64(solutionLimits.Output)},
				pipeMap{In: pipeIndex{Index: m, Fd: 6 + 2*i}, Out: pipeIndex{Index: i, Fd: 0}, Proxy: true, Max: int64(interactorLimits.Output)})
		}
	}
	// An interactor that writes no output still yields an empty one.
	intr.CopyIn[interactorOutput] = *memoryFile("")
	intr.CopyOutCached = []string{interactorOutput}
	results, err := s.client.run(ctx, request{Cmd: append(cmds, *intr), PipeMapping: pipes})
	if err != nil {
		return nil, fmt.Errorf("interactive file execution: %w", err)
	}
	b := results[m]
	outputID := b.FileIDs[interactorOutput]
	status, score, message := interactor.verdict(b, "")
	sol := solutionRun(results[:m])
	// As in DOMjudge, a rejection wins only when the interactor exited first; otherwise the solution's failure caused it.
	interactorFirst := !slices.ContainsFunc(results[:m], func(r result) bool { return r.RunTime <= b.RunTime })
	interactorRejected := interactorFirst && status != StatusOK && status != StatusPoints
	if sol.Status != StatusOK && status != StatusFail && !interactorRejected {
		status, score = sol.Status, nil
	}
	return &InteractiveResult{Status: status, Score: score, Message: message, OutputID: outputID, SolutionResult: sol,
		InteractorResult: RunResult{Status: statusToString(b.Status), ExitStatus: b.ExitStatus, Time: safeDuration(b.Time), Memory: safeInt64(b.Memory), Stderr: []byte(b.Files["stderr"])}}, nil
}

func limitStatus(s Status) bool { return s == StatusTLE || s == StatusMLE || s == StatusOLE }

// solutionRun reports the largest time and memory of the copies and the copy that failed, preferring an exceeded limit
// over a copy killed by the pipes closing after it.
func solutionRun(results []result) RunResult {
	run := RunResult{Status: StatusOK}
	for _, r := range results {
		run.Time, run.Memory = max(run.Time, safeDuration(r.Time)), max(run.Memory, safeInt64(r.Memory))
		if status := statusToString(r.Status); r.Status != resultAccepted && (run.Status == StatusOK || limitStatus(status) && !limitStatus(run.Status)) {
			run.Status, run.ExitStatus, run.Stderr = status, r.ExitStatus, []byte(r.Files["stderr"])
		}
	}
	if run.Status == StatusOK {
		run.ExitStatus, run.Stderr = results[0].ExitStatus, []byte(results[0].Files["stderr"])
	}
	return run
}
