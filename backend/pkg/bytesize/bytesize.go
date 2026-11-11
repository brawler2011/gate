// Package bytesize parses human-readable data sizes in YAML configuration.
package bytesize

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Size is a number of bytes. In YAML it is written either as a plain integer
// or with a binary unit suffix: B, KiB, MiB, GiB, TiB (for example "512MiB").
// It encodes to JSON as a plain integer.
type Size int64

const (
	B   Size = 1
	KiB      = 1024 * B
	MiB      = 1024 * KiB
	GiB      = 1024 * MiB
	TiB      = 1024 * GiB
)

// Units are checked longest suffix first so that "B" does not shadow "KiB".
var units = []struct {
	suffix string
	size   Size
}{
	{"KiB", KiB},
	{"MiB", MiB},
	{"GiB", GiB},
	{"TiB", TiB},
	{"B", B},
}

// Parse converts "8GiB", "64 KiB" or "1024" to a Size. Decimal units such as
// "GB" or "G" are rejected so that 10^9 and 2^30 are never confused.
func Parse(s string) (Size, error) {
	text := strings.TrimSpace(s)
	multiplier := B
	for _, u := range units {
		if strings.HasSuffix(text, u.suffix) {
			text = strings.TrimSpace(strings.TrimSuffix(text, u.suffix))
			multiplier = u.size
			break
		}
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid data size %q: use an integer with an optional B, KiB, MiB, GiB or TiB suffix", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid data size %q: must not be negative", s)
	}
	if n > math.MaxInt64/int64(multiplier) {
		return 0, fmt.Errorf("invalid data size %q: overflows int64", s)
	}
	return Size(n) * multiplier, nil
}

func (s *Size) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: data size must be a scalar", value.Line)
	}
	parsed, err := Parse(value.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", value.Line, err)
	}
	*s = parsed
	return nil
}

// Uint64 returns the size for APIs that take unsigned byte counts.
// A negative size cannot come from Parse and is clamped to zero.
func (s Size) Uint64() uint64 {
	if s < 0 {
		return 0
	}
	return uint64(s)
}
