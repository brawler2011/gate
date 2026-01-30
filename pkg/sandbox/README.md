# Sandbox Package

Go-Judge SDK for compiling and executing problem components in isolation.

## Features

- **Multi-language support**: C++, Python, Go, Java, C
- **Component compilation**: Checkers, validators, generators, interactors
- **Solution execution**: Compile and run user solutions
- **Resource limits**: CPU time, memory, process limits
- **Multiple protocols**: HTTP and gRPC
- **Orchestration**: High-level workflows for testing

## Quick Start

### Initialize Client

```go
import "gate/core/pkg/sandbox"

// HTTP client
client, err := sandbox.NewClient(sandbox.ClientConfig{
    Protocol: sandbox.ProtocolHTTP,
    BaseURL:  "http://localhost:5050",
    Timeout:  30 * time.Second,
})

// Or gRPC client
client, err := sandbox.NewClient(sandbox.ClientConfig{
    Protocol: sandbox.ProtocolGRPC,
    BaseURL:  "localhost:5051",
    Timeout:  30 * time.Second,
})
```

### Compile and Run Solution

```go
orchestrator := sandbox.NewOrchestrator(client)

result, err := orchestrator.JudgeSolution(ctx, sandbox.JudgeSolutionRequest{
    SolutionCode:     sourceCode,
    SolutionLanguage: "cpp17",
    CheckerFileID:    checkerBinary.FileID,
    Input:            testInput,
    Answer:           testAnswer,
    TimeLimitMs:      1000,
    MemoryLimitMB:    256,
})

fmt.Printf("Verdict: %s\n", result.Verdict)
fmt.Printf("Time: %dms\n", result.Time/1000000)
fmt.Printf("Memory: %dKB\n", result.Memory/1024)
```

### Compile Problem Components

```go
// Compile checker
checkerBinary, err := orchestrator.CompileComponentFromSource(ctx,
    "checker",
    checkerCode,
    "cpp17",
    dependencies, // map[string]string with testlib.h, etc.
)

// Compile validator
validatorBinary, err := orchestrator.CompileComponentFromSource(ctx,
    "validator",
    validatorCode,
    "cpp17",
    nil,
)

// Compile generator
generatorBinary, err := orchestrator.CompileComponentFromSource(ctx,
    "generator",
    generatorCode,
    "cpp17",
    nil,
)
```

### Generate Test Data

```go
generated, err := orchestrator.GenerateTest(ctx, sandbox.GenerateTestRequest{
    GeneratorFileID: generatorBinary.FileID,
    Arguments:       []string{"10", "100"},
    Seed:            42,
    Limits:          limits,
})

if generated.Success {
    fmt.Printf("Generated: %s\n", string(generated.Input))
}
```

### Validate Test Input

```go
validation, err := orchestrator.ValidateTest(ctx, sandbox.ValidateTestRequest{
    ValidatorFileID: validatorBinary.FileID,
    Input:           testInput,
    Limits:          limits,
})

if validation.Valid {
    fmt.Println("Test input is valid")
}
```

## Integration with Problem Format

```go
import (
    "gate/core/pkg/sandbox"
    "gate/core/pkg/problemformat"
)

// Load problem manifest
manifest, err := problemformat.LoadManifest(problemDir)

// Load and compile checker
checkerMeta, err := sandbox.LoadComponentFromManifest(manifest, "checker")
checkerCode, err := sandbox.LoadComponentSource(problemDir, checkerMeta)
dependencies, err := sandbox.LoadDependencies(problemDir, checkerMeta.Dependencies)

checkerBinary, err := orchestrator.CompileComponentFromSource(ctx,
    "checker",
    checkerCode,
    checkerMeta.Compiler,
    dependencies,
)

// Extract limits from manifest
limits := sandbox.ExtractLimitsFromManifest(manifest)
```

## Supported Languages

| Language | Identifier | Extension | Compilation |
|----------|------------|-----------|-------------|
| C++11    | cpp11      | .cpp      | Yes         |
| C++14    | cpp14      | .cpp      | Yes         |
| C++17    | cpp17      | .cpp      | Yes         |
| C++20    | cpp20      | .cpp      | Yes         |
| C11      | c11        | .c        | Yes         |
| Python 3 | python3    | .py       | No          |
| Go       | golang     | .go       | Yes         |
| Java 11  | java11     | .java     | Yes         |
| Java 17  | java17     | .java     | Yes         |

## Testing

### Unit Tests

```bash
go test -v ./core/pkg/sandbox
```

### Integration Tests

Integration tests require a running go-judge instance:

```bash
# Start go-judge (from workshop directory)
docker-compose up -d go-judge

# Run integration tests
go test -v -tags=integration ./core/pkg/sandbox
```

## Architecture

```
Client (HTTP/gRPC) → Compiler → go-judge (compile)
                   ↓
                   Executor → go-judge (execute)
                   ↓
                   Orchestrator (high-level workflows)
```

## Error Handling

All methods return descriptive errors. Check `Success` field in result structs:

```go
binary, err := compiler.CompileComponent(ctx, req)
if err != nil {
    // Network or request error
    log.Fatal(err)
}
if !binary.Success {
    // Compilation failed
    log.Printf("Compile error: %s\n", binary.CompileLog)
}
```

## Verdicts

- `OK` - Correct answer
- `WA` - Wrong answer
- `PE` - Presentation error
- `TLE` - Time limit exceeded
- `MLE` - Memory limit exceeded
- `RE` - Runtime error
- `CE` - Compilation error
- `IE` - Internal error
- `FAIL` - Checker/validator failure

## Resource Limits

```go
limits := sandbox.ResourceLimits{
    CPUTimeMs: 1000,  // 1 second
    MemoryMB:  256,   // 256 MB
    ProcLimit: 1,     // 1 process
    StackMB:   256,   // 256 MB stack
}
```

## Examples

See `*_test.go` files for more examples.
