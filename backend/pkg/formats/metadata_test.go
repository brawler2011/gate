package formats_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats"
	"github.com/brawler2011/gate/backend/pkg/formats/icpc"
	"github.com/brawler2011/gate/backend/pkg/formats/polygon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolygonParser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "polygon-parser-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = os.MkdirAll(filepath.Join(tempDir, "statements"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "statements", "en.html"), []byte("<h1>A+B</h1>"), 0o600)
	require.NoError(t, err)

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<problem short-name="APlusB" revision="1">
  <judging>
    <testset name="tests">
      <time-limit>2000</time-limit>
      <memory-limit>268435456</memory-limit>
      <test-count>2</test-count>
      <input-path-pattern>tests/%02d.in</input-path-pattern>
      <answer-path-pattern>tests/%02d.ans</answer-path-pattern>
      <tests>
        <test method="manual" sample="true" />
        <test method="generated" cmd="gen 10" sample="false" />
      </tests>
      <groups>
        <group name="samples" points-policy="complete-group" points="0" />
        <group name="main" points-policy="each-test" points="100">
          <dependencies>
            <dependency group="samples" />
          </dependencies>
        </group>
      </groups>
    </testset>
  </judging>
  <files>
    <executables>
      <executable>
        <source path="checkers/checker.cpp" type="checker" />
        <binary path="checkers/checker" type="checker" />
      </executable>
    </executables>
  </files>
  <statements>
    <statement language="en" charset="utf-8" type="application/x-tex" path="statements/en.html" />
  </statements>
</problem>`

	err = os.WriteFile(filepath.Join(tempDir, "problem.xml"), []byte(xmlContent), 0o600)
	require.NoError(t, err)

	plan, err := polygon.NewParser().Parse(tempDir)
	require.NoError(t, err)

	require.Equal(t, "scoring", plan.Problem.Type)
	assert.Equal(t, 2000, plan.Problem.Limits.TimeMs)
	assert.Equal(t, 256, plan.Problem.Limits.MemoryMb)
	assert.Equal(t, "APlusB", plan.Problem.Title)

	require.NotEmpty(t, plan.Mappings)

	parsedFormat, err := formats.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parsedFormat)

	parser, err := formats.GetParser(parsedFormat)
	require.NoError(t, err)
	assert.Equal(t, "polygon", parser.GetFormat())
}

func TestICPCParser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "icpc-parser-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = os.WriteFile(filepath.Join(tempDir, "problem.yaml"), []byte(`
name: "A + B"
author: "Gate"
source: "Integration"
limits:
  time: 1.5
  memory: 128
  output: 32
  code: 64
validation:
  type: "default"
`), 0o600)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "statement", "en"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "statement", "en", "problem.md"), []byte("# A + B"), 0o600)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "data", "sample"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "sample", "sample1.in"), []byte("1 2\n"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "sample", "sample1.ans"), []byte("3\n"), 0o600)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "data", "secret"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "secret", "test1.in"), []byte("10 20\n"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "data", "secret", "test1.ans"), []byte("30\n"), 0o600)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "output_validators"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "output_validators", "check.cpp"), []byte("int main(){}"), 0o600)
	require.NoError(t, err)

	plan, err := icpc.NewParser().Parse(tempDir)
	require.NoError(t, err)

	assert.Equal(t, "pass-fail", plan.Problem.Type)
	assert.Equal(t, 1500, plan.Problem.Limits.TimeMs)
	assert.Equal(t, 128, plan.Problem.Limits.MemoryMb)
	assert.Equal(t, "A + B", plan.Problem.Title)

	format, err := formats.DetectFormat(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "icpc", format)
}
