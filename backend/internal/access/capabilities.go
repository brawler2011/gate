package access

import (
	"context"
	"slices"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// CapabilityRequest describes the resource or relationship projected for the caller.
// Role is the current target role, never the caller's effective role.
type CapabilityRequest struct {
	Scope          string
	ID             uuid.UUID
	Kind           string // empty for resource; problem, member or link for relationships
	Target         uuid.UUID
	Role           string
	OrganizationID uuid.UUID
}
type CapabilityFacts interface {
	PolicyFacts
	LoadOrganizationFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.OrganizationContext, error)
	LoadCapabilityUsers(context.Context, []uuid.UUID) (map[uuid.UUID]m.User, error)
}
type Capabilities struct{ facts CapabilityFacts }

func NewCapabilities(facts CapabilityFacts) *Capabilities { return &Capabilities{facts: facts} }

// Project loads each scope once for the complete response, independent of page size.
// Errors abort the response; capabilities are advisory, writes still authorize.
func (uc *Capabilities) Project(ctx context.Context, requests []CapabilityRequest) ([][]string, error) {
	in := m.PolicyInput{Actor: m.UserFromContext(ctx), Now: m.ReadTime(ctx)}
	if err := validateInput(in); err != nil {
		return nil, err
	}
	ids := map[string][]uuid.UUID{}
	var targets []uuid.UUID
	for _, r := range requests {
		if !slices.Contains(ids[r.Scope], r.ID) {
			ids[r.Scope] = append(ids[r.Scope], r.ID)
		}
		if r.Kind == "member" {
			targets = append(targets, r.Target)
		}
		if r.Kind == "link" && !slices.Contains(ids["team"], r.Target) {
			ids["team"] = append(ids["team"], r.Target)
		}
	}
	orgs, err := uc.facts.LoadOrganizationFacts(ctx, in.Actor.Id, ids["organization"])
	if err != nil {
		return nil, err
	}
	teams, err := uc.facts.LoadTeamFacts(ctx, in.Actor.Id, ids["team"])
	if err != nil {
		return nil, err
	}
	problems, err := uc.facts.LoadProblemFacts(ctx, in.Actor.Id, ids["problem"])
	if err != nil {
		return nil, err
	}
	contests, err := uc.facts.LoadContestFacts(ctx, in.Actor.Id, ids["contest"])
	if err != nil {
		return nil, err
	}
	users, err := uc.facts.LoadCapabilityUsers(ctx, targets)
	if err != nil {
		return nil, err
	}
	result := make([][]string, len(requests))
	for i, r := range requests {
		if r.Kind == "link" {
			team, ok := teams[r.Target]
			if !ok {
				return nil, pkg.ErrNotFound
			}
			r.OrganizationID = team.Organization.ID
		}
		caps := []string{}
		add := func(name string, err error) {
			if err == nil {
				caps = append(caps, name)
			}
		}
		switch r.Scope {
		case "organization":
			c, ok := orgs[r.ID]
			if !ok {
				return nil, pkg.ErrNotFound
			}
			if r.Kind == "member" {
				u, ok := users[r.Target]
				if !ok {
					return nil, pkg.ErrNotFound
				}
				caps = orgMemberCapabilities(in, c, r, u)
			} else {
				for _, a := range []struct {
					name   string
					action m.OrgAction
				}{{"view", m.ActionViewOrganization}, {"manage", m.ActionManageOrganization}, {"delete", m.ActionDeleteOrganization}, {"manage_members", m.ActionManageOrganizationMembers}, {"list_members", m.ActionListOrganizationMembers}, {"create_team", m.ActionCreateTeam}, {"create_problem", m.ActionCreateProblem}, {"create_contest", m.ActionCreateContest}, {"join", m.ActionJoinOrganization}, {"request_join", m.ActionRequestOrganizationMembership}} {
					add(a.name, RequireOrganization(in, c, a.action))
				}
				state := ownerState(c.Owners, uuid.Nil)
				add("leave", RequireLeaveOrganization(in, c, state, afterRemoval(state, in.Actor.Id)))
				if containsOwner(state, in.Actor.Id) && RequireOwnerState(in, state) == nil {
					add("transfer_owner", RequireOrganization(in, c, m.ActionTransferOrganizationOwner))
				}
			}
		case "team":
			c, ok := teams[r.ID]
			if !ok {
				return nil, pkg.ErrNotFound
			}
			if r.Kind == "member" {
				u, ok := users[r.Target]
				if !ok {
					return nil, pkg.ErrNotFound
				}
				for _, role := range []m.TeamRole{m.TeamMember, m.TeamMaintainer} {
					if string(role) == r.Role {
						continue
					}
					if RequireGrantTeamRole(in, c, m.RoleChange[m.TeamRole]{Target: u, Old: m.TeamRole(r.Role), New: role}) == nil {
						caps = append(caps, "change_role")
						break
					}
				}
				add("remove", RequireGrantTeamRole(in, c, m.RoleChange[m.TeamRole]{Target: u, Old: m.TeamRole(r.Role)}))
			} else {
				for _, a := range []struct {
					name   string
					action m.TeamAction
				}{{"view", m.ActionViewTeam}, {"edit", m.ActionUpdateTeam}, {"delete", m.ActionDeleteTeam}, {"manage_members", m.ActionManageTeamMembers}, {"list_members", m.ActionListTeamMembers}, {"list_resources", m.ActionListTeamResources}, {"leave", m.ActionLeaveTeam}} {
					add(a.name, RequireTeam(in, c, a.action))
				}
			}
		case "problem":
			c, ok := problems[r.ID]
			if !ok {
				return nil, pkg.ErrNotFound
			}
			switch r.Kind {
			case "member":
				u, ok := users[r.Target]
				if !ok {
					return nil, pkg.ErrNotFound
				}
				caps = problemMemberCapabilities(in, c, r, u)
			case "link":
				for _, role := range []m.ProblemTeamRole{m.ProblemTeamRead, m.ProblemTeamWrite, m.ProblemTeamAdmin} {
					if string(role) == r.Role {
						continue
					}
					if RequireGrantProblemTeam(in, c, r.OrganizationID, m.ProblemTeamRole(r.Role), role) == nil {
						caps = append(caps, "change_role")
						break
					}
				}
				add("remove", RequireGrantProblemTeam(in, c, r.OrganizationID, m.ProblemTeamRole(r.Role), ""))
			default:
				for _, a := range []struct {
					name   string
					action m.ProblemAction
				}{{"view", m.ActionViewProblem}, {"use", m.ActionUseProblem}, {"edit", m.ActionEditProblem}, {"manage", m.ActionAdminProblem}, {"delete", m.ActionDeleteProblem}, {"manage_members", m.ActionManageProblemMembers}, {"list_members", m.ActionListProblemMembers}} {
					add(a.name, RequireProblem(in, c, a.action))
				}
				state := ownerState(c.Owners, c.OwnerID)
				if containsOwner(state, in.Actor.Id) && RequireOwnerState(in, state) == nil {
					add("transfer_owner", RequireProblem(in, c, m.ActionTransferProblemOwner))
				}
			}
		case "contest":
			c, ok := contests[r.ID]
			if !ok {
				return nil, pkg.ErrNotFound
			}
			if err := ApplyContestLifecycle(in, &c); err != nil {
				return nil, err
			}
			switch r.Kind {
			case "problem":
				add("view_statements", RequireContest(in, c, m.ActionGetContestProblem))
				if !slices.Contains(c.BlockedProblems, r.Target) {
					add("submit", RequireContest(in, c, m.ActionCreateSubmission))
				}
				for _, name := range []string{"relink_package", "remove", "reorder"} {
					add(name, RequireContest(in, c, m.ActionManageContest))
				}
			case "member":
				u, ok := users[r.Target]
				if !ok {
					return nil, pkg.ErrNotFound
				}
				caps = contestMemberCapabilities(in, c, r, u)
			case "link":
				for _, role := range []m.ContestTeamRole{m.ContestTeamParticipant, m.ContestTeamModerator} {
					if string(role) == r.Role {
						continue
					}
					if RequireGrantContestTeam(in, c, r.OrganizationID, m.ContestTeamRole(r.Role), role) == nil {
						caps = append(caps, "change_role")
						break
					}
				}
				add("remove", RequireGrantContestTeam(in, c, r.OrganizationID, m.ContestTeamRole(r.Role), ""))
			default:
				for _, a := range []struct {
					name   string
					action m.ContestAction
				}{{"view", m.ActionGetContest}, {"view_statements", m.ActionGetContestProblem}, {"edit", m.ActionUpdateContest}, {"manage", m.ActionManageContest}, {"manage_members", m.ActionAdminContest}, {"list_members", m.ActionListContestMembers}, {"delete", m.ActionDeleteContest}, {"view_scoreboard", m.ActionGetMonitor}, {"view_live_scoreboard", m.ActionViewLiveScoreboard}, {"list_submissions", m.ActionListUsersSubmissions}, {"list_own_submissions", m.ActionListOwnSubmissions}, {"submit", m.ActionCreateSubmission}} {
					add(a.name, RequireContest(in, c, a.action))
				}
				state := ownerState(c.Owners, c.OwnerID)
				if containsOwner(state, in.Actor.Id) && RequireOwnerState(in, state) == nil {
					add("transfer_owner", RequireContest(in, c, m.ActionTransferContestOwner))
				}
				c.Submission = m.SubmissionContext{ContestID: c.ID, OwnerID: uuid.New()}
				add("view_other_submission", RequireContest(in, c, m.ActionGetOtherUserSubmission))
				otherDetails := RequireContest(in, c, m.ActionGetSubmissionDetails)
				c.Submission.OwnerID = in.Actor.Id
				add("view_own_submission", RequireContest(in, c, m.ActionGetOwnSubmission))
				ownDetails := RequireContest(in, c, m.ActionGetSubmissionDetails)
				if otherDetails == nil || ownDetails == nil {
					caps = append(caps, "view_submission_details")
				}
			}
		default:
			return nil, pkg.NoPermission
		}
		slices.Sort(caps)
		result[i] = caps
	}
	return result, nil
}

