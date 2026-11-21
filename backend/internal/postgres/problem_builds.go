package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ProblemBuildOptions = model.BuildOptions

func (r *ProblemDraftsRepo) EnqueueBuild(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, options ProblemBuildOptions) (model.MutationResult, error) {
	if options.MaxAttempts < 1 || options.Duration <= 0 || options.Lease <= 0 {
		return model.MutationResult{}, pkg.ErrInvalid
	}
	return authorizedMutation(func() (model.MutationResult, error) { return r.enqueueBuild(ctx, k, id, expected, options) })
}

func (r *ProblemDraftsRepo) enqueueBuild(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, options ProblemBuildOptions) (model.MutationResult, error) {
	tx, err := beginSerializable(ctx, r.pool)
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
	if err := authorizeEdit(ctx, q, id); err != nil {
		return model.MutationResult{}, err
	}
	revision, err := q.LockProblemDraft(ctx, id)
	if err != nil {
		return model.MutationResult{}, dbError("enqueue build", err)
	}
	if revision != expected {
		return model.MutationResult{}, pkg.ErrStaleRevision
	}
	org, err := q.ProblemOrganizationID(ctx, id)
	if err != nil {
		return model.MutationResult{}, dbError("enqueue build", err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.ErrForbidden
	}
	draft, err := readProblemDraft(ctx, q, id)
	if err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.ValidateBuild(r.quotas, options.Languages); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "draft cannot be built")
	}
	snapshot := model.BuildSnapshot{OrganizationID: org, Draft: draft, Limits: options.Limits, InputObjects: draft.InputObjects()}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return model.MutationResult{}, dbError("encode build snapshot", err)
	}
	buildID := uuid.New()
	if err := q.InsertProblemBuild(ctx, sqlc.InsertProblemBuildParams{
		ID: buildID, ProblemID: id, OrganizationID: org, Revision: revision, Snapshot: data,
		MaxAttempts: int32(options.MaxAttempts), DurationMs: options.Duration.Milliseconds(), //nolint:gosec // validated small attempt count
	}); err != nil {
		return model.MutationResult{}, dbError("insert build", err)
	}
	for _, oid := range snapshot.InputObjects {
		if err := q.InsertBuildInput(ctx, sqlc.InsertBuildInputParams{BuildID: buildID, OrganizationID: org, ObjectID: oid}); err != nil {
			return model.MutationResult{}, objectReferenceErr(err, "insert build input")
		}
	}
	result, err := finishOperation(ctx, tx, k, 202, map[string]any{"build_id": buildID, "revision": revision})
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}

func mapProblemBuild(b sqlc.ProblemBuild) (model.ProblemBuild, error) {
	result := model.ProblemBuild{ID: b.ID, ProblemID: b.ProblemID, OrganizationID: b.OrganizationID, Revision: b.Revision, State: b.State, Attempt: int(b.Attempt), MaxAttempts: int(b.MaxAttempts), LeaseToken: b.LeaseToken, LeaseDeadline: b.LeaseDeadline, Deadline: b.Deadline, CreatedAt: b.CreatedAt, FinishedAt: b.FinishedAt, Diagnostic: b.Diagnostic}
	if err := json.Unmarshal(b.Snapshot, &result.Snapshot); err != nil {
		return result, dbError("decode build snapshot", err)
	}
	if b.Solutions != nil {
		if err := json.Unmarshal(b.Solutions, &result.Solutions); err != nil {
			return result, dbError("decode build solutions", err)
		}
	}
	if b.ProgramTests != nil {
		if err := json.Unmarshal(b.ProgramTests, &result.ProgramTests); err != nil {
			return result, dbError("decode build program tests", err)
		}
	}
	return result, nil
}

func (r *ProblemDraftsRepo) GetBuild(ctx context.Context, id uuid.UUID) (model.ProblemBuild, error) {
	row, err := sqlc.New(r.pool).ReadProblemBuild(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, dbError("get build", err)
	}
	return mapProblemBuild(row)
}

