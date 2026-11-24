//go:build integration

package postgres_test

import (
	"context"
	"fmt"
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

func buildableFixture(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft, uuid.UUID) {
	t.Helper()
	ctx, pool, repo, org, d := draftFixture(t)
	oid := draftObject(t, ctx, pool, org, d.ProblemID)
	d = makeBuildableDraft(d, oid)
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	return ctx, pool, repo, org, d, oid
}

func enqueue(t *testing.T, ctx context.Context, repo *postgres.ProblemDraftsRepo, org uuid.UUID, d model.Draft) {
	t.Helper()
	_, err := repo.EnqueueBuild(ctx, d.ProblemID, d.Revision, testBuildOptions())
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

// A racing build snapshots its requested revision or conflicts; it never mixes revisions.
func TestBuildRequestSnapshotsOneRevisionWhileEditing(t *testing.T) {
	t.Parallel()
	ctx, _, repo, _, d, _ := buildableFixture(t)
	built, conflicted := 0, 0
	for attempt := 0; attempt < 8; attempt++ {
		current, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		before := current
		before.Title = "before " + uuid.NewString()
		_, err = repo.SaveDraft(ctx, before)
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
			_, buildErr = repo.EnqueueBuild(ctx, requested.ProblemID, requested.Revision, testBuildOptions())
		}()
		go func() {
			defer wg.Done()
			<-start
			_, editErr = repo.SaveDraft(ctx, edited)
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

func packageObject(org uuid.UUID) model.StoredObject {
	return model.NewStoredObject(org, model.UploadSpec{Purpose: "package", SHA256: strings.Repeat("a", 64), Size: 10})
}

// An attempt whose lease ran out publishes nothing, even before recovery requeues it.
func TestExpiredLeaseCannotPublish(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	expireLease(t, ctx, pool, build.ID)
	output := packageObject(org)
	_, err = repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output, model.NewPackageIndex(d, built(d), nil, nil))
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, build.ID))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, output.ID))
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_builds WHERE id=$1 AND state='running'`, build.ID))
}

// Recovered attempts cannot publish; the current attempt publishes one replayable package.
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
	_, err = repo.PublishBuild(ctx, first.ID, *first.LeaseToken, stale, model.NewPackageIndex(d, built(d), nil, nil))
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, stale.ID))

	output := packageObject(org)
	packageID, err := repo.PublishBuild(ctx, second.ID, *second.LeaseToken, output, model.NewPackageIndex(d, built(d), nil, nil))
	require.NoError(t, err)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM problem_packages WHERE id=$1 AND object_id=$2`, packageID, output.ID))
	// A retry after a lost commit response resolves to the same Package and records nothing.
	again, err := repo.PublishBuild(ctx, second.ID, *second.LeaseToken, packageObject(org), model.NewPackageIndex(d, built(d), nil, nil))
	require.NoError(t, err)
	require.Equal(t, packageID, again)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE purpose='package'`))
}

// GC keeps every object a draft, build or package still references.
func TestGCKeepsReferencedObjects(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	input := draftObject(t, ctx, pool, org, d.ProblemID)
	answer := draftObject(t, ctx, pool, org, d.ProblemID)
	asset := draftObject(t, ctx, pool, org, d.ProblemID)
	d = makeBuildableDraft(d, input)
	d.Tests[0].Answer.ObjectID = &answer
	d.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: asset}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	for _, object := range []uuid.UUID{input, answer, asset} {
		require.True(t, objectExists(t, ctx, pool, repo, object), "the draft references it")
	}

	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	output := packageObject(org)
	_, err = repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output, model.NewPackageIndex(d, built(d), nil, nil))
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	d.Tests = nil
	_, err = repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	require.True(t, objectExists(t, ctx, pool, repo, input), "the build snapshot references it")
	require.True(t, objectExists(t, ctx, pool, repo, output.ID), "the package references it")
	require.True(t, objectExists(t, ctx, pool, repo, asset), "the package assets reference it")

	time.Sleep(5 * time.Millisecond)
	require.NoError(t, repo.PrunePackages(ctx, time.Millisecond))
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	require.True(t, objectExists(t, ctx, pool, repo, asset), "the draft still references it")
	for _, object := range []uuid.UUID{input, answer, output.ID} {
		require.False(t, objectExists(t, ctx, pool, repo, object))
	}
}

// A draft only references objects stored with it, so GC never races a new reference.
func TestDraftRejectsObjectsItDoesNotHold(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	other := helperCreateOrg(t, ctx, pool, "other-"+uuid.NewString(), "Other")
	for name, object := range map[string]uuid.UUID{
		"another organization's object": insertAvailableObject(t, ctx, pool, other),
		"an unattached object":          insertAvailableObject(t, ctx, pool, org),
	} {
		candidate := d
		candidate.Assets = []model.DraftAsset{{Name: "f.png", ObjectID: object}}
		_, err := repo.SaveDraft(ctx, candidate)
		require.ErrorIs(t, err, pkg.ErrConflict, name)
	}

	asset := draftObject(t, ctx, pool, org, d.ProblemID)
	d.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: asset}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	removed := d
	removed.Assets = nil
	_, err = repo.SaveDraft(ctx, removed)
	require.NoError(t, err)
	d.Revision++
	_, err = repo.SaveDraft(ctx, d)
	require.ErrorIs(t, err, pkg.ErrConflict, "a dropped object cannot come back")
}

// GC racing with edits that keep an object never deletes it.
func TestObjectDeletionRacingWithEdits(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	object := draftObject(t, ctx, pool, org, d.ProblemID)
	d.Assets = []model.DraftAsset{{Name: "a0.png", ObjectID: object}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	for attempt := 1; attempt <= 10; attempt++ {
		current, err := repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		current.Assets[0].Name = fmt.Sprintf("a%d.png", attempt)
		var gcErr, editErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, gcErr = repo.DeleteUnreferencedObjects(ctx)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, editErr = repo.SaveDraft(ctx, current)
		}()
		close(start)
		wg.Wait()
		require.NoError(t, gcErr)
		require.NoError(t, editErr)
		require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, object))
	}
}

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

// GC preserves referenced objects and bytes that are still within retention.
func TestGCDeletesUnreferencedObjectsAndOrphanedBytes(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	referenced := draftObject(t, ctx, pool, org, d.ProblemID)
	unreferenced := insertAvailableObject(t, ctx, pool, org)
	d.Assets = []model.DraftAsset{{Name: "a.png", ObjectID: referenced}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	key := func(id uuid.UUID) string { return "objects/" + org.String() + "/" + id.String() }
	old, young := time.Now().Add(-2*time.Hour), time.Now()
	orphanOld, orphanYoung := key(uuid.New()), key(uuid.New())
	store := &sweepStore{objects: map[string]time.Time{
		// Old bytes: a parallel test's GC may delete the unreferenced row first, leaving them to the orphan sweep.
		key(referenced): old, key(unreferenced): old, orphanOld: old, orphanYoung: young,
	}}
	gc := builder.GC{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Repo: repo, Store: store, Bucket: "objects", TempDir: t.TempDir(),
		ObjectRetention: time.Hour, BuildRetention: time.Hour, PackageRetention: time.Hour, Interval: time.Minute}
	require.NoError(t, gc.Collect(ctx))

	require.ElementsMatch(t, []string{key(unreferenced), orphanOld}, store.deleted)
	require.Equal(t, 1, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, referenced))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects WHERE id=$1`, unreferenced))
}

