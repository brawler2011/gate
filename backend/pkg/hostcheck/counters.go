package hostcheck

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Placeholder until the strict-mode PoC measures real noise.
const maxFrequencyDrift = 0.02

// Under this many reference cycles, about a millisecond, a core did too little work to show its frequency.
const minReferenceCycles = 1_000_000

// counters are what the cores did since boot: time stolen by a hypervisor, and how fast they ran.
type counters map[int]coreCounters

type coreCounters struct {
	steal uint64
	// Intel thermal throttle events; absent elsewhere.
	throttles    uint64
	hasThrottles bool
	// ACPI CPPC reference and delivered performance counters (APERF/MPERF on x86); absent without CPPC.
	reference, delivered uint64
	hasCPPC              bool
	// The delivered/reference ratio of a core running at its fixed frequency.
	expected float64
}

// counters reads the counters of cores; a core with no throttle signal is an error, since throttling could go unseen.
func (h Host) counters(cores []int) (counters, error) {
	stat, err := h.stat()
	if err != nil {
		return nil, err
	}
	all := counters{}
	for _, c := range cores {
		jiffies, ok := stat[c]
		if !ok {
			return nil, fmt.Errorf("cpu%d is not in /proc/stat", c)
		}
		cc := coreCounters{steal: jiffies[7]}
		dir := filepath.Join(h.Sys, "devices/system/cpu", "cpu"+strconv.Itoa(c))
		if exists(filepath.Join(dir, "thermal_throttle")) {
			for _, name := range []string{"core_throttle_count", "package_throttle_count"} {
				n, err := readUint(filepath.Join(dir, "thermal_throttle", name))
				if err != nil {
					return nil, err
				}
				cc.throttles += n
			}
			cc.hasThrottles = true
		}
		if exists(filepath.Join(dir, "acpi_cppc/feedback_ctrs")) {
			if err := cc.readCPPC(dir); err != nil {
				return nil, err
			}
		}
		if !cc.hasThrottles && !cc.hasCPPC {
			return nil, fmt.Errorf("cpu%d has neither thermal_throttle nor acpi_cppc counters", c)
		}
		all[c] = cc
	}
	return all, nil
}

func (cc *coreCounters) readCPPC(dir string) error {
	path := filepath.Join(dir, "acpi_cppc/feedback_ctrs")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	for _, field := range strings.Fields(string(data)) {
		key, value, _ := strings.Cut(field, ":")
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse %s: %q", path, data)
		}
		switch key {
		case "ref":
			cc.reference = n
		case "del":
			cc.delivered = n
		}
	}
	var values [4]uint64
	for i, name := range []string{"acpi_cppc/nominal_freq", "acpi_cppc/nominal_perf", "acpi_cppc/reference_perf", "cpufreq/scaling_max_freq"} {
		if values[i], err = readUint(filepath.Join(dir, name)); err != nil {
			return err
		}
		if values[i] == 0 {
			return fmt.Errorf("%s is 0", filepath.Join(dir, name))
		}
	}
	nominalMHz, nominalPerf, referencePerf, maxKHz := values[0], values[1], values[2], values[3]
	// The reference counter ticks at reference_perf, the delivered one at the performance the core runs at.
	cc.expected = float64(maxKHz) / 1000 / float64(nominalMHz) * float64(nominalPerf) / float64(referencePerf)
	cc.hasCPPC = true
	return nil
}

// cppcDelta handles counters that wrap at 32 bits, as the kernel's cppc_cpufreq does.
func cppcDelta(now, before uint64) uint64 {
	if now >= before || before > math.MaxUint32 {
		return now - before
	}
	return (now - before) & math.MaxUint32
}

func readUint(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %q", path, data)
	}
	return n, nil
}

func (after counters) changed(before counters) []string {
	var f findings
	for c, a := range after {
		b, ok := before[c]
		if !ok {
			f.add("cpu%d has no earlier counters", c)
			continue
		}
		if a.steal > b.steal {
			f.add("cpu%d: %d jiffies stolen by the hypervisor", c, a.steal-b.steal)
		}
		if a.hasThrottles && a.throttles > b.throttles {
			f.add("cpu%d was thermally throttled %d times", c, a.throttles-b.throttles)
		}
		if a.hasCPPC && a.expected != b.expected {
			f.add("cpu%d: frequency limits changed", c)
			continue
		}
		ref, del := cppcDelta(a.reference, b.reference), cppcDelta(a.delivered, b.delivered)
		if a.hasCPPC && ref >= minReferenceCycles {
			ratio := float64(del) / float64(ref) / a.expected
			if ratio < 1-maxFrequencyDrift || ratio > 1+maxFrequencyDrift {
				f.add("cpu%d ran at %.1f%% of its fixed frequency", c, ratio*100)
			}
		}
	}
	slices.Sort(f)
	return f
}
