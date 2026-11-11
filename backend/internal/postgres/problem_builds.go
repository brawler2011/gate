package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		return model.MutationResult{}, pkg.ErrBadInput
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
	revision, err := q.LockProblemDraft(ctx, id)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if revision != expected {
		return model.MutationResult{}, pkg.ErrConflict
	}
	org, err := q.ProblemOrganizationID(ctx, id)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.NoPermission
	}
	draft, err := readProblemDraft(ctx, q, id)
	if err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.Validate(r.quotas, true); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "draft cannot be built")
	}
	for _, c := range draft.Components {
		if _, ok := options.Toolchain.Languages[c.Language]; !ok {
			return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, nil, "component language is unavailable in pinned toolchain")
		}
	}
	snapshot := model.BuildSnapshot{SchemaVersion: model.DraftSnapshotVersion, OrganizationID: org, Draft: draft, Toolchain: options.Toolchain, Limits: options.Limits, InputObjects: draft.InputObjects()}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return model.MutationResult{}, fmt.Errorf("encode build snapshot: %w", err)
	}
	objects, err := lockObjects(ctx, tx, snapshot.InputObjects)
	if err != nil {
		return model.MutationResult{}, err
	}
	for _, o := range objects {
		if o.OrganizationID != org || o.State != "available" {
			return model.MutationResult{}, pkg.ErrConflict
		}
	}
	buildID := uuid.New()
	if err := q.InsertProblemBuild(ctx, sqlc.InsertProblemBuildParams{
		ID: buildID, ProblemID: id, OrganizationID: org, Revision: revision, Snapshot: data, SchemaVersion: model.DraftSnapshotVersion,
		ToolchainDigest: options.Toolchain.Digest, MaxAttempts: int32(options.MaxAttempts), DurationMs: options.Duration.Milliseconds(), //nolint:gosec // validated small attempt count
	}); err != nil {
		return model.MutationResult{}, fmt.Errorf("insert build: %w", err)
	}
	for _, oid := range snapshot.InputObjects {
		if err := q.InsertBuildInput(ctx, sqlc.InsertBuildInputParams{BuildID: buildID, ObjectID: oid}); err != nil {
			return model.MutationResult{}, fmt.Errorf("insert build input: %w", err)
		}
	}
	if err := refreshObjectReferences(ctx, tx, snapshot.InputObjects); err != nil {
		return model.MutationResult{}, err
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
		return result, fmt.Errorf("decode build snapshot: %w", err)
	}
	return result, nil
}

func (r *ProblemDraftsRepo) GetBuild(ctx context.Context, id uuid.UUID) (model.ProblemBuild, error) {
	row, err := sqlc.New(r.pool).ReadProblemBuild(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, HandlePgErr(err)
	}
	return mapProblemBuild(row)
}

