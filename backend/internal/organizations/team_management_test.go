package organizations

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type teamManagementStore struct {
	TeamTxStore
	team       *organizationsmodel.Team
	manager    bool
	maintainer bool
	actorErr   error
	writes     int
}

func (s *teamManagementStore) GetTeamByID(context.Context, uuid.UUID) (*organizationsmodel.Team, error) {
	return s.team, nil
}
func (s *teamManagementStore) CanOrganization(context.Context, uuid.UUID, accessmodel.OrgAction) (bool, error) {
	return s.manager, s.actorErr
}
func (s *teamManagementStore) GetTeamMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.TeamMember, error) {
	if !s.maintainer {
		return nil, pkg.ErrNotFound
	}
	return &organizationsmodel.TeamMember{Role: organizationsmodel.TeamRoleMaintainer}, nil
}
func (s *teamManagementStore) UpdateTeam(context.Context, uuid.UUID, *organizationsmodel.UpdateTeamInput) error {
	s.writes++
	return nil
}
func (s *teamManagementStore) DeleteTeam(context.Context, uuid.UUID) error {
	s.writes++
	return nil
}
func (s *teamManagementStore) RemoveTeamMember(context.Context, uuid.UUID, uuid.UUID) error {
	s.writes++
	return nil
}

type teamManagementTransactions struct{ store *teamManagementStore }

func (t teamManagementTransactions) Within(_ context.Context, fn func(TeamTxStore) error) error {
	return fn(t.store)
}

func TestTeamManagementAuthorization(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	expired := pkg.Wrap(pkg.ErrUnauthenticated, nil, "expired")
	for _, tc := range []struct {
		name                string
		op                  string
		manager, maintainer bool
		visible             bool
		actorErr, want      error
	}{
		{name: "maintainer updates", op: "update", maintainer: true, visible: true},
		{name: "member cannot update", op: "update", visible: true, want: pkg.ErrForbidden},
		{name: "secret team stays hidden", op: "update", want: pkg.ErrNotFound},
		{name: "maintainer cannot delete", op: "delete", maintainer: true, visible: true, want: pkg.ErrForbidden},
		{name: "manager deletes", op: "delete", manager: true, visible: true},
		{name: "member leaves", op: "leave", visible: true},
		{name: "expired caller cannot leave", op: "leave", visible: true, actorErr: expired, want: pkg.ErrUnauthenticated},
		{name: "member cannot remove others", op: "remove", visible: true, want: pkg.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			teamID := uuid.New()
			store := &teamManagementStore{team: &organizationsmodel.Team{ID: teamID, OrganizationID: uuid.New()}, manager: tc.manager, maintainer: tc.maintainer, actorErr: tc.actorErr}
			uc := NewTeamsUseCase(nil, nil, teamManagementTransactions{store}, &teamsListPermissions{view: tc.visible})
			ctx := accessmodel.WithUser(context.Background(), actor)
			var err error
			switch tc.op {
			case "update":
				err = uc.UpdateTeam(ctx, teamID, &organizationsmodel.UpdateTeamInput{})
			case "delete":
				err = uc.DeleteTeam(ctx, teamID)
			case "leave":
				err = uc.RemoveTeamMember(ctx, teamID, actor.Id)
			case "remove":
				err = uc.RemoveTeamMember(ctx, teamID, uuid.New())
			}
			require.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				require.NoError(t, err)
				require.Equal(t, 1, store.writes)
			} else {
				require.Zero(t, store.writes)
			}
		})
	}
}
