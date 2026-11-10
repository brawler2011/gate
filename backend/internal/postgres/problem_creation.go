package postgres

import (
	"context"
	"fmt"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (r *ProblemDraftsRepo) FinalizeCreate(ctx context.Context, k model.MutationKey, user uuid.UUID, draft model.Draft) (model.MutationResult, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, false); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid template")
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
	allowed, err := bindAuthorization(q).HasOrganizationPermission(ctx, k.OrganizationID, user, accessmodel.ActionCreateProblem)
	if err != nil {
		return model.MutationResult{}, err
	}
	if !allowed {
		return model.MutationResult{}, pkg.NoPermission
	}
	repo := &ProblemsRepo{queries: q}
	if err := repo.CreateProblem(ctx, &model.CreateProblemParams{ID: draft.ProblemID, OrganizationID: k.OrganizationID, OwnerID: &user, Title: draft.Title, ShortName: draft.ShortName, Visibility: model.ProblemVisibilityPrivate}); err != nil {
		return model.MutationResult{}, err
	}
	if err := repo.CreateProblemMember(ctx, &model.CreateProblemMemberParams{ProblemID: draft.ProblemID, UserID: user, Role: model.ProblemRoleOwner}); err != nil {
		return model.MutationResult{}, err
	}
	assigned, err := operationUploads(ctx, tx, k)
	if err != nil {
		return model.MutationResult{}, err
	}
	all := append(draft.InputObjects(), assigned...)
	objects, err := lockObjects(ctx, tx, all)
	if err != nil {
		return model.MutationResult{}, err
	}
	for _, oid := range all {
		if objects[oid].OrganizationID != k.OrganizationID {
			return model.MutationResult{}, pkg.NoPermission
		}
	}
	for _, oid := range assigned {
		published, err := q.PublishUploadedObjectIfActive(ctx, oid)
		if err != nil {
			return model.MutationResult{}, fmt.Errorf("publish uploaded object: %w", err)
		}
		if published != 1 {
			return model.MutationResult{}, pkg.ErrConflict
		}
	}
	for _, oid := range draft.InputObjects() {
		available, err := q.ObjectIsAvailable(ctx, oid)
		if err != nil {
			return model.MutationResult{}, fmt.Errorf("check object state: %w", err)
		}
		if !available {
			return model.MutationResult{}, pkg.ErrConflict
		}
	}
	if err := writeDraft(ctx, tx, draft); err != nil {
		return model.MutationResult{}, err
	}
	if err := clearOperationUploads(ctx, tx, k, assigned); err != nil {
		return model.MutationResult{}, err
	}
	result, err := finishOperation(ctx, tx, k, 200, map[string]uuid.UUID{"id": draft.ProblemID})
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}
