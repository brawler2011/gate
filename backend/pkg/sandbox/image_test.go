package sandbox

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Changing compiler flags invalidates binaries even when the image digest stays the same.
func TestImageCompilerCoversLanguageConfig(t *testing.T) {
	image := func(digest, compile string) Image {
		return Image{Identity: Identity{Digest: digest}, Config: &Config{Langs: map[string]Language{"cpp": {Compile: compile}, "python": {}}}}
	}
	base := image("sha256:aa", "g++ -O2")
	assert.Equal(t, base.Compiler(), image("sha256:aa", "g++ -O2").Compiler())
	assert.NotEqual(t, base.Compiler(), image("sha256:aa", "g++ -O2 -DX").Compiler())
	assert.NotEqual(t, base.Compiler(), image("sha256:bb", "g++ -O2").Compiler())
}

// An unreachable go-judge is retried until the deadline and reported as such.
func TestReadImageWaitsForUnreachableJudge(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1", &http.Client{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err = ReadImage(ctx, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go-judge unreachable")
}
