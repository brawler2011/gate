package models

import (
	"time"

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
	UserID    uuid.UUID
	Question  string
}

type AnswerContestClarificationInput struct {
	ClarificationID       uuid.UUID
	ContestID             uuid.UUID
	Answer                string
	AnsweredBy            uuid.UUID
	PublishAsAnnouncement bool
	AnnouncementTitle     string
}

type ContestClarificationsFilter struct {
	ProblemID *uuid.UUID
	Status    *string
	Page      int32
	PageSize  int32
}

type ContestClarificationsList struct {
	Clarifications []ContestClarification
	Pagination     Pagination
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
