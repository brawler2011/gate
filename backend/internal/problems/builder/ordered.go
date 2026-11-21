package builder

import (
	"context"
	"sync"
)

// ordered runs fn for indexes 0..n-1, at most limit at once, and returns what a sequential loop would:
// the lowest failing index's error. A failure cancels only higher indexes, which a loop would not reach.
func ordered(ctx context.Context, n, limit int, fn func(context.Context, int) error) error {
	var (
		mu      sync.Mutex
		group   sync.WaitGroup
		cancels = make([]context.CancelFunc, n)
		failed  = n
		first   error
	)
	slots := make(chan struct{}, limit)
	for i := range n {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
		}
		mu.Lock()
		if failed < i || ctx.Err() != nil {
			mu.Unlock()
			break
		}
		taskCtx, cancel := context.WithCancel(ctx)
		cancels[i] = cancel
		mu.Unlock()
		group.Add(1)
		go func() {
			defer group.Done()
			defer func() { <-slots }()
			err := fn(taskCtx, i)
			mu.Lock()
			defer mu.Unlock()
			if err == nil || i > failed {
				return
			}
			failed, first = i, err
			for _, cancel := range cancels[i+1:] {
				if cancel != nil {
					cancel()
				}
			}
		}()
	}
	group.Wait()
	for _, cancel := range cancels {
		if cancel != nil {
			cancel()
		}
	}
	if first != nil {
		return first
	}
	return ctx.Err()
}
