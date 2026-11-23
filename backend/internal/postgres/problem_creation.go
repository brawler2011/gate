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

func (r *ProblemDraftsRepo) FinalizeCreate(ctx context.Context, org, user uuid.UUID, draft model.Draft, objects []model.StoredObject) error {
	if err := draft.ComputeScores(); err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "invalid weights")
	}
	if err := draft.Validate(r.quotas, nil); err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "invalid template")
	}
	_, err := serializableRetry(func() (struct{}, error) { return struct{}{}, r.finalizeCreate(ctx, org, user, draft, objects) })
	return err
}

func (r *ProblemDraftsRepo) finalizeCreate(ctx context.Context, org, user uuid.UUID, draft model.Draft, objects []model.StoredObject) error {
	tx, err := beginSerializable(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := access.Organization(ctx, commandAccess(q), org, accessmodel.OrganizationCreate); err != nil {
		return err
	}
	repo := &ProblemsRepo{queries: q}
	if err := repo.CreateProblem(ctx, &model.CreateProblemParams{ID: draft.ProblemID, OrganizationID: org, Title: draft.Title, ShortName: draft.ShortName, Visibility: model.ProblemVisibilityPrivate}); err != nil {
		return err
	}
	if err := repo.CreateProblemMember(ctx, &model.CreateProblemMemberParams{ProblemID: draft.ProblemID, UserID: user, Role: accessmodel.ProblemOwner}); err != nil {
		return err
	}
	if err := insertObjects(ctx, tx, objects); err != nil {
		return err
	}
	if err := writeDraft(ctx, tx, draft, objectIDs(objects)); err != nil {
		return err
	}
	return commitTx(ctx, tx)
}
