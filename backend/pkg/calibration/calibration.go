// Package calibration measures how fast and how steadily judge cores run, against a reference machine.
package calibration

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"golang.org/x/sync/errgroup"
)

//go:embed programs/*.c
var programs embed.FS

// Profiles stress different parts of a core, so hardware that differs in one is not scaled as if it were uniform.
var Profiles = []string{"branchy", "compute", "random", "stream"}

// Calibration is what a verdict's time limits were scaled by.
type Calibration struct {
	K float64
	// Empty when no benchmark ran, as in dirty mode.
	Benchmark string
}

var None = Calibration{K: 1}

// Uncalibrated provides None where no benchmark runs.
func Uncalibrated() Calibration { return None }

func (c Calibration) Limit(d time.Duration) time.Duration { return time.Duration(float64(d) * c.K) }

// Normalize converts a time measured here to the reference machine's.
func (c Calibration) Normalize(d time.Duration) time.Duration { return time.Duration(float64(d) / c.K) }

type Settings struct {
	Repeats int `yaml:"repeats"`
	// Largest (max-min)/median of a profile's times, on one core and between cores.
	MaxSpread float64 `yaml:"max_spread"`
	// Largest relative difference between profiles' k; beyond it the host is different hardware, not slower.
	MaxDisagreement float64 `yaml:"max_disagreement"`
	// Median CPU time of each profile on the reference machine; empty until it is measured.
	Reference map[string]time.Duration `yaml:"reference"`
}

// ID names the benchmark: the programs and the judge image that compiles them.
func ID(imageDigest string) string {
	h := sha256.New()
	for _, p := range Profiles {
		source, _ := programs.ReadFile("programs/" + p + ".c")
		h.Write(source)
	}
	h.Write([]byte(imageDigest))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Run is one program's run: the CPU it ran on and its CPU time.
type Run struct {
	CPU  int
	Time time.Duration
}

// Measurements are every profile's runs.
type Measurements map[string][]Run

// Measure runs each profile on all cores at once, repeats times, as judging loads them.
func Measure(ctx context.Context, sb *sandbox.Sandbox, files *sandbox.FileTransport, compile sandbox.FileRunLimits, cores, repeats int) (Measurements, error) {
	m := Measurements{}
	for _, profile := range Profiles {
		source, err := programs.ReadFile("programs/" + profile + ".c")
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", profile, err)
		}
		exe, err := sb.CompileWithLimits(ctx, source, "c", nil, compile)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", profile, err)
		}
		runs, err := measure(ctx, sb, files, exe, cores, repeats)
		release(files, exe.PrimaryFileID)
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", profile, err)
		}
		m[profile] = runs
	}
	return m, nil
}

var runLimits = sandbox.FileRunLimits{CPU: 10 * time.Second, Wall: 30 * time.Second, Memory: 256 * bytesize.MiB, Processes: 1, Output: bytesize.KiB, Diagnostic: bytesize.KiB}

func measure(ctx context.Context, sb *sandbox.Sandbox, files *sandbox.FileTransport, exe sandbox.Executable, cores, repeats int) ([]Run, error) {
	var mu sync.Mutex
	var runs []Run
	for range repeats {
		g, ctx := errgroup.WithContext(ctx)
		for range cores {
			g.Go(func() error {
				r, err := sb.RunSolution(ctx, exe, "c", sandbox.File{}, sandbox.Streams{}, runLimits)
				if err != nil {
					return err //nolint:wrapcheck // describes the run
				}
				release(files, r.OutputID)
				cpu, err := strconv.Atoi(strings.TrimSpace(string(r.Stderr)))
				if r.Status != sandbox.StatusOK || err != nil {
					return fmt.Errorf("status %s, stderr %q", r.Status, r.Stderr)
				}
				mu.Lock()
				runs = append(runs, Run{CPU: cpu, Time: r.Time})
				mu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err //nolint:wrapcheck // describes the run
		}
	}
	return runs, nil
}

func release(files *sandbox.FileTransport, id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = files.Delete(ctx, id)
}

// Evaluate returns k, the geometric mean of the profiles' slowdowns against the reference, or why the host is unfit.
func (s Settings) Evaluate(m Measurements) (float64, []string) {
	var reasons []string
	fail := func(format string, args ...any) { reasons = append(reasons, fmt.Sprintf(format, args...)) }
	medians := map[string]time.Duration{}
	for _, profile := range Profiles {
		perCore := map[int][]time.Duration{}
		var all []time.Duration
		for _, r := range m[profile] {
			perCore[r.CPU] = append(perCore[r.CPU], r.Time)
			all = append(all, r.Time)
		}
		if len(all) == 0 {
			fail("%s: no runs", profile)
			continue
		}
		var coreMedians []time.Duration
		for _, cpu := range slices.Sorted(maps.Keys(perCore)) {
			times := perCore[cpu]
			if spread := spread(times); spread > s.MaxSpread {
				fail("%s on cpu%d varies by %.1f%%", profile, cpu, spread*100)
			}
			coreMedians = append(coreMedians, median(times))
		}
		if spread := spread(coreMedians); spread > s.MaxSpread {
			fail("%s differs between cores by %.1f%%", profile, spread*100)
		}
		medians[profile] = median(all)
	}
	if len(reasons) > 0 {
		return 0, reasons
	}
	if len(s.Reference) == 0 {
		return 0, []string{fmt.Sprintf("judge.benchmark.reference is empty; on a reference machine set it to the measured %s", formatMedians(medians))}
	}
	var ks []float64
	logSum := 0.0
	for _, profile := range Profiles {
		ref, ok := s.Reference[profile]
		if !ok || ref <= 0 {
			return 0, []string{fmt.Sprintf("judge.benchmark.reference has no %s", profile)}
		}
		k := float64(medians[profile]) / float64(ref)
		ks = append(ks, k)
		logSum += math.Log(k)
	}
	if lo, hi := slices.Min(ks), slices.Max(ks); hi/lo-1 > s.MaxDisagreement {
		return 0, []string{fmt.Sprintf("profiles disagree on k (%.3f-%.3f): this is different hardware, not a slower reference; measured %s", lo, hi, formatMedians(medians))}
	}
	return math.Exp(logSum / float64(len(ks))), nil
}

func formatMedians(medians map[string]time.Duration) string {
	parts := make([]string, 0, len(medians))
	for _, p := range Profiles {
		parts = append(parts, fmt.Sprintf("%s: %s", p, medians[p].Round(time.Millisecond/10)))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func median(times []time.Duration) time.Duration {
	sorted := slices.Sorted(slices.Values(times))
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func spread(times []time.Duration) float64 {
	return float64(slices.Max(times)-slices.Min(times)) / float64(median(times))
}
