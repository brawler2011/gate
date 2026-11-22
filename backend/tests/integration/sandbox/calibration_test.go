//go:build integration

package sandbox_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The benchmark programs build with the judge image's compiler, and go-judge pins one run loop per listed core.
func TestIntegrationCalibration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	image, err := judgeimage.Ref()
	require.NoError(t, err)
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"5050/tcp"}, Privileged: true,
		Cmd:        []string{"-http-addr=0.0.0.0:5050", "-parallelism=2", "-cpuset=0,1", "-no-fallback"},
		WaitingFor: wait.ForHTTP("/version").WithPort("5050/tcp"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background(), testcontainers.StopTimeout(0))) })
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "5050/tcp")
	require.NoError(t, err)
	base := fmt.Sprintf("http://%s:%s", host, port.Port())
	client, err := sandbox.NewClient(base, &http.Client{})
	require.NoError(t, err)
	files, err := sandbox.NewFileTransport(base, &http.Client{Timeout: time.Minute})
	require.NoError(t, err)
	cfg, err := sandbox.LoadConfig("../../../../judge/langs.toml")
	require.NoError(t, err)

	probes, err := client.Probes(ctx, 2)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"0", "1"}, []string{probes[0].CPUs, probes[1].CPUs})

	m, err := calibration.Measure(ctx, sandbox.NewSandbox(client, cfg), files, dbfixtures.BuildLimits(t).Compile, 2, 1)
	require.NoError(t, err)
	for _, profile := range calibration.Profiles {
		runs := m[profile]
		require.Len(t, runs, 2, profile)
		assert.ElementsMatch(t, []int{0, 1}, []int{runs[0].CPU, runs[1].CPU}, profile)
		for _, r := range runs {
			assert.Greater(t, r.Time, 10*time.Millisecond, profile)
		}
	}
}
