package postgres

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemDraftsRepo struct {
	pool   *pgxpool.Pool
	quotas model.DraftQuotas
}

func NewProblemDraftsRepo(pool *pgxpool.Pool, quotas model.DraftQuotas) *ProblemDraftsRepo {
	return &ProblemDraftsRepo{pool: pool, quotas: quotas}
}

func readProblemDraft(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (model.Draft, error) {
	row, err := q.ReadProblemDraft(ctx, id)
	if err != nil {
		return model.Draft{}, dbError("read problem draft", err)
	}
	d := model.Draft{ProblemID: row.ProblemID, Revision: row.Revision, Title: row.Title, ShortName: row.ShortName, Type: row.Type,
		TimeLimitMs: int(row.TimeLimitMs), MemoryLimitMb: int(row.MemoryLimitMb), MaxScore: row.MaxScore}
	if err := json.Unmarshal(row.Content, &d.DraftContent); err != nil {
		return d, dbError("decode draft content", err)
	}
	d.Normalize()
	return d, nil
}

func (r *ProblemDraftsRepo) GetDraft(ctx context.Context, id uuid.UUID) (model.Draft, error) {
	return readProblemDraft(ctx, sqlc.New(r.pool), id)
}

func beginReadCommitted(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	return tx, nil
}

func beginSerializable(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	return tx, nil
}

// serializableRetry reruns a serializable transaction that lost a race.
func serializableRetry[T any](run func() (T, error)) (T, error) {
	var result T
	var err error
	for range serializationAttempts {
		if result, err = run(); !isRetryable(err) {
			break
		}
	}
	return result, retryable(err)
}

func authorizeEdit(ctx context.Context, q *sqlc.Queries, problemID uuid.UUID) error {
	_, err := access.Problem(ctx, commandAccess(q), problemID, accessmodel.ProblemEdit)
	return err
}

func commitTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return dbError("commit transaction", err)
	}
	return nil
}

func rollback(ctx context.Context, tx pgx.Tx) { _ = tx.Rollback(context.WithoutCancel(ctx)) }

func (r *ProblemDraftsRepo) SaveDraft(ctx context.Context, draft model.Draft) (model.Draft, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.Draft{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid weights")
	}
	return serializableRetry(func() (model.Draft, error) { return r.saveDraft(ctx, draft) })
}

func (r *ProblemDraftsRepo) saveDraft(ctx context.Context, draft model.Draft) (model.Draft, error) {
	tx, err := beginSerializable(ctx, r.pool)
	if err != nil {
		return model.Draft{}, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if err := authorizeEdit(ctx, q, draft.ProblemID); err != nil {
		return model.Draft{}, err
	}
	revision, err := q.LockProblemDraft(ctx, draft.ProblemID)
	if err != nil {
		return model.Draft{}, dbError("save draft", err)
	}
	if revision != draft.Revision {
		return model.Draft{}, pkg.ErrStaleRevision
	}
	old, err := readProblemDraft(ctx, q, draft.ProblemID)
	if err != nil {
		return model.Draft{}, err
	}
	if err := draft.Validate(r.quotas, &old); err != nil {
		return model.Draft{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid draft")
	}
	known, err := q.DraftObjectIDs(ctx, draft.ProblemID)
	if err != nil {
		return model.Draft{}, dbError("save draft", err)
	}
	draft.Revision++
	if err := writeDraft(ctx, tx, draft, known); err != nil {
		return model.Draft{}, err
	}
	return draft, commitTx(ctx, tx)
}

// writeDraft accepts only objects in known, so a reference is always created with its object or copied from a live one,
// which object GC relies on.
func writeDraft(ctx context.Context, tx pgx.Tx, d model.Draft, known []uuid.UUID) error {
	d.Normalize()
	objects := d.InputObjects()
	for _, id := range objects {
		if !slices.Contains(known, id) {
			return pkg.Wrap(pkg.ErrConflict, nil, "object "+id.String()+" is not part of this draft")
		}
	}
	content, err := json.Marshal(d.DraftContent)
	if err != nil {
		return dbError("encode draft content", err)
	}
	if err := sqlc.New(tx).UpdateProblemDraftRow(ctx, sqlc.UpdateProblemDraftRowParams{
		ProblemID: d.ProblemID, Revision: d.Revision, Title: d.Title, ShortName: d.ShortName, Type: d.Type,
		TimeLimitMs: int32(d.TimeLimitMs), MemoryLimitMb: int32(d.MemoryLimitMb), MaxScore: d.MaxScore, //nolint:gosec // validated against quotas
		Content: content, ObjectIds: objects,
	}); err != nil {
		return dbError("update draft", err)
	}
	return nil
}