func ownerState(owners []m.User, primary uuid.UUID) m.OwnerState {
	return m.OwnerState{Owners: owners, PrimaryID: primary}
}
func afterRemoval(before m.OwnerState, target uuid.UUID) m.OwnerState {
	after := m.OwnerState{PrimaryID: before.PrimaryID}
	for _, u := range before.Owners {
		if u.Id != target {
			after.Owners = append(after.Owners, u)
		}
	}
	return after
}

// Relationship controls probe valid target-specific transitions, including the
// full final owner set. They never expose a blanket right to assign any role.
func memberCapabilities[R ~string](r CapabilityRequest, u m.User, before m.OwnerState, roles []R, grant func(m.RoleChange[R]) error, transfer func() error) []string {
	caps := []string{}
	for _, role := range roles {
		if string(role) == r.Role {
			continue
		}
		after := before
		if r.Role == "owner" {
			after = afterRemoval(before, u.Id)
		}
		if grant(m.RoleChange[R]{Target: u, Old: R(r.Role), New: role, BeforeOwners: before, AfterOwners: after}) == nil {
			caps = append(caps, "change_role")
			break
		}
	}
	if grant(m.RoleChange[R]{Target: u, Old: R(r.Role), BeforeOwners: before, AfterOwners: afterRemoval(before, u.Id)}) == nil {
		caps = append(caps, "remove")
	}
	if transfer() != nil {
		return caps
	}
	return append(caps, "transfer_owner")
}
func orgMemberCapabilities(in m.PolicyInput, c m.OrganizationContext, r CapabilityRequest, u m.User) []string {
	before := ownerState(c.Owners, uuid.Nil)
	return memberCapabilities(r, u, before, []m.OrganizationRole{m.OrganizationMember, m.OrganizationAdmin}, func(change m.RoleChange[m.OrganizationRole]) error { return RequireGrantOrgRole(in, c, change) }, func() error { _, err := PlanOrganizationOwnerTransfer(in, c, before, u, in.Actor.Id); return err })
}
func problemMemberCapabilities(in m.PolicyInput, c m.ProblemContext, r CapabilityRequest, u m.User) []string {
	before := ownerState(c.Owners, c.OwnerID)
	return memberCapabilities(r, u, before, []m.ProblemRole{m.ProblemViewer, m.ProblemModerator}, func(change m.RoleChange[m.ProblemRole]) error { return RequireGrantProblemRole(in, c, change) }, func() error {
		if c.OwnerID != in.Actor.Id && c.DirectRole != m.ProblemOwner {
			return pkg.NoPermission
		}
		_, err := PlanProblemOwnerTransfer(in, c, before, u, in.Actor.Id)
		return err
	})
}
func contestMemberCapabilities(in m.PolicyInput, c m.ContestContext, r CapabilityRequest, u m.User) []string {
	before := ownerState(c.Owners, c.OwnerID)
	return memberCapabilities(r, u, before, []m.ContestRole{m.ContestParticipant, m.ContestModerator}, func(change m.RoleChange[m.ContestRole]) error { return RequireGrantContestRole(in, c, change) }, func() error {
		if c.OwnerID != in.Actor.Id && c.DirectRole != m.ContestOwner {
			return pkg.NoPermission
		}
		_, err := PlanContestOwnerTransfer(in, c, before, u, in.Actor.Id)
		return err
	})
}
