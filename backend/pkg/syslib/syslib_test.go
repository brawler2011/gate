package syslib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Imported copies are matched against upstream history by Git blob ID, so a
// hashing mistake would silently stop recognizing every upstream copy.
func TestIsUpstream(t *testing.T) {
	for _, f := range Files() {
		require.True(t, f.IsUpstream(f.Content), f.Name)
		require.True(t, f.IsUpstream(strings.ReplaceAll(f.Content, "\n", "\r\n")), f.Name)
		require.False(t, f.IsUpstream(f.Content+"// patched\n"), f.Name)
		require.NotEmpty(t, f.Version, f.Name)
	}
}
