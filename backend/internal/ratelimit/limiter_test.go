package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

type clock struct{ at time.Time }

func (c *clock) set(at time.Time)        { c.at = at }
func (c *clock) advance(d time.Duration) { c.at = c.at.Add(d) }
func testLimiter() (*Limiter, *clock) {
	c := &clock{at: time.Now()}
	l := New(NewMemory())
	l.Clock = func() time.Time { return c.at }
	return l, c
}

func requireLimited(t *testing.T, err error, retry time.Duration) {
	t.Helper()
	var limited *pkg.RateLimitError
	require.ErrorAs(t, err, &limited)
	require.Equal(t, retry, limited.RetryAfter)
}

func TestSlidingQuotas(t *testing.T) {
	for _, tc := range []struct {
		name   string
		quotas []Quota
	}{
		{"email", append([]Quota(nil), EmailQuotas...)}, {"submissions", append([]Quota(nil), SubmissionQuotas...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			for _, q := range tc.quotas {
				l, _ := testLimiter()
				// Relax shorter windows to test each quota tier independently.
				for i := range tc.quotas {
					if tc.quotas[i].Window < q.Window {
						tc.quotas[i].Limit = q.Limit + 1
					}
				}
				for range q.Limit {
					require.NoError(t, l.Allow(ctx, "quota", tc.quotas...))
				}
				requireLimited(t, l.Allow(ctx, "quota", tc.quotas...), q.Window)
			}
		})
	}
}

func TestSlidingWindowExpiryAndIsolation(t *testing.T) {
	l, c := testLimiter()
	ctx := context.Background()
	q := Quota{2, time.Minute}
	require.NoError(t, l.Allow(ctx, "a", q))
	c.advance(30 * time.Second)
	require.NoError(t, l.Allow(ctx, "a", q))
	requireLimited(t, l.Allow(ctx, "a", q), 30*time.Second)
	require.NoError(t, l.Allow(ctx, "b", q))
	c.advance(30 * time.Second)
	require.NoError(t, l.Allow(ctx, "a", q))
	requireLimited(t, l.Allow(ctx, "a", q), 30*time.Second)
}

func TestConcurrentQuota(t *testing.T) {
	l := New(nil)
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
	l, c := testLimiter()
	ctx := context.Background()
	for range 15 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	requireLimited(t, l.LoginFailure(ctx, "user"), 2*time.Minute)
	c.advance(time.Minute)
	requireLimited(t, l.LoginFailure(ctx, "user"), time.Minute)
	c.advance(time.Minute)
	require.NoError(t, l.LoginFailure(ctx, "user"))
	l.LoginSuccess(ctx, "user")
	for range 15 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	c.advance(5 * time.Minute)
	require.NoError(t, l.LoginFailure(ctx, "user"))
}

func TestLoginFailureSlidingWindow(t *testing.T) {
	l, c := testLimiter()
	ctx := context.Background()
	now := c.at
	for range 10 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	c.set(now.Add(4 * time.Minute))
	for range 5 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	c.set(now.Add(5 * time.Minute))
	// Expired failures release quota while unexpired failures still count.
	for range 10 {
		require.NoError(t, l.LoginFailure(ctx, "user"))
	}
	requireLimited(t, l.LoginFailure(ctx, "user"), 2*time.Minute)
}

func TestLoginIdentifierCaseInsensitive(t *testing.T) {
	l, _ := testLimiter()
	ctx := context.Background()
	for attempt := range 15 {
		name := "test_user"
		if attempt%2 == 0 {
			name = "TEST_USER"
		}
		require.NoError(t, l.LoginFailure(ctx, name))
	}
	requireLimited(t, l.LoginFailure(ctx, "Test_User"), 2*time.Minute)
	l.LoginSuccess(ctx, "tEST_uSER")
	require.NoError(t, l.LoginFailure(ctx, "TEST_USER"))
}
