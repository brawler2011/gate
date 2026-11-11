package points

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExactAuthoredPoints(t *testing.T) {
	third, err := Parse("3.33")
	require.NoError(t, err)
	last, err := Parse("3.34")
	require.NoError(t, err)
	require.Equal(t, 10*Unit, third+third+last)
	data, err := json.Marshal(third + last)
	require.NoError(t, err)
	require.Equal(t, "6.67", string(data))
	for _, bad := range []string{"0.001", "1e2", "-1", "NaN", "1000000000.01"} {
		_, err := Parse(bad)
		require.Error(t, err)
	}
	numeric, err := (third + last).NumericValue()
	require.NoError(t, err)
	var restored Value
	require.NoError(t, restored.ScanNumeric(numeric))
	require.Equal(t, third+last, restored)
}
