//go:build integration

package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestImmutableS3Object(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "chrislusf/seaweedfs:4.53_full", ExposedPorts: []string{"8333/tcp"},
		Cmd:        []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=100", "-master.volumeSizeLimitMB=32", "-dir=/data"},
		WaitingFor: wait.ForHTTP("/").WithPort("8333/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "8333/tcp")
	require.NoError(t, err)
	store := NewS3Storage(S3Config{Endpoint: fmt.Sprintf("http://%s:%s", host, port.Port()), AccessKey: "test", SecretKey: "test", Region: "us-east-1"})
	require.NoError(t, store.EnsureBucket(ctx, "objects"))
	f, err := os.Create(filepath.Join(t.TempDir(), "object"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	h := sha256.New()
	const size = int64(24 * 1024 * 1024)
	_, err = io.CopyN(io.MultiWriter(f, h), rand.Reader, size)
	require.NoError(t, err)
	digest := hex.EncodeToString(h.Sum(nil))
	require.NoError(t, store.PutImmutable(ctx, "objects", "unique-key", f, size, digest))
	require.Error(t, store.PutImmutable(ctx, "objects", "unique-key", f, size, digest), "provider must reject replacement of an existing UUID")
	require.NoError(t, store.VerifyObject(ctx, "objects", "unique-key", size, digest))
	require.Error(t, store.VerifyObject(ctx, "objects", "unique-key", size-1, digest))
	require.NoError(t, store.DeleteFile(ctx, "objects", "unique-key"))
}
