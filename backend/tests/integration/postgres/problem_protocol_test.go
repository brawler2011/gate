//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testBuildOptions() postgres.ProblemBuildOptions {
	return postgres.ProblemBuildOptions{
		Languages:   map[string]sandbox.Language{"cpp17": {Type: "compiler"}},
		MaxAttempts: 3, Duration: time.Minute, Lease: time.Minute,
	}
}

// buildableFixture is a saved, buildable draft that references one available object.
func buildableFixture(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft, uuid.UUID) {
	t.Helper()
	ctx, pool, repo, org, d := draftFixture(t)
	oid := insertAvailableObject(t, ctx, pool, org)
	d = makeBuildableDraft(d, oid)
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	return ctx, pool, repo, org, d, oid
}

func enqueue(t *testing.T, ctx context.Context, repo *postgres.ProblemDraftsRepo, org uuid.UUID, d model.Draft) {
	t.Helper()
	_, err := repo.EnqueueBuild(ctx, mutationKey(org, "build", d.Revision), d.ProblemID, d.Revision, testBuildOptions())
	require.NoError(t, err)
}

func expireLease(t *testing.T, ctx context.Context, pool *pgxpool.Pool, build uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE problem_builds SET lease_deadline=clock_timestamp()-interval '1 second' WHERE id=$1`, build)
	require.NoError(t, err)
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&n))
	return n
}

// An editor and a build request race for the same draft: the build either conflicts (the edit
// won) or snapshots exactly the revision it was requested for, never a mixture with the edit.
func TestBuildRequestSnapshotsOneRevisionWhileEditing(t *testing.T) {
	t.Parallel()
	ctx, _, repo, org, d, _ := buildableFixture(t)
	built, conflicted := 0, 0
	for attempt := 0; attempt < 8; attempt++ {
		current, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		before := current
		before.Title = "before " + uuid.NewString()
		_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", before), before)
		require.NoError(t, err)
		requested, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)

		edited := requested
		edited.Title = "after " + uuid.NewString()
		var buildErr, editErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, buildErr = repo.EnqueueBuild(ctx, mutationKey(org, "build", requested.Revision), requested.ProblemID, requested.Revision, testBuildOptions())
		}()
		go func() {
			defer wg.Done()
			<-start
			_, editErr = repo.SaveDraft(ctx, mutationKey(org, "edit", edited), edited)
		}()
		close(start)
		wg.Wait()
		require.NoError(t, editErr, "the edit starts from the current revision and always succeeds")
		if buildErr != nil {
			require.ErrorIs(t, buildErr, pkg.ErrConflict)
			conflicted++
			continue
		}
		built++
		build, err := repo.ClaimBuild(ctx, time.Minute)
		require.NoError(t, err)
		require.Equal(t, requested.Revision, build.Snapshot.Draft.Revision)
		require.Equal(t, requested.Title, build.Snapshot.Draft.Title, "the snapshot never sees the concurrent edit")
	}
	t.Logf("built=%d conflicted=%d", built, conflicted)
}

// The snapshot's object set and the held input references are enforced to be equal by the
// database itself, so a writer that gets them out of sync cannot commit.
func TestBuildSnapshotInputSetMustMatchReferences(t *testing.T) {
	t.Parallel()
	ctx, pool, _, org, d, oid := buildableFixture(t)
	snapshot := `{"organization_id":"` + org.String() + `",` +
		`"input_objects":["` + oid.String() + `"],"draft":{"problem_id":"` + d.ProblemID.String() + `","revision":` + "2" + `,` +
		`"tests":[{"input":{"object_id":"` + oid.String() + `"},"answer":{"object_id":null}}],"assets":[]}}`
	_, err := pool.Exec(ctx, `INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,state,max_attempts,deadline)
 VALUES($1,$2,$3,2,$4::jsonb,'queued',1,clock_timestamp()+interval '1 minute')`,
		uuid.New(), d.ProblemID, org, snapshot)
	require.ErrorContains(t, err, "snapshot input object set mismatch")
}

func packageObject(org uuid.UUID) model.StoredObject {
	return model.NewStoredObject(org, model.UploadSpec{Purpose: "package", SHA256: strings.Repeat("a", 64), Size: 10})
}

// A stale worker cannot finish or publish after its attempt was recovered, and its archive
// never gets a row; the newer attempt publishes, and a retry resolves to the same Package.
func TestStaleAttemptCannotPublish(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	enqueue(t, ctx, repo, org, d)
	first, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	expireLease(t, ctx, pool, first.ID)
	require.NoError(t, repo.RecoverBuilds(ctx))
	second, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, *first.LeaseToken, *second.LeaseToken)

	stale := packageObject(org)
	require.ErrorIs(t, repo.FinishBuildAttempt(ctx, first.ID, *first.LeaseToken, []byte(`{"message":"late"}`), false), pkg.ErrConflict)
	_, err = repo.PublishBuild(ctx, first.ID, *first.LeaseToken, stale, nil)
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, stale.ID))

	output := packageObject(org)
	packageID, err := repo.PublishBuild(ctx, second.ID, *second.LeaseToken, output, nil)
	require.NoError(t, err)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE id=$1 AND object_id=$2`, packageID, output.ID))
	// A retry after a lost commit response resolves to the same Package and records nothing.
	again, err := repo.PublishBuild(ctx, second.ID, *second.LeaseToken, packageObject(org), nil)
	require.NoError(t, err)
	require.Equal(t, packageID, again)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE purpose='package'`))
}

// Every reference is a foreign key: an object row that something references cannot be
// deleted, by GC or directly, and a reference must point at an object of its own organization.
func TestObjectReferencesAreForeignKeys(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	input := insertAvailableObject(t, ctx, pool, org)
	answer := insertAvailableObject(t, ctx, pool, org)
	asset := insertAvailableObject(t, ctx, pool, org)
	packageAsset := insertAvailableObject(t, ctx, pool, org)
	d = makeBuildableDraft(d, input)
	d.Tests[0].Answer.ObjectID = &answer
	d.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: asset}}
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	requireHeld := func(object uuid.UUID, constraint string) {
		t.Helper()
		deleted, err := repo.DeleteObject(ctx, object)
		require.NoError(t, err)
		require.False(t, deleted)
		_, err = pool.Exec(ctx, `DELETE FROM stored_objects WHERE id=$1`, object)
		require.ErrorContains(t, err, constraint)
	}
	for _, object := range []uuid.UUID{input, answer, asset} {
		requireHeld(object, "problem_draft_objects_object_id_organization_id_fkey")
	}

	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	output := packageObject(org)
	packageID, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output, nil)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	d.Tests, d.Assets = nil, nil
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	requireHeld(input, "problem_build_inputs_object_id_fkey")
	requireHeld(output.ID, "problem_packages_object_id_fkey")
	_, err = pool.Exec(ctx, `INSERT INTO problem_package_assets(package_id,organization_id,name,object_id) VALUES($1,$2,'b.png',$3)`, packageID, org, packageAsset)
	require.NoError(t, err)
	requireHeld(packageAsset, "problem_package_assets_object_id_fkey")

	other := helperCreateOrg(t, ctx, pool, "other-"+uuid.NewString(), "Other")
	foreign := insertAvailableObject(t, ctx, pool, other)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	d.Assets = []model.DraftAsset{{Name: "f.png", ObjectID: foreign}}
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.ErrorIs(t, err, pkg.ErrConflict, "a draft cannot reference another organization's object")
	_, err = pool.Exec(ctx, `INSERT INTO problem_package_assets(package_id,organization_id,name,object_id) VALUES($1,$2,'f.png',$3)`, packageID, org, foreign)
	require.ErrorContains(t, err, "problem_package_assets_object_id_fkey")
	_, err = pool.Exec(ctx, `INSERT INTO problem_package_assets(package_id,organization_id,name,object_id) VALUES($1,$2,'f.png',$3)`, packageID, other, foreign)
	require.ErrorContains(t, err, "problem_package_assets_package_id_fkey", "the reference's organization is its owner's")

	// Once the Package and the build are pruned, nothing holds the objects any more.
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, repo.PrunePackages(ctx, time.Millisecond))
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	for _, object := range []uuid.UUID{input, answer, asset, packageAsset, output.ID} {
		deleted, err := repo.DeleteObject(ctx, object)
		require.NoError(t, err)
		require.True(t, deleted)
	}
}

// GC deleting an object and a writer adding the first reference to it serialize on the
// object row: a reference committed while GC waits makes the deletion fail.
func TestObjectDeletionWaitsForConcurrentReference(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	object := insertAvailableObject(t, ctx, pool, org)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO problem_draft_objects(problem_id,organization_id,object_id) VALUES($1,$2,$3)`, d.ProblemID, org, object)
	require.NoError(t, err)

	type outcome struct {
		deleted bool
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		deleted, err := repo.DeleteObject(ctx, object)
		done <- outcome{deleted, err}
	}()
	select {
	case <-done:
		t.Fatal("the deletion must wait for the referencing transaction")
	case <-time.After(300 * time.Millisecond):
	}
	require.NoError(t, tx.Commit(ctx))

	got := <-done
	require.NoError(t, got.err)
	require.False(t, got.deleted, "a reference committed during the wait forbids deletion")
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, object))
}

