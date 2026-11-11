package sandbox

import (
	"context"
	"errors"
	"fmt"
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
