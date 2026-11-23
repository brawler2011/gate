package organizationsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type RequestStatus string

const (
	RequestStatusPending  RequestStatus = "pending"
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
	JoinPolicy  accessmodel.JoinPolicy
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrganizationMember struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           accessmodel.OrganizationRole
	Username       string
	Email          string
	CreatedAt      time.Time
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
	JoinPolicy  accessmodel.JoinPolicy
}

type UpdateOrganizationInput struct {
	Login       *string
	Name        *string
	Description *string
	AvatarURL   *string
	JoinPolicy  *accessmodel.JoinPolicy
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
	Role           accessmodel.OrganizationRole
}

type CreateOrganizationJoinRequestInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Message        *string
}
