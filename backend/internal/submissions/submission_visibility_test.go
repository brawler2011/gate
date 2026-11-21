package submissions

import (
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSubmissionViewFreeze(t *testing.T) {
	now := time.Now()
	end := now.Add(30 * time.Minute)
	boundary := end.Add(-time.Hour)
	viewer := uuid.New()
	other := uuid.New()
	for _, test := range []struct {
		name      string
		status    contestsmodel.FreezeStatus
		created   time.Time
		author    *uuid.UUID
		moderator bool
		duration  bool
		frozen    bool
	}{
		{"before", "auto", boundary.Add(-time.Nanosecond), &other, false, true, false},
		{"boundary", "auto", boundary, &other, false, true, true},
		{"after", "auto", boundary.Add(time.Nanosecond), &other, false, true, true},
		{"own", "auto", boundary, &viewer, false, true, false},
		{"moderator", "auto", boundary, &other, true, true, false},
		{"forced without boundary", "frozen", boundary.Add(-time.Hour), &other, false, false, true},
		{"unfrozen", "unfrozen", boundary, &other, false, true, false},
		{"missing author", "auto", boundary, nil, false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			contest := contestsmodel.Contest{Settings: contestsmodel.ContestSettings{FreezeStatus: test.status, SubmissionsReviewScope: contestsmodel.ScopePublic, SubmissionDetailsScope: contestsmodel.ScopePublic}}
			if test.duration {
				contest.Settings.FreezeDurationMinutes = new(int32(60))
			}
			if test.duration {
				contest.EndTime = &end
			}
			role := accessmodel.ContestParticipant
			if test.moderator {
				role = accessmodel.ContestModerator
			}
			view := SubmissionView(viewerAccess(viewer, contest, role, now), test.author, test.created)
			require.Equal(t, test.frozen, view.Frozen)
			require.Equal(t, !test.frozen, view.Source)
			require.Equal(t, !test.frozen, view.Details)
		})
	}
	futureEnd := now.Add(2 * time.Hour)
	contest := contestsmodel.Contest{EndTime: &futureEnd, Settings: contestsmodel.ContestSettings{FreezeStatus: contestsmodel.FreezeStatusAuto, FreezeDurationMinutes: new(int32(60))}}
	require.False(t, SubmissionView(viewerAccess(viewer, contest, "", now), &other, futureEnd).Frozen)
}

func viewerAccess(viewer uuid.UUID, contest contestsmodel.Contest, role accessmodel.ContestRole, now time.Time) access.ContestAccess {
	return access.ContestAccess{Actor: accessmodel.User{Id: viewer, Role: accessmodel.UserRoleUser}, Facts: access.ContestFacts{Contest: contest, Role: role, Now: now}}
}
