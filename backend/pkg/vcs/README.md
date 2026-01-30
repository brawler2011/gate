# VCS Package

Git-based version control service for problem workshop.

## Features

- Git repository management per problem
- File operations (read, write, delete, list)
- Commit operations with full history
- Integration with problemformat SDK
- Manifest validation

## Usage

### Initialize Service

```go
import "github.com/gate149/core/pkg/vcs"

vcsService := vcs.NewGoGitService("/path/to/repos")
```

### Create Problem Repository

```go
problemID := uuid.New()
err := vcsService.InitProblemRepo(ctx, problemID)
```

This creates:
- Git repository at `/path/to/repos/{problemID}/`
- Directory structure: `statement/`, `tests/`, `solutions/`, `checkers/`, `validators/`, `generators/`, `interactors/`, `media/`
- Initial `.gitignore` and `README.md`
- Initial commit

### File Operations

```go
// Write file
err := vcsService.WriteFile(ctx, problemID, "checkers/checker.cpp", []byte(code))

// Read file
content, err := vcsService.ReadFile(ctx, problemID, "checkers/checker.cpp")

// List files
files, err := vcsService.ListFiles(ctx, problemID, "checkers")

// Delete file
err := vcsService.DeleteFile(ctx, problemID, "checkers/old_checker.cpp")
```

### Git Operations

```go
// Commit changes
commitSHA, err := vcsService.Commit(ctx, problemID, "Add checker", "User Name", "user@example.com")

// Get status
status, err := vcsService.GetStatus(ctx, problemID)

// Get history
commits, err := vcsService.GetHistory(ctx, problemID, 20)

// Check for uncommitted changes
hasChanges, err := vcsService.HasUncommittedChanges(ctx, problemID)
```

### Manifest Integration

```go
// Initialize default manifest
err := vcsService.InitDefaultManifest(ctx, problemID, "Problem Title")

// Load manifest
manifest, err := vcsService.LoadManifest(ctx, problemID)

// Modify and save
manifest.TimeLimitMs = 2000
err = vcsService.SaveManifest(ctx, problemID, manifest)

// Validate repository structure
err = vcsService.ValidateRepoStructure(ctx, problemID)
```

## Thread Safety

The service uses a `sync.RWMutex` to ensure thread-safe Git operations. Multiple goroutines can safely read files concurrently, but write operations are serialized.

## Testing

```bash
go test ./core/pkg/vcs
```

See `gogit_test.go` for examples.
