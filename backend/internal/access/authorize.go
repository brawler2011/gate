package access

import (
	"context"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Actor reloads the caller so role and expiry changes apply to the next request; guests stay guests.
func Actor(ctx context.Context, l Loader) (m.User, error) {
	id := m.UserFromContext(ctx).Id
	if id == uuid.Nil {
		return m.Guest, nil
	}
	actor, err := l.Actor(ctx, id)
	if err != nil {
		return m.User{}, pkg.OnNotFound(err, pkg.ErrUnauthenticated, "")
	}
	return actor, nil
}

type OrganizationAccess struct {
	Actor m.User
	Facts OrganizationFacts
}

func (a OrganizationAccess) Can(action m.OrganizationAction) bool {
	return OrganizationAllows(a.Actor, a.Facts, action)
}

type TeamAccess struct {
	Actor m.User
	Facts TeamFacts
}

func (a TeamAccess) Can(action m.TeamAction) bool { return TeamAllows(a.Actor, a.Facts, action) }

type ProblemAccess struct {
	Actor m.User
	Facts ProblemFacts
}

func (a ProblemAccess) Can(action m.ProblemAction) bool {
	return ProblemAllows(a.Actor, a.Facts, action)
}

type ContestAccess struct {
	Actor m.User
	Facts ContestFacts
}

func (a ContestAccess) Can(action m.ContestAction) bool {
	return ContestAllows(a.Actor, a.Facts, action)
}

// Role is the caller's effective contest role; site administrators hold the top one.
func (a ContestAccess) Role() m.ContestRole {
	if r := rank(a.Actor, a.Facts.Role); r > 0 {
		return m.ContestRole(a.Facts.Role.Ladder()[r-1])
	}
	return ""
}

// Participates reports whether the caller competes as a participant; upsolvers only see their own submissions.
func (a ContestAccess) Participates() bool { return newContestState(a.Actor, a.Facts).participant }

func (a ContestAccess) CanSubmission(author uuid.UUID, action m.SubmissionAction) bool {
	return SubmissionAllows(a.Actor, a.Facts, author, action)
}

// Organization authorizes action and returns the access for further checks on the same read.
func Organization(ctx context.Context, l Loader, id uuid.UUID, action m.OrganizationAction) (OrganizationAccess, error) {
	actor, facts, err := load(ctx, l, id, l.OrganizationFacts)
	a := OrganizationAccess{actor, facts}
	if err != nil {
		return a, err
	}
	return a, decide(actor, a.Can(action), a.Can(m.OrganizationView))
}

func Team(ctx context.Context, l Loader, id uuid.UUID, action m.TeamAction) (TeamAccess, error) {
	actor, facts, err := load(ctx, l, id, l.TeamFacts)
	a := TeamAccess{actor, facts}
	if err != nil {
		return a, err
	}
	return a, decide(actor, a.Can(action), a.Can(m.TeamView))
}

func Problem(ctx context.Context, l Loader, id uuid.UUID, action m.ProblemAction) (ProblemAccess, error) {
	actor, facts, err := load(ctx, l, id, l.ProblemFacts)
	a := ProblemAccess{actor, facts}
	if err != nil {
		return a, err
	}
	return a, decide(actor, a.Can(action), a.Can(m.ProblemView))
}

func Contest(ctx context.Context, l Loader, id uuid.UUID, action m.ContestAction) (ContestAccess, error) {
	actor, facts, err := load(ctx, l, id, l.ContestFacts)
	a := ContestAccess{actor, facts}
	if err != nil {
		return a, err
	}
	return a, decide(actor, a.Can(action), a.Can(m.ContestView))
}

func load[F any](ctx context.Context, l Loader, id uuid.UUID, read func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]F, error)) (m.User, F, error) {
	var facts F
	actor, err := Actor(ctx, l)
	if err != nil {
		return actor, facts, err
	}
	all, err := read(ctx, actor.Id, []uuid.UUID{id})
	if err != nil {
		return actor, facts, err
	}
	facts, ok := all[id]
	if !ok {
		return actor, facts, pkg.ErrNotFound
	}
	return actor, facts, nil
}

// A denied guest must log in; a resource the caller cannot see is hidden, a visible one is forbidden.
func decide(actor m.User, allowed, visible bool) error {
	switch {
	case allowed:
		return nil
	case actor.IsGuest():
		return pkg.ErrUnauthenticated
	case visible:
		return pkg.ErrForbidden
	default:
		return pkg.ErrNotFound
	}
}

// ChangeMember authorizes setting target's role; next == "" removes it.
func (a OrganizationAccess) ChangeMember(target uuid.UUID, current, next m.OrganizationRole) error {
	return changeMember(a.Actor, target, a.Can(m.OrganizationManageMembers), a.Facts.Role, current, next)
}

func (a TeamAccess) ChangeMember(target uuid.UUID, current, next m.TeamRole) error {
	return changeMember(a.Actor, target, a.Can(m.TeamManage), a.Facts.Role, current, next)
}

func (a ProblemAccess) ChangeMember(target uuid.UUID, current, next m.ProblemRole) error {
	return changeMember(a.Actor, target, a.Can(m.ProblemManage), a.Facts.Role, current, next)
}

func (a ContestAccess) ChangeMember(target uuid.UUID, current, next m.ContestRole) error {
	return changeMember(a.Actor, target, a.Can(m.ContestManage), a.Facts.Role, current, next)
}

// ChangeTeam authorizes a team grant on the resource; a team is never the caller.
func (a ProblemAccess) ChangeTeam(current, next m.ProblemRole) error {
	return changeMember(a.Actor, uuid.Nil, a.Can(m.ProblemManage), a.Facts.Role, current, next)
}

func (a ContestAccess) ChangeTeam(current, next m.ContestRole) error {
	return changeMember(a.Actor, uuid.Nil, a.Can(m.ContestManage), a.Facts.Role, current, next)
}

func changeMember[R m.Role](actor m.User, target uuid.UUID, manage bool, role, current, next R) error {
	if target != actor.Id && !manage {
		return pkg.ErrForbidden
	}
	if !ChangeRole(actor, role, target, current, next) {
		return pkg.ErrForbidden
	}
	return nil
}

// Visibility tells list queries whose grants to join; site administrators see everything.
func Visibility(ctx context.Context, l Loader) (viewer uuid.UUID, all bool, err error) {
	actor, err := Actor(ctx, l)
	return actor.Id, actor.IsAdmin(), err
}
