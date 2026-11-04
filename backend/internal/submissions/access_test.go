package submissions

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type denySubmissionPermissions struct{ Permissions }

func (denySubmissionPermissions) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return false, nil
}

func TestCommandsEnforceAccessWithoutHTTP(t *testing.T) {
	uc := &SubmissionsUseCase{permissions: denySubmissionPermissions{}}
	contestID, submissionID := uuid.New(), uuid.New()
	for _, caller := range []accessmodel.User{accessmodel.Guest, {Id: uuid.New(), Role: accessmodel.UserRoleUser}} {
		ctx := accessmodel.WithUser(context.Background(), caller)
		want := pkg.NoPermission
		if caller.IsGuest() {
			want = pkg.ErrUnauthenticated
		}
		_, err := uc.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{ContestId: contestID, UserId: caller.Id})
		require.ErrorIs(t, err, want)
		require.ErrorIs(t, uc.BlockSubmission(ctx, contestID, submissionID, nil), want)
		_, err = uc.RejudgeSubmissions(ctx, submissionsmodel.RejudgeFilter{ContestID: contestID})
		require.ErrorIs(t, err, want)
	}
	_, err := uc.GetSubmission(context.Background(), submissionID)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	_, err = uc.ListSubmissions(context.Background(), submissionsmodel.SubmissionsFilter{})
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
