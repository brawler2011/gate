package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeLangs(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langs.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadConfigIgnoresPackage(t *testing.T) {
	cfg, err := LoadConfig(writeLangs(t, `
[python]
package = "python314"
type = "interpreter"
code_file = "foo.py"
execute = "/usr/bin/python3 ${dir}/foo.py"
`))
	require.NoError(t, err)
	require.Equal(t, map[string]Language{"python": {Type: "interpreter", CodeFile: "foo.py", Execute: "/usr/bin/python3 ${dir}/foo.py"}}, cfg.Langs)
}

func TestLoadConfigRejectsUnknownKey(t *testing.T) {
	_, err := LoadConfig(writeLangs(t, `
[python]
type = "interpreter"
exec = "/usr/bin/python3"
`))
	require.ErrorContains(t, err, "python.exec")
}

func TestLoadConfigRejectsEmpty(t *testing.T) {
	_, err := LoadConfig(writeLangs(t, ""))
	require.Error(t, err)
}

func TestLoadConfigRepositoryLangs(t *testing.T) {
	cfg, err := LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"cpp", "python", "go", "java"}, keys(cfg.Langs))
}

func keys(m map[string]Language) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
