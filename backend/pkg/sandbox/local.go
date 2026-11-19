package sandbox

import (
	"fmt"
	"path/filepath"
)

// File is a sandbox input: an uploaded file ID or a path that go-judge opens itself.
type File struct{ id, path string }

func FileID(id string) File { return File{id: id} }

func (f File) request() (*cmdFile, error) {
	if f.path != "" {
		if !filepath.IsAbs(f.path) || filepath.Clean(f.path) != f.path {
			return nil, fmt.Errorf("invalid sandbox file path")
		}
		return localFile(f.path), nil
	}
	if !validFileID(f.id) {
		return nil, fmt.Errorf("invalid sandbox file ID")
	}
	return cachedFile(f.id), nil
}

// SharedDir is a worker directory that go-judge sees mounted at Sandbox.
type SharedDir struct{ Local, Sandbox string }

func (d SharedDir) File(path string) (File, error) {
	rel, err := filepath.Rel(d.Local, path)
	if err != nil || !filepath.IsLocal(rel) || d.Sandbox == "" {
		return File{}, fmt.Errorf("%s is outside the directory shared with the sandbox", path)
	}
	return File{path: filepath.Join(d.Sandbox, rel)}, nil
}
