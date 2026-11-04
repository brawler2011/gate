package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/submissions"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type visibilityStub struct {
	ContestsUC
	ContestsReader
	PermissionsUC
	SubmissionsUC
	contest       contestsmodel.Contest
	permissions   accessmodel.ContestPermissions
	submission    submissionsmodel.Submission
	permissionErr error
	listed        bool
}

func (m *visibilityStub) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return m.contest, nil
}

func (m *visibilityStub) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error) {
	return m.contest, nil
}

func (m *visibilityStub) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*accessmodel.ContestPermissions, error) {
	return &m.permissions, m.permissionErr
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
	stub := &visibilityStub{contest: contestsmodel.Contest{ID: contestID, Settings: map[string]any{"freeze_status": contestsmodel.FreezeStatusFrozen}}, permissions: accessmodel.ContestPermissions{ListUsersSubmissions: true, ListOwnSubmissions: true, GetOtherUserSubmission: true, GetSubmissionDetails: true}, submission: submissionsmodel.Submission{ID: uuid.New(), ContestID: &contestID, CreatedBy: &author, CreatedAt: time.Now(), State: 200, Score: 100, TimeStat: 5, Submission: "secret", TestDetails: &submissionsmodel.SubmissionTestDetails{}, FailedTest: new(int32), BanReason: new(string)}}
	application := submissions.NewSubmissionsUseCase(&visibilityRepo{stub: stub}, stub, nil, nil, stub)
	h := NewCoreServer(
		nil,
		nil,
		testContestsModule{ContestsUC: stub, PermissionsUC: stub, ContestsReader: stub},
		nil,
		application,
		nil,
		nil,
		nil,
	)
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
			list, err := h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{})
			require.NoError(t, err)
			for _, dto := range []any{&get.Submission, &list.Submissions[0]} {
				data, err := json.Marshal(dto)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				if own {
					require.InDelta(t, 200, payload["state"], 0)
					require.NotContains(t, payload, "frozen")
				} else {
					require.Equal(t, true, payload["frozen"])
					for _, key := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "failed_test", "test_details", "submission", "ban_reason"} {
						require.NotContains(t, payload, key)
					}
				}
			}
		})
	}
	stub.submission.CreatedBy = &author
	stub.listed = false
	_, err := h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{State: corev1.NewOptInt32(200)})
	require.ErrorIs(t, err, pkg.ErrBadInput)
	require.False(t, stub.listed)
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{State: corev1.NewOptInt32(200), UserId: corev1.NewOptUUID(viewer)})
	require.NoError(t, err)
	stub.permissions.ManageContest = true
	get, err := h.GetSubmission(ctx, corev1.GetSubmissionParams{SubmissionID: stub.submission.ID})
	require.NoError(t, err)
	require.Equal(t, int32(200), get.Submission.State.Value)
	require.True(t, get.Submission.Submission.IsSet())
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{State: corev1.NewOptInt32(200)})
	require.NoError(t, err)
	stub.permissions.ManageContest = false
	stub.contest.Settings = map[string]any{"freeze_status": contestsmodel.FreezeStatusUnfrozen}
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{State: corev1.NewOptInt32(200)})
	require.NoError(t, err)
	stub.permissionErr = errors.New("db")
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
func (r *visibilityRepo) ListSubmissions(context.Context, submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, int64, error) {
	r.stub.listed = true
	return []submissionsmodel.Submission{r.stub.submission}, 1, 1, nil
}
