package events

import (
	"context"

	"github.com/google/uuid"
)

type Event struct {
	AggregateID uuid.UUID
	Type        string
	Payload     any
}

type Writer interface {
	Record(context.Context, Event) error
}
