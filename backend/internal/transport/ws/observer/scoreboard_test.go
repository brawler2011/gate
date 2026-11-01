package observer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type scoreboardStub struct {
	*accessStub
	rows, snapshots int
}

func (s *scoreboardStub) result(userID uuid.UUID, unfrozen bool) *models.ScoreboardResponse {
	contest, _ := s.GetContest(context.Background(), s.contest.ID)
	solved := !contest.IsFrozenAt(time.Now()) || unfrozen
	var count int32
	if solved {
		count = 1
	}
	return &models.ScoreboardResponse{ContestID: contest.ID, IsFrozen: contest.IsFrozenAt(time.Now()),
		Problems: []models.ScoreboardProblemHeader{}, Items: []models.ScoreboardItem{{UserID: userID, Username: "alice",
			ProblemsSolved: count, ProblemResults: []models.ContestProblemResult{}}}}
}
func (s *scoreboardStub) GetContestScoreboardUser(_ context.Context, _, userID uuid.UUID, unfrozen bool) (*models.ScoreboardResponse, error) {
	s.rows++
	return s.result(userID, unfrozen), nil
}
func (s *scoreboardStub) GetContestScoreboard(_ context.Context, _, userID uuid.UUID, unfrozen bool) (*models.ScoreboardResponse, error) {
	s.snapshots++
	return s.result(userID, unfrozen), nil
}

func TestScoreboardProjectionIsSharedAcrossSpectators(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "frozen"}[frozen], func(t *testing.T) {
			contestID, userID := uuid.New(), uuid.New()
			contest := models.Contest{ID: contestID}
			if frozen {
				contest.Settings = map[string]interface{}{"freeze_status": models.FreezeStatusFrozen}
			}
			source := &scoreboardStub{accessStub: &accessStub{contest: contest}}
			h := NewHub(10, source)
			require.NoError(t, h.trackScoreboard(context.Background(), &contestID))
			defer h.releaseScoreboard(&contestID)
			for i := 0; i < 5000; i++ {
				id := uuid.New()
				h.subscribers[id] = &Subscriber{id: id, filter: &SubmissionsFilter{ContestId: &contestID}, state: StateLive, outbox: make(chan *WrappedEvent, 2)}
			}
			event := models.SubmissionCompletedEvent{Id: uuid.New(), SubmissionEventMeta: models.SubmissionEventMeta{ContestId: &contestID, UserId: &userID}}
			payload, err := json.Marshal(event)
			require.NoError(t, err)
			require.NoError(t, h.Handle(context.WithValue(context.Background(), models.MessageStreamSequenceKey, uint64(1)), models.SubmissionEventCompleted, payload))
			expected := 1
			if frozen {
				expected = 2
			}
			require.Equal(t, expected, source.rows)
			require.Zero(t, source.snapshots)
			var first *WrappedEvent
			for _, sub := range h.subscribers {
				received := <-sub.outbox
				if first == nil {
					first = received
				}
				require.Same(t, first, received)
			}
			require.NotEmpty(t, first.scoreboardPublic)
			require.NotEmpty(t, first.scoreboardUnfrozen)
			source.contest.Settings = map[string]interface{}{"freeze_status": models.FreezeStatusUnfrozen, "penalty_per_attempt": 30}
			require.NoError(t, h.Handle(context.WithValue(context.Background(), models.MessageStreamSequenceKey, uint64(2)), models.SubmissionEventCompleted, payload))
			require.Equal(t, 1, source.snapshots)
			for _, sub := range h.subscribers {
				received := <-sub.outbox
				var frame struct {
					Payload struct {
						Replace bool `json:"replace"`
					} `json:"payload"`
				}
				require.NoError(t, json.Unmarshal(received.scoreboardPublic, &frame))
				require.True(t, frame.Payload.Replace)
			}
		})
	}
}

func TestScoreboardFramesRespectCurrentPermissions(t *testing.T) {
	for _, manager := range []bool{false, true} {
		t.Run(map[bool]string{false: "spectator", true: "manager"}[manager], func(t *testing.T) {
			f := newWSFixture(t)
			f.stub.permissions = models.ContestPermissions{GetMonitor: true, ManageContest: manager}
			f.stub.contest.Settings = map[string]interface{}{"freeze_status": models.FreezeStatusFrozen}
			source := &scoreboardStub{accessStub: f.stub}
			f.hub.contests = source
			conn := f.connect(t, f.path("submissions"))
			f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{Id: uuid.New(), State: models.Accepted, SubmissionEventMeta: f.meta(uuid.New())})
			raw := readPayload(t, conn)
			if manager {
				require.InDelta(t, models.Accepted, raw["state"], 0)
			} else {
				require.Equal(t, true, raw["frozen"])
				require.NotContains(t, raw, "state")
			}
			public := readPayload(t, conn)
			require.Equal(t, false, public["unfrozen"])
			sb := public["scoreboard"].(map[string]any)
			require.InDelta(t, 0, sb["items"].([]any)[0].(map[string]any)["problems_solved"], 0)
			if manager {
				live := readPayload(t, conn)
				require.Equal(t, true, live["unfrozen"])
				require.InDelta(t, 1, live["scoreboard"].(map[string]any)["items"].([]any)[0].(map[string]any)["problems_solved"], 0)
			}
			f.stub.change(func() { f.stub.permissions = models.ContestPermissions{} })
			f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{Id: uuid.New(), SubmissionEventMeta: f.meta(uuid.New())})
			requireClose(t, conn, websocket.ClosePolicyViolation)
		})
	}
}

func TestLiveSubscriptionSkipsOldHistoryWithoutCursor(t *testing.T) {
	f := newWSFixture(t)
	f.hub.ring = NewRingBuffer[*WrappedEvent](1)
	oldID, newID := uuid.New(), uuid.New()
	f.publish(t, models.SubmissionEventCreated, &models.SubmissionCreatedEvent{Id: oldID, SubmissionEventMeta: f.meta(f.stub.user.Id)})
	f.publish(t, models.SubmissionEventCreated, &models.SubmissionCreatedEvent{Id: oldID, SubmissionEventMeta: f.meta(f.stub.user.Id)})
	conn := f.connect(t, strings.TrimSuffix(f.path("submissions"), "&since=0"))
	require.Eventually(t, func() bool { f.hub.mu.RLock(); defer f.hub.mu.RUnlock(); return len(f.hub.subscribers) == 1 }, time.Second, time.Millisecond)
	f.publish(t, models.SubmissionEventCreated, &models.SubmissionCreatedEvent{Id: newID, SubmissionEventMeta: f.meta(f.stub.user.Id)})
	require.Equal(t, newID.String(), readPayload(t, conn)["id"])
}
