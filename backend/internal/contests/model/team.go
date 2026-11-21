package contestsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

type ContestTeam struct {
	ContestID uuid.UUID
	TeamID    uuid.UUID
	Role      accessmodel.ContestRole
	TeamName  string
	TeamSlug  string
	CreatedAt time.Time
}
