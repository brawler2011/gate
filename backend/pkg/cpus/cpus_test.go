package cpus

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhysicalCountsAllowedCoresWithinQuota(t *testing.T) {
	sysfs := t.TempDir()
	// Four cores with two SMT threads each: CPU n and n+4 share a core.
	for cpu := range 8 {
		dir := filepath.Join(sysfs, "cpu"+strconv.Itoa(cpu), "topology")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		core := cpu % 4
		require.NoError(t, os.WriteFile(filepath.Join(dir, "core_cpus_list"), []byte(strconv.Itoa(core)+","+strconv.Itoa(core+4)+"\n"), 0o600))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "cpufreq"), 0o700))
	all := func(int) bool { return true }
	for name, tc := range map[string]struct {
		quota   string
		allowed func(int) bool
		want    int
	}{
		"no cgroup":        {"", all, 4},
		"unlimited":        {"max 100000\n", all, 4},
		"partial quota":    {"150000 100000\n", all, 2},
		"quota over cores": {"800000 100000\n", all, 4},
		"tiny quota":       {"1000 100000\n", all, 1},
		"pinned siblings":  {"", func(cpu int) bool { return cpu == 1 || cpu == 5 }, 1},
		"pinned cores":     {"", func(cpu int) bool { return cpu < 3 }, 3},
	} {
		quota := filepath.Join(t.TempDir(), "cpu.max")
		if tc.quota != "" {
			require.NoError(t, os.WriteFile(quota, []byte(tc.quota), 0o600))
		}
		got, err := physical(sysfs, quota, tc.allowed)
		require.NoError(t, err, name)
		require.Equal(t, tc.want, got, name)
	}
	quota := filepath.Join(t.TempDir(), "cpu.max")
	require.NoError(t, os.WriteFile(quota, []byte("garbage"), 0o600))
	_, err := physical(sysfs, quota, all)
	require.Error(t, err)
}
