// Package diskcache keeps immutable directories on local disk within a size budget.
package diskcache

import (
	"container/list"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Cache is not shared between processes: Open wipes the root, so entries never outlive the process.
type Cache struct {
	root    string
	limit   int64
	mu      sync.Mutex
	entries map[string]*entry
	idle    *list.List // unpinned entries, most recently used first
	size    int64
}

type entry struct {
	key   string
	ready chan struct{}
	err   error
	// abandoned: the filling caller's context ended, so err is not a verdict on the key.
	abandoned bool
	size  int64
	refs  int
	idle  *list.Element
}

// Handle pins an entry until Release, so eviction never removes a directory in use.
type Handle struct {
	cache *Cache
	entry *entry
	once  sync.Once
}

func Open(root string, limit int64) (*Cache, error) {
	if err := os.RemoveAll(root); err != nil {
		return nil, fmt.Errorf("wipe cache: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &Cache{root: root, limit: limit, entries: map[string]*entry{}, idle: list.New()}, nil
}

// Acquire returns the entry for key, running fill into an empty directory once if it is missing.
// Concurrent callers wait for the same fill and share its error. Keys must be valid file names.
func (c *Cache) Acquire(ctx context.Context, key string, fill func(ctx context.Context, dir string) error) (*Handle, error) {
	if !fs.ValidPath(key) || filepath.Base(key) != key || key[0] == '.' {
		return nil, fmt.Errorf("invalid cache key %q", key)
	}
	for {
		c.mu.Lock()
		e, ok := c.entries[key]
		if !ok {
			e = &entry{key: key, ready: make(chan struct{}), refs: 1}
			c.entries[key] = e
			c.mu.Unlock()
			return c.load(ctx, e, fill)
		}
		e.refs++
		if e.idle != nil {
			c.idle.Remove(e.idle)
			e.idle = nil
		}
		c.mu.Unlock()
		select {
		case <-e.ready:
		case <-ctx.Done():
			c.release(e)
			return nil, ctx.Err() //nolint:wrapcheck // callers check for context errors
		}
		if e.err == nil {
			return &Handle{cache: c, entry: e}, nil
		}
		c.release(e)
		if !e.abandoned {
			return nil, e.err
		}
	}
}

func (c *Cache) load(ctx context.Context, e *entry, fill func(ctx context.Context, dir string) error) (*Handle, error) {
	size, err := c.publish(ctx, e.key, fill)
	c.mu.Lock()
	if err != nil {
		delete(c.entries, e.key)
		e.err = err
		e.abandoned = ctx.Err() != nil
		close(e.ready)
		c.mu.Unlock()
		return nil, err
	}
	e.size = size
	c.size += size
	close(e.ready)
	victims := c.evict()
	c.mu.Unlock()
	removeAll(victims)
	return &Handle{cache: c, entry: e}, nil
}

// publish renames a complete directory into place, so a crash never leaves a partial entry visible.
func (c *Cache) publish(ctx context.Context, key string, fill func(ctx context.Context, dir string) error) (int64, error) {
	tmp, err := os.MkdirTemp(c.root, ".fill-")
	if err != nil {
		return 0, fmt.Errorf("mkdirtemp: %w", err)
	}
	if err := fill(ctx, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return 0, err
	}
	size, err := dirSize(tmp)
	if err == nil {
		if err = os.Rename(tmp, filepath.Join(c.root, key)); err != nil {
			err = fmt.Errorf("rename: %w", err)
		}
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return 0, err
	}
	return size, nil
}

func (c *Cache) release(e *entry) {
	c.mu.Lock()
	e.refs--
	var victims []string
	if e.refs == 0 && e.err == nil && c.entries[e.key] == e {
		e.idle = c.idle.PushFront(e)
		victims = c.evict()
	}
	c.mu.Unlock()
	removeAll(victims)
}

// evict moves least recently used idle entries aside under the lock, so a refill of the same key never meets the old directory.
func (c *Cache) evict() []string {
	var victims []string
	for c.size > c.limit && c.idle.Len() > 0 {
		e := c.idle.Remove(c.idle.Back()).(*entry) //nolint:forcetypeassert // idle holds only entries
		e.idle = nil
		delete(c.entries, e.key)
		c.size -= e.size
		trash, err := os.MkdirTemp(c.root, ".evict-")
		if err == nil {
			err = os.Rename(filepath.Join(c.root, e.key), filepath.Join(trash, "entry"))
		}
		if err != nil {
			// Keep the directory accounted; a refill would collide with it.
			c.entries[e.key] = e
			c.size += e.size
			e.idle = c.idle.PushFront(e)
			if trash != "" {
				victims = append(victims, trash)
			}
			break
		}
		victims = append(victims, trash)
	}
	return victims
}

func removeAll(paths []string) {
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
}

func dirSize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat: %w", err)
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("measure cache entry: %w", err)
	}
	return size, nil
}

func (h *Handle) Dir() string { return filepath.Join(h.cache.root, h.entry.key) }

func (h *Handle) Release() { h.once.Do(func() { h.cache.release(h.entry) }) }