// Retention preserves pinned packages and releases unconsumed archives for GC.
func TestPackageRetentionKeepsPinnedPackages(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	require.NoError(t, f.create(f.original))
	unused := f.pack(t, f.problem, time.Now().Add(-2*time.Hour))
	drafts := postgres.NewProblemDraftsRepo(f.pool, model.DraftQuotas{Files: 10, Tests: 10, Components: 10, Subtasks: 10})

	var unusedObject, pinnedObject uuid.UUID
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT object_id FROM problem_packages WHERE id=$1`, unused).Scan(&unusedObject))
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT object_id FROM problem_packages WHERE id=$1`, f.original).Scan(&pinnedObject))
	require.NoError(t, drafts.PrunePackages(f.ctx, time.Millisecond))

	require.Zero(t, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, unused))
	require.Equal(t, 1, countRows(t, f.ctx, f.pool, `SELECT count(*) FROM problem_packages WHERE id=$1`, f.original), "pinned package is kept")
	require.False(t, objectExists(t, f.ctx, f.pool, drafts, unusedObject), "the archive of the removed Package becomes collectable")
	require.True(t, objectExists(t, f.ctx, f.pool, drafts, pinnedObject))

	// Even direct SQL cannot remove a pinned Package: the foreign keys restrict it.
	require.Error(t, func() error {
		_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_packages WHERE id=$1`, f.original)
		return err
	}(), "foreign keys forbid deleting a pinned package")
}

func claimedBuildWithAsset(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft, model.ProblemBuild, model.StoredObject, uuid.UUID) {
	t.Helper()
	ctx, pool, repo, org, d, _ := buildableFixture(t)
	asset := draftObject(t, ctx, pool, org, d.ProblemID)
	d.Assets = []model.DraftAsset{{Name: "pic.png", ObjectID: asset}}
	d.Statements = []model.DraftStatement{{Language: "ru", Legend: "Сложите числа"}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	enqueue(t, ctx, repo, org, d)
	build, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	return ctx, pool, repo, org, d, build, packageObject(org), asset
}

// Package publication retains statement samples and assets until pruning.
func TestPackageStatementContentLivesWithPackage(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d, build, output, asset := claimedBuildWithAsset(t)
	samples := []model.Sample{{Input: "1 2\n", Output: "3\n"}}
	pdf := model.NewStoredObject(org, model.UploadSpec{Purpose: "statement", SHA256: strings.Repeat("1", 64), Size: 3})
	packageID, err := repo.PublishBuild(ctx, build.ID, *build.LeaseToken, output, model.NewPackageIndex(d, built(d), []model.StatementPDF{{Language: "ru", Object: pdf}}, samples))
	require.NoError(t, err)

	statement, err := repo.GetPackageStatement(ctx, packageID)
	require.NoError(t, err)
	require.Equal(t, samples, statement.Samples)
	require.Equal(t, d.Title, statement.Title)
	require.Equal(t, built(d), statement.Statements)
	key, err := repo.FindPackagePDF(ctx, packageID, "ru")
	require.NoError(t, err)
	require.Equal(t, pdf.Key, key)
	require.Equal(t, map[string]string{"pic.png": strings.Repeat("0", 64)}, statement.Assets)
	found, err := repo.FindPackageAssets(ctx, packageID, strings.Repeat("0", 64))
	require.NoError(t, err)
	require.Equal(t, []model.PackageAsset{{Name: "pic.png", Key: "objects/" + org.String() + "/" + asset.String()}}, found)

	// The draft drops the asset and changes its statement; the Package keeps both.
	d.Assets = nil
	d.Statements = []model.DraftStatement{{Language: "ru", Legend: "changed"}}
	_, err = repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	statement, err = repo.GetPackageStatement(ctx, packageID)
	require.NoError(t, err)
	require.Equal(t, "Сложите числа", statement.Statements[0].Legend)
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	require.True(t, objectExists(t, ctx, pool, repo, asset), "an asset of a live Package is not collected")
	require.True(t, objectExists(t, ctx, pool, repo, pdf.ID), "a statement PDF of a live Package is not collected")

	require.NoError(t, repo.PrunePackages(ctx, time.Millisecond))
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM problem_package_assets WHERE package_id=$1`, packageID))
	require.NoError(t, repo.PruneBuilds(ctx, time.Millisecond))
	require.False(t, objectExists(t, ctx, pool, repo, asset), "the asset of a pruned Package becomes collectable")
	require.False(t, objectExists(t, ctx, pool, repo, pdf.ID), "the statement PDF of a pruned Package becomes collectable")
}

func built(d model.Draft) []model.BuiltStatement {
	statements := []model.BuiltStatement{}
	for _, s := range d.Statements {
		statements = append(statements, model.BuiltStatement{DraftStatement: s, HTML: map[string]string{"legend": "<p>" + s.Legend + "</p>"}})
	}
	return statements
}
