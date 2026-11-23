package contestsmodel

import (
	"time"

	paging "github.com/brawler2011/gate/backend/internal/page"
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
	Title     string
	Body      string
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

func (a ContestAnnouncement) CreatedEvent() ContestAnnouncementCreatedEvent {
	return ContestAnnouncementCreatedEvent{ID: a.ID, ContestID: a.ContestID, ProblemID: a.ProblemID, ProblemTitle: a.ProblemTitle, ProblemLetter: a.ProblemLetter,
		AuthorID: a.AuthorID, AuthorUsername: a.AuthorUsername, Title: a.Title, Body: a.Body, CreatedAt: a.CreatedAt}
}

type ContestAnnouncementDeletedEvent struct {
	ID        uuid.UUID `json:"id"`
	ContestID uuid.UUID `json:"contest_id"`
}

type ContestAnnouncementsList struct {
	Announcements []ContestAnnouncement
	Pagination    paging.Pagination
}
