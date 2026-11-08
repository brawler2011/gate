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

type capabilityFactsStub struct {
	metadataFactsStub
	orgs  map[uuid.UUID]m.OrganizationContext
	users map[uuid.UUID]m.User
	calls int
}

func (f *capabilityFactsStub) LoadOrganizationFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.OrganizationContext, error) {
	f.calls++
	return f.orgs, f.err
}
func (f *capabilityFactsStub) LoadCapabilityUsers(context.Context, []uuid.UUID) (map[uuid.UUID]m.User, error) {
	f.calls++
	return f.users, f.err
}

func TestCapabilitiesRelationshipBoundaries(t *testing.T) {
	owner := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	member := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	org := m.OrganizationContext{ID: uuid.New(), Role: m.OrganizationOwner, JoinPolicy: m.JoinOpen, Owners: []m.User{owner}}
	problem := m.ProblemContext{ID: uuid.New(), Organization: org, OwnerID: owner.Id, Visibility: m.VisibilityPrivate, Owners: []m.User{owner}}
	contest := m.ContestContext{ID: uuid.New(), Organization: org, OwnerID: owner.Id, Visibility: m.VisibilityPrivate, ParticipationMode: m.ParticipationOpen, Owners: []m.User{owner}}
	f := &capabilityFactsStub{metadataFactsStub: metadataFactsStub{problems: map[uuid.UUID]m.ProblemContext{problem.ID: problem}, contests: map[uuid.UUID]m.ContestContext{contest.ID: contest}}, orgs: map[uuid.UUID]m.OrganizationContext{org.ID: org}, users: map[uuid.UUID]m.User{owner.Id: owner, member.Id: member}}
	uc := NewCapabilities(f)
	ctx := m.WithUser(m.WithReadTime(context.Background(), time.Now()), owner)
	requests := []CapabilityRequest{
		{Scope: "organization", ID: org.ID, Kind: "member", Target: owner.Id, Role: "owner"},
		{Scope: "problem", ID: problem.ID, Kind: "member", Target: owner.Id, Role: "owner"},
		{Scope: "contest", ID: contest.ID, Kind: "member", Target: owner.Id, Role: "owner"},
		{Scope: "contest", ID: contest.ID, Kind: "member", Target: member.Id, Role: "moderator"},
		{Scope: "contest", ID: contest.ID, Kind: "problem", Target: uuid.New()},
	}
	caps, err := uc.Project(ctx, requests)
	require.NoError(t, err)
	for _, row := range caps[:3] {
		require.Empty(t, row)
		require.NotNil(t, row)
	}
	require.ElementsMatch(t, []string{"change_role", "remove", "transfer_owner"}, caps[3])
	require.ElementsMatch(t, []string{"view_statements", "submit", "relink_package", "remove", "reorder"}, caps[4])
	require.NotContains(t, caps[4], "edit")
	require.NotContains(t, caps[4], "use")
	// Inherited organization ownership never gives a direct ownership transfer.
	actor := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	caps, err = uc.Project(m.WithUser(ctx, actor), requests[3:4])
	require.NoError(t, err)
	require.NotContains(t, caps[0], "transfer_owner")
	org.Role = m.OrganizationAdmin
	contest.Organization = org
	f.contests[contest.ID] = contest
	caps, err = uc.Project(m.WithUser(ctx, actor), requests[3:4])
	require.NoError(t, err)
	require.Empty(t, caps[0], "admin cannot change an equal moderator or transfer ownership")
	// A moderator may remove a participant but cannot change its role to an equal
	// moderator; rewriting the same participant role is not a change capability.
	request := requests[3]
	request.Role = "participant"
	caps, err = uc.Project(m.WithUser(ctx, actor), []CapabilityRequest{request})
	require.NoError(t, err)
	require.Equal(t, []string{"remove"}, caps[0])
}

func TestCapabilitiesTemporalBlockAndFailClosed(t *testing.T) {
	now := time.Now().UTC()
	start, end := now, now.Add(time.Hour)
	u := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	c := m.ContestContext{ID: uuid.New(), Organization: m.OrganizationContext{ID: uuid.New(), JoinPolicy: m.JoinOpen}, Visibility: m.VisibilityPublic, ParticipationMode: m.ParticipationOpen, StartTime: &start, EndTime: &end, BlockedProblems: []uuid.UUID{uuid.New()}}
	f := &capabilityFactsStub{metadataFactsStub: metadataFactsStub{contests: map[uuid.UUID]m.ContestContext{c.ID: c}}}
	requests := []CapabilityRequest{{Scope: "contest", ID: c.ID}, {Scope: "contest", ID: c.ID, Kind: "problem", Target: c.BlockedProblems[0]}}
	for _, test := range []struct {
		name               string
		now                time.Time
		submit, statements bool
	}{{"upcoming", now.Add(-time.Nanosecond), false, false}, {"start equality", start, true, true}, {"end equality", end, false, true}} {
		t.Run(test.name, func(t *testing.T) {
			ctx := m.WithUser(m.WithReadTime(context.Background(), test.now), u)
			caps, err := NewCapabilities(f).Project(ctx, requests)
			require.NoError(t, err)
			require.Equal(t, test.submit, contains(caps[0], "submit"))
			require.Equal(t, test.statements, contains(caps[1], "view_statements"))
			require.NotContains(t, caps[1], "submit")
		})
	}
	f.err = context.Canceled
	caps, err := NewCapabilities(f).Project(m.WithUser(m.WithReadTime(context.Background(), now), u), requests)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, caps)
	u.ExpiresAt = &now
	_, err = NewCapabilities(f).Project(m.WithUser(m.WithReadTime(context.Background(), now), u), requests)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func TestContestAudienceOwnRevocation(t *testing.T) {
	user := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	in := m.PolicyInput{Actor: user, Now: time.Now().UTC()}
	c := m.ContestContext{ID: uuid.New(), Organization: m.OrganizationContext{ID: uuid.New(), JoinPolicy: m.JoinOpen}, Visibility: m.VisibilityPrivate, ParticipationMode: m.ParticipationInviteOnly, DirectRole: m.ContestParticipant}
	require.NoError(t, RequireContestAudience(in, c, "participants", uuid.Nil))
	require.NoError(t, RequireContestAudience(in, c, "own", user.Id))
	require.ErrorIs(t, RequireContestAudience(in, c, "own", uuid.New()), pkg.NoPermission)
	require.ErrorIs(t, RequireContestAudience(in, c, "managers", uuid.Nil), pkg.NoPermission)
	c.DirectRole = ""
	for _, audience := range []string{"public", "participants", "managers", "own", "unknown"} {
		require.ErrorIs(t, RequireContestAudience(in, c, audience, user.Id), pkg.NoPermission)
	}
}
