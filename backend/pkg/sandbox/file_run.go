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

// GenerateStep is one program of a generator pipeline.
type GenerateStep struct {
	Executable Executable
	Language   string
	Args       []string
}

// GenerateResult reports the last step that failed, if any; the caller owns OutputID and FileIDs even on failure.
type GenerateResult struct {
	FileRunResult
	Step    int
	FileIDs map[string]string
}

const sigpipe = 13

// Generate runs steps connected by pipes, each under limits. The output is the last step's standard output or, with files,
// those of the files it wrote. A step killed by SIGPIPE does not fail, as in a shell pipeline, and the last failed step is
// reported, since a reader that fails breaks the pipe of the writers before it.
func (s *Sandbox) Generate(ctx context.Context, steps []GenerateStep, files []string, limits FileRunLimits) (*GenerateResult, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("no generator steps")
	}
	cmds := make([]cmd, len(steps))
	var pipes []pipeMap
	for i, step := range steps {
		c, err := s.fileCommand(step.Executable, step.Language, File{}, step.Args, nil, limits)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			c.Files[0] = nil
			// A proxy would drain the writer after the reader exits; a direct pipe sends it SIGPIPE, as a shell does.
			pipes = append(pipes, pipeMap{In: pipeIndex{Index: i - 1, Fd: 1}, Out: pipeIndex{Index: i, Fd: 0}})
		}
		if i < len(steps)-1 {
			c.Files[1], c.CopyOutCached = nil, nil
		}
		cmds[i] = *c
	}
	last := &cmds[len(cmds)-1]
	if len(files) > 0 {
		last.CopyOutCached = nil
		for _, name := range files {
			if _, taken := last.CopyIn[name]; !ValidSolutionFile(name) || taken {
				return nil, fmt.Errorf("generated file %q is invalid or clashes with the executable", name)
			}
			last.CopyOutCached = append(last.CopyOutCached, name+"?")
		}
	}
	results, err := s.client.run(ctx, request{Cmd: cmds, PipeMapping: pipes})
	if err != nil {
		return nil, fmt.Errorf("execute generator: %w", err)
	}
	out := &GenerateResult{FileRunResult: FileRunResult{Status: StatusOK}, FileIDs: map[string]string{}}
	r := results[len(results)-1]
	out.OutputID = r.FileIDs["stdout"]
	for _, name := range files {
		if id, ok := r.FileIDs[name]; ok {
			out.FileIDs[name] = id
		}
	}
	for i, r := range results {
		out.Time, out.Memory = max(out.Time, safeDuration(r.Time)), max(out.Memory, safeInt64(r.Memory))
		status := statusToString(r.Status)
		if status == StatusOK && r.ExitStatus != 0 {
			status = StatusRE
		}
		if i == len(results)-1 && slices.ContainsFunc(r.FileError, func(e fileError) bool { return e.Type == "CopyOutSizeExceeded" }) {
			status = StatusOLE
		}
		if status == StatusOK || i < len(results)-1 && r.Status == resultSignalled && r.ExitStatus == sigpipe {
			continue
		}
		out.Status, out.Step, out.ExitStatus, out.Stderr = status, i, r.ExitStatus, []byte(r.Files["stderr"])
	}
	return out, nil
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
	// As in DOMjudge, a rejection wins over the solution's failure only when the interactor exited first.
	interactorRejected := status != StatusOK && status != StatusPoints && exitedBefore(b, results[:m])
	if sol.Status != StatusOK && status != StatusFail && !interactorRejected {
		status, score = sol.Status, nil
	}
	return &InteractiveResult{Status: status, Score: score, Message: message, OutputID: outputID, SolutionResult: sol,
		InteractorResult: RunResult{Status: statusToString(b.Status), ExitStatus: b.ExitStatus, Time: safeDuration(b.Time), Memory: safeInt64(b.Memory), Stderr: []byte(b.Files["stderr"])}}, nil
}

// exitSkew covers go-judge recording a killed process's end a few milliseconds after its pipe peer sees EOF.
const exitSkew = 100 * time.Millisecond

// exitedBefore reports whether r ended before the first failed solution copy.
func exitedBefore(r result, solutions []result) bool {
	for _, s := range solutions {
		if s.Status != resultAccepted && safeDuration(r.RunTime)+exitSkew >= safeDuration(s.RunTime) {
			return false
		}
	}
	return true
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
