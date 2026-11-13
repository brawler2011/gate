package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnavailable(t *testing.T) {
	assert.True(t, unavailable(fmt.Errorf("exec: %w", status.Error(codes.Unavailable, "connection refused"))))
	assert.False(t, unavailable(fmt.Errorf("exec: %w", status.Error(codes.Internal, "boom"))))
	assert.False(t, unavailable(errors.New("invalid identity.json")))
}

// An unreachable go-judge is retried until the deadline and reported as such.
func TestReadImageWaitsForUnreachableJudge(t *testing.T) {
	client, err := NewClient("127.0.0.1:1")
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err = ReadImage(ctx, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go-judge unreachable")
}

// An API instance reads the manifest that the judge image ships, copied to a directory.
func TestReadImageDir(t *testing.T) {
	dir := t.TempDir()
	langs, err := os.ReadFile(filepath.Join("..", "..", "..", "judge", "langs.toml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "langs.toml"), langs, 0o600))
	digest := "sha256:" + strings.Repeat("ab", 32)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "identity.json"), []byte(`{"digest":"`+digest+`","architecture":"x86_64","abi":"linux-gnu"}`), 0o600))

	image, err := ReadImageDir(dir)
	require.NoError(t, err)
	assert.Equal(t, Identity{Digest: digest, Architecture: "x86_64", ABI: "linux-gnu"}, image.Identity)
	assert.Contains(t, image.Config.Langs, "cpp")

	_, err = ReadImageDir(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity.json")
}
