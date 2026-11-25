package sandbox

import (
	"os"
	"path/filepath"
	"slices"
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

// Each grader-capable language needs its own entry grader or linked source, and Python and Java also the solution name.
func TestGradedLanguagesOfRepositoryCatalog(t *testing.T) {
	cfg, err := LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	cpp := []string{"cpp23", "cpp20", "cpp17"}
	all := []string{"grader.cpp", "grader.h", "grader.py", "grader.java"}
	for _, tc := range []struct {
		name     string
		graders  []string
		solution string
		want     []string
	}{
		{"c++ only", []string{"grader.cpp", "grader.h"}, "aplusb", cpp},
		{"every grader", all, "aplusb", append(slices.Clone(cpp), "python3", "pypy3", "java")},
		{"no solution name", all, "", cpp},
		{"invalid solution name", all, "a-b", cpp},
		{"reserved solution name", all, "class", cpp},
		{"python without entry", []string{"lib.py"}, "aplusb", []string{}},
		{"solution named like the entry", []string{"grader.py"}, "grader", []string{}},
		{"solution named like the interpreted file", []string{"grader.py"}, "foo", []string{}},
		{"grader named like the solution", []string{"grader.py", "aplusb.py"}, "aplusb", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, cfg.Graded(tc.graders, tc.solution))
		})
	}
}

// Extension suggestions follow catalog order rather than identifier order.
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
