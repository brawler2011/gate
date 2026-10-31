package models_test

import (
	"math"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/stretchr/testify/require"
)

func TestContestFreezeDuration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  *int32
	}{
		{"absent", nil, nil}, {"string", "60", nil}, {"zero", 0, nil}, {"negative", -1, nil},
		{"overflow", float64(math.MaxInt64), nil}, {"underflow", int64(math.MinInt64), nil},
		{"json number", float64(45), int32Pointer(45)}, {"int", int(45), int32Pointer(45)},
		{"int32", int32(45), int32Pointer(45)}, {"int64", int64(45), int32Pointer(45)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := models.Contest{Settings: map[string]interface{}{"freeze_duration_minutes": tc.value}}
			require.Equal(t, tc.want, c.GetFreezeDurationMinutes())
		})
	}
	require.Nil(t, (&models.Contest{}).GetFreezeDurationMinutes())
}

func TestContestFreezeStatus(t *testing.T) {
	for _, tc := range []struct {
		value interface{}
		want  string
	}{
		{nil, models.FreezeStatusAuto}, {"invalid", models.FreezeStatusAuto},
		{models.FreezeStatusAuto, models.FreezeStatusAuto},
		{models.FreezeStatusFrozen, models.FreezeStatusFrozen},
		{models.FreezeStatusUnfrozen, models.FreezeStatusUnfrozen},
	} {
		c := models.Contest{Settings: map[string]interface{}{"freeze_status": tc.value}}
		require.Equal(t, tc.want, c.GetFreezeStatus())
	}
	require.Equal(t, models.FreezeStatusAuto, (&models.Contest{}).GetFreezeStatus())
}

func TestContestFreezeTime(t *testing.T) {
	end := time.Date(2020, 1, 1, 17, 0, 0, 0, time.UTC)
	cutoff := end.Add(-time.Hour)
	for _, tc := range []struct {
		name     string
		end      *time.Time
		duration interface{}
		want     *time.Time
	}{
		{"one hour before end", &end, 60, &cutoff},
		{"no end", nil, 60, nil}, {"no duration", &end, nil, nil},
		{"zero duration", &end, 0, nil}, {"negative duration", &end, -1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := models.Contest{EndTime: tc.end, Settings: map[string]interface{}{"freeze_duration_minutes": tc.duration}}
			require.Equal(t, tc.want, c.GetFreezeTime())
		})
	}
}

func TestContestFreezeBoundaries(t *testing.T) {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Hour)
	cutoff := end.Add(-time.Hour)
	for _, tc := range []struct {
		name, status string
		end          *time.Time
		duration     interface{}
		at           time.Time
		frozen       bool
	}{
		{"before cutoff", models.FreezeStatusAuto, &end, 60, cutoff.Add(-time.Nanosecond), false},
		{"at cutoff", models.FreezeStatusAuto, &end, 60, cutoff, true},
		{"after cutoff", models.FreezeStatusAuto, &end, 60, cutoff.Add(time.Nanosecond), true},
		{"at end", models.FreezeStatusAuto, &end, 60, end, true},
		{"remains frozen after end", models.FreezeStatusAuto, &end, 60, end.Add(24 * time.Hour), true},
		{"no duration", models.FreezeStatusAuto, &end, nil, end, false},
		{"zero duration", models.FreezeStatusAuto, &end, 0, end, false},
		{"no end", models.FreezeStatusAuto, nil, 60, end, false},
		{"manual freeze before start", models.FreezeStatusFrozen, &end, 0, start.Add(-time.Hour), true},
		{"manual freeze without duration", models.FreezeStatusFrozen, &end, nil, start, true},
		{"manual thaw during freeze", models.FreezeStatusUnfrozen, &end, 60, cutoff, false},
		{"manual thaw after end", models.FreezeStatusUnfrozen, &end, 60, end.Add(time.Hour), false},
		{"duration exceeds contest", models.FreezeStatusAuto, &end, 400, start, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := models.Contest{StartTime: &start, EndTime: tc.end, Settings: map[string]interface{}{"freeze_status": tc.status, "freeze_duration_minutes": tc.duration}}
			require.Equal(t, tc.frozen, c.IsFrozenAt(tc.at))
		})
	}
}

func int32Pointer(value int32) *int32 { return &value }
