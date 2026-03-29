package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	pb "github.com/criyle/go-judge/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the main client for interacting with go-judge
type Client struct {
	grpcClient pb.ExecutorClient
	grpcConn   *grpc.ClientConn
}

// NewClient creates a new sandbox client
func NewClient(config ClientConfig) (*Client, error) {
	conn, err := grpc.NewClient(config.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024)), // 100MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	return &Client{
		grpcClient: pb.NewExecutorClient(conn),
		grpcConn:   conn,
	}, nil
}

// Close closes the client connections
func (c *Client) Close() error {
	if c.grpcConn != nil {
		return c.grpcConn.Close()
	}
	return nil
}

// Compile compiles source code and returns the compiled binary
func (c *Client) Compile(ctx context.Context, req CompileRequest) (*CompileResult, error) {
	return c.compileGRPC(ctx, req)
}

// Execute executes a compiled binary
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	return c.executeGRPC(ctx, req)
}

// compileGRPC compiles using gRPC API
func (c *Client) compileGRPC(ctx context.Context, req CompileRequest) (*CompileResult, error) {
	langConfig, ok := GetLanguageConfig(NormalizeLanguageName(req.Language))
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	if !langConfig.NeedsCompilation {
		return &CompileResult{
			Success: true,
			FileID:  "",
		}, nil
	}

	// Build compilation command
	compileCmd := buildCommand(langConfig.CompileCommand, map[string]string{
		"{source}": req.SourceFile,
		"{output}": req.OutputFile,
	})

	// Prepare CopyIn files
	copyIn := make(map[string]*pb.Request_File)
	copyIn[req.SourceFile] = newMemoryFile([]byte(req.SourceCode))

	// Add dependencies
	for filename, content := range req.Dependencies {
		copyIn[filename] = newMemoryFile([]byte(content))
	}

	// Build gRPC request
	cmd := pb.Request_CmdType_builder{
		Args: compileCmd,
		Env:  langConfig.CompilerEnv,
		Files: []*pb.Request_File{
			newMemoryFile([]byte("")), // stdin
			newPipeFile("stdout"),     // stdout
			newPipeFile("stderr"),     // stderr
		},
		CpuTimeLimit:   uint64(max(0, req.Limits.ToNanoseconds())),
		ClockTimeLimit: uint64(max(0, req.Limits.ToNanoseconds()*2)), // clock limit = 2x CPU limit
		MemoryLimit:    uint64(req.Limits.ToBytes()),
		ProcLimit:      uint64(req.Limits.ProcLimit),
		CopyIn:         copyIn,
		CopyOutCached: []*pb.Request_CmdCopyOutFile{
			pb.Request_CmdCopyOutFile_builder{
				Name: req.OutputFile,
			}.Build(),
		},
	}.Build()

	grpcReq := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	if c.grpcClient == nil {
		return nil, fmt.Errorf("gRPC client is not initialized")
	}

	grpcResp, err := c.grpcClient.Exec(ctx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("gRPC exec failed: %w", err)
	}

	if len(grpcResp.GetResults()) == 0 {
		return nil, fmt.Errorf("no results returned from go-judge")
	}

	result := grpcResp.GetResults()[0]

	// Safely convert uint64 to int64
	timeVal := result.GetTime()
	if timeVal > uint64(1<<63-1) {
		timeVal = uint64(1<<63 - 1)
	}
	memVal := result.GetMemory()
	if memVal > uint64(1<<63-1) {
		memVal = uint64(1<<63 - 1)
	}

	compileResult := &CompileResult{
		Success:    result.GetStatus() == pb.Response_Result_Accepted,
		ExitStatus: int(result.GetExitStatus()),
		Time:       int64(timeVal),
		Memory:     int64(memVal),
		Stdout:     string(result.GetFiles()["stdout"]),
		Stderr:     string(result.GetFiles()["stderr"]),
	}

	// Extract FileID
	if fileId, ok := result.GetFileIDs()[req.OutputFile]; ok {
		compileResult.FileID = fileId
	}

	return compileResult, nil
}

