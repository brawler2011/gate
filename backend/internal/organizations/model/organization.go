package organizationsmodel

import (
	"time"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type OrganizationRole string

const (
	OrgRoleOwner  OrganizationRole = "owner"
	OrgRoleAdmin  OrganizationRole = "admin"
	OrgRoleMember OrganizationRole = "member"
)

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

type OrganizationMember struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           OrganizationRole
	Username       string
	Email          string
	CreatedAt      time.Time
}

type OrganizationInvitation struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	OrganizationName      string
	OrganizationLogin     string
	OrganizationAvatarURL *string
	UserID                uuid.UUID
	Username              string
	Email                 string
	InviterID             uuid.UUID
	InviterUsername       string
	Role                  OrganizationRole
	Status                RequestStatus
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

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

type CreateOrganizationInput struct {
	Login       string
	Name        string
	Description string
	AvatarURL   *string
	JoinPolicy  OrganizationJoinPolicy
}

type UpdateOrganizationInput struct {
	Login       *string
	Name        *string
	Description *string
	AvatarURL   *string
	JoinPolicy  *OrganizationJoinPolicy
}

type OrganizationFilter struct {
	Search   string
	Page     int32
	PageSize int32
}

type OrganizationList struct {
	Organizations []Organization
	Pagination    paging.Pagination
}

type AddOrganizationMemberInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           OrganizationRole
}

type CreateOrganizationInvitationInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	InviterID      uuid.UUID
	Role           OrganizationRole
}

type CreateOrganizationJoinRequestInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Message        *string
}
