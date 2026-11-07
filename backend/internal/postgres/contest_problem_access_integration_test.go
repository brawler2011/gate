//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type contestProblemFixture struct {
	pool                                                *pgxpool.Pool
	ctx                                                 context.Context
	actor, sourceOrg, problem, other, contest, original uuid.UUID
	repo                                                *ContestsRepo
	permissions                                         *access.PermissionsUseCase
	uc                                                  *contests.ContestsUseCase
}

func newContestProblemFixture(t *testing.T) *contestProblemFixture {
	t.Helper()
	f := &contestProblemFixture{pool: getSharedTestPool(t), actor: uuid.New(), problem: uuid.New(), other: uuid.New(), contest: uuid.New()}
	f.ctx = accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.actor, Role: accessmodel.UserRoleUser})
	_, err := f.pool.Exec(f.ctx, `INSERT INTO users(id, username, password_hash) VALUES ($1,$2,'unused')`, f.actor, uuid.NewString())
	require.NoError(t, err)
	destination := helperCreateOrg(t, f.ctx, f.pool, uuid.NewString(), "Destination")
	f.sourceOrg = helperCreateOrg(t, f.ctx, f.pool, uuid.NewString(), "Source")
	f.repo = NewContestsRepo(f.pool)
	require.NoError(t, f.repo.CreateContest(f.ctx, &contestsmodel.CreateContestParams{ID: f.contest, OrganizationID: destination, OwnerID: &f.actor, Title: "Attachment", Login: uuid.NewString(), Visibility: contestsmodel.ContestVisibilityPublic}))
	for _, problem := range []uuid.UUID{f.problem, f.other} {
		_, err = f.pool.Exec(f.ctx, `INSERT INTO problems(id,organization_id,title,short_name,visibility) VALUES ($1,$2,'Source problem',$3,'private')`, problem, f.sourceOrg, uuid.NewString())
		require.NoError(t, err)
	}
	f.original = f.pack(t, f.problem, "ready", time.Now().Add(-time.Hour))
	f.permissions = access.NewPermissionsUseCase(f.repo, NewUsersRepo(f.pool), NewProblemsRepo(f.pool), NewTeamsRepo(f.pool), NewOrganizationsRepo(f.pool))
	f.uc = contests.NewContestsUseCase(f.repo, NewOrganizationsRepo(f.pool), NewContestCommandTransactions(f.pool), f.permissions)
	return f
}

func (f *contestProblemFixture) pack(t *testing.T, problem uuid.UUID, status string, created time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	hash := strings.ReplaceAll(id.String(), "-", "")
	_, err := f.pool.Exec(f.ctx, `INSERT INTO problem_packages(id,problem_id,organization_id,package_hash,status,version,created_at)
 VALUES ($1,$2,$3,$4,$5,(SELECT COALESCE(MAX(version),0)+1 FROM problem_packages WHERE problem_id=$2),$6)`, id, problem, f.sourceOrg, hash+hash, status, created)
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
	for _, operation := range []string{"create", "repeat create", "update"} {
		t.Run(operation, func(t *testing.T) {
			f := newContestProblemFixture(t)
			if operation != "create" {
				f.grantRead(t)
				require.NoError(t, f.create(f.original))
				_, err := f.pool.Exec(f.ctx, `DELETE FROM problem_members WHERE problem_id=$1 AND user_id=$2`, f.problem, f.actor)
				require.NoError(t, err)
			}
			candidate := f.pack(t, f.problem, "ready", time.Now())
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
	for _, operation := range []string{"create", "repeat create", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, kind := range []string{"foreign ready", "pending", "building", "failed", "missing"} {
				t.Run(kind, func(t *testing.T) {
					f := newContestProblemFixture(t)
					f.grantRead(t)
					if operation != "create" {
						require.NoError(t, f.create(f.original))
					}
					candidate := uuid.New()
					if kind == "foreign ready" {
						candidate = f.pack(t, f.other, "ready", time.Now())
					} else if kind != "missing" {
						candidate = f.pack(t, f.problem, kind, time.Now())
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
			newer := f.pack(t, f.problem, "ready", time.Now().Add(-time.Minute))
			f.pack(t, f.problem, "pending", time.Now())
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

func TestContestProblemRequiresReadyPackageForAutomaticSelection(t *testing.T) {
	f := newContestProblemFixture(t)
	f.grantRead(t)
	_, err := f.pool.Exec(f.ctx, `UPDATE problem_packages SET status='failed' WHERE id=$1`, f.original)
	require.NoError(t, err)
	require.ErrorIs(t, f.create(uuid.Nil), pkg.ErrBadInput)
	_, err = f.repo.GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: f.problem})
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

type pinnedContestStatement struct{ problem, pack uuid.UUID }

func (s *pinnedContestStatement) ReadStatement(_ context.Context, problem, pack uuid.UUID, _ string) (*problemsmodel.Statement, []problemsmodel.Sample) {
	s.problem, s.pack = problem, pack
	return &problemsmodel.Statement{Legend: "Pinned statement"}, nil
}

func TestContestProblemReadUsesContestAccessAndPinnedPackage(t *testing.T) {
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
	reader := contests.NewDetailsReader(f.repo, NewProblemsRepo(f.pool), statements, f.permissions, nil, nil)
	details, err := reader.GetContestProblemDetails(visitor, contest.OrganizationLogin, contest.Login, f.problem)
	require.NoError(t, err)
	require.Equal(t, "Pinned statement", details.Statement.Legend)
	require.Equal(t, f.problem, statements.problem)
	require.Equal(t, f.original, statements.pack)
}

func TestContestProblemCommandsWorkWithSingleConnectionPool(t *testing.T) {
	f := newContestProblemFixture(t)
	f.grantRead(t)
	config := f.pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(f.ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	repo := NewContestsRepo(pool)
	permissions := access.NewPermissionsUseCase(repo, NewUsersRepo(pool), NewProblemsRepo(pool), NewTeamsRepo(pool), NewOrganizationsRepo(pool))
	uc := contests.NewContestsUseCase(repo, NewOrganizationsRepo(pool), NewContestCommandTransactions(pool), permissions)
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, uc.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: f.original}))
	newer := f.pack(t, f.problem, "ready", time.Now())
	require.NoError(t, uc.UpdateContestProblemPackage(ctx, f.contest, f.problem, newer))
	require.Equal(t, newer, f.pinned(t))
}
