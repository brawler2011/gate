package builder

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrderedReportsLowestFailureAndCancelsOnlyLaterTasks(t *testing.T) {
	var running, peak atomic.Int32
	cancelled := make([]atomic.Bool, 8)
	err := ordered(context.Background(), 8, 3, func(ctx context.Context, i int) error {
		peak.Store(max(peak.Load(), running.Add(1)))
		defer running.Add(-1)
		switch i {
		case 2:
			// Fails after test 3 does; a sequential loop would still report it.
			time.Sleep(100 * time.Millisecond)
			return fmt.Errorf("test %d", i)
		case 3:
			return fmt.Errorf("test %d", i)
		}
		select {
		case <-ctx.Done():
			cancelled[i].Store(true)
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})
	require.EqualError(t, err, "test 2")
	require.LessOrEqual(t, peak.Load(), int32(3))
	require.False(t, cancelled[0].Load() || cancelled[1].Load(), "earlier tests run to completion")
}

func TestOrderedRunsEveryIndexOnSuccess(t *testing.T) {
	done := make([]bool, 20)
	require.NoError(t, ordered(context.Background(), len(done), 4, func(_ context.Context, i int) error {
		done[i] = true
		return nil
	}))
	require.NotContains(t, done, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, ordered(ctx, 3, 1, func(context.Context, int) error { return nil }), context.Canceled)
}
