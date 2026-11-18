package diskcache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeBytes(n int) func(context.Context, string) error {
	return func(_ context.Context, dir string) error {
		return os.WriteFile(filepath.Join(dir, "data"), []byte(strings.Repeat("x", n)), 0o600)
	}
}

func entries(t *testing.T, root string) []string {
	t.Helper()
	list, err := os.ReadDir(root)
	require.NoError(t, err)
	names := []string{}
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

func TestConcurrentAcquireFillsOnce(t *testing.T) {
	c, err := Open(t.TempDir(), 1<<20)
	require.NoError(t, err)
	var fills atomic.Int32
	gate := make(chan struct{})
	fill := func(ctx context.Context, dir string) error {
		fills.Add(1)
		<-gate
		return writeBytes(1)(ctx, dir)
	}
	var wg sync.WaitGroup
	handles := make([]*Handle, 8)
	for i := range handles {
		wg.Go(func() {
			h, err := c.Acquire(context.Background(), "a", fill)
			assert.NoError(t, err)
			handles[i] = h
		})
	}
	require.Eventually(t, func() bool { return fills.Load() > 0 }, 5*time.Second, time.Millisecond)
	close(gate)
	wg.Wait()
	assert.EqualValues(t, 1, fills.Load())
	for _, h := range handles {
		assert.FileExists(t, filepath.Join(h.Dir(), "data"))
		h.Release()
	}
}

func TestWaiterRefillsAfterFillerIsCanceled(t *testing.T) {
	c, err := Open(t.TempDir(), 1<<20)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	failed := make(chan error)
	go func() {
		_, err := c.Acquire(ctx, "a", func(ctx context.Context, _ string) error {
			close(started)
			<-ctx.Done()
			return errors.New("rpc error: code = Canceled")
		})
		failed <- err
	}()
	<-started
	waiter := make(chan error)
	go func() {
		h, err := c.Acquire(context.Background(), "a", writeBytes(1))
		if err == nil {
			h.Release()
		}
		waiter <- err
	}()
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.entries["a"].refs == 2
	}, 5*time.Second, time.Millisecond)
	cancel()
	require.Error(t, <-failed)
	require.NoError(t, <-waiter)
}

func TestFailedFillLeavesNoEntry(t *testing.T) {
	root := t.TempDir()
	c, err := Open(root, 1<<20)
	require.NoError(t, err)
	_, err = c.Acquire(context.Background(), "a", func(ctx context.Context, dir string) error {
		require.NoError(t, writeBytes(1)(ctx, dir))
		return errors.New("checksum mismatch")
	})
	require.ErrorContains(t, err, "checksum mismatch")
	assert.Empty(t, entries(t, root), "a partial fill is never published")

	h, err := c.Acquire(context.Background(), "a", writeBytes(1))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(h.Dir(), "data"))
}

func TestEvictsLeastRecentlyUsedIdleEntries(t *testing.T) {
	root := t.TempDir()
	c, err := Open(root, 25)
	require.NoError(t, err)
	acquire := func(key string, size int) *Handle {
		h, err := c.Acquire(context.Background(), key, writeBytes(size))
		require.NoError(t, err)
		return h
	}
	pinned := acquire("pinned", 10)
	acquire("old", 10).Release()
	acquire("recent", 10).Release()
	assert.ElementsMatch(t, []string{"pinned", "recent"}, entries(t, root), "the least recently used idle entry goes first")

	acquire("over", 20)
	assert.ElementsMatch(t, []string{"pinned", "over"}, entries(t, root), "pinned entries survive even over the limit")
	assert.FileExists(t, filepath.Join(pinned.Dir(), "data"))

	pinned.Release()
	assert.ElementsMatch(t, []string{"over"}, entries(t, root), "a released entry is evicted once the cache is over its limit")
}

func TestOpenWipesPreviousContents(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".fill-crashed"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o700))
	c, err := Open(root, 1<<20)
	require.NoError(t, err)
	assert.Empty(t, entries(t, root))
	h, err := c.Acquire(context.Background(), "a", writeBytes(1))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(h.Dir(), "data"))
}
