package access

import (
	"context"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Checkers read the caller from ctx and are implemented over the pool and inside transactions.
type ContestChecker interface {
	CanContest(context.Context, uuid.UUID, m.ContestAction) (bool, error)
}
type ProblemChecker interface {
	CanProblem(context.Context, uuid.UUID, m.ProblemAction) (bool, error)
}
type OrganizationChecker interface {
	CanOrganization(context.Context, uuid.UUID, m.OrgAction) (bool, error)
}
type TeamChecker interface {
	CanTeam(context.Context, uuid.UUID, m.TeamAction) (bool, error)
}

func Contest(ctx context.Context, c ContestChecker, id uuid.UUID, action m.ContestAction) error {
	return authorize(ctx, func(a m.ContestAction) (bool, error) { return c.CanContest(ctx, id, a) }, action, m.ActionGetContest)
}
func Problem(ctx context.Context, c ProblemChecker, id uuid.UUID, action m.ProblemAction) error {
	return authorize(ctx, func(a m.ProblemAction) (bool, error) { return c.CanProblem(ctx, id, a) }, action, m.ActionViewProblem)
}
func Organization(ctx context.Context, c OrganizationChecker, id uuid.UUID, action m.OrgAction) error {
	return authorize(ctx, func(a m.OrgAction) (bool, error) { return c.CanOrganization(ctx, id, a) }, action, m.ActionViewOrganization)
}
func Team(ctx context.Context, c TeamChecker, id uuid.UUID, action m.TeamAction) error {
	return authorize(ctx, func(a m.TeamAction) (bool, error) { return c.CanTeam(ctx, id, a) }, action, m.ActionViewTeam)
}

// A denied guest must log in; a resource the caller cannot see is hidden, a visible one is forbidden.
func authorize[A comparable](ctx context.Context, can func(A) (bool, error), action, view A) error {
	allowed, err := can(action)
	if err != nil || allowed {
		return err
	}
	if m.UserFromContext(ctx).IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if action != view {
		visible, err := can(view)
		if err != nil {
			return err
		}
		if visible {
			return pkg.ErrForbidden
		}
	}
	return pkg.ErrNotFound
}
