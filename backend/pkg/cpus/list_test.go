package cpus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseList(t *testing.T) {
	for in, want := range map[string][]int{"": nil, "3": {3}, "2-4,0": {0, 2, 3, 4}, "1,1-2": {1, 2}} {
		got, err := ParseList(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"a", "3-1", "1,", "-1", "1-"} {
		_, err := ParseList(in)
		assert.Error(t, err, in)
	}
}
