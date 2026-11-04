package observer

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

type AuthUC interface {
	Authenticate(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error)
}

type ContestReader interface {
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)
}

type Scoreboard interface {
	ProjectScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)
	GetContestScoreboardUser(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)

	// Contest Join Requests

}

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*accessmodel.ContestPermissions, error)
}
