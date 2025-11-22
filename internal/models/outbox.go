package models

import (
	"time"

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

// OutboxEvent represents an event in the outbox table
type OutboxEvent struct {
	Id            uuid.UUID         `db:"id"`
	AggregateId   uuid.UUID         `db:"aggregate_id"`
	AggregateType string            `db:"aggregate_type"`
	EventType     OutboxEventType   `db:"event_type"`
	Payload       []byte            `db:"payload"` // jsonb stored as []byte
	Status        OutboxEventStatus `db:"status"`
	CreatedAt     time.Time         `db:"created_at"`
	ProcessedAt   *time.Time        `db:"processed_at"`
	RetryCount    int               `db:"retry_count"`
	ErrorMessage  *string           `db:"error_message"`
}

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