// Concurrent GC and a new reference never leave a dangling reference: either the object
// is deleted and the reference is rejected, or the reference wins and the object stays.
func TestObjectDeletionRacingWithNewReference(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	for attempt := 0; attempt < 10; attempt++ {
		object := insertAvailableObject(t, ctx, pool, org)
		current, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		candidate := current
		candidate.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: object}}

		var deleted bool
		var deleteErr, linkErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			deleted, deleteErr = repo.DeleteObject(ctx, object)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, linkErr = repo.SaveDraft(ctx, mutationKey(org, "edit", candidate), candidate)
		}()
		close(start)
		wg.Wait()
		require.NoError(t, deleteErr)
		if deleted {
			require.ErrorIs(t, linkErr, pkg.ErrConflict, "a deleted object cannot gain a reference")
			require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, object))
			continue
		}
		require.NoError(t, linkErr)
		require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_draft_objects WHERE object_id=$1`, object))
		// Release the reference again so the next round starts from a clean draft.
		cleared, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		cleared.Assets = nil
		_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", cleared), cleared)
		require.NoError(t, err)
	}
}

// An executor whose operation lease expired cannot finalize, and after a takeover its
// uploaded bytes never get a row; the new executor finalizes with its own objects.
func TestUploadFinalizeIsFencedByOperationToken(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	change := model.UploadChange{Part: "asset", AssetName: "a.png"}
	key := mutationKey(org, "upload", change)
	replay, err := repo.BeginOperation(ctx, key)
	require.NoError(t, err)
	require.Nil(t, replay)
	spec := model.UploadSpec{Purpose: "asset", SHA256: strings.Repeat("a", 64), Size: 5}
	stale := model.NewStoredObject(org, spec)
	_, err = pool.Exec(ctx, `UPDATE request_idempotency SET lease_deadline=clock_timestamp()-interval '1 second' WHERE key=$1`, key.Key)
	require.NoError(t, err)
	_, err = repo.FinalizeUploads(ctx, key, d.ProblemID, d.Revision, change, []model.StoredObject{stale})
	require.ErrorIs(t, err, pkg.ErrConflict, "an expired executor cannot finalize")

	retry := key
	retry.Token = uuid.New()
	replay, err = repo.BeginOperation(ctx, retry)
	require.NoError(t, err)
	require.Nil(t, replay)
	_, err = repo.FinalizeUploads(ctx, key, d.ProblemID, d.Revision, change, []model.StoredObject{stale})
	require.ErrorIs(t, err, pkg.ErrConflict, "a taken-over executor cannot finalize")
	fresh := model.NewStoredObject(org, spec)
	result, err := repo.FinalizeUploads(ctx, retry, d.ProblemID, d.Revision, change, []model.StoredObject{fresh})
	require.NoError(t, err)
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, stale.ID))
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_draft_objects WHERE object_id=$1`, fresh.ID))

	// A lost response replays the stored result before anything is uploaded again.
	again := retry
	again.Token = uuid.New()
	replay, err = repo.BeginOperation(ctx, again)
	require.NoError(t, err)
	require.NotNil(t, replay)
	require.JSONEq(t, string(result.Body), string(replay.Body))
}

