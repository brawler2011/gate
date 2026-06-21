//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const natsPort = "4222/tcp"

func TestVerdictCalculation(t *testing.T) {
	t.Run("StandardVerdict_AllPassed", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testProblem())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "OK", Time: 2000000, Memory: 2048 * 1024},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.Accepted, verdict.State)
		assert.Equal(t, int32(100), verdict.Score)
		assert.Equal(t, int32(2), verdict.MaxTime) // 2ms
	})

	t.Run("StandardVerdict_WrongAnswer", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testProblem())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "WA", Time: 2000000, Memory: 2048 * 1024, Message: "Wrong answer"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.GotWA, verdict.State)
		assert.Equal(t, int32(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 2, *verdict.FailedTest)
	})

	t.Run("StandardVerdict_TimeLimitExceeded", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testProblem())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "TLE", Time: 10000000000, Memory: 1024 * 1024, Message: "Time limit exceeded"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.GotTL, verdict.State)
	})

	t.Run("ScoringVerdict_PartialScore", func(t *testing.T) {
		prob := testProblemWithGroups()
		calculator := judge.NewVerdictCalculator("scoring", prob)

		score50 := 50.0
		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Score: &score50},
			{TestNumber: 2, Verdict: "WA", Score: nil},
			{TestNumber: 3, Verdict: "OK", Score: nil},
			{TestNumber: 4, Verdict: "WA", Score: nil},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.Accepted, verdict.State) // Scoring always Accepted
		assert.Greater(t, verdict.Score, int32(0))
	})
}

func TestComponentCache(t *testing.T) {
	cache := judge.NewComponentCache(nil)

	t.Run("CacheSetAndGet", func(t *testing.T) {
		key := "problem-123:checker:abc123"
		fileID := "file-xyz"

		cache.Set(key, fileID)

		cachedFileID, found := cache.Get(key)
		assert.True(t, found)
		assert.Equal(t, fileID, cachedFileID)
	})

	t.Run("CacheMiss", func(t *testing.T) {
		_, found := cache.Get("non-existent-key")
		assert.False(t, found)
	})

	t.Run("CacheSize", func(t *testing.T) {
		cache.Clear()
		assert.Equal(t, 0, cache.Size())

		cache.Set("key1", "file1")
		cache.Set("key2", "file2")
		assert.Equal(t, 2, cache.Size())
	})

	t.Run("CacheEviction", func(t *testing.T) {
		cache.Clear()

		for i := 0; i < 5; i++ {
			key := "key-" + string(rune(i))
			cache.Set(key, "file-"+string(rune(i)))
			time.Sleep(10 * time.Millisecond)
		}

		assert.LessOrEqual(t, cache.Size(), 1000)
	})
}

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

	publisher := judge.NewEventPublisher(js)

	submissionID := uuid.New()
	meta := models.SubmissionEventMeta{
		UserId:       nil,
		Username:     "testuser",
		ContestId:    nil,
		ContestTitle: "Test Contest",
		ProblemId:    nil,
		ProblemTitle: "Test Problem",
		Language:     models.Cpp,
	}

	require.NoError(t, publisher.PublishQueued(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishCompilingStarted(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishTestingStarted(ctx, submissionID, meta))
	require.NoError(t, publisher.PublishTestStarted(ctx, submissionID, 1, meta))
	require.NoError(t, publisher.PublishCompleted(ctx, submissionID, models.Accepted, 100, 1000, 256, 20, meta))

	expectedSubjects := []string{
		models.SubmissionEventQueued,
		models.SubmissionEventCompilingStarted,
		models.SubmissionEventTestingStarted,
		models.SubmissionEventTestStarted,
		models.SubmissionEventCompleted,
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

func testProblem() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "pass-fail",
		Subtasks: map[string]gfmt.Subtask{
			"samples": {
				Points: 100,
				Policy: "complete",
				Tests: []gfmt.Test{
					{Manual: "01.in"},
					{Manual: "02.in"},
				},
			},
		},
	}
}

func testProblemWithGroups() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "scoring",
		Subtasks: map[string]gfmt.Subtask{
			"group1": {
				Points: 50,
				Policy: "each",
				Tests: []gfmt.Test{
					{Manual: "01.in"},
					{Manual: "02.in"},
				},
			},
			"group2": {
				Points: 50,
				Policy: "complete",
				Tests: []gfmt.Test{
					{Manual: "03.in"},
					{Manual: "04.in"},
				},
			},
		},
	}
}
