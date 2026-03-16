package vcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"
)

// GoGitService implements the Service interface using go-git
type GoGitService struct {
	baseDir string
	// locks holds one *sync.RWMutex per problem UUID (created lazily, never
	// deleted). Keeping entries in the map permanently prevents a race where a
	// goroutine that fetched a mutex pointer before DeleteProblemRepo could
	// race with InitProblemRepo obtaining a different mutex for the same ID.
	locks sync.Map
}

// NewGoGitService creates a new GoGitService
func NewGoGitService(baseDir string) *GoGitService {
	return &GoGitService{baseDir: baseDir}
}

// repoLock returns the RWMutex for a specific problem repository, creating it
// on first access. This allows concurrent operations on different problems.
func (s *GoGitService) repoLock(problemID uuid.UUID) *sync.RWMutex {
	mu, _ := s.locks.LoadOrStore(problemID, &sync.RWMutex{})
	return mu.(*sync.RWMutex)
}

// getRepoPath returns the filesystem path to a problem's repository
func (s *GoGitService) getRepoPath(problemID uuid.UUID) string {
	return filepath.Join(s.baseDir, problemID.String())
}

// GetRepoPath returns the filesystem path to the repository
func (s *GoGitService) GetRepoPath(problemID uuid.UUID) string {
	return s.getRepoPath(problemID)
}