// sweepStore is an object store listing with controlled modification times.
type sweepStore struct {
	objects map[string]time.Time
	deleted []string
}

func (s *sweepStore) DeleteFile(_ context.Context, _, key string) error {
	delete(s.objects, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *sweepStore) ListObjects(context.Context, string, string) ([]storage.ObjectInfo, error) {
	listed := []storage.ObjectInfo{}
	for key, modified := range s.objects {
		listed = append(listed, storage.ObjectInfo{Key: key, LastModified: modified})
	}
	return listed, nil
}

// GC deletes unreferenced object rows with their bytes, and bytes without a row once they
// are older than the retention; bytes with a row and young bytes stay.
func TestGCDeletesUnreferencedObjectsAndOrphanedBytes(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	referenced := insertAvailableObject(t, ctx, pool, org)
	unreferenced := insertAvailableObject(t, ctx, pool, org)
	d.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: referenced}}
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	key := func(id uuid.UUID) string { return "objects/" + org.String() + "/" + id.String() }
	old, young := time.Now().Add(-2*time.Hour), time.Now()
	orphanOld, orphanYoung := key(uuid.New()), key(uuid.New())
	store := &sweepStore{objects: map[string]time.Time{
		key(referenced): old, key(unreferenced): young, orphanOld: old, orphanYoung: young,
	}}
	gc := builder.GC{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Repo: repo, Store: store, Bucket: "objects", TempDir: t.TempDir(),
		ObjectRetention: time.Hour, BuildRetention: time.Hour, PackageRetention: time.Hour, Interval: time.Minute}
	require.NoError(t, gc.Collect(ctx))

	require.ElementsMatch(t, []string{key(unreferenced), orphanOld}, store.deleted)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, referenced))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, unreferenced))
}

