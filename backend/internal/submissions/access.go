package submissions

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Permissions interface {
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
	GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*accessmodel.ContestPermissions, error)
}

func (uc *SubmissionsUseCase) requireContest(ctx context.Context, id uuid.UUID, action accessmodel.ContestAction) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, id, user.Id, action)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.NoPermission
	}
	return nil
}
