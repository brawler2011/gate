package syslib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Git blob hashing must recognize imported upstream library copies.
func TestIsUpstream(t *testing.T) {
	for _, f := range Files() {
		require.True(t, f.IsUpstream(f.Content), f.Name)
		require.True(t, f.IsUpstream(strings.ReplaceAll(f.Content, "\n", "\r\n")), f.Name)
		require.False(t, f.IsUpstream(f.Content+"// patched\n"), f.Name)
		require.NotEmpty(t, f.Version, f.Name)
	}
}
