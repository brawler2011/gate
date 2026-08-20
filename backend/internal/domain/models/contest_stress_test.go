package models_test

import (
	"math"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContest_FreezeBoundaryAndStress(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 19, 17, 0, 0, 0, time.UTC) // 5 hour contest
	freezeDuration := int32(60)                              // 1 hour freeze
	freezeTime := endTime.Add(-60 * time.Minute)             // 16:00:00

	t.Run("Auto Mode Exact Nanosecond Boundaries", func(t *testing.T) {
		t.Parallel()
		c := models.Contest{
			StartTime: &startTime,
			EndTime:   &endTime,
			Settings: map[string]interface{}{
				"freeze_status":           models.FreezeStatusAuto,
				"freeze_duration_minutes": freezeDuration,
			},
		}

		ft := c.GetFreezeTime()
		require.NotNil(t, ft)
		assert.Equal(t, freezeTime, *ft)

		// Nanosecond before freeze cutoff -> NOT frozen
		assert.False(t, c.IsFrozenAt(freezeTime.Add(-1*time.Nanosecond)))
		// Exact cutoff timestamp -> FROZEN
		assert.True(t, c.IsFrozenAt(freezeTime))
		// Nanosecond after cutoff -> FROZEN
		assert.True(t, c.IsFrozenAt(freezeTime.Add(1*time.Nanosecond)))
		// Mid-freeze window -> FROZEN
		assert.True(t, c.IsFrozenAt(freezeTime.Add(30*time.Minute)))
		// Exact end time -> FROZEN
		assert.True(t, c.IsFrozenAt(endTime))
		// Nanosecond after end time -> FROZEN (stays frozen post-contest)
		assert.True(t, c.IsFrozenAt(endTime.Add(1*time.Nanosecond)))
		// 1 day after end time -> FROZEN
		assert.True(t, c.IsFrozenAt(endTime.Add(24*time.Hour)))
		// 1 year after end time -> FROZEN
		assert.True(t, c.IsFrozenAt(endTime.Add(365*24*time.Hour)))
	})

	t.Run("Manual Frozen Override Modes", func(t *testing.T) {
		t.Parallel()

		// Manual frozen with 0 duration
		cFrozenZeroDur := models.Contest{
			StartTime: &startTime,
			EndTime:   &endTime,
			Settings: map[string]interface{}{
				"freeze_status":           models.FreezeStatusFrozen,
				"freeze_duration_minutes": 0,
			},
		}
		assert.Nil(t, cFrozenZeroDur.GetFreezeTime())
		assert.True(t, cFrozenZeroDur.IsFrozenAt(startTime.Add(-24*time.Hour)))
		assert.True(t, cFrozenZeroDur.IsFrozenAt(startTime))
		assert.True(t, cFrozenZeroDur.IsFrozenAt(startTime.Add(10*time.Minute)))
		assert.True(t, cFrozenZeroDur.IsFrozenAt(endTime.Add(24*time.Hour)))

		// Manual frozen with nil duration
		cFrozenNilDur := models.Contest{
			StartTime: &startTime,
			EndTime:   &endTime,
			Settings: map[string]interface{}{
				"freeze_status": models.FreezeStatusFrozen,
			},
		}
		assert.Nil(t, cFrozenNilDur.GetFreezeTime())
		assert.True(t, cFrozenNilDur.IsFrozenAt(startTime))
		assert.True(t, cFrozenNilDur.IsFrozenAt(endTime))
	})

	t.Run("Manual Unfrozen Override Modes", func(t *testing.T) {
		t.Parallel()

		// Unfrozen mode overrides timer even during freeze window and after end time
		cUnfrozen := models.Contest{
			StartTime: &startTime,
			EndTime:   &endTime,
			Settings: map[string]interface{}{
				"freeze_status":           models.FreezeStatusUnfrozen,
				"freeze_duration_minutes": 60,
			},
		}
		assert.False(t, cUnfrozen.IsFrozenAt(startTime))
		assert.False(t, cUnfrozen.IsFrozenAt(freezeTime.Add(-1*time.Minute)))
		assert.False(t, cUnfrozen.IsFrozenAt(freezeTime))
		assert.False(t, cUnfrozen.IsFrozenAt(freezeTime.Add(30*time.Minute)))
		assert.False(t, cUnfrozen.IsFrozenAt(endTime))
		assert.False(t, cUnfrozen.IsFrozenAt(endTime.Add(10*time.Hour)))
	})

	t.Run("Auto Mode With Duration Exceeding Contest Duration", func(t *testing.T) {
		t.Parallel()

		// Contest is 5 hours (300 min), freeze duration is 400 min
		// Freeze begins 100 min before contest start -> frozen throughout contest
		cOverFreeze := models.Contest{
			StartTime: &startTime,
			EndTime:   &endTime,
			Settings: map[string]interface{}{
				"freeze_status":           models.FreezeStatusAuto,
				"freeze_duration_minutes": 400,
			},
		}
		ft := cOverFreeze.GetFreezeTime()
		require.NotNil(t, ft)
		assert.True(t, ft.Before(startTime))
		assert.True(t, cOverFreeze.IsFrozenAt(startTime))
		assert.True(t, cOverFreeze.IsFrozenAt(startTime.Add(1*time.Hour)))
		assert.True(t, cOverFreeze.IsFrozenAt(endTime))
	})

	t.Run("Type Support and Boundary Limits for Duration", func(t *testing.T) {
		t.Parallel()

		types := []interface{}{
			int64(45),
			int32(45),
			int(45),
			float64(45),
		}
		for _, val := range types {
			c := models.Contest{
				Settings: map[string]interface{}{"freeze_duration_minutes": val},
			}
			dur := c.GetFreezeDurationMinutes()
			require.NotNil(t, dur)
			assert.Equal(t, int32(45), *dur)
		}

		// String or invalid types
		cInvalid := models.Contest{
			Settings: map[string]interface{}{"freeze_duration_minutes": "sixty"},
		}
		assert.Nil(t, cInvalid.GetFreezeDurationMinutes())

		// Extreme float overflow / underflow beyond int32
		cOverflow := models.Contest{
			Settings: map[string]interface{}{"freeze_duration_minutes": float64(math.MaxInt64)},
		}
		assert.Nil(t, cOverflow.GetFreezeDurationMinutes())
	})
}
