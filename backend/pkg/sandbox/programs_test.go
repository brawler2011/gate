package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checker(testlib string) Program {
	return Program{Source: []byte("#include \"testlib.h\"\nint main() {}\n"), Language: "cpp17", Deps: map[string][]byte{"testlib.h": []byte(testlib)}}
}

func openCache(t *testing.T) *diskcache.Cache {
	t.Helper()
	cache, err := diskcache.Open(t.TempDir(), 1<<30)
	require.NoError(t, err)
	return cache
}

// Concurrent requests share compilation; compiler or library changes invalidate the shared cache.
func TestProgramCacheCompilesEachContentOncePerCompiler(t *testing.T) {
	cache := openCache(t)
	var compiles atomic.Int32
	release := make(chan struct{})
	compile := func(_ context.Context, p Program, _ FileRunLimits, dir string) error {
		compiles.Add(1)
		<-release
		return os.WriteFile(filepath.Join(dir, "entry"), p.Deps["testlib.h"], 0o600)
	}
	x := &ProgramCache{Cache: cache, Compiler: "aa", Compile: compile}

	var wg sync.WaitGroup
	dirs := make([]string, 8)
	errs := make([]error, len(dirs))
	for i := range dirs {
		wg.Go(func() {
			h, err := x.Get(context.Background(), checker("v1"), FileRunLimits{})
			if errs[i] = err; err == nil {
				dirs[i] = h.Dir()
			}
		})
	}
	require.Eventually(t, func() bool { return compiles.Load() > 0 }, 5*time.Second, time.Millisecond)
	close(release)
	wg.Wait()
	require.NoError(t, errors.Join(errs...))
	for _, dir := range dirs {
		assert.Equal(t, dirs[0], dir)
	}
	assert.FileExists(t, filepath.Join(dirs[0], "entry"))
	assert.EqualValues(t, 1, compiles.Load())

	again, err := x.Get(context.Background(), checker("v1"), FileRunLimits{CPU: time.Second})
	require.NoError(t, err)
	assert.Equal(t, dirs[0], again.Dir(), "limits are not part of the key")
	assert.EqualValues(t, 1, compiles.Load())

	changed, err := x.Get(context.Background(), checker("v2"), FileRunLimits{})
	require.NoError(t, err)
	assert.NotEqual(t, dirs[0], changed.Dir())
	assert.EqualValues(t, 2, compiles.Load())

	y := &ProgramCache{Cache: cache, Compiler: "bb", Compile: compile}
	other, err := y.Get(context.Background(), checker("v1"), FileRunLimits{})
	require.NoError(t, err)
	assert.NotEqual(t, dirs[0], other.Dir())
	assert.EqualValues(t, 3, compiles.Load())
}

// Cache identity distinguishes file boundaries and names, even when concatenated bytes match.
func TestProgramKeySeparatesFields(t *testing.T) {
	base := Program{Source: []byte("ab"), Language: "cpp17", Deps: map[string][]byte{"x.h": []byte("c")}}
	keys := map[string]Program{}
	for _, p := range []Program{
		base,
		{Source: []byte("a"), Language: "cpp17", Deps: map[string][]byte{"x.h": []byte("bc")}},
		{Source: []byte("ab"), Language: "cpp17", Deps: map[string][]byte{"y.h": []byte("c")}},
		{Source: []byte("ab"), Language: "cpp20", Deps: map[string][]byte{"x.h": []byte("c")}},
		{Source: []byte("ab"), Language: "cpp17"},
	} {
		key := p.key("aa")
		require.NotContains(t, keys, key, "%+v collides with %+v", p, keys[key])
		keys[key] = p
	}
}

// A failed or interrupted compilation leaves no entry, so the next one compiles again.
func TestProgramCacheDoesNotKeepFailedCompilation(t *testing.T) {
	failing := true
	cache := &ProgramCache{Cache: openCache(t), Compiler: "aa", Compile: func(_ context.Context, _ Program, _ FileRunLimits, dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "entry"), []byte("partial"), 0o600); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if failing {
			return errors.New("compiler crashed")
		}
		return nil
	}}
	_, err := cache.Get(context.Background(), checker("v1"), FileRunLimits{})
	require.ErrorContains(t, err, "compiler crashed")
	failing = false
	h, err := cache.Get(context.Background(), checker("v1"), FileRunLimits{})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(h.Dir(), "entry"))
}
