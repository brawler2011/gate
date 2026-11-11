// Package judgeimage names the go-judge image built from the current judge/
// sources. Tests, `task judge:image`, CI and deploy all use this one name.
package judgeimage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

const repo = "ghcr.io/brawler2011/gate-go-judge"

// Ref is repo:<content hash of judge/>. Hashing the files rather than a commit
// gives uncommitted changes their own image instead of a stale one.
func Ref() (string, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "judge")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash judge sources: %w", err)
	}
	sort.Strings(paths)
	sum := sha256.New()
	for _, p := range paths {
		f, err := os.Open(filepath.Join(dir, p))
		if err != nil {
			return "", fmt.Errorf("open judge source: %w", err)
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return "", fmt.Errorf("read judge source %s: %w", p, err)
		}
		fmt.Fprintf(sum, "%s %x\n", p, h.Sum(nil))
	}
	return fmt.Sprintf("%s:%x", repo, sum.Sum(nil)), nil
}
