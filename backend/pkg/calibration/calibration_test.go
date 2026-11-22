package calibration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ms = time.Millisecond

// uniform gives every profile the same runs on cpu2 and cpu3, scaled per profile.
func uniform(scale map[string]float64, times ...time.Duration) Measurements {
	m := Measurements{}
	for _, p := range Profiles {
		s := scale[p]
		if s == 0 {
			s = 1
		}
		for _, cpu := range []int{2, 3} {
			for _, t := range times {
				m[p] = append(m[p], Run{CPU: cpu, Time: time.Duration(float64(t) * s)})
			}
		}
	}
	return m
}

var reference = map[string]time.Duration{"branchy": 200 * ms, "compute": 200 * ms, "random": 200 * ms, "stream": 200 * ms}

func settings() Settings {
	return Settings{Repeats: 3, MaxSpread: 0.02, MaxDisagreement: 0.05, Reference: reference}
}

func TestEvaluate(t *testing.T) {
	k, reasons := settings().Evaluate(uniform(nil, 250*ms, 251*ms, 252*ms))
	require.Empty(t, reasons)
	assert.InDelta(t, 1.255, k, 1e-9)

	// Slower in some profiles than others, within tolerance: k is their geometric mean.
	k, reasons = settings().Evaluate(uniform(map[string]float64{"random": 1.04}, 200*ms))
	require.Empty(t, reasons)
	assert.InDelta(t, 1.00985, k, 1e-4)
}

func TestCheckRound(t *testing.T) {
	require.NoError(t, checkRound([]Run{{CPU: 3}, {CPU: 2}}, []int{2, 3}))
	require.Error(t, checkRound([]Run{{CPU: 2}, {CPU: 2}}, []int{2, 3}), "a late request reused a core")
	require.Error(t, checkRound([]Run{{CPU: 2}, {CPU: 4}}, []int{2, 3}))
}

func TestEvaluateRejects(t *testing.T) {
	noisy := uniform(nil, 200*ms, 200*ms, 210*ms)
	slowCore := uniform(nil, 200*ms)
	for i := range slowCore["compute"] {
		if slowCore["compute"][i].CPU == 3 {
			slowCore["compute"][i].Time = 210 * ms
		}
	}
	unreferenced := settings()
	unreferenced.Reference = nil
	for name, tc := range map[string]struct {
		s    Settings
		m    Measurements
		want string
	}{
		"noisy core":         {settings(), noisy, "on cpu2 varies by 5.0%"},
		"cores differ":       {settings(), slowCore, "compute differs between cores by 4.9%"},
		"no reference":       {unreferenced, uniform(nil, 200*ms), "set it to the measured {branchy: 200ms, compute: 200ms, random: 200ms, stream: 200ms}"},
		"different hardware": {settings(), uniform(map[string]float64{"stream": 1.2}, 200*ms), "profiles disagree"},
		"missing profile":    {settings(), Measurements{"compute": {{CPU: 2, Time: ms}}}, "branchy: no runs"},
	} {
		k, reasons := tc.s.Evaluate(tc.m)
		assert.Zero(t, k, name)
		if assert.NotEmpty(t, reasons, name) {
			assert.Contains(t, reasons[0], tc.want, name)
		}
	}
}
