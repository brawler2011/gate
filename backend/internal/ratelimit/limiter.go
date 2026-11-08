// Package ratelimit implements shared, atomic Redis quotas. Redis failures are fail-open.
package ratelimit

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Quota struct {
	Limit  int
	Window time.Duration
}

var (
	EmailQuotas      = []Quota{{5, 5 * time.Minute}, {20, time.Hour}, {50, 24 * time.Hour}}
	SubmissionQuotas = []Quota{{60, 5 * time.Minute}, {300, time.Hour}}
)

// Each key is a sliding log. All windows are checked before adding an event,
// so rejected requests neither consume quota nor extend the wait.
var quotaScript = redis.NewScript(`
local now = redis.call('TIME')
local ms = now[1]*1000 + math.floor(now[2]/1000)
local retention = tonumber(ARGV[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ms-retention)
local retry = 0
for i=3,#ARGV,2 do
 local window = tonumber(ARGV[i])
 local limit = tonumber(ARGV[i+1])
 local count = redis.call('ZCOUNT', KEYS[1], '('..(ms-window), '+inf')
 if count >= limit then
  local oldest = redis.call('ZRANGEBYSCORE', KEYS[1], '('..(ms-window), '+inf', 'WITHSCORES', 'LIMIT', count-limit, 1)
  retry = math.max(retry, tonumber(oldest[2])+window-ms)
 end
end
if retry > 0 then return retry end
redis.call('ZADD', KEYS[1], ms, ARGV[2])
redis.call('PEXPIRE', KEYS[1], retention)
return 0
`)

// A cooldown marker replaces the failure log after the sixteenth failure.
// Rejected attempts do not refresh its TTL; a successful login deletes the key.
var failureScript = redis.NewScript(`
if redis.call('ZSCORE', KEYS[1], 'cooldown') then
 return redis.call('PTTL', KEYS[1])
end
local now = redis.call('TIME')
local ms = now[1]*1000 + math.floor(now[2]/1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ms-300000)
if redis.call('ZCARD', KEYS[1]) >= 15 then
 redis.call('DEL', KEYS[1])
 redis.call('ZADD', KEYS[1], 0, 'cooldown')
 redis.call('PEXPIRE', KEYS[1], 120000)
 return 120000
end
redis.call('ZADD', KEYS[1], ms, ARGV[1])
redis.call('PEXPIRE', KEYS[1], 300000)
return 0
`)

type Limiter struct{ client redis.Scripter }

func New(client redis.Scripter) *Limiter { return &Limiter{client: client} }

func (l *Limiter) Allow(ctx context.Context, key string, quotas ...Quota) error {
	if l == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	var retention time.Duration
	args := []any{int64(0), uuid.NewString()}
	for _, q := range quotas {
		if q.Window > retention {
			retention = q.Window
		}
		args = append(args, q.Window.Milliseconds(), q.Limit)
	}
	args[0] = retention.Milliseconds()
	ms, err := quotaScript.Run(ctx, l.client, []string{key}, args...).Int64()
	return result(ms, err)
}

func (l *Limiter) Email(ctx context.Context, email string) error {
	return l.Allow(ctx, "rl:email:"+strings.ToLower(email), EmailQuotas...)
}

func (l *Limiter) LoginFailure(ctx context.Context, identifier string) error {
	if l == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	ms, err := failureScript.Run(ctx, l.client, []string{"rl:fail:login:" + identifier}, uuid.NewString()).Int64()
	return result(ms, err)
}

func (l *Limiter) LoginSuccess(ctx context.Context, identifier string) {
	if l == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	// Scripter deliberately keeps the dependency small; deleting through Lua is atomic.
	_, err := l.client.Eval(ctx, `return redis.call('DEL', KEYS[1])`, []string{"rl:fail:login:" + identifier}).Result()
	if err != nil {
		slog.Warn("rate limiter unavailable; allowing request", "error", err)
	}
}

func result(ms int64, err error) error {
	if err != nil {
		slog.Warn("rate limiter unavailable; allowing request", "error", err)
		return nil
	}
	if ms > 0 {
		return &pkg.RateLimitError{RetryAfter: time.Duration(ms) * time.Millisecond}
	}
	return nil
}
