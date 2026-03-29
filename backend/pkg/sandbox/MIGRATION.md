# Migration Guide: Workshop Backend to Sandbox SDK

This document describes the migration of the workshop backend from the old `judge.GoJudgeClient` to the new `sandbox` package.

## Changes Made

### 1. Package Structure

**Old:**
- `workshop/backend/internal/judge/gojudge.go` - Basic HTTP client for go-judge

**New:**
- `core/pkg/sandbox/` - Complete SDK with:
  - `client.go` - gRPC client for go-judge
  - `compiler.go` - Multi-language compilation
  - `executor.go` - Component execution
  - `orchestrator.go` - High-level workflows
  - `languages.go` - Language configurations
  - `types.go` - Common types
  - `manifest_integration.go` - Problem format integration

### 2. Handler Updates

**File:** `workshop/backend/internal/transport/http/handlers.go`

**Changes:**
- Replaced `judge *judge.GoJudgeClient` with `orchestrator *sandbox.Orchestrator`
- Updated `InvokeProblem` to use `orchestrator.JudgeSolution()`
- Added multi-language support (C++, Python, Go, Java)
- Improved error handling and verdict mapping

**Before:**
```go
result, err := h.judge.CompileAndRun(
    string(solutionContent),
    string(input),
    2*1e9,
    256*1024*1024,
)
```

**After:**
```go
result, err := h.orchestrator.JudgeSolution(ctx, sandbox.JudgeSolutionRequest{
    SolutionCode:     string(solutionContent),
    SolutionLanguage: language,
    CheckerFileID:    "",
    Input:            input,
    Answer:           expectedOutput,
    TimeLimitMs:      2000,
    MemoryLimitMB:    256,
})
```

### 3. Main Server Updates

**File:** `workshop/backend/cmd/server/main.go`

**Changes:**
- Replaced `judge.NewGoJudgeClient()` with `sandbox.NewClient()` and `sandbox.NewOrchestrator()`
- Added proper client cleanup with `defer sandboxClient.Close()`
- Added timeout configuration

**Before:**
```go
judgeClient := judge.NewGoJudgeClient(goJudgeURL)
handlers := httpTransport.NewHandlers(vcsService, judgeClient, polygonRepo)
```

**After:**
```go
sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
    Addr:    goJudgeURL,
    Timeout: 60 * time.Second,
})
defer sandboxClient.Close()

orchestrator := sandbox.NewOrchestrator(sandboxClient)
handlers := httpTransport.NewHandlers(vcsService, orchestrator, polygonRepo)
```

### 4. Dependencies

**Updated Files:**
- `workshop/backend/go.mod` - Added `gate/core` and `github.com/criyle/go-judge/pb`
- `core/go.mod` - Changed module name to `gate/core`, added go-judge dependency

### 5. Language Support

The new SDK supports multiple languages:

| Language | Extension | Supported |
|----------|-----------|-----------|
| C++      | .cpp      | ✓         |
| Python   | .py       | ✓         |
| Go       | .go       | ✓         |
| Java     | .java     | ✓         |
| C        | .c        | ✓         |

### 6. Verdict Mapping

Added proper verdict mapping from sandbox to polygon format:

```go
func mapVerdict(sandboxVerdict string) polygonv1.Verdict {
    switch sandboxVerdict {
    case "OK": return polygonv1.OK
    case "WA": return polygonv1.WA
    case "TLE": return polygonv1.TLE
    case "MLE": return polygonv1.MLE
    case "RE": return polygonv1.RE
    case "CE": return polygonv1.CE
    case "PE": return polygonv1.PE
    default: return polygonv1.IE
    }
}
```

## Benefits

1. **Multi-language support**: Not limited to C++ anymore
2. **Better error handling**: Proper error messages and status codes
3. **Modular design**: Separate compiler, executor, and orchestrator
4. **Stable transport**: gRPC-only client for go-judge
5. **Resource management**: Proper cleanup and timeout handling
6. **Extensible**: Easy to add checkers, validators, generators
7. **Testable**: Unit and integration tests included
8. **Problem format integration**: Works seamlessly with `core/pkg/problemformat`

## Testing

### Before Migration
```bash
# Only basic compilation and execution
# Limited to C++
# No proper error handling
```

### After Migration
```bash
# Unit tests
go test -v ./core/pkg/sandbox

# Integration tests (requires go-judge)
docker-compose up -d go-judge
go test -v -tags=integration ./core/pkg/sandbox
```

## Backward Compatibility

The old `workshop/backend/internal/judge/gojudge.go` file is **deprecated** but not yet removed. It can be safely deleted after verifying the migration works correctly.

To remove:
```bash
rm workshop/backend/internal/judge/gojudge.go
```

## Future Enhancements

1. **Checker support**: Compile and use custom checkers
2. **Generator support**: Generate tests dynamically
3. **Validator support**: Validate test inputs
4. **Interactor support**: Interactive problems
5. **gRPC protocol**: For better performance
6. **Binary caching**: Reuse compiled binaries across requests

## Migration Checklist

- [x] Create sandbox SDK package
- [x] Implement multi-language support
- [x] Update handler to use orchestrator
- [x] Update main server initialization
- [x] Update dependencies in go.mod
- [x] Add unit tests
- [x] Add integration tests
- [x] Document changes

## Rollback Procedure

If issues arise, rollback by:

1. Revert `workshop/backend/internal/transport/http/handlers.go`
2. Revert `workshop/backend/cmd/server/main.go`
3. Revert `workshop/backend/go.mod`
4. Restore old `judge.GoJudgeClient` import

## Support

For issues or questions, refer to:
- `core/pkg/sandbox/README.md` - SDK documentation
- `core/pkg/sandbox/*_test.go` - Usage examples
- Integration tests for real-world scenarios
