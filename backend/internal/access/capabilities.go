package access

import (
	"context"
	"slices"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// CapabilityRequest names a resource, or a row of it (Kind problem, member or link) with the row's current Role.
type CapabilityRequest struct {
	Scope  string
	ID     uuid.UUID
	Kind   string
	Target uuid.UUID
	Role   string
}

type Capabilities struct{ l Loader }

func NewCapabilities(l Loader) *Capabilities { return &Capabilities{l: l} }

type named[A any] struct {
	name   string
	action A
}

// Project lists what the caller may do; it is advisory and every command authorizes again.
func (c *Capabilities) Project(ctx context.Context, requests []CapabilityRequest) ([][]string, error) {
	actor, err := Actor(ctx, c.l)
	if err != nil {
		return nil, err
	}
	ids := map[string][]uuid.UUID{}
	for _, r := range requests {
		if !slices.Contains(ids[r.Scope], r.ID) {
			ids[r.Scope] = append(ids[r.Scope], r.ID)
		}
	}
	orgs, err := c.l.OrganizationFacts(ctx, actor.Id, ids["organization"])
	if err != nil {
		return nil, err
	}
	teams, err := c.l.TeamFacts(ctx, actor.Id, ids["team"])
	if err != nil {
		return nil, err
	}
	problems, err := c.l.ProblemFacts(ctx, actor.Id, ids["problem"])
	if err != nil {
		return nil, err
	}
	contests, err := c.l.ContestFacts(ctx, actor.Id, ids["contest"])
	if err != nil {
		return nil, err
	}
	result := make([][]string, len(requests))
	for i, r := range requests {
		var caps []string
		var ok bool
		switch r.Scope {
		case "organization":
			var f OrganizationFacts
			if f, ok = orgs[r.ID]; ok {
				caps = organizationCapabilities(OrganizationAccess{actor, f}, r)
			}
		case "team":
			var f TeamFacts
			if f, ok = teams[r.ID]; ok {
				caps = teamCapabilities(TeamAccess{actor, f}, r)
			}
		case "problem":
			var f ProblemFacts
			if f, ok = problems[r.ID]; ok {
				caps = problemCapabilities(ProblemAccess{actor, f}, r)
			}
		case "contest":
			var f ContestFacts
			if f, ok = contests[r.ID]; ok {
				caps = contestCapabilities(ContestAccess{actor, f}, r)
			}
		}
		if !ok {
			return nil, pkg.ErrNotFound
		}
		slices.Sort(caps)
		result[i] = caps
	}
	return result, nil
}

func allowed[A any](can func(A) bool, actions ...named[A]) []string {
	caps := []string{}
	for _, a := range actions {
		if can(a.action) {
			caps = append(caps, a.name)
		}
	}
	return caps
}

// memberCapabilities offers change_role when any other role is grantable and remove when revoking is.
func memberCapabilities[R m.Role](change func(R, R) error, current R) []string {
	caps := []string{}
	var zero R
	for _, role := range zero.Ladder() {
		if R(role) != current && change(current, R(role)) == nil {
			caps = append(caps, "change_role")
			break
		}
	}
	if change(current, "") == nil {
		caps = append(caps, "remove")
	}
	return caps
}

func organizationCapabilities(a OrganizationAccess, r CapabilityRequest) []string {
	if r.Kind == "member" {
		return memberCapabilities(func(from, to m.OrganizationRole) error { return a.ChangeMember(r.Target, from, to) }, m.OrganizationRole(r.Role))
	}
	return allowed(a.Can,
		named[m.OrganizationAction]{"view", m.OrganizationView}, named[m.OrganizationAction]{"manage", m.OrganizationManage},
		named[m.OrganizationAction]{"delete", m.OrganizationDelete}, named[m.OrganizationAction]{"manage_members", m.OrganizationManageMembers},
		named[m.OrganizationAction]{"list_members", m.OrganizationListMembers}, named[m.OrganizationAction]{"create_team", m.OrganizationManage},
		named[m.OrganizationAction]{"create_problem", m.OrganizationManage}, named[m.OrganizationAction]{"create_contest", m.OrganizationManage},
		named[m.OrganizationAction]{"join", m.OrganizationJoin}, named[m.OrganizationAction]{"request_join", m.OrganizationRequestJoin},
		named[m.OrganizationAction]{"leave", m.OrganizationLeave})
}

func teamCapabilities(a TeamAccess, r CapabilityRequest) []string {
	if r.Kind == "member" {
		return memberCapabilities(func(from, to m.TeamRole) error { return a.ChangeMember(r.Target, from, to) }, m.TeamRole(r.Role))
	}
	return allowed(a.Can,
		named[m.TeamAction]{"view", m.TeamView}, named[m.TeamAction]{"edit", m.TeamManage}, named[m.TeamAction]{"delete", m.TeamDelete},
		named[m.TeamAction]{"manage_members", m.TeamManage}, named[m.TeamAction]{"list_members", m.TeamView},
		named[m.TeamAction]{"list_resources", m.TeamView}, named[m.TeamAction]{"leave", m.TeamLeave})
}

func problemCapabilities(a ProblemAccess, r CapabilityRequest) []string {
	switch r.Kind {
	case "member":
		return memberCapabilities(func(from, to m.ProblemRole) error { return a.ChangeMember(r.Target, from, to) }, m.ProblemRole(r.Role))
	case "link":
		return memberCapabilities(a.ChangeTeam, m.ProblemRole(r.Role))
	}
	return allowed(a.Can,
		named[m.ProblemAction]{"view", m.ProblemView}, named[m.ProblemAction]{"use", m.ProblemUse}, named[m.ProblemAction]{"edit", m.ProblemEdit},
		named[m.ProblemAction]{"manage", m.ProblemManage}, named[m.ProblemAction]{"delete", m.ProblemDelete},
		named[m.ProblemAction]{"manage_members", m.ProblemManage}, named[m.ProblemAction]{"list_members", m.ProblemManage})
}

func contestCapabilities(a ContestAccess, r CapabilityRequest) []string {
	switch r.Kind {
	case "member":
		return memberCapabilities(func(from, to m.ContestRole) error { return a.ChangeMember(r.Target, from, to) }, m.ContestRole(r.Role))
	case "link":
		return memberCapabilities(a.ChangeTeam, m.ContestRole(r.Role))
	case "problem":
		caps := allowed(a.Can,
			named[m.ContestAction]{"view_statements", m.ContestViewProblems}, named[m.ContestAction]{"relink_package", m.ContestManage},
			named[m.ContestAction]{"remove", m.ContestManage}, named[m.ContestAction]{"reorder", m.ContestManage})
		if a.Can(m.ContestSubmit) && !slices.Contains(a.Facts.Blocked, r.Target) {
			caps = append(caps, "submit")
		}
		return caps
	}
	caps := allowed(a.Can,
		named[m.ContestAction]{"view", m.ContestView}, named[m.ContestAction]{"view_statements", m.ContestViewProblems},
		named[m.ContestAction]{"edit", m.ContestManage}, named[m.ContestAction]{"manage", m.ContestManage},
		named[m.ContestAction]{"manage_members", m.ContestManage}, named[m.ContestAction]{"list_members", m.ContestManage},
		named[m.ContestAction]{"delete", m.ContestDelete}, named[m.ContestAction]{"view_scoreboard", m.ContestViewScoreboard},
		named[m.ContestAction]{"view_live_scoreboard", m.ContestManage}, named[m.ContestAction]{"list_submissions", m.ContestListSubmissions},
		named[m.ContestAction]{"list_own_submissions", m.ContestListOwnSubmissions}, named[m.ContestAction]{"submit", m.ContestSubmit})
	other := uuid.New()
	if a.CanSubmission(other, m.SubmissionView) {
		caps = append(caps, "view_other_submission")
	}
	if a.CanSubmission(a.Actor.Id, m.SubmissionView) {
		caps = append(caps, "view_own_submission")
	}
	if a.CanSubmission(other, m.SubmissionViewDetails) || a.CanSubmission(a.Actor.Id, m.SubmissionViewDetails) {
		caps = append(caps, "view_submission_details")
	}
	return caps
}
