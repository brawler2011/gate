// Package judgeimage names the go-judge image built from the current judge/
// sources. Tests, `task judge:image`, CI and deploy all use this one name.
package judgeimage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const repo = "ghcr.io/brawler2011/gate-go-judge"

// Ref is repo:<content hash of judge/>. It hashes the working-tree contents
// of the files git does not ignore, so uncommitted changes get their own image
// and editor or direnv files never change the name.
func Ref() (string, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "judge")
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list judge sources: %w", err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		// Files deleted in the working tree are still listed by --cached.
		if _, err := os.Stat(filepath.Join(dir, p)); p != "" && err == nil {
			paths = append(paths, p)
		}
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
