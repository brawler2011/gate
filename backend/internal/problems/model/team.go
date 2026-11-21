package problemsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/google/uuid"
)

type ProblemTeam struct {
	ProblemID uuid.UUID
	TeamID    uuid.UUID
	Role      accessmodel.ProblemRole
	TeamName  string
	TeamSlug  string
	CreatedAt time.Time
}
