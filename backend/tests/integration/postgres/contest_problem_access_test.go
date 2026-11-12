//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/postgres"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type contestProblemFixture struct {
	pool                                                *pgxpool.Pool
	ctx                                                 context.Context
	actor, sourceOrg, problem, other, contest, original uuid.UUID
	repo                                                *postgres.ContestsRepo
	permissions                                         *access.MetadataPermissions
	uc                                                  *contests.ContestsUseCase
}

func newContestProblemFixture(t *testing.T) *contestProblemFixture {
	t.Helper()
	f := &contestProblemFixture{pool: getTestPool(t), actor: uuid.New(), problem: uuid.New(), other: uuid.New(), contest: uuid.New()}
	f.ctx = accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.actor, Role: accessmodel.UserRoleUser})
	_, err := f.pool.Exec(f.ctx, `INSERT INTO users(id, username, password_hash) VALUES ($1,$2,'unused')`, f.actor, uuid.NewString())
	require.NoError(t, err)
	destination := helperCreateOrg(t, f.ctx, f.pool, uuid.NewString(), "Destination")
	f.sourceOrg = helperCreateOrg(t, f.ctx, f.pool, uuid.NewString(), "Source")
	f.repo = postgres.NewContestsRepo(f.pool)
	require.NoError(t, f.repo.CreateContest(f.ctx, &contestsmodel.CreateContestParams{ID: f.contest, OrganizationID: destination, OwnerID: &f.actor, Title: "Attachment", Login: uuid.NewString(), Visibility: contestsmodel.ContestVisibilityPublic}))
	for _, problem := range []uuid.UUID{f.problem, f.other} {
		err = dbfixtures.InsertProblem(f.ctx, f.pool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: f.sourceOrg, Title: "Source problem", ShortName: uuid.NewString()})
		require.NoError(t, err)
	}
	f.original = f.pack(t, f.problem, time.Now().Add(-time.Hour))
	f.permissions = access.NewMetadataPermissions(access.NewPermissionsUseCase(f.repo, postgres.NewUsersRepo(f.pool), postgres.NewProblemsRepo(f.pool), postgres.NewTeamsRepo(f.pool), postgres.NewOrganizationsRepo(f.pool)), postgres.NewPolicyFactsRepo(f.pool))
	f.uc = contests.NewContestsUseCase(f.repo, postgres.NewOrganizationsRepo(f.pool), postgres.NewContestCommandTransactions(f.pool), f.permissions)
	return f
}

func (f *contestProblemFixture) pack(t *testing.T, problem uuid.UUID, created time.Time) uuid.UUID {
	t.Helper()
	id, err := dbfixtures.InsertPackage(f.ctx, f.pool, problem, f.sourceOrg, created)
	require.NoError(t, err)
	return id
}

func (f *contestProblemFixture) grantRead(t *testing.T) {
	t.Helper()
	_, err := f.pool.Exec(f.ctx, `INSERT INTO problem_members(problem_id,user_id,role) VALUES ($1,$2,'viewer')`, f.problem, f.actor)
	require.NoError(t, err)
}

func (f *contestProblemFixture) create(pack uuid.UUID) error {
	return f.uc.CreateContestProblem(f.ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: pack})
}

func (f *contestProblemFixture) pinned(t *testing.T) uuid.UUID {
	t.Helper()
	link, err := f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
	require.NoError(t, err)
	return link.PackageID
}

