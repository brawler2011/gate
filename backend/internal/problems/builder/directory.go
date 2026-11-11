package builder

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type directory struct {
	root      string
	remaining int64
	files     int
	maxFiles  int
}

// errBudgetExceeded marks a deterministic overrun of the build budgets: retrying the same snapshot fails the same way.
var errBudgetExceeded = errors.New("build disk quota exceeded")

type quotaWriter struct {
	writer    io.Writer
	directory *directory
}

func (w quotaWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.directory.remaining {
		return 0, errBudgetExceeded
	}
	n, err := w.writer.Write(p)
	w.directory.remaining -= int64(n)
	return n, err
}
func (d *directory) create(path string) (*os.File, error) {
	if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00") {
		return nil, fmt.Errorf("invalid build path %q", path)
	}
	if d.files >= d.maxFiles {
		return nil, fmt.Errorf("%w: file count", errBudgetExceeded)
	}
	full := filepath.Join(d.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		return nil, fmt.Errorf("mkdirall: %w", err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("openfile: %w", err)
	}
	d.files++
	return f, nil
}
func (d *directory) copy(path string, src io.Reader, limit int64) (string, int64, error) {
	if limit < 0 {
		return "", 0, fmt.Errorf("negative file quota")
	}
	f, err := d.create(path)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(quotaWriter{f, d}, hash), io.LimitReader(src, limit))
	closeErr := f.Close()
	if err != nil {
		return "", n, fmt.Errorf("copy: %w", err)
	}
	if closeErr != nil {
		return "", n, fmt.Errorf("close: %w", closeErr)
	}
	var extra [1]byte
	if n == limit {
		nextra, err := src.Read(extra[:])
		if nextra > 0 {
			return "", n, fmt.Errorf("%w: file size", errBudgetExceeded)
		}
		if err != nil && err != io.EOF {
			return "", n, fmt.Errorf("read: %w", err)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}
func (d *directory) write(path string, data []byte) error {
	f, err := d.create(path)
	if err != nil {
		return err
	}
	_, err = quotaWriter{f, d}.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close: %w", closeErr)
	}
	return nil
}

// archive writes in lexical order with fixed metadata, and hashes only a
// successfully closed ZIP. Archive bytes share the whole-build disk budget.
func (d *directory) archive(ctx context.Context, path string) (digest string, size int64, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", 0, fmt.Errorf("openfile: %w", err)
	}
	defer f.Close()
	hash := sha256.New()
	z := zip.NewWriter(io.MultiWriter(quotaWriter{f, d}, hash))
	err = filepath.WalkDir(d.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("build symlink is forbidden")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("info: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("build nonregular file is forbidden")
		}
		rel, err := filepath.Rel(d.root, path)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}
		h := &zip.FileHeader{Name: filepath.ToSlash(rel), Method: zip.Deflate}
		h.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		h.SetMode(0600)
		out, err := z.CreateHeader(h)
		if err != nil {
			return fmt.Errorf("createheader: %w", err)
		}
		in, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		_, err = io.Copy(out, in)
		closeErr := in.Close()
		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("close: %w", closeErr)
		}
		return nil
	})
	if err != nil {
		_ = z.Close()
		return "", 0, fmt.Errorf("walkdir: %w", err)
	}
	if err := z.Close(); err != nil {
		return "", 0, fmt.Errorf("close: %w", err)
	}
	stat, err := f.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("stat: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), stat.Size(), nil
}
