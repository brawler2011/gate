package hostcheck

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestPinned(t *testing.T) {
	t0 := time.Unix(1000, 0)
	probe := func(ms int, cpus string) sandbox.Probe {
		return sandbox.Probe{Start: t0.Add(time.Duration(ms) * time.Millisecond), CPUs: cpus}
	}
	for name, tc := range map[string]struct {
		cores  []int
		probes []sandbox.Probe
		want   int // findings
	}{
		"one loop per core":    {[]int{2, 3}, []sandbox.Probe{probe(5, "3"), probe(0, "2"), probe(520, "2")}, 0},
		"unpinned loops":       {[]int{2, 3}, []sandbox.Probe{probe(0, "2-3"), probe(5, "2-3"), probe(510, "2-3")}, 1},
		"two loops on a core":  {[]int{2}, []sandbox.Probe{probe(0, "2"), probe(3, "2")}, 1},
		"one loop for two":     {[]int{2, 3}, []sandbox.Probe{probe(0, "2"), probe(505, "2"), probe(1010, "2")}, 1},
		"extra loop elsewhere": {[]int{2, 3}, []sandbox.Probe{probe(0, "2"), probe(1, "3"), probe(2, "4")}, 1},
	} {
		m := &Monitor{Cores: tc.cores, Probes: func(context.Context, int) ([]sandbox.Probe, error) { return tc.probes, nil }}
		assert.Len(t, m.pinned(t.Context()), tc.want, name)
	}
}
