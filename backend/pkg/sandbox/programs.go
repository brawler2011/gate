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

type Program struct {
	Source   []byte
	Language string
	Deps     map[string][]byte
}

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

// CompileFunc compiles p into dir as "entry" and "deps/<name>".
type CompileFunc func(ctx context.Context, p Program, limits FileRunLimits, dir string) error

// ProgramCache publishes entries by atomic rename, so every visible entry is complete.
type ProgramCache struct {
	Root     string
	Compiler string // Image.Compiler()
	Compile  CompileFunc
	group    singleflight.Group
}

func NewProgramCache(root string, client *Client, files *FileTransport, image Image) *ProgramCache {
	sb := NewSandbox(client, image.Config)
	return &ProgramCache{Root: root, Compiler: image.Compiler(), Compile: func(ctx context.Context, p Program, limits FileRunLimits, dir string) error {
		return sb.CompileTo(ctx, files, p, limits, dir)
	}}
}

// Get reuses compiled programs regardless of the requested limits.
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

// UploadCompiled transfers ownership of the sandbox files to the caller.
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
