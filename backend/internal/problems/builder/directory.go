package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type directory struct {
	mu        sync.Mutex
	root      string
	remaining int64
	files     int
	maxFiles  int
}

// Budget overruns are deterministic; retrying the same snapshot cannot help.
var errBudgetExceeded = errors.New("build disk quota exceeded")

type quotaWriter struct {
	writer    io.Writer
	directory *directory
}

func (w quotaWriter) Write(p []byte) (int, error) {
	d := w.directory
	d.mu.Lock()
	if int64(len(p)) > d.remaining {
		d.mu.Unlock()
		return 0, errBudgetExceeded
	}
	d.remaining -= int64(len(p))
	d.mu.Unlock()
	n, err := w.writer.Write(p)
	d.mu.Lock()
	d.remaining += int64(len(p) - n)
	d.mu.Unlock()
	return n, err
}
func (d *directory) create(path string) (*os.File, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
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
