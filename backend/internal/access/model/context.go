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
		return pkg.ErrForbidden
	}
	return nil
}

func RequireSelfOrAdmin(ctx context.Context, id uuid.UUID) error {
	user := UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	if !user.IsAdmin() && user.Id != id {
		return pkg.ErrForbidden
	}
	return nil
}

func RequireUser(ctx context.Context) (User, error) {
	user := UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return User{}, pkg.ErrUnauthenticated
	}
	return user, nil
}

type readTimeKey struct{}

// Use a server-supplied clock consistently across SQL scopes and in-memory authorization.
func WithReadTime(ctx context.Context, now time.Time) context.Context {
	return context.WithValue(ctx, readTimeKey{}, now.UTC())
}
func ReadTime(ctx context.Context) time.Time {
	if now, ok := ctx.Value(readTimeKey{}).(time.Time); ok {
		return now
	}
	return time.Now().UTC()
}

type scoreboardFreezeKey struct{}

func WithScoreboardFreeze(ctx context.Context, boundary *time.Time) context.Context {
	return context.WithValue(ctx, scoreboardFreezeKey{}, boundary)
}
func ScoreboardFreeze(ctx context.Context) *time.Time {
	t, _ := ctx.Value(scoreboardFreezeKey{}).(*time.Time)
	return t
}
