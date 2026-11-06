package inbox

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	pruneFn func(ctx context.Context, cutoff time.Time) (int64, error)
}

func (m *mockRepository) PruneReceipts(ctx context.Context, cutoff time.Time) (int64, error) {
	if m.pruneFn != nil {
		return m.pruneFn(ctx, cutoff)
	}
	return 0, nil
}

func TestPruner_Prune_Success(t *testing.T) {
	t.Parallel()

	retention := 7 * 24 * time.Hour
	before := time.Now().Add(-retention)

	var recordedCutoff time.Time
	repo := &mockRepository{
		pruneFn: func(_ context.Context, cutoff time.Time) (int64, error) {
			recordedCutoff = cutoff
			return 5, nil
		},
	}

	pruner := NewPruner(repo, retention, time.Hour)
	deleted, err := pruner.Prune(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(5), deleted)

	after := time.Now().Add(-retention)
	assert.True(t, !recordedCutoff.Before(before) && !recordedCutoff.After(after), "cutoff must match retention window")
}

func TestPruner_Prune_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("db prune failure")
	repo := &mockRepository{
		pruneFn: func(_ context.Context, _ time.Time) (int64, error) {
			return 0, expectedErr
		},
	}

	pruner := NewPruner(repo, 24*time.Hour, time.Hour)
	deleted, err := pruner.Prune(context.Background())
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, int64(0), deleted)
}

func TestPruner_Start_PeriodicAndShutdown(t *testing.T) {
	t.Parallel()

	var pruneCalls atomic.Int64
	repo := &mockRepository{
		pruneFn: func(_ context.Context, _ time.Time) (int64, error) {
			pruneCalls.Add(1)
			return 0, nil
		},
	}

	interval := 10 * time.Millisecond
	pruner := NewPruner(repo, time.Hour, interval)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		pruner.Start(ctx)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pruner did not stop within timeout after context cancellation")
	}

	assert.GreaterOrEqual(t, pruneCalls.Load(), int64(2), "pruner must execute on startup and on ticker")
}
