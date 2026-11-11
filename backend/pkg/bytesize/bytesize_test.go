package bytesize

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]Size{
		"0": 0, "1024": 1024, "17B": 17, "64KiB": 64 * KiB, "512 MiB": 512 * MiB, "8GiB": 8 * GiB, "1TiB": TiB,
	} {
		got, err := Parse(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "8GB", "8G", "1.5GiB", "-1", "MiB", "9999999TiB"} {
		_, err := Parse(in)
		require.Error(t, err, in)
	}
}

func TestYAMLAndJSON(t *testing.T) {
	var v struct {
		A Size `yaml:"a"`
		B Size `yaml:"b"`
	}
	require.NoError(t, yaml.Unmarshal([]byte("a: 8GiB\nb: 65536\n"), &v))
	require.Equal(t, 8*GiB, v.A)
	require.Equal(t, 64*KiB, v.B)
	require.Error(t, yaml.Unmarshal([]byte("a: [1]\n"), &v))

	data, err := json.Marshal(v.A)
	require.NoError(t, err)
	require.JSONEq(t, "8589934592", string(data), "snapshots keep plain integer bytes")
}
