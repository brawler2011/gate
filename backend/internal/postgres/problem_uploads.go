package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func operationUploads(ctx context.Context, tx pgx.Tx, k model.MutationKey) ([]uuid.UUID, error) {
	ids, err := sqlc.New(tx).ListOperationUploads(ctx, sqlc.ListOperationUploadsParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return nil, fmt.Errorf("list operation uploads: %w", err)
	}
	return ids, nil
}

func clearOperationUploads(ctx context.Context, tx pgx.Tx, k model.MutationKey, ids []uuid.UUID) error {
	q := sqlc.New(tx)
	if err := q.ClearOperationUploadID(ctx, sqlc.ClearOperationUploadIDParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key}); err != nil {
		return fmt.Errorf("clear operation upload: %w", err)
	}
	if err := q.DeleteOperationUploads(ctx, sqlc.DeleteOperationUploadsParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key}); err != nil {
		return fmt.Errorf("delete operation uploads: %w", err)
	}
	return refreshObjectReferences(ctx, tx, ids)
}

func (r *ProblemDraftsRepo) ReserveUploads(ctx context.Context, k model.MutationKey, specs []model.UploadSpec, ttl time.Duration) (model.UploadReservation, error) {
	if ttl <= 0 || len(specs) > r.quotas.Files {
		return model.UploadReservation{}, pkg.ErrBadInput
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.UploadReservation{}, err
	}
	defer rollback(ctx, tx)
	replay, err := r.reserveOperation(ctx, tx, k)
	if err != nil {
		return model.UploadReservation{}, err
	}
	if replay != nil {
		return model.UploadReservation{Replay: replay}, nil
	}
	existing, err := operationUploads(ctx, tx, k)
	if err != nil {
		return model.UploadReservation{}, err
	}
	locked, err := lockObjects(ctx, tx, existing)
	if err != nil {
		return model.UploadReservation{}, err
	}
	q := sqlc.New(tx)
	reuse := len(existing) == len(specs)
	for i, id := range existing {
		o := locked[id]
		if i >= len(specs) || o.Sha256 != specs[i].SHA256 || o.Size != specs[i].Size || o.Purpose != specs[i].Purpose {
			return model.UploadReservation{}, pkg.ErrConflict
		}
		active, err := q.UploadObjectActive(ctx, id)
		if err != nil {
			return model.UploadReservation{}, fmt.Errorf("check upload deadline: %w", err)
		}
		reuse = reuse && active
	}
	result := model.UploadReservation{Objects: []model.StoredObject{}}
	if reuse {
		for _, id := range existing {
			result.Objects = append(result.Objects, mapStoredObject(locked[id]))
		}
		return result, commitTx(ctx, tx)
	}
	if err := clearOperationUploads(ctx, tx, k, existing); err != nil {
		return result, err
	}
	for i, spec := range specs {
		id := uuid.New()
		if err := q.InsertUploadObject(ctx, sqlc.InsertUploadObjectParams{
			ID: id, OrganizationID: k.OrganizationID, Purpose: spec.Purpose, Key: fmt.Sprintf("objects/%s/%s", k.OrganizationID, id),
			Sha256: spec.SHA256, Size: spec.Size, TtlMs: ttl.Milliseconds(),
		}); err != nil {
			return result, fmt.Errorf("register upload object: %w", err)
		}
		if err := q.InsertOperationUpload(ctx, sqlc.InsertOperationUploadParams{
			OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, Ordinal: int32(i + 1), ObjectID: id, //nolint:gosec // bounded by quota
		}); err != nil {
			return result, fmt.Errorf("assign upload to operation: %w", err)
		}
		if i == 0 {
			if err := q.SetOperationUploadID(ctx, sqlc.SetOperationUploadIDParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key, UploadID: &id}); err != nil {
				return result, fmt.Errorf("record operation upload: %w", err)
			}
		}
		object, err := q.ReadStoredObject(ctx, id)
		if err != nil {
			return result, fmt.Errorf("read upload object: %w", err)
		}
		result.Objects = append(result.Objects, mapStoredObject(object))
	}
	return result, commitTx(ctx, tx)
}

func applyUploadChange(d *model.Draft, change model.UploadChange, ids []uuid.UUID) error {
	required := 1
	if change.Part == "pair" {
		required = 2
	}
	if len(ids) != required {
		return pkg.ErrBadInput
	}
	if change.Part == "asset" {
		if change.Test != nil || !model.ValidDraftPath(change.AssetPath) {
			return pkg.ErrBadInput
		}
		asset := model.DraftAsset{Path: change.AssetPath, ObjectID: ids[0]}
		found := false
		for i, a := range d.Assets {
			if a.Path == asset.Path {
				d.Assets[i] = asset
				found = true
				break
			}
		}
		if !found {
			d.Assets = append(d.Assets, asset)
		}
		return nil
	}
	if change.Test == nil || change.AssetPath != "" {
		return pkg.ErrBadInput
	}
	test := *change.Test
	switch change.Part {
	case "input":
		if test.Input.Mode != "manual" {
			return pkg.ErrBadInput
		}
		test.Input.ObjectID = &ids[0]
	case "answer":
		if test.Answer.Mode != "provided" {
			return pkg.ErrBadInput
		}
		test.Answer.ObjectID = &ids[0]
	case "pair":
		if test.Input.Mode != "manual" || test.Answer.Mode != "provided" {
			return pkg.ErrBadInput
		}
		test.Input.ObjectID = &ids[0]
		test.Answer.ObjectID = &ids[1]
	default:
		return pkg.ErrBadInput
	}
	found := false
	for i, t := range d.Tests {
		if t.ID == test.ID {
			d.Tests[i] = test
			found = true
			break
		}
	}
	if !found {
		d.Tests = append(d.Tests, test)
	}
	return nil
}

