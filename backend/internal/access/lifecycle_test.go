package access

import (
	"testing"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestContestLifecycleBoundaries(t *testing.T) {
	in, _, _, contest := policyFixture()
	start := in.Now
	end := start.Add(time.Hour)
	contest.StartTime = &start
	contest.EndTime = &end
	contest.DirectRole = m.ContestParticipant
	contest.MonitorScope = "public"
	for _, tc := range []struct {
		name               string
		now                time.Time
		statements, submit bool
	}{
		{"before start", start.Add(-time.Nanosecond), false, false},
		{"at start", start, true, true},
		{"before end", end.Add(-time.Nanosecond), true, true},
		{"at end", end, true, false},
		{"after end", end.Add(time.Nanosecond), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.Now = tc.now
			c := contest
			require.NoError(t, ApplyContestLifecycle(in, &c))
			matrixResult(t, tc.statements, RequireContest(in, c, m.ActionGetContestProblem))
			matrixResult(t, tc.submit, RequireContest(in, c, m.ActionCreateSubmission))
		})
	}
	in.Now = start.Add(-time.Nanosecond)
	contest.DirectRole = m.ContestModerator
	require.NoError(t, ApplyContestLifecycle(in, &contest))
	require.NoError(t, RequireContest(in, contest, m.ActionGetContestProblem))
	require.NoError(t, RequireContest(in, contest, m.ActionGetMonitor))
	require.ErrorIs(t, RequireContest(in, contest, m.ActionCreateSubmission), pkg.ErrForbidden)
	in.Now = end
	require.NoError(t, ApplyContestLifecycle(in, &contest))
	require.ErrorIs(t, RequireContest(in, contest, m.ActionCreateSubmission), pkg.ErrForbidden)
}

func TestContestCompositePolicyIsStrictAcrossOrganizations(t *testing.T) {
	in, _, source, destination := policyFixture()
	destination.OwnerID = in.Actor.Id
	source.Organization.ID = uuid.New()
	pack := PackageContext{ID: uuid.New(), ProblemID: source.ID}
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, pack), pkg.ErrForbidden)
	source.DirectRole = m.ProblemViewer
	require.NoError(t, RequireAttachProblem(in, destination, source, pack))
	bad := pack
	bad.ProblemID = uuid.New()
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, bad), pkg.ErrInvalid)
	destination.OwnerID = uuid.Nil
	destination.DirectRole = m.ContestParticipant
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, pack), pkg.ErrForbidden)
}
