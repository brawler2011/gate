package models

import (
	"time"

	"github.com/google/uuid"
)

type ContestDraft struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	ContestID uuid.UUID `json:"contest_id"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ContestDraftCreation struct {
	ContestID uuid.UUID
	UserID    uuid.UUID
	Code      string
}

type ContestDraftsFilter struct {
	ContestID uuid.UUID
	UserID    uuid.UUID
	Page      int32
	PageSize  int32
}

type ContestDraftsList struct {
	Drafts     []ContestDraft `json:"drafts"`
	Pagination Pagination     `json:"pagination"`
}
