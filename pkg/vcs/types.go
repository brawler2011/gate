package vcs

import "time"

// Commit represents a git commit
type Commit struct {
	SHA       string
	Message   string
	Author    string
	Email     string
	Timestamp time.Time
}

// FileEntry represents a file or directory in the repository
type FileEntry struct {
	Path        string
	IsDirectory bool
	Size        int64
}

// FileStatus represents the git status of a file
type FileStatus struct {
	Path     string
	Staging  string // "added", "modified", "deleted", "unmodified"
	Worktree string // "added", "modified", "deleted", "unmodified"
}

// FileDiff represents changes in a file
type FileDiff struct {
	Path      string
	OldPath   string
	IsNew     bool
	IsDeleted bool
	IsRenamed bool
	Patch     string
}
