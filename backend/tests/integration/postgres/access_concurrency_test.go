//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Two owners racing to remove owners must leave exactly one: each decision is checked against the state it commits on.
func TestOwnerRemovalRaceKeepsAnOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := getTestPool(t)
	org := helperCreateOrg(t, ctx, pool, "owners-"+uuid.NewString(), "Owners")
	uc := problems.NewProblemsUseCase(postgres.NewProblemsRepo(pool), postgres.NewProblemTransactions(pool), postgres.NewOrganizationsRepo(pool), postgres.NewAccess(pool), postgres.NewPackagesRepo(pool))
	for _, tc := range []struct {
		name string
		// victim returns whom owner i removes.
		victim func(owners [2]uuid.UUID, i int) uuid.UUID
		denied []error
	}{
		{"each removes the other", func(o [2]uuid.UUID, i int) uuid.UUID { return o[1-i] }, []error{pkg.ErrForbidden, pkg.ErrNotFound}},
		{"both leave", func(o [2]uuid.UUID, i int) uuid.UUID { return o[i] }, []error{pkg.ErrOwnerRequired}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 10 {
				var owners [2]uuid.UUID
				for i := range owners {
					owners[i] = uuid.New()
					_, err := pool.Exec(ctx, `INSERT INTO users(id,username,password_hash) VALUES ($1,$2,'unused')`, owners[i], uuid.NewString())
					require.NoError(t, err)
					_, err = pool.Exec(ctx, `INSERT INTO organization_members(organization_id,user_id,role) VALUES ($1,$2,'member')`, org, owners[i])
					require.NoError(t, err)
				}
				problem := uuid.New()
				require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: org, Owner: owners[0], Title: "Owners", ShortName: problem.String()}))
				_, err := pool.Exec(ctx, `INSERT INTO problem_members(problem_id,user_id,role) VALUES ($1,$2,'owner')`, problem, owners[1])
				require.NoError(t, err)

				var start, done sync.WaitGroup
				start.Add(1)
				errs := make([]error, 2)
				for i := range owners {
					done.Go(func() {
						actor := accessmodel.WithUser(ctx, accessmodel.User{Id: owners[i], Role: accessmodel.UserRoleUser})
						start.Wait()
						errs[i] = uc.RemoveProblemMember(actor, problem, tc.victim(owners, i))
					})
				}
				start.Done()
				done.Wait()

				succeeded := 0
				for _, err := range errs {
					if err == nil {
						succeeded++
						continue
					}
					require.True(t, isAny(err, tc.denied), "unexpected error %v", err)
				}
				require.Equal(t, 1, succeeded, "errors: %v", errs)
				var remaining int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM problem_members WHERE problem_id=$1 AND role='owner'`, problem).Scan(&remaining))
				require.Equal(t, 1, remaining)
			}
		})
	}
}

// A command racing an account expiry either commits before it or sees the account gone.
func TestCommandWaitsForAccountExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := getTestPool(t)
	org := helperCreateOrg(t, ctx, pool, "expiry-"+uuid.NewString(), "Expiry")
	owner, problem := helperOrgOwner(t, ctx, pool, org), uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: org, Owner: owner, Title: "Expiry", ShortName: problem.String()}))
	uc := problems.NewProblemsUseCase(postgres.NewProblemsRepo(pool), postgres.NewProblemTransactions(pool), postgres.NewOrganizationsRepo(pool), postgres.NewAccess(pool), postgres.NewPackagesRepo(pool))

	expiry, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = expiry.Rollback(ctx) }()
	_, err = expiry.Exec(ctx, `UPDATE users SET expires_at = now() - interval '1 minute' WHERE id = $1`, owner)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		public := "public"
		actor := accessmodel.WithUser(ctx, accessmodel.User{Id: owner, Role: accessmodel.UserRoleUser})
		done <- uc.UpdateProblem(actor, problem, &problemsmodel.ProblemUpdate{Visibility: &public})
	}()
	select {
	case err := <-done:
		t.Fatalf("command finished while the account change was uncommitted: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	require.NoError(t, expiry.Commit(ctx))
	require.Error(t, <-done)
	var visibility string
	require.NoError(t, pool.QueryRow(ctx, `SELECT visibility FROM problems WHERE id = $1`, problem).Scan(&visibility))
	require.Equal(t, "private", visibility)
}

func isAny(err error, targets []error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
