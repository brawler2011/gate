package organizationsmodel

import (
	"time"

	"github.com/google/uuid"
)

type BatchCreateOrganizationUsersInput struct {
	OrgLogin string
	Prefix   string
	Count    int32
	TTLDays  *int32
}

type BatchCreatedUserItem struct {
	ID        uuid.UUID
	Username  string
	Password  string
	ExpiresAt *time.Time
}

type BatchCreateOrganizationUsersResult struct {
	Users []BatchCreatedUserItem
}
