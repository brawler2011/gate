package dbfixtures

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// DiskTempDir returns a scratch directory on a real disk and removes it after the test.
// Builds refuse tmpfs for large files, and the default temporary directory may be tmpfs.
func DiskTempDir(t testing.TB) string {
	t.Helper()
	for _, base := range []string{"", "/var/tmp"} {
		dir, err := os.MkdirTemp(base, "gate-test-")
		if err != nil {
			continue
		}
		var stat unix.Statfs_t
		if err := unix.Statfs(dir, &stat); err == nil && stat.Type == unix.TMPFS_MAGIC {
			_ = os.RemoveAll(dir)
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return dir
	}
	t.Fatal("no disk-backed temporary directory available")
	return ""
}
