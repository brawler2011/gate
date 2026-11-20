package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemDraftsRepo struct {
	pool           *pgxpool.Pool
	quotas         model.DraftQuotas
	operationLease time.Duration
}

func NewProblemDraftsRepo(pool *pgxpool.Pool, quotas model.DraftQuotas, operationLease time.Duration) *ProblemDraftsRepo {
	return &ProblemDraftsRepo{pool: pool, quotas: quotas, operationLease: operationLease}
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

func commitTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return dbError("commit transaction", err)
	}
	return nil
}

func rollback(ctx context.Context, tx pgx.Tx) { _ = tx.Rollback(context.WithoutCancel(ctx)) }

// reserveOperation must precede every owner/object lock in the transaction.
func (r *ProblemDraftsRepo) reserveOperation(ctx context.Context, tx pgx.Tx, k model.MutationKey) (*model.MutationResult, error) {
	if k.OrganizationID == uuid.Nil || k.Token == uuid.Nil || len(k.Key) == 0 || len(k.Key) > 200 || r.operationLease <= 0 {
		return nil, pkg.ErrInvalid
	}
	q := sqlc.New(tx)
	if err := q.ReserveOperationRow(ctx, sqlc.ReserveOperationRowParams{
		OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Fingerprint: k.Fingerprint, Token: k.Token,
		LeaseMs: r.operationLease.Milliseconds(),
	}); err != nil {
		return nil, dbError("reserve operation", err)
	}
	row, err := q.LockOperationRow(ctx, sqlc.LockOperationRowParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return nil, dbError("lock operation", err)
	}
	if row.Fingerprint != k.Fingerprint {
		return nil, pkg.Wrap(pkg.ErrConflict, nil, "idempotency key reused for different request")
	}
	if row.HttpStatus != nil {
		return &model.MutationResult{Status: int(*row.HttpStatus), Body: row.Result}, nil
	}
	// Time is checked again after acquiring the row lock, in a separate statement.
	active, err := q.OperationLeaseActive(ctx, sqlc.OperationLeaseActiveParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return nil, dbError("check operation lease", err)
	}
	if active && row.Token != k.Token {
		return nil, pkg.Wrap(pkg.ErrConflict, nil, "operation is running")
	}
	if row.Token != k.Token {
		if err := q.TakeOverOperation(ctx, sqlc.TakeOverOperationParams{
			Token: k.Token, LeaseMs: r.operationLease.Milliseconds(), OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key,
		}); err != nil {
			return nil, dbError("take over operation", err)
		}
	}
	return nil, nil
}

func finishOperation(ctx context.Context, tx pgx.Tx, k model.MutationKey, status int, body any) (model.MutationResult, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return model.MutationResult{}, dbError("encode operation result", err)
	}
	code := int32(status) //nolint:gosec // HTTP status codes are small constants.
	updated, err := sqlc.New(tx).FinishOperationRow(ctx, sqlc.FinishOperationRowParams{
		Result: data, HttpStatus: &code, OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Token: k.Token,
	})
	if err != nil {
		return model.MutationResult{}, dbError("store operation result", err)
	}
	if updated != 1 {
		return model.MutationResult{}, pkg.ErrConflict
	}
	return model.MutationResult{Status: status, Body: data}, nil
}

// Retries retain the key and fingerprint to resolve a lost commit response without duplication.
func (r *ProblemDraftsRepo) resolveMutation(ctx context.Context, tx pgx.Tx, k model.MutationKey, result model.MutationResult) (model.MutationResult, error) {
	err := commitTx(ctx, tx)
	if err == nil {
		return result, nil
	}
	resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	stored, readErr := sqlc.New(r.pool).ReadOperationResult(resolveCtx, sqlc.ReadOperationResultParams{
		OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Fingerprint: k.Fingerprint,
	})
	if readErr == nil && stored.HttpStatus != nil {
		return model.MutationResult{Status: int(*stored.HttpStatus), Body: stored.Result}, nil
	}
	return model.MutationResult{}, err
}

func (r *ProblemDraftsRepo) SaveDraft(ctx context.Context, k model.MutationKey, draft model.Draft) (model.MutationResult, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid weights")
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.MutationResult{}, err
	}
	defer rollback(ctx, tx)
	replay, err := r.reserveOperation(ctx, tx, k)
	if err != nil {
		return model.MutationResult{}, err
	}
	if replay != nil {
		return *replay, nil
	}
	q := sqlc.New(tx)
	revision, err := q.LockProblemDraft(ctx, draft.ProblemID)
	if err != nil {
		return model.MutationResult{}, dbError("save draft", err)
	}
	if revision != draft.Revision {
		return model.MutationResult{}, pkg.ErrConflict
	}
	org, err := q.ProblemOrganizationID(ctx, draft.ProblemID)
	if err != nil {
		return model.MutationResult{}, dbError("save draft", err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.ErrForbidden
	}
	old, err := readProblemDraft(ctx, q, draft.ProblemID)
	if err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.Validate(r.quotas, &old); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid draft")
	}
	draft.Revision++
	if err := writeDraft(ctx, tx, draft); err != nil {
		return model.MutationResult{}, err
	}
	result, err := finishOperation(ctx, tx, k, 200, draft)
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}

// Foreign keys reject missing or cross-organization draft objects with ErrConflict.
func writeDraft(ctx context.Context, tx pgx.Tx, d model.Draft) error {
	d.Normalize()
	content, err := json.Marshal(d.DraftContent)
	if err != nil {
		return dbError("encode draft content", err)
	}
	q := sqlc.New(tx)
	if err := q.UpdateProblemDraftRow(ctx, sqlc.UpdateProblemDraftRowParams{
		ProblemID: d.ProblemID, Revision: d.Revision, Title: d.Title, ShortName: d.ShortName, Type: d.Type,
		TimeLimitMs: int32(d.TimeLimitMs), MemoryLimitMb: int32(d.MemoryLimitMb), MaxScore: d.MaxScore, //nolint:gosec // validated against quotas
		Content: content,
	}); err != nil {
		return dbError("update draft", err)
	}
	// Derive references from content under the revision lock to replace both atomically.
	if err := q.DeleteDraftObjects(ctx, d.ProblemID); err != nil {
		return dbError("clear draft objects", err)
	}
	for _, oid := range d.InputObjects() {
		if err := q.InsertDraftObject(ctx, sqlc.InsertDraftObjectParams{ProblemID: d.ProblemID, ObjectID: oid}); err != nil {
			return objectReferenceErr(err, "insert draft object")
		}
	}
	return nil
}

func objectReferenceErr(err error, action string) error {
	if isForeignKeyViolation(err) {
		return pkg.Wrap(pkg.ErrConflict, err, "object is missing or belongs to another organization")
	}
	return fmt.Errorf("%s: %w", action, err)
}
