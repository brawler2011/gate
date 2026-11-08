package contests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mockContestRepoForCache struct {
	ContestsRepo
	mu        sync.Mutex
	contests  map[string]contestsmodel.Contest
	callCount int64
}

func newMockContestRepoForCache() *mockContestRepoForCache {
	return &mockContestRepoForCache{
		contests: make(map[string]contestsmodel.Contest),
	}
}

func (m *mockContestRepoForCache) putContest(c contestsmodel.Contest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := contestLookupKey(c.OrganizationLogin, c.Login)
	m.contests[key] = c
}

func (m *mockContestRepoForCache) GetContestByOrgLoginAndContestLogin(_ context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	atomic.AddInt64(&m.callCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()

	key := contestLookupKey(orgLogin, contestLogin)
	c, ok := m.contests[key]
	if !ok {
		return contestsmodel.Contest{}, pkg.ErrNotFound
	}
	return c, nil
}

func (m *mockContestRepoForCache) GetContest(_ context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.contests {
		if c.ID == id {
			return c, nil
		}
	}
	return contestsmodel.Contest{}, pkg.ErrNotFound
}

type mockTxStoreForCache struct {
	CommandTxStore
	repo *mockContestRepoForCache
}

func (s *mockTxStoreForCache) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	return s.repo.GetContest(ctx, id)
}

func (s *mockTxStoreForCache) UpdateContest(_ context.Context, input contestsmodel.ContestUpdateInput) error {
	s.repo.mu.Lock()
	defer s.repo.mu.Unlock()

	for key, c := range s.repo.contests {
		if c.ID == input.ID {
			delete(s.repo.contests, key)
			if input.Login != nil {
				c.Login = *input.Login
			}
			if input.Title != nil {
				c.Title = *input.Title
			}
			newKey := contestLookupKey(c.OrganizationLogin, c.Login)
			s.repo.contests[newKey] = c
			return nil
		}
	}
	return pkg.ErrNotFound
}

func (s *mockTxStoreForCache) DeleteContest(_ context.Context, id uuid.UUID) error {
	s.repo.mu.Lock()
	defer s.repo.mu.Unlock()

	for key, c := range s.repo.contests {
		if c.ID == id {
			delete(s.repo.contests, key)
			return nil
		}
	}
	return pkg.ErrNotFound
}

type allowAllPermissions struct{}

func (allowAllPermissions) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}

func (allowAllPermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}

func setupCacheTestFixture() (*ContestsUseCase, *mockContestRepoForCache, context.Context) {
	repo := newMockContestRepoForCache()
	txStore := &mockTxStoreForCache{repo: repo}
	tx := testTransactions[CommandTxStore]{store: txStore}
	perms := allowAllPermissions{}

	uc := NewContestsUseCase(repo, nil, tx, perms)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{
		Id:   uuid.New(),
		Role: accessmodel.UserRoleAdmin,
	})

	return uc, repo, ctx
}

func TestGetContestByOrgLoginAndContestLogin_CacheMissAndHit(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	contestID := uuid.New()
	sampleContest := contestsmodel.Contest{
		ID:                contestID,
		OrganizationLogin: "codeforces",
		Login:             "round-999",
		Title:             "Codeforces Round 999",
	}
	repo.putContest(sampleContest)

	// 1. First call: Cache miss, queries repo
	res1, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "codeforces", "round-999")
	require.NoError(t, err)
	require.Equal(t, contestID, res1.ID)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// 2. Second call: Cache hit, returned from RAM without querying repo
	res2, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "codeforces", "round-999")
	require.NoError(t, err)
	require.Equal(t, contestID, res2.ID)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount), "repo call count must remain 1 on cache hit")

	// 3. Normalized key check: Case insensitivity (casing variants hit same cache key)
	testCases := []struct {
		org   string
		login string
	}{
		{"CODEFORCES", "ROUND-999"},
		{"CodeForces", "Round-999"},
		{"codeforces", "ROUND-999"},
		{"CODEFORCES", "round-999"},
	}
	for _, tc := range testCases {
		res, err := uc.GetContestByOrgLoginAndContestLogin(ctx, tc.org, tc.login)
		require.NoError(t, err)
		require.Equal(t, contestID, res.ID)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount), "normalized key variants must hit RAM cache")
}

func TestGetContestByOrgLoginAndContestLogin_RepoErrorNotCached(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	// Cache miss on non-existent contest
	_, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "unknown-org", "unknown-contest")
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// Second call should also query repo, not cache the negative result
	_, err = uc.GetContestByOrgLoginAndContestLogin(ctx, "unknown-org", "unknown-contest")
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.callCount))
}

