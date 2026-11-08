package contestsmodel

import (
	"time"

	"github.com/google/uuid"
)

// ContestTeam represents team-based access to a contest
type ContestTeam struct {
	ContestID uuid.UUID
	TeamID    uuid.UUID
	Role      ContestRole
	TeamName  string
	TeamSlug  string
	CreatedAt time.Time
}
