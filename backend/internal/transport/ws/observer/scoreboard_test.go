package observer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type scoreboardStub struct {
	*accessStub
	rows, snapshots int
}

func (s *scoreboardStub) result(userID uuid.UUID, unfrozen bool) *contestsmodel.ScoreboardResponse {
	contest, _ := s.GetContest(context.Background(), s.contest.ID)
	solved := !contest.IsFrozenAt(time.Now()) || unfrozen
	var count int32
	if solved {
		count = 1
	}
	return &contestsmodel.ScoreboardResponse{ContestID: contest.ID, IsFrozen: contest.IsFrozenAt(time.Now()),
		Problems: []contestsmodel.ScoreboardProblemHeader{}, Items: []contestsmodel.ScoreboardItem{{UserID: userID, Username: "alice",
			ProblemsSolved: count, ProblemResults: []contestsmodel.ContestProblemResult{}}}}
}

func (s *scoreboardStub) GetContestScoreboardUser(_ context.Context, _, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	s.rows++
	return s.result(userID, unfrozen), nil
}

func (s *scoreboardStub) ProjectScoreboard(_ context.Context, _ uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	s.snapshots++
	return s.result(uuid.Nil, unfrozen), nil
}

func TestScoreboardProjectionIsSharedAcrossSpectators(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "frozen"}[frozen], func(t *testing.T) {
			contestID, userID := uuid.New(), uuid.New()
			contest := contestsmodel.Contest{ID: contestID, OrganizationID: uuid.New(), Visibility: "public", Settings: contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, FreezeStatus: contestsmodel.FreezeStatusAuto, MonitorScope: contestsmodel.ScopeParticipant, SubmissionsListScope: contestsmodel.ScopeModerator, SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}}
			if frozen {
				contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, MonitorScope: contestsmodel.ScopePublic, SubmissionsListScope: contestsmodel.ScopePublic, FreezeStatus: contestsmodel.FreezeStatusFrozen, SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}
			}
			source := &scoreboardStub{accessStub: &accessStub{contest: contest}}
			h := NewHub(source, source)
			require.NoError(t, h.trackScoreboard(context.Background(), &contestID))
			defer h.releaseScoreboard(&contestID)
			for i := 0; i < 5000; i++ {
				id := uuid.New()
				h.subscribers[id] = &Subscriber{subscription: &subscription{id: id, outbox: make(chan *WrappedEvent, 2)}, filter: &SubmissionsFilter{ContestId: &contestID}}
			}
			event := submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{ContestId: &contestID, UserId: &userID}}
			payload, err := json.Marshal(event)
			require.NoError(t, err)
			require.NoError(t, h.Handle(context.Background(), submissionsmodel.SubmissionEventCompleted, payload))
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
			source.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, MonitorScope: contestsmodel.ScopePublic, SubmissionsListScope: contestsmodel.ScopePublic, FreezeStatus: contestsmodel.FreezeStatusUnfrozen, SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}
			require.NoError(t, h.Handle(context.Background(), submissionsmodel.SubmissionEventCompleted, payload))
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
			f.stub.permissions = contestPermissions{GetMonitor: true, ManageContest: manager}
			f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, MonitorScope: contestsmodel.ScopePublic, SubmissionsListScope: contestsmodel.ScopePublic, FreezeStatus: contestsmodel.FreezeStatusFrozen, SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}
			source := &scoreboardStub{accessStub: f.stub}
			f.hub.contests = source
			f.hub.scoreboard = source
			conn := f.connect(t, f.path("submissions"))
			f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: submissionsmodel.Accepted, SubmissionEventMeta: f.meta(uuid.New())})
			raw := readPayload(t, conn)
			if manager {
				require.InDelta(t, submissionsmodel.Accepted, raw["state"], 0)
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
			f.stub.change(func() { f.stub.permissions = contestPermissions{} })
			f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), SubmissionEventMeta: f.meta(uuid.New())})
			requireClose(t, conn, websocket.ClosePolicyViolation)
		})
	}
}

func TestFinishPublishesFinalSnapshotWithoutJudgeEvent(t *testing.T) {
	for _, status := range []string{"frozen", "unfrozen"} {
		t.Run(status, func(t *testing.T) {
			f := newWSFixture(t)
			end := time.Now().Add(300 * time.Millisecond)
			f.stub.contest.EndTime = &end
			f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, MonitorScope: contestsmodel.ScopePublic, SubmissionsListScope: contestsmodel.ScopePublic, FreezeStatus: contestsmodel.FreezeStatus(status), SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}
			f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
			source := &scoreboardStub{accessStub: f.stub}
			f.hub.contests = source
			f.hub.scoreboard = source
			conn := f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
			p := readPayload(t, conn)
			require.Equal(t, true, p["replace"])
			sb := p["scoreboard"].(map[string]any)
			require.Equal(t, false, sb["is_frozen"])
			require.InDelta(t, 1, sb["items"].([]any)[0].(map[string]any)["problems_solved"], 0)
			requireClose(t, conn, websocket.CloseTryAgainLater)
		})
	}

}

func TestMonitorAudienceDoesNotPublishModeratorOnlySubmissions(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
	f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, MonitorScope: contestsmodel.ScopePublic, SubmissionsListScope: contestsmodel.ScopeModerator, FreezeStatus: contestsmodel.FreezeStatusFrozen, SubmissionsReviewScope: contestsmodel.ScopeModerator, SubmissionDetailsScope: contestsmodel.ScopeModerator}
	source := &scoreboardStub{accessStub: f.stub}
	f.hub.contests = source
	f.hub.scoreboard = source
	conn := f.connect(t, f.path("submissions"))
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	p := readPayload(t, conn)
	require.Contains(t, p, "scoreboard")
	require.NotContains(t, p, "id")
	require.NotContains(t, p, "state")
}
