package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
