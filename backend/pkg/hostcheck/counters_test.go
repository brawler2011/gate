package hostcheck

import (
	"fmt"
	"log/slog"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// counterHost is cpu2 of an AMD host with CPPC, fixed at its 2 GHz nominal frequency.
func counterHost(t *testing.T, files map[string]string) Host {
	t.Helper()
	base := map[string]string{
		"proc/stat": "cpu  1 1 1 1 1 1 1 0 0 0\ncpu2 10 0 10 1000 0 0 0 0 0 0\n",
		"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs":  "ref:1000000000 del:1000000000",
		"sys/devices/system/cpu/cpu2/acpi_cppc/nominal_freq":   "2000",
		"sys/devices/system/cpu/cpu2/acpi_cppc/nominal_perf":   "73",
		"sys/devices/system/cpu/cpu2/acpi_cppc/reference_perf": "73",
		"sys/devices/system/cpu/cpu2/cpufreq/scaling_max_freq": "2000000",
	}
	maps.Copy(base, files)
	return writeHost(t, Host{Sys: "sys", Proc: "proc"}, base)
}

func TestCountersChanged(t *testing.T) {
	before, err := counterHost(t, nil).counters([]int{2})
	require.NoError(t, err)
	for name, tc := range map[string]struct {
		files map[string]string
		want  string // a substring of the only finding; empty for none
	}{
		"steady":     {map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": "ref:3000000000 del:3010000000"}, ""},
		"idle core":  {map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": "ref:1000000500 del:1000000100"}, ""},
		"throttled":  {map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": "ref:3000000000 del:2800000000"}, "ran at 90.0%"},
		"boosted":    {map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": "ref:3000000000 del:3400000000"}, "ran at 120.0%"},
		"stolen":     {map[string]string{"proc/stat": "cpu2 10 0 10 1000 0 0 0 4 0 0\n"}, "4 jiffies stolen"},
		"lower freq": {map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": "ref:3000000000 del:1500000000", "sys/devices/system/cpu/cpu2/cpufreq/scaling_max_freq": "1500000"}, "frequency limits changed"},
	} {
		t.Run(name, func(t *testing.T) {
			after, err := counterHost(t, tc.files).counters([]int{2})
			require.NoError(t, err)
			got := after.changed(before)
			if tc.want == "" {
				assert.Empty(t, got)
			} else if assert.Len(t, got, 1) {
				assert.Contains(t, got[0], tc.want)
			}
		})
	}
}

func TestCPPC32BitWrap(t *testing.T) {
	ctrs := func(ref, del uint64) map[string]string {
		return map[string]string{"sys/devices/system/cpu/cpu2/acpi_cppc/feedback_ctrs": fmt.Sprintf("ref:%d del:%d", ref, del)}
	}
	before, err := counterHost(t, ctrs(4_294_000_000, 4_294_000_000)).counters([]int{2})
	require.NoError(t, err)
	// Both wrapped: 2,000,000 reference and 1,600,000 delivered cycles, a 20% slowdown.
	after, err := counterHost(t, ctrs(1_032_704, 632_704)).counters([]int{2})
	require.NoError(t, err)
	got := after.changed(before)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "ran at 80.0%")
	// Only reference wrapped, at full speed.
	before, err = counterHost(t, ctrs(4_294_000_000, 1_000_000_000)).counters([]int{2})
	require.NoError(t, err)
	after, err = counterHost(t, ctrs(1_032_704, 1_002_000_000)).counters([]int{2})
	require.NoError(t, err)
	assert.Empty(t, after.changed(before))
}

func TestIntelThrottleCount(t *testing.T) {
	intel := func(count string) map[string]string {
		return map[string]string{
			"sys/devices/system/cpu/cpu2/thermal_throttle/core_throttle_count":    count,
			"sys/devices/system/cpu/cpu2/thermal_throttle/package_throttle_count": "7",
		}
	}
	before, err := counterHost(t, intel("3")).counters([]int{2})
	require.NoError(t, err)
	after, err := counterHost(t, intel("5")).counters([]int{2})
	require.NoError(t, err)
	assert.Equal(t, []string{"cpu2 was thermally throttled 2 times"}, after.changed(before))
}

func TestNoThrottleSignalFails(t *testing.T) {
	h := writeHost(t, Host{Sys: "sys", Proc: "proc"}, map[string]string{"proc/stat": "cpu2 1 1 1 1 1 1 1 0\n"})
	_, err := h.counters([]int{2})
	assert.ErrorContains(t, err, "neither thermal_throttle nor acpi_cppc")
}

func TestAdmitClosesForGood(t *testing.T) {
	h := counterHost(t, nil)
	baseline, err := h.counters([]int{2})
	require.NoError(t, err)
	m := &Monitor{Host: h, Cores: []int{2}, Logger: slog.New(slog.DiscardHandler), checked: true, last: baseline}
	assert.True(t, m.Admit(t.Context()))
	m.Host = counterHost(t, map[string]string{"proc/stat": "cpu2 10 0 10 1000 0 0 0 1 0 0\n"})
	assert.False(t, m.Admit(t.Context()))
	m.Host = h
	assert.False(t, m.Admit(t.Context()), "a host that recovers stays closed until restart")
	assert.False(t, m.Ready())
}
