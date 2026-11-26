package sandbox

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Pandoc logs raw blocks it drops as AST nodes, as in EGOI 2025 windturbines.
func TestPandocLogWithObjectContents(t *testing.T) {
	data := `[
		{"type": "BlockNotRendered", "verbosity": "INFO", "contents": {"c": ["html", "<td style=\"zoom: 60%;\">"], "t": "RawBlock"}},
		{"type": "SkippedContent", "verbosity": "INFO", "contents": "\\vspace", "line": 3, "column": 1}
	]`
	var log []PandocMessage
	require.NoError(t, json.Unmarshal([]byte(data), &log))
	require.JSONEq(t, `{"c":["html","<td style=\"zoom: 60%;\">"],"t":"RawBlock"}`, string(log[0].Contents))
	require.Equal(t, PandocContents(`\vspace`), log[1].Contents)
}
