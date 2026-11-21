package postgres

import (
	"context"

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
	// Authorizing commands run serializable, so a revocation racing with them commits after them or aborts them.
	serializable bool
}

const serializationAttempts = 3

// Within reruns fn after a lost serialization race, so fn must have no effects outside the transaction.
func (p transactions[T]) Within(ctx context.Context, fn func(T) error) error {
	var err error
	for range serializationAttempts {
		if err = p.once(ctx, fn); !p.serializable || !isRetryable(err) {
			break
		}
	}
	return retryable(err)
}

func (p transactions[T]) once(ctx context.Context, fn func(T) error) error {
	options := pgx.TxOptions{}
	if p.serializable {
		options.IsoLevel = pgx.Serializable
	}
	tx, err := p.pool.BeginTx(ctx, options)
	if err != nil {
		return dbError("begin", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(p.bind(sqlc.New(tx))); err != nil {
		return err
	}
	return dbError("commit", tx.Commit(ctx))
}

func authorizing[T any](pool *pgxpool.Pool, bind func(*sqlc.Queries) T) transactions[T] {
	return transactions[T]{pool: pool, bind: bind, serializable: true}
}

func NewTeamTransactions(pool *pgxpool.Pool) organizations.TeamTransactions {
	return authorizing(pool, func(q *sqlc.Queries) organizations.TeamTxStore {
		return struct {
			*TeamsRepo
			*OrganizationsRepo
			Access
		}{&TeamsRepo{q: q}, &OrganizationsRepo{q: q}, commandAccess(q)}
	})
}

func NewProblemTransactions(pool *pgxpool.Pool) problems.ProblemTransactions {
	return authorizing(pool, func(q *sqlc.Queries) problems.ProblemTxStore {
		return struct {
			*ProblemsRepo
			*OrganizationsRepo
			*PackagesRepo
			Access
		}{&ProblemsRepo{queries: q}, &OrganizationsRepo{q: q}, &PackagesRepo{queries: q}, commandAccess(q)}
	})
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
	return authorizing(pool, func(q *sqlc.Queries) organizations.OrganizationTxStore {
		return struct {
			*OrganizationsRepo
			*UsersRepo
			*NotificationsRepo
			Access
		}{&OrganizationsRepo{q: q}, &UsersRepo{queries: q}, &NotificationsRepo{queries: q}, commandAccess(q)}
	})
}

func NewSubmissionTransactions(pool *pgxpool.Pool) submissions.SubmissionTransactions {
	return authorizing(pool, func(q *sqlc.Queries) submissions.SubmissionTxStore {
		return struct {
			*SubmissionsRepo
			*OutboxRepo
			*JudgeJobs
			*ContestsRepo
			Access
		}{&SubmissionsRepo{queries: q}, &OutboxRepo{queries: q}, &JudgeJobs{queries: q}, &ContestsRepo{queries: q}, commandAccess(q)}
	})
}

func NewContestEventsTransactions(pool *pgxpool.Pool) contests.ContestEventsTransactions {
	return authorizing(pool, func(q *sqlc.Queries) contests.ContestEventsTxStore {
		return struct {
			*AnnouncementsRepo
			*ClarificationsRepo
			*OutboxRepo
			Access
		}{&AnnouncementsRepo{queries: q}, &ClarificationsRepo{queries: q}, &OutboxRepo{queries: q}, commandAccess(q)}
	})
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
	return authorizing(pool, func(q *sqlc.Queries) contests.CommandTxStore {
		return struct {
			*ContestsRepo
			*SubmissionsRepo
			*JudgeJobs
			*OutboxRepo
			*OrganizationsRepo
			*NotificationsRepo
			Access
		}{&ContestsRepo{queries: q}, &SubmissionsRepo{queries: q}, &JudgeJobs{queries: q}, &OutboxRepo{queries: q}, &OrganizationsRepo{q: q}, &NotificationsRepo{queries: q}, commandAccess(q)}
	})
}

func NewDraftTransactions(pool *pgxpool.Pool) contests.DraftTransactions {
	return authorizing(pool, func(q *sqlc.Queries) contests.DraftTxStore {
		return struct {
			*DraftsRepo
			Access
		}{&DraftsRepo{queries: q}, commandAccess(q)}
	})
}

func NewBlogTransactions(pool *pgxpool.Pool) blogs.Transactions {
	return authorizing(pool, func(q *sqlc.Queries) blogs.TxStore {
		return struct {
			*BlogsRepo
			Access
		}{&BlogsRepo{queries: q}, commandAccess(q)}
	})
}

// retryable marks a race still lost after retries; the client may repeat the request.
func retryable(err error) error {
	if isRetryable(err) {
		return pkg.Wrap(pkg.ErrConcurrentUpdate, err, "concurrent update, retry")
	}
	return err
}
