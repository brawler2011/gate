package builder

import (
	"context"
	"testing"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// sweepRepo has no rows; looking keys up takes long enough for a young key to
// cross the retention, as a slow or suspended collector would.
type sweepRepo struct {
	lookupDelay time.Duration
	looked      [][]string
}

func (r *sweepRepo) RecoverBuilds(context.Context) error                { return nil }
func (r *sweepRepo) PrunePackages(context.Context, time.Duration) error { return nil }
func (r *sweepRepo) PruneBuilds(context.Context, time.Duration) error   { return nil }
func (r *sweepRepo) ListUnreferencedObjects(context.Context) ([]model.StoredObject, error) {
	return nil, nil
}
func (r *sweepRepo) DeleteObject(context.Context, uuid.UUID) (bool, error) { return false, nil }
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
		// Young when listed, but past the retention once the lookup returns: its
		// writer may commit a row after the lookup, so it must survive this sweep.
		{Key: "objects/young", LastModified: now.Add(-retention + 50*time.Millisecond)},
	}}
	repo := &sweepRepo{lookupDelay: 100 * time.Millisecond}
	gc := &GC{Repo: repo, Store: store, Bucket: "b", TempDir: t.TempDir(), ObjectRetention: retention}

	require.NoError(t, gc.Collect(context.Background()))

	require.Equal(t, []string{"objects/old"}, store.deleted)
	require.Equal(t, [][]string{{"objects/old"}}, repo.looked)
}
