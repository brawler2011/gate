package contests

import (
	"context"
	"encoding/json"
	"fmt"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
)

func recordTestEvent(ctx context.Context, e eventdata.Event, create func(context.Context, *eventdata.CreateOutboxEventParams) error) error {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("encode test event: %w", err)
	}
	return create(ctx, &eventdata.CreateOutboxEventParams{Id: uuid.New(), AggregateID: e.AggregateID, EventType: e.Type, Payload: data, Headers: map[string]string{requestid.Header: requestid.Get(ctx)}})
}

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }
