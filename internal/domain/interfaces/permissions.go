package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*models.ContestPermissions, error)
	GetProblemPermissions(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ProblemPermissions, error)
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action models.ContestAction) (bool, error)
	HasProblemPermission(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, action models.ProblemAction) (bool, error)
}
