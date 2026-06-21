package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/criyle/go-judge/pb"
)

// Status represents the result status of a run or evaluation.
type Status string

const (
	// Default size for pipe collectors (10 MB)
	defaultPipeCollectorSize = 10 * 1024 * 1024
	// Small size for dummy command pipe collectors (1 KB)
	dummyPipeCollectorSize = 1024

	// Compile limits for interpreters (dummy run)
	interpreterCompileCpuTimeLimit = 5 * time.Second
	interpreterCompileMemoryLimit  = 256 * 1024 * 1024
	interpreterCompileProcLimit    = 150

	// Compile limits for compilers
	compilerCompileCpuTimeLimit = 10 * time.Second
	compilerCompileMemoryLimit  = 512 * 1024 * 1024
	compilerCompileProcLimit    = 150

	// Generator execution limits
	generatorCpuTimeLimit = 5 * time.Second
	generatorMemoryLimit  = 256 * 1024 * 1024
	generatorProcLimit    = 50

	// Checker execution limits
	checkerCpuTimeLimit = 5 * time.Second
	checkerMemoryLimit  = 512 * 1024 * 1024
	checkerProcLimit    = 50

	// Interactor execution limits
	interactorCpuTimeLimit   = 5 * time.Second
	interactorClockTimeLimit = 10 * time.Second
	interactorMemoryLimit    = 512 * 1024 * 1024
	interactorProcLimit      = 50

	// Solution execution limits defaults
	solutionProcLimit           = 50
	solutionClockTimeMultiplier = 2
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

// RunResult represents the result of executing a program.
type RunResult struct {
	Status     Status
	ExitStatus int
	Time       time.Duration
	Memory     int64 // in bytes
	Stdout     []byte
	Stderr     []byte
}

// CheckerResult represents the evaluation of a solution output by a checker.
type CheckerResult struct {
	Status     Status
	ExitStatus int
	Message    string
	Score      *float64
}

// InteractiveResult represents the outcome of an interactive solution execution.
type InteractiveResult struct {
	Status           Status
	SolutionResult   RunResult
	InteractorResult RunResult
	Message          string
	Score            *float64
}

// Executable represents a compiled program or cached script with its dependencies in the sandbox.
type Executable struct {
	PrimaryFileID string            // ID основного исполняемого файла или скрипта
	Dependencies  map[string]string // Карта зависимостей: "имя_файла -> fileID"
}

// Sandbox wraps the Client and Config for easy execution.
type Sandbox struct {
	client *Client
	config *Config
}

// NewSandbox creates a high-level wrapper.
func NewSandbox(client *Client, config *Config) *Sandbox {
	return &Sandbox{
		client: client,
		config: config,
	}
}

// Compile compiles the source code using the configured compiler and returns the Executable descriptor.
// For interpreted languages, it caches the source code and all its dependencies.
func (s *Sandbox) Compile(ctx context.Context, sourceCode []byte, langKey string, dependencies map[string][]byte) (Executable, error) {
	lang, ok := s.config.Langs[langKey]
	if !ok {
		return Executable{}, fmt.Errorf("unsupported language: %s", langKey)
	}

	// NOTE: можно как-то общий код чуть повыносить, этот иф по факту просто код дублирует
	if lang.Type == "interpreter" || lang.Compile == "" {
		copyIn := make(map[string]*pb.Request_File)
		copyIn[lang.CodeFile] = newMemoryFile(sourceCode)
		for depName, depContent := range dependencies {
			copyIn[depName] = newMemoryFile(depContent)
		}

		copyOutCached := []*pb.Request_CmdCopyOutFile{
			pb.Request_CmdCopyOutFile_builder{Name: lang.CodeFile}.Build(),
		}
		for depName := range dependencies {
			copyOutCached = append(copyOutCached, pb.Request_CmdCopyOutFile_builder{Name: depName}.Build())
		}

		// Cache the script directly using a dummy command
		cmd := pb.Request_CmdType_builder{
			Args: []string{"/bin/true"},
			Files: []*pb.Request_File{
				newMemoryFile([]byte("")),
				newPipeCollector("stdout", dummyPipeCollectorSize),
				newPipeCollector("stderr", dummyPipeCollectorSize),
			},
			CopyIn:        copyIn,
			CopyOutCached: copyOutCached,
			CpuTimeLimit:  uint64(interpreterCompileCpuTimeLimit.Nanoseconds()),
			MemoryLimit:   interpreterCompileMemoryLimit,
			ProcLimit:     interpreterCompileProcLimit,
			Env:           []string{"PATH=/usr/local/go/bin:/usr/bin:/bin", "GOCACHE=/tmp", "HOME=/tmp"},
		}.Build()

		req := pb.Request_builder{
			Cmd: []*pb.Request_CmdType{cmd},
		}.Build()

		resp, err := s.client.Exec(ctx, req)
		if err != nil {
			return Executable{}, fmt.Errorf("failed to cache script: %w", err)
		}
		if len(resp.GetResults()) == 0 {
			return Executable{}, fmt.Errorf("no results returned during caching")
		}
		res := resp.GetResults()[0]
		if res.GetStatus() != pb.Response_Result_Accepted {
			return Executable{}, fmt.Errorf("caching dummy execution failed: status=%v, exitStatus=%v, stderr=%s",
				res.GetStatus(), res.GetExitStatus(), string(res.GetFiles()["stderr"]))
		}

		fileID, ok := res.GetFileIDs()[lang.CodeFile]
		if !ok {
			return Executable{}, fmt.Errorf("fileID not found for cached script %s", lang.CodeFile)
		}

		cachedDeps := make(map[string]string)
		for depName := range dependencies {
			depFileID, ok := res.GetFileIDs()[depName]
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

	// Prepare compile command args
	// e.g. /usr/bin/g++ -O2 -Wall -std=c++17 -o ${name} foo.cc
	compileCmd := strings.ReplaceAll(lang.Compile, "${name}", outputName)
	compileCmdArgs := strings.Fields(compileCmd)

	copyIn := make(map[string]*pb.Request_File)
	copyIn[lang.CodeFile] = newMemoryFile(sourceCode)
	for depName, depContent := range dependencies {
		copyIn[depName] = newMemoryFile(depContent)
	}

	cmd := pb.Request_CmdType_builder{
		Args: compileCmdArgs,
		Files: []*pb.Request_File{
			newMemoryFile([]byte("")),
			newPipeCollector("stdout", defaultPipeCollectorSize),
			newPipeCollector("stderr", defaultPipeCollectorSize),
		},
		CopyIn: copyIn,
		CopyOutCached: []*pb.Request_CmdCopyOutFile{
			pb.Request_CmdCopyOutFile_builder{Name: outputName}.Build(),
		},
		CpuTimeLimit: uint64(compilerCompileCpuTimeLimit.Nanoseconds()),
		MemoryLimit:  compilerCompileMemoryLimit,
		ProcLimit:    compilerCompileProcLimit,
		Env:          []string{"PATH=/usr/local/go/bin:/usr/bin:/bin", "GOCACHE=/tmp", "HOME=/tmp"},
	}.Build()

	req := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	resp, err := s.client.Exec(ctx, req)
	if err != nil {
		return Executable{}, fmt.Errorf("compilation failed to dispatch: %w", err)
	}
	if len(resp.GetResults()) == 0 {
		return Executable{}, fmt.Errorf("compilation returned no results")
	}

	res := resp.GetResults()[0]
	if res.GetStatus() != pb.Response_Result_Accepted {
		return Executable{}, fmt.Errorf("compilation failed:\nstatus: %v\nexit code: %d\nstdout: %s\nstderr: %s",
			res.GetStatus(), res.GetExitStatus(), string(res.GetFiles()["stdout"]), string(res.GetFiles()["stderr"]))
	}

	fileID, ok := res.GetFileIDs()[outputName]
	if !ok {
		return Executable{}, fmt.Errorf("fileID not found in compilation results for %s", outputName)
	}
	return Executable{
		PrimaryFileID: fileID,
		Dependencies:  nil,
	}, nil
}

// Generate executes the compiled generator with the given arguments and returns stdout.
func (s *Sandbox) Generate(ctx context.Context, gen Executable, args []string) ([]byte, error) {
	copyIn := map[string]*pb.Request_File{
		"generator": newCachedFile(gen.PrimaryFileID),
	}
	for depName, depFileID := range gen.Dependencies {
		copyIn[depName] = newCachedFile(depFileID)
	}

	cmd := pb.Request_CmdType_builder{
		Args: append([]string{"./generator"}, args...),
		Files: []*pb.Request_File{
			newMemoryFile([]byte("")),
			newPipeCollector("stdout", defaultPipeCollectorSize),
			newPipeCollector("stderr", defaultPipeCollectorSize),
		},
		CopyIn:       copyIn,
		CpuTimeLimit: uint64(generatorCpuTimeLimit.Nanoseconds()),
		MemoryLimit:  generatorMemoryLimit,
		ProcLimit:    generatorProcLimit,
		Env:          []string{"PATH=/usr/bin:/bin"},
	}.Build()

	req := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	resp, err := s.client.Exec(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("generator failed to dispatch: %w", err)
	}
	if len(resp.GetResults()) == 0 {
		return nil, fmt.Errorf("generator returned no results")
	}

	res := resp.GetResults()[0]
	if res.GetStatus() != pb.Response_Result_Accepted {
		return nil, fmt.Errorf("generator execution failed: status=%v, exitStatus=%v, stderr=%s",
			res.GetStatus(), res.GetExitStatus(), string(res.GetFiles()["stderr"]))
	}

	return res.GetFiles()["stdout"], nil
}

// Test executes the solution with the given input and limits, returning the result.
func (s *Sandbox) Test(ctx context.Context, sol Executable, langKey string, input []byte, timeLimitMs, memoryLimitMb int) (*RunResult, error) {
	lang, ok := s.config.Langs[langKey]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", langKey)
	}

	var cmdArgs []string
	copyIn := make(map[string]*pb.Request_File)

	if lang.Type == "compiler" {
		outputName := lang.CompileFile
		if outputName == "" {
			outputName = "solution"
		}
		copyIn[outputName] = newCachedFile(sol.PrimaryFileID)
		execTemplate := strings.ReplaceAll(lang.Execute, "${name}", outputName)
		execTemplate = strings.ReplaceAll(execTemplate, "${dir}", ".")
		cmdArgs = strings.Fields(execTemplate)
	} else {
		copyIn[lang.CodeFile] = newCachedFile(sol.PrimaryFileID)
		execTemplate := strings.ReplaceAll(lang.Execute, "${dir}", ".")
		cmdArgs = strings.Fields(execTemplate)
	}

	for depName, depFileID := range sol.Dependencies {
		copyIn[depName] = newCachedFile(depFileID)
	}

	cmd := pb.Request_CmdType_builder{
		Args: cmdArgs,
		Files: []*pb.Request_File{
			newMemoryFile(input),
			newPipeCollector("stdout", defaultPipeCollectorSize),
			newPipeCollector("stderr", defaultPipeCollectorSize),
		},
		CopyIn:         copyIn,
		CpuTimeLimit:   uint64(timeLimitMs) * uint64(time.Millisecond),
		ClockTimeLimit: uint64(timeLimitMs) * uint64(time.Millisecond) * solutionClockTimeMultiplier,
		MemoryLimit:    uint64(memoryLimitMb) * 1024 * 1024,
		ProcLimit:      solutionProcLimit,
		Env:            []string{"PATH=/usr/bin:/bin"},
	}.Build()

	req := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	resp, err := s.client.Exec(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("solution failed to dispatch: %w", err)
	}
	if len(resp.GetResults()) == 0 {
		return nil, fmt.Errorf("solution returned no results")
	}

	res := resp.GetResults()[0]
	statusStr := statusToString(res.GetStatus())

	return &RunResult{
		Status:     statusStr,
		ExitStatus: int(res.GetExitStatus()),
		Time:       time.Duration(res.GetTime()),
		Memory:     int64(res.GetMemory()),
		Stdout:     res.GetFiles()["stdout"],
		Stderr:     res.GetFiles()["stderr"],
	}, nil
}

// Check runs a checker program with args input.txt output.txt answer.txt and returns checker verdict/message.
func (s *Sandbox) Check(ctx context.Context, checker Executable, input, output, answer []byte) (*CheckerResult, error) {
	copyIn := map[string]*pb.Request_File{
		"checker":    newCachedFile(checker.PrimaryFileID),
		"input.txt":  newMemoryFile(input),
		"output.txt": newMemoryFile(output),
		"answer.txt": newMemoryFile(answer),
	}
	for depName, depFileID := range checker.Dependencies {
		copyIn[depName] = newCachedFile(depFileID)
	}

	cmd := pb.Request_CmdType_builder{
		Args: []string{"./checker", "input.txt", "output.txt", "answer.txt"},
		Files: []*pb.Request_File{
			newMemoryFile([]byte("")),
			newPipeCollector("stdout", defaultPipeCollectorSize),
			newPipeCollector("stderr", defaultPipeCollectorSize),
		},
		CopyIn:       copyIn,
		CpuTimeLimit: uint64(checkerCpuTimeLimit.Nanoseconds()),
		MemoryLimit:  checkerMemoryLimit,
		ProcLimit:    checkerProcLimit,
		Env:          []string{"PATH=/usr/bin:/bin"},
	}.Build()

	req := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	resp, err := s.client.Exec(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("checker failed to dispatch: %w", err)
	}
	if len(resp.GetResults()) == 0 {
		return nil, fmt.Errorf("checker returned no results")
	}

	res := resp.GetResults()[0]
	stderr := strings.TrimSpace(string(res.GetFiles()["stderr"]))
	stdout := strings.TrimSpace(string(res.GetFiles()["stdout"]))

	status, score := parseTestlibVerdict(int(res.GetExitStatus()), stdout, stderr)

	return &CheckerResult{
		Status:     status,
		ExitStatus: int(res.GetExitStatus()),
		Message:    stderr,
		Score:      score,
	}, nil
}

// Interact runs the solution and interactor together, connected via bidirectional pipes.
func (s *Sandbox) Interact(ctx context.Context, sol Executable, solLangKey string, interactor Executable, input []byte, timeLimitMs, memoryLimitMb int) (*InteractiveResult, error) {
	lang, ok := s.config.Langs[solLangKey]
	if !ok {
		return nil, fmt.Errorf("unsupported solution language: %s", solLangKey)
	}

	var solCmdArgs []string
	solCopyIn := make(map[string]*pb.Request_File)

	if lang.Type == "compiler" {
		outputName := lang.CompileFile
		if outputName == "" {
			outputName = "solution"
		}
		solCopyIn[outputName] = newCachedFile(sol.PrimaryFileID)
		execTemplate := strings.ReplaceAll(lang.Execute, "${name}", outputName)
		execTemplate = strings.ReplaceAll(execTemplate, "${dir}", ".")
		solCmdArgs = strings.Fields(execTemplate)
	} else {
		solCopyIn[lang.CodeFile] = newCachedFile(sol.PrimaryFileID)
		execTemplate := strings.ReplaceAll(lang.Execute, "${dir}", ".")
		solCmdArgs = strings.Fields(execTemplate)
	}

	for depName, depFileID := range sol.Dependencies {
		solCopyIn[depName] = newCachedFile(depFileID)
	}

	solCmd := pb.Request_CmdType_builder{
		Args: solCmdArgs,
		Files: []*pb.Request_File{
			nil, // pipe stdin
			nil, // pipe stdout
			newPipeCollector("stdout", defaultPipeCollectorSize), // redirect stderr to stdout collector key
		},
		CopyIn:         solCopyIn,
		CpuTimeLimit:   uint64(timeLimitMs) * uint64(time.Millisecond),
		ClockTimeLimit: uint64(timeLimitMs) * uint64(time.Millisecond) * solutionClockTimeMultiplier,
		MemoryLimit:    uint64(memoryLimitMb) * 1024 * 1024,
		ProcLimit:      solutionProcLimit,
		Env:            []string{"PATH=/usr/bin:/bin"},
	}.Build()

	interactorCopyIn := map[string]*pb.Request_File{
		"interactor": newCachedFile(interactor.PrimaryFileID),
		"input.txt":  newMemoryFile(input),
		"answer.txt": newMemoryFile([]byte{}),
	}
	for depName, depFileID := range interactor.Dependencies {
		interactorCopyIn[depName] = newCachedFile(depFileID)
	}

	interactorCmd := pb.Request_CmdType_builder{
		Args: []string{"./interactor", "input.txt", "log.txt", "answer.txt"},
		Files: []*pb.Request_File{
			nil, // pipe stdin
			nil, // pipe stdout
			newPipeCollector("stdout", defaultPipeCollectorSize),
		},
		CopyIn:         interactorCopyIn,
		CpuTimeLimit:   uint64(interactorCpuTimeLimit.Nanoseconds()),
		ClockTimeLimit: uint64(interactorClockTimeLimit.Nanoseconds()),
		MemoryLimit:    interactorMemoryLimit,
		ProcLimit:      interactorProcLimit,
		Env:            []string{"PATH=/usr/bin:/bin"},
		CopyOut: []*pb.Request_CmdCopyOutFile{
			pb.Request_CmdCopyOutFile_builder{Name: "log.txt"}.Build(),
		},
	}.Build()

	pipeMappings := []*pb.Request_PipeMap{
		// Solution stdout (Command 0, FD 1) -> Interactor stdin (Command 1, FD 0)
		pb.Request_PipeMap_builder{
			In: pb.Request_PipeMap_PipeIndex_builder{
				Index: 0,
				Fd:    1,
			}.Build(),
			Out: pb.Request_PipeMap_PipeIndex_builder{
				Index: 1,
				Fd:    0,
			}.Build(),
			Max: defaultPipeCollectorSize,
		}.Build(),
		// Interactor stdout (Command 1, FD 1) -> Solution stdin (Command 0, FD 0)
		pb.Request_PipeMap_builder{
			In: pb.Request_PipeMap_PipeIndex_builder{
				Index: 1,
				Fd:    1,
			}.Build(),
			Out: pb.Request_PipeMap_PipeIndex_builder{
				Index: 0,
				Fd:    0,
			}.Build(),
			Max: defaultPipeCollectorSize,
		}.Build(),
	}

	req := pb.Request_builder{
		Cmd:         []*pb.Request_CmdType{solCmd, interactorCmd},
		PipeMapping: pipeMappings,
	}.Build()

	resp, err := s.client.Exec(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("interactive exec failed to dispatch: %w", err)
	}
	if len(resp.GetResults()) < 2 {
		return nil, fmt.Errorf("interactive exec expected at least 2 results, got %d", len(resp.GetResults()))
	}

	solRes := resp.GetResults()[0]
	intRes := resp.GetResults()[1]

	solRun := RunResult{
		Status:     statusToString(solRes.GetStatus()),
		ExitStatus: int(solRes.GetExitStatus()),
		Time:       time.Duration(solRes.GetTime()),
		Memory:     int64(solRes.GetMemory()),
		Stderr:     solRes.GetFiles()["stdout"], // stderr was redirected to stdout key
	}

	intRun := RunResult{
		Status:     statusToString(intRes.GetStatus()),
		ExitStatus: int(intRes.GetExitStatus()),
		Time:       time.Duration(intRes.GetTime()),
		Memory:     int64(intRes.GetMemory()),
		Stderr:     intRes.GetFiles()["stdout"],
	}

	// Interactor exit code determines correctness
	var status Status
	var score *float64
	if intRes.GetStatus() != pb.Response_Result_Accepted && intRes.GetStatus() != pb.Response_Result_NonZeroExitStatus {
		status = StatusFail
	} else {
		logContent := string(intRes.GetFiles()["log.txt"])
		stderrContent := string(intRes.GetFiles()["stdout"]) // interactor stderr is redirected to stdout key
		status, score = parseTestlibVerdict(int(intRes.GetExitStatus()), stderrContent, logContent)
	}

	// If interactor was OK/POINTS but solution timed out/crashed, solution status takes precedence
	if (status == StatusOK || status == StatusPoints) && solRes.GetStatus() != pb.Response_Result_Accepted {
		status = solRun.Status
	}

	return &InteractiveResult{
		Status:           status,
		SolutionResult:   solRun,
		InteractorResult: intRun,
		Message:          strings.TrimSpace(string(intRes.GetFiles()["stdout"])), // interactor writes feedback/errors to stderr
		Score:            score,
	}, nil
}

func statusToString(status pb.Response_Result_StatusType) Status {
	switch status {
	case pb.Response_Result_Accepted:
		return StatusOK
	case pb.Response_Result_MemoryLimitExceeded:
		return StatusMLE
	case pb.Response_Result_TimeLimitExceeded:
		return StatusTLE
	case pb.Response_Result_OutputLimitExceeded:
		return StatusOLE
	case pb.Response_Result_FileError:
		return StatusFail
	case pb.Response_Result_NonZeroExitStatus:
		return StatusRE
	case pb.Response_Result_Signalled:
		return StatusRE
	case pb.Response_Result_DangerousSyscall:
		return StatusFail
	case pb.Response_Result_InternalError:
		return StatusFail
	default:
		return StatusFail
	}
}
