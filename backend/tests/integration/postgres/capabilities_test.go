//go:build integration

package postgres_test

import (
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCapabilityFactsRemainBatchedAndNonlocking(t *testing.T) {
	f := newContestProblemFixture(t)
	f.grantRead(t)
	pool, recorder := newRecordingPool(t)
	uc := access.NewCapabilities(postgres.NewPolicyFactsRepo(pool))
	for _, size := range []int{1, 100} {
		recorder.reset()
		requests := make([]access.CapabilityRequest, size)
		for i := range requests {
			requests[i] = access.CapabilityRequest{Scope: "contest", ID: f.contest, Kind: "member", Target: f.actor, Role: "owner"}
		}
		values, err := uc.Project(f.ctx, requests)
		require.NoError(t, err)
		require.Len(t, values, size)
		queries := recorder.recorded()
		require.Len(t, queries, 2, "one contest facts and one users batch, independent of row count")
		for _, query := range queries {
			upper := strings.ToUpper(query)
			for _, forbidden := range []string{"FOR UPDATE", "FOR SHARE", "FOR NO KEY UPDATE", "FOR KEY SHARE", "BEGIN", "PG_ADVISORY"} {
				require.NotContains(t, upper, forbidden)
			}
		}
	}
	// A persisted per-problem block removes submit from the link without exposing
	// any permissions on its inaccessible source Problem.
	_, err := f.pool.Exec(f.ctx, `INSERT INTO contest_user_problem_blocks(contest_id,user_id,problem_id) VALUES($1,$2,$3)`, f.contest, f.actor, f.problem)
	require.NoError(t, err)
	values, err := uc.Project(f.ctx, []access.CapabilityRequest{{Scope: "contest", ID: f.contest, Kind: "problem", Target: f.problem}})
	require.NoError(t, err)
	require.NotContains(t, values[0], "submit")
	require.Contains(t, values[0], "view_statements")
	require.NotContains(t, values[0], "use")
	// Resource facts do not accidentally grant rights to a different, missing row.
	_, err = uc.Project(f.ctx, []access.CapabilityRequest{{Scope: "contest", ID: uuid.New()}})
	require.Error(t, err)
}
