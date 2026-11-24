package postgres

import (
	"context"
	"errors"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookletsRepo struct{ pool *pgxpool.Pool }

func NewBookletsRepo(pool *pgxpool.Pool) *BookletsRepo { return &BookletsRepo{pool: pool} }

func (r *BookletsRepo) QueueBooklet(ctx context.Context, contestID uuid.UUID, language, source, sourceSHA256 string) error {
	err := sqlc.New(r.pool).QueueContestBooklet(ctx, sqlc.QueueContestBookletParams{ContestID: contestID, Language: language, Source: source, SourceSha256: sourceSHA256})
	return dbError("queue booklet", err)
}

func (r *BookletsRepo) KeepBooklets(ctx context.Context, contestID uuid.UUID, languages []string) error {
	err := sqlc.New(r.pool).DeleteContestBookletsExcept(ctx, sqlc.DeleteContestBookletsExceptParams{ContestID: contestID, Languages: languages})
	return dbError("delete booklets", err)
}

func (r *BookletsRepo) ListBooklets(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error) {
	rows, err := sqlc.New(r.pool).ListContestBooklets(ctx, contestID)
	if err != nil {
		return nil, dbError("list booklets", err)
	}
	booklets := make([]contestsmodel.Booklet, len(rows))
	for i, row := range rows {
		booklets[i] = contestsmodel.Booklet{Language: row.Language, State: row.State, SourceSHA256: row.SourceSha256, UpdatedAt: row.UpdatedAt}
		if row.Diagnostic != nil {
			booklets[i].Diagnostic = *row.Diagnostic
		}
		if row.ObjectKey != nil {
			booklets[i].ObjectKey = *row.ObjectKey
		}
	}
	return booklets, nil
}

func (r *BookletsRepo) ClaimBooklet(ctx context.Context, lease time.Duration) (*contestsmodel.BookletJob, error) {
	token := uuid.New()
	row, err := sqlc.New(r.pool).ClaimContestBooklet(ctx, sqlc.ClaimContestBookletParams{Token: token, LeaseMs: lease.Milliseconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError("claim booklet", err)
	}
	return &contestsmodel.BookletJob{ContestID: row.ContestID, OrganizationID: row.OrganizationID, Language: row.Language, Source: row.Source, Attempt: int(row.Attempt), Token: token}, nil
}

// CompleteBooklet registers the uploaded PDF together with its only reference.
func (r *BookletsRepo) CompleteBooklet(ctx context.Context, job contestsmodel.BookletJob, object problemsmodel.StoredObject) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if err := q.InsertStoredObject(ctx, sqlc.InsertStoredObjectParams{ID: object.ID, OrganizationID: object.OrganizationID, Purpose: object.Purpose, Key: object.Key, Sha256: object.SHA256, Size: object.Size}); err != nil {
		return dbError("insert booklet object", err)
	}
	updated, err := q.CompleteContestBooklet(ctx, sqlc.CompleteContestBookletParams{ObjectID: object.ID, ContestID: job.ContestID, Language: job.Language, Token: job.Token})
	if err != nil {
		return dbError("complete booklet", err)
	}
	if updated != 1 {
		return pkg.ErrConflict
	}
	return commitTx(ctx, tx)
}

func (r *BookletsRepo) FailBooklet(ctx context.Context, job contestsmodel.BookletJob, diagnostic string) error {
	updated, err := sqlc.New(r.pool).FailContestBooklet(ctx, sqlc.FailContestBookletParams{Diagnostic: diagnostic, ContestID: job.ContestID, Language: job.Language, Token: job.Token})
	return finished("fail booklet", updated, err)
}

func (r *BookletsRepo) RequeueBooklet(ctx context.Context, job contestsmodel.BookletJob) error {
	updated, err := sqlc.New(r.pool).RequeueContestBooklet(ctx, sqlc.RequeueContestBookletParams{ContestID: job.ContestID, Language: job.Language, Token: job.Token})
	return finished("requeue booklet", updated, err)
}

func finished(op string, updated int64, err error) error {
	if err != nil {
		return dbError(op, err)
	}
	if updated != 1 {
		return pkg.ErrConflict
	}
	return nil
}
