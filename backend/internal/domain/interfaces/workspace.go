package interfaces

import (
	"context"

	"github.com/google/uuid"
)

// WorkspaceLocker serializes a complete workspace operation across its readers and writers.
type WorkspaceLocker interface {
	WithWorkspaceLock(context.Context, uuid.UUID, func(context.Context) error) error
}
