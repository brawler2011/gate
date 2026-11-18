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

	"github.com/brawler2011/gate/backend/pkg/diskcache"
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

type ProgramCache struct {
	Cache    *diskcache.Cache
	Compiler string // Image.Compiler()
	Compile  CompileFunc
}

func NewProgramCache(cache *diskcache.Cache, client *Client, files *FileTransport, image Image) *ProgramCache {
	sb := NewSandbox(client, image.Config)
	return &ProgramCache{Cache: cache, Compiler: image.Compiler(), Compile: func(ctx context.Context, p Program, limits FileRunLimits, dir string) error {
		return sb.CompileTo(ctx, files, p, limits, dir)
	}}
}

// Get reuses compiled programs regardless of the requested limits; the caller releases the handle.
func (c *ProgramCache) Get(ctx context.Context, p Program, limits FileRunLimits) (*diskcache.Handle, error) {
	return c.Cache.Acquire(ctx, "program-"+p.key(c.Compiler), func(ctx context.Context, dir string) error { //nolint:wrapcheck // compilation errors keep their kind for the caller
		return c.Compile(ctx, p, limits, dir)
	})
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
