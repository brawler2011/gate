//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	judgeworker "github.com/brawler2011/gate/backend/internal/messaging/judge"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const natsPort = "4222/tcp"

func TestEventPublishing(t *testing.T) {
	ctx := context.Background()
	js, natsConn := newNATSJetStream(t)
	defer natsConn.Close()

	err := pkg.EnsureSubmissionsStream(ctx, js)
	require.NoError(t, err)

	sub, err := natsConn.SubscribeSync("submissions.>")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = natsConn.Flush()
	require.NoError(t, err)

	publisher := judgeworker.NewEventPublisher(js)

	submissionID := uuid.New()
	meta := submissionsmodel.SubmissionEventMeta{
		UserId:       nil,
		Username:     "testuser",
		ContestId:    nil,
		ContestTitle: "Test Contest",
		ProblemId:    nil,
		ProblemTitle: "Test Problem",
		Language:     submissionsmodel.Cpp,
	}

	require.NoError(t, publisher.PublishQueued(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishCompilingStarted(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishTestingStarted(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishTestStarted(ctx, submissionID, 1, meta))
	require.NoError(t, publisher.PublishCompleted(ctx, submissionID, submissionsmodel.Accepted, 100, 1000, 256, 20, nil, submissionsmodel.Saved, meta))

	expectedSubjects := []string{
		submissionsmodel.SubmissionEventQueued,
		submissionsmodel.SubmissionEventCompilingStarted,
		submissionsmodel.SubmissionEventTestingStarted,
		submissionsmodel.SubmissionEventTestStarted,
		submissionsmodel.SubmissionEventCompleted,
	}

	for _, subject := range expectedSubjects {
		msg, err := sub.NextMsg(3 * time.Second)
		require.NoError(t, err)
		assert.Equal(t, subject, msg.Subject)
		assert.NotEmpty(t, msg.Data)
	}
}

func newNATSJetStream(t *testing.T) (jetstream.JetStream, *nats.Conn) {
	t.Helper()

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.10-alpine",
			ExposedPorts: []string{natsPort},
			Cmd:          []string{"-js"},
			WaitingFor: wait.ForLog("Server is ready").
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, natsPort)
	require.NoError(t, err)

	natsURL := fmt.Sprintf("nats://%s:%s", host, port.Port())
	natsConn, err := pkg.NewNatsConn(natsURL)
	require.NoError(t, err)

	js, err := jetstream.New(natsConn)
	require.NoError(t, err)

	return js, natsConn
}
