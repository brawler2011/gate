package hostcheck

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"
)

const (
	sampleWindow    = time.Second
	recheckInterval = time.Hour
)

// Monitor keeps judging closed until the host passes its checks, and closes it for good when a recheck fails:
// a restart re-verifies the host, so a contest never runs on two configurations.
type Monitor struct {
	Host  Host
	Cores []int
	// RunCPUs runs n overlapping programs in go-judge and returns the CPU list each may use.
	RunCPUs func(ctx context.Context, n int) ([]string, error)
	Logger  *slog.Logger

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

// pinned checks that go-judge runs exactly one program per judge core, each on its own core.
func (m *Monitor) pinned(ctx context.Context) []string {
	lists, err := m.RunCPUs(ctx, len(m.Cores))
	if err != nil {
		return []string{fmt.Sprintf("run in go-judge: %v", err)}
	}
	var f findings
	want := make([]string, len(m.Cores))
	for i, c := range m.Cores {
		want[i] = strconv.Itoa(c)
	}
	slices.Sort(lists)
	slices.Sort(want)
	if !slices.Equal(lists, want) {
		f.add("go-judge ran %d overlapping programs on CPUs %v, want one on each judge core", len(lists), lists)
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
