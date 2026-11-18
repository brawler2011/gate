package judging

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

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cachedPackage() *LoadedPackage {
	return &LoadedPackage{
		Format:     &gfmt.GateFormat{Problem: &gfmt.Problem{Programs: map[string]gfmt.Program{"checker": {Source: "checkers/check.cpp", Language: "cpp17"}}}},
		Descriptor: model.PackageDescriptor{ID: uuid.New()},
	}
}

// Concurrent jobs of one package compile its checker once per image; another
// image compiles its own.
func TestProgramCacheCompilesOncePerPackageAndImage(t *testing.T) {
	root := t.TempDir()
	var compiles atomic.Int32
	release := make(chan struct{})
	compile := func(_ context.Context, _ *LoadedPackage, _ gfmt.Program, dir string) error {
		compiles.Add(1)
		<-release
		return os.WriteFile(filepath.Join(dir, "entry"), []byte("binary"), 0o600)
	}
	x := &ProgramCache{Root: root, Compiler: "aa", Compile: compile}
	pkg := cachedPackage()

	var wg sync.WaitGroup
	dirs := make([]string, 8)
	for i := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir, err := x.Get(context.Background(), pkg, "checker")
			assert.NoError(t, err)
			dirs[i] = dir
		}()
	}
	require.Eventually(t, func() bool { return compiles.Load() > 0 }, 5*time.Second, time.Millisecond)
	close(release)
	wg.Wait()
	for _, dir := range dirs {
		assert.FileExists(t, filepath.Join(dir, "entry"))
	}
	dir, err := x.Get(context.Background(), pkg, "checker")
	require.NoError(t, err)
	assert.Equal(t, dirs[0], dir)
	assert.EqualValues(t, 1, compiles.Load())

	y := &ProgramCache{Root: root, Compiler: "bb", Compile: compile}
	other, err := y.Get(context.Background(), pkg, "checker")
	require.NoError(t, err)
	assert.NotEqual(t, dir, other)
	assert.EqualValues(t, 2, compiles.Load())
}

// A failed or interrupted compilation leaves no entry, so the next job compiles again.
func TestProgramCacheDoesNotKeepFailedCompilation(t *testing.T) {
	failing := true
	cache := &ProgramCache{Root: t.TempDir(), Compiler: "aa", Compile: func(_ context.Context, _ *LoadedPackage, _ gfmt.Program, dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "entry"), []byte("partial"), 0o600); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if failing {
			return errors.New("compiler crashed")
		}
		return nil
	}}
	pkg := cachedPackage()
	_, err := cache.Get(context.Background(), pkg, "checker")
	require.ErrorContains(t, err, "compiler crashed")
	failing = false
	dir, err := cache.Get(context.Background(), pkg, "checker")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "entry"))
	_, err = cache.Get(context.Background(), pkg, "interactor")
	require.ErrorContains(t, err, "no interactor")
}