func TestContestProblemRejectsUnauthorizedSource(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"create", "repeat create", "update"} {
		t.Run(operation, func(t *testing.T) {
			f := newContestProblemFixture(t)
			if operation != "create" {
				f.grantRead(t)
				require.NoError(t, f.create(f.original))
				_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_members WHERE problem_id=$1 AND user_id=$2`, f.problem, f.actor)
				require.NoError(t, err)
			}
			candidate := f.pack(t, f.problem, time.Now())
			var err error
			if operation == "update" {
				err = f.uc.UpdateContestProblemPackage(f.ctx, f.contest, f.problem, candidate)
			} else {
				err = f.create(candidate)
			}
			require.ErrorIs(t, err, pkg.NoPermission)
			if operation == "create" {
				_, err = f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
				require.ErrorIs(t, err, pkg.ErrNotFound)
			} else {
				require.Equal(t, f.original, f.pinned(t))
			}
		})
	}
}

func TestContestProblemValidatesExplicitPackage(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"create", "repeat create", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, kind := range []string{"foreign package", "missing"} {
				t.Run(kind, func(t *testing.T) {
					f := newContestProblemFixture(t)
					f.grantRead(t)
					if operation != "create" {
						require.NoError(t, f.create(f.original))
					}
					candidate := uuid.New()
					if kind == "foreign package" {
						candidate = f.pack(t, f.other, time.Now())
					}
					var err error
					if operation == "update" {
						err = f.uc.UpdateContestProblemPackage(f.ctx, f.contest, f.problem, candidate)
					} else {
						err = f.create(candidate)
					}
					require.ErrorIs(t, err, pkg.ErrBadInput)
					if operation == "create" {
						_, err = f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
						require.ErrorIs(t, err, pkg.ErrNotFound)
					} else {
						require.Equal(t, f.original, f.pinned(t))
					}
				})
			}
		})
	}
}

func TestContestProblemAcceptsReadableSourcesAndPreservesOrdinal(t *testing.T) {
	t.Parallel()
	for _, public := range []bool{false, true} {
		name := "private read grant"
		if public {
			name = "public source"
		}
		t.Run(name, func(t *testing.T) {
			f := newContestProblemFixture(t)
			if public {
				_, err := f.pool.Exec(f.ctx, `UPDATE problems SET visibility='public' WHERE id=$1`, f.problem)
				require.NoError(t, err)
			} else {
				f.grantRead(t)
			}
			require.NoError(t, f.create(f.original))
			require.Equal(t, f.original, f.pinned(t))
			newer := f.pack(t, f.problem, time.Now().Add(-time.Minute))
			require.NoError(t, f.create(uuid.Nil))
			require.Equal(t, newer, f.pinned(t))
			problems, err := f.repo.GetContestProblems(f.ctx, f.contest)
			require.NoError(t, err)
			require.Len(t, problems, 1)
			require.Equal(t, 1, problems[0].Ordinal)
			require.NoError(t, f.uc.UpdateContestProblemPackage(f.ctx, f.contest, f.problem, f.original))
			require.Equal(t, f.original, f.pinned(t))
			require.ErrorIs(t, f.uc.UpdateContestProblemPackage(f.ctx, f.contest, f.problem, uuid.Nil), pkg.ErrBadInput)
			require.Equal(t, f.original, f.pinned(t))
		})
	}
}

func TestContestProblemAutomaticSelectionRequiresAPackage(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	// f.other has never been built, so there is nothing to pin.
	_, err := f.pool.Exec(f.ctx, `INSERT INTO problem_members(problem_id,user_id,role) VALUES ($1,$2,'viewer')`, f.other, f.actor)
	require.NoError(t, err)
	err = f.uc.CreateContestProblem(f.ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.other, PackageId: uuid.Nil})
	require.ErrorIs(t, err, pkg.ErrBadInput)
	_, err = f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.other})
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

type pinnedContestStatement struct{ problem, pack uuid.UUID }

func (s *pinnedContestStatement) ReadStatement(_ context.Context, problem, pack uuid.UUID, _ string) (*problemsmodel.Statement, []problemsmodel.Sample) {
	s.problem, s.pack = problem, pack
	return &problemsmodel.Statement{Legend: "Pinned statement"}, nil
}

func TestContestProblemReadUsesContestAccessAndPinnedPackage(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	require.NoError(t, f.create(f.original))
	_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_members WHERE problem_id=$1 AND user_id=$2`, f.problem, f.actor)
	require.NoError(t, err)
	// This visitor has no source-problem permission; the public contest grants the read.
	visitor := accessmodel.WithUser(context.Background(), accessmodel.Guest)
	allowed, err := f.permissions.HasProblemPermission(visitor, f.problem, uuid.Nil, accessmodel.ActionViewProblem)
	require.NoError(t, err)
	require.False(t, allowed)
	contest, err := f.repo.GetContest(f.ctx, f.contest)
	require.NoError(t, err)
	statements := &pinnedContestStatement{}
	reader := contests.NewDetailsReader(f.repo, statements, f.permissions, nil, nil)
	details, err := reader.GetContestProblemDetails(visitor, contest.OrganizationLogin, contest.Login, f.problem)
	require.NoError(t, err)
	require.Equal(t, "Pinned statement", details.Statement.Legend)
	require.Equal(t, f.problem, statements.problem)
	require.Equal(t, f.original, statements.pack)
}

func TestContestProblemCommandsWorkWithSingleConnectionPool(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	config := f.pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(f.ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	repo := postgres.NewContestsRepo(pool)
	permissions := access.NewPermissionsUseCase(repo, postgres.NewUsersRepo(pool), postgres.NewProblemsRepo(pool), postgres.NewTeamsRepo(pool), postgres.NewOrganizationsRepo(pool))
	uc := contests.NewContestsUseCase(repo, postgres.NewOrganizationsRepo(pool), postgres.NewContestCommandTransactions(pool), permissions)
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, uc.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: f.original}))
	newer := f.pack(t, f.problem, time.Now())
	require.NoError(t, uc.UpdateContestProblemPackage(ctx, f.contest, f.problem, newer))
	require.Equal(t, newer, f.pinned(t))
}

