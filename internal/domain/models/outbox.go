package models

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEventStatus = string

const (
	OutboxEventStatusPending    OutboxEventStatus = "pending"
	OutboxEventStatusProcessing OutboxEventStatus = "processing"
	OutboxEventStatusCompleted  OutboxEventStatus = "completed"
	OutboxEventStatusFailed     OutboxEventStatus = "failed"
)

type OutboxEventType = string

const (
	OutboxEventSubmissionCreated OutboxEventType = "submission.created"
)

type OutboxEvent struct {
	Id           uuid.UUID         `json:"id"`
	AggregateID  uuid.UUID         `json:"aggregate_id"`
	EventType    OutboxEventType   `json:"event_type"`
	Payload      []byte            `json:"payload"`
	Status       OutboxEventStatus `json:"status"`
	RetryCount   int32             `json:"retry_count"`
	ErrorMessage *string           `json:"error_message,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	LockedAt    *time.Time `json:"locked_at,omitempty"`
	DeadlineAt  *time.Time `json:"deadline_at,omitempty"`
}

type CreateOutboxEventParams struct {
	Id          uuid.UUID       `json:"id"`
	AggregateID uuid.UUID       `json:"aggregate_id"`
	EventType   OutboxEventType `json:"event_type"`
	Payload     []byte          `json:"payload"`
}
