package vcs

// FileEntry represents a file or directory in the workspace.
type FileEntry struct {
	Path        string
	IsDirectory bool
	Size        int64
}
