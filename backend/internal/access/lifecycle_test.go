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
	freeze := end.Add(-15 * time.Minute)
	contest.StartTime = &start
	contest.EndTime = &end
	contest.DirectRole = m.ContestParticipant
	contest.Settings = map[string]interface{}{"monitor_scope": "public", "freeze_duration_minutes": 15}
	for _, tc := range []struct {
		name                       string
		now                        time.Time
		statements, submit, frozen bool
	}{
		{"before start", start.Add(-time.Nanosecond), false, false, false},
		{"at start", start, true, true, false},
		{"before freeze", freeze.Add(-time.Nanosecond), true, true, false},
		{"at freeze", freeze, true, true, true},
		{"before end", end.Add(-time.Nanosecond), true, true, true},
		{"at end", end, true, false, false},
		{"after end", end.Add(time.Nanosecond), true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.Now = tc.now
			c := contest
			require.NoError(t, ApplyContestLifecycle(in, &c))
			matrixResult(t, tc.statements, RequireContest(in, c, m.ActionGetContestProblem))
			matrixResult(t, tc.submit, RequireContest(in, c, m.ActionCreateSubmission))
			domain := ContestFromFacts(c)
			require.Equal(t, tc.frozen, domain.IsFrozenAt(in.Now))
		})
	}
	in.Now = start.Add(-time.Nanosecond)
	contest.DirectRole = m.ContestModerator
	require.NoError(t, ApplyContestLifecycle(in, &contest))
	require.NoError(t, RequireContest(in, contest, m.ActionGetContestProblem))
	require.NoError(t, RequireContest(in, contest, m.ActionGetMonitor))
	require.ErrorIs(t, RequireContest(in, contest, m.ActionCreateSubmission), pkg.NoPermission)
	in.Now = end
	require.NoError(t, ApplyContestLifecycle(in, &contest))
	require.ErrorIs(t, RequireContest(in, contest, m.ActionCreateSubmission), pkg.NoPermission)
}

func TestContestCompositePolicyIsStrictAcrossOrganizations(t *testing.T) {
	in, _, source, destination := policyFixture()
	destination.OwnerID = in.Actor.Id
	source.Organization.ID = uuid.New()
	pack := PackageContext{ID: uuid.New(), ProblemID: source.ID, Status: "ready"}
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, pack), pkg.NoPermission)
	source.DirectRole = m.ProblemViewer
	require.NoError(t, RequireAttachProblem(in, destination, source, pack))
	for _, status := range []string{"pending", "building", "failed", "unknown"} {
		bad := pack
		bad.Status = status
		require.ErrorIs(t, RequireAttachProblem(in, destination, source, bad), pkg.ErrBadInput)
	}
	bad := pack
	bad.ProblemID = uuid.New()
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, bad), pkg.ErrBadInput)
	destination.OwnerID = uuid.Nil
	destination.DirectRole = m.ContestParticipant
	require.ErrorIs(t, RequireAttachProblem(in, destination, source, pack), pkg.NoPermission)
}

func TestContestSettingsFailClosed(t *testing.T) {
	for _, setting := range []string{"monitor_scope", "submissions_list_scope", "submissions_review_scope", "submission_details_scope", "hide_statements", "freeze_status", "freeze_duration_minutes"} {
		t.Run(setting, func(t *testing.T) {
			in, _, _, c := policyFixture()
			c.Settings = map[string]interface{}{setting: "unknown"}
			require.ErrorIs(t, ApplyContestLifecycle(in, &c), pkg.NoPermission)
		})
	}
}
