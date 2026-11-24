package dbfixtures

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var typeset struct {
	sync.Mutex
	client *sandbox.Client
}

// Typesetter shares one typeset container per test binary; Ryuk removes it.
func Typesetter(t testing.TB) *sandbox.Typesetter {
	t.Helper()
	typeset.Lock()
	defer typeset.Unlock()
	if typeset.client == nil {
		image, err := judgeimage.TypesetRef()
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{Started: true, ContainerRequest: testcontainers.ContainerRequest{
			Image: image, ExposedPorts: []string{"5050/tcp"}, Privileged: true,
			Cmd:        []string{"-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files"},
			WaitingFor: wait.ForHTTP("/version").WithPort("5050/tcp").WithStartupTimeout(2 * time.Minute)}})
		if err != nil {
			t.Fatal(err)
		}
		endpoint, err := c.PortEndpoint(ctx, "5050/tcp", "http")
		if err != nil {
			t.Fatal(err)
		}
		if typeset.client, err = sandbox.NewClient(endpoint, &http.Client{}); err != nil {
			t.Fatal(err)
		}
	}
	return sandbox.NewTypesetter(typeset.client, BuildLimits(t).Typeset)
}