func TestContestSnapshotSurvivesSourceChanges(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	require.NoError(t, f.create(f.original))
	before, err := f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, `WITH d AS (UPDATE problem_drafts SET title='Changed source',time_limit_ms=4321,memory_limit_mb=768 WHERE problem_id=$1) UPDATE problems SET visibility='public',owner_id=$2 WHERE id=$1`, f.problem, f.actor)
	require.NoError(t, err)
	newer := f.pack(t, f.problem, time.Now())
	require.NotEqual(t, f.original, newer)
	after, err := f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
	require.NoError(t, err)
	require.Equal(t, before, after)
	// No source grant remains. A contest participant still reads the old snapshot.
	_, err = f.pool.Exec(f.ctx, `DELETE FROM problem_members WHERE problem_id=$1`, f.problem)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, `UPDATE problems SET visibility='private',owner_id=NULL WHERE id=$1`, f.problem)
	require.NoError(t, err)
	contest, err := f.repo.GetContest(f.ctx, f.contest)
	require.NoError(t, err)
	statements := &pinnedContestStatement{}
	reader := contests.NewDetailsReader(f.repo, statements, f.permissions, nil, nil)
	visitor := accessmodel.WithUser(context.Background(), accessmodel.Guest)
	details, err := reader.GetContestProblemDetails(visitor, contest.OrganizationLogin, contest.Login, f.problem)
	require.NoError(t, err)
	require.Equal(t, before.Title, details.Problem.Title)
	require.Equal(t, before.TimeLimitMs, details.Problem.TimeLimitMs)
	require.Equal(t, f.original, statements.pack)
}

func TestContestCommandGuardRejectsRevokedDecision(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	transactions := postgres.NewContestCommandTransactions(f.pool)
	decision, err := transactions.Prepare(f.ctx)
	require.NoError(t, err)
	allowed, err := f.permissions.HasProblemPermission(decision, f.problem, f.actor, accessmodel.ActionUseProblem)
	require.NoError(t, err)
	require.True(t, allowed)
	_, err = f.pool.Exec(f.ctx, `DELETE FROM problem_members WHERE problem_id=$1 AND user_id=$2`, f.problem, f.actor)
	require.NoError(t, err)
	wrote := false
	err = transactions.Within(decision, func(contests.CommandTxStore) error { wrote = true; return nil })
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.False(t, wrote)
	require.ErrorIs(t, f.create(f.original), pkg.NoPermission)
}

func TestContestLifecyclePostgresBoundaries(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	start := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	end := start.Add(2 * time.Hour)
	freeze := end.Add(-30 * time.Minute)
	_, err := f.pool.Exec(f.ctx, `UPDATE contests SET start_time=$2,end_time=$3,settings='{"monitor_scope":"public","freeze_duration_minutes":30}' WHERE id=$1`, f.contest, start, end)
	require.NoError(t, err)
	visitor := uuid.New()
	_, err = f.pool.Exec(f.ctx, `INSERT INTO users(id,username,password_hash) VALUES($1,$2,'unused')`, visitor, uuid.NewString())
	require.NoError(t, err)
	for _, tc := range []struct {
		name         string
		now          time.Time
		read, submit bool
	}{
		{"pre-start", start.Add(-time.Nanosecond), false, false},
		{"start equality", start, true, true},
		{"freeze equality", freeze, true, true},
		{"end equality", end, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := accessmodel.WithReadTime(f.ctx, tc.now)
			allowed, err := f.permissions.HasContestPermission(ctx, f.contest, visitor, accessmodel.ActionGetContestProblem)
			require.NoError(t, err)
			require.Equal(t, tc.read, allowed)
			allowed, err = f.permissions.HasContestPermission(ctx, f.contest, visitor, accessmodel.ActionCreateSubmission)
			require.NoError(t, err)
			require.Equal(t, tc.submit, allowed)
			allowed, err = f.permissions.HasContestPermission(ctx, f.contest, f.actor, accessmodel.ActionCreateSubmission)
			require.NoError(t, err)
			require.Equal(t, tc.submit, allowed)
		})
	}
}

func TestContestCommandDeadlineRollsBackMutation(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	txs := postgres.NewContestCommandTransactions(f.pool)
	ctx, err := txs.Prepare(f.ctx)
	require.NoError(t, err)
	expired := time.Now().Add(-time.Second)
	ctx = accessmodel.WithDecisionDeadline(ctx, &expired)
	err = txs.Within(ctx, func(tx contests.CommandTxStore) error {
		return tx.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: f.original})
	})
	require.ErrorIs(t, err, pkg.NoPermission)
	_, err = f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
	require.ErrorIs(t, err, pkg.ErrNotFound)
}
