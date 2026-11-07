package access

import (
	"testing"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRoleDelegationMatrix(t *testing.T) {
	// Exhaust every old/new role pair, including absent grants and unknown values.
	target := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	for _, owner := range []bool{false, true} {
		for _, old := range []m.OrganizationRole{"", m.OrganizationMember, m.OrganizationAdmin, m.OrganizationOwner, "invalid"} {
			for _, next := range []m.OrganizationRole{"", m.OrganizationMember, m.OrganizationAdmin, m.OrganizationOwner, "invalid"} {
				in, org, _, _ := policyFixture()
				org.Role = m.OrganizationAdmin
				if owner {
					org.Role = m.OrganizationOwner
				}
				before := m.OwnerState{Owners: []m.User{in.Actor}}
				if old == m.OrganizationOwner {
					before.Owners = append(before.Owners, target)
				}
				after := m.OwnerState{Owners: []m.User{in.Actor}}
				if next == m.OrganizationOwner {
					after.Owners = append(after.Owners, target)
				}
				allowed := old != "invalid" && next != "invalid" && (old != "" || next != "") && (owner || ((old == "" || old == m.OrganizationMember) && (next == "" || next == m.OrganizationMember)))
				matrixResult(t, allowed, RequireGrantOrgRole(in, org, m.RoleChange[m.OrganizationRole]{Target: target, Old: old, New: next, BeforeOwners: before, AfterOwners: after}))
			}
		}
		for _, old := range []m.ProblemRole{"", m.ProblemViewer, m.ProblemModerator, m.ProblemAdmin, m.ProblemOwner, "invalid"} {
			for _, next := range []m.ProblemRole{"", m.ProblemViewer, m.ProblemModerator, m.ProblemAdmin, m.ProblemOwner, "invalid"} {
				in, _, p, _ := policyFixture()
				p.Organization.Role = m.OrganizationAdmin
				if owner {
					p.DirectRole = m.ProblemOwner
				}
				before := m.OwnerState{Owners: []m.User{in.Actor}}
				after := m.OwnerState{Owners: []m.User{in.Actor}}
				if old == m.ProblemOwner {
					before.Owners = append(before.Owners, target)
				}
				if next == m.ProblemOwner {
					after.Owners = append(after.Owners, target)
				}
				validOld := old == "" || old == m.ProblemViewer || old == m.ProblemModerator || old == m.ProblemOwner
				validNext := next == "" || next == m.ProblemViewer || next == m.ProblemModerator || next == m.ProblemOwner
				allowed := validOld && validNext && (old != "" || next != "") && (owner || (old != m.ProblemOwner && next != m.ProblemOwner))
				matrixResult(t, allowed, RequireGrantProblemRole(in, p, m.RoleChange[m.ProblemRole]{Target: target, Old: old, New: next, BeforeOwners: before, AfterOwners: after}))
			}
		}
		for _, old := range []m.ContestRole{"", m.ContestParticipant, m.ContestModerator, m.ContestOwner, "invalid"} {
			for _, next := range []m.ContestRole{"", m.ContestParticipant, m.ContestModerator, m.ContestOwner, "invalid"} {
				in, _, _, c := policyFixture()
				c.DirectRole = m.ContestModerator
				if owner {
					c.DirectRole = m.ContestOwner
				}
				before := m.OwnerState{Owners: []m.User{in.Actor}}
				after := m.OwnerState{Owners: []m.User{in.Actor}}
				if old == m.ContestOwner {
					before.Owners = append(before.Owners, target)
				}
				if next == m.ContestOwner {
					after.Owners = append(after.Owners, target)
				}
				allowed := old != "invalid" && next != "invalid" && (old != "" || next != "") && (owner || ((old == "" || old == m.ContestParticipant) && (next == "" || next == m.ContestParticipant)))
				matrixResult(t, allowed, RequireGrantContestRole(in, c, m.RoleChange[m.ContestRole]{Target: target, Old: old, New: next, BeforeOwners: before, AfterOwners: after}))
			}
		}
	}
	for _, orgAdmin := range []bool{false, true} {
		for _, old := range []m.TeamRole{"", m.TeamMember, m.TeamMaintainer, "invalid"} {
			for _, next := range []m.TeamRole{"", m.TeamMember, m.TeamMaintainer, "invalid"} {
				in, org, _, _ := policyFixture()
				if orgAdmin {
					org.Role = m.OrganizationAdmin
				}
				c := m.TeamContext{ID: uuid.New(), Organization: org, Role: m.TeamMaintainer, Privacy: m.TeamSecret}
				allowed := old != "invalid" && next != "invalid" && (old != "" || next != "") && (orgAdmin || ((old == "" || old == m.TeamMember) && (next == "" || next == m.TeamMember)))
				matrixResult(t, allowed, RequireGrantTeamRole(in, c, m.RoleChange[m.TeamRole]{Target: target, Old: old, New: next}))
			}
		}
	}
}

func TestResourceTeamDelegation(t *testing.T) {
	for _, owner := range []bool{false, true} {
		for _, sameOrg := range []bool{false, true} {
			for _, old := range []m.ProblemTeamRole{"", m.ProblemTeamRead, m.ProblemTeamWrite, m.ProblemTeamAdmin, "owner", "invalid"} {
				for _, next := range []m.ProblemTeamRole{"", m.ProblemTeamRead, m.ProblemTeamWrite, m.ProblemTeamAdmin, "owner", "invalid"} {
					in, org, p, _ := policyFixture()
					p.Organization.Role = m.OrganizationAdmin
					if owner {
						p.DirectRole = m.ProblemOwner
					}
					if !sameOrg {
						org.ID = uuid.New()
					}
					validOld := old == "" || old == m.ProblemTeamRead || old == m.ProblemTeamWrite || old == m.ProblemTeamAdmin
					validNext := next == "" || next == m.ProblemTeamRead || next == m.ProblemTeamWrite || next == m.ProblemTeamAdmin
					allowed := sameOrg && validOld && validNext && (old != "" || next != "") && (owner || (old != m.ProblemTeamAdmin && next != m.ProblemTeamAdmin))
					matrixResult(t, allowed, RequireGrantProblemTeam(in, p, org.ID, old, next))
				}
			}
			for _, old := range []m.ContestTeamRole{"", m.ContestTeamParticipant, m.ContestTeamModerator, "owner", "invalid"} {
				for _, next := range []m.ContestTeamRole{"", m.ContestTeamParticipant, m.ContestTeamModerator, "owner", "invalid"} {
					in, org, _, c := policyFixture()
					c.DirectRole = m.ContestModerator
					if owner {
						c.DirectRole = m.ContestOwner
					}
					if !sameOrg {
						org.ID = uuid.New()
					}
					validOld := old == "" || old == m.ContestTeamParticipant || old == m.ContestTeamModerator
					validNext := next == "" || next == m.ContestTeamParticipant || next == m.ContestTeamModerator
					allowed := sameOrg && validOld && validNext && (old != "" || next != "") && (owner || (old != m.ContestTeamModerator && next != m.ContestTeamModerator))
					matrixResult(t, allowed, RequireGrantContestTeam(in, c, org.ID, old, next))
				}
			}
		}
	}
}

func TestLastOwnerAndTransfer(t *testing.T) {
	in, org, p, c := policyFixture()
	org.Role = m.OrganizationOwner
	p.DirectRole = m.ProblemOwner
	c.DirectRole = m.ContestOwner
	before := m.OwnerState{Owners: []m.User{in.Actor}, PrimaryID: in.Actor.Id}
	target := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	require.ErrorIs(t, RequireLeaveOrganization(in, org, before, m.OwnerState{}), pkg.ErrConflict)
	require.ErrorIs(t, RequireOwnerState(in, m.OwnerState{}), pkg.ErrConflict)
	require.ErrorIs(t, RequireOwnerState(in, m.OwnerState{Owners: []m.User{in.Actor, in.Actor}}), pkg.ErrConflict)
	require.ErrorIs(t, RequireOwnerState(in, m.OwnerState{Owners: []m.User{in.Actor}, PrimaryID: target.Id}), pkg.ErrConflict)
	after, err := PlanOrganizationOwnerTransfer(in, org, before, target, in.Actor.Id)
	require.NoError(t, err)
	require.Equal(t, target.Id, after.PrimaryID)
	require.Equal(t, []m.User{target}, after.Owners)
	require.Equal(t, []m.User{in.Actor}, before.Owners, "planning cannot mutate loaded facts")
	require.NoError(t, RequireLeaveOrganization(in, org, m.OwnerState{Owners: []m.User{in.Actor, target}}, m.OwnerState{Owners: []m.User{target}}))
	after, err = PlanProblemOwnerTransfer(in, p, before, target, in.Actor.Id)
	require.NoError(t, err)
	require.NoError(t, RequireOwnerState(in, after))
	after, err = PlanContestOwnerTransfer(in, c, before, target, uuid.Nil)
	require.NoError(t, err)
	require.Len(t, after.Owners, 2)
	_, err = PlanContestOwnerTransfer(in, c, before, target, target.Id)
	require.ErrorIs(t, err, pkg.NoPermission)
	_, err = PlanContestOwnerTransfer(in, c, before, target, uuid.New())
	require.ErrorIs(t, err, pkg.ErrConflict)
	expiry := in.Now.Add(time.Hour)
	target.ExpiresAt = &expiry
	_, err = PlanOrganizationOwnerTransfer(in, org, before, target, in.Actor.Id)
	require.ErrorIs(t, err, pkg.NoPermission)
	require.ErrorIs(t, RequireGrantOrgRole(in, org, m.RoleChange[m.OrganizationRole]{Target: target, New: m.OrganizationOwner, BeforeOwners: before, AfterOwners: m.OwnerState{Owners: []m.User{in.Actor, target}}}), pkg.NoPermission)
	// ACL calls cannot modify self; leaving and transfer are separate operations.
	require.ErrorIs(t, RequireGrantOrgRole(in, org, m.RoleChange[m.OrganizationRole]{Target: in.Actor, Old: m.OrganizationOwner, New: m.OrganizationMember}), pkg.NoPermission)
	require.ErrorIs(t, RequireGrantProblemRole(in, p, m.RoleChange[m.ProblemRole]{Target: in.Actor, Old: m.ProblemOwner}), pkg.NoPermission)
	require.ErrorIs(t, RequireGrantContestRole(in, c, m.RoleChange[m.ContestRole]{Target: in.Actor, Old: m.ContestOwner}), pkg.NoPermission)
}

func TestOwnerChangeCannotAlterOtherOwners(t *testing.T) {
	in, org, _, _ := policyFixture()
	org.Role = m.OrganizationOwner
	target := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	other := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	before := m.OwnerState{Owners: []m.User{in.Actor, target, other}, PrimaryID: target.Id}
	change := m.RoleChange[m.OrganizationRole]{Target: target, Old: m.OrganizationOwner, New: m.OrganizationMember, BeforeOwners: before, AfterOwners: m.OwnerState{Owners: []m.User{in.Actor, other}, PrimaryID: other.Id}}
	require.NoError(t, RequireGrantOrgRole(in, org, change), "an owner can remove another owner while preserving the explicit set")
	change.AfterOwners = m.OwnerState{Owners: []m.User{in.Actor}, PrimaryID: in.Actor.Id}
	require.ErrorIs(t, RequireGrantOrgRole(in, org, change), pkg.ErrConflict)
	change.AfterOwners = m.OwnerState{Owners: []m.User{other}}
	require.ErrorIs(t, RequireGrantOrgRole(in, org, change), pkg.ErrConflict)
	change.AfterOwners = m.OwnerState{}
	require.ErrorIs(t, RequireGrantOrgRole(in, org, change), pkg.ErrConflict)
}

func TestResourceLastOwnerAndInheritedOwner(t *testing.T) {
	in, _, p, c := policyFixture()
	p.Organization.Role = m.OrganizationOwner
	c.Organization.Role = m.OrganizationOwner
	target := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	before := m.OwnerState{Owners: []m.User{target}, PrimaryID: target.Id}
	require.ErrorIs(t, RequireGrantProblemRole(in, p, m.RoleChange[m.ProblemRole]{Target: target, Old: m.ProblemOwner, New: m.ProblemViewer, BeforeOwners: before}), pkg.ErrConflict)
	require.ErrorIs(t, RequireGrantContestRole(in, c, m.RoleChange[m.ContestRole]{Target: target, Old: m.ContestOwner, BeforeOwners: before}), pkg.ErrConflict)
	next := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	after, err := PlanProblemOwnerTransfer(in, p, before, next, target.Id)
	require.NoError(t, err)
	require.Equal(t, next.Id, after.PrimaryID)
	require.Equal(t, []m.User{next}, after.Owners)
	after, err = PlanContestOwnerTransfer(in, c, before, next, target.Id)
	require.NoError(t, err)
	require.Equal(t, []m.User{next}, after.Owners)
	// An inherited owner is not counted as the surviving explicit owner.
	require.ErrorIs(t, RequireOwnerState(in, m.OwnerState{}), pkg.ErrConflict)
}
func TestInvalidAndExpiredGrantTargets(t *testing.T) {
	in, org, p, c := policyFixture()
	org.Role = m.OrganizationOwner
	p.DirectRole = m.ProblemOwner
	c.DirectRole = m.ContestOwner
	for _, role := range []string{m.UserRoleGuest, "unknown", m.UserRoleUser} {
		target := m.User{Id: uuid.New(), Role: role}
		if role == m.UserRoleUser {
			target.ExpiresAt = &in.Now
		}
		require.Error(t, RequireGrantOrgRole(in, org, m.RoleChange[m.OrganizationRole]{Target: target, New: m.OrganizationMember}))
		require.Error(t, RequireGrantProblemRole(in, p, m.RoleChange[m.ProblemRole]{Target: target, New: m.ProblemViewer}))
		require.Error(t, RequireGrantContestRole(in, c, m.RoleChange[m.ContestRole]{Target: target, New: m.ContestParticipant}))
	}
	team := m.TeamContext{ID: uuid.New(), Organization: org, Privacy: m.TeamSecret, Role: m.TeamMaintainer}
	require.ErrorIs(t, RequireGrantTeamRole(in, team, m.RoleChange[m.TeamRole]{Target: in.Actor, Old: m.TeamMember, New: m.TeamMaintainer}), pkg.NoPermission)
}
