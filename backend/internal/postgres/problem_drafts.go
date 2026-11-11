package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
	value, err := q.ReadProblemDraft(ctx, id)
	if err != nil {
		return model.Draft{}, HandlePgErr(err)
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data, err = json.Marshal(value)
	}
	if err != nil {
		return model.Draft{}, fmt.Errorf("marshal: %w", err)
	}
	var draft model.Draft
	if err := json.Unmarshal(data, &draft); err != nil {
		return draft, fmt.Errorf("decode draft: %w", err)
	}
	return draft, nil
}

func (r *ProblemDraftsRepo) GetDraft(ctx context.Context, id uuid.UUID) (model.Draft, error) {
	return readProblemDraft(ctx, sqlc.New(r.pool), id)
}

func beginReadCommitted(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
}

func commitTx(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// rollback is deferred after Begin; the error of a rollback after a commit is irrelevant.
func rollback(ctx context.Context, tx pgx.Tx) { _ = tx.Rollback(ctx) }

// reserveOperation must precede every owner/object lock in the transaction.
func (r *ProblemDraftsRepo) reserveOperation(ctx context.Context, tx pgx.Tx, k model.MutationKey) (*model.MutationResult, error) {
	if k.OrganizationID == uuid.Nil || k.Token == uuid.Nil || len(k.Key) == 0 || len(k.Key) > 200 || r.operationLease <= 0 {
		return nil, pkg.ErrBadInput
	}
	q := sqlc.New(tx)
	if err := q.ReserveOperationRow(ctx, sqlc.ReserveOperationRowParams{
		OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Fingerprint: k.Fingerprint, Token: k.Token,
		LeaseMs: r.operationLease.Milliseconds(),
	}); err != nil {
		return nil, fmt.Errorf("reserve operation: %w", err)
	}
	row, err := q.LockOperationRow(ctx, sqlc.LockOperationRowParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return nil, fmt.Errorf("lock operation: %w", err)
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
		return nil, fmt.Errorf("check operation lease: %w", err)
	}
	if active && row.Token != k.Token {
		return nil, pkg.Wrap(pkg.ErrConflict, nil, "operation is running")
	}
	if row.Token != k.Token {
		if err := q.TakeOverOperation(ctx, sqlc.TakeOverOperationParams{
			Token: k.Token, LeaseMs: r.operationLease.Milliseconds(), OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key,
		}); err != nil {
			return nil, fmt.Errorf("take over operation: %w", err)
		}
	}
	return nil, nil
}

func finishOperation(ctx context.Context, tx pgx.Tx, k model.MutationKey, status int, body any) (model.MutationResult, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return model.MutationResult{}, fmt.Errorf("encode operation result: %w", err)
	}
	code := int32(status) //nolint:gosec // HTTP status codes are small constants.
	updated, err := sqlc.New(tx).FinishOperationRow(ctx, sqlc.FinishOperationRowParams{
		Result: data, HttpStatus: &code, OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Token: k.Token,
	})
	if err != nil {
		return model.MutationResult{}, fmt.Errorf("store operation result: %w", err)
	}
	if updated != 1 {
		return model.MutationResult{}, pkg.ErrConflict
	}
	return model.MutationResult{Status: status, Body: data}, nil
}

// resolveMutation returns a durable result even if the commit acknowledgement
// was lost. A retry retains the same key and fingerprint and cannot duplicate it.
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

func lockObjects(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) (map[uuid.UUID]sqlc.StoredObject, error) {
	ids = slices.Clone(ids)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	ids = slices.Compact(ids)
	q := sqlc.New(tx)
	for _, id := range ids {
		if _, err := q.LockStoredObject(ctx, id); err != nil {
			return nil, HandlePgErr(err)
		}
	}
	objects := make(map[uuid.UUID]sqlc.StoredObject, len(ids))
	for _, id := range ids {
		o, err := q.ReadStoredObject(ctx, id)
		if err != nil {
			return nil, HandlePgErr(err)
		}
		objects[id] = o
	}
	return objects, nil
}

func (r *ProblemDraftsRepo) SaveDraft(ctx context.Context, k model.MutationKey, draft model.Draft) (model.MutationResult, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, false); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid draft")
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
		return model.MutationResult{}, HandlePgErr(err)
	}
	if revision != draft.Revision {
		return model.MutationResult{}, pkg.ErrConflict
	}
	org, err := q.ProblemOrganizationID(ctx, draft.ProblemID)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.NoPermission
	}
	old, err := readProblemDraft(ctx, q, draft.ProblemID)
	if err != nil {
		return model.MutationResult{}, err
	}
	ids := append(old.InputObjects(), draft.InputObjects()...)
	objects, err := lockObjects(ctx, tx, ids)
	if err != nil {
		return model.MutationResult{}, err
	}
	for _, id := range draft.InputObjects() {
		o := objects[id]
		if o.OrganizationID != org || o.State != "available" {
			return model.MutationResult{}, pkg.ErrConflict
		}
	}
	draft.Revision++
	if err := writeDraft(ctx, tx, draft); err != nil {
		return model.MutationResult{}, err
	}
	// Every reference change updates the object's unreferenced timestamp under
	// the same object lock. The query sees committed concurrent reference writers.
	if err := refreshObjectReferences(ctx, tx, ids); err != nil {
		return model.MutationResult{}, err
	}
	result, err := finishOperation(ctx, tx, k, 200, draft)
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}

