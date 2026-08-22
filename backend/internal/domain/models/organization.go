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

// OrganizationJoinPolicy defines how users can join an organization
type OrganizationJoinPolicy string

const (
	OrgJoinPolicyOpen       OrganizationJoinPolicy = "open"
	OrgJoinPolicyByRequest  OrganizationJoinPolicy = "by_request"
	OrgJoinPolicyInviteOnly OrganizationJoinPolicy = "invite_only"
)

type RequestStatus string

const (
	RequestStatusPending  RequestStatus = "pending"
	RequestStatusAccepted RequestStatus = "accepted"
	RequestStatusDeclined RequestStatus = "declined"
	RequestStatusApproved RequestStatus = "approved"
	RequestStatusRejected RequestStatus = "rejected"
	RequestStatusCanceled RequestStatus = "canceled"
)

// Organization represents a school, university, or company
type Organization struct {
	ID          uuid.UUID
	Login       string
	Name        string
	Description string
	AvatarURL   *string
	JoinPolicy  OrganizationJoinPolicy
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

// OrganizationInvitation represents an invite sent from an org admin to a user
type OrganizationInvitation struct {
	ID                     uuid.UUID
	OrganizationID         uuid.UUID
	OrganizationName       string
	OrganizationLogin      string
	OrganizationAvatarURL  *string
	UserID                 uuid.UUID
	Username               string
	Email                  string
	InviterID              uuid.UUID
	InviterUsername        string
	Role                   OrganizationRole
	Status                 RequestStatus
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// OrganizationJoinRequest represents a request submitted by a user to join an org
type OrganizationJoinRequest struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	OrganizationName  string
	OrganizationLogin string
	UserID            uuid.UUID
	Username          string
	Email             string
	Message           *string
	Status            RequestStatus
	ReviewedBy        *uuid.UUID
	ReviewerUsername  *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CreateOrganizationInput is the input for creating an organization
type CreateOrganizationInput struct {
	Login       string
	Name        string
	Description string
	AvatarURL   *string
	JoinPolicy  OrganizationJoinPolicy
	CreatorID   uuid.UUID // The user who creates the org becomes the owner
}

// UpdateOrganizationInput is the input for updating an organization
type UpdateOrganizationInput struct {
	Login       *string
	Name        *string
	Description *string
	AvatarURL   *string
	JoinPolicy  *OrganizationJoinPolicy
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

// CreateOrganizationInvitationInput is the input for inviting a user
type CreateOrganizationInvitationInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	InviterID      uuid.UUID
	Role           OrganizationRole
}

// CreateOrganizationJoinRequestInput is the input for requesting to join an org
type CreateOrganizationJoinRequestInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Message        *string
}
