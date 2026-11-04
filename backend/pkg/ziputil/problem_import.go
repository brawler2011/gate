package ziputil

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractZip(r *zip.Reader, destDir string) error {
	cleanDest := filepath.Clean(destDir)
	const maxDecompressedFileSize = 500 * 1024 * 1024 // 500MB limit per file

	for _, f := range r.File {
		path := filepath.Join(cleanDest, filepath.Clean(f.Name))
		if !strings.HasPrefix(path, cleanDest+string(os.PathSeparator)) && path != cleanDest {
			return fmt.Errorf("invalid file path in zip: %s", f.Name)
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
