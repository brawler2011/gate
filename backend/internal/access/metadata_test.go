package access

import (
	"context"
	"testing"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type metadataFactsStub struct {
	problems map[uuid.UUID]m.ProblemContext
	contests map[uuid.UUID]m.ContestContext
	teams    map[uuid.UUID]m.TeamContext
	err      error
}

func (f metadataFactsStub) LoadProblemFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.ProblemContext, error) {
	return f.problems, f.err
}
func (f metadataFactsStub) LoadContestFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.ContestContext, error) {
	return f.contests, f.err
}
func (f metadataFactsStub) LoadTeamFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.TeamContext, error) {
	return f.teams, f.err
}

func TestMetadataPermissionsFailClosed(t *testing.T) {
	id := uuid.New()
	org := m.OrganizationContext{ID: uuid.New(), JoinPolicy: m.JoinOpen}
	valid := metadataFactsStub{
		problems: map[uuid.UUID]m.ProblemContext{id: {ID: id, Organization: org, Visibility: m.VisibilityPublic}},
		contests: map[uuid.UUID]m.ContestContext{id: {ID: id, Organization: org, Visibility: m.VisibilityPublic, ParticipationMode: m.ParticipationOpen}},
		teams:    map[uuid.UUID]m.TeamContext{id: {ID: id, Organization: org, Privacy: m.TeamClosed}},
	}
	users := newMockUsersReader()
	u := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	users.users[u.Id] = u
	for _, tc := range []struct {
		name  string
		facts metadataFactsStub
		err   error
	}{
		{name: "load failure", facts: metadataFactsStub{err: context.Canceled}, err: context.Canceled},
		{name: "missing resource", err: pkg.ErrNotFound},
		{name: "valid", facts: valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewMetadataPermissions(NewPermissionsUseCase(nil, users, nil, nil, nil), tc.facts)
			for name, check := range map[string]func() (bool, error){
				"problem": func() (bool, error) {
					return uc.HasProblemPermission(context.Background(), id, u.Id, m.ActionViewProblem)
				},
				"contest": func() (bool, error) {
					return uc.HasContestPermission(context.Background(), id, u.Id, m.ActionViewContest)
				},
				"team": func() (bool, error) { return uc.HasTeamPermission(context.Background(), id, u.Id, m.ActionViewTeam) },
			} {
				t.Run(name, func(t *testing.T) {
					allowed, err := check()
					require.ErrorIs(t, err, tc.err)
					require.Equal(t, tc.err == nil, allowed)
				})
			}
		})
	}
	// A persisted expiry at the fixed request clock rejects the actor even if the
	// session context still carries active credentials. No facts may be loaded.
	now := time.Now().UTC()
	u.ExpiresAt = &now
	users.users[u.Id] = u
	uc := NewMetadataPermissions(NewPermissionsUseCase(nil, users, nil, nil, nil), nil)
	ctx := m.WithReadTime(context.Background(), now)
	allowed, err := uc.HasProblemPermission(ctx, id, u.Id, m.ActionViewProblem)
	require.False(t, allowed)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	allowed, err = uc.HasContestPermission(ctx, id, u.Id, m.ActionViewContest)
	require.False(t, allowed)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	allowed, err = uc.HasTeamPermission(ctx, id, u.Id, m.ActionViewTeam)
	require.False(t, allowed)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
