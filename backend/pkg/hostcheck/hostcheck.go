// Package hostcheck verifies that judge cores give repeatable timing. It reads the host and never changes it.
package hostcheck

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg/cpus"
	"golang.org/x/sys/unix"
)

type Host struct {
	// Where sysfs and procfs are mounted.
	Sys, Proc string
	// Container runtimes mask /proc/interrupts, so it is read where it is not.
	Interrupts func(context.Context) ([]byte, error)
}

// Placeholder until the strict-mode PoC measures real noise.
const maxBusyPercent = 2

type findings []string

func (f *findings) add(format string, args ...any) { *f = append(*f, fmt.Sprintf(format, args...)) }

// read reports an unreadable value as a finding: what cannot be verified does not pass.
func (f *findings) read(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		f.add("cannot read %s: %v", path, err)
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

func (f *findings) list(path string) []int {
	s, ok := f.read(path)
	if !ok || s == "(null)" {
		return nil
	}
	list, err := cpus.ParseList(s)
	if err != nil {
		f.add("%s: %v", path, err)
	}
	return list
}

func (f *findings) frequency(path string) (uint64, bool) {
	s, ok := f.read(path)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		f.add("invalid frequency in %s: %q", path, s)
		return 0, false
	}
	return n, true
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Settings checks the host configuration that decides how fast and how steadily the cores run.
func (h Host) Settings(cores []int) []string {
	var f findings
	cpu := filepath.Join(h.Sys, "devices/system/cpu")
	if v, ok := f.read(filepath.Join(cpu, "smt/active")); ok && v != "0" {
		f.add("SMT is active")
	}
	if noTurbo := filepath.Join(cpu, "intel_pstate/no_turbo"); exists(noTurbo) {
		if v, ok := f.read(noTurbo); ok && v != "1" {
			f.add("turbo is on: intel_pstate/no_turbo is %s", v)
		}
	} else if v, ok := f.read(filepath.Join(cpu, "cpufreq/boost")); ok && v != "0" {
		f.add("turbo is on: cpufreq/boost is %s", v)
	}
	isolated := f.list(filepath.Join(cpu, "isolated"))
	nohz := f.list(filepath.Join(cpu, "nohz_full"))
	for _, c := range cores {
		dir := filepath.Join(cpu, "cpu"+strconv.Itoa(c))
		if v, ok := f.read(filepath.Join(dir, "topology/thread_siblings_list")); ok && v != strconv.Itoa(c) {
			f.add("cpu%d shares its core with CPUs %s", c, v)
		}
		if boost := filepath.Join(dir, "cpufreq/boost"); exists(boost) {
			if v, ok := f.read(boost); ok && v != "0" {
				f.add("cpu%d: turbo is on", c)
			}
		}
		if v, ok := f.read(filepath.Join(dir, "cpufreq/scaling_governor")); ok && v != "performance" {
			f.add("cpu%d: governor is %s, not performance", c, v)
		}
		lo, okLo := f.frequency(filepath.Join(dir, "cpufreq/scaling_min_freq"))
		hi, okHi := f.frequency(filepath.Join(dir, "cpufreq/scaling_max_freq"))
		if okLo && okHi && lo != hi {
			f.add("cpu%d: frequency is not fixed: %d-%d kHz", c, lo, hi)
		}
		if !slices.Contains(isolated, c) {
			f.add("cpu%d is not isolated (isolcpus)", c)
		}
		if !slices.Contains(nohz, c) {
			f.add("cpu%d is not in nohz_full", c)
		}
	}
	h.irqDefault(&f, cores)
	h.numa(&f, cores)
	h.power(&f)
	return f
}

func (h Host) irqDefault(f *findings, cores []int) {
	path := filepath.Join(h.Proc, "irq/default_smp_affinity")
	v, ok := f.read(path)
	if !ok {
		return
	}
	mask, ok := new(big.Int).SetString(strings.ReplaceAll(v, ",", ""), 16)
	if !ok {
		f.add("invalid %s: %q", path, v)
		return
	}
	for _, c := range cores {
		if mask.Bit(c) == 1 {
			f.add("new IRQs may go to cpu%d (irqaffinity)", c)
		}
	}
}

func (h Host) numa(f *findings, cores []int) {
	paths, _ := filepath.Glob(filepath.Join(h.Sys, "devices/system/node/node[0-9]*/cpulist"))
	if len(paths) == 0 {
		f.add("no NUMA topology in %s", filepath.Join(h.Sys, "devices/system/node"))
		return
	}
	nodes := map[int]string{}
	for _, p := range paths {
		for _, c := range f.list(p) {
			nodes[c] = filepath.Base(filepath.Dir(p))
		}
	}
	used := map[string]bool{}
	for _, c := range cores {
		if nodes[c] == "" {
			f.add("cpu%d is in no NUMA node", c)
		}
		used[nodes[c]] = true
	}
	if len(used) > 1 {
		f.add("judge cores span %d NUMA nodes", len(used))
	}
}

// power fails a laptop on battery, where firmware may cap the CPU.
func (h Host) power(f *findings) {
	types, _ := filepath.Glob(filepath.Join(h.Sys, "class/power_supply/*/type"))
	battery, external := false, false
	for _, p := range types {
		kind, ok := f.read(p)
		if !ok {
			continue
		}
		switch kind {
		case "Battery":
			battery = true
		case "Mains", "USB":
			if online, ok := f.read(filepath.Join(filepath.Dir(p), "online")); ok && online == "1" {
				external = true
			}
		}
	}
	if battery && !external {
		f.add("running on battery")
	}
}

// ProcessAffinity reports judge cores this process may run on; they belong to go-judge alone.
func ProcessAffinity(cores []int) []string {
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return []string{fmt.Sprintf("sched_getaffinity: %v", err)}
	}
	var f findings
	for _, c := range cores {
		if set.IsSet(c) {
			f.add("the worker process may run on cpu%d", c)
		}
	}
	return f
}

