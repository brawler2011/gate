// Package cpus counts the cores a process can keep busy.
package cpus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// Physical counts physical cores in this process's CPU affinity, capped by the CPU quotas of its cgroup v2 and its ancestors.
func Physical() (int, error) {
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return 0, fmt.Errorf("sched_getaffinity: %w", err)
	}
	membership, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return 0, fmt.Errorf("read cgroup: %w", err)
	}
	var group string
	for line := range strings.Lines(string(membership)) {
		if g, ok := strings.CutPrefix(line, "0::"); ok {
			group = strings.TrimSpace(g)
		}
	}
	return physical("/sys/devices/system/cpu", "/sys/fs/cgroup", group, set.IsSet)
}

func physical(sysfs, cgroupfs, group string, allowed func(cpu int) bool) (int, error) {
	paths, err := filepath.Glob(filepath.Join(sysfs, "cpu[0-9]*", "topology", "core_cpus_list"))
	if err != nil {
		return 0, fmt.Errorf("glob topology: %w", err)
	}
	cores := map[string]bool{}
	for _, p := range paths {
		cpu, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(filepath.Dir(filepath.Dir(p))), "cpu"))
		if err != nil || !allowed(cpu) {
			continue
		}
		siblings, err := os.ReadFile(p)
		if err != nil {
			return 0, fmt.Errorf("read topology: %w", err)
		}
		cores[strings.TrimSpace(string(siblings))] = true
	}
	if len(cores) == 0 {
		return 0, fmt.Errorf("no CPU topology in %s", sysfs)
	}
	n := len(cores)
	// A quota anywhere up the hierarchy caps the process.
	for dir := filepath.Join(cgroupfs, filepath.Clean("/"+group)); ; dir = filepath.Dir(dir) {
		quota, err := cpuQuota(filepath.Join(dir, "cpu.max"))
		if err != nil {
			return 0, err
		}
		if quota > 0 {
			n = min(n, quota)
		}
		if dir == filepath.Clean(cgroupfs) {
			return n, nil
		}
	}
}

// cpuQuota returns the CPUs a cpu.max file allows, rounded up; 0 when it sets no limit.
func cpuQuota(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read CPU quota: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 2 && fields[0] == "max" {
		return 0, nil
	}
	var quota, period int64
	if len(fields) == 2 {
		quota, err = strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			period, err = strconv.ParseInt(fields[1], 10, 64)
		}
	}
	if len(fields) != 2 || err != nil || quota <= 0 || period <= 0 {
		return 0, fmt.Errorf("invalid CPU quota %q in %s", strings.TrimSpace(string(data)), path)
	}
	return int((quota + period - 1) / period), nil
}
