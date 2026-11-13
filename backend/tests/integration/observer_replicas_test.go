//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/brawler2011/gate/backend/internal/messaging/pubsub"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

type subjectRecorder chan string

func (r subjectRecorder) Dispatch(_ context.Context, subject string, _ []byte) error {
	r <- subject
	return nil
}

// TestObserverEventsReachEveryReplica runs two backend replicas' observer
// subscribers side by side, as during a rolling update: each WebSocket hub keeps
// its own state, so every replica must receive every event.
func TestObserverEventsReachEveryReplica(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	js, _ := newNATSJetStream(t)
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))
	require.NoError(t, pkg.EnsureContestsStream(ctx, js))

	// Published before the replicas start: a new replica does not replay history.
	_, err := js.Publish(ctx, "submissions.created", []byte("{}"))
	require.NoError(t, err)

	var replicas [2]subjectRecorder
	for i := range replicas {
		replicas[i] = make(subjectRecorder, 10)
		submissions, err := pubsub.NewSubmissionsSub(ctx, js, replicas[i])
		require.NoError(t, err)
		contests, err := pubsub.NewContestsSub(ctx, js, replicas[i])
		require.NoError(t, err)
		for _, sub := range []*pubsub.StreamSubscriber{submissions, contests} {
			done := make(chan struct{})
			go func() { _ = sub.Start(ctx); close(done) }()
			t.Cleanup(func() { sub.Stop(); <-done })
		}
	}

	for _, subject := range []string{"submissions.updated", "contest.announcement_created"} {
		_, err := js.Publish(ctx, subject, []byte("{}"))
		require.NoError(t, err)
	}
	for i, replica := range replicas {
		got := map[string]bool{}
		for len(got) < 2 {
			select {
			case subject := <-replica:
				got[subject] = true
			case <-time.After(10 * time.Second):
				t.Fatalf("replica %d received only %v", i, got)
			}
		}
		require.Equal(t, map[string]bool{"submissions.updated": true, "contest.announcement_created": true}, got, "replica %d", i)
	}
}
