package postgres

import (
	"context"
	"maps"
	"slices"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A completed operation replays its stored result before any upload.
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

// Insert objects with their first references in the same transaction.
func insertObjects(ctx context.Context, tx pgx.Tx, objects []model.StoredObject) error {
	q := sqlc.New(tx)
	for _, o := range objects {
		if err := q.InsertStoredObject(ctx, sqlc.InsertStoredObjectParams{
			ID: o.ID, OrganizationID: o.OrganizationID, Purpose: o.Purpose, Key: o.Key, Sha256: o.SHA256, Size: o.Size,
		}); err != nil {
			return dbError("insert stored object", err)
		}
	}
	return nil
}

func upsert[T any, K comparable](list *[]T, entry T, key func(T) K) {
	for i := range *list {
		if key((*list)[i]) == key(entry) {
			(*list)[i] = entry
			return
		}
	}
	*list = append(*list, entry)
}

func applyUploadChange(d *model.Draft, change model.UploadChange, objects map[string]model.StoredObject) error {
	set := 0
	for _, present := range []bool{change.AssetName != "", change.Test != nil, change.CheckerTest != nil, change.ValidatorTest != nil} {
		if present {
			set++
		}
	}
	if set != 1 || len(objects) == 0 || (change.Validator != "") != (change.ValidatorTest != nil) {
		return pkg.ErrInvalid
	}
	ids := map[string]*uuid.UUID{}
	for slot, o := range objects {
		if change.Purpose(slot) == "" {
			return pkg.ErrInvalid
		}
		ids[slot] = &o.ID
	}
	switch {
	case change.AssetName != "":
		if !model.ValidFileName(change.AssetName) {
			return pkg.ErrInvalid
		}
		upsert(&d.Assets, model.DraftAsset{Name: change.AssetName, ObjectID: *ids["asset"]}, func(a model.DraftAsset) string { return a.Name })
	case change.Test != nil:
		test := *change.Test
		if id := ids["input"]; id != nil {
			if test.Input.Mode != "manual" {
				return pkg.ErrInvalid
			}
			test.Input.ObjectID = id
		}
		if id := ids["answer"]; id != nil {
			if test.Answer.Mode != "provided" {
				return pkg.ErrInvalid
			}
			test.Answer.ObjectID = id
		}
		upsert(&d.Tests, test, func(t model.DraftTest) uuid.UUID { return t.ID })
	case change.CheckerTest != nil:
		if d.Checker == nil {
			return pkg.ErrInvalid
		}
		test := *change.CheckerTest
		for slot, field := range map[string]*uuid.UUID{"input": &test.Input, "output": &test.Output, "answer": &test.Answer} {
			if id := ids[slot]; id != nil {
				*field = *id
			}
		}
		upsert(&d.Checker.Tests, test, func(t model.DraftCheckerTest) uuid.UUID { return t.ID })
	default:
		i := slices.IndexFunc(d.Validators, func(v model.DraftValidator) bool { return v.Name == change.Validator })
		if i < 0 {
			return pkg.ErrInvalid
		}
		test := *change.ValidatorTest
		test.Input = *ids["input"]
		upsert(&d.Validators[i].Tests, test, func(t model.DraftValidatorTest) uuid.UUID { return t.ID })
	}
	return nil
}

// The caller must store uploads with verified SHA-256 values before finalization.
func (r *ProblemDraftsRepo) FinalizeUploads(ctx context.Context, k model.MutationKey, id uuid.UUID, expected int64, change model.UploadChange, objects map[string]model.StoredObject) (model.MutationResult, error) {
	return r.finalizeUploadDraft(ctx, k, id, expected, slices.Collect(maps.Values(objects)), func(d *model.Draft) error { return applyUploadChange(d, change, objects) })
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
	// Finalization cannot take over expired operations; a new executor must begin explicitly.
	operation, err := q.LockOperationForFinalize(ctx, sqlc.LockOperationForFinalizeParams{OrganizationID: k.OrganizationID, Operation: k.Operation, Key: k.Key})
	if err != nil {
		return model.MutationResult{}, dbError("finalize upload draft", err)
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
		return model.MutationResult{}, dbError("finalize upload draft", err)
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
		return model.MutationResult{}, dbError("finalize upload draft", err)
	}
	if org != k.OrganizationID {
		return model.MutationResult{}, pkg.ErrForbidden
	}
	before := draft
	before.Statements = slices.Clone(draft.Statements)
	if err := apply(&draft); err != nil {
		return model.MutationResult{}, err
	}
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, &before); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid upload change")
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
