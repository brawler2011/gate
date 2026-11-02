package pg

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactions[T any] struct {
	pool *pgxpool.Pool
	bind func(*sqlc.Queries) T
}

func (p transactions[T]) Within(ctx context.Context, fn func(T) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return HandlePgErr(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(p.bind(sqlc.New(tx))); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func NewTeamTransactions(pool *pgxpool.Pool) usecase.TeamTransactions {
	return transactions[usecase.TeamTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.TeamTxStore {
		return struct{ *TeamsRepo }{&TeamsRepo{q: q}}
	}}
}

func NewProblemTransactions(pool *pgxpool.Pool) usecase.ProblemTransactions {
	return transactions[usecase.ProblemTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.ProblemTxStore {
		return struct{ *ProblemsRepo }{&ProblemsRepo{queries: q}}
	}}
}

func NewIdentityTransactions(pool *pgxpool.Pool) usecase.IdentityTransactions {
	return transactions[usecase.IdentityTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.IdentityTxStore {
		return struct {
			*UsersRepo
			*AuthRepo
			*ContestsRepo
		}{&UsersRepo{queries: q}, &AuthRepo{queries: q}, &ContestsRepo{queries: q}}
	}}
}

func NewOrganizationTransactions(pool *pgxpool.Pool) usecase.OrganizationTransactions {
	return transactions[usecase.OrganizationTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.OrganizationTxStore {
		return struct {
			*OrganizationsRepo
			*UsersRepo
		}{&OrganizationsRepo{q: q}, &UsersRepo{queries: q}}
	}}
}

func NewSubmissionTransactions(pool *pgxpool.Pool) usecase.SubmissionTransactions {
	return transactions[usecase.SubmissionTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.SubmissionTxStore {
		return struct {
			*SubmissionsRepo
			*OutboxRepo
		}{&SubmissionsRepo{queries: q}, &OutboxRepo{queries: q}}
	}}
}

func NewContestEventsTransactions(pool *pgxpool.Pool) usecase.ContestEventsTransactions {
	return transactions[usecase.ContestEventsTxStore]{pool: pool, bind: func(q *sqlc.Queries) usecase.ContestEventsTxStore {
		return struct {
			*AnnouncementsRepo
			*ClarificationsRepo
			*OutboxRepo
		}{&AnnouncementsRepo{queries: q}, &ClarificationsRepo{queries: q}, &OutboxRepo{queries: q}}
	}}
}
