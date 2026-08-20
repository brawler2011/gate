package helpers

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SubprocessResult captures command execution output and status.
type SubprocessResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// RunCommand executes an external command with timeout.
func RunCommand(ctx context.Context, dir, name string, args ...string) (SubprocessResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return SubprocessResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, err
}

// RunTask executes `task <taskName>` in the repository root directory.
// AGENTS.md strictly mandates using `task` (never `go-task`).
func RunTask(t *testing.T, taskName string, timeout time.Duration) (SubprocessResult, error) {
	t.Helper()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	repoRoot := FindRepoRoot()
	return RunCommand(ctx, repoRoot, "task", taskName)
}

// RunBunScript executes `bun run <scriptName>` in the frontend directory.
// AGENTS.md strictly mandates using `bun` (never `npm`, `yarn`, `pnpm`).
func RunBunScript(t *testing.T, scriptName string, timeout time.Duration) (SubprocessResult, error) {
	t.Helper()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	frontendDir := GetFrontendPath()
	return RunCommand(ctx, frontendDir, "bun", "run", scriptName)
}

// AssertTaskSuccess executes a task command and asserts exit code 0.
func AssertTaskSuccess(t *testing.T, taskName string, timeout time.Duration) {
	t.Helper()
	res, err := RunTask(t, taskName, timeout)
	require.NoError(t, err, "Task '%s' failed (exit code %d):\nStdout:\n%s\nStderr:\n%s",
		taskName, res.ExitCode, res.Stdout, res.Stderr)
	require.Equal(t, 0, res.ExitCode, "Task '%s' returned non-zero exit code: %d", taskName, res.ExitCode)
}

// AssertBunSuccess executes a bun script and asserts exit code 0.
func AssertBunSuccess(t *testing.T, scriptName string, timeout time.Duration) {
	t.Helper()
	res, err := RunBunScript(t, scriptName, timeout)
	require.NoError(t, err, "Bun script '%s' failed (exit code %d):\nStdout:\n%s\nStderr:\n%s",
		scriptName, res.ExitCode, res.Stdout, res.Stderr)
	require.Equal(t, 0, res.ExitCode, "Bun script '%s' returned non-zero exit code: %d", scriptName, res.ExitCode)
}
