package access

import (
	"context"
	"slices"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Field order matches the API models so transport converts them directly.
type OrganizationPermissions struct {
	Manage, ManageMembers, Delete, ListMembers, Create, Invite, Join, RequestJoin, Leave bool
}

type TeamPermissions struct{ Manage, Delete, Leave bool }

type ProblemPermissions struct{ Use, Read, Edit, Manage, Delete bool }

type ContestPermissions struct {
	ViewProblems, Submit, ListOwnSubmissions, ListSubmissions, ViewSubmissionDetails, ViewScoreboard, Manage, Delete, StartVirtual bool
}

type ContestProblemPermissions struct{ Submit, Manage, EditProblem bool }

// MemberPermissions describe one role grant: a user membership or a team link.
type MemberPermissions struct{ ChangeRole, Remove bool }

type PermissionIDs struct{ Organizations, Teams, Problems, Contests []uuid.UUID }

type Permissions struct{ l Loader }

func NewPermissions(l Loader) *Permissions { return &Permissions{l: l} }

// PermissionSet is advisory: every command authorizes again.
type PermissionSet struct {
	actor    m.User
	orgs     map[uuid.UUID]OrganizationFacts
	teams    map[uuid.UUID]TeamFacts
	problems map[uuid.UUID]ProblemFacts
	contests map[uuid.UUID]ContestFacts
}

func (p *Permissions) Load(ctx context.Context, ids PermissionIDs) (*PermissionSet, error) {
	actor, err := Actor(ctx, p.l)
	if err != nil {
		return nil, err
	}
	s := &PermissionSet{actor: actor}
	if s.orgs, err = p.l.OrganizationFacts(ctx, actor.Id, unique(ids.Organizations)); err != nil {
		return nil, err
	}
	if s.teams, err = p.l.TeamFacts(ctx, actor.Id, unique(ids.Teams)); err != nil {
		return nil, err
	}
	if s.problems, err = p.l.ProblemFacts(ctx, actor.Id, unique(ids.Problems)); err != nil {
		return nil, err
	}
	if s.contests, err = p.l.ContestFacts(ctx, actor.Id, unique(ids.Contests)); err != nil {
		return nil, err
	}
	return s, nil
}

func unique(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func lookup[F any](facts map[uuid.UUID]F, id uuid.UUID) (F, error) {
	f, ok := facts[id]
	if !ok {
		return f, pkg.ErrNotFound
	}
	return f, nil
}

func (s *PermissionSet) organization(id uuid.UUID) (OrganizationAccess, error) {
	f, err := lookup(s.orgs, id)
	return OrganizationAccess{s.actor, f}, err
}

func (s *PermissionSet) team(id uuid.UUID) (TeamAccess, error) {
	f, err := lookup(s.teams, id)
	return TeamAccess{s.actor, f}, err
}

func (s *PermissionSet) problem(id uuid.UUID) (ProblemAccess, error) {
	f, err := lookup(s.problems, id)
	return ProblemAccess{s.actor, f}, err
}

func (s *PermissionSet) contest(id uuid.UUID) (ContestAccess, error) {
	f, err := lookup(s.contests, id)
	return ContestAccess{s.actor, f}, err
}

func (s *PermissionSet) Organization(id uuid.UUID) (OrganizationPermissions, error) {
	a, err := s.organization(id)
	if err != nil {
		return OrganizationPermissions{}, err
	}
	return OrganizationPermissions{
		Manage:        a.Can(m.OrganizationManage),
		ManageMembers: a.Can(m.OrganizationManageMembers),
		Delete:        a.Can(m.OrganizationDelete),
		ListMembers:   a.Can(m.OrganizationListMembers),
		Create:        a.Can(m.OrganizationCreate),
		Invite:        a.Can(m.OrganizationInvite),
		Join:          a.Can(m.OrganizationJoin),
		RequestJoin:   a.Can(m.OrganizationRequestJoin),
		Leave:         a.Can(m.OrganizationLeave),
	}, nil
}

func (s *PermissionSet) Team(id uuid.UUID) (TeamPermissions, error) {
	a, err := s.team(id)
	if err != nil {
		return TeamPermissions{}, err
	}
	return TeamPermissions{Manage: a.Can(m.TeamManage), Delete: a.Can(m.TeamDelete), Leave: a.Can(m.TeamLeave)}, nil
}

func (s *PermissionSet) Problem(id uuid.UUID) (ProblemPermissions, error) {
	a, err := s.problem(id)
	if err != nil {
		return ProblemPermissions{}, err
	}
	return ProblemPermissions{
		Use: a.Can(m.ProblemUse), Read: a.Can(m.ProblemRead), Edit: a.Can(m.ProblemEdit), Manage: a.Can(m.ProblemManage), Delete: a.Can(m.ProblemDelete),
	}, nil
}

func (s *PermissionSet) Contest(id uuid.UUID) (ContestPermissions, error) {
	a, err := s.contest(id)
	if err != nil {
		return ContestPermissions{}, err
	}
	details := func(author uuid.UUID) bool { return a.Submission(author).Results.Tests() }
	return ContestPermissions{
		ViewProblems:          a.Can(m.ContestViewProblems),
		Submit:                a.Can(m.ContestSubmit),
		ListOwnSubmissions:    a.Can(m.ContestListOwnSubmissions),
		ListSubmissions:       a.Can(m.ContestListSubmissions),
		ViewSubmissionDetails: details(uuid.New()) || details(a.Actor.Id),
		ViewScoreboard:        a.Can(m.ContestViewScoreboard),
		Manage:                a.Can(m.ContestManage),
		Delete:                a.Can(m.ContestDelete),
		StartVirtual:          a.Can(m.ContestStartVirtual),
	}, nil
}

func (s *PermissionSet) ContestProblem(id, problem uuid.UUID) (ContestProblemPermissions, error) {
	a, err := s.contest(id)
	if err != nil {
		return ContestProblemPermissions{}, err
	}
	return ContestProblemPermissions{
		Submit: a.Can(m.ContestSubmit) && !slices.Contains(a.Facts.Blocked, problem),
		Manage: a.Can(m.ContestManage),
		// A source problem the actor cannot see has no facts and is not editable.
		EditProblem: ProblemAllows(s.actor, s.problems[problem], m.ProblemEdit),
	}, nil
}

// grant offers change_role when any other role is grantable and remove when revoking is.
func grant[R m.Role](change func(R, R) error, current R) MemberPermissions {
	var p MemberPermissions
	for _, role := range current.Ladder() {
		if R(role) != current && change(current, R(role)) == nil {
			p.ChangeRole = true
			break
		}
	}
	p.Remove = change(current, "") == nil
	return p
}

func (s *PermissionSet) OrganizationMember(id, user uuid.UUID, role m.OrganizationRole) (MemberPermissions, error) {
	a, err := s.organization(id)
	if err != nil {
		return MemberPermissions{}, err
	}
	return grant(func(from, to m.OrganizationRole) error { return a.ChangeMember(user, from, to) }, role), nil
}

func (s *PermissionSet) TeamMember(id, user uuid.UUID, role m.TeamRole) (MemberPermissions, error) {
	a, err := s.team(id)
	if err != nil {
		return MemberPermissions{}, err
	}
	return grant(func(from, to m.TeamRole) error { return a.ChangeMember(user, from, to) }, role), nil
}

func (s *PermissionSet) ProblemMember(id, user uuid.UUID, role m.ProblemRole) (MemberPermissions, error) {
	a, err := s.problem(id)
	if err != nil {
		return MemberPermissions{}, err
	}
	return grant(func(from, to m.ProblemRole) error { return a.ChangeMember(user, from, to) }, role), nil
}

func (s *PermissionSet) ContestMember(id, user uuid.UUID, role m.ContestRole) (MemberPermissions, error) {
	a, err := s.contest(id)
	if err != nil {
		return MemberPermissions{}, err
	}
	return grant(func(from, to m.ContestRole) error { return a.ChangeMember(user, from, to) }, role), nil
}

func (s *PermissionSet) ContestTeam(id uuid.UUID, role m.ContestRole) (MemberPermissions, error) {
	a, err := s.contest(id)
	if err != nil {
		return MemberPermissions{}, err
	}
	return grant(a.ChangeTeam, role), nil
}
