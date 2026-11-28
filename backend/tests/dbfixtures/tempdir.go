package dbfixtures

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"golang.org/x/sys/unix"
)

// Builds refuse tmpfs; the default temporary directory may be tmpfs.
func DiskTempDir(t testing.TB) string {
	t.Helper()
	dir, err := diskTempDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func diskTempDir() (string, error) {
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
		return dir, nil
	}
	return "", errors.New("no disk-backed temporary directory available")
}

func DiskCache(t testing.TB) *diskcache.Cache {
	t.Helper()
	cache, err := diskcache.Open(DiskTempDir(t), 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

var programs struct {
	sync.Mutex
	dir   string
	cache *diskcache.Cache
}

// Programs shares compiled programs across the test binary; keys cover the source and the language configuration.
func Programs(t testing.TB) *diskcache.Cache {
	t.Helper()
	programs.Lock()
	defer programs.Unlock()
	if programs.cache == nil {
		dir, err := diskTempDir()
		if err != nil {
			t.Fatal(err)
		}
		cache, err := diskcache.Open(dir, 4<<30)
		if err != nil {
			_ = os.RemoveAll(dir)
			t.Fatal(err)
		}
		programs.dir, programs.cache = dir, cache
	}
	return programs.cache
}

// RemovePrograms deletes the cache of Programs; call it from TestMain after m.Run.
func RemovePrograms() {
	programs.Lock()
	defer programs.Unlock()
	if programs.dir != "" {
		_ = os.RemoveAll(programs.dir)
	}
}
