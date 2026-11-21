package dbfixtures

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/brawler2011/gate/backend/config"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
)

// BuildLimits are the limits of deploy/local/config.yaml, which tests share with local runs.
func BuildLimits(t testing.TB) model.BuildLimits {
	t.Helper()
	_, self, _, _ := runtime.Caller(0)
	cfg, err := config.Read(filepath.Join(filepath.Dir(self), "..", "..", "..", "deploy", "local", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Problems.Limits
}
