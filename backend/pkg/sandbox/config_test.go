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
[python3]
name = "Python 3"
extensions = ["py"]
package = "python314"
type = "interpreter"
code_file = "foo.py"
execute = "/usr/bin/python3 ${dir}/foo.py"
`))
	require.NoError(t, err)
	require.Equal(t, map[string]Language{"python3": {Name: "Python 3", Extensions: []string{"py"}, Type: "interpreter", CodeFile: "foo.py", Execute: "/usr/bin/python3 ${dir}/foo.py"}}, cfg.Langs)
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

func TestLoadConfigRequiresNameAndExtensions(t *testing.T) {
	_, err := LoadConfig(writeLangs(t, `
[python3]
extensions = ["py"]
code_file = "foo.py"
execute = "/usr/bin/python3 foo.py"
`))
	require.ErrorContains(t, err, `language "python3" needs name`)
	_, err = LoadConfig(writeLangs(t, `
[python3]
name = "Python 3"
extensions = [".py"]
code_file = "foo.py"
execute = "/usr/bin/python3 foo.py"
`))
	require.ErrorContains(t, err, `invalid extension ".py"`)
}

func TestLoadConfigRepositoryLangs(t *testing.T) {
	cfg, err := LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	require.Len(t, cfg.Order, len(cfg.Langs))
}

// Configurations sharing an extension: the catalog order, not the
// identifiers, decides which one the extension suggests.
func TestForPathSuggestsFirstConfigurationInCatalogOrder(t *testing.T) {
	cfg, err := LoadConfig(writeLangs(t, `
[cpp20]
name = "C++20"
extensions = ["cpp", "cc"]
code_file = "foo.cc"
execute = "run"

[cpp17]
name = "C++17"
extensions = ["cpp", "cc", "cxx"]
code_file = "foo.cc"
execute = "run"
`))
	require.NoError(t, err)
	require.Equal(t, []string{"cpp20", "cpp17"}, cfg.Order)
	require.Equal(t, "cpp20", cfg.ForPath("solutions/a.CPP"))
	require.Equal(t, "cpp17", cfg.ForPath("lib/a.cxx"))
	require.Empty(t, cfg.ForPath("solutions/a.rs"))
	require.Empty(t, cfg.ForPath("solutions/Makefile"))
}
