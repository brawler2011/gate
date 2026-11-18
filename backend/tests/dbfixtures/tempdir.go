package dbfixtures

import (
	"os"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"golang.org/x/sys/unix"
)

// Builds refuse tmpfs; the default temporary directory may be tmpfs.
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

func DiskCache(t testing.TB) *diskcache.Cache {
	t.Helper()
	cache, err := diskcache.Open(DiskTempDir(t), 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}
