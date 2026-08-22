package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotificationTypeOrgInvitation          NotificationType = "org_invitation"
	NotificationTypeOrgJoinRequest         NotificationType = "org_join_request"
	NotificationTypeOrgJoinApproved        NotificationType = "org_join_approved"
	NotificationTypeOrgJoinRejected        NotificationType = "org_join_rejected"
	NotificationTypeContestJoinRequest     NotificationType = "contest_join_request"
	NotificationTypeContestJoinApproved    NotificationType = "contest_join_approved"
	NotificationTypeContestJoinRejected    NotificationType = "contest_join_rejected"
	NotificationTypeSystem                 NotificationType = "system"
)

type Notification struct {
	ID        uuid.UUID              `json:"id"`
	UserID    uuid.UUID              `json:"user_id"`
	Type      NotificationType       `json:"type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Link      *string                `json:"link,omitempty"`
	Data      map[string]interface{} `json:"data"`
	IsRead    bool                   `json:"is_read"`
	CreatedAt time.Time              `json:"created_at"`
}

type CreateNotificationInput struct {
	UserID uuid.UUID
	Type   NotificationType
	Title  string
	Body   string
	Link   *string
	Data   map[string]interface{}
}

type NotificationFilter struct {
	Page       int32
	PageSize   int32
	UnreadOnly bool
}

type NotificationsList struct {
	Notifications []Notification
	Pagination    Pagination
}

func MarshalNotificationData(data map[string]interface{}) ([]byte, error) {
	if data == nil {
		return []byte("{}"), nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal notification data: %w", err)
	}
	return bytes, nil
}

func UnmarshalNotificationData(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return make(map[string]interface{})
	}
	var res map[string]interface{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return make(map[string]interface{})
	}
	return res
}
