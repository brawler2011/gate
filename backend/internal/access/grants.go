package access

import (
	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// delegation checks both ends: checking only the requested role permits removing
// an owner by demotion. Self changes use leave/transfer, never an ACL bypass.
func delegation(in m.PolicyInput, target m.User, actor, old, next, owner int) error {
	if target.Id == uuid.Nil || target.Id == in.Actor.Id || old < 0 || next < 0 || (old == 0 && next == 0) {
		return pkg.NoPermission
	}
	if target.Role != m.UserRoleUser && target.Role != m.UserRoleAdmin {
		return pkg.NoPermission
	}
	if next > 0 {
		if err := validateInput(m.PolicyInput{Actor: target, Now: in.Now}); err != nil {
			return err
		}
	}
	if actor == owner {
		return nil
	}
	return decision(old < actor && next < actor)
}
func RequireGrantOrgRole(in m.PolicyInput, c m.OrganizationContext, change m.RoleChange[m.OrganizationRole]) error {
	if err := RequireOrganization(in, c, m.ActionManageOrganizationMembers); err != nil {
		return err
	}
	r, err := organizationRank(in, c)
	if err != nil {
		return err
	}
	old, next := orgRank(change.Old), orgRank(change.New)
	if err := delegation(in, change.Target, r, old, next, 3); err != nil {
		return err
	}
	if (old == 3 || next == 3) && !containsOwner(change.BeforeOwners, in.Actor.Id) {
		return pkg.ErrConflict
	}
	return checkOwnerChange(in, change.Target, old == 3, next == 3, change.BeforeOwners, change.AfterOwners)
}
func RequireGrantTeamRole(in m.PolicyInput, c m.TeamContext, change m.RoleChange[m.TeamRole]) error {
	if err := RequireTeam(in, c, m.ActionManageTeamMembers); err != nil {
		return err
	}
	r, err := teamPolicyRank(in, c)
	if err != nil {
		return err
	}
	return delegation(in, change.Target, r, teamRank(change.Old), teamRank(change.New), 3)
}
func RequireGrantProblemRole(in m.PolicyInput, c m.ProblemContext, change m.RoleChange[m.ProblemRole]) error {
	if err := RequireProblem(in, c, m.ActionManageProblemMembers); err != nil {
		return err
	}
	if change.Old == m.ProblemAdmin || change.New == m.ProblemAdmin {
		return pkg.NoPermission
	}
	r, err := problemPolicyRank(in, c)
	if err != nil {
		return err
	}
	old, next := problemRank(change.Old), problemRank(change.New)
	if err := delegation(in, change.Target, r, old, next, 4); err != nil {
		return err
	}
	return checkOwnerChange(in, change.Target, old == 4, next == 4, change.BeforeOwners, change.AfterOwners)
}
func RequireGrantContestRole(in m.PolicyInput, c m.ContestContext, change m.RoleChange[m.ContestRole]) error {
	if err := RequireContest(in, c, m.ActionAdminContest); err != nil {
		return err
	}
	r, err := contestPolicyRank(in, c)
	if err != nil {
		return err
	}
	old, next := contestRank(change.Old), contestRank(change.New)
	if err := delegation(in, change.Target, r, old, next, 3); err != nil {
		return err
	}
	return checkOwnerChange(in, change.Target, old == 3, next == 3, change.BeforeOwners, change.AfterOwners)
}

// Resource-team ACLs require resource administration and the same organization;
// editing a team never authorizes editing its resource links.
func RequireGrantProblemTeam(in m.PolicyInput, c m.ProblemContext, teamOrganizationID uuid.UUID, old, next m.ProblemTeamRole) error {
	if err := RequireProblem(in, c, m.ActionManageProblemMembers); err != nil {
		return err
	}
	r, err := problemPolicyRank(in, c)
	if err != nil {
		return err
	}
	return teamDelegation(teamOrganizationID == c.Organization.ID, r, optionalProblemTeamRank(old), optionalProblemTeamRank(next), 4)
}
func RequireGrantContestTeam(in m.PolicyInput, c m.ContestContext, teamOrganizationID uuid.UUID, old, next m.ContestTeamRole) error {
	if err := RequireContest(in, c, m.ActionAdminContest); err != nil {
		return err
	}
	r, err := contestPolicyRank(in, c)
	if err != nil {
		return err
	}
	return teamDelegation(teamOrganizationID == c.Organization.ID, r, optionalContestTeamRank(old), optionalContestTeamRank(next), 3)
}
func optionalProblemTeamRank(r m.ProblemTeamRole) int {
	if r == "" {
		return 0
	}
	return problemTeamRank(r)
}
func optionalContestTeamRank(r m.ContestTeamRole) int {
	if r == "" {
		return 0
	}
	return contestTeamRank(r)
}
func teamDelegation(sameOrg bool, actor, old, next, owner int) error {
	if !sameOrg || old < 0 || next < 0 || (old == 0 && next == 0) {
		return pkg.NoPermission
	}
	return decision(actor == owner || (old < actor && next < actor))
}

func containsOwner(state m.OwnerState, id uuid.UUID) bool {
	for _, u := range state.Owners {
		if u.Id == id {
			return true
		}
	}
	return false
}

func validOwner(in m.PolicyInput, u m.User) bool {
	return u.Id != uuid.Nil && (u.IsUser() || u.IsAdmin()) && u.ExpiresAt == nil && !in.Now.IsZero()
}
func ownerIDs(in m.PolicyInput, state m.OwnerState) (map[uuid.UUID]struct{}, error) {
	ids := make(map[uuid.UUID]struct{}, len(state.Owners))
	for _, u := range state.Owners {
		if !validOwner(in, u) {
			return nil, pkg.ErrConflict
		}
		if _, duplicate := ids[u.Id]; duplicate {
			return nil, pkg.ErrConflict
		}
		ids[u.Id] = struct{}{}
	}
	if len(ids) == 0 {
		return nil, pkg.ErrConflict
	}
	if state.PrimaryID != uuid.Nil {
		if _, ok := ids[state.PrimaryID]; !ok {
			return nil, pkg.ErrConflict
		}
	}
	return ids, nil
}

// RequireOwnerState is also the in-memory invariant for lifecycle and bulk
// mutations. Repositories must check the final owner set in a short serializable
// mutation transaction; this function performs no I/O and takes no locks.
func RequireOwnerState(in m.PolicyInput, state m.OwnerState) error {
	if err := validateInput(in); err != nil {
		return err
	}
	_, err := ownerIDs(in, state)
	return err
}
func checkOwnerChange(in m.PolicyInput, target m.User, wasOwner, becomesOwner bool, before, after m.OwnerState) error {
	if !wasOwner && !becomesOwner {
		return nil
	}
	if becomesOwner && !validOwner(in, target) {
		return pkg.NoPermission
	}
	previous, err := ownerIDs(in, before)
	if err != nil {
		return err
	}
	next, err := ownerIDs(in, after)
	if err != nil {
		return err
	}
	_, existed := previous[target.Id]
	_, exists := next[target.Id]
	if existed != wasOwner || exists != becomesOwner {
		return pkg.ErrConflict
	}
	delete(previous, target.Id)
	delete(next, target.Id)
	if len(previous) != len(next) {
		return pkg.ErrConflict
	}
	for id := range previous {
		if _, ok := next[id]; !ok {
			return pkg.ErrConflict
		}
	}
	// A primary owner cannot disappear without assigning its replacement. Do not
	// silently drop the primary-owner representation during a grant change.
	if before.PrimaryID != uuid.Nil && after.PrimaryID == uuid.Nil {
		return pkg.ErrConflict
	}
	return nil
}
func RequireLeaveOrganization(in m.PolicyInput, c m.OrganizationContext, before, after m.OwnerState) error {
	if err := RequireOrganization(in, c, m.ActionLeaveOrganization); err != nil {
		return err
	}
	return checkOwnerChange(in, in.Actor, c.Role == m.OrganizationOwner, false, before, after)
}

// PlanOwnerTransfer constructs the entire atomic transition. Scope wrappers
// establish real ownership first. removeID may identify an explicit owner to
// remove after adding the recipient; nil retains all previous owners.
func planOwnerTransfer(in m.PolicyInput, before m.OwnerState, target m.User, removeID uuid.UUID) (m.OwnerState, error) {
	ids, err := ownerIDs(in, before)
	if err != nil {
		return m.OwnerState{}, err
	}
	if !validOwner(in, target) || target.Id == removeID {
		return m.OwnerState{}, pkg.NoPermission
	}
	if removeID != uuid.Nil {
		if _, ok := ids[removeID]; !ok {
			return m.OwnerState{}, pkg.ErrConflict
		}
	}
	after := m.OwnerState{PrimaryID: before.PrimaryID}
	for _, u := range before.Owners {
		if u.Id != removeID {
			after.Owners = append(after.Owners, u)
		}
	}
	if _, exists := ids[target.Id]; !exists {
		after.Owners = append(after.Owners, target)
	}
	if before.PrimaryID != uuid.Nil {
		after.PrimaryID = target.Id
	}
	return after, nil
}
func PlanOrganizationOwnerTransfer(in m.PolicyInput, c m.OrganizationContext, before m.OwnerState, target m.User, removeID uuid.UUID) (m.OwnerState, error) {
	if err := RequireOrganization(in, c, m.ActionTransferOrganizationOwner); err != nil {
		return m.OwnerState{}, err
	}
	if !containsOwner(before, in.Actor.Id) {
		return m.OwnerState{}, pkg.ErrConflict
	}
	return planOwnerTransfer(in, before, target, removeID)
}
func PlanProblemOwnerTransfer(in m.PolicyInput, c m.ProblemContext, before m.OwnerState, target m.User, removeID uuid.UUID) (m.OwnerState, error) {
	if err := RequireProblem(in, c, m.ActionTransferProblemOwner); err != nil {
		return m.OwnerState{}, err
	}
	return planOwnerTransfer(in, before, target, removeID)
}
func PlanContestOwnerTransfer(in m.PolicyInput, c m.ContestContext, before m.OwnerState, target m.User, removeID uuid.UUID) (m.OwnerState, error) {
	if err := RequireContest(in, c, m.ActionTransferContestOwner); err != nil {
		return m.OwnerState{}, err
	}
	return planOwnerTransfer(in, before, target, removeID)
}
