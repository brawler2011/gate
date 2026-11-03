package problemsmodel

import (
	"time"

	"github.com/google/uuid"
)

// ProblemPermission defines team-level permissions for problems
type ProblemPermission string

const (
	ProblemPermissionAdmin ProblemPermission = "admin"
	ProblemPermissionWrite ProblemPermission = "write"
	ProblemPermissionRead  ProblemPermission = "read"
)

// ProblemTeam represents team-based access to a problem
type ProblemTeam struct {
	ProblemID  uuid.UUID
	TeamID     uuid.UUID
	Permission ProblemPermission
	TeamName   string
	TeamSlug   string
	CreatedAt  time.Time
}