// Activity samples the cores for window: steal and device interrupts always fail, and with idle so does any work.
func (h Host) Activity(ctx context.Context, cores []int, window time.Duration, idle bool) []string {
	before, err := h.sample(ctx, cores)
	if err != nil {
		return []string{err.Error()}
	}
	select {
	case <-ctx.Done():
		return []string{ctx.Err().Error()}
	case <-time.After(window):
	}
	after, err := h.sample(ctx, cores)
	if err != nil {
		return []string{err.Error()}
	}
	return compare(before, after, cores, idle)
}

type sample struct {
	stat map[int][]uint64          // cpu -> /proc/stat jiffies
	irqs map[string]map[int]uint64 // device IRQ -> cpu -> count
}

func (h Host) sample(ctx context.Context, cores []int) (sample, error) {
	s := sample{stat: map[int][]uint64{}, irqs: map[string]map[int]uint64{}}
	stat, err := os.ReadFile(filepath.Join(h.Proc, "stat"))
	if err != nil {
		return s, fmt.Errorf("read /proc/stat: %w", err)
	}
	for line := range strings.Lines(string(stat)) {
		fields := strings.Fields(line)
		if len(fields) < 9 || !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
			continue
		}
		c, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu"))
		if err != nil {
			return s, fmt.Errorf("parse /proc/stat line %q", strings.TrimSpace(line))
		}
		// user nice system idle iowait irq softirq steal; guest time is already in user.
		for _, v := range fields[1:9] {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return s, fmt.Errorf("parse /proc/stat line %q", strings.TrimSpace(line))
			}
			s.stat[c] = append(s.stat[c], n)
		}
	}
	interrupts, err := h.Interrupts(ctx)
	if err != nil {
		return s, fmt.Errorf("read /proc/interrupts: %w", err)
	}
	s.irqs, err = parseInterrupts(string(interrupts), cores)
	return s, err
}

// parseInterrupts returns the device IRQ counts on cores, failing when any is missing.
func parseInterrupts(text string, cores []int) (map[string]map[int]uint64, error) {
	lines := strings.Split(text, "\n")
	column := map[int]int{}
	for i, name := range strings.Fields(lines[0]) {
		c, err := strconv.Atoi(strings.TrimPrefix(name, "CPU"))
		if err != nil {
			return nil, fmt.Errorf("parse /proc/interrupts header %q", lines[0])
		}
		column[c] = i
	}
	irqs := map[string]map[int]uint64{}
	for _, line := range lines[1:] {
		label, rest, ok := strings.Cut(line, ":")
		label = strings.TrimSpace(label)
		// Only numbered rows are device interrupts; named ones are per-CPU timers and IPIs.
		if _, err := strconv.Atoi(label); !ok || err != nil {
			continue
		}
		counts := strings.Fields(rest)
		irqs[label] = map[int]uint64{}
		for _, c := range cores {
			i, ok := column[c]
			if !ok {
				return nil, fmt.Errorf("cpu%d is not in /proc/interrupts", c)
			}
			if i >= len(counts) {
				return nil, fmt.Errorf("parse /proc/interrupts line %q", strings.TrimSpace(line))
			}
			n, err := strconv.ParseUint(counts[i], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse /proc/interrupts line %q", strings.TrimSpace(line))
			}
			irqs[label][c] = n
		}
	}
	if len(irqs) == 0 {
		return nil, errors.New("no device interrupts in /proc/interrupts")
	}
	return irqs, nil
}

func compare(before, after sample, cores []int, idle bool) []string {
	var f findings
	for _, c := range cores {
		b, a := before.stat[c], after.stat[c]
		if len(b) != 8 || len(a) != 8 {
			f.add("cpu%d is not in /proc/stat", c)
			continue
		}
		var d [8]uint64
		var total uint64
		for i := range d {
			d[i] = a[i] - b[i]
			total += d[i]
		}
		if d[7] > 0 {
			f.add("cpu%d: %d jiffies stolen by the hypervisor", c, d[7])
		}
		busy := total - d[3] - d[4]
		if idle && total > 0 && busy*100 > total*maxBusyPercent {
			f.add("cpu%d is busy %d%% while nothing is judged", c, busy*100/total)
		}
	}
	for irq, counts := range after.irqs {
		for _, c := range cores {
			if n, ok := counts[c]; ok && n > before.irqs[irq][c] {
				f.add("IRQ %s was delivered to cpu%d", irq, c)
			}
		}
	}
	slices.Sort(f)
	return f
}
