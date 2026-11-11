//go:build integration

package sandbox

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegrationSandboxFiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{"5050/tcp", "5051/tcp"}, Privileged: true,
		Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-file-store", "-parallelism=1", "-output-limit=256m"},
		WaitingFor: wait.ForHTTP("/version").WithPort("5050/tcp"),
	}
	if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
		req.Image = image
	} else {
		req.FromDockerfile = testcontainers.FromDockerfile{Context: "../../../judge", Dockerfile: "Dockerfile", KeepImage: true}
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background(), testcontainers.StopTimeout(0))) })
	host, err := c.Host(ctx)
	require.NoError(t, err)
	httpPort, err := c.MappedPort(ctx, "5050/tcp")
	require.NoError(t, err)
	grpcPort, err := c.MappedPort(ctx, "5051/tcp")
	require.NoError(t, err)
	transport, err := NewFileTransport(fmt.Sprintf("http://%s:%s", host, httpPort.Port()), &http.Client{Timeout: time.Minute})
	require.NoError(t, err)
	client, err := NewClient(host + ":" + grpcPort.Port())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg, err := LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	sb := NewSandbox(client, cfg)
	program, err := sb.Compile(ctx, []byte("#include <unistd.h>\nint main(){char b[32768]; ssize_t n; while((n=read(0,b,sizeof b))>0){ssize_t p=0; while(p<n){ssize_t w=write(1,b+p,n-p); if(w<=0)return 2;p+=w;}} return n<0;}"), "cpp", nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, transport.Delete(context.Background(), program.PrimaryFileID)) })
	const size = int64(144 * 1024 * 1024)
	f, err := os.Create(filepath.Join(t.TempDir(), "input"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	hash := sha256.New()
	_, err = io.CopyN(io.MultiWriter(f, hash), rand.Reader, size)
	require.NoError(t, err)
	expected := fmt.Sprintf("%x", hash.Sum(nil))
	limits := FileRunLimits{CPU: 10 * time.Second, Wall: 30 * time.Second, Memory: 64 * 1024 * 1024, Processes: 10, Output: bytesize.Size(size), Diagnostic: 4096}
	runtime.GC()
	var initial runtime.MemStats
	runtime.ReadMemStats(&initial)
	var peak uint64
	stop := make(chan struct{})
	var sampler sync.WaitGroup
	sampler.Add(1)
	go func() {
		defer sampler.Done()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			var stat runtime.MemStats
			runtime.ReadMemStats(&stat)
			if stat.HeapAlloc > peak {
				peak = stat.HeapAlloc
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	var stopOnce sync.Once
	stopSampling := func() { stopOnce.Do(func() { close(stop); sampler.Wait() }) }
	defer stopSampling()
	for range 3 {
		id, err := transport.Upload(ctx, f, size)
		require.NoError(t, err)
		result, err := sb.RunFiles(ctx, program, "cpp", id, nil, nil, limits)
		require.NoError(t, err)
		require.Equal(t, StatusOK, result.Status, "%s", result.Stderr)
		require.NotEmpty(t, result.OutputID)
		hash.Reset()
		n, err := transport.Download(ctx, result.OutputID, hash, size)
		require.NoError(t, err)
		require.Equal(t, size, n)
		require.Equal(t, expected, fmt.Sprintf("%x", hash.Sum(nil)))
		_, err = transport.Download(ctx, result.OutputID, io.Discard, size-1)
		require.Error(t, err)
		require.NoError(t, transport.Delete(ctx, id))
		require.NoError(t, transport.Delete(ctx, result.OutputID))
	}
	// Stop sampling before reading peak to avoid a race with the sampler.
	stopSampling()
	require.Less(t, peak-initial.HeapAlloc, uint64(48*1024*1024), "heap grew with test bodies")
	t.Logf("streamed 3 x %d bytes, peak Go heap increase %d bytes", size, peak-initial.HeapAlloc)
	inputID, err := transport.Upload(ctx, f, size)
	require.NoError(t, err)
	defer func() { require.NoError(t, transport.Delete(context.Background(), inputID)) }()
	limits.Output = 1024
	r, err := sb.RunFiles(ctx, program, "cpp", inputID, nil, nil, limits)
	require.NoError(t, err)
	require.Equal(t, StatusOLE, r.Status)
	if r.OutputID != "" {
		require.NoError(t, transport.Delete(ctx, r.OutputID))
	}
	cancelled, stopRun := context.WithCancel(ctx)
	stopRun()
	_, err = sb.RunFiles(cancelled, program, "cpp", inputID, nil, nil, limits)
	require.Error(t, err)
}
