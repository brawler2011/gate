package postgres

import (
	"context"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RateLimits struct{ pool *pgxpool.Pool }

func NewRateLimits(pool *pgxpool.Pool) *RateLimits { return &RateLimits{pool: pool} }

func (r *RateLimits) Update(ctx context.Context, key string, now time.Time, fn func(*ratelimit.Entry) time.Time) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		row, err := q.LockRateLimit(ctx, key)
		if err != nil {
			return err
		}
		entry := ratelimit.Entry{Hits: row.Hits}
		if row.BlockedUntil != nil {
			entry.BlockedUntil = *row.BlockedUntil
		}
		expires := fn(&entry)
		if !expires.After(now) {
			return q.DeleteRateLimit(ctx, key)
		}
		if entry.Hits == nil {
			entry.Hits = []time.Time{}
		}
		var blocked *time.Time
		if !entry.BlockedUntil.IsZero() {
			blocked = &entry.BlockedUntil
		}
		return q.SaveRateLimit(ctx, sqlc.SaveRateLimitParams{Key: key, Hits: entry.Hits, BlockedUntil: blocked, ExpiresAt: expires})
	})
	if err != nil {
		return dbError("update rate limit", err)
	}
	return dbError("delete expired rate limits", sqlc.New(r.pool).DeleteExpiredRateLimits(ctx, now))
}
