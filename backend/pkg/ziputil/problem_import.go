package ziputil

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
)

func ExtractZip(r *zip.Reader, destDir string) error {
	cleanDest := filepath.Clean(destDir)
	const maxDecompressedFileSize = 500 * 1024 * 1024

	for _, f := range r.File {
		path := filepath.Join(cleanDest, filepath.Clean(f.Name))
		if !strings.HasPrefix(path, cleanDest+string(os.PathSeparator)) && path != cleanDest {
			return pkg.Invalid(pkg.ErrZipInvalid, fmt.Sprintf("invalid file path in zip: %s", f.Name), map[string]any{"path": f.Name})
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", path, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(path)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}

		if _, err := io.CopyN(out, rc, maxDecompressedFileSize); err != nil && !errors.Is(err, io.EOF) {
			_ = out.Close()
			_ = rc.Close()
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}

		_ = out.Close()
		_ = rc.Close()
	}

	return nil
}

// Oversized entries fail extraction instead of being silently truncated.
func ExtractZipLimited(r *zip.Reader, destDir string, maxBytes int64, maxEntries int) error {
	if maxBytes <= 0 || maxEntries <= 0 {
		return fmt.Errorf("invalid ZIP extraction quotas")
	}
	remaining := maxBytes
	seen := make(map[string]bool)
	entries := 0
	var makeDir func(string) error
	makeDir = func(path string) error {
		if path == "." {
			return nil
		}
		if seen[path] {
			return nil
		}
		if err := makeDir(filepath.ToSlash(filepath.Dir(path))); err != nil {
			return err
		}
		if entries >= maxEntries {
			return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "ZIP has too many files")
		}
		full := filepath.Join(destDir, filepath.FromSlash(path))
		if err := os.Mkdir(full, 0700); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		seen[path] = true
		entries++
		return nil
	}
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return fmt.Errorf("mkdirall: %w", err)
	}
	for _, entry := range r.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\\x00") || entry.Mode()&os.ModeSymlink != 0 {
			return pkg.Invalid(pkg.ErrZipInvalid, fmt.Sprintf("invalid ZIP path %q", entry.Name), map[string]any{"path": entry.Name})
		}
		if entry.FileInfo().IsDir() {
			if err := makeDir(name); err != nil {
				return err
			}
			continue
		}
		if !entry.Mode().IsRegular() || seen[name] {
			return pkg.Invalid(pkg.ErrZipInvalid, fmt.Sprintf("invalid ZIP entry %q", name), map[string]any{"path": name})
		}
		if entry.UncompressedSize64 > uint64(remaining) {
			return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, fmt.Sprintf("ZIP entry %q exceeds the size quota", name))
		}
		if err := makeDir(filepath.ToSlash(filepath.Dir(name))); err != nil {
			return err
		}
		if entries >= maxEntries {
			return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "ZIP has too many files")
		}
		// The sandbox user reopens judged test files through /dev/fd, which checks file permissions; private directories still hide them.
		file, err := os.OpenFile(filepath.Join(destDir, filepath.FromSlash(name)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("openfile: %w", err)
		}
		seen[name] = true
		entries++
		src, err := entry.Open()
		if err != nil {
			_ = file.Close()
			return pkg.Invalid(pkg.ErrZipInvalid, fmt.Sprintf("unsupported ZIP entry %q: %v", name, err), map[string]any{"path": name})
		}
		n, copyErr := io.Copy(file, io.LimitReader(zipEntryReader{src, name}, remaining+1))
		closeErr := errors.Join(src.Close(), file.Close())
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		if n > remaining || uint64(n) != entry.UncompressedSize64 { //nolint:gosec // n is a non-negative copy count
			return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "ZIP size quota or entry size mismatch")
		}
		remaining -= n
	}
	return nil
}

// zipEntryReader separates a corrupt archive from a failing disk during io.Copy.
type zipEntryReader struct {
	io.Reader
	name string
}

func (r zipEntryReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, pkg.Invalid(pkg.ErrZipInvalid, fmt.Sprintf("corrupt ZIP entry %q", r.name), map[string]any{"path": r.name})
	}
	return n, err //nolint:wrapcheck // io.EOF must stay unwrapped for io.Copy.
}
