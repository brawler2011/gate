package pubsub

import (
	"context"
)

type Dispatcher interface {
	Dispatch(ctx context.Context, eventType string, payload []byte) error
}

type EventHandler interface {
	Handle(context.Context, string, []byte) error
}
