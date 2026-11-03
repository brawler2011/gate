package accessmodel

import (
	"context"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type callerKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, callerKey{}, user)
}

func UserFromContext(ctx context.Context) User {
	if user, ok := ctx.Value(callerKey{}).(User); ok {
		return user
	}
	return Guest
}

func RequireAdmin(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if !user.IsAdmin() {
		return pkg.NoPermission
	}
	return nil
}

func RequireSelfOrAdmin(ctx context.Context, id uuid.UUID) error {
	user := UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if !user.IsAdmin() && user.Id != id {
		return pkg.NoPermission
	}
	return nil
}
