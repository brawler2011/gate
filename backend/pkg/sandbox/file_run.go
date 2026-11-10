package sandbox

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	pb "github.com/criyle/go-judge/pb"
)

// FileRunLimits apply to one command. Large streams go to the disk file store;
// only bounded diagnostics are returned in the unary Exec response.
type FileRunLimits struct {
	CPU        time.Duration `yaml:"cpu" json:"cpu"`
	Wall       time.Duration `yaml:"wall" json:"wall"`
	Memory     uint64        `yaml:"memory" json:"memory"`
	Processes  uint64        `yaml:"processes" json:"processes"`
	Output     int64         `yaml:"output" json:"output"`
	Diagnostic int64         `yaml:"diagnostic" json:"diagnostic"`
}

type FileRunResult struct {
	Status     Status
	ExitStatus int
	Time       time.Duration
	Memory     int64
	OutputID   string
	Stderr     []byte
}

func (s *Sandbox) fileCommand(executable Executable, language, inputID string, args, extraIDs []string, limits FileRunLimits) (*pb.Request_CmdType, error) {
	if limits.CPU <= 0 || limits.Wall <= 0 || limits.Memory == 0 || limits.Processes == 0 || limits.Output <= 0 || limits.Diagnostic <= 0 || limits.Diagnostic > 64*1024 {
		return nil, fmt.Errorf("invalid sandbox file execution limits")
	}
	lang, ok := s.config.Langs[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
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
	if inputID != "" {
		if !validFileID(inputID) {
			return nil, fmt.Errorf("invalid input file ID")
		}
		stdin = newCachedFile(inputID)
	}
	// A pipe keeps disk page-cache charges for large outputs in the trusted
	// transport, rather than charging them as contestant process memory.
	stdout := pb.Request_File_builder{Pipe: pb.Request_PipeCollector_builder{Name: "stdout", Max: limits.Output, Pipe: true}.Build()}.Build()
	files := []*pb.Request_File{stdin, stdout, newPipeCollector("stderr", limits.Diagnostic)}
	for _, id := range extraIDs {
		if !validFileID(id) {
			return nil, fmt.Errorf("invalid extra file ID")
		}
		files = append(files, newCachedFile(id))
	}
	argv := strings.Fields(strings.NewReplacer("${name}", name, "${dir}", ".").Replace(lang.Execute))
	argv = append(argv, args...)
	cmd := pb.Request_CmdType_builder{
		Args: argv, Env: []string{"PATH=/usr/local/go/bin:/usr/bin:/bin"}, Files: files, CopyIn: copyIn,
		CopyOutCached: []*pb.Request_CmdCopyOutFile{pb.Request_CmdCopyOutFile_builder{Name: "stdout"}.Build()},
		CpuTimeLimit:  uint64(limits.CPU), ClockTimeLimit: uint64(limits.Wall), MemoryLimit: limits.Memory, ProcLimit: limits.Processes,
		CopyOutMax: uint64(limits.Output),
	}.Build()
	return cmd, nil
}

// RunFiles passes input and extra files as file descriptors, avoiding copies of
// large test bodies into the sandbox's tmpfs. argv may refer to /dev/fd/3 etc.
// The returned output ID belongs to the caller, including on a failed command.
func (s *Sandbox) RunFiles(ctx context.Context, executable Executable, language, inputID string, args, extraIDs []string, limits FileRunLimits) (*FileRunResult, error) {
	cmd, err := s.fileCommand(executable, language, inputID, args, extraIDs, limits)
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

// CheckFiles runs testlib against regular descriptors; large inputs and answers
// never need to be copied into the sandbox working directory.
func (s *Sandbox) CheckFiles(ctx context.Context, executable Executable, language, input, output, answer string, limits FileRunLimits) (*CheckerResult, error) {
	limits.Output = limits.Diagnostic
	cmd, err := s.fileCommand(executable, language, "", []string{"/dev/fd/3", "/dev/fd/4", "/dev/fd/5"}, []string{input, output, answer}, limits)
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

func (s *Sandbox) InteractFiles(ctx context.Context, solution Executable, solutionLanguage string, interactor Executable, interactorLanguage, inputID string, solutionLimits, interactorLimits FileRunLimits) (*InteractiveResult, error) {
	sol, err := s.fileCommand(solution, solutionLanguage, "", nil, nil, solutionLimits)
	if err != nil {
		return nil, err
	}
	intr, err := s.fileCommand(interactor, interactorLanguage, "", []string{"/dev/fd/3", "log.txt"}, []string{inputID}, interactorLimits)
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
	intr.SetCopyOut([]*pb.Request_CmdCopyOutFile{pb.Request_CmdCopyOutFile_builder{Name: "log.txt", Optional: true}.Build()})
	intr.SetCopyOutMax(uint64(interactorLimits.Diagnostic)) //nolint:gosec // limits are validated non-negative by the caller
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
	status, score := parseTestlibVerdict(int(b.GetExitStatus()), string(b.GetFiles()["stderr"]), string(b.GetFiles()["log.txt"]))
	if b.GetStatus() != pb.Response_Result_Accepted && b.GetStatus() != pb.Response_Result_NonZeroExitStatus {
		status = StatusFail
		score = nil
	}
	if (status == StatusOK || status == StatusPoints) && a.GetStatus() != pb.Response_Result_Accepted {
		status = statusToString(a.GetStatus())
		score = nil
	}
	return &InteractiveResult{Status: status, Score: score, Message: string(b.GetFiles()["stderr"]),
		SolutionResult:   RunResult{Status: statusToString(a.GetStatus()), ExitStatus: int(a.GetExitStatus()), Time: safeDuration(a.GetTime()), Memory: safeInt64(a.GetMemory()), Stderr: a.GetFiles()["stderr"]},
		InteractorResult: RunResult{Status: statusToString(b.GetStatus()), ExitStatus: int(b.GetExitStatus()), Time: safeDuration(b.GetTime()), Memory: safeInt64(b.GetMemory()), Stderr: b.GetFiles()["stderr"]}}, nil
}
