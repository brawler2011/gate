package contestsmodel

import (
	"time"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type ClarificationStatus string

const (
	ClarificationStatusPending  ClarificationStatus = "pending"
	ClarificationStatusAnswered ClarificationStatus = "answered"
)

type ContestClarification struct {
	ID                 uuid.UUID           `json:"id"`
	ContestID          uuid.UUID           `json:"contest_id"`
	ProblemID          *uuid.UUID          `json:"problem_id,omitempty"`
	ProblemTitle       *string             `json:"problem_title,omitempty"`
	ProblemLetter      *string             `json:"problem_letter,omitempty"`
	UserID             uuid.UUID           `json:"user_id"`
	Username           string              `json:"username"`
	Question           string              `json:"question"`
	Answer             *string             `json:"answer,omitempty"`
	AnsweredBy         *uuid.UUID          `json:"answered_by,omitempty"`
	AnsweredByUsername *string             `json:"answered_by_username,omitempty"`
	Status             ClarificationStatus `json:"status"`
	CreatedAt          time.Time           `json:"created_at"`
	AnsweredAt         *time.Time          `json:"answered_at,omitempty"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

type CreateContestClarificationInput struct {
	ContestID uuid.UUID
	ProblemID *uuid.UUID
	Question  string
}

type AnswerContestClarificationInput struct {
	ClarificationID       uuid.UUID
	ContestID             uuid.UUID
	Answer                string
	PublishAsAnnouncement bool
	AnnouncementTitle     string
}

type ContestClarificationsFilter struct {
	ProblemID *uuid.UUID
	Status    *string
	Page      int32
	PageSize  int32
}

const (
	ContestEventClarificationCreated  = "contest.clarification_created"
	ContestEventClarificationAnswered = "contest.clarification_answered"
)

type ContestClarificationCreatedEvent struct {
	ID            uuid.UUID  `json:"id"`
	ContestID     uuid.UUID  `json:"contest_id"`
	ProblemID     *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle  *string    `json:"problem_title,omitempty"`
	ProblemLetter *string    `json:"problem_letter,omitempty"`
	UserID        uuid.UUID  `json:"user_id"`
	Username      string     `json:"username"`
	Question      string     `json:"question"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ContestClarificationAnsweredEvent struct {
	ID                 uuid.UUID  `json:"id"`
	ContestID          uuid.UUID  `json:"contest_id"`
	ProblemID          *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle       *string    `json:"problem_title,omitempty"`
	ProblemLetter      *string    `json:"problem_letter,omitempty"`
	UserID             uuid.UUID  `json:"user_id"`
	Username           string     `json:"username"`
	Question           string     `json:"question"`
	Answer             string     `json:"answer"`
	AnsweredBy         uuid.UUID  `json:"answered_by"`
	AnsweredByUsername string     `json:"answered_by_username"`
	AnsweredAt         time.Time  `json:"answered_at"`
}

func (c ContestClarification) CreatedEvent() ContestClarificationCreatedEvent {
	return ContestClarificationCreatedEvent{ID: c.ID, ContestID: c.ContestID, ProblemID: c.ProblemID, ProblemTitle: c.ProblemTitle, ProblemLetter: c.ProblemLetter,
		UserID: c.UserID, Username: c.Username, Question: c.Question, CreatedAt: c.CreatedAt}
}

func (c ContestClarification) AnsweredEvent() ContestClarificationAnsweredEvent {
	e := ContestClarificationAnsweredEvent{ID: c.ID, ContestID: c.ContestID, ProblemID: c.ProblemID, ProblemTitle: c.ProblemTitle, ProblemLetter: c.ProblemLetter,
		UserID: c.UserID, Username: c.Username, Question: c.Question, AnsweredAt: c.CreatedAt}
	if c.Answer != nil {
		e.Answer = *c.Answer
	}
	if c.AnsweredBy != nil {
		e.AnsweredBy = *c.AnsweredBy
	}
	if c.AnsweredByUsername != nil {
		e.AnsweredByUsername = *c.AnsweredByUsername
	}
	if c.AnsweredAt != nil {
		e.AnsweredAt = *c.AnsweredAt
	}
	return e
}

type ContestClarificationsList struct {
	Clarifications []ContestClarification
	Pagination     paging.Pagination
}
