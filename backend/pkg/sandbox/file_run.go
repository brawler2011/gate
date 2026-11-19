package sandbox

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg/bytesize"
	pb "github.com/criyle/go-judge/pb"
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

func (s *Sandbox) fileCommand(executable Executable, language string, input File, args []string, extras []File, limits FileRunLimits) (*pb.Request_CmdType, error) {
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
	copyIn := map[string]*pb.Request_File{name: newCachedFile(executable.PrimaryFileID)}
	for path, id := range executable.Dependencies {
		if !fs.ValidPath(path) || path == name || !validFileID(id) {
			return nil, fmt.Errorf("invalid executable dependency")
		}
		copyIn[path] = newCachedFile(id)
	}
	stdin := newMemoryFile(nil)
	if input != (File{}) {
		var err error
		if stdin, err = input.request(); err != nil {
			return nil, err
		}
	}
	// The pipe charges output page-cache memory to the trusted transport instead of the contestant.
	stdout := pb.Request_File_builder{Pipe: pb.Request_PipeCollector_builder{Name: "stdout", Max: int64(limits.Output), Pipe: true}.Build()}.Build()
	files := []*pb.Request_File{stdin, stdout, newPipeCollector("stderr", int64(limits.Diagnostic))}
	for _, f := range extras {
		r, err := f.request()
		if err != nil {
			return nil, err
		}
		files = append(files, r)
	}
	argv := strings.Fields(strings.NewReplacer("${name}", name, "${dir}", ".").Replace(lang.Execute))
	argv = append(argv, args...)
	cmd := pb.Request_CmdType_builder{
		Args: argv, Env: []string{"PATH=/usr/bin:/bin"}, Files: files, CopyIn: copyIn,
		CopyOutCached: []*pb.Request_CmdCopyOutFile{pb.Request_CmdCopyOutFile_builder{Name: "stdout"}.Build()},
		CpuTimeLimit:  uint64(limits.CPU), ClockTimeLimit: uint64(limits.Wall), MemoryLimit: limits.Memory.Uint64(), ProcLimit: limits.Processes,
		CopyOutMax: uint64(limits.Output),
	}.Build()
	return cmd, nil
}

// RunFiles exposes extra files as /dev/fd/3 onward without copying them into tmpfs.
// The caller owns the returned output ID even when the command fails.
func (s *Sandbox) RunFiles(ctx context.Context, executable Executable, language string, input File, args []string, extras []File, limits FileRunLimits) (*FileRunResult, error) {
	cmd, err := s.fileCommand(executable, language, input, args, extras, limits)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Exec(ctx, pb.Request_builder{Cmd: []*pb.Request_CmdType{cmd}}.Build())
	if err != nil {
		return nil, fmt.Errorf("execute sandbox files: %w", err)
	}
	if len(resp.GetResults()) != 1 {
		return nil, fmt.Errorf("sandbox returned unexpected number of results")
	}
	r := resp.GetResults()[0]
	status := statusToString(r.GetStatus())
	if status == StatusOK && r.GetExitStatus() != 0 {
		status = StatusRE
	}
	return &FileRunResult{
		Status: status, ExitStatus: int(r.GetExitStatus()), Time: safeDuration(r.GetTime()), Memory: safeInt64(r.GetMemory()),
		OutputID: r.GetFileIDs()["stdout"], Stderr: r.GetFiles()["stderr"],
	}, nil
}

