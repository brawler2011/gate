package models_test

import (
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContest_GetFreezeDurationMinutes(t *testing.T) {
	// Case 1: Settings is nil
	c1 := models.Contest{}
	assert.Nil(t, c1.GetFreezeDurationMinutes())

	// Case 2: Key not present
	c2 := models.Contest{Settings: map[string]interface{}{}}
	assert.Nil(t, c2.GetFreezeDurationMinutes())

	// Case 3: Key is float64 (from JSON decoding)
	c3 := models.Contest{Settings: map[string]interface{}{"freeze_duration_minutes": float64(60)}}
	dur3 := c3.GetFreezeDurationMinutes()
	require.NotNil(t, dur3)
	assert.Equal(t, int32(60), *dur3)

	// Case 4: Key is int32
	c4 := models.Contest{Settings: map[string]interface{}{"freeze_duration_minutes": int32(45)}}
	dur4 := c4.GetFreezeDurationMinutes()
	require.NotNil(t, dur4)
	assert.Equal(t, int32(45), *dur4)

	// Case 5: Key is int
	c5 := models.Contest{Settings: map[string]interface{}{"freeze_duration_minutes": 30}}
	dur5 := c5.GetFreezeDurationMinutes()
	require.NotNil(t, dur5)
	assert.Equal(t, int32(30), *dur5)

	// Case 6: Key is nil value
	c6 := models.Contest{Settings: map[string]interface{}{"freeze_duration_minutes": nil}}
	assert.Nil(t, c6.GetFreezeDurationMinutes())
}

func TestContest_GetFreezeStatus(t *testing.T) {
	// Case 1: Settings is nil -> defaults to auto
	c1 := models.Contest{}
	assert.Equal(t, models.FreezeStatusAuto, c1.GetFreezeStatus())

	// Case 2: Key not present -> defaults to auto
	c2 := models.Contest{Settings: map[string]interface{}{}}
	assert.Equal(t, models.FreezeStatusAuto, c2.GetFreezeStatus())

	// Case 3: "frozen"
	c3 := models.Contest{Settings: map[string]interface{}{"freeze_status": "frozen"}}
	assert.Equal(t, models.FreezeStatusFrozen, c3.GetFreezeStatus())

	// Case 4: "unfrozen"
	c4 := models.Contest{Settings: map[string]interface{}{"freeze_status": "unfrozen"}}
	assert.Equal(t, models.FreezeStatusUnfrozen, c4.GetFreezeStatus())

	// Case 5: "auto"
	c5 := models.Contest{Settings: map[string]interface{}{"freeze_status": "auto"}}
	assert.Equal(t, models.FreezeStatusAuto, c5.GetFreezeStatus())

	// Case 6: Unknown/invalid string -> defaults to auto
	c6 := models.Contest{Settings: map[string]interface{}{"freeze_status": "invalid_val"}}
	assert.Equal(t, models.FreezeStatusAuto, c6.GetFreezeStatus())
}

func TestContest_GetFreezeTime(t *testing.T) {
	endTime := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)

	// Case 1: Duration set and EndTime set -> EndTime - duration
	c1 := models.Contest{
		EndTime:  &endTime,
		Settings: map[string]interface{}{"freeze_duration_minutes": 60},
	}
	expectedFreezeTime := endTime.Add(-60 * time.Minute)
	ft1 := c1.GetFreezeTime()
	require.NotNil(t, ft1)
	assert.Equal(t, expectedFreezeTime, *ft1)

	// Case 2: EndTime is nil -> nil
	c2 := models.Contest{
		EndTime:  nil,
		Settings: map[string]interface{}{"freeze_duration_minutes": 60},
	}
	assert.Nil(t, c2.GetFreezeTime())

	// Case 3: Duration is 0 -> nil
	c3 := models.Contest{
		EndTime:  &endTime,
		Settings: map[string]interface{}{"freeze_duration_minutes": 0},
	}
	assert.Nil(t, c3.GetFreezeTime())

	// Case 4: Duration is negative -> nil
	c4 := models.Contest{
		EndTime:  &endTime,
		Settings: map[string]interface{}{"freeze_duration_minutes": -10},
	}
	assert.Nil(t, c4.GetFreezeTime())

	// Case 5: No duration setting -> nil
	c5 := models.Contest{
		EndTime:  &endTime,
		Settings: map[string]interface{}{},
	}
	assert.Nil(t, c5.GetFreezeTime())
}

func TestContest_IsFrozenAt(t *testing.T) {
	startTime := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	freezeTime := endTime.Add(-60 * time.Minute) // 14:00:00

	// 1. Auto mode with 60m freeze duration
	cAuto := models.Contest{
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           "auto",
			"freeze_duration_minutes": 60,
		},
	}

	// Before freeze time (13:59:59) -> false
	assert.False(t, cAuto.IsFrozenAt(freezeTime.Add(-1*time.Second)))
	// At freeze time (14:00:00) -> true
	assert.True(t, cAuto.IsFrozenAt(freezeTime))
	// During freeze window (14:30:00) -> true
	assert.True(t, cAuto.IsFrozenAt(freezeTime.Add(30*time.Minute)))
	// At contest end time (15:00:00) -> true
	assert.True(t, cAuto.IsFrozenAt(endTime))
	// After contest end time (16:00:00) -> remains true until manually unfrozen
	assert.True(t, cAuto.IsFrozenAt(endTime.Add(1*time.Hour)))

	// 2. Auto mode with 0 or missing freeze duration -> never freezes
	cAutoNoDur := models.Contest{
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status": "auto",
		},
	}
	assert.False(t, cAutoNoDur.IsFrozenAt(freezeTime))
	assert.False(t, cAutoNoDur.IsFrozenAt(endTime))
	assert.False(t, cAutoNoDur.IsFrozenAt(endTime.Add(1*time.Hour)))

	// 3. Auto mode with nil EndTime -> false
	cAutoNoEnd := models.Contest{
		StartTime: &startTime,
		EndTime:   nil,
		Settings: map[string]interface{}{
			"freeze_status":           "auto",
			"freeze_duration_minutes": 60,
		},
	}
	assert.False(t, cAutoNoEnd.IsFrozenAt(freezeTime))

	// 4. Frozen mode -> immediately true regardless of timer
	cFrozen := models.Contest{
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status": "frozen",
		},
	}
	assert.True(t, cFrozen.IsFrozenAt(startTime.Add(-1*time.Hour)))
	assert.True(t, cFrozen.IsFrozenAt(startTime))
	assert.True(t, cFrozen.IsFrozenAt(freezeTime.Add(-1*time.Hour)))
	assert.True(t, cFrozen.IsFrozenAt(freezeTime))
	assert.True(t, cFrozen.IsFrozenAt(endTime.Add(1*time.Hour)))

	// 5. Unfrozen mode -> false even past freeze time and end time
	cUnfrozen := models.Contest{
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           "unfrozen",
			"freeze_duration_minutes": 60,
		},
	}
	assert.False(t, cUnfrozen.IsFrozenAt(startTime))
	assert.False(t, cUnfrozen.IsFrozenAt(freezeTime.Add(-1*time.Second)))
	assert.False(t, cUnfrozen.IsFrozenAt(freezeTime))
	assert.False(t, cUnfrozen.IsFrozenAt(freezeTime.Add(30*time.Minute)))
	assert.False(t, cUnfrozen.IsFrozenAt(endTime))
	assert.False(t, cUnfrozen.IsFrozenAt(endTime.Add(2*time.Hour)))
}
