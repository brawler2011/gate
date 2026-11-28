//go:build integration

package postgres_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/testcontainers/testcontainers-go"
)

func TestMain(m *testing.M) {
	code := m.Run()
	dbfixtures.RemovePrograms()
	// SeaweedFS ignores SIGTERM, so graceful termination waits for the full Docker timeout.
	for _, c := range shared.containers {
		if err := c.Terminate(context.Background(), testcontainers.StopTimeout(0)); err != nil {
			slog.Error("terminate shared container", "error", err)
		}
	}
	os.Exit(code)
}
