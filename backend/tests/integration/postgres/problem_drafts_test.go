//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func draftFixture(t *testing.T) (context.Context, *pgxpool.Pool, *postgres.ProblemDraftsRepo, uuid.UUID, model.Draft) {
	t.Helper()
	ctx := context.Background()
	pool := getTestPool(t)
	org := helperCreateOrg(t, ctx, pool, "draft-"+uuid.NewString(), "Drafts")
	id, owner := uuid.New(), helperOrgOwner(t, ctx, pool, org)
	// Draft writes authorize the caller, so the fixture acts as the problem owner.
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: owner, Role: accessmodel.UserRoleUser})
	require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: id, OrganizationID: org, Owner: owner, Title: "Initial", ShortName: "draft"}))
	repo := postgres.NewProblemDraftsRepo(pool, model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10})
	d, err := repo.GetDraft(ctx, id)
	require.NoError(t, err)
	return ctx, pool, repo, org, d
}
func makeBuildableDraft(d model.Draft, oid uuid.UUID) model.Draft {
	g := uuid.New()
	d.Checker = checker("cpp17", "int main(){return 0;}")
	d.Subtasks = []model.DraftSubtask{{ID: g, Ordinal: 1, Name: "All", Policy: "complete", Feedback: "complete"}}
	d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: g, Input: model.DraftInput{Mode: "manual", ObjectID: &oid}, Answer: model.DraftAnswer{Mode: "provided", ObjectID: &oid}}}
	return d
}
func insertAvailableObject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size) VALUES($1,$2,'test-input',$4,$3,0)`,
		id, org, strings.Repeat("0", 64), "objects/"+org.String()+"/"+id.String())
	require.NoError(t, err)
	return id
}

// draftObject stores an object as an upload does, so the draft may reference it.
func draftObject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, problem uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `WITH o AS (INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size) VALUES($1,$2,'test-input',$4,$3,0))
UPDATE problem_drafts SET object_ids=array_append(object_ids,$1) WHERE problem_id=$5`,
		id, org, strings.Repeat("0", 64), "objects/"+org.String()+"/"+id.String(), problem)
	require.NoError(t, err)
	return id
}

// objectExists runs object GC and reports whether the object survived it.
func objectExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *postgres.ProblemDraftsRepo, id uuid.UUID) bool {
	t.Helper()
	_, err := repo.DeleteUnreferencedObjects(ctx)
	require.NoError(t, err)
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM stored_objects WHERE id=$1`, id).Scan(&n))
	return n == 1
}
func TestProblemDraftConcurrentEdits(t *testing.T) {
	t.Parallel()
	ctx, _, repo, _, d := draftFixture(t)
	d.Statements = []model.DraftStatement{{Language: "en", Legend: "Original"}}
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	start := make(chan struct{})
	for _, title := range []string{"First", "Second"} {
		candidate := d
		candidate.Title = title
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.SaveDraft(ctx, candidate)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, pkg.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	current, err := repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	require.Equal(t, int64(2), current.Revision)
}
func TestProblemBuildSnapshotAndRecovery(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	oid := draftObject(t, ctx, pool, org, d.ProblemID)
	d = makeBuildableDraft(d, oid)
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	options := postgres.ProblemBuildOptions{Languages: map[string]sandbox.Language{"cpp17": {Type: "compiler"}}, MaxAttempts: 3, Duration: time.Minute, Lease: time.Second}
	buildID, err := repo.EnqueueBuild(ctx, d.ProblemID, d.Revision, options)
	require.NoError(t, err)
	d.Title = "Edited after snapshot"
	_, err = repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	b, err := repo.ClaimBuild(ctx, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, buildID, b.ID)
	require.Equal(t, "Initial", b.Snapshot.Draft.Title)
	require.Equal(t, int64(2), b.Revision)
	// Let database time pass the lease; heartbeat must never revive it.
	_, err = pool.Exec(ctx, `SELECT pg_sleep(0.01)`)
	require.NoError(t, err)
	require.ErrorIs(t, repo.HeartbeatBuild(ctx, b.ID, *b.LeaseToken, time.Minute), pkg.ErrConflict)
	require.NoError(t, repo.RecoverBuilds(ctx))
	next, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	require.Equal(t, b.ID, next.ID)
	require.NotEqual(t, *b.LeaseToken, *next.LeaseToken)
	require.Equal(t, b.Snapshot, next.Snapshot)
	require.ErrorIs(t, repo.FinishBuildAttempt(ctx, b.ID, *b.LeaseToken, json.RawMessage(`{"message":"late"}`), false), pkg.ErrConflict)
	err = repo.CancelBuild(ctx, b.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.HeartbeatBuild(ctx, next.ID, *next.LeaseToken, time.Minute), pkg.ErrConflict)
}

// Renames preserve object references and update tests that name the program.
func TestProblemDraftRenamesKeepReferences(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	object := draftObject(t, ctx, pool, org, d.ProblemID)
	generator, subtask := "gen", uuid.New()
	d.Library = []model.DraftLibraryFile{{Name: "testlib.h", Content: "// lib"}}
	d.Generators = []model.DraftNamedProgram{named(generator, "cpp17", "int main(){}")}
	d.Assets = []model.DraftAsset{{Name: "pic.png", ObjectID: object}}
	d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "complete", Feedback: "complete"}}
	d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Input: model.DraftInput{Mode: "generator", Generator: &generator}, Answer: model.DraftAnswer{Mode: "provided", ObjectID: &object}}}
	_, err := repo.SaveDraft(ctx, d)
	require.NoError(t, err)
	saved, err := repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)

	saved.Library[0].Name = "testlib-0.9.h"
	saved.Generators[0].Name = "random"
	saved.Tests[0].Input.Generator = &saved.Generators[0].Name
	saved.Assets[0].Name = "graph.png"
	_, err = repo.SaveDraft(ctx, saved)
	require.NoError(t, err)
	renamed, err := repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	require.Equal(t, "random", renamed.Generators[0].Name)
	require.Equal(t, "testlib-0.9.h", renamed.Library[0].Name)
	require.Equal(t, saved.Tests, renamed.Tests)
	require.Equal(t, []model.DraftAsset{{Name: "graph.png", ObjectID: object}}, renamed.Assets)

	// Renaming a program must preserve objects still used by its tests.
	renamed.Assets = nil
	_, err = repo.SaveDraft(ctx, renamed)
	require.NoError(t, err)
	require.True(t, objectExists(t, ctx, pool, repo, object), "a draft reference keeps the object")
}
