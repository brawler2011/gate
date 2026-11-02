package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewAnnouncementsRepo(pool *pgxpool.Pool) *AnnouncementsRepo {
	return &AnnouncementsRepo{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *AnnouncementsRepo) WithTx(tx pgx.Tx) interfaces.AnnouncementsRepo {
	return &AnnouncementsRepo{
		pool:    r.pool,
		queries: r.queries.WithTx(tx),
	}
}

func (r *AnnouncementsRepo) CreateAnnouncement(ctx context.Context, input *models.CreateContestAnnouncementInput) (*models.ContestAnnouncement, error) {
	id := uuid.New()
	row, err := r.queries.CreateContestAnnouncement(ctx, sqlc.CreateContestAnnouncementParams{
		ID:        id,
		ContestID: input.ContestID,
		ProblemID: uuidPtrToPgtypeUUID(input.ProblemID),
		AuthorID:  input.AuthorID,
		Title:     input.Title,
		Body:      input.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("create contest announcement in db: %w", err)
	}

	return r.GetAnnouncementByID(ctx, row.ID)
}

func (r *AnnouncementsRepo) GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*models.ContestAnnouncement, error) {
	row, err := r.queries.GetContestAnnouncementByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("get contest announcement by id: %w", err)
	}

	return &models.ContestAnnouncement{
		ID:             row.ID,
		ContestID:      row.ContestID,
		ProblemID:      pgtypeUUIDToPtr(row.ProblemID),
		ProblemTitle:   row.ProblemTitle,
		ProblemLetter:  ordinalToLetter(row.ProblemOrdinal),
		AuthorID:       row.AuthorID,
		AuthorUsername: row.AuthorUsername,
		Title:          row.Title,
		Body:           row.Body,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func (r *AnnouncementsRepo) ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*models.ContestAnnouncementsList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	rows, err := r.queries.ListContestAnnouncements(ctx, sqlc.ListContestAnnouncementsParams{
		ContestID: contestID,
		Limit:     pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list contest announcements: %w", err)
	}

	total, err := r.queries.CountContestAnnouncements(ctx, contestID)
	if err != nil {
		return nil, fmt.Errorf("count contest announcements: %w", err)
	}

	announcements := make([]models.ContestAnnouncement, len(rows))
	for i, row := range rows {
		announcements[i] = models.ContestAnnouncement{
			ID:             row.ID,
			ContestID:      row.ContestID,
			ProblemID:      pgtypeUUIDToPtr(row.ProblemID),
			ProblemTitle:   row.ProblemTitle,
			ProblemLetter:  ordinalToLetter(row.ProblemOrdinal),
			AuthorID:       row.AuthorID,
			AuthorUsername: row.AuthorUsername,
			Title:          row.Title,
			Body:           row.Body,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		}
	}

	return &models.ContestAnnouncementsList{
		Announcements: announcements,
		Pagination:    models.NewPagination(page, pageSize, safeInt32(total)),
	}, nil
}

func (r *AnnouncementsRepo) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	err := r.queries.DeleteContestAnnouncement(ctx, sqlc.DeleteContestAnnouncementParams{
		ID:        id,
		ContestID: contestID,
	})
	if err != nil {
		return fmt.Errorf("delete contest announcement: %w", err)
	}
	return nil
}
