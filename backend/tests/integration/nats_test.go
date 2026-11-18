//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const natsPort = "4222/tcp"

// JetStream is optional so core NATS tests cannot accidentally depend on it.
func newNATSConn(t *testing.T, enableJetStream bool) *nats.Conn {
	t.Helper()
	ctx := context.Background()
	request := testcontainers.ContainerRequest{
		Image:        "nats:2.10-alpine",
		ExposedPorts: []string{natsPort},
		// Wait for NATS initialization as well as TCP readiness on the mapped port.
		WaitingFor: wait.ForAll(wait.ForLog("Server is ready"), wait.ForListeningPort(natsPort)),
	}
	if enableJetStream {
		request.Cmd = []string{"-js"}
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	})
	// A failed start can still return a container that needs to be removed.
	if container != nil {
		t.Cleanup(func() { require.NoError(t, terminate(container)) })
	}
	require.NoError(t, err)

	endpoint, err := container.PortEndpoint(ctx, natsPort, "nats")
	require.NoError(t, err)
	conn, err := nats.Connect(endpoint)
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	return conn
}

func newNATSJetStream(t *testing.T) (jetstream.JetStream, *nats.Conn) {
	t.Helper()
	conn := newNATSConn(t, true)
	js, err := jetstream.New(conn)
	require.NoError(t, err)
	return js, conn
}
