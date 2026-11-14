package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadSampleStoresValidBoundedText(t *testing.T) {
	root := t.TempDir()
	e := &execution{dir: &directory{root: root}}
	write := func(name, text string) string {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0600))
		return name
	}
	exact := strings.Repeat("a", maxSampleBytes)
	// A two-byte character split by the limit becomes a replacement character.
	split := strings.Repeat("a", maxSampleBytes-1) + "é"
	for name, tc := range map[string]struct{ text, want string }{
		"plain":       {"1 2\n", "1 2\n"},
		"nul":         {"a\x00b", "a�b"},
		"invalid":     {"a\xffb", "a�b"},
		"exact limit": {exact, exact},
		"truncated":   {exact + "b", exact + "\n…"},
		"split rune":  {split, strings.Repeat("a", maxSampleBytes-1) + "�\n…"},
	} {
		got, err := e.readSample(write(strings.ReplaceAll(name, " ", "-"), tc.text))
		require.NoError(t, err, name)
		require.Equal(t, tc.want, got, name)
	}
	got, err := e.readSample("")
	require.NoError(t, err)
	require.Empty(t, got, "a test without an answer file, as in interactive problems, has an empty output")
}
