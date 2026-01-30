package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoGitService_InitProblemRepo(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "vcs-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Test init
	err = service.InitProblemRepo(ctx, problemID)
	require.NoError(t, err)

	// Verify repo exists
	assert.True(t, service.RepoExists(ctx, problemID))

	// Verify directories were created
	repoPath := service.GetRepoPath(problemID)
	assert.DirExists(t, filepath.Join(repoPath, "statement"))
	assert.DirExists(t, filepath.Join(repoPath, "tests"))
	assert.DirExists(t, filepath.Join(repoPath, "solutions"))
	assert.DirExists(t, filepath.Join(repoPath, "checkers"))
	assert.DirExists(t, filepath.Join(repoPath, "validators"))
	assert.DirExists(t, filepath.Join(repoPath, "generators"))

	// Verify .gitignore exists
	assert.FileExists(t, filepath.Join(repoPath, ".gitignore"))

	// Verify initial commit was made
	commits, err := service.GetHistory(ctx, problemID, 10)
	require.NoError(t, err)
	assert.Len(t, commits, 1)
	assert.Equal(t, "Initial commit", commits[0].Message)
}

func TestGoGitService_FileOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vcs-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Init repo
	err = service.InitProblemRepo(ctx, problemID)
	require.NoError(t, err)

	// Test write file
	testContent := []byte("test content")
	err = service.WriteFile(ctx, problemID, "test.txt", testContent)
	require.NoError(t, err)

	// Test read file
	content, err := service.ReadFile(ctx, problemID, "test.txt")
	require.NoError(t, err)
	assert.Equal(t, testContent, content)

	// Test list files
	files, err := service.ListFiles(ctx, problemID, "")
	require.NoError(t, err)
	assert.Greater(t, len(files), 0)

	// Find test.txt in files
	found := false
	for _, f := range files {
		if f.Path == "test.txt" {
			found = true
			assert.False(t, f.IsDirectory)
			break
		}
	}
	assert.True(t, found, "test.txt should be in file list")

	// Test delete file
	err = service.DeleteFile(ctx, problemID, "test.txt")
	require.NoError(t, err)

	// Verify file is deleted
	_, err = service.ReadFile(ctx, problemID, "test.txt")
	assert.Error(t, err)
}

func TestGoGitService_CommitOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vcs-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Init repo
	err = service.InitProblemRepo(ctx, problemID)
	require.NoError(t, err)

	// Write a file
	err = service.WriteFile(ctx, problemID, "new-file.txt", []byte("content"))
	require.NoError(t, err)

	// Check status before commit
	hasChanges, err := service.HasUncommittedChanges(ctx, problemID)
	require.NoError(t, err)
	assert.True(t, hasChanges)

	// Commit changes
	commitSHA, err := service.Commit(ctx, problemID, "Add new file", "Test User", "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, commitSHA)

	// Check status after commit
	hasChanges, err = service.HasUncommittedChanges(ctx, problemID)
	require.NoError(t, err)
	assert.False(t, hasChanges)

	// Verify commit history
	commits, err := service.GetHistory(ctx, problemID, 10)
	require.NoError(t, err)
	assert.Len(t, commits, 2) // Initial commit + our commit
	assert.Equal(t, "Add new file", commits[0].Message)
	assert.Equal(t, "Test User", commits[0].Author)

	// Verify current SHA
	currentSHA, err := service.GetCurrentSHA(ctx, problemID)
	require.NoError(t, err)
	assert.Equal(t, commitSHA, currentSHA)
}

func TestGoGitService_ManifestIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vcs-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Init repo
	err = service.InitProblemRepo(ctx, problemID)
	require.NoError(t, err)

	// Create default manifest
	err = service.InitDefaultManifest(ctx, problemID, "Test Problem")
	require.NoError(t, err)

	// Load manifest
	manifest, err := service.LoadManifest(ctx, problemID)
	require.NoError(t, err)
	assert.NotNil(t, manifest)
	assert.Equal(t, "pass-fail", manifest.ProblemType)
	assert.Equal(t, 1000, manifest.TimeLimitMs)
	assert.Equal(t, 256, manifest.MemoryLimitMb)

	// Check statement
	assert.Contains(t, manifest.Statements, "en")
	assert.Equal(t, "Test Problem", manifest.Statements["en"].Title)

	// Modify and save manifest
	manifest.TimeLimitMs = 2000
	err = service.SaveManifest(ctx, problemID, manifest)
	require.NoError(t, err)

	// Load again and verify
	manifest2, err := service.LoadManifest(ctx, problemID)
	require.NoError(t, err)
	assert.Equal(t, 2000, manifest2.TimeLimitMs)
}

func TestGoGitService_ValidateRepoStructure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vcs-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Init repo
	err = service.InitProblemRepo(ctx, problemID)
	require.NoError(t, err)

	// Create manifest
	err = service.InitDefaultManifest(ctx, problemID, "Test")
	require.NoError(t, err)

	// Validate structure
	err = service.ValidateRepoStructure(ctx, problemID)
	assert.NoError(t, err)

	// Delete manifest and validate again
	err = service.DeleteFile(ctx, problemID, "manifest.json")
	require.NoError(t, err)

	err = service.ValidateRepoStructure(ctx, problemID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.json not found")
}
