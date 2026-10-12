// Package queuecontext carries existing outbox metadata to event publishers.
package queuecontext

import (
	"context"
	"github.com/google/uuid"
	"time"
)

const EventIDHeader = "Gate-Outbox-Event-ID"
const QueuedAtHeader = "Gate-Queued-At"

type key struct{}
type Metadata struct {
	EventID  uuid.UUID
	QueuedAt time.Time
}

func With(ctx context.Context, meta Metadata) context.Context {
	return context.WithValue(ctx, key{}, meta)
}
func Get(ctx context.Context) Metadata { meta, _ := ctx.Value(key{}).(Metadata); return meta }
