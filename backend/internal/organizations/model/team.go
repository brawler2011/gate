package organizationsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type Team struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TeamMember struct {
	TeamID    uuid.UUID
	UserID    uuid.UUID
	Role      accessmodel.TeamRole
	Username  string
	Email     string
	CreatedAt time.Time
}

type CreateTeamInput struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
}

type UpdateTeamInput struct {
	Name        *string
	Description *string
}

type AddTeamMemberInput struct {
	TeamID uuid.UUID
	UserID uuid.UUID
	Role   accessmodel.TeamRole
}

type TeamsFilter struct {
	OrganizationID *uuid.UUID
	Search         string
	Page           int32
	PageSize       int32
}

type TeamsList struct {
	Teams      []Team
	Pagination paging.Pagination
}
