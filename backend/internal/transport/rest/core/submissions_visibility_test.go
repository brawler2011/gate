package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/submissions"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type visibilityStub struct {
	ContestsUC
	SubmissionsUC
	contest    contestsmodel.Contest
	submission submissionsmodel.Submission
	listed     bool
}

func (m *visibilityStub) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return m.contest, nil
}

func (m *visibilityStub) GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error) {
	return m.submission, nil
}

func (m *visibilityStub) ListSubmissions(context.Context, submissionsmodel.SubmissionsFilter) (*submissionsmodel.SubmissionsList, error) {
	m.listed = true
	return &submissionsmodel.SubmissionsList{Submissions: []submissionsmodel.Submission{m.submission}}, nil
}

func TestSubmissionRESTMaskAndStateFilter(t *testing.T) {
	viewer, author, contestID := uuid.New(), uuid.New(), uuid.New()
	public := contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, FreezeStatus: contestsmodel.FreezeStatusFrozen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), Sources: contestsmodel.Same(contestsmodel.ScopePublic), Reports: contestsmodel.Same(contestsmodel.ScopePublic), Feedback: contestsmodel.Same(contestsmodel.FeedbackFull)}
	stub := &visibilityStub{contest: contestsmodel.Contest{ID: contestID, Visibility: contestsmodel.ContestVisibilityPublic, Settings: public}, submission: submissionsmodel.Submission{ID: uuid.New(), ContestID: &contestID, CreatedBy: &author, CreatedAt: time.Now(), State: 200, Score: 100, TimeStat: 5, Submission: "secret", TestDetails: &submissionsmodel.SubmissionTestDetails{}, FailedTest: new(int32), BanReason: new(string)}}
	loader := &accesstest.Loader{}
	setContest := func() {
		loader.Contests = map[uuid.UUID]access.ContestFacts{contestID: {Contest: stub.contest, Now: time.Now()}}
	}
	setContest()
	application := submissions.NewSubmissionsUseCase(&visibilityRepo{stub: stub}, nil, loader)
	h := NewCoreServer(Services{Contests: stub, Submissions: application}, false)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: viewer, Role: accessmodel.UserRoleUser})
	for _, own := range []bool{false, true} {
		t.Run(map[bool]string{true: "own", false: "other"}[own], func(t *testing.T) {
			if own {
				stub.submission.CreatedBy = &viewer
			} else {
				stub.submission.CreatedBy = &author
			}
			get, err := h.GetSubmission(ctx, corev1.GetSubmissionParams{SubmissionID: stub.submission.ID})
			require.NoError(t, err)
			list, err := h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{ContestID: contestID})
			require.NoError(t, err)
			for _, dto := range []any{&get.Submission, &list.Submissions[0]} {
				data, err := json.Marshal(dto)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				if own {
					require.InDelta(t, 200, payload["state"], 0)
					require.NotContains(t, payload, "result_hidden")
				} else {
					require.Equal(t, true, payload["result_hidden"])
					for _, key := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "failed_test", "test_details", "submission", "ban_reason"} {
						require.NotContains(t, payload, key)
					}
				}
			}
		})
	}
	stub.submission.CreatedBy = &author
	stub.listed = false
	_, err := h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{ContestID: contestID, State: corev1.NewOptInt32(200)})
	require.ErrorIs(t, err, pkg.ErrScoreboardFrozen)
	require.False(t, stub.listed)
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{ContestID: contestID, State: corev1.NewOptInt32(200), UserId: corev1.NewOptUUID(viewer)})
	require.NoError(t, err)
	loader.Grant(contestID, viewer, string(accessmodel.ContestModerator))
	get, err := h.GetSubmission(ctx, corev1.GetSubmissionParams{SubmissionID: stub.submission.ID})
	require.NoError(t, err)
	require.Equal(t, int32(200), get.Submission.State.Value)
	require.True(t, get.Submission.Submission.IsSet())
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{ContestID: contestID, State: corev1.NewOptInt32(200)})
	require.NoError(t, err)
	loader.Grant(contestID, viewer, "")
	stub.contest.Settings.FreezeStatus = contestsmodel.FreezeStatusUnfrozen
	setContest()
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{ContestID: contestID, State: corev1.NewOptInt32(200)})
	require.NoError(t, err)
	loader.Err = errors.New("db")
	_, err = h.GetSubmission(ctx, corev1.GetSubmissionParams{SubmissionID: stub.submission.ID})
	require.Error(t, err)
}

type visibilityRepo struct {
	submissions.SubmissionsRepo
	stub *visibilityStub
}

func (r *visibilityRepo) GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error) {
	return r.stub.submission, nil
}
func (r *visibilityRepo) ListSubmissions(context.Context, submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, error) {
	r.stub.listed = true
	return []submissionsmodel.Submission{r.stub.submission}, 1, nil
}