func TestUpdateContest_InvalidatesOldKeyWhenLoginChanges(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	contestID := uuid.New()
	contest := contestsmodel.Contest{
		ID:                contestID,
		OrganizationLogin: "acm-icpc",
		Login:             "semifinal-2026",
		Title:             "Semifinals 2026",
	}
	repo.putContest(contest)

	// Populate cache
	got, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "acm-icpc", "semifinal-2026")
	require.NoError(t, err)
	require.Equal(t, contestID, got.ID)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// Update contest login: old key "acm-icpc/semifinal-2026" -> new key "acm-icpc/finals-2026"
	newLogin := "finals-2026"
	newTitle := "Finals 2026"
	err = uc.UpdateContest(ctx, contestsmodel.ContestUpdateInput{
		ID:    contestID,
		Login: &newLogin,
		Title: &newTitle,
	})
	require.NoError(t, err)

	// Old key must be invalidated from cache: next call queries repo and gets NotFound
	_, err = uc.GetContestByOrgLoginAndContestLogin(ctx, "acm-icpc", "semifinal-2026")
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.callCount), "should have queried repo after old key invalidated")

	// New key must query repo and populate cache
	gotNew, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "acm-icpc", "finals-2026")
	require.NoError(t, err)
	require.Equal(t, contestID, gotNew.ID)
	require.Equal(t, "finals-2026", gotNew.Login)
	require.Equal(t, int64(3), atomic.LoadInt64(&repo.callCount))

	// Subsequent request to new key hits cache
	gotCached, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "acm-icpc", "finals-2026")
	require.NoError(t, err)
	require.Equal(t, contestID, gotCached.ID)
	require.Equal(t, int64(3), atomic.LoadInt64(&repo.callCount), "new key should now hit RAM cache")
}

func TestDeleteContest_InvalidatesCache(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	contestID := uuid.New()
	contest := contestsmodel.Contest{
		ID:                contestID,
		OrganizationLogin: "yandex",
		Login:             "cup-2026",
		Title:             "Yandex Cup 2026",
	}
	repo.putContest(contest)

	// Populate cache
	got, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "yandex", "cup-2026")
	require.NoError(t, err)
	require.Equal(t, contestID, got.ID)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// Delete contest
	err = uc.DeleteContest(ctx, contestID)
	require.NoError(t, err)

	// Cache must be invalidated: next call queries repo again and gets NotFound
	_, err = uc.GetContestByOrgLoginAndContestLogin(ctx, "yandex", "cup-2026")
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.callCount), "should have queried repo after delete invalidation")
}

func TestContestCache_TTLExpiration(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	now := time.Now()
	uc.cache.now = func() time.Time { return now }
	uc.cache.ttl = 2 * time.Minute

	contestID := uuid.New()
	repo.putContest(contestsmodel.Contest{
		ID:                contestID,
		OrganizationLogin: "tinkoff",
		Login:             "generation",
		Title:             "Tinkoff Generation",
	})

	// 1. Initial lookup -> miss, caches entry
	_, err := uc.GetContestByOrgLoginAndContestLogin(ctx, "tinkoff", "generation")
	require.NoError(t, err)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// 2. Advance time within TTL (1 minute) -> hit
	now = now.Add(1 * time.Minute)
	_, err = uc.GetContestByOrgLoginAndContestLogin(ctx, "tinkoff", "generation")
	require.NoError(t, err)
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.callCount))

	// 3. Advance time past TTL (3 minutes total) -> expired, queries repo
	now = now.Add(2 * time.Minute)
	_, err = uc.GetContestByOrgLoginAndContestLogin(ctx, "tinkoff", "generation")
	require.NoError(t, err)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.callCount), "expired entry must trigger repo query")
}

func TestContestCache_ConcurrencyAndThreadSafety(t *testing.T) {
	uc, repo, ctx := setupCacheTestFixture()

	// Seed multiple contests
	const numContests = 10
	var contestIDs []uuid.UUID
	for i := 0; i < numContests; i++ {
		id := uuid.New()
		contestIDs = append(contestIDs, id)
		repo.putContest(contestsmodel.Contest{
			ID:                id,
			OrganizationLogin: "org",
			Login:             fmt.Sprintf("contest-%d", i),
			Title:             fmt.Sprintf("Contest %d", i),
		})
	}

	const numGoroutines = 50
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		goroutineID := g
		go func() {
			defer wg.Done()
			for it := 0; it < iterations; it++ {
				idx := (goroutineID + it) % numContests
				contestLogin := fmt.Sprintf("contest-%d", idx)

				switch it % 5 {
				case 0, 1, 2:
					// Read operations (with varied casing)
					org := "org"
					if it%2 == 0 {
						org = "ORG"
					}
					_, _ = uc.GetContestByOrgLoginAndContestLogin(ctx, org, strings.ToUpper(contestLogin))
				case 3:
					// Update operation
					newTitle := fmt.Sprintf("Updated %d by %d", idx, goroutineID)
					_ = uc.UpdateContest(ctx, contestsmodel.ContestUpdateInput{
						ID:    contestIDs[idx],
						Title: &newTitle,
					})
				case 4:
					// Direct cache get/delete operations
					_, _ = uc.cache.Get("org", contestLogin)
				}
			}
		}()
	}

	wg.Wait()
}
