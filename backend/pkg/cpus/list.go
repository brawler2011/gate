package cpus

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ParseList parses a kernel CPU list such as "0-2,5" into sorted, distinct CPU numbers.
func ParseList(s string) ([]int, error) {
	var list []int
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for part := range strings.SplitSeq(s, ",") {
		lo, hi, isRange := strings.Cut(strings.TrimSpace(part), "-")
		first, err := strconv.Atoi(lo)
		last := first
		if err == nil && isRange {
			last, err = strconv.Atoi(hi)
		}
		if err != nil || first < 0 || last < first {
			return nil, fmt.Errorf("invalid CPU list %q", s)
		}
		for cpu := first; cpu <= last; cpu++ {
			list = append(list, cpu)
		}
	}
	slices.Sort(list)
	return slices.Compact(list), nil
}