// RepoExists checks if a repository exists for a problem
func (s *GoGitService) RepoExists(ctx context.Context, problemID uuid.UUID) bool {
	repoPath := s.getRepoPath(problemID)
	gitPath := filepath.Join(repoPath, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// InitProblemRepo creates a new Git repository for a problem
func (s *GoGitService) InitProblemRepo(ctx context.Context, problemID uuid.UUID) error {
	mu := s.repoLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	repoPath := s.getRepoPath(problemID)

	// Create directories
	dirs := []string{
		filepath.Join(repoPath, "tests"),
		filepath.Join(repoPath, "solutions"),
		filepath.Join(repoPath, "checkers"),
		filepath.Join(repoPath, "validators"),
		filepath.Join(repoPath, "generators"),
		filepath.Join(repoPath, "interactors"),
		filepath.Join(repoPath, "media"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		return fmt.Errorf("failed to init git repo: %w", err)
	}

	// Create .gitignore
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	gitignoreContent := []byte("*.o\n*.exe\n*.out\n.compiled/\n")
	if err := os.WriteFile(gitignorePath, gitignoreContent, 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Create initial README
	readmePath := filepath.Join(repoPath, "README.md")
	readmeContent := []byte("# Problem\n\nThis is a problem repository.\n")
	if err := os.WriteFile(readmePath, readmeContent, 0644); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	// Create initial commit
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if _, err := w.Add(".gitignore"); err != nil {
		return fmt.Errorf("failed to add .gitignore: %w", err)
	}
	if _, err := w.Add("README.md"); err != nil {
		return fmt.Errorf("failed to add README: %w", err)
	}

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "System",
			Email: "system@workshop",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	return nil
}

// DeleteProblemRepo deletes the repository for a problem
func (s *GoGitService) DeleteProblemRepo(ctx context.Context, problemID uuid.UUID) error {
	mu := s.repoLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	repoPath := s.getRepoPath(problemID)
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to delete repo: %w", err)
	}
	return nil
}

// ReadFile reads a file from the working copy
func (s *GoGitService) ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	fullPath := filepath.Join(s.getRepoPath(problemID), path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return content, nil
}

// WriteFile writes a file to the working copy (without committing)
func (s *GoGitService) WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error {
	mu := s.repoLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	fullPath := filepath.Join(s.getRepoPath(problemID), path)

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// DeleteFile deletes a file from the working copy
func (s *GoGitService) DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error {
	mu := s.repoLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	fullPath := filepath.Join(s.getRepoPath(problemID), path)
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}

	return nil
}

// ListFiles lists all files in a directory
func (s *GoGitService) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]FileEntry, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	fullPath := filepath.Join(s.getRepoPath(problemID), dirPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []FileEntry
	for _, entry := range entries {
		// Skip .git directory
		if entry.Name() == ".git" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(dirPath, entry.Name())
		if dirPath == "" || dirPath == "." {
			relPath = entry.Name()
		}

		files = append(files, FileEntry{
			Path:        relPath,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
		})
	}

	return files, nil
}

// Commit commits changes to the repository
func (s *GoGitService) Commit(ctx context.Context, problemID uuid.UUID, message, authorName, authorEmail string) (string, error) {
	mu := s.repoLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := git.PlainOpen(s.getRepoPath(problemID))
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes
	if _, err := w.Add("."); err != nil {
		return "", fmt.Errorf("failed to add changes: %w", err)
	}

	// Set default email if not provided
	if authorEmail == "" {
		authorEmail = authorName + "@workshop"
	}

	// Commit
	commitHash, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return commitHash.String(), nil
}

// GetStatus returns the list of modified files
func (s *GoGitService) GetStatus(ctx context.Context, problemID uuid.UUID) ([]FileStatus, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	repo, err := git.PlainOpen(s.getRepoPath(problemID))
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var fileStatuses []FileStatus
	for file, fileStatus := range status {
		fileStatuses = append(fileStatuses, FileStatus{
			Path:     file,
			Staging:  statusCodeToString(fileStatus.Staging),
			Worktree: statusCodeToString(fileStatus.Worktree),
		})
	}

	return fileStatuses, nil
}

// GetHistory returns the commit history
func (s *GoGitService) GetHistory(ctx context.Context, problemID uuid.UUID, limit int) ([]Commit, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	repo, err := git.PlainOpen(s.getRepoPath(problemID))
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	var commits []Commit
	count := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if limit > 0 && count >= limit {
			return io.EOF
		}
		commits = append(commits, Commit{
			SHA:       c.Hash.String(),
			Message:   c.Message,
			Author:    c.Author.Name,
			Email:     c.Author.Email,
			Timestamp: c.Author.When,
		})
		count++
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// GetCommitDiff returns the diff for a specific commit
func (s *GoGitService) GetCommitDiff(ctx context.Context, problemID uuid.UUID, commitSHA string) ([]FileDiff, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	repo, err := git.PlainOpen(s.getRepoPath(problemID))
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	hash := plumbing.NewHash(commitSHA)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	// Get parent commit
	parent, err := commit.Parent(0)
	var parentTree *object.Tree
	if err == nil {
		parentTree, _ = parent.Tree()
	}

	currentTree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var diffs []FileDiff
	changes, err := currentTree.Diff(parentTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	for _, change := range changes {
		patch, err := change.Patch()
		patchStr := ""
		if err == nil && patch != nil {
			patchStr = patch.String()
		}

		from, to, err := change.Files()
		if err != nil {
			continue
		}

		fileDiff := FileDiff{
			IsNew:     from == nil,
			IsDeleted: to == nil,
			Patch:     patchStr,
		}

		if from != nil {
			fileDiff.OldPath = from.Name
			fileDiff.Path = from.Name
		}
		if to != nil {
			fileDiff.Path = to.Name
			if from != nil && from.Name != to.Name {
				fileDiff.IsRenamed = true
			}
		}

		diffs = append(diffs, fileDiff)
	}

	return diffs, nil
}

// GetCurrentSHA returns the current HEAD SHA
func (s *GoGitService) GetCurrentSHA(ctx context.Context, problemID uuid.UUID) (string, error) {
	mu := s.repoLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	repo, err := git.PlainOpen(s.getRepoPath(problemID))
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func (s *GoGitService) HasUncommittedChanges(ctx context.Context, problemID uuid.UUID) (bool, error) {
	status, err := s.GetStatus(ctx, problemID)
	if err != nil {
		return false, err
	}

	for _, fileStatus := range status {
		if fileStatus.Staging != "unmodified" || fileStatus.Worktree != "unmodified" {
			return true, nil
		}
	}

	return false, nil
}

// statusCodeToString converts git status code to string
func statusCodeToString(code git.StatusCode) string {
	switch code {
	case git.Unmodified:
		return "unmodified"
	case git.Modified:
		return "modified"
	case git.Added:
		return "added"
	case git.Deleted:
		return "deleted"
	case git.Renamed:
		return "renamed"
	case git.Copied:
		return "copied"
	case git.UpdatedButUnmerged:
		return "updated"
	default:
		return "unknown"
	}
}
