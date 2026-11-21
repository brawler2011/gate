package access

import (
	"context"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

// Facts carry the actor's effective role from the *_access views; site administration is applied by the policy.
type OrganizationFacts struct {
	ID         uuid.UUID
	Role       m.OrganizationRole
	JoinPolicy m.JoinPolicy
}

type TeamFacts struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Role           m.TeamRole
	Member         bool
	Secret         bool
}

type ProblemFacts struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Role           m.ProblemRole
	Public         bool
}

// Now is the database clock of the read, so a transaction decides at the instant it records.
type ContestFacts struct {
	Contest contestsmodel.Contest
	Role    m.ContestRole
	Now     time.Time
	// Blocked lists problems the actor may no longer submit to.
	Blocked []uuid.UUID
}

// Loader reads facts the same way over the pool and inside transactions; missing ids are absent from the map.
type Loader interface {
	Actor(ctx context.Context, id uuid.UUID) (m.User, error)
	OrganizationFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]OrganizationFacts, error)
	TeamFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]TeamFacts, error)
	ProblemFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]ProblemFacts, error)
	ContestFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]ContestFacts, error)
}
