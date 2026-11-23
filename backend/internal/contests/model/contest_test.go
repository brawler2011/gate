package contestsmodel_test

import (
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/stretchr/testify/require"
)

func TestContestFreezeTime(t *testing.T) {
	end := time.Date(2020, 1, 1, 17, 0, 0, 0, time.UTC)
	cutoff := end.Add(-time.Hour)
	for _, tc := range []struct {
		name     string
		end      *time.Time
		duration *int32
		want     *time.Time
	}{
		{"one hour before end", &end, new(int32(60)), &cutoff},
		{"no end", nil, new(int32(60)), nil}, {"no duration", &end, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := contestsmodel.Contest{EndTime: tc.end, Settings: contestsmodel.ContestSettings{FreezeStatus: contestsmodel.FreezeStatusAuto, FreezeDurationMinutes: tc.duration}}
			require.Equal(t, tc.want, c.GetFreezeTime())
		})
	}
}

func TestContestFreezeBoundaries(t *testing.T) {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Hour)
	cutoff := end.Add(-time.Hour)
	hour := new(int32(60))
	for _, tc := range []struct {
		name     string
		status   contestsmodel.FreezeStatus
		end      *time.Time
		duration *int32
		at       time.Time
		frozen   bool
	}{
		{"before cutoff", contestsmodel.FreezeStatusAuto, &end, hour, cutoff.Add(-time.Nanosecond), false},
		{"at cutoff", contestsmodel.FreezeStatusAuto, &end, hour, cutoff, true},
		{"after cutoff", contestsmodel.FreezeStatusAuto, &end, hour, cutoff.Add(time.Nanosecond), true},
		{"at end", contestsmodel.FreezeStatusAuto, &end, hour, end, false},
		{"unfreezes after end", contestsmodel.FreezeStatusAuto, &end, hour, end.Add(24 * time.Hour), false},
		{"no duration", contestsmodel.FreezeStatusAuto, &end, nil, end, false},
		{"no end", contestsmodel.FreezeStatusAuto, nil, hour, end, false},
		{"manual freeze before start", contestsmodel.FreezeStatusFrozen, &end, nil, start.Add(-time.Hour), false},
		{"manual freeze without duration", contestsmodel.FreezeStatusFrozen, &end, nil, start, true},
		{"manual thaw during freeze", contestsmodel.FreezeStatusUnfrozen, &end, hour, cutoff, false},
		{"manual thaw after end", contestsmodel.FreezeStatusUnfrozen, &end, hour, end.Add(time.Hour), false},
		{"duration exceeds contest", contestsmodel.FreezeStatusAuto, &end, new(int32(400)), start, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := contestsmodel.Contest{StartTime: &start, EndTime: tc.end, Settings: contestsmodel.ContestSettings{FreezeStatus: tc.status, FreezeDurationMinutes: tc.duration}}
			require.Equal(t, tc.frozen, c.IsFrozenAt(tc.at))
		})
	}
}

func TestContestFreezeLastsUntilUnfreezeTime(t *testing.T) {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	end, thaw := start.Add(5*time.Hour), start.Add(7*time.Hour)
	c := contestsmodel.Contest{StartTime: &start, EndTime: &end, Settings: contestsmodel.ContestSettings{
		FreezeStatus: contestsmodel.FreezeStatusAuto, FreezeDurationMinutes: new(int32(60)), UnfreezeTime: &thaw}}
	require.True(t, c.IsFrozenAt(end))
	require.True(t, c.IsFrozenAt(thaw.Add(-time.Nanosecond)))
	require.False(t, c.IsFrozenAt(thaw))
}