// FinalizeUploads is the only transition to available for author uploads. The
// caller has checked every completed object's persisted size and SHA-256.
func (r *ProblemDraftsRepo) FinalizeUploads(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, change model.UploadChange) (model.MutationResult, error) {
	return r.finalizeUploadDraft(ctx, k, id, expected, func(d *model.Draft, ids []uuid.UUID) error { return applyUploadChange(d, change, ids) })
}
func (r *ProblemDraftsRepo) finalizeUploadDraft(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, apply func(*model.Draft, []uuid.UUID) error) (model.MutationResult, error) {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.MutationResult{}, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	// Finalization never takes over an expired operation. A new executor must
	// reserve explicitly, and the previous executor loses its fencing token.
	operation, err := q.LockOperationForFinalize(ctx, sqlc.LockOperationForFinalizeParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if operation.Fingerprint != k.Fingerprint {
		return model.MutationResult{}, pkg.ErrConflict
	}
	if operation.HttpStatus != nil {
		return model.MutationResult{Status: int(*operation.HttpStatus), Body: operation.Result}, nil
	}
	if operation.Token != k.Token {
		return model.MutationResult{}, pkg.ErrConflict
	}
	revision, err := q.LockProblemDraft(ctx, id)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if revision != expected {
		return model.MutationResult{}, pkg.ErrConflict
	}
	draft, err := readProblemDraft(ctx, q, id)
	if err != nil {
		return model.MutationResult{}, err
	}
	org, err := q.ProblemOrganizationID(ctx, id)
	if err != nil {
		return model.MutationResult{}, HandlePgErr(err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.NoPermission
	}
	assigned, err := operationUploads(ctx, tx, k)
	if err != nil {
		return model.MutationResult{}, err
	}
	previous := draft.InputObjects()
	if err := apply(&draft, assigned); err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, false); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid upload change")
	}
	all := append(append(previous, draft.InputObjects()...), assigned...)
	objects, err := lockObjects(ctx, tx, all)
	if err != nil {
		return model.MutationResult{}, err
	}
	for _, oid := range draft.InputObjects() {
		o := objects[oid]
		if o.OrganizationID != org || (o.State != "available" && !slices.Contains(assigned, oid)) {
			return model.MutationResult{}, pkg.ErrConflict
		}
	}
	for _, oid := range assigned {
		active, err := q.UploadObjectActive(ctx, oid)
		if err != nil {
			return model.MutationResult{}, fmt.Errorf("check upload deadline: %w", err)
		}
		if !active {
			return model.MutationResult{}, pkg.ErrConflict
		}
		if err := q.PublishUploadedObject(ctx, oid); err != nil {
			return model.MutationResult{}, fmt.Errorf("publish uploaded object: %w", err)
		}
	}
	draft.Revision++
	if err := writeDraft(ctx, tx, draft); err != nil {
		return model.MutationResult{}, err
	}
	if err := clearOperationUploads(ctx, tx, k, assigned); err != nil {
		return model.MutationResult{}, err
	}
	if err := refreshObjectReferences(ctx, tx, all); err != nil {
		return model.MutationResult{}, err
	}
	result, err := finishOperation(ctx, tx, k, 200, draft)
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}

func (r *ProblemDraftsRepo) CloseExpiredUploads(ctx context.Context) error {
	keys, err := sqlc.New(r.pool).ListExpiredOperations(ctx)
	if err != nil {
		return fmt.Errorf("list expired operations: %w", err)
	}
	for _, key := range keys {
		if err := r.closeExpiredUpload(ctx, model.MutationKey{OrganizationID: key.OrganizationID, Operation: key.Operation, Key: key.Key}); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProblemDraftsRepo) closeExpiredUpload(ctx context.Context, k model.MutationKey) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockOperation(ctx, sqlc.LockOperationParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key}); err != nil {
		return fmt.Errorf("lock operation: %w", err)
	}
	expired, err := q.OperationExpired(ctx, sqlc.OperationExpiredParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return fmt.Errorf("check operation lease: %w", err)
	}
	if !expired {
		return nil
	}
	ids, err := operationUploads(ctx, tx, k)
	if err != nil {
		return err
	}
	if _, err := lockObjects(ctx, tx, ids); err != nil {
		return err
	}
	if err := clearOperationUploads(ctx, tx, k, ids); err != nil {
		return err
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) FinalizeImport(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, draft model.Draft) (model.MutationResult, error) {
	return r.finalizeUploadDraft(ctx, k, id, expected, func(d *model.Draft, _ []uuid.UUID) error {
		draft.ProblemID = id
		draft.Revision = expected
		*d = draft
		return nil
	})
}
