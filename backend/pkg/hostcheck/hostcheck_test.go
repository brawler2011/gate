package hostcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tunedHost is a 4-CPU AMD laptop on AC with cores 2-3 set up for judging.
func tunedHost(t *testing.T) (Host, map[string]string) {
	t.Helper()
	files := map[string]string{
		"sys/devices/system/cpu/smt/active":     "0",
		"sys/devices/system/cpu/cpufreq/boost":  "0",
		"sys/devices/system/cpu/isolated":       "2-3",
		"sys/devices/system/cpu/nohz_full":      "2-3",
		"sys/devices/system/node/node0/cpulist": "0-3",
		"sys/class/power_supply/BAT0/type":      "Battery",
		"sys/class/power_supply/AC0/type":       "Mains",
		"sys/class/power_supply/AC0/online":     "1",
		"proc/irq/default_smp_affinity":         "3",
	}
	for _, cpu := range []string{"0", "1", "2", "3"} {
		dir := "sys/devices/system/cpu/cpu" + cpu + "/"
		files[dir+"topology/thread_siblings_list"] = cpu
		files[dir+"cpufreq/scaling_governor"] = "performance"
		files[dir+"cpufreq/scaling_min_freq"] = "2000000"
		files[dir+"cpufreq/scaling_max_freq"] = "2000000"
	}
	return Host{Sys: "sys", Proc: "proc"}, files
}

func writeHost(t *testing.T, h Host, files map[string]string) Host {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0o600))
	}
	return Host{Sys: filepath.Join(root, h.Sys), Proc: filepath.Join(root, h.Proc)}
}

func TestSettings(t *testing.T) {
	cores := []int{2, 3}
	for name, tc := range map[string]struct {
		set  map[string]string
		drop []string
		want string // a substring of every finding; empty for none
	}{
		"tuned":                {},
		"smt":                  {set: map[string]string{"sys/devices/system/cpu/smt/active": "1"}, want: "SMT is active"},
		"sibling":              {set: map[string]string{"sys/devices/system/cpu/cpu2/topology/thread_siblings_list": "1-2"}, want: "cpu2 shares its core"},
		"boost":                {set: map[string]string{"sys/devices/system/cpu/cpufreq/boost": "1"}, want: "turbo is on"},
		"intel turbo":          {set: map[string]string{"sys/devices/system/cpu/intel_pstate/no_turbo": "0"}, want: "intel_pstate/no_turbo is 0"},
		"intel without turbo":  {set: map[string]string{"sys/devices/system/cpu/intel_pstate/no_turbo": "1", "sys/devices/system/cpu/cpufreq/boost": "1"}},
		"per-core boost":       {set: map[string]string{"sys/devices/system/cpu/cpu3/cpufreq/boost": "1"}, want: "cpu3: turbo is on"},
		"unknown turbo":        {drop: []string{"sys/devices/system/cpu/cpufreq/boost"}, want: "cannot read"},
		"governor":             {set: map[string]string{"sys/devices/system/cpu/cpu2/cpufreq/scaling_governor": "powersave"}, want: "cpu2: governor is powersave"},
		"frequency range":      {set: map[string]string{"sys/devices/system/cpu/cpu3/cpufreq/scaling_min_freq": "400000"}, want: "cpu3: frequency is not fixed"},
		"no cpufreq":           {drop: []string{"sys/devices/system/cpu/cpu2/cpufreq/scaling_max_freq"}, want: "cannot read"},
		"not isolated":         {set: map[string]string{"sys/devices/system/cpu/isolated": "3"}, want: "cpu2 is not isolated"},
		"no nohz_full":         {set: map[string]string{"sys/devices/system/cpu/nohz_full": "(null)"}, want: "is not in nohz_full"},
		"irq default":          {set: map[string]string{"proc/irq/default_smp_affinity": "00000000,00000007"}, want: "new IRQs may go to cpu2"},
		"two nodes":            {set: map[string]string{"sys/devices/system/node/node0/cpulist": "0-2", "sys/devices/system/node/node1/cpulist": "3"}, want: "span 2 NUMA nodes"},
		"battery":              {set: map[string]string{"sys/class/power_supply/AC0/online": "0"}, want: "running on battery"},
		"usb-c charger":        {set: map[string]string{"sys/class/power_supply/AC0/type": "USB"}},
		"server without power": {drop: []string{"sys/class/power_supply/BAT0/type", "sys/class/power_supply/AC0/type"}},
	} {
		t.Run(name, func(t *testing.T) {
			h, files := tunedHost(t)
			for k, v := range tc.set {
				files[k] = v
			}
			for _, k := range tc.drop {
				delete(files, k)
			}
			got := writeHost(t, h, files).Settings(cores)
			if tc.want == "" {
				assert.Empty(t, got)
				return
			}
			require.NotEmpty(t, got)
			for _, g := range got {
				assert.Contains(t, g, tc.want)
			}
		})
	}
}

func TestActivity(t *testing.T) {
	stat := func(cpu2, cpu3 string) string {
		return "cpu  1 2 3 4 5 6 7 8 0 0\ncpu2 " + cpu2 + " 0 0\ncpu3 " + cpu3 + " 0 0\nintr 1\n"
	}
	interrupts := func(irq string) string {
		return "           CPU0       CPU2       CPU3\n  0:  5  0  0  IO-APIC  timer\n 60:  1  0  " + irq + "  PCI-MSI  nvme0q3\nLOC:  9  9  9  Local timer interrupts\n"
	}
	parse := func(t *testing.T, statText, irqText string) sample {
		t.Helper()
		h := writeHost(t, Host{Sys: "sys", Proc: "proc"}, map[string]string{"proc/stat": statText, "proc/interrupts": irqText})
		s, err := h.sample()
		require.NoError(t, err)
		return s
	}
	idle := parse(t, stat("10 0 10 1000 0 0 0 0", "10 0 10 1000 0 0 0 0"), interrupts("0"))
	for name, tc := range map[string]struct {
		cpu2, irq string
		idle      bool
		want      []string
	}{
		"quiet":             {cpu2: "10 0 11 1100 0 0 0 0", irq: "0", idle: true},
		"busy":              {cpu2: "60 0 10 1050 0 0 0 0", irq: "0", idle: true, want: []string{"cpu2 is busy 50%"}},
		"busy while judged": {cpu2: "60 0 10 1050 0 0 0 0", irq: "0"},
		"steal":             {cpu2: "10 0 10 1100 0 0 0 3", irq: "0", want: []string{"cpu2: 3 jiffies stolen"}},
		"device interrupt":  {cpu2: "10 0 10 1100 0 0 0 0", irq: "4", want: []string{"IRQ 60 was delivered to cpu3"}},
	} {
		t.Run(name, func(t *testing.T) {
			after := parse(t, stat(tc.cpu2, "10 0 10 1100 0 0 0 0"), interrupts(tc.irq))
			got := compare(idle, after, []int{2, 3}, tc.idle)
			require.Len(t, got, len(tc.want), strings.Join(got, "; "))
			for i, w := range tc.want {
				assert.Contains(t, got[i], w)
			}
		})
	}
	offline := compare(idle, idle, []int{5}, true)
	assert.Equal(t, []string{"cpu5 is not in /proc/stat"}, offline)
}
