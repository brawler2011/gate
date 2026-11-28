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

// Ref names the minimal judge image built from judge/.
func Ref() (string, error) { return ref("judge", "ghcr.io/brawler2011/gate-go-judge") }

// FullRef names the full judge image built from judge/.
func FullRef() (string, error) { return ref("judge", "ghcr.io/brawler2011/gate-go-judge-full") }

// TypesetRef names the typeset image built from typeset/.
func TypesetRef() (string, error) { return ref("typeset", "ghcr.io/brawler2011/gate-typeset") }

// Hash tracked and unignored working-tree contents so uncommitted changes get a distinct image.
func ref(source, repo string) (string, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", source)
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list %s sources: %w", source, err)
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
			return "", fmt.Errorf("open %s source: %w", source, err)
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return "", fmt.Errorf("read %s source %s: %w", source, p, err)
		}
		fmt.Fprintf(sum, "%s %x\n", p, h.Sum(nil))
	}
	return fmt.Sprintf("%s:%x", repo, sum.Sum(nil)), nil
}
