package access

import (
	"context"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Transactions runs fn in one serializable transaction and reruns it after a lost race.
// Commands open them only through the Within helpers below, so each one decides inside the transaction that writes.
type Transactions[T Loader] interface {
	Within(context.Context, func(T) error) error
}

// WithinActor runs a command on the caller's own behalf; the actor row stays locked until commit.
func WithinActor[T Loader](ctx context.Context, tr Transactions[T], fn func(T, m.User) error) error {
	return tr.Within(ctx, func(tx T) error {
		actor, err := Actor(ctx, tx)
		if err != nil {
			return err
		}
		if actor.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		return fn(tx, actor)
	})
}

func WithinOrganization[T Loader](ctx context.Context, tr Transactions[T], id uuid.UUID, action m.OrganizationAction, fn func(T, OrganizationAccess) error) error {
	return within(ctx, tr, func(tx T) (OrganizationAccess, error) { return Organization(ctx, tx, id, action) }, fn)
}

func WithinTeam[T Loader](ctx context.Context, tr Transactions[T], id uuid.UUID, action m.TeamAction, fn func(T, TeamAccess) error) error {
	return within(ctx, tr, func(tx T) (TeamAccess, error) { return Team(ctx, tx, id, action) }, fn)
}

func WithinProblem[T Loader](ctx context.Context, tr Transactions[T], id uuid.UUID, action m.ProblemAction, fn func(T, ProblemAccess) error) error {
	return within(ctx, tr, func(tx T) (ProblemAccess, error) { return Problem(ctx, tx, id, action) }, fn)
}

func WithinContest[T Loader](ctx context.Context, tr Transactions[T], id uuid.UUID, action m.ContestAction, fn func(T, ContestAccess) error) error {
	return within(ctx, tr, func(tx T) (ContestAccess, error) { return Contest(ctx, tx, id, action) }, fn)
}

func within[T Loader, A any](ctx context.Context, tr Transactions[T], decide func(T) (A, error), fn func(T, A) error) error {
	return tr.Within(ctx, func(tx T) error {
		a, err := decide(tx)
		if err != nil {
			return err
		}
		return fn(tx, a)
	})
}
