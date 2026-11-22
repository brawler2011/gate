package hostcheck

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

const (
	sampleWindow = time.Second
	// Probes run for 500 ms; one that starts sooner after the first ran beside it.
	probeOverlap    = 400 * time.Millisecond
	recheckInterval = time.Hour
)

// Open lets a worker take jobs without host checks, as dirty mode does.
type Open struct{}

func (Open) Ready() bool                { return true }
func (Open) Admit(context.Context) bool { return true }

// Monitor keeps judging closed until the host passes its checks, and closes it for good when a later check fails:
// a restart re-verifies the host, so a contest never runs on two configurations.
type Monitor struct {
	Host  Host
	Cores []int
	// Probes sends n programs to go-judge at once.
	Probes func(ctx context.Context, n int) ([]sandbox.Probe, error)
	// Benchmark runs once the host passes the other checks; k never changes afterwards.
	Benchmark func(ctx context.Context) (calibration.Calibration, []string)
	Logger    *slog.Logger

	mu          sync.Mutex
	checked     bool
	reasons     []string
	last        counters
	calibration calibration.Calibration
}

// Calibration is valid once the monitor is ready.
func (m *Monitor) Calibration() calibration.Calibration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calibration
}

// Status reports whether the worker may take judging and build jobs, and if not, why.
func (m *Monitor) Status() (bool, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.checked {
		return false, []string{"host checks are running"}
	}
	return len(m.reasons) == 0, m.reasons
}

// Ready is the cheap check for idle loops; Admit is the one before taking a job.
func (m *Monitor) Ready() bool {
	ok, _ := m.Status()
	return ok
}

// Admit closes judging if a hypervisor stole time from the judge cores or they were throttled since the last call.
func (m *Monitor) Admit(ctx context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.checked || len(m.reasons) > 0 {
		return false
	}
	now, err := m.Host.counters(m.Cores)
	if err != nil {
		m.fail(ctx, []string{err.Error()})
		return false
	}
	reasons := now.changed(m.last)
	m.last = now
	m.fail(ctx, reasons)
	return len(reasons) == 0
}

func (m *Monitor) Run(ctx context.Context) error {
	reasons := append(m.Host.Settings(m.Cores), ProcessAffinity(m.Cores)...)
	reasons = append(reasons, m.pinned(ctx)...)
	reasons = append(reasons, m.Host.Activity(ctx, m.Cores, sampleWindow, true)...)
	var calib calibration.Calibration
	if len(reasons) == 0 {
		var unfit []string
		calib, unfit = m.Benchmark(ctx)
		reasons = append(reasons, unfit...)
	}
	// After the benchmark, so the first Admit also sees throttling during it.
	baseline, err := m.Host.counters(m.Cores)
	if err != nil {
		reasons = append(reasons, err.Error())
	}
	if ctx.Err() != nil {
		return nil
	}
	m.mu.Lock()
	m.checked, m.last, m.calibration = true, baseline, calib
	m.fail(ctx, reasons)
	m.mu.Unlock()
	if len(reasons) == 0 {
		m.Logger.InfoContext(ctx, "strict judging host checks passed", "cpus", m.Cores, "k", calib.K, "benchmark", calib.Benchmark)
	}
	ticker := time.NewTicker(recheckInterval)
	defer ticker.Stop()
	for m.Ready() {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		// Cores may be busy judging now, so only steal and interrupts count.
		reasons := append(m.Host.Settings(m.Cores), m.Host.Activity(ctx, m.Cores, sampleWindow, false)...)
		if ctx.Err() != nil {
			return nil
		}
		m.mu.Lock()
		m.fail(ctx, reasons)
		m.mu.Unlock()
		m.Admit(ctx)
	}
	<-ctx.Done()
	return nil
}

// pinned checks that go-judge runs one program per judge core at a time, each on its own core:
// of one probe more than there are cores, the first ones cover the cores and the last waits.
func (m *Monitor) pinned(ctx context.Context) []string {
	n := len(m.Cores)
	probes, err := m.Probes(ctx, n+1)
	if err != nil {
		return []string{fmt.Sprintf("run in go-judge: %v", err)}
	}
	slices.SortFunc(probes, func(a, b sandbox.Probe) int { return a.Start.Compare(b.Start) })
	var f findings
	got := make([]string, n)
	want := make([]string, n)
	for i, c := range m.Cores {
		got[i], want[i] = probes[i].CPUs, strconv.Itoa(c)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		f.add("go-judge ran programs at once on CPUs %v, want one on each judge core %v", got, want)
	}
	if probes[n].Start.Sub(probes[0].Start) < probeOverlap {
		f.add("go-judge runs more than %d programs at once", n)
	}
	return f
}

// fail closes judging for good; the caller holds mu.
func (m *Monitor) fail(ctx context.Context, reasons []string) {
	m.reasons = append(m.reasons, reasons...)
	for _, r := range reasons {
		m.Logger.ErrorContext(ctx, "strict judging host check failed, judging paused until restart", "reason", r)
	}
}
