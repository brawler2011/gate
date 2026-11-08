package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func testLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), server
}

func TestSlidingQuotas(t *testing.T) {
	for _, tc := range []struct {
		name   string
		quotas []Quota
	}{
		{"email", append([]Quota(nil), EmailQuotas...)}, {"submissions", append([]Quota(nil), SubmissionQuotas...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, server := testLimiter(t)
			ctx := context.Background()
			// Advance Redis's own clock as well as TTLs; Lua uses TIME.
			now := time.Now()
			server.SetTime(now)
			for _, q := range tc.quotas {
				for i := range tc.quotas {
					if tc.quotas[i].Window < q.Window {
						tc.quotas[i].Limit = q.Limit + 1
					}
				}
				for range q.Limit {
					require.NoError(t, l.Allow(ctx, "quota", tc.quotas...))
				}
				var limited *pkg.RateLimitError
				require.ErrorAs(t, l.Allow(ctx, "quota", tc.quotas...), &limited)
				require.Positive(t, limited.RetryAfter)
				require.LessOrEqual(t, limited.RetryAfter, q.Window)
				// Start a fresh log for the next window, isolate each tier by using
				// a relaxed shorter-window limit.
				server.FlushAll()
			}
		})
	}
}

func TestSlidingWindowExpiryAndIsolation(t *testing.T) {
	l, server := testLimiter(t)
	ctx := context.Background()
	now := time.Now()
	server.SetTime(now)
	q := Quota{2, time.Minute}
	require.NoError(t, l.Allow(ctx, "a", q))
	server.SetTime(now.Add(30 * time.Second))
	require.NoError(t, l.Allow(ctx, "a", q))
	var limited *pkg.RateLimitError
	require.ErrorAs(t, l.Allow(ctx, "a", q), &limited)
	require.Equal(t, 30*time.Second, limited.RetryAfter)
	require.NoError(t, l.Allow(ctx, "b", q))
	server.SetTime(now.Add(time.Minute))
	require.NoError(t, l.Allow(ctx, "a", q))
	require.ErrorAs(t, l.Allow(ctx, "a", q), &limited)
	require.Equal(t, 30*time.Second, limited.RetryAfter)
	require.Equal(t, time.Minute, server.TTL("a"))
}

func TestConcurrentQuota(t *testing.T) {
	l, _ := testLimiter(t)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if l.Allow(context.Background(), "shared", Quota{10, time.Minute}) == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 10, accepted.Load())
}

func TestLoginFailuresAndReset(t *testing.T) {
	l, server := testLimiter(t)
	ctx := context.Background()
	for range 15 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	var limited *pkg.RateLimitError
	require.ErrorAs(t, l.LoginFailure(ctx, "user"), &limited)
	require.Equal(t, 2*time.Minute, limited.RetryAfter)
	server.FastForward(time.Minute)
	require.ErrorAs(t, l.LoginFailure(ctx, "user"), &limited)
	require.Equal(t, time.Minute, limited.RetryAfter)
	server.FastForward(time.Minute)
	require.NoError(t, l.LoginFailure(ctx, "user"))
	l.LoginSuccess(ctx, "user")
	require.False(t, server.Exists("rl:fail:login:user"))
	for range 15 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	server.FastForward(5 * time.Minute)
	require.NoError(t, l.LoginFailure(ctx, "user"))
}

func TestRedisFailureAllowsRequests(t *testing.T) {
	l, server := testLimiter(t)
	server.Close()
	ctx := context.Background()
	require.NoError(t, l.Allow(ctx, "key", Quota{1, time.Minute}))
	require.NoError(t, l.LoginFailure(ctx, "user"))
	l.LoginSuccess(ctx, "user")
}

func TestLoginFailureSlidingWindow(t *testing.T) {
	l, server := testLimiter(t)
	ctx := context.Background()
	now := time.Now()
	server.SetTime(now)
	for range 10 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	server.SetTime(now.Add(4 * time.Minute))
	for range 5 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	server.SetTime(now.Add(5 * time.Minute))
	// Only the first ten failures have expired, leaving room for ten more.
	for range 10 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	var limited *pkg.RateLimitError
	require.ErrorAs(t, l.LoginFailure(ctx, "user"), &limited)
	require.Equal(t, 2*time.Minute, limited.RetryAfter)
}