func (r *ProblemDraftsRepo) ClaimBuild(ctx context.Context, lease time.Duration) (model.ProblemBuild, error) {
	if lease <= 0 {
		return model.ProblemBuild{}, pkg.ErrBadInput
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
		return model.ProblemBuild{}, fmt.Errorf("select queued build: %w", err)
	}
	exhausted, err := q.BuildExhausted(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, fmt.Errorf("check build budget: %w", err)
	}
	if exhausted {
		if err := q.FailExhaustedBuild(ctx, id); err != nil {
			return model.ProblemBuild{}, fmt.Errorf("fail exhausted build: %w", err)
		}
		return model.ProblemBuild{}, commitTx(ctx, tx)
	}
	token := uuid.New()
	if err := q.StartBuildAttempt(ctx, sqlc.StartBuildAttemptParams{Token: &token, LeaseMs: lease.Milliseconds(), ID: id}); err != nil {
		return model.ProblemBuild{}, fmt.Errorf("start build attempt: %w", err)
	}
	row, err := q.ReadProblemBuild(ctx, id)
	if err != nil {
		return model.ProblemBuild{}, fmt.Errorf("read claimed build: %w", err)
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
		return pkg.ErrBadInput
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockProblemBuild(ctx, id); err != nil {
		return HandlePgErr(err)
	}
	renewed, err := q.RenewBuildLease(ctx, sqlc.RenewBuildLeaseParams{LeaseMs: lease.Milliseconds(), ID: id, Token: &token})
	if err != nil {
		return fmt.Errorf("renew build lease: %w", err)
	}
	if renewed != 1 {
		return pkg.ErrConflict
	}
	return commitTx(ctx, tx)
}

// closeBuildAttempt runs only under the Build lock and releases exactly the
// closing attempt's outputs. Input roots and newer attempts are untouched.
func closeBuildAttempt(ctx context.Context, tx pgx.Tx, id, token uuid.UUID) error {
	q := sqlc.New(tx)
	ids, err := q.ListAttemptOutputObjects(ctx, sqlc.ListAttemptOutputObjectsParams{BuildID: id, AttemptToken: &token})
	if err != nil {
		return fmt.Errorf("list attempt outputs: %w", err)
	}
	if _, err := lockObjects(ctx, tx, ids); err != nil {
		return err
	}
	if err := q.DeleteAttemptOutputs(ctx, sqlc.DeleteAttemptOutputsParams{BuildID: id, AttemptToken: &token}); err != nil {
		return fmt.Errorf("release attempt outputs: %w", err)
	}
	return refreshObjectReferences(ctx, tx, ids)
}

func (r *ProblemDraftsRepo) FinishBuildAttempt(ctx context.Context, id, token uuid.UUID, diagnostic json.RawMessage, retry bool) error {
	if len(diagnostic) > 131072 || !json.Valid(diagnostic) {
		return pkg.ErrBadInput
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockProblemBuild(ctx, id); err != nil {
		return HandlePgErr(err)
	}
	active, err := q.BuildAttemptActive(ctx, sqlc.BuildAttemptActiveParams{ID: id, LeaseToken: &token})
	if err != nil {
		return fmt.Errorf("check build attempt: %w", err)
	}
	if !active {
		return pkg.ErrConflict
	}
	if err := closeBuildAttempt(ctx, tx, id, token); err != nil {
		return err
	}
	if err := q.FinishBuildAttemptRow(ctx, sqlc.FinishBuildAttemptRowParams{Retry: retry, Diagnostic: diagnostic, ID: id, Token: &token}); err != nil {
		return fmt.Errorf("finish build attempt: %w", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) RecoverBuilds(ctx context.Context) error {
	// Candidate selection never locks an object before its owner Build.
	ids, err := sqlc.New(r.pool).ListRecoverableBuildIDs(ctx)
	if err != nil {
		return fmt.Errorf("list recoverable builds: %w", err)
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
		return HandlePgErr(err)
	}
	if b.State != "running" || b.LeaseToken == nil {
		return nil
	}
	expired, err := q.BuildAttemptExpired(ctx, id)
	if err != nil {
		return fmt.Errorf("check build attempt: %w", err)
	}
	if !expired {
		return nil
	}
	if err := closeBuildAttempt(ctx, tx, id, *b.LeaseToken); err != nil {
		return err
	}
	if err := q.RequeueExpiredBuild(ctx, id); err != nil {
		return fmt.Errorf("requeue expired build: %w", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) CancelBuild(ctx context.Context, k model.MutationKey, id uuid.UUID) (model.MutationResult, error) {
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
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if b.OrganizationID != k.OrganizationID {
		return model.MutationResult{}, pkg.NoPermission
	}
	if b.State != "queued" && b.State != "running" {
		return model.MutationResult{}, pkg.ErrConflict
	}
	if b.LeaseToken != nil {
		if err := closeBuildAttempt(ctx, tx, id, *b.LeaseToken); err != nil {
			return model.MutationResult{}, err
		}
	}
	if err := q.CancelBuildRow(ctx, id); err != nil {
		return model.MutationResult{}, fmt.Errorf("cancel build: %w", err)
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
		return nil, fmt.Errorf("list builds: %w", err)
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
