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
	EventTypeSubmissionTest    OutboxEventType = "submission.test"
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

// SubmissionTestPayload represents the payload for submission.test events
type SubmissionTestPayload struct {
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