//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testBuildOptions() postgres.ProblemBuildOptions {
	return postgres.ProblemBuildOptions{
		Toolchain:   model.Toolchain{Digest: "sha256:" + strings.Repeat("a", 64), Architecture: "amd64", ABI: "linux-gnu", Languages: map[string]sandbox.Language{"cpp": {Type: "compiler"}}},
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
	snapshot := `{"schema_version":1,"organization_id":"` + org.String() + `","toolchain":{"digest":"sha256:` + strings.Repeat("a", 64) + `"},` +
		`"input_objects":["` + oid.String() + `"],"draft":{"problem_id":"` + d.ProblemID.String() + `","revision":` + "2" + `,` +
		`"tests":[{"input":{"object_id":"` + oid.String() + `"},"answer":{"object_id":null}}],"assets":[]}}`
	_, err := pool.Exec(ctx, `INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,schema_version,toolchain_digest,state,max_attempts,deadline)
 VALUES($1,$2,$3,2,$4::jsonb,1,$5,'queued',1,clock_timestamp()+interval '1 minute')`,
		uuid.New(), d.ProblemID, org, snapshot, "sha256:"+strings.Repeat("a", 64))
	require.ErrorContains(t, err, "snapshot input object set mismatch")
}

// Closing a stale attempt releases only that attempt's output hold and never the snapshot
// inputs or a newer attempt's hold; a stale worker cannot finish or publish.
func TestStaleAttemptCannotTouchNewerAttempt(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	enqueue(t, ctx, repo, org, d)
	first, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	firstOutput, err := repo.RegisterBuildOutput(ctx, first.ID, *first.LeaseToken, strings.Repeat("1", 64), 10, time.Minute)
	require.NoError(t, err)

	expireLease(t, ctx, pool, first.ID)
	require.NoError(t, repo.RecoverBuilds(ctx))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='output'`, first.ID),
		"recovery releases the closed attempt's output hold")

	second, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, *first.LeaseToken, *second.LeaseToken)
	secondOutput, err := repo.RegisterBuildOutput(ctx, second.ID, *second.LeaseToken, strings.Repeat("2", 64), 10, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, firstOutput.ID, secondOutput.ID, "a new attempt never reuses a closed upload UUID")

	// The stale worker's late calls are all rejected and leave the new attempt untouched.
	require.ErrorIs(t, repo.FinishBuildAttempt(ctx, first.ID, *first.LeaseToken, []byte(`{"message":"late"}`), false), pkg.ErrConflict)
	_, err = repo.PublishBuild(ctx, first.ID, *first.LeaseToken, firstOutput.ID, nil)
	require.ErrorIs(t, err, pkg.ErrConflict)
	_, err = repo.RegisterBuildOutput(ctx, first.ID, *first.LeaseToken, strings.Repeat("3", 64), 10, time.Minute)
	require.ErrorIs(t, err, pkg.ErrConflict)

	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='output' AND attempt_token=$2`, second.ID, *second.LeaseToken))
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='input'`, first.ID))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, first.ID))
}

// Publication moves ownership of the archive from the attempt to the Package in one commit.
func TestPublishBuildHandsOwnershipToPackageAtomically(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	output, err := repo.RegisterBuildOutput(ctx, build.ID, *build.LeaseToken, strings.Repeat("a", 64), 10, time.Minute)
	require.NoError(t, err)
	require.Equal(t, "uploading", output.State)

	packageID, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output.ID, nil)
	require.NoError(t, err)
	var state string
	var unreferenced *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,unreferenced_at FROM stored_objects WHERE id=$1`, output.ID).Scan(&state, &unreferenced))
	require.Equal(t, "available", state)
	require.Nil(t, unreferenced, "the Package holds the object from the moment it is available")
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE id=$1 AND object_id=$2 AND build_id=$3`, packageID, output.ID, build.ID))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='output'`, build.ID))
	got, err := repo.GetBuild(ctx, build.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", got.State)

	// A retry after a lost commit response resolves to the same Package.
	again, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output.ID, nil)
	require.NoError(t, err)
	require.Equal(t, packageID, again)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, build.ID))
}

func makeUnreferenced(t *testing.T, ctx context.Context, pool *pgxpool.Pool, object uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE stored_objects SET unreferenced_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, object)
	require.NoError(t, err)
}

// A collector that is already waiting for the object lock must observe a reference committed
// while it waited: the reference check is a separate statement after the lock arrives.
func TestObjectDeletionSeesReferenceCommittedWhileWaitingForLock(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	object := insertAvailableObject(t, ctx, pool, org)
	makeUnreferenced(t, ctx, pool, object)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = sqlc.New(tx).LockStoredObject(ctx, object)
	require.NoError(t, err)

	type outcome struct {
		started bool
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		started, err := repo.BeginObjectDeletion(ctx, object, time.Millisecond)
		done <- outcome{started, err}
	}()
	select {
	case <-done:
		t.Fatal("the collector must wait for the object lock")
	case <-time.After(300 * time.Millisecond):
	}
	// The lock holder links the object into the draft, then commits.
	_, err = tx.Exec(ctx, `INSERT INTO problem_draft_assets(problem_id,path,object_id) VALUES($1,'assets/a.png',$2)`, d.ProblemID, object)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	got := <-done
	require.NoError(t, got.err)
	require.False(t, got.started, "a reference committed during the wait forbids deletion")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM stored_objects WHERE id=$1`, object).Scan(&state))
	require.Equal(t, "available", state)
}

func TestObjectDeletionProtocolIsOneWay(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, _ := draftFixture(t)
	object := insertAvailableObject(t, ctx, pool, org)

	started, err := repo.BeginObjectDeletion(ctx, object, time.Hour)
	require.NoError(t, err)
	require.False(t, started, "recently released objects are retained")

	makeUnreferenced(t, ctx, pool, object)
	started, err = repo.BeginObjectDeletion(ctx, object, time.Millisecond)
	require.NoError(t, err)
	require.True(t, started)
	// Once deleting, no reference can be created and the state cannot return to available.
	_, err = pool.Exec(ctx, `UPDATE stored_objects SET state='available' WHERE id=$1`, object)
	require.ErrorContains(t, err, "invalid object transition")
	started, err = repo.BeginObjectDeletion(ctx, object, time.Millisecond)
	require.NoError(t, err)
	require.True(t, started, "a failed physical delete is retried from the deleting state")

	require.NoError(t, repo.CompleteObjectDeletion(ctx, object))
	require.ErrorIs(t, repo.CompleteObjectDeletion(ctx, object), pkg.ErrConflict)
	started, err = repo.BeginObjectDeletion(ctx, object, time.Millisecond)
	require.NoError(t, err)
	require.False(t, started)
}

// An upload that missed its deadline can never become a working reference, and its
// operation is closed so the object becomes collectable.
func TestExpiredUploadIsNeverPublishedAndBecomesCollectable(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	change := model.UploadChange{Part: "asset", AssetPath: "assets/late.png"}
	key := mutationKey(org, "upload", struct {
		Revision int64
		Change   model.UploadChange
	}{d.Revision, change})
	reservation, err := repo.ReserveUploads(ctx, key, []model.UploadSpec{{Purpose: "asset", SHA256: strings.Repeat("a", 64), Size: 5}}, 150*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, reservation.Objects, 1)
	object := reservation.Objects[0]
	require.Equal(t, "uploading", object.State)

	_, err = pool.Exec(ctx, `SELECT pg_sleep(0.3)`)
	require.NoError(t, err)
	_, err = repo.FinalizeUploads(ctx, key, d.ProblemID, d.Revision, change)
	require.Error(t, err, "an expired upload cannot be finalized")
	var draftAssets int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM problem_draft_assets WHERE problem_id=$1`, d.ProblemID).Scan(&draftAssets))
	require.Zero(t, draftAssets)

	// Retrying the operation after expiry uses a brand-new UUID, never the closed key.
	retry, err := repo.ReserveUploads(ctx, key, []model.UploadSpec{{Purpose: "asset", SHA256: strings.Repeat("a", 64), Size: 5}}, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, object.ID, retry.Objects[0].ID)
	require.NotEqual(t, object.Key, retry.Objects[0].Key)

	started, err := repo.BeginObjectDeletion(ctx, object.ID, time.Millisecond)
	require.NoError(t, err)
	require.True(t, started, "the abandoned, expired upload is collectable")
}

