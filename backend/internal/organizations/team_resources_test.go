package organizations

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type teamResourcesRepo struct {
	TeamsRepo
	team           *organizationsmodel.Team
	caller, teamID uuid.UUID
	calls          int
	err            error
}

func (r *teamResourcesRepo) GetTeamByID(context.Context, uuid.UUID) (*organizationsmodel.Team, error) {
	return r.team, nil
}
func (r *teamResourcesRepo) GetTeamContests(_ context.Context, teamID, caller uuid.UUID) ([]contestsmodel.Contest, error) {
	r.teamID, r.caller = teamID, caller
	r.calls++
	return []contestsmodel.Contest{{ID: teamID}}, r.err
}
func (r *teamResourcesRepo) GetTeamProblems(_ context.Context, teamID, caller uuid.UUID) ([]problemsmodel.Problem, error) {
	r.teamID, r.caller = teamID, caller
	r.calls++
	return []problemsmodel.Problem{{ID: teamID}}, r.err
}

func TestTeamResourcesAuthorization(t *testing.T) {
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	expired := caller
	past := time.Now().Add(-time.Hour)
	expired.ExpiresAt = &past
	for _, resource := range []string{"contests", "problems"} {
		for _, tc := range []struct {
			name                            string
			user                            accessmodel.User
			actor                           uuid.UUID
			view                            bool
			repoErr, permissionErr, wantErr error
			wantCalls                       int
		}{
			{name: "allowed", user: caller, actor: caller.Id, view: true, wantCalls: 1},
			{name: "storage failure", user: caller, actor: caller.Id, view: true, repoErr: context.Canceled, wantErr: context.Canceled, wantCalls: 1},
			{name: "team denied", user: caller, actor: caller.Id, wantErr: pkg.NoPermission},
			{name: "permission failure", user: caller, actor: caller.Id, permissionErr: context.Canceled, wantErr: context.Canceled},
			{name: "spoofed caller", user: caller, actor: uuid.New(), view: true, wantErr: pkg.NoPermission},
			{name: "guest", user: accessmodel.Guest, wantErr: pkg.ErrUnauthenticated},
			{name: "expired", user: expired, actor: caller.Id, view: true, wantErr: pkg.ErrUnauthenticated},
		} {
			t.Run(resource+"/"+tc.name, func(t *testing.T) {
				teamID := uuid.New()
				repo := &teamResourcesRepo{team: &organizationsmodel.Team{ID: teamID, OrganizationID: uuid.New(), Privacy: organizationsmodel.TeamPrivacyClosed}, err: tc.repoErr}
				permissions := &teamsListPermissions{view: tc.view}
				if tc.permissionErr != nil {
					permissions.errAction = accessmodel.ActionViewOrganization
				}
				uc := NewTeamsUseCase(repo, permissions, nil)
				ctx := accessmodel.WithUser(context.Background(), tc.user)
				var err error
				if resource == "contests" {
					var result []contestsmodel.Contest
					result, err = uc.GetTeamContests(ctx, teamID, tc.actor)
					if tc.wantErr == nil {
						require.Equal(t, []contestsmodel.Contest{{ID: teamID}}, result)
					}
				} else {
					var result []problemsmodel.Problem
					result, err = uc.GetTeamProblems(ctx, teamID, tc.actor)
					if tc.wantErr == nil {
						require.Equal(t, []problemsmodel.Problem{{ID: teamID}}, result)
					}
				}
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, tc.wantCalls, repo.calls)
				if repo.calls > 0 {
					require.Equal(t, caller.Id, repo.caller)
					require.Equal(t, teamID, repo.teamID)
				}
			})
		}
	}
}
