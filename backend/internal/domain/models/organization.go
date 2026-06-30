package models

import (
	"time"

	"github.com/google/uuid"
)

// OrganizationRole defines roles within an organization
type OrganizationRole string

const (
	OrgRoleOwner  OrganizationRole = "owner"
	OrgRoleAdmin  OrganizationRole = "admin"
	OrgRoleMember OrganizationRole = "member"
)

// Organization represents a school, university, or company
type Organization struct {
	ID          uuid.UUID
	Login       string
	Name        string
	Description string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrganizationMember represents a user's membership in an organization
type OrganizationMember struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           OrganizationRole
	Username       string
	Email          string
	CreatedAt      time.Time
}

// CreateOrganizationInput is the input for creating an organization
type CreateOrganizationInput struct {
	Login       string
	Name        string
	Description string
	AvatarURL   *string
	CreatorID   uuid.UUID // The user who creates the org becomes the owner
}

// UpdateOrganizationInput is the input for updating an organization
type UpdateOrganizationInput struct {
	Name        *string
	Description *string
	AvatarURL   *string
}

// OrganizationFilter is used for filtering organizations
type OrganizationFilter struct {
	Search   string
	Page     int32
	PageSize int32
}

// OrganizationList is the paginated list of organizations
type OrganizationList struct {
	Organizations []Organization
	Pagination    Pagination
}

// AddOrganizationMemberInput is the input for adding a member to an organization
type AddOrganizationMemberInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           OrganizationRole
}
