package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookletsRepo struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewBookletsRepo(pool *pgxpool.Pool) *BookletsRepo {
	return &BookletsRepo{pool: pool, q: sqlc.New(pool)}
}

// QueueBooklets re-keys the contest's booklets in the caller's transaction, after its changes.
func (r *BookletsRepo) QueueBooklets(ctx context.Context, contestID uuid.UUID) error {
	return queueBooklets(ctx, r.q, sqlc.ListBookletInputsParams{ContestID: &contestID})
}

func (r *BookletsRepo) QueueOrganizationBooklets(ctx context.Context, organizationID uuid.UUID) error {
	return queueBooklets(ctx, r.q, sqlc.ListBookletInputsParams{OrganizationID: &organizationID})
}

// RekeyAllBooklets queues booklets whose key no contest change renewed, such as after a generator version bump.
func (r *BookletsRepo) RekeyAllBooklets(ctx context.Context) error {
	return r.rekey(ctx, sqlc.ListBookletInputsParams{})
}

func (r *BookletsRepo) RekeyBooklets(ctx context.Context, contestID uuid.UUID) error {
	return r.rekey(ctx, sqlc.ListBookletInputsParams{ContestID: &contestID})
}

// Serializable, so a re-key from inputs read before a concurrent contest change cannot overwrite that change's key.
func (r *BookletsRepo) rekey(ctx context.Context, scope sqlc.ListBookletInputsParams) error {
	_, err := serializableRetry(func() (struct{}, error) {
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return struct{}{}, dbError("begin transaction", err)
		}
		defer rollback(ctx, tx)
		if err := queueBooklets(ctx, sqlc.New(tx), scope); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, commitTx(ctx, tx)
	})
	return err
}

func queueBooklets(ctx context.Context, q *sqlc.Queries, scope sqlc.ListBookletInputsParams) error {
	booklets, err := listBooklets(ctx, q, scope)
	if err != nil {
		return err
	}
	keep := make([]string, 0, len(booklets))
	queued := false
	for _, b := range booklets {
		keep = append(keep, b.ContestID.String()+"/"+b.Language)
		n, err := q.QueueBooklet(ctx, sqlc.QueueBookletParams{ContestID: b.ContestID, OrganizationID: b.OrganizationID, Language: b.Language, Key: b.Key()})
		if err != nil {
			return dbError("queue booklet", err)
		}
		queued = queued || n > 0
	}
	if err := q.DeleteOtherBooklets(ctx, sqlc.DeleteOtherBookletsParams{ContestID: scope.ContestID, OrganizationID: scope.OrganizationID, Keep: keep}); err != nil {
		return dbError("delete other booklets", err)
	}
	if queued {
		return dbError("wake booklet workers", q.WakeBookletWorkers(ctx))
	}
	return nil
}

func (r *BookletsRepo) ListBooklets(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error) {
	return listBooklets(ctx, r.q, sqlc.ListBookletInputsParams{ContestID: &contestID})
}

func listBooklets(ctx context.Context, q *sqlc.Queries, scope sqlc.ListBookletInputsParams) ([]contestsmodel.Booklet, error) {
	rows, err := q.ListBookletInputs(ctx, scope)
	if err != nil {
		return nil, dbError("list booklet inputs", err)
	}
	booklets := make([]contestsmodel.Booklet, len(rows))
	for i, row := range rows {
		var problems []contestsmodel.BookletProblem
		if err := json.Unmarshal(row.Problems, &problems); err != nil {
			return nil, fmt.Errorf("decode booklet problems: %w", err)
		}
		inputs, err := contestsmodel.NewBookletInputs(row.ContestID, row.OrganizationID, row.Language, row.Title, row.Place, row.OrganizationName, row.StartTime, row.TimeZone, problems)
		if err != nil {
			return nil, err
		}
		booklets[i].BookletInputs = inputs
		if row.StoredKey != nil {
			booklets[i].Stored = &contestsmodel.StoredBooklet{Key: *row.StoredKey, State: contestsmodel.BookletState(deref(row.StoredState)),
				Diagnostic: deref(row.StoredDiagnostic), UpdatedAt: *row.StoredUpdatedAt, ObjectKey: deref(row.ObjectKey)}
		}
	}
	return booklets, nil
}

// ClaimBooklet returns nil when no booklet waits.
func (r *BookletsRepo) ClaimBooklet(ctx context.Context, lease time.Duration) (*contestsmodel.BookletJob, error) {
	token := uuid.New()
	row, err := r.q.ClaimBooklet(ctx, sqlc.ClaimBookletParams{LeaseToken: token, LeaseSeconds: lease.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError("claim booklet", err)
	}
	return &contestsmodel.BookletJob{ContestID: row.ContestID, OrganizationID: row.OrganizationID, Language: row.Language, Key: row.Key, Attempt: int(row.Attempt), LeaseToken: token}, nil
}

// CompleteBooklet returns pkg.ErrConflict once the booklet was re-keyed or reclaimed.
func (r *BookletsRepo) CompleteBooklet(ctx context.Context, job contestsmodel.BookletJob, object problemsmodel.StoredObject) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if err := q.InsertStoredObject(ctx, sqlc.InsertStoredObjectParams{ID: object.ID, OrganizationID: object.OrganizationID, Purpose: object.Purpose, Key: object.Key, Sha256: object.SHA256, Size: object.Size}); err != nil {
		return dbError("insert stored object", err)
	}
	n, err := q.CompleteBooklet(ctx, sqlc.CompleteBookletParams{ObjectID: object.ID, ContestID: job.ContestID, Language: job.Language, Key: job.Key, LeaseToken: job.LeaseToken})
	if err != nil {
		return dbError("complete booklet", err)
	}
	if n == 0 {
		return pkg.ErrConflict
	}
	return commitTx(ctx, tx)
}

func (r *BookletsRepo) FailBooklet(ctx context.Context, job contestsmodel.BookletJob, diagnostic string) error {
	n, err := r.q.FailBooklet(ctx, sqlc.FailBookletParams{Diagnostic: diagnostic, ContestID: job.ContestID, Language: job.Language, Key: job.Key, LeaseToken: job.LeaseToken})
	return finished("fail booklet", n, err)
}

func (r *BookletsRepo) RequeueBooklet(ctx context.Context, job contestsmodel.BookletJob) error {
	n, err := r.q.RequeueBooklet(ctx, sqlc.RequeueBookletParams{ContestID: job.ContestID, Language: job.Language, Key: job.Key, LeaseToken: job.LeaseToken})
	return finished("requeue booklet", n, err)
}

func finished(op string, n int64, err error) error {
	if err != nil {
		return dbError(op, err)
	}
	if n == 0 {
		return pkg.ErrConflict
	}
	return nil
}