func (r *ProblemDraftsRepo) ClaimBuild(ctx context.Context, lease time.Duration) (model.ProblemBuild, error) {
	if lease <= 0 {
		return model.ProblemBuild{}, pkg.ErrInvalid
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.ProblemBuild{}, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	id, err := q.NextProblemBuild(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ProblemBuild{}, nil
	}
	if err != nil {
		return model.ProblemBuild{}, dbError("select queued build", err)
	}
	exhausted, err := q.BuildExhausted(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, dbError("check build budget", err)
	}
	if exhausted {
		if err := q.FailExhaustedBuild(ctx, id); err != nil {
			return model.ProblemBuild{}, dbError("fail exhausted build", err)
		}
		return model.ProblemBuild{}, commitTx(ctx, tx)
	}
	token := uuid.New()
	if err := q.StartBuildAttempt(ctx, sqlc.StartBuildAttemptParams{Token: &token, LeaseMs: lease.Milliseconds(), ID: id}); err != nil {
		return model.ProblemBuild{}, dbError("start build attempt", err)
	}
	row, err := q.ReadProblemBuild(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, dbError("read claimed build", err)
	}
	build, err := mapProblemBuild(row)
	if err != nil {
		return build, err
	}
	if err := commitTx(ctx, tx); err != nil {
		return model.ProblemBuild{}, err
	}
	return build, nil
}

func (r *ProblemDraftsRepo) HeartbeatBuild(ctx context.Context, id, token uuid.UUID, lease time.Duration) error {
	if lease <= 0 {
		return pkg.ErrInvalid
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockProblemBuild(ctx, id); err != nil {
		return dbError("heartbeat build", err)
	}
	renewed, err := q.RenewBuildLease(ctx, sqlc.RenewBuildLeaseParams{LeaseMs: lease.Milliseconds(), ID: id, Token: &token})
	if err != nil {
		return dbError("renew build lease", err)
	}
	if renewed != 1 {
		return pkg.ErrConflict
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) FinishBuildAttempt(ctx context.Context, id, token uuid.UUID, diagnostic json.RawMessage, retry bool) error {
	if len(diagnostic) > 131072 || !json.Valid(diagnostic) {
		return pkg.ErrInvalid
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockProblemBuild(ctx, id); err != nil {
		return dbError("finish build attempt", err)
	}
	active, err := q.BuildAttemptActive(ctx, sqlc.BuildAttemptActiveParams{ID: id, LeaseToken: &token})
	if err != nil {
		return dbError("check build attempt", err)
	}
	if !active {
		return pkg.ErrConflict
	}
	if err := q.FinishBuildAttemptRow(ctx, sqlc.FinishBuildAttemptRowParams{Retry: retry, Diagnostic: diagnostic, ID: id, Token: &token}); err != nil {
		return dbError("finish build attempt", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) RecordBuildSolutions(ctx context.Context, id, token uuid.UUID, results []model.SolutionResult) error {
	data, err := json.Marshal(results)
	if err != nil {
		return dbError("encode build solutions", err)
	}
	updated, err := sqlc.New(r.pool).RecordBuildSolutions(ctx, sqlc.RecordBuildSolutionsParams{Solutions: data, ID: id, Token: &token})
	if err != nil {
		return dbError("record build solutions", err)
	}
	if updated != 1 {
		return pkg.ErrConflict
	}
	return nil
}

func (r *ProblemDraftsRepo) RecordBuildProgramTests(ctx context.Context, id, token uuid.UUID, results []model.ProgramTestResult) error {
	data, err := json.Marshal(results)
	if err != nil {
		return dbError("encode build program tests", err)
	}
	updated, err := sqlc.New(r.pool).RecordBuildProgramTests(ctx, sqlc.RecordBuildProgramTestsParams{ProgramTests: data, ID: id, Token: &token})
	if err != nil {
		return dbError("record build program tests", err)
	}
	if updated != 1 {
		return pkg.ErrConflict
	}
	return nil
}

func (r *ProblemDraftsRepo) RecoverBuilds(ctx context.Context) error {
	ids, err := sqlc.New(r.pool).ListRecoverableBuildIDs(ctx)
	if err != nil {
		return dbError("list recoverable builds", err)
	}
	for _, id := range ids {
		if err := r.recoverBuild(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProblemDraftsRepo) recoverBuild(ctx context.Context, id uuid.UUID) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return dbError("recover build", err)
	}
	if b.State != "running" || b.LeaseToken == nil {
		return nil
	}
	expired, err := q.BuildAttemptExpired(ctx, id)
	if err != nil {
		return dbError("check build attempt", err)
	}
	if !expired {
		return nil
	}
	if err := q.RequeueExpiredBuild(ctx, id); err != nil {
		return dbError("requeue expired build", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) CancelBuild(ctx context.Context, k model.MutationKey, id uuid.UUID) (model.MutationResult, error) {
	return authorizedMutation(func() (model.MutationResult, error) { return r.cancelBuild(ctx, k, id) })
}

func (r *ProblemDraftsRepo) cancelBuild(ctx context.Context, k model.MutationKey, id uuid.UUID) (model.MutationResult, error) {
	tx, err := beginSerializable(ctx, r.pool)
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
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return model.MutationResult{}, dbError("cancel build", err)
	}
	if b.OrganizationID != k.OrganizationID {
		return model.MutationResult{}, pkg.ErrForbidden
	}
	if err := authorizeEdit(ctx, q, b.ProblemID); err != nil {
		return model.MutationResult{}, err
	}
	if b.State != "queued" && b.State != "running" {
		return model.MutationResult{}, pkg.ErrBuildFinished
	}
	if err := q.CancelBuildRow(ctx, id); err != nil {
		return model.MutationResult{}, dbError("cancel build", err)
	}
	result, err := finishOperation(ctx, tx, k, 200, map[string]any{"build_id": id, "state": "cancelled"})
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}

func (r *ProblemDraftsRepo) ListBuilds(ctx context.Context, id uuid.UUID) ([]model.ProblemBuild, error) {
	ids, err := sqlc.New(r.pool).ListProblemBuildIDs(ctx, id)
	if err != nil {
		return nil, dbError("list builds", err)
	}
	result := make([]model.ProblemBuild, 0, len(ids))
	for _, bid := range ids {
		b, err := r.GetBuild(ctx, bid)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}
