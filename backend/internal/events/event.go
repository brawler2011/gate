// Package events signals row changes over Postgres LISTEN/NOTIFY.
package events

import (
	"context"

	"github.com/google/uuid"
)

// Notice names a changed row; receivers load its current state because NOTIFY payloads are capped at 8 KB.
type Notice struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id"`
	// Set only when the row is gone, such as a deleted announcement.
	ContestID *uuid.UUID `json:"contest_id,omitempty"`
	// The test a submission started.
	Number int32 `json:"number,omitempty"`
}

// Writer notifies after the surrounding transaction commits; delivery is best-effort.
type Writer interface {
	Notify(context.Context, Notice) error
}
