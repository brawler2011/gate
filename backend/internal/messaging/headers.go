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
