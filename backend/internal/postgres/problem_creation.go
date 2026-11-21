package postgres

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (r *ProblemDraftsRepo) FinalizeCreate(ctx context.Context, k model.MutationKey, user uuid.UUID, draft model.Draft, objects []model.StoredObject) (model.MutationResult, error) {
	if err := draft.ComputeScores(); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, nil); err != nil {
		return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid template")
	}
	return authorizedMutation(func() (model.MutationResult, error) { return r.finalizeCreate(ctx, k, user, draft, objects) })
}

func (r *ProblemDraftsRepo) finalizeCreate(ctx context.Context, k model.MutationKey, user uuid.UUID, draft model.Draft, objects []model.StoredObject) (model.MutationResult, error) {
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
	if _, err := access.Organization(ctx, Access{q}, k.OrganizationID, accessmodel.OrganizationManage); err != nil {
		return model.MutationResult{}, err
	}
	repo := &ProblemsRepo{queries: q}
	if err := repo.CreateProblem(ctx, &model.CreateProblemParams{ID: draft.ProblemID, OrganizationID: k.OrganizationID, Title: draft.Title, ShortName: draft.ShortName, Visibility: model.ProblemVisibilityPrivate}); err != nil {
		return model.MutationResult{}, err
	}
	if err := repo.CreateProblemMember(ctx, &model.CreateProblemMemberParams{ProblemID: draft.ProblemID, UserID: user, Role: accessmodel.ProblemOwner}); err != nil {
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
