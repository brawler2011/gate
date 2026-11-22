package hostcheck

import (
	"context"
	"log/slog"
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
	// Pinned returns the CPU list a program sees when go-judge runs it on cpu.
	Pinned func(ctx context.Context, cpu string) (string, error)
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

func (m *Monitor) pinned(ctx context.Context) []string {
	var f findings
	for _, c := range m.Cores {
		cpu := strconv.Itoa(c)
		got, err := m.Pinned(ctx, cpu)
		if err != nil {
			f.add("run on cpu%d in go-judge: %v", c, err)
		} else if got != cpu {
			f.add("go-judge run on cpu%d may use CPUs %s", c, got)
		}
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
