package accessmodel

import (
	"context"
	"time"

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
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	if !user.IsAdmin() {
		return pkg.NoPermission
	}
	return nil
}

func RequireSelfOrAdmin(ctx context.Context, id uuid.UUID) error {
	user := UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	if !user.IsAdmin() && user.Id != id {
		return pkg.NoPermission
	}
	return nil
}

func RequireCaller(ctx context.Context, id uuid.UUID) error {
	user := UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	if user.Id != id {
		return pkg.NoPermission
	}
	return nil
}

type readTimeKey struct{}

// WithReadTime fixes the authorization clock for the whole request, including
// SQL scopes and in-memory decisions. Callers must use a server-supplied time.
func WithReadTime(ctx context.Context, now time.Time) context.Context {
	return context.WithValue(ctx, readTimeKey{}, now.UTC())
}
func ReadTime(ctx context.Context) time.Time {
	if now, ok := ctx.Value(readTimeKey{}).(time.Time); ok {
		return now
	}
	return time.Now().UTC()
}
