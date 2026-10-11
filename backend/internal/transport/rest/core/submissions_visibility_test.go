package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type visibilityStub struct {
	interfaces.ContestsUC
	interfaces.PermissionsUC
	interfaces.SubmissionsUC
	contest       models.Contest
	permissions   models.ContestPermissions
	submission    models.Submission
	permissionErr error
	listed        bool
}

func (m *visibilityStub) GetContest(context.Context, uuid.UUID) (models.Contest, error) {
	return m.contest, nil
}
func (m *visibilityStub) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (models.Contest, error) {
	return m.contest, nil
}
func (m *visibilityStub) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*models.ContestPermissions, error) {
	return &m.permissions, m.permissionErr
}
func (m *visibilityStub) GetSubmission(context.Context, uuid.UUID) (models.Submission, error) {
	return m.submission, nil
}
func (m *visibilityStub) ListSubmissions(context.Context, models.SubmissionsFilter) (*models.SubmissionsList, error) {
	m.listed = true
	return &models.SubmissionsList{Submissions: []models.Submission{m.submission}}, nil
}

func TestSubmissionRESTMaskAndStateFilter(t *testing.T) {
	viewer, author, contestID := uuid.New(), uuid.New(), uuid.New()
	stub := &visibilityStub{contest: models.Contest{ID: contestID, Settings: map[string]any{"freeze_status": models.FreezeStatusFrozen}}, permissions: models.ContestPermissions{ListUsersSubmissions: true, GetOtherUserSubmission: true, GetSubmissionDetails: true}, submission: models.Submission{ID: uuid.New(), ContestID: &contestID, CreatedBy: &author, CreatedAt: time.Now(), State: 200, Score: 100, TimeStat: 5, Submission: "secret", TestDetails: &models.SubmissionTestDetails{}, FailedTest: new(int32), BanReason: new(string)}}
	h := NewCoreServer(nil, stub, stub, stub, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := middleware.WithUser(context.Background(), models.User{Id: viewer, Role: models.UserRoleUser})
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
	stub.contest.Settings = map[string]any{"freeze_status": models.FreezeStatusUnfrozen}
	_, err = h.ListContestSubmissions(ctx, corev1.ListContestSubmissionsParams{State: corev1.NewOptInt32(200)})
	require.NoError(t, err)
	stub.permissionErr = errors.New("db")
	_, err = h.GetSubmission(ctx, corev1.GetSubmissionParams{SubmissionID: stub.submission.ID})
	require.Error(t, err)
}