func writeDraft(ctx context.Context, tx pgx.Tx, d model.Draft) error {
	q := sqlc.New(tx)
	if err := q.UpdateProblemDraftRow(ctx, sqlc.UpdateProblemDraftRowParams{
		ProblemID: d.ProblemID, Revision: d.Revision, Title: d.Title, ShortName: d.ShortName, Type: d.Type,
		TimeLimitMs: int32(d.TimeLimitMs), MemoryLimitMb: int32(d.MemoryLimitMb), MaxScore: d.MaxScore, //nolint:gosec // validated against quotas
		CheckerID: d.CheckerID, ValidatorID: d.ValidatorID, InteractorID: d.InteractorID, MainSolutionID: d.MainSolutionID,
	}); err != nil {
		return fmt.Errorf("update draft: %w", err)
	}
	// Children are replaced wholesale; revision locking makes the replacement atomic.
	for _, clear := range []func(context.Context, uuid.UUID) error{
		q.DeleteDraftTests, q.DeleteDraftAssets, q.DeleteDraftSubtaskDependencies,
		q.DeleteDraftSubtasks, q.DeleteDraftComponents, q.DeleteDraftFiles,
	} {
		if err := clear(ctx, d.ProblemID); err != nil {
			return fmt.Errorf("clear draft children: %w", err)
		}
	}
	for path, content := range d.Files {
		if err := q.InsertDraftFile(ctx, sqlc.InsertDraftFileParams{ProblemID: d.ProblemID, Path: path, Content: content}); err != nil {
			return fmt.Errorf("insert draft file: %w", err)
		}
	}
	for _, c := range d.Components {
		deps := c.Dependencies
		if deps == nil {
			deps = map[string]string{}
		}
		encoded, err := json.Marshal(deps)
		if err != nil {
			return fmt.Errorf("encode component dependencies: %w", err)
		}
		if err := q.InsertDraftComponent(ctx, sqlc.InsertDraftComponentParams{
			ProblemID: d.ProblemID, ID: c.ID, Role: c.Role, Path: c.Path, Language: c.Language, Dependencies: encoded,
		}); err != nil {
			return fmt.Errorf("insert draft component: %w", err)
		}
	}
	for _, g := range d.Subtasks {
		if err := q.InsertDraftSubtask(ctx, sqlc.InsertDraftSubtaskParams{
			ProblemID: d.ProblemID, ID: g.ID, Ordinal: int32(g.Ordinal), Name: g.Name, Points: g.Points, Policy: g.Policy, //nolint:gosec // bounded by quotas
		}); err != nil {
			return fmt.Errorf("insert draft subtask: %w", err)
		}
	}
	for _, g := range d.Subtasks {
		for _, dep := range g.DependsOn {
			if err := q.InsertDraftSubtaskDependency(ctx, sqlc.InsertDraftSubtaskDependencyParams{ProblemID: d.ProblemID, SubtaskID: g.ID, DependsOn: dep}); err != nil {
				return fmt.Errorf("insert subtask dependency: %w", err)
			}
		}
	}
	for _, t := range d.Tests {
		args := t.Input.Args
		if args == nil {
			args = []string{}
		}
		if err := q.InsertDraftTest(ctx, sqlc.InsertDraftTestParams{
			ProblemID: d.ProblemID, ID: t.ID, Ordinal: int32(t.Ordinal), SubtaskID: t.SubtaskID, Sample: t.Sample, Points: t.Points, //nolint:gosec // bounded by quotas
			InputMode: t.Input.Mode, InputObjectID: t.Input.ObjectID, GeneratorID: t.Input.ComponentID, GeneratorArgs: args, Seed: t.Input.Seed,
			AnswerMode: t.Answer.Mode, AnswerObjectID: t.Answer.ObjectID, SolutionID: t.Answer.ComponentID,
		}); err != nil {
			return fmt.Errorf("insert draft test: %w", err)
		}
	}
	for _, a := range d.Assets {
		if err := q.InsertDraftAsset(ctx, sqlc.InsertDraftAssetParams{ProblemID: d.ProblemID, Path: a.Path, ObjectID: a.ObjectID}); err != nil {
			return fmt.Errorf("insert draft asset: %w", err)
		}
	}
	return nil
}

func refreshObjectReferences(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) error {
	q := sqlc.New(tx)
	for _, id := range ids {
		if err := q.RefreshObjectReferences(ctx, id); err != nil {
			return fmt.Errorf("refresh object references: %w", err)
		}
	}
	return nil
}
