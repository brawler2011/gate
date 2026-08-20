package helpers

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	repoRoot     string
	repoRootOnce sync.Once
)

// FindRepoRoot discovers the absolute path of the repository root by locating Taskfile.yml with PROJECT.md/AGENTS.md.
func FindRepoRoot() string {
	repoRootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			panic("failed to get current working directory: " + err.Error())
		}

		for {
			taskfilePath := filepath.Join(dir, "Taskfile.yml")
			projectMdPath := filepath.Join(dir, "PROJECT.md")
			agentsMdPath := filepath.Join(dir, "AGENTS.md")

			if FileExists(taskfilePath) && (FileExists(projectMdPath) || FileExists(agentsMdPath)) {
				repoRoot = dir
				return
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached filesystem root, fallback to known standard path
				if fi, err := os.Stat("/Projects/gate/gate/Taskfile.yml"); err == nil && !fi.IsDir() {
					repoRoot = "/Projects/gate/gate"
					return
				}
				panic("could not find repository root containing Taskfile.yml and PROJECT.md")
			}
			dir = parent
		}
	})
	return repoRoot
}

// GetDeployPath returns the absolute path to a specific deploy directory (e.g. "local", "dev", "prod", "common").
func GetDeployPath(subpath ...string) string {
	elems := append([]string{FindRepoRoot(), "deploy"}, subpath...)
	return filepath.Join(elems...)
}

// GetBackendPath returns the absolute path to backend/ or subdirectories within it.
func GetBackendPath(subpath ...string) string {
	elems := append([]string{FindRepoRoot(), "backend"}, subpath...)
	return filepath.Join(elems...)
}

// GetFrontendPath returns the absolute path to frontend/ or subdirectories within it.
func GetFrontendPath(subpath ...string) string {
	elems := append([]string{FindRepoRoot(), "frontend"}, subpath...)
	return filepath.Join(elems...)
}

// FileExists checks whether a file exists at the given path and is not a directory.
func FileExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fi.IsDir()
}

// DirExists checks whether a directory exists at the given path.
func DirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// OutboxEvent mirrors the domain models.OutboxEvent struct for cross-package E2E verification.
type OutboxEvent struct {
	Id           string            `json:"id"`
	AggregateID  string            `json:"aggregate_id"`
	EventType    string            `json:"event_type"`
	Payload      []byte            `json:"payload"`
	Headers      map[string]string `json:"headers"`
	Status       string            `json:"status"`
	RetryCount   int32             `json:"retry_count"`
	ErrorMessage *string           `json:"error_message,omitempty"`
}

// CreateOutboxEventParams mirrors the domain models.CreateOutboxEventParams struct.
type CreateOutboxEventParams struct {
	Id          string            `json:"id"`
	AggregateID string            `json:"aggregate_id"`
	EventType   string            `json:"event_type"`
	Payload     []byte            `json:"payload"`
	Headers     map[string]string `json:"headers"`
}

