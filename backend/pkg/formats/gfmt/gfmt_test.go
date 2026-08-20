package gfmt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTestInputAndOutput(t *testing.T) {
	tempDir := t.TempDir()
	testsDir := filepath.Join(tempDir, "tests")
	require.NoError(t, os.MkdirAll(testsDir, 0755))

	// Create test files
	// Test 01: has .in and .out
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "01.in"), []byte("1 2\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "01.out"), []byte("3\n"), 0600))

	// Test 02: has .in and .ans
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "02.in"), []byte("3 4\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "02.ans"), []byte("7\n"), 0600))

	// Test 03: has only .in (no answer file)
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "03.in"), []byte("5 6\n"), 0600))

	// Test 04: has .in and .a
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "04.in"), []byte("10 20\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "04.a"), []byte("30\n"), 0600))

	g := &GateFormat{Path: tempDir}

	t.Run("GetTestInput with .in extension", func(t *testing.T) {
		data, err := g.GetTestInput("01.in")
		require.NoError(t, err)
		assert.Equal(t, "1 2\n", string(data))
	})

	t.Run("GetTestInput without extension", func(t *testing.T) {
		data, err := g.GetTestInput("01")
		require.NoError(t, err)
		assert.Equal(t, "1 2\n", string(data))
	})

	t.Run("GetTestOutput with .in extension resolves .out", func(t *testing.T) {
		data, err := g.GetTestOutput("01.in")
		require.NoError(t, err)
		assert.Equal(t, "3\n", string(data))
	})

	t.Run("GetTestOutput without extension resolves .out", func(t *testing.T) {
		data, err := g.GetTestOutput("01")
		require.NoError(t, err)
		assert.Equal(t, "3\n", string(data))
	})

	t.Run("GetTestOutput with .in extension resolves .ans", func(t *testing.T) {
		data, err := g.GetTestOutput("02.in")
		require.NoError(t, err)
		assert.Equal(t, "7\n", string(data))
	})

	t.Run("GetTestOutput with .in extension resolves .a", func(t *testing.T) {
		data, err := g.GetTestOutput("04.in")
		require.NoError(t, err)
		assert.Equal(t, "30\n", string(data))
	})

	t.Run("GetTestOutput fails when no answer file exists", func(t *testing.T) {
		data, err := g.GetTestOutput("03.in")
		require.Error(t, err)
		assert.Nil(t, data)
	})

	t.Run("GetTestOutput fails for nonexistent test", func(t *testing.T) {
		data, err := g.GetTestOutput("99.in")
		require.Error(t, err)
		assert.Nil(t, data)
	})
}
