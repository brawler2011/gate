package postgres

import (
	"context"
	"errors"
	"fmt"

	paging "github.com/brawler2011/gate/backend/internal/page"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClarificationsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewClarificationsRepo(pool *pgxpool.Pool) *ClarificationsRepo {
	return &ClarificationsRepo{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *ClarificationsRepo) WithTx(tx pgx.Tx) *ClarificationsRepo {
	return &ClarificationsRepo{
		pool:    r.pool,
		queries: r.queries.WithTx(tx),
	}
}

func (r *ClarificationsRepo) CreateClarification(ctx context.Context, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error) {
	id := uuid.New()
	row, err := r.queries.CreateContestClarification(ctx, sqlc.CreateContestClarificationParams{
		ID:        id,
		ContestID: input.ContestID,
		ProblemID: uuidPtrToPgtypeUUID(input.ProblemID),
		UserID:    input.UserID,
		Question:  input.Question,
	})
	if err != nil {
		return nil, fmt.Errorf("create contest clarification in db: %w", err)
	}

	return r.GetClarificationByID(ctx, row.ID)
}

func (r *ClarificationsRepo) GetClarificationByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestClarification, error) {
	row, err := r.queries.GetContestClarificationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("get contest clarification by id: %w", err)
	}

	return &contestsmodel.ContestClarification{
		ID:                 row.ID,
		ContestID:          row.ContestID,
		ProblemID:          pgtypeUUIDToPtr(row.ProblemID),
		ProblemTitle:       row.ProblemTitle,
		ProblemLetter:      ordinalToLetter(row.ProblemOrdinal),
		UserID:             row.UserID,
		Username:           row.Username,
		Question:           row.Question,
		Answer:             row.Answer,
		AnsweredBy:         pgtypeUUIDToPtr(row.AnsweredBy),
		AnsweredByUsername: row.AnsweredByUsername,
		Status:             contestsmodel.ClarificationStatus(row.Status),
		CreatedAt:          row.CreatedAt,
		AnsweredAt:         pgtypeTimestamptzToPtr(row.AnsweredAt),
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

func (r *ClarificationsRepo) ListClarificationsForUser(ctx context.Context, contestID, userID uuid.UUID, page, pageSize int32) (*contestsmodel.ContestClarificationsList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	rows, err := r.queries.ListContestClarificationsForUser(ctx, sqlc.ListContestClarificationsForUserParams{
		ContestID: contestID,
		UserID:    userID,
		Limit:     pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list contest clarifications for user: %w", err)
	}

	total, err := r.queries.CountContestClarificationsForUser(ctx, sqlc.CountContestClarificationsForUserParams{
		ContestID: contestID,
		UserID:    userID,
	})
	if err != nil {
		return nil, fmt.Errorf("count contest clarifications for user: %w", err)
	}

	clarifications := make([]contestsmodel.ContestClarification, len(rows))
	for i, row := range rows {
		clarifications[i] = contestsmodel.ContestClarification{
			ID:                 row.ID,
			ContestID:          row.ContestID,
			ProblemID:          pgtypeUUIDToPtr(row.ProblemID),
			ProblemTitle:       row.ProblemTitle,
			ProblemLetter:      ordinalToLetter(row.ProblemOrdinal),
			UserID:             row.UserID,
			Username:           row.Username,
			Question:           row.Question,
			Answer:             row.Answer,
			AnsweredBy:         pgtypeUUIDToPtr(row.AnsweredBy),
			AnsweredByUsername: row.AnsweredByUsername,
			Status:             contestsmodel.ClarificationStatus(row.Status),
			CreatedAt:          row.CreatedAt,
			AnsweredAt:         pgtypeTimestamptzToPtr(row.AnsweredAt),
			UpdatedAt:          row.UpdatedAt,
		}
	}

	return &contestsmodel.ContestClarificationsList{
		Clarifications: clarifications,
		Pagination:     paging.NewPagination(page, pageSize, safeInt32(total)),
	}, nil
}

func (r *ClarificationsRepo) ListClarificationsForModerator(ctx context.Context, contestID uuid.UUID, filter *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error) {
	page := int32(1)
	pageSize := int32(50)
	if filter != nil {
		if filter.Page >= 1 {
			page = filter.Page
		}
		if filter.PageSize >= 1 && filter.PageSize <= 100 {
			pageSize = filter.PageSize
		}
	}
	offset := (page - 1) * pageSize

	var problemID pgtype.UUID
	if filter != nil && filter.ProblemID != nil {
		problemID = pgtype.UUID{Bytes: *filter.ProblemID, Valid: true}
	}

	var status *string
	if filter != nil && filter.Status != nil && *filter.Status != "" {
		status = filter.Status
	}

	rows, err := r.queries.ListContestClarificationsForModerator(ctx, sqlc.ListContestClarificationsForModeratorParams{
		ContestID: contestID,
		ProblemID: problemID,
		Status:    status,
		Limit:     pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list contest clarifications for moderator: %w", err)
	}

	total, err := r.queries.CountContestClarificationsForModerator(ctx, sqlc.CountContestClarificationsForModeratorParams{
		ContestID: contestID,
		ProblemID: problemID,
		Status:    status,
	})
	if err != nil {
		return nil, fmt.Errorf("count contest clarifications for moderator: %w", err)
	}

	clarifications := make([]contestsmodel.ContestClarification, len(rows))
	for i, row := range rows {
		clarifications[i] = contestsmodel.ContestClarification{
			ID:                 row.ID,
			ContestID:          row.ContestID,
			ProblemID:          pgtypeUUIDToPtr(row.ProblemID),
			ProblemTitle:       row.ProblemTitle,
			ProblemLetter:      ordinalToLetter(row.ProblemOrdinal),
			UserID:             row.UserID,
			Username:           row.Username,
			Question:           row.Question,
			Answer:             row.Answer,
			AnsweredBy:         pgtypeUUIDToPtr(row.AnsweredBy),
			AnsweredByUsername: row.AnsweredByUsername,
			Status:             contestsmodel.ClarificationStatus(row.Status),
			CreatedAt:          row.CreatedAt,
			AnsweredAt:         pgtypeTimestamptzToPtr(row.AnsweredAt),
			UpdatedAt:          row.UpdatedAt,
		}
	}

	return &contestsmodel.ContestClarificationsList{
		Clarifications: clarifications,
		Pagination:     paging.NewPagination(page, pageSize, safeInt32(total)),
	}, nil
}

func (r *ClarificationsRepo) AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*contestsmodel.ContestClarification, error) {
	var ansPtr *string
	if answer != "" {
		ansPtr = &answer
	}
	ansByPtr := pgtype.UUID{Bytes: answeredBy, Valid: true}

	_, err := r.queries.AnswerContestClarification(ctx, sqlc.AnswerContestClarificationParams{
		Answer:     ansPtr,
		AnsweredBy: ansByPtr,
		ID:         id,
		ContestID:  contestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("answer contest clarification: %w", err)
	}

	return r.GetClarificationByID(ctx, id)
}
