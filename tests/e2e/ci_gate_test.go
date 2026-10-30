package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Execute the workflow's real aggregate script rather than copying its logic.
func TestCIRequiredGate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(helpers.FindRepoRoot(), ".github", "workflows", "ci.yml"))
	require.NoError(t, err)
	var workflow struct {
		Jobs map[string]struct {
			Needs []string `yaml:"needs"`
			If    string   `yaml:"if"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &workflow))
	gate, ok := workflow.Jobs["ci-status"]
	require.True(t, ok)
	require.Equal(t, "always()", gate.If, "gate must run when dependencies fail")
	var checks []string
	for name := range workflow.Jobs {
		if name != "ci-status" {
			checks = append(checks, name)
		}
	}
	require.NotEmpty(t, checks)
	require.ElementsMatch(t, checks, gate.Needs, "every CI job must gate success")
	require.Len(t, gate.Steps, 1)
	script := gate.Steps[0].Run
	require.NotEmpty(t, script)
	run := func(t *testing.T, failedJob, result string) {
		t.Helper()
		rendered := script
		for _, name := range checks {
			status := "success"
			if name == failedJob {
				status = result
			}
			rendered = strings.ReplaceAll(rendered, "${{ needs."+name+".result }}", status)
		}
		require.NotContains(t, rendered, "${{", "unresolved workflow expression")
		output, err := exec.Command("bash", "-e", "-o", "pipefail", "-c", rendered).CombinedOutput()
		if failedJob == "" {
			require.NoError(t, err, "%s", output)
		} else {
			require.Error(t, err, "gate accepted %s=%s: %s", failedJob, result, output)
		}
	}
	t.Run("all-success", func(t *testing.T) { run(t, "", "") })
	for _, name := range checks {
		for _, result := range []string{"failure", "cancelled", "skipped"} {
			t.Run(name+"/"+result, func(t *testing.T) { run(t, name, result) })
		}
	}
}
