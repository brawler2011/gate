package postgres

import (
	"context"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	organizations "github.com/brawler2011/gate/backend/internal/organizations"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
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

func NewTeamTransactions(pool *pgxpool.Pool) organizations.TeamTransactions {
	return transactions[organizations.TeamTxStore]{pool: pool, bind: func(q *sqlc.Queries) organizations.TeamTxStore {
		return struct {
			*TeamsRepo
			*OrganizationsRepo
			*authorization
		}{&TeamsRepo{q: q}, &OrganizationsRepo{q: q}, bindAuthorization(q)}
	}}
}

func NewProblemTransactions(pool *pgxpool.Pool) problems.ProblemTransactions {
	return transactions[problems.ProblemTxStore]{pool: pool, bind: func(q *sqlc.Queries) problems.ProblemTxStore {
		return struct{ *ProblemsRepo }{&ProblemsRepo{queries: q}}
	}}
}

func NewIdentityTransactions(pool *pgxpool.Pool) identity.IdentityTransactions {
	return transactions[identity.IdentityTxStore]{pool: pool, bind: func(q *sqlc.Queries) identity.IdentityTxStore {
		return struct {
			*UsersRepo
			*AuthRepo
			*ContestsRepo
		}{&UsersRepo{queries: q}, &AuthRepo{queries: q}, &ContestsRepo{queries: q}}
	}}
}

func NewOrganizationTransactions(pool *pgxpool.Pool) organizations.OrganizationTransactions {
	return transactions[organizations.OrganizationTxStore]{pool: pool, bind: func(q *sqlc.Queries) organizations.OrganizationTxStore {
		return struct {
			*OrganizationsRepo
			*UsersRepo
			*NotificationsRepo
			*authorization
		}{&OrganizationsRepo{q: q}, &UsersRepo{queries: q}, &NotificationsRepo{queries: q}, bindAuthorization(q)}
	}}
}

func NewSubmissionTransactions(pool *pgxpool.Pool) submissions.SubmissionTransactions {
	return transactions[submissions.SubmissionTxStore]{pool: pool, bind: func(q *sqlc.Queries) submissions.SubmissionTxStore {
		return struct {
			*SubmissionsRepo
			*OutboxRepo
			*JudgeJobs
			*ContestsRepo
			*authorization
		}{&SubmissionsRepo{queries: q}, &OutboxRepo{queries: q}, &JudgeJobs{queries: q}, &ContestsRepo{queries: q}, bindAuthorization(q)}
	}}
}

func NewContestEventsTransactions(pool *pgxpool.Pool) contests.ContestEventsTransactions {
	return transactions[contests.ContestEventsTxStore]{pool: pool, bind: func(q *sqlc.Queries) contests.ContestEventsTxStore {
		return struct {
			*AnnouncementsRepo
			*ClarificationsRepo
			*OutboxRepo
			*ContestsRepo
			*authorization
		}{&AnnouncementsRepo{queries: q}, &ClarificationsRepo{queries: q}, &OutboxRepo{queries: q}, &ContestsRepo{queries: q}, bindAuthorization(q)}
	}}
}

func NewJudgeTransactions(pool *pgxpool.Pool) judging.JudgeTransactions {
	return transactions[judging.JudgeTxStore]{pool: pool, bind: func(q *sqlc.Queries) judging.JudgeTxStore {
		return struct {
			*JudgeJobs
			*SubmissionsRepo
			*ContestsRepo
			*OutboxRepo
		}{&JudgeJobs{q}, &SubmissionsRepo{queries: q}, &ContestsRepo{queries: q}, &OutboxRepo{queries: q}}
	}}
}

func NewContestCommandTransactions(pool *pgxpool.Pool) contests.CommandTransactions {
	return transactions[contests.CommandTxStore]{pool: pool, bind: func(q *sqlc.Queries) contests.CommandTxStore {
		return struct {
			*ContestsRepo
			*SubmissionsRepo
			*JudgeJobs
			*OutboxRepo
			*authorization
			*OrganizationsRepo
			*UsersRepo
			*NotificationsRepo
		}{&ContestsRepo{queries: q}, &SubmissionsRepo{queries: q}, &JudgeJobs{queries: q}, &OutboxRepo{queries: q}, bindAuthorization(q), &OrganizationsRepo{q: q}, &UsersRepo{queries: q}, &NotificationsRepo{queries: q}}
	}}
}

func NewDraftTransactions(pool *pgxpool.Pool) contests.DraftTransactions {
	return transactions[contests.DraftTxStore]{pool: pool, bind: func(q *sqlc.Queries) contests.DraftTxStore {
		return struct {
			*DraftsRepo
			*ContestsRepo
			*authorization
		}{&DraftsRepo{queries: q}, &ContestsRepo{queries: q}, bindAuthorization(q)}
	}}
}
