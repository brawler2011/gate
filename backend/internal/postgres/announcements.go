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
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementsRepo struct {
	queries *sqlc.Queries
}

func NewAnnouncementsRepo(pool *pgxpool.Pool) *AnnouncementsRepo {
	return &AnnouncementsRepo{
		queries: sqlc.New(pool),
	}
}

func (r *AnnouncementsRepo) CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error) {
	id := uuid.New()
	row, err := r.queries.CreateContestAnnouncement(ctx, sqlc.CreateContestAnnouncementParams{
		ID:        id,
		ContestID: input.ContestID,
		ProblemID: input.ProblemID,
		AuthorID:  input.AuthorID,
		Title:     input.Title,
		Body:      input.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("create contest announcement in db: %w", err)
	}

	return r.GetAnnouncementByID(ctx, row.ID)
}

func (r *AnnouncementsRepo) GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestAnnouncement, error) {
	row, err := r.queries.GetContestAnnouncementByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("get contest announcement by id: %w", err)
	}

	return &contestsmodel.ContestAnnouncement{
		ID:             row.ID,
		ContestID:      row.ContestID,
		ProblemID:      row.ProblemID,
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

func (r *AnnouncementsRepo) ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*contestsmodel.ContestAnnouncementsList, error) {
	offset := Offset(page, pageSize)

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

	announcements := make([]contestsmodel.ContestAnnouncement, len(rows))
	for i, row := range rows {
		announcements[i] = contestsmodel.ContestAnnouncement{
			ID:             row.ID,
			ContestID:      row.ContestID,
			ProblemID:      row.ProblemID,
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

	return &contestsmodel.ContestAnnouncementsList{
		Announcements: announcements,
		Pagination:    paging.NewPagination(page, pageSize, safeInt32(total)),
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
