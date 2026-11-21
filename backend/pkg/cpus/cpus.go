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

// Physical counts physical cores in this process's CPU affinity, capped by its cgroup v2 CPU quota.
func Physical() (int, error) {
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return 0, fmt.Errorf("sched_getaffinity: %w", err)
	}
	return physical("/sys/devices/system/cpu", "/sys/fs/cgroup/cpu.max", set.IsSet)
}

func physical(sysfs, quotaFile string, allowed func(cpu int) bool) (int, error) {
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
	data, err := os.ReadFile(quotaFile)
	if errors.Is(err, fs.ErrNotExist) {
		return len(cores), nil
	}
	if err != nil {
		return 0, fmt.Errorf("read CPU quota: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 2 && fields[0] == "max" {
		return len(cores), nil
	}
	var quota, period int64
	if len(fields) == 2 {
		quota, err = strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			period, err = strconv.ParseInt(fields[1], 10, 64)
		}
	}
	if len(fields) != 2 || err != nil || quota <= 0 || period <= 0 {
		return 0, fmt.Errorf("invalid CPU quota %q", strings.TrimSpace(string(data)))
	}
	return min(len(cores), int((quota+period-1)/period)), nil
}
