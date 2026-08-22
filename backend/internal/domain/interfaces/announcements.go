package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AnnouncementsRepo interface {
	WithTx(tx pgx.Tx) AnnouncementsRepo
	CreateAnnouncement(ctx context.Context, input *models.CreateContestAnnouncementInput) (*models.ContestAnnouncement, error)
	GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*models.ContestAnnouncement, error)
	ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*models.ContestAnnouncementsList, error)
	DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error
}
