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
		root, leaf string // cpu.max of the cgroup root and of the process's cgroup /pod/app
		allowed    func(int) bool
		want       int
	}{
		"no quota":         {"", "", all, 4},
		"unlimited":        {"max 100000\n", "max 100000\n", all, 4},
		"partial quota":    {"", "150000 100000\n", all, 2},
		"quota over cores": {"", "800000 100000\n", all, 4},
		"tiny quota":       {"", "1000 100000\n", all, 1},
		"ancestor quota":   {"200000 100000\n", "max 100000\n", all, 2},
		"pinned siblings":  {"", "", func(cpu int) bool { return cpu == 1 || cpu == 5 }, 1},
		"pinned cores":     {"", "", func(cpu int) bool { return cpu < 3 }, 3},
	} {
		cgroupfs := t.TempDir()
		leaf := filepath.Join(cgroupfs, "pod", "app")
		require.NoError(t, os.MkdirAll(leaf, 0o700))
		for dir, quota := range map[string]string{cgroupfs: tc.root, leaf: tc.leaf} {
			if quota != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "cpu.max"), []byte(quota), 0o600))
			}
		}
		got, err := physical(sysfs, cgroupfs, "/pod/app", tc.allowed)
		require.NoError(t, err, name)
		require.Equal(t, tc.want, got, name)
	}
	cgroupfs := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cgroupfs, "cpu.max"), []byte("garbage"), 0o600))
	_, err := physical(sysfs, cgroupfs, "/", all)
	require.Error(t, err)
}
