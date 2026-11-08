package accessmodel

import (
	"context"
	"time"
)

type commandDecisionKey struct{}

// CommandDecision binds nonlocking prechecks to an optimistic write guard.
// Revision is captured before reading facts; any intervening security change
// invalidates it. ValidUntil is exclusive, including the contest's end boundary.
type CommandDecision struct {
	Revision   int64
	ValidUntil *time.Time
}

func WithCommandDecision(ctx context.Context, d CommandDecision) context.Context {
	return context.WithValue(ctx, commandDecisionKey{}, d)
}
func DecisionFromContext(ctx context.Context) (CommandDecision, bool) {
	d, ok := ctx.Value(commandDecisionKey{}).(CommandDecision)
	return d, ok
}
func WithDecisionDeadline(ctx context.Context, deadline *time.Time) context.Context {
	d, ok := DecisionFromContext(ctx)
	if !ok || deadline == nil {
		return ctx
	}
	if d.ValidUntil == nil || deadline.Before(*d.ValidUntil) {
		d.ValidUntil = deadline
	}
	return WithCommandDecision(ctx, d)
}
