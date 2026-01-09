package interfaces

import (
	"context"
)

type EventHandler interface {
	Handle(ctx context.Context, eventType string, payload []byte) error
}

type EventDispatcher interface {
	Dispatch(ctx context.Context, eventType string, payload []byte) error
}
