//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/messaging/booklet"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCompiler struct{}

func (fakeCompiler) CompilePDF(_ context.Context, tex string, _ sandbox.FileRunLimits) ([]byte, error) {
	if tex == "broken" {
		return nil, errors.New("LaTeX compilation error:\n! Undefined control sequence.")
	}
	return []byte("%PDF " + tex), nil
}

// API instances delegate booklet compilation to workers over NATS.
func TestBookletCompiledOnWorker(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	conn := newNATSConn(t, false)
	limits := dbfixtures.BuildLimits(t).Booklet
	client := booklet.NewClient(conn, limits)

	_, err := client.CompilePDF(ctx, "no worker yet")
	require.ErrorIs(t, err, nats.ErrNoResponders)

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- booklet.NewResponder(conn, fakeCompiler{}, limits).Run(runCtx) }()
	t.Cleanup(func() {
		stop()
		require.NoError(t, <-done)
	})
	require.Eventually(t, func() bool {
		_, err := client.CompilePDF(ctx, "probe")
		return err == nil
	}, 10*time.Second, 50*time.Millisecond)

	pdf, err := client.CompilePDF(ctx, "contest")
	require.NoError(t, err)
	assert.Equal(t, "%PDF contest", string(pdf))

	_, err = client.CompilePDF(ctx, "broken")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "! Undefined control sequence.")
}
