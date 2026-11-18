package postgres

import (
	"context"
	"fmt"
	"slices"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BeginOperation reserves an idempotent operation whose bytes are uploaded
// before it is finalized. It returns the stored result of a finished
// operation, so a replay uploads nothing.
func (r *ProblemDraftsRepo) BeginOperation(ctx context.Context, k model.MutationKey) (*model.MutationResult, error) {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	replay, err := r.reserveOperation(ctx, tx, k)
	if err != nil || replay != nil {
		return replay, err
	}
	return nil, commitTx(ctx, tx)
}

// insertObjects records uploaded objects. The caller references them in the
// same transaction, so a row never exists without a reference or its bytes.
func insertObjects(ctx context.Context, tx pgx.Tx, objects []model.StoredObject) error {
	q := sqlc.New(tx)
	for _, o := range objects {
		if err := q.InsertStoredObject(ctx, sqlc.InsertStoredObjectParams{
			ID: o.ID, OrganizationID: o.OrganizationID, Purpose: o.Purpose, Key: o.Key, Sha256: o.SHA256, Size: o.Size,
		}); err != nil {
			return fmt.Errorf("insert stored object: %w", err)
		}
	}
	return nil
}

func objectIDs(objects []model.StoredObject) []uuid.UUID {
	ids := make([]uuid.UUID, len(objects))
	for i, o := range objects {
		ids[i] = o.ID
	}
	return ids
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
		if change.Test != nil || !model.ValidFileName(change.AssetName) {
			return pkg.ErrBadInput
		}
		asset := model.DraftAsset{Name: change.AssetName, ObjectID: ids[0]}
		found := false
		for i, a := range d.Assets {
			if a.Name == asset.Name {
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
	if change.Test == nil || change.AssetName != "" {
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

// FinalizeUploads applies an upload change whose bytes the caller has stored
// under the objects' keys with a verified SHA-256.
func (r *ProblemDraftsRepo) FinalizeUploads(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, change model.UploadChange, objects []model.StoredObject) (model.MutationResult, error) {
	return r.finalizeUploadDraft(ctx, k, id, expected, objects, func(d *model.Draft) error { return applyUploadChange(d, change, objectIDs(objects)) })
}

func (r *ProblemDraftsRepo) FinalizeImport(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, draft model.Draft, objects []model.StoredObject) (model.MutationResult, error) {
	return r.finalizeUploadDraft(ctx, k, id, expected, objects, func(d *model.Draft) error {
		draft.ProblemID = id
		draft.Revision = expected
		*d = draft
		return nil
	})
}

func (r *ProblemDraftsRepo) finalizeUploadDraft(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, objects []model.StoredObject, apply func(*model.Draft) error) (model.MutationResult, error) {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.MutationResult{}, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	// Finalization never takes over an expired operation. A new executor must
	// begin explicitly, and the previous executor loses its fencing token.
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
	before := draft
	before.Statements = slices.Clone(draft.Statements)
	if err := apply(&draft); err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, &before); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid upload change")
	}
	if err := insertObjects(ctx, tx, objects); err != nil {
		return model.MutationResult{}, err
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
