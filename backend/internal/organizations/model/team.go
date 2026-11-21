package organizationsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type TeamPrivacy string

const (
	TeamPrivacySecret TeamPrivacy = "secret" // Only members can see the team
	TeamPrivacyClosed TeamPrivacy = "closed" // Anyone in org can see, but only invited can join
)

type Team struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	Privacy        TeamPrivacy
	ParentTeamID   *uuid.UUID
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
	Privacy        TeamPrivacy
	ParentTeamID   *uuid.UUID
}

type UpdateTeamInput struct {
	Name        *string
	Description *string
	Privacy     *TeamPrivacy
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
