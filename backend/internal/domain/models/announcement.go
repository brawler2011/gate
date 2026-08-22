package models

import (
	"time"

	"github.com/google/uuid"
)

type ContestAnnouncement struct {
	ID             uuid.UUID  `json:"id"`
	ContestID      uuid.UUID  `json:"contest_id"`
	ProblemID      *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle   *string    `json:"problem_title,omitempty"`
	ProblemLetter  *string    `json:"problem_letter,omitempty"`
	AuthorID       uuid.UUID  `json:"author_id"`
	AuthorUsername string     `json:"author_username"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateContestAnnouncementInput struct {
	ContestID uuid.UUID
	ProblemID *uuid.UUID
	AuthorID  uuid.UUID
	Title     string
	Body      string
}

type ContestAnnouncementsList struct {
	Announcements []ContestAnnouncement
	Pagination    Pagination
}

const (
	ContestEventAnnouncementCreated = "contest.announcement_created"
	ContestEventAnnouncementDeleted = "contest.announcement_deleted"
)

type ContestAnnouncementCreatedEvent struct {
	ID             uuid.UUID  `json:"id"`
	ContestID      uuid.UUID  `json:"contest_id"`
	ProblemID      *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle   *string    `json:"problem_title,omitempty"`
	ProblemLetter  *string    `json:"problem_letter,omitempty"`
	AuthorID       uuid.UUID  `json:"author_id"`
	AuthorUsername string     `json:"author_username"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ContestAnnouncementDeletedEvent struct {
	ID        uuid.UUID `json:"id"`
	ContestID uuid.UUID `json:"contest_id"`
}