// Retention removes only Packages without consumers: a Package pinned by a contest survives
// any age, an unused one is deleted and its archive is released for collection.
func TestPackageRetentionKeepsPinnedPackages(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	require.NoError(t, f.create(f.original))
	unused := f.pack(t, f.problem, time.Now().Add(-2*time.Hour))
	drafts := postgres.NewProblemDraftsRepo(f.pool, model.DraftQuotas{Files: 10, Tests: 10, Components: 10, Subtasks: 10}, time.Minute)

	var unusedObject, pinnedObject uuid.UUID
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT object_id FROM problem_packages WHERE id=$1`, unused).Scan(&unusedObject))
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT object_id FROM problem_packages WHERE id=$1`, f.original).Scan(&pinnedObject))
	require.NoError(t, drafts.PrunePackages(f.ctx, time.Millisecond))

	require.Zero(t, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, unused))
	require.Equal(t, 1, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, f.original), "pinned package is kept")
	deleted, err := drafts.DeleteObject(f.ctx, unusedObject)
	require.NoError(t, err)
	require.True(t, deleted, "the archive of the removed Package becomes collectable")
	deleted, err = drafts.DeleteObject(f.ctx, pinnedObject)
	require.NoError(t, err)
	require.False(t, deleted)

	// Even direct SQL cannot remove a pinned Package: the foreign keys restrict it.
	require.Error(t, func() error {
		_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_packages WHERE id=$1`, f.original)
		return err
	}(), "foreign keys forbid deleting a pinned package")
}

// claimedBuildWithAsset is a claimed build whose draft has one statement asset,
// with its output object ready to publish.
func claimedBuildWithAsset(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft, model.ProblemBuild, model.StoredObject, uuid.UUID) {
	t.Helper()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	asset := insertAvailableObject(t, ctx, pool, org)
	d.Assets = []model.DraftAsset{{Name: "pic.png", ObjectID: asset}}
	d.Statements = []model.DraftStatement{{Language: "ru", Legend: "Сложите числа"}}
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	return ctx, pool, repo, org, d, build, packageObject(org), asset
}

// A Package stores its statement samples and references its statement assets at
// publication; the assets stay alive with the Package and are released when it is pruned.
func TestPackageStatementContentLivesWithPackage(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, build, output, asset := claimedBuildWithAsset(t)
	samples := []model.Sample{{Input: "1 2\n", Output: "3\n"}}
	packageID, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output, samples)
	require.NoError(t, err)

	statement, err := repo.GetPackageStatement(ctx, packageID)
	require.NoError(t, err)
	require.Equal(t, samples, statement.Samples)
	require.Equal(t, d.Title, statement.Title)
	require.Equal(t, d.Statements, statement.Statements)
	require.Equal(t, map[string]string{"pic.png": strings.Repeat("0", 64)}, statement.Assets)
	found, err := repo.FindPackageAssets(ctx, packageID, strings.Repeat("0", 64))
	require.NoError(t, err)
	require.Equal(t, []model.PackageAsset{{Name: "pic.png", Key: "objects/" + org.String() + "/" + asset.String()}}, found)

	// The draft drops the asset and changes its statement; the Package keeps both.
	d.Assets = nil
	d.Statements = []model.DraftStatement{{Language: "ru", Legend: "changed"}}
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	statement, err = repo.GetPackageStatement(ctx, packageID)
	require.NoError(t, err)
	require.Equal(t, "Сложите числа", statement.Statements[0].Legend)
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	deleted, err := repo.DeleteObject(ctx, asset)
	require.NoError(t, err)
	require.False(t, deleted, "an asset of a live Package is not collected")
	_, err = pool.Exec(ctx, `UPDATE problem_package_assets SET name='other.png' WHERE package_id=$1`, packageID)
	require.ErrorContains(t, err, "package is immutable")

	require.NoError(t, repo.PrunePackages(ctx, time.Millisecond))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_package_assets WHERE package_id=$1`, packageID))
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	deleted, err = repo.DeleteObject(ctx, asset)
	require.NoError(t, err)
	require.True(t, deleted, "the asset of a pruned Package becomes collectable")
}
