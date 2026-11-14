package contests

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type scoreboardAccessRepo struct {
	ContestsRepo
	contest contestsmodel.Contest
	reads   int
}

func (r *scoreboardAccessRepo) GetContestScoreboardSubmissions(context.Context, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	return nil, nil
}

func (r *scoreboardAccessRepo) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return r.contest, nil
}
func (*scoreboardAccessRepo) GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	return []contestsmodel.ContestProblem{}, nil
}
func (r *scoreboardAccessRepo) GetContestScoreboardFromStandings(context.Context, uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error) {
	r.reads++
	return nil, map[uuid.UUID]string{}, nil
}

type scoreboardAccessPermissions struct{ monitor, manage bool }

func (scoreboardAccessPermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return false, nil
}

func (p scoreboardAccessPermissions) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	if action == accessmodel.ActionGetMonitor {
		return p.monitor, nil
	}
	return p.manage, nil
}

func TestScoreboardAccessAndFreezeWithoutHTTP(t *testing.T) {
	for _, tc := range []struct {
		name                                                   string
		monitor, manage, unfrozen, future, owner, admin, guest bool
		want                                                   error
	}{
		{name: "participant frozen monitor", monitor: true},
		{name: "participant cannot unfreeze", monitor: true, unfrozen: true, want: pkg.NoPermission},
		{name: "moderator live monitor", monitor: true, manage: true, unfrozen: true},
		{name: "admin live monitor", monitor: true, manage: true, unfrozen: true, admin: true},
		{name: "participant before start", monitor: true, future: true, want: pkg.NoPermission},
		{name: "owner before start", monitor: true, future: true, owner: true, manage: true},
		{name: "moderator before start", monitor: true, manage: true, future: true},
		{name: "missing monitor permission", want: pkg.NoPermission},
		{name: "guest", guest: true, want: pkg.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			if tc.admin {
				user.Role = accessmodel.UserRoleAdmin
			}
			if tc.guest {
				user = accessmodel.Guest
			}
			repo := &scoreboardAccessRepo{contest: contestsmodel.Contest{ID: uuid.New(), Settings: map[string]any{"freeze_status": contestsmodel.FreezeStatusFrozen}}}
			if tc.future {
				start := time.Now().Add(time.Hour)
				repo.contest.StartTime = &start
			}
			if tc.owner {
				repo.contest.OwnerID = &user.Id
			}
			uc := NewContestsUseCase(repo, nil, nil, scoreboardAccessPermissions{tc.monitor, tc.manage})
			response, err := uc.GetContestScoreboard(accessmodel.WithUser(context.Background(), user), repo.contest.ID, tc.unfrozen)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Nil(t, response)
				require.Zero(t, repo.reads)
				return
			}
			require.NoError(t, err)
			require.Equal(t, !tc.future, response.IsFrozen)
			require.Equal(t, 1, repo.reads)
		})
	}
}
