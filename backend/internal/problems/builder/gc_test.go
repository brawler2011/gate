package builder

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Slow lookups let young objects cross retention during a sweep.
type sweepRepo struct {
	lookupDelay time.Duration
	looked      [][]string
}

func (r *sweepRepo) RecoverBuilds(context.Context) error                { return nil }
func (r *sweepRepo) PrunePackages(context.Context, time.Duration) error { return nil }
func (r *sweepRepo) PruneBuilds(context.Context, time.Duration) error   { return nil }
func (r *sweepRepo) DeleteUnreferencedObjects(context.Context) ([]string, error) { return nil, nil }
func (r *sweepRepo) ActiveBuildAttempt(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (r *sweepRepo) ExistingObjectKeys(_ context.Context, keys []string) (map[string]bool, error) {
	r.looked = append(r.looked, keys)
	time.Sleep(r.lookupDelay)
	return map[string]bool{}, nil
}

type sweepStore struct {
	objects []storage.ObjectInfo
	deleted []string
}

func (s *sweepStore) ListObjects(context.Context, string, string) ([]storage.ObjectInfo, error) {
	return s.objects, nil
}
func (s *sweepStore) DeleteFile(_ context.Context, _, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func TestSweepJudgesAgeBeforeLookingUpRows(t *testing.T) {
	const retention = time.Hour
	now := time.Now()
	store := &sweepStore{objects: []storage.ObjectInfo{
		{Key: "objects/old", LastModified: now.Add(-2 * retention)},
		// Objects young at the cutoff must survive even if lookups cross their retention deadline.
		{Key: "objects/young", LastModified: now.Add(-retention + 50*time.Millisecond)},
	}}
	repo := &sweepRepo{lookupDelay: 100 * time.Millisecond}
	gc := &GC{Repo: repo, Store: store, Bucket: "b", TempDir: t.TempDir(), ObjectRetention: retention}

	require.NoError(t, gc.Collect(context.Background()))

	require.Equal(t, []string{"objects/old"}, store.deleted)
	require.Equal(t, [][]string{{"objects/old"}}, repo.looked)
}
