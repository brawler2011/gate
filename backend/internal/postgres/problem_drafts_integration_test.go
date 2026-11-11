//go:build integration

package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"strings"
	"sync"
	"testing"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func draftFixture(t *testing.T) (context.Context, *pgxpool.Pool, *ProblemDraftsRepo, uuid.UUID, model.Draft) {
	t.Helper()
	ctx := context.Background()
	pool := getSharedTestPool(t)
	org := helperCreateOrg(t, ctx, pool, "draft-"+uuid.NewString(), "Drafts")
	id := uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: id, OrganizationID: org, Title: "Initial", ShortName: "draft"}))
	repo := NewProblemDraftsRepo(pool, model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10}, time.Minute)
	d, err := repo.GetDraft(ctx, id)
	require.NoError(t, err)
	return ctx, pool, repo, org, d
}
func mutationKey(org uuid.UUID, op string, body any) model.MutationKey {
	b, _ := json.Marshal(body)
	hash := sha256.Sum256(b)
	return model.MutationKey{OrganizationID: org, Operation: op, Key: uuid.NewString(), Fingerprint: hex.EncodeToString(hash[:]), Token: uuid.New()}
}
func makeBuildableDraft(d model.Draft, oid uuid.UUID) model.Draft {
	c := uuid.New()
	g := uuid.New()
	d.Files = map[string]string{"checkers/check.cpp": "int main(){return 0;}"}
	d.Components = []model.DraftComponent{{ID: c, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"}}
	d.CheckerID = &c
	d.Subtasks = []model.DraftSubtask{{ID: g, Ordinal: 1, Name: "All", Policy: "complete"}}
	d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: g, Input: model.DraftInput{Mode: "manual", ObjectID: &oid}, Answer: model.DraftAnswer{Mode: "provided", ObjectID: &oid}}}
	return d
}
func insertAvailableObject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size,state,upload_deadline,unreferenced_at)
 VALUES($1,$2,'test-input',$4,$3,0,'uploading',clock_timestamp()+interval '1 hour',clock_timestamp());`, id, org, strings.Repeat("0", 64), "objects/"+org.String()+"/"+id.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE stored_objects SET state='available' WHERE id=$1`, id)
	require.NoError(t, err)
	return id
}
func TestProblemDraftConcurrentEditsAndReplay(t *testing.T) {
	ctx, pool, repo, org, d := draftFixture(t)
	d.Files = map[string]string{"statements/en.md": "Original"}
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
			_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", candidate), candidate)
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
	current.Title = "After retry"
	key := mutationKey(org, "edit", current)
	first, err := repo.SaveDraft(ctx, key, current)
	require.NoError(t, err)
	key.Token = uuid.New() // lost HTTP response: a new executor must replay the result.
	second, err := repo.SaveDraft(ctx, key, current)
	require.NoError(t, err)
	require.JSONEq(t, string(first.Body), string(second.Body))
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM problem_drafts WHERE problem_id=$1`, d.ProblemID).Scan(&revision))
	require.Equal(t, int64(3), revision)
	key.Fingerprint = strings.Repeat("f", 64)
	_, err = repo.SaveDraft(ctx, key, current)
	require.ErrorIs(t, err, pkg.ErrConflict)
}
func TestProblemBuildSnapshotAndRecovery(t *testing.T) {
	ctx, pool, repo, org, d := draftFixture(t)
	oid := insertAvailableObject(t, ctx, pool, org)
	d = makeBuildableDraft(d, oid)
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	options := ProblemBuildOptions{Toolchain: model.Toolchain{Digest: "sha256:" + strings.Repeat("a", 64), Architecture: "amd64", ABI: "linux-gnu", Languages: map[string]sandbox.Language{"cpp": {Type: "compiler"}}}, MaxAttempts: 3, Duration: time.Minute, Lease: time.Second}
	key := mutationKey(org, "build", d.Revision)
	result, err := repo.EnqueueBuild(ctx, key, d.ProblemID, d.Revision, options)
	require.NoError(t, err)
	var body struct {
		BuildID uuid.UUID `json:"build_id"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &body))
	replay, err := repo.EnqueueBuild(ctx, key, d.ProblemID, d.Revision, options)
	require.NoError(t, err)
	require.JSONEq(t, string(result.Body), string(replay.Body))
	d.Title = "Edited after snapshot"
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	b, err := repo.ClaimBuild(ctx, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, body.BuildID, b.ID)
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
	var roots int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='input'`, b.ID).Scan(&roots))
	require.Equal(t, 1, roots)
	// Snapshot and root set cannot be corrupted even by a direct DB writer.
	_, err = pool.Exec(ctx, `UPDATE problem_builds SET snapshot=jsonb_set(snapshot,'{draft,title}','"Tampered"') WHERE id=$1`, b.ID)
	require.ErrorContains(t, err, "immutable build")
	_, err = pool.Exec(ctx, `DELETE FROM problem_build_objects WHERE build_id=$1 AND role='input'`, b.ID)
	require.ErrorContains(t, err, "immutable")
	_, err = repo.CancelBuild(ctx, mutationKey(org, "cancel", b.ID), b.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.HeartbeatBuild(ctx, next.ID, *next.LeaseToken, time.Minute), pkg.ErrConflict)
}
