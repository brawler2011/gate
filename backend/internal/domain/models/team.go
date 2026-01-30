package models

import (
	"time"

	"github.com/google/uuid"
)

// TeamRole defines roles within a team
type TeamRole string

const (
	TeamRoleMaintainer TeamRole = "maintainer"
	TeamRoleMember     TeamRole = "member"
)

// TeamPrivacy defines team visibility
type TeamPrivacy string

const (
	TeamPrivacySecret TeamPrivacy = "secret" // Only members can see the team
	TeamPrivacyClosed TeamPrivacy = "closed" // Anyone in org can see, but only invited can join
)

// Team represents a group of users within an organization
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

// TeamMember represents a user's membership in a team
type TeamMember struct {
	TeamID    uuid.UUID
	UserID    uuid.UUID
	Role      TeamRole
	Username  string
	Email     string
	CreatedAt time.Time
}

// CreateTeamInput is the input for creating a team
type CreateTeamInput struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	Privacy        TeamPrivacy
	ParentTeamID   *uuid.UUID
}

// UpdateTeamInput is the input for updating a team
type UpdateTeamInput struct {
	Name        *string
	Description *string
	Privacy     *TeamPrivacy
}

// AddTeamMemberInput is the input for adding a member to a team
type AddTeamMemberInput struct {
	TeamID uuid.UUID
	UserID uuid.UUID
	Role   TeamRole
}

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

// ContestTeam represents team-based access to a contest
type ContestTeam struct {
	ContestID uuid.UUID
	TeamID    uuid.UUID
	Role      ContestRole
	TeamName  string
	TeamSlug  string
	CreatedAt time.Time
}
