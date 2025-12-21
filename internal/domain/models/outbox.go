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
	EventTypeSubmissionTest    OutboxEventType = "submission.test"
	EventTypeSubmissionTested  OutboxEventType = "submission.tested"
	EventTypeUserUpdated       OutboxEventType = "user.updated"
)

// SubmissionCreatedPayload represents the payload for submission.created events
type SubmissionCreatedPayload struct {
	SubmissionId uuid.UUID `json:"submission_id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	ContestId    uuid.UUID `json:"contest_id"`
	Language     int32     `json:"language"`
	CreatedBy    uuid.UUID `json:"created_by"`
}

// SubmissionTestPayload represents the payload for submission.test events
type SubmissionTestPayload struct {
	SubmissionId uuid.UUID `json:"submission_id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	ContestId    uuid.UUID `json:"contest_id"`
	Language     int32     `json:"language"`
	CreatedBy    uuid.UUID `json:"created_by"`
}

// SubmissionTestedPayload represents the payload for submission.tested events
type SubmissionTestedPayload struct {
	SubmissionId uuid.UUID `json:"submission_id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	ContestId    uuid.UUID `json:"contest_id"`
	State        State     `json:"state"`
	Score        int32     `json:"score"`
	TimeStat     int32     `json:"time_stat"`
	MemoryStat   int32     `json:"memory_stat"`
	Language     int32     `json:"language"`
	CreatedBy    uuid.UUID `json:"created_by"`
}

// UserUpdatedPayload represents the payload for user.updated events
type UserUpdatedPayload struct {
	UserId   uuid.UUID  `json:"user_id"`
	Username *string    `json:"username,omitempty"`
	Role     *UserRole  `json:"role,omitempty"`
	Email    *string    `json:"email,omitempty"`
	Name     *string    `json:"name,omitempty"`
	Surname  *string    `json:"surname,omitempty"`
	Bio      *string    `json:"bio,omitempty"`
	ImgId    *uuid.UUID `json:"img_id,omitempty"`
}

// TestProgressEventType represents the type of testing progress event
type TestProgressEventType string

const (
	TestProgressEventTestingStarted   TestProgressEventType = "testing_started"
	TestProgressEventTestCompleted    TestProgressEventType = "test_completed"
	TestProgressEventTestingCompleted TestProgressEventType = "testing_completed"
)

// TestProgressEvent represents a NATS event for testing progress updates
type TestProgressEvent struct {
	Type         TestProgressEventType `json:"type"`
	SubmissionId uuid.UUID             `json:"submission_id"`
	TestNumber   int                   `json:"test_number,omitempty"`
	TotalTests   int                   `json:"total_tests"`
	Passed       bool                  `json:"passed,omitempty"`
	State        State                 `json:"state,omitempty"`
}

// OutboxEvent represents an event in the outbox pattern
type OutboxEvent struct {
	ID            uuid.UUID         `json:"id"`
	AggregateID   uuid.UUID         `json:"aggregate_id"`
	AggregateType string            `json:"aggregate_type"`
	EventType     OutboxEventType   `json:"event_type"`
	Payload       []byte            `json:"payload"`
	Status        OutboxEventStatus `json:"status"`
	RetryCount    int32             `json:"retry_count"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
}
