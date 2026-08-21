package pg

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftsRepo struct {
	queries *sqlc.Queries
}

func NewDraftsRepo(db *pgxpool.Pool) *DraftsRepo {
	return &DraftsRepo{
		queries: sqlc.New(db),
	}
}

func (r *DraftsRepo) WithTx(tx pgx.Tx) interfaces.DraftsRepo {
	return &DraftsRepo{
		queries: sqlc.New(tx),
	}
}

func (r *DraftsRepo) CreateDraft(ctx context.Context, creation *models.ContestDraftCreation) (uuid.UUID, error) {
	id, err := r.queries.CreateDraft(ctx, sqlc.CreateDraftParams{
		ContestID: creation.ContestID,
		UserID:    creation.UserID,
		ProblemID: creation.ProblemID,
		Language:  creation.Language,
		Code:      creation.Code,
	})
	if err != nil {
		return uuid.Nil, HandlePgErr(err)
	}
	return id, nil
}

func (r *DraftsRepo) GetDraft(ctx context.Context, id uuid.UUID) (models.ContestDraft, error) {
	row, err := r.queries.GetDraft(ctx, id)
	if err != nil {
		return models.ContestDraft{}, HandlePgErr(err)
	}

	var username string
	if row.Username != nil {
		username = *row.Username
	}

	var problemTitle string
	if row.ProblemTitle != nil {
		problemTitle = *row.ProblemTitle
	}

	return models.ContestDraft{
		ID:           row.ID,
		ContestID:    row.ContestID,
		UserID:       row.UserID,
		Username:     username,
		ProblemID:    row.ProblemID,
		ProblemTitle: problemTitle,
		Position:     row.Position,
		Language:     row.Language,
		Code:         row.Code,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func (r *DraftsRepo) GetDraftsCountByProblem(ctx context.Context, contestID, userID, problemID uuid.UUID) (int64, error) {
	count, err := r.queries.GetDraftsCountByProblem(ctx, sqlc.GetDraftsCountByProblemParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
	})
	if err != nil {
		return 0, HandlePgErr(err)
	}
	return count, nil
}

func (r *DraftsRepo) ListDrafts(ctx context.Context, filter models.ContestDraftsFilter) ([]models.ContestDraft, int32, error) {
	totalCount, err := r.queries.CountDrafts(ctx, sqlc.CountDraftsParams{
		ContestID: filter.ContestID,
		UserID:    nullableUUIDToPgtype(filter.UserID),
		ProblemID: nullableUUIDToPgtype(filter.ProblemID),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	rows, err := r.queries.ListDrafts(ctx, sqlc.ListDraftsParams{
		ContestID: filter.ContestID,
		UserID:    nullableUUIDToPgtype(filter.UserID),
		ProblemID: nullableUUIDToPgtype(filter.ProblemID),
		OffsetVal: Offset(filter.Page, filter.PageSize),
		LimitVal:  filter.PageSize,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	drafts := make([]models.ContestDraft, 0, len(rows))
	for _, row := range rows {
		var username string
		if row.Username != nil {
			username = *row.Username
		}

		var problemTitle string
		if row.ProblemTitle != nil {
			problemTitle = *row.ProblemTitle
		}

		drafts = append(drafts, models.ContestDraft{
			ID:           row.ID,
			ContestID:    row.ContestID,
			UserID:       row.UserID,
			Username:     username,
			ProblemID:    row.ProblemID,
			ProblemTitle: problemTitle,
			Position:     row.Position,
			Language:     row.Language,
			Code:         row.Code,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}

	return drafts, safeInt32(totalCount), nil
}

func (r *DraftsRepo) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.DeleteDraft(ctx, id); err != nil {
		return HandlePgErr(err)
	}
	return nil
}
