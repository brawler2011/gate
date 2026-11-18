package problemsmodel

import (
	"time"

	"github.com/google/uuid"
)

type ProblemPermission string

const (
	ProblemPermissionAdmin ProblemPermission = "admin"
	ProblemPermissionWrite ProblemPermission = "write"
	ProblemPermissionRead  ProblemPermission = "read"
)

type ProblemTeam struct {
	ProblemID  uuid.UUID
	TeamID     uuid.UUID
	Permission ProblemPermission
	TeamName   string
	TeamSlug   string
	CreatedAt  time.Time
}
