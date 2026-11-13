//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/messaging/booklet"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type fakeCompiler struct{}

func (fakeCompiler) CompilePDF(_ context.Context, tex string) ([]byte, error) {
	if tex == "broken" {
		return nil, errors.New("LaTeX compilation error:\n! Undefined control sequence.")
	}
	return []byte("%PDF " + tex), nil
}

// TestBookletCompiledOnWorker sends booklets from an API instance, which has no
// sandbox, to a worker over NATS.
func TestBookletCompiledOnWorker(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "nats:2.10-alpine", ExposedPorts: []string{"4222/tcp"},
		WaitingFor: wait.ForAll(wait.ForLog("Server is ready"), wait.ForListeningPort("4222/tcp")),
	}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, terminate(container)) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	conn, err := nats.Connect(fmt.Sprintf("nats://%s:%s", host, port.Port()))
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	client := booklet.NewClient(conn)

	_, err = client.CompilePDF(ctx, "no worker yet")
	require.ErrorIs(t, err, nats.ErrNoResponders)

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- booklet.NewResponder(conn, fakeCompiler{}).Run(runCtx) }()
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
