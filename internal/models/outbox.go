package models

import (
	"github.com/google/uuid"
)

// OutboxEventStatus represents the status of an outbox event
type OutboxEventStatus string

const (
	OutboxEventStatusPending    OutboxEventStatus = "pending"
	OutboxEventStatusProcessing OutboxEventStatus = "processing"
	OutboxEventStatusCompleted  OutboxEventStatus = "completed"
	OutboxEventStatusFailed     OutboxEventStatus = "failed"
)

// OutboxEventType represents the type of event
type OutboxEventType string

const (
	EventTypeSubmissionCreated OutboxEventType = "submission.created"
	EventTypeSubmissionTested  OutboxEventType = "submission.tested"
)

// SubmissionCreatedPayload represents the payload for submission.created events
type SubmissionCreatedPayload struct {
	SubmissionId uuid.UUID `json:"submission_id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	ContestId    uuid.UUID `json:"contest_id"`
	Language     int64     `json:"language"`
	CreatedBy    uuid.UUID `json:"created_by"`
}

// SubmissionTestedPayload represents the payload for submission.tested events
type SubmissionTestedPayload struct {
	SubmissionId uuid.UUID `json:"submission_id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	ContestId    uuid.UUID `json:"contest_id"`
	State        State     `json:"state"`
	Score        int64     `json:"score"`
	TimeStat     int64     `json:"time_stat"`
	MemoryStat   int64     `json:"memory_stat"`
	Language     int64     `json:"language"`
	CreatedBy    uuid.UUID `json:"created_by"`
}
