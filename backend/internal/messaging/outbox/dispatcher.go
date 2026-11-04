package outbox

import (
	"context"
	"fmt"
	"log/slog"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
)

type EventDispatcher struct {
	handlers map[string]EventHandler
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[string]EventHandler),
	}
}

func (d *EventDispatcher) Register(eventType string, handler EventHandler) {
	if _, ok := d.handlers[eventType]; ok {
		panic(fmt.Sprintf("handler already registered for event type: %s", eventType))
	}

	d.handlers[eventType] = handler
}

func (d *EventDispatcher) Dispatch(ctx context.Context, eventType string, payload []byte) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before dispatch: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "recovered from panic in event handler", "panic", r)
			err = fmt.Errorf("event handler panic: %v", r)
		}
	}()

	handler, ok := d.handlers[eventType]
	if !ok {
		return &eventdata.PermanentError{Cause: fmt.Errorf("no handler registered for event type: %s", eventType)}
	}

	err = handler.Handle(ctx, eventType, payload)
	if err != nil {
		return err
	}

	return nil
}
