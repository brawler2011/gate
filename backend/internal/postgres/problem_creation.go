package postgres

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// FinalizeCreate creates the problem from a draft whose uploaded objects the
// caller has stored; a template clone references its package's objects.
func (r *ProblemDraftsRepo) FinalizeCreate(ctx context.Context, k model.MutationKey, user uuid.UUID, draft model.Draft, objects []model.StoredObject) (model.MutationResult, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, nil); err != nil {
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
	if err := insertObjects(ctx, tx, objects); err != nil {
		return model.MutationResult{}, err
	}
	if err := writeDraft(ctx, tx, draft); err != nil {
		return model.MutationResult{}, err
	}
	result, err := finishOperation(ctx, tx, k, 200, map[string]uuid.UUID{"id": draft.ProblemID})
	if err != nil {
		return result, err
	}
	return r.resolveMutation(ctx, tx, k, result)
}
