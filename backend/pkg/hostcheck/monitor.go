package hostcheck

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

const (
	sampleWindow = time.Second
	// Probes run for 500 ms; one that starts sooner after the first ran beside it.
	probeOverlap    = 400 * time.Millisecond
	recheckInterval = time.Hour
)

// Monitor keeps judging closed until the host passes its checks, and closes it for good when a recheck fails:
// a restart re-verifies the host, so a contest never runs on two configurations.
type Monitor struct {
	Host  Host
	Cores []int
	// Probes sends n programs to go-judge at once.
	Probes func(ctx context.Context, n int) ([]sandbox.Probe, error)
	Logger *slog.Logger

	mu      sync.Mutex
	checked bool
	reasons []string
}

// Ready reports whether the worker may take judging and build jobs, and if not, why.
func (m *Monitor) Ready() (bool, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.checked {
		return false, []string{"host checks are running"}
	}
	return len(m.reasons) == 0, m.reasons
}

func (m *Monitor) Run(ctx context.Context) error {
	reasons := append(m.Host.Settings(m.Cores), ProcessAffinity(m.Cores)...)
	reasons = append(reasons, m.pinned(ctx)...)
	reasons = append(reasons, m.Host.Activity(ctx, m.Cores, sampleWindow, true)...)
	m.report(ctx, reasons)
	ticker := time.NewTicker(recheckInterval)
	defer ticker.Stop()
	for len(reasons) == 0 {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		// Cores may be busy judging now, so only steal and interrupts count.
		reasons = append(m.Host.Settings(m.Cores), m.Host.Activity(ctx, m.Cores, sampleWindow, false)...)
		m.report(ctx, reasons)
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

func (m *Monitor) report(ctx context.Context, reasons []string) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	m.checked, m.reasons = true, reasons
	m.mu.Unlock()
	if len(reasons) == 0 {
		m.Logger.InfoContext(ctx, "strict judging host checks passed", "cpus", m.Cores)
		return
	}
	for _, r := range reasons {
		m.Logger.ErrorContext(ctx, "strict judging host check failed, judging paused until restart", "reason", r)
	}
}
