package interfaces

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// WorkspaceLocker serializes a complete workspace operation across its readers and writers.
type WorkspaceLocker interface {
	WithWorkspaceLock(context.Context, uuid.UUID, func(context.Context) error) error
}

type WorkspaceOwnership interface {
	Check(context.Context) error
}

type workspaceOwnershipKey struct{}

func WithWorkspaceOwnership(ctx context.Context, ownership WorkspaceOwnership) context.Context {
	return context.WithValue(ctx, workspaceOwnershipKey{}, ownership)
}

// CheckWorkspaceOwnership also rejects canceled standalone operations.
func CheckWorkspaceOwnership(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("workspace operation canceled: %w", err)
	}
	if ownership, ok := ctx.Value(workspaceOwnershipKey{}).(WorkspaceOwnership); ok {
		return ownership.Check(ctx)
	}
	return nil
}
