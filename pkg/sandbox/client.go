package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pb "github.com/criyle/go-judge/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the main client for interacting with go-judge
type Client struct {
	httpClient *http.Client
	grpcClient pb.ExecutorClient
	grpcConn   *grpc.ClientConn
	baseURL    string
	protocol   Protocol
}

// NewClient creates a new sandbox client
func NewClient(config ClientConfig) (*Client, error) {
	client := &Client{
		baseURL:  config.BaseURL,
		protocol: config.Protocol,
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	switch config.Protocol {
	case ProtocolHTTP:
		client.httpClient = &http.Client{
			Timeout: timeout,
		}
	case ProtocolGRPC:
		conn, err := grpc.NewClient(config.BaseURL, 
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024)), // 100MB
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
		}
		client.grpcClient = pb.NewExecutorClient(conn)
		client.grpcConn = conn
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}

	return client, nil
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
	if c.protocol == ProtocolHTTP {
		return c.compileHTTP(ctx, req)
	}
	return c.compileGRPC(ctx, req)
}

// Execute executes a compiled binary
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	if c.protocol == ProtocolHTTP {
		return c.executeHTTP(ctx, req)
	}
	return c.executeGRPC(ctx, req)
}

// compileHTTP compiles using HTTP API
func (c *Client) compileHTTP(ctx context.Context, req CompileRequest) (*CompileResult, error) {
	langConfig, ok := GetLanguageConfig(NormalizeLanguageName(req.Language))
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	if !langConfig.NeedsCompilation {
		// For interpreted languages, just return the source as-is
		return &CompileResult{
			Success: true,
			FileID:  "", // No file ID for interpreted languages
		}, nil
	}

	// Build compilation command
	compileCmd := buildCommand(langConfig.CompileCommand, map[string]string{
		"{source}": req.SourceFile,
		"{output}": req.OutputFile,
	})

	// Prepare CopyIn files
	copyIn := make(map[string]interface{})
	copyIn[req.SourceFile] = map[string]string{"content": req.SourceCode}

	// Add dependencies
	for filename, content := range req.Dependencies {
		copyIn[filename] = map[string]string{"content": content}
	}

	// Build HTTP request
	httpReq := map[string]interface{}{
		"cmd": []map[string]interface{}{
			{
				"args":        compileCmd,
				"env":         langConfig.CompilerEnv,
				"files":       []interface{}{map[string]string{}, map[string]string{}, map[string]string{}}, // stdin, stdout, stderr
				"cpuLimit":    req.Limits.ToNanoseconds(),
				"memoryLimit": req.Limits.ToBytes(),
				"procLimit":   req.Limits.ProcLimit,
				"copyIn":      copyIn,
				"copyOut":     []string{req.OutputFile},
				"copyOutCached": []string{req.OutputFile},
			},
		},
	}

	body, err := json.Marshal(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("go-judge returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results returned from go-judge")
	}

	result := results[0]
	compileResult := &CompileResult{
		Success:    result["status"].(string) == "Accepted",
		ExitStatus: int(result["exitStatus"].(float64)),
		Time:       int64(result["time"].(float64)),
		Memory:     int64(result["memory"].(float64)),
	}

	// Extract stdout/stderr
	if files, ok := result["files"].(map[string]interface{}); ok {
		if stdout, ok := files["stdout"].(string); ok {
			compileResult.Stdout = stdout
		}
		if stderr, ok := files["stderr"].(string); ok {
			compileResult.Stderr = stderr
		}
	}

	// Extract FileID from fileIds
	if fileIds, ok := result["fileIds"].(map[string]interface{}); ok {
		if fileId, ok := fileIds[req.OutputFile].(string); ok {
			compileResult.FileID = fileId
		}
	}

	return compileResult, nil
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
		Args:  compileCmd,
		Env:   langConfig.CompilerEnv,
		Files: []*pb.Request_File{
			newMemoryFile([]byte("")), // stdin
			newPipeFile("stdout"),      // stdout
			newPipeFile("stderr"),      // stderr
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

// executeHTTP executes using HTTP API
func (c *Client) executeHTTP(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	// Prepare CopyIn
	copyIn := make(map[string]interface{})
	if req.BinaryFileID != "" {
		// Use cached file from compilation
		copyIn[strings.TrimPrefix(req.ExecutableName, "./")] = map[string]string{"src": req.BinaryFileID}
	}

	// Add additional input files
	for filename, content := range req.Files {
		copyIn[filename] = map[string]string{"content": string(content)}
	}

	// Prepare stdin
	stdinFile := map[string]string{"content": string(req.Stdin)}

	// Build HTTP request
	httpReq := map[string]interface{}{
		"cmd": []map[string]interface{}{
			{
				"args":        append([]string{req.ExecutableName}, req.Args...),
				"env":         []string{"PATH=/usr/bin:/bin"},
				"files":       []interface{}{stdinFile, map[string]string{}, map[string]string{}}, // stdin, stdout, stderr
				"cpuLimit":    req.Limits.ToNanoseconds(),
				"memoryLimit": req.Limits.ToBytes(),
				"procLimit":   req.Limits.ProcLimit,
				"copyIn":      copyIn,
			},
		},
	}

	body, err := json.Marshal(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("go-judge returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results returned from go-judge")
	}

	result := results[0]
	execResult := &ExecuteResult{
		Status:     result["status"].(string),
		ExitStatus: int(result["exitStatus"].(float64)),
		Time:       int64(result["time"].(float64)),
		Memory:     int64(result["memory"].(float64)),
	}

	// Extract stdout/stderr
	if files, ok := result["files"].(map[string]interface{}); ok {
		if stdout, ok := files["stdout"].(string); ok {
			execResult.Stdout = []byte(stdout)
		}
		if stderr, ok := files["stderr"].(string); ok {
			execResult.Stderr = []byte(stderr)
		}
	}

	return execResult, nil
}

// executeGRPC executes using gRPC API
func (c *Client) executeGRPC(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
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
