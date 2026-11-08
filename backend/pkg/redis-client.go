package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a new Redis client with connection pooling configuration
func NewRedisClient(dsn string) (*redis.Client, error) {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	// Configure connection pool
	opts.ContextTimeoutEnabled = true
	opts.PoolSize = 10
	opts.MinIdleConns = 5
	opts.ConnMaxLifetime = 30 * time.Minute
	opts.ConnMaxIdleTime = 5 * time.Minute

	// Configure timeouts
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("redis unavailable at startup; rate limits will fail open", "error", err)
	}

	return client, nil
}
