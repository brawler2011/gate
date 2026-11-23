// Package ratelimit implements sliding-window quotas.
package ratelimit

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
)

type Quota struct {
	Limit  int
	Window time.Duration
}

var (
	EmailQuotas      = []Quota{{5, 5 * time.Minute}, {20, time.Hour}, {50, 24 * time.Hour}}
	SubmissionQuotas = []Quota{{60, 5 * time.Minute}, {300, time.Hour}}
)

const (
	loginFailures = 15
	loginWindow   = 5 * time.Minute
	loginCooldown = 2 * time.Minute
)

// Entry is the state of one key; Hits are sorted oldest first.
type Entry struct {
	Hits         []time.Time
	BlockedUntil time.Time
}

// Store runs fn atomically for key and keeps the entry until the time fn returns.
type Store interface {
	Update(ctx context.Context, key string, now time.Time, fn func(*Entry) time.Time) error
}

// Limiter keeps request quotas in process memory and auth quotas in a store shared by all pods.
type Limiter struct {
	local  Store
	shared Store
	Clock  func() time.Time
}

func New(shared Store) *Limiter {
	return &Limiter{local: NewMemory(), shared: shared, Clock: time.Now}
}

func (l *Limiter) now() time.Time { return l.Clock().UTC().Truncate(time.Microsecond) }

func (l *Limiter) Allow(ctx context.Context, key string, quotas ...Quota) error {
	if l == nil {
		return nil
	}
	return l.take(ctx, l.local, key, quotas)
}

func (l *Limiter) Email(ctx context.Context, email string) error {
	if l == nil {
		return nil
	}
	return l.take(ctx, l.shared, "email:"+strings.ToLower(email), EmailQuotas)
}

// Rejected attempts do not extend the cooldown; a successful login clears it.
func (l *Limiter) LoginFailure(ctx context.Context, identifier string) error {
	if l == nil {
		return nil
	}
	now := l.now()
	var retry time.Duration
	err := l.shared.Update(ctx, loginKey(identifier), now, func(e *Entry) time.Time {
		if now.Before(e.BlockedUntil) {
			retry = e.BlockedUntil.Sub(now)
			return e.BlockedUntil
		}
		e.Hits = since(e.Hits, now.Add(-loginWindow))
		if len(e.Hits) >= loginFailures {
			e.Hits, e.BlockedUntil, retry = nil, now.Add(loginCooldown), loginCooldown
			return e.BlockedUntil
		}
		e.Hits = record(e.Hits, now)
		return now.Add(loginWindow)
	})
	return limited(retry, err)
}

func (l *Limiter) LoginSuccess(ctx context.Context, identifier string) {
	if l == nil {
		return
	}
	err := l.shared.Update(ctx, loginKey(identifier), l.now(), func(e *Entry) time.Time {
		*e = Entry{}
		return time.Time{}
	})
	if err != nil {
		slog.WarnContext(ctx, "clear login failures", "error", err)
	}
}

func loginKey(identifier string) string { return "login:" + strings.ToLower(identifier) }

// Every window is checked before recording, so rejected requests cannot extend the wait.
func (l *Limiter) take(ctx context.Context, store Store, key string, quotas []Quota) error {
	now := l.now()
	var retention, retry time.Duration
	for _, q := range quotas {
		retention = max(retention, q.Window)
	}
	err := store.Update(ctx, key, now, func(e *Entry) time.Time {
		e.Hits = since(e.Hits, now.Add(-retention))
		for _, q := range quotas {
			if recent := since(e.Hits, now.Add(-q.Window)); len(recent) >= q.Limit {
				retry = max(retry, recent[len(recent)-q.Limit].Add(q.Window).Sub(now))
			}
		}
		if retry == 0 {
			e.Hits = record(e.Hits, now)
		}
		if len(e.Hits) == 0 {
			return time.Time{}
		}
		return e.Hits[len(e.Hits)-1].Add(retention)
	})
	return limited(retry, err)
}

// since returns the hits after cutoff.
func since(hits []time.Time, cutoff time.Time) []time.Time {
	return hits[after(hits, cutoff):]
}

// A request that waited for the lock may be older than hits recorded meanwhile.
func record(hits []time.Time, now time.Time) []time.Time {
	return slices.Insert(hits, after(hits, now), now)
}

func after(hits []time.Time, t time.Time) int {
	i, _ := slices.BinarySearchFunc(hits, t, func(hit, t time.Time) int {
		if hit.After(t) {
			return 1
		}
		return -1
	})
	return i
}

func limited(retry time.Duration, err error) error {
	if err != nil {
		return err
	}
	if retry > 0 {
		return &pkg.RateLimitError{RetryAfter: retry}
	}
	return nil
}

// Memory is a Store for one process.
type Memory struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	swept   time.Time
}

type memoryEntry struct {
	Entry
	expires time.Time
}

func NewMemory() *Memory { return &Memory{entries: map[string]memoryEntry{}} }

func (m *Memory) Update(_ context.Context, key string, now time.Time, fn func(*Entry) time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.Sub(m.swept) >= time.Minute {
		for k, e := range m.entries {
			if !e.expires.After(now) {
				delete(m.entries, k)
			}
		}
		m.swept = now
	}
	e := m.entries[key]
	if e.expires = fn(&e.Entry); e.expires.After(now) {
		m.entries[key] = e
	} else {
		delete(m.entries, key)
	}
	return nil
}