func (s *Sandbox) CheckFiles(ctx context.Context, executable Executable, language string, input, output, answer File, limits FileRunLimits) (*CheckerResult, error) {
	limits.Output = limits.Diagnostic
	cmd, err := s.fileCommand(executable, language, File{}, []string{"/dev/fd/3", "/dev/fd/4", "/dev/fd/5"}, []File{input, output, answer}, limits)
	if err != nil {
		return nil, err
	}
	cmd.SetCopyOutCached(nil)
	resp, err := s.client.Exec(ctx, pb.Request_builder{Cmd: []*pb.Request_CmdType{cmd}}.Build())
	if err != nil {
		return nil, fmt.Errorf("checker file execution: %w", err)
	}
	if len(resp.GetResults()) != 1 {
		return nil, fmt.Errorf("checker execution requires one result")
	}
	r := resp.GetResults()[0]
	status, score := parseTestlibVerdict(int(r.GetExitStatus()), string(r.GetFiles()["stdout"]), string(r.GetFiles()["stderr"]))
	if r.GetStatus() != pb.Response_Result_Accepted && r.GetStatus() != pb.Response_Result_NonZeroExitStatus {
		status = StatusFail
		score = nil
	}
	return &CheckerResult{Status: status, ExitStatus: int(r.GetExitStatus()), Message: string(r.GetFiles()["stderr"]), Score: score}, nil
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
	for _, cmd := range []*pb.Request_CmdType{sol, intr} {
		files := cmd.GetFiles()
		files[0] = nil
		files[1] = nil
		cmd.SetFiles(files)
		cmd.SetCopyOutCached(nil)
	}
	// An interactor that writes no output still yields an empty one.
	copyIn := intr.GetCopyIn()
	copyIn[interactorOutput] = newMemoryFile(nil)
	intr.SetCopyIn(copyIn)
	intr.SetCopyOutCached([]*pb.Request_CmdCopyOutFile{pb.Request_CmdCopyOutFile_builder{Name: interactorOutput}.Build()})
	pipes := []*pb.Request_PipeMap{
		pb.Request_PipeMap_builder{In: pb.Request_PipeMap_PipeIndex_builder{Index: 0, Fd: 1}.Build(), Out: pb.Request_PipeMap_PipeIndex_builder{Index: 1, Fd: 0}.Build(), Proxy: true, Max: uint64(solutionLimits.Output)}.Build(),   //nolint:gosec // limits are validated non-negative by the caller
		pb.Request_PipeMap_builder{In: pb.Request_PipeMap_PipeIndex_builder{Index: 1, Fd: 1}.Build(), Out: pb.Request_PipeMap_PipeIndex_builder{Index: 0, Fd: 0}.Build(), Proxy: true, Max: uint64(interactorLimits.Output)}.Build(), //nolint:gosec // limits are validated non-negative by the caller
	}
	resp, err := s.client.Exec(ctx, pb.Request_builder{Cmd: []*pb.Request_CmdType{sol, intr}, PipeMapping: pipes}.Build())
	if err != nil {
		return nil, fmt.Errorf("interactive file execution: %w", err)
	}
	if len(resp.GetResults()) != 2 {
		return nil, fmt.Errorf("interactive execution requires two results")
	}
	a, b := resp.GetResults()[0], resp.GetResults()[1]
	outputID := b.GetFileIDs()[interactorOutput]
	status, score := parseTestlibVerdict(int(b.GetExitStatus()), "", string(b.GetFiles()["stderr"]))
	if b.GetStatus() != pb.Response_Result_Accepted && b.GetStatus() != pb.Response_Result_NonZeroExitStatus {
		status = StatusFail
		score = nil
	}
	if (status == StatusOK || status == StatusPoints) && a.GetStatus() != pb.Response_Result_Accepted {
		status = statusToString(a.GetStatus())
		score = nil
	}
	return &InteractiveResult{Status: status, Score: score, Message: string(b.GetFiles()["stderr"]), OutputID: outputID,
		SolutionResult:   RunResult{Status: statusToString(a.GetStatus()), ExitStatus: int(a.GetExitStatus()), Time: safeDuration(a.GetTime()), Memory: safeInt64(a.GetMemory()), Stderr: a.GetFiles()["stderr"]},
		InteractorResult: RunResult{Status: statusToString(b.GetStatus()), ExitStatus: int(b.GetExitStatus()), Time: safeDuration(b.GetTime()), Memory: safeInt64(b.GetMemory()), Stderr: b.GetFiles()["stderr"]}}, nil
}
