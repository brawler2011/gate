package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/sync/singleflight"
)

// Program is a source to compile: its language configuration and the files it
// compiles with, by name.
type Program struct {
	Source   []byte
	Language string
	Deps     map[string][]byte
}

// key identifies what compiling p on compiler produces.
func (p Program) key(compiler string) string {
	h := sha256.New()
	field := func(h hash.Hash, b []byte) {
		_ = binary.Write(h, binary.BigEndian, uint64(len(b)))
		h.Write(b)
	}
	field(h, []byte(compiler))
	field(h, []byte(p.Language))
	field(h, p.Source)
	names := make([]string, 0, len(p.Deps))
	for name := range p.Deps {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		field(h, []byte(name))
		field(h, p.Deps[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CompileFunc compiles p within limits and writes the result into dir: the
// primary file as "entry" and each dependency under "deps/".
type CompileFunc func(ctx context.Context, p Program, limits FileRunLimits, dir string) error

// ProgramCache keeps the programs this worker's judge image compiled on local
// disk, by content: the source, the language, the files it compiles with and
// the compiler. Problem builds and the judge share it, so an unchanged checker
// is not recompiled on rebuild, and the judge reuses what the build compiled.
//
// An entry is <Root>/<key>. It is compiled in a temporary directory next to it
// and renamed into place, so a present entry is always complete; a crash leaves
// only a temporary directory, which is never read. Entries are never
// invalidated; the cache lives on pod scratch space and starts empty with the pod.
type ProgramCache struct {
	Root string
	// Compiler identifies the judge image and its language configuration (Image.Compiler).
	Compiler string
	Compile  CompileFunc
	group    singleflight.Group
}

// NewProgramCache returns a cache in root that compiles with image in the
// sandbox client runs and downloads the results through files.
func NewProgramCache(root string, client *Client, files *FileTransport, image Image) *ProgramCache {
	sb := NewSandbox(client, image.Config)
	return &ProgramCache{Root: root, Compiler: image.Compiler(), Compile: func(ctx context.Context, p Program, limits FileRunLimits, dir string) error {
		return sb.CompileTo(ctx, files, p, limits, dir)
	}}
}

// Get returns the directory holding p compiled, compiling it within limits
// unless it is cached. Limits are not part of the key: they decide only
// whether a compilation succeeds.
func (c *ProgramCache) Get(ctx context.Context, p Program, limits FileRunLimits) (string, error) {
	dir := filepath.Join(c.Root, p.key(c.Compiler))
	_, err, _ := c.group.Do(dir, func() (any, error) {
		if _, err := os.Stat(dir); err == nil {
			return nil, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("stat: %w", err)
		}
		if err := os.MkdirAll(c.Root, 0o700); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		tmp, err := os.MkdirTemp(c.Root, ".tmp-")
		if err != nil {
			return nil, fmt.Errorf("mkdirtemp: %w", err)
		}
		if err := c.Compile(ctx, p, limits, tmp); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
		if err := os.Rename(tmp, dir); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, fmt.Errorf("rename: %w", err)
		}
		return nil, nil
	})
	return dir, err //nolint:wrapcheck // compilation errors keep their kind for the caller
}

// CompileTo compiles p within limits and downloads the result into dir in the
// ProgramCache layout.
func (s *Sandbox) CompileTo(ctx context.Context, files *FileTransport, p Program, limits FileRunLimits, dir string) error {
	executable, err := s.CompileWithLimits(ctx, p.Source, p.Language, p.Deps, limits)
	if err != nil {
		return err
	}
	defer files.release(executable)
	download := func(name, id string) error {
		if !fs.ValidPath(name) || name == "." {
			return fmt.Errorf("invalid compiled file name %q", name)
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		_, copyErr := files.Download(ctx, id, f, int64(limits.Output))
		return errors.Join(copyErr, f.Close())
	}
	if err := download("entry", executable.PrimaryFileID); err != nil {
		return err
	}
	for name, id := range executable.Dependencies {
		if err := download("deps/"+name, id); err != nil {
			return err
		}
	}
	return nil
}

// UploadCompiled uploads a compiled program from dir (the ProgramCache layout)
// to the sandbox. The caller releases the returned files.
func (t *FileTransport) UploadCompiled(ctx context.Context, dir string, limit int64) (Executable, error) {
	upload := func(path string) (string, error) {
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open: %w", err)
		}
		defer f.Close()
		return t.Upload(ctx, f, limit)
	}
	executable := Executable{Dependencies: map[string]string{}}
	var err error
	if executable.PrimaryFileID, err = upload(filepath.Join(dir, "entry")); err != nil {
		return Executable{}, err
	}
	deps := filepath.Join(dir, "deps")
	err = filepath.WalkDir(deps, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if errors.Is(err, fs.ErrNotExist) && path == deps {
				return nil
			}
			return err
		}
		name, err := filepath.Rel(deps, path)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}
		id, err := upload(path)
		if err != nil {
			return err
		}
		executable.Dependencies[filepath.ToSlash(name)] = id
		return nil
	})
	if err != nil {
		t.release(executable)
		return Executable{}, fmt.Errorf("upload compiled program: %w", err)
	}
	return executable, nil
}

// release deletes the sandbox files of executable, logging failures: a file
// left behind only takes space in the sandbox file store.
func (t *FileTransport) release(executable Executable) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := []string{executable.PrimaryFileID}
	for _, id := range executable.Dependencies {
		ids = append(ids, id)
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if err := t.Delete(ctx, id); err != nil {
			slog.WarnContext(ctx, "release sandbox file", "error", err)
		}
	}
}
