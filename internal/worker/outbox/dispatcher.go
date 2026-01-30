package outbox

import (
	"context"
	"fmt"

	"github.com/gate149/core/internal/domain/interfaces"
)

type EventDispatcher struct {
	handlers map[string]interfaces.EventHandler
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[string]interfaces.EventHandler),
	}
}

func (d *EventDispatcher) Register(eventType string, handler interfaces.EventHandler) {
	if _, ok := d.handlers[eventType]; ok {
		panic(fmt.Sprintf("handler already registered for event type: %s", eventType))
	}

	d.handlers[eventType] = handler
}

func (d *EventDispatcher) Dispatch(ctx context.Context, eventType string, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before dispatch: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("recovered from panic in event handler: %v\n", r)
		}
	}()

	handler, ok := d.handlers[eventType]
	if !ok {
		return fmt.Errorf("no handler registered for event type: %s", eventType)
	}

	err := handler.Handle(ctx, eventType, payload)
	if err != nil {
		return err
	}

	return nil
}