// executeGRPC executes using gRPC API
func (c *Client) executeGRPC(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	if c.grpcClient == nil {
		return nil, fmt.Errorf("gRPC client is not initialized")
	}

	// Prepare CopyIn
	copyIn := make(map[string]*pb.Request_File)
	if req.BinaryFileID != "" {
		copyIn[strings.TrimPrefix(req.ExecutableName, "./")] = newCachedFile(req.BinaryFileID)
	}

	// Add additional input files
	for filename, content := range req.Files {
		copyIn[filename] = newMemoryFile(content)
	}

	// Build gRPC request
	cmd := pb.Request_CmdType_builder{
		Args: append([]string{req.ExecutableName}, req.Args...),
		Env:  []string{"PATH=/usr/bin:/bin"},
		Files: []*pb.Request_File{
			newMemoryFile(req.Stdin), // stdin
			newPipeFile("stdout"),    // stdout
			newPipeFile("stderr"),    // stderr
		},
		CpuTimeLimit:   uint64(max(0, req.Limits.ToNanoseconds())),
		ClockTimeLimit: uint64(max(0, req.Limits.ToNanoseconds()*2)),
		MemoryLimit:    uint64(req.Limits.ToBytes()),
		ProcLimit:      uint64(req.Limits.ProcLimit),
		CopyIn:         copyIn,
	}.Build()

	grpcReq := pb.Request_builder{
		Cmd: []*pb.Request_CmdType{cmd},
	}.Build()

	grpcResp, err := c.grpcClient.Exec(ctx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("gRPC exec failed: %w", err)
	}

	if len(grpcResp.GetResults()) == 0 {
		return nil, fmt.Errorf("no results returned from go-judge")
	}

	result := grpcResp.GetResults()[0]

	// Safely convert uint64 to int64
	timeVal := result.GetTime()
	if timeVal > uint64(1<<63-1) {
		timeVal = uint64(1<<63 - 1)
	}
	memVal := result.GetMemory()
	if memVal > uint64(1<<63-1) {
		memVal = uint64(1<<63 - 1)
	}

	execResult := &ExecuteResult{
		Status:     statusToString(result.GetStatus()),
		ExitStatus: int(result.GetExitStatus()),
		Time:       int64(timeVal),
		Memory:     int64(memVal),
		Stdout:     result.GetFiles()["stdout"],
		Stderr:     result.GetFiles()["stderr"],
	}

	return execResult, nil
}

// Helper functions for gRPC

func newMemoryFile(content []byte) *pb.Request_File {
	memory := pb.Request_MemoryFile_builder{
		Content: content,
	}.Build()
	return pb.Request_File_builder{
		Memory: memory,
	}.Build()
}

func newCachedFile(fileID string) *pb.Request_File {
	cached := pb.Request_CachedFile_builder{
		FileID: fileID,
	}.Build()
	return pb.Request_File_builder{
		Cached: cached,
	}.Build()
}

func newPipeFile(name string) *pb.Request_File {
	return pb.Request_File_builder{
		Pipe: pb.Request_PipeCollector_builder{
			Name: name,
			Max:  10485760, // 10MB
		}.Build(),
	}.Build()
}

func statusToString(status pb.Response_Result_StatusType) string {
	switch status {
	case pb.Response_Result_Accepted:
		return "Accepted"
	case pb.Response_Result_MemoryLimitExceeded:
		return "Memory Limit Exceeded"
	case pb.Response_Result_TimeLimitExceeded:
		return "Time Limit Exceeded"
	case pb.Response_Result_OutputLimitExceeded:
		return "Output Limit Exceeded"
	case pb.Response_Result_FileError:
		return "File Error"
	case pb.Response_Result_NonZeroExitStatus:
		return "Nonzero Exit Status"
	case pb.Response_Result_Signalled:
		return "Runtime Error"
	case pb.Response_Result_DangerousSyscall:
		return "Dangerous Syscall"
	case pb.Response_Result_InternalError:
		return "Internal Error"
	default:
		return "Runtime Error" // Default for unknown statuses
	}
}

// buildCommand replaces placeholders in command template
func buildCommand(template []string, replacements map[string]string) []string {
	result := make([]string, len(template))
	for i, part := range template {
		result[i] = part
		for placeholder, value := range replacements {
			result[i] = strings.ReplaceAll(result[i], placeholder, value)
		}
	}
	return result
}

// ComputeSHA256 computes SHA256 hash of binary content
func ComputeSHA256(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
