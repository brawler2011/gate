package contests

import (
	"strings"
	"sync"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

type contestCacheEntry struct {
	contest  contestsmodel.Contest
	cachedAt time.Time
}

type contestCache struct {
	mu      sync.RWMutex
	entries map[string]*contestCacheEntry
	byID    map[uuid.UUID]string
	ttl     time.Duration
	now     func() time.Time
}

func newContestCache(ttl time.Duration) *contestCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &contestCache{
		entries: make(map[string]*contestCacheEntry),
		byID:    make(map[uuid.UUID]string),
		ttl:     ttl,
		now:     time.Now,
	}
}

func contestLookupKey(orgLogin, contestLogin string) string {
	return strings.ToLower(orgLogin) + "/" + strings.ToLower(contestLogin)
}

func (c *contestCache) Get(orgLogin, contestLogin string) (contestsmodel.Contest, bool) {
	if c == nil {
		return contestsmodel.Contest{}, false
	}
	key := contestLookupKey(orgLogin, contestLogin)

	c.mu.RLock()
	entry, exists := c.entries[key]
	if !exists {
		c.mu.RUnlock()
		return contestsmodel.Contest{}, false
	}

	expired := c.now().Sub(entry.cachedAt) > c.ttl
	contest := entry.contest
	c.mu.RUnlock()

	if expired {
		c.mu.Lock()
		if e, ok := c.entries[key]; ok && c.now().Sub(e.cachedAt) > c.ttl {
			delete(c.entries, key)
			if e.contest.ID != uuid.Nil {
				delete(c.byID, e.contest.ID)
			}
		}
		c.mu.Unlock()
		return contestsmodel.Contest{}, false
	}

	return contest, true
}

func (c *contestCache) Set(orgLogin, contestLogin string, contest contestsmodel.Contest) {
	if c == nil {
		return
	}
	effectiveOrg := orgLogin
	if effectiveOrg == "" {
		effectiveOrg = contest.OrganizationLogin
	}
	effectiveLogin := contestLogin
	if effectiveLogin == "" {
		effectiveLogin = contest.Login
	}
	key := contestLookupKey(effectiveOrg, effectiveLogin)

	c.mu.Lock()
	defer c.mu.Unlock()

	if contest.ID != uuid.Nil {
		if oldKey, ok := c.byID[contest.ID]; ok && oldKey != key {
			delete(c.entries, oldKey)
		}
		c.byID[contest.ID] = key
	}

	c.entries[key] = &contestCacheEntry{
		contest:  contest,
		cachedAt: c.now(),
	}
}

func (c *contestCache) Delete(orgLogin, contestLogin string) {
	if c == nil {
		return
	}
	key := contestLookupKey(orgLogin, contestLogin)
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		if entry.contest.ID != uuid.Nil {
			delete(c.byID, entry.contest.ID)
		}
		delete(c.entries, key)
	}
}

func (c *contestCache) DeleteByID(id uuid.UUID) {
	if c == nil || id == uuid.Nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if key, ok := c.byID[id]; ok {
		delete(c.entries, key)
		delete(c.byID, id)
	}
}

func (c *contestCache) GetByID(id uuid.UUID) (contestsmodel.Contest, bool) {
	if c == nil || id == uuid.Nil {
		return contestsmodel.Contest{}, false
	}
	c.mu.RLock()
	key, ok := c.byID[id]
	if !ok {
		c.mu.RUnlock()
		return contestsmodel.Contest{}, false
	}
	entry, exists := c.entries[key]
	if !exists {
		c.mu.RUnlock()
		return contestsmodel.Contest{}, false
	}
	expired := c.now().Sub(entry.cachedAt) > c.ttl
	contest := entry.contest
	c.mu.RUnlock()

	if expired {
		return contestsmodel.Contest{}, false
	}
	return contest, true
}

func (c *contestCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