// Concurrent GC and a new reference never delete a working object: whichever side takes the
// object lock first decides, and the loser observes the outcome.
func TestObjectDeletionRacingWithNewReference(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	for attempt := 0; attempt < 10; attempt++ {
		object := insertAvailableObject(t, ctx, pool, org)
		makeUnreferenced(t, ctx, pool, object)
		current, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		candidate := current
		candidate.Assets = []model.DraftAsset{{Path: "assets/a.png", ObjectID: object}}

		var deleting bool
		var deleteErr, linkErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			deleting, deleteErr = repo.BeginObjectDeletion(ctx, object, time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, linkErr = repo.SaveDraft(ctx, mutationKey(org, "edit", candidate), candidate)
		}()
		close(start)
		wg.Wait()
		require.NoError(t, deleteErr)
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM stored_objects WHERE id=$1`, object).Scan(&state))
		if deleting {
			require.Error(t, linkErr, "an object being deleted cannot gain a reference")
			require.Equal(t, "deleting", state)
			continue
		}
		require.NoError(t, linkErr)
		require.Equal(t, "available", state)
		require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_draft_assets WHERE object_id=$1`, object))
		// Release the reference again so the next round starts from a clean draft.
		cleared, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		cleared.Assets = nil
		_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", cleared), cleared)
		require.NoError(t, err)
	}
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

	var unusedObject uuid.UUID
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT object_id FROM problem_packages WHERE id=$1`, unused).Scan(&unusedObject))
	require.NoError(t, drafts.PrunePackages(f.ctx, time.Millisecond))

	require.Zero(t, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, unused))
	require.Equal(t, 1, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, f.original), "pinned package is kept")
	var releasedAt *time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT unreferenced_at FROM stored_objects WHERE id=$1`, unusedObject).Scan(&releasedAt))
	require.NotNil(t, releasedAt, "the archive of the removed Package becomes collectable")

	// Even direct SQL cannot remove a pinned Package: the foreign keys restrict it.
	require.Error(t, func() error {
		_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_packages WHERE id=$1`, f.original)
		return err
	}(), "foreign keys forbid deleting a pinned package")
}

// A Package stores its statement samples and references its statement assets at
// publication; the assets stay alive with the Package and are released when it is pruned.
// claimedBuildWithAsset is a claimed build whose draft has one statement asset,
// with its output registered and ready to publish.
func claimedBuildWithAsset(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft, model.ProblemBuild, model.StoredObject, uuid.UUID) {
	t.Helper()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	asset := insertAvailableObject(t, ctx, pool, org)
	d.Assets = []model.DraftAsset{{Path: "assets/pic.png", ObjectID: asset}}
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	output, err := repo.RegisterBuildOutput(ctx, build.ID, *build.LeaseToken, strings.Repeat("a", 64), 10, time.Minute)
	require.NoError(t, err)
	return ctx, pool, repo, org, d, build, output, asset
}

func TestPackageStatementContentLivesWithPackage(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, build, output, asset := claimedBuildWithAsset(t)
	samples := []model.Sample{{Input: "1 2\n", Output: "3\n"}}
	packageID, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output.ID, samples)
	require.NoError(t, err)

	statement, err := repo.GetPackageStatement(ctx, packageID)
	require.NoError(t, err)
	require.Equal(t, samples, statement.Samples)
	require.Equal(t, map[string]string{"assets/pic.png": strings.Repeat("0", 64)}, statement.Assets)
	found, err := repo.FindPackageAssets(ctx, packageID, strings.Repeat("0", 64))
	require.NoError(t, err)
	require.Equal(t, []model.PackageAsset{{Path: "assets/pic.png", Key: "objects/" + org.String() + "/" + asset.String()}}, found)

	// The draft drops the asset; the Package keeps it.
	d.Assets = nil
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	var unreferenced *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT unreferenced_at FROM stored_objects WHERE id=$1`, asset).Scan(&unreferenced))
	require.Nil(t, unreferenced, "the Package references its asset")
	deleting, err := repo.BeginObjectDeletion(ctx, asset, time.Millisecond)
	require.NoError(t, err)
	require.False(t, deleting, "an asset of a live Package is not collected")
	_, err = pool.Exec(ctx, `UPDATE problem_package_assets SET path='assets/other.png' WHERE package_id=$1`, packageID)
	require.ErrorContains(t, err, "package is immutable")

	require.NoError(t, repo.PrunePackages(ctx, time.Millisecond))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_package_assets WHERE package_id=$1`, packageID))
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	require.NoError(t, pool.QueryRow(ctx, `SELECT unreferenced_at FROM stored_objects WHERE id=$1`, asset).Scan(&unreferenced))
	require.NotNil(t, unreferenced, "the asset of a pruned Package becomes collectable")
}

// Problem creation from a template bumps the access revision and then locks the
// template's objects. Publishing and pruning a Package with the same asset take
// the access revision before that object too, so neither can deadlock with it.
func TestPackageAssetLocksFollowAccessRevision(t *testing.T) {
	ctx, pool, repo, _, _, build, output, asset := claimedBuildWithAsset(t)
	cloneAgainst := func(name string, operation func() error) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `UPDATE access_revision SET revision=revision+1`)
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() { done <- operation() }()
		require.Eventually(t, func() bool {
			return countRows(t, ctx, pool, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%FROM access_revision FOR UPDATE%'`) > 0
		}, 10*time.Second, 10*time.Millisecond, "%s waits for the access revision first", name)
		_, err = tx.Exec(ctx, `SET LOCAL lock_timeout='5s'`)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM stored_objects WHERE id=$1 FOR UPDATE`, asset)
		require.NoError(t, err, "%s does not hold the asset while waiting", name)
		require.NoError(t, tx.Commit(ctx))
		require.NoError(t, <-done, name)
	}

	var packageID uuid.UUID
	cloneAgainst("publication", func() error {
		var err error
		packageID, err = repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output.ID, nil)
		return err
	})
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_package_assets WHERE package_id=$1 AND object_id=$2`, packageID, asset))
	time.Sleep(5 * time.Millisecond)
	cloneAgainst("pruning", func() error { return repo.PrunePackages(ctx, time.Millisecond) })
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, packageID))
}
