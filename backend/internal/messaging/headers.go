package messaging

import (
	"context"
	"strconv"
	"time"

	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func Headers(ctx context.Context) nats.Header {
	headers := nats.Header{}
	if id := requestid.Get(ctx); id != "" {
		headers.Set(requestid.Header, id)
	}
	meta := queuecontext.Get(ctx)
	if meta.EventID != uuid.Nil {
		headers.Set("Nats-Msg-Id", meta.EventID.String())
		headers.Set(queuecontext.EventIDHeader, meta.EventID.String())
	}
	if !meta.QueuedAt.IsZero() {
		headers.Set(queuecontext.QueuedAtHeader, meta.QueuedAt.Format(time.RFC3339Nano))
	}
	if meta.AggregateID != uuid.Nil {
		headers.Set("Gate-Aggregate-ID", meta.AggregateID.String())
	}
	if meta.AggregateVersion > 0 {
		headers.Set("Gate-Aggregate-Version", strconv.FormatInt(meta.AggregateVersion, 10))
	}
	return headers
}

func Context(ctx context.Context, headers nats.Header) context.Context {
	ctx = requestid.With(ctx, headers.Get(requestid.Header))
	meta := queuecontext.Metadata{}
	if id, err := uuid.Parse(headers.Get(queuecontext.EventIDHeader)); err == nil {
		meta.EventID = id
	}
	if id, err := uuid.Parse(headers.Get("Gate-Aggregate-ID")); err == nil {
		meta.AggregateID = id
	}
	if version, err := strconv.ParseInt(headers.Get("Gate-Aggregate-Version"), 10, 64); err == nil && version > 0 {
		meta.AggregateVersion = version
	}
	if queued, err := time.Parse(time.RFC3339Nano, headers.Get(queuecontext.QueuedAtHeader)); err == nil {
		meta.QueuedAt = queued
	}
	return queuecontext.With(ctx, meta)
}
