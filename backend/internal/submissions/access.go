package submissions

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

type ContestAccess interface {
	CanContest(context.Context, uuid.UUID, accessmodel.ContestAction) (bool, error)
}

type Permissions interface {
	CanContest(context.Context, uuid.UUID, accessmodel.ContestAction) (bool, error)
	GetContestPermissions(context.Context, uuid.UUID) (*accessmodel.ContestPermissions, error)
}
