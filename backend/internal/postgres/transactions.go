package postgres

import (
	"context"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgx/v5"

	"github.com/brawler2011/gate/backend/internal/blogs"
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
	decision, guarded := m.DecisionFromContext(ctx)
	options := pgx.TxOptions{}
	if guarded {
		options.IsoLevel = pgx.Serializable
	}
	tx, err := p.pool.BeginTx(ctx, options)
	if err != nil {
		return HandlePgErr(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if guarded {
		revision, err := sqlc.New(tx).GetAccessRevision(ctx)
		if err != nil {
			return HandlePgErr(err)
		}
		if revision != decision.Revision {
			return pkg.ErrConflict
		}
	}
	if err := fn(p.bind(sqlc.New(tx))); err != nil {
		return err
	}
	if guarded && decision.ValidUntil != nil {
		now, err := sqlc.New(tx).GetCommandTime(ctx)
		if err != nil {
			return HandlePgErr(err)
		}
		if !now.Before(*decision.ValidUntil) {
			return pkg.NoPermission
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func NewTeamTransactions(pool *pgxpool.Pool) organizations.TeamTransactions {
	return transactions[organizations.TeamTxStore]{pool: pool, bind: func(q *sqlc.Queries) organizations.TeamTxStore {
		return struct {
			authorizationTeams
			authorizationOrganizations
			*authorization
		}{authorizationTeams{&TeamsRepo{q: q}}, authorizationOrganizations{&OrganizationsRepo{q: q}}, bindAuthorization(q)}
	}}
}

func NewProblemTransactions(pool *pgxpool.Pool) problems.ProblemTransactions {
	return transactions[problems.ProblemTxStore]{pool: pool, bind: func(q *sqlc.Queries) problems.ProblemTxStore {
		return struct {
			*ProblemsRepo
			authorizationOrganizations
			*authorization
			*PackagesRepo
			authorizationUsers
		}{&ProblemsRepo{queries: q}, authorizationOrganizations{&OrganizationsRepo{q: q}}, bindAuthorization(q), &PackagesRepo{queries: q}, authorizationUsers{q}}
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
			authorizationOrganizations
			organizationUsers
			*NotificationsRepo
			*authorization
		}{authorizationOrganizations{&OrganizationsRepo{q: q}}, organizationUsers{&UsersRepo{queries: q}}, &NotificationsRepo{queries: q}, bindAuthorization(q)}
	}}
}

func NewSubmissionTransactions(pool *pgxpool.Pool) submissions.SubmissionTransactions {
	return newGuardedTransactions(pool, transactions[submissions.SubmissionTxStore]{pool: pool, bind: func(q *sqlc.Queries) submissions.SubmissionTxStore {
		return struct {
			*SubmissionsRepo
			*OutboxRepo
			*JudgeJobs
			*ContestsRepo
			*authorization
		}{&SubmissionsRepo{queries: q}, &OutboxRepo{queries: q}, &JudgeJobs{queries: q}, &ContestsRepo{queries: q}, bindAuthorization(q)}
	}})
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
	return newGuardedTransactions(pool, transactions[contests.CommandTxStore]{pool: pool, bind: func(q *sqlc.Queries) contests.CommandTxStore {
		return struct {
			*ContestsRepo
			*SubmissionsRepo
			*JudgeJobs
			*OutboxRepo
			*authorization
			authorizationOrganizations
			*NotificationsRepo
		}{&ContestsRepo{queries: q}, &SubmissionsRepo{queries: q}, &JudgeJobs{queries: q}, &OutboxRepo{queries: q}, bindAuthorization(q), authorizationOrganizations{&OrganizationsRepo{q: q}}, &NotificationsRepo{queries: q}}
	}})
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

func NewBlogTransactions(pool *pgxpool.Pool) blogs.Transactions {
	return transactions[blogs.TxStore]{pool: pool, bind: func(q *sqlc.Queries) blogs.TxStore {
		return struct {
			*BlogsRepo
			authorizationUsers
		}{&BlogsRepo{queries: q}, authorizationUsers{q}}
	}}
}
