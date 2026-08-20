package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// GrafanaDatasourceConfig represents datasources.yaml structure.
type GrafanaDatasourceConfig struct {
	APIVersion  int                 `yaml:"apiVersion"`
	Datasources []GrafanaDatasource `yaml:"datasources"`
}

type GrafanaDatasource struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	Access    string                 `yaml:"access"`
	URL       string                 `yaml:"url"`
	IsDefault bool                   `yaml:"isDefault"`
	UID       string                 `yaml:"uid"`
	JsonData  map[string]interface{} `yaml:"jsonData"`
}

// GrafanaDashboard represents the structure of a Grafana dashboard JSON file.
type GrafanaDashboard struct {
	UID       string                 `json:"uid"`
	Title     string                 `json:"title"`
	Tags      []string               `json:"tags"`
	Timezone  string                 `json:"timezone"`
	SchemaVer int                    `json:"schemaVersion"`
	Panels    []GrafanaPanel         `json:"panels"`
	Templating map[string]interface{} `json:"templating"`
}

type GrafanaPanel struct {
	ID          int                    `json:"id"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"`
	GridPos     GrafanaGridPos         `json:"gridPos"`
	Datasource  interface{}            `json:"datasource"`
	Targets     []GrafanaTarget        `json:"targets"`
	Panels      []GrafanaPanel         `json:"panels"` // Nested panels in row
}

type GrafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

type GrafanaTarget struct {
	Expr         string `json:"expr"`
	RefID        string `json:"refId"`
	LegendFormat string `json:"legendFormat"`
}

// ParseDatasourcesYAML reads and parses Grafana provisioning datasources.yaml.
func ParseDatasourcesYAML(path string) (*GrafanaDatasourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read datasources file %s: %w", path, err)
	}

	var cfg GrafanaDatasourceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse datasources yaml %s: %w", path, err)
	}

	return &cfg, nil
}

// ValidateDatasourcesYAML asserts valid datasources config for VictoriaMetrics, Tempo, and Loki.
func ValidateDatasourcesYAML(t *testing.T, path string) *GrafanaDatasourceConfig {
	t.Helper()
	cfg, err := ParseDatasourcesYAML(path)
	require.NoError(t, err, "Failed to parse Grafana datasources provisioning at %s", path)
	require.Equal(t, 1, cfg.APIVersion, "Grafana datasources apiVersion should be 1")
	require.NotEmpty(t, cfg.Datasources, "No datasources defined in %s", path)

	var hasVM, hasTempo, hasLoki bool
	for _, ds := range cfg.Datasources {
		switch strings.ToLower(ds.Type) {
		case "prometheus":
			if ds.IsDefault || strings.Contains(strings.ToLower(ds.Name), "victoriametrics") {
				hasVM = true
				require.Equal(t, "proxy", ds.Access, "VictoriaMetrics datasource access mode should be proxy")
			}
		case "tempo":
			hasTempo = true
			require.Equal(t, "proxy", ds.Access, "Tempo datasource access mode should be proxy")
			// Validate tracesToLogsV2
			if t2l, ok := ds.JsonData["tracesToLogsV2"].(map[string]interface{}); ok {
				require.NotEmpty(t, t2l["datasourceUid"], "Tempo tracesToLogsV2 must specify datasourceUid")
			}
		case "loki":
			hasLoki = true
			require.Equal(t, "proxy", ds.Access, "Loki datasource access mode should be proxy")
			// Validate derivedFields
			if dfs, ok := ds.JsonData["derivedFields"].([]interface{}); ok {
				require.NotEmpty(t, dfs, "Loki datasource derivedFields should not be empty")
			}
		}
	}

	require.True(t, hasVM, "VictoriaMetrics (Prometheus) datasource missing")
	require.True(t, hasTempo, "Tempo datasource missing")
	require.True(t, hasLoki, "Loki datasource missing")

	return cfg
}

// ParseDashboardJSON reads and parses a Grafana dashboard JSON file.
func ParseDashboardJSON(path string) (*GrafanaDashboard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboard file %s: %w", path, err)
	}

	var dash GrafanaDashboard
	if err := json.Unmarshal(data, &dash); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard JSON %s: %w", path, err)
	}

	return &dash, nil
}

// ValidateDashboardJSON validates dashboard JSON syntax, UID, panel count, grid layout, and PromQL targets.
func ValidateDashboardJSON(t *testing.T, path string, expectedUID string, minPanels int) *GrafanaDashboard {
	t.Helper()
	dash, err := ParseDashboardJSON(path)
	require.NoError(t, err, "Failed to parse Grafana dashboard JSON at %s", path)

	if expectedUID != "" {
		require.Equal(t, expectedUID, dash.UID, "Dashboard UID mismatch in %s", path)
	}
	require.NotEmpty(t, dash.Title, "Dashboard title must not be empty in %s", path)

	allPanels := FlattenPanels(dash.Panels)
	require.GreaterOrEqual(t, len(allPanels), minPanels,
		"Dashboard %s has %d panels, expected at least %d", path, len(allPanels), minPanels)

	for _, p := range allPanels {
		if p.Type == "row" {
			continue
		}
		// Validate grid position constraints: x + w <= 24, h >= 2
		require.True(t, p.GridPos.X >= 0 && p.GridPos.X < 24, "Panel %s has invalid gridPos.x: %d", p.Title, p.GridPos.X)
		require.True(t, p.GridPos.W > 0 && p.GridPos.W <= 24, "Panel %s has invalid gridPos.w: %d", p.Title, p.GridPos.W)
		require.True(t, p.GridPos.X+p.GridPos.W <= 24, "Panel %s exceeds 24-column grid (x=%d, w=%d)", p.Title, p.GridPos.X, p.GridPos.W)
		require.True(t, p.GridPos.H >= 1, "Panel %s has invalid gridPos.h: %d", p.Title, p.GridPos.H)

		// Validate PromQL targets
		for _, target := range p.Targets {
			if target.Expr != "" {
				AssertValidPromQL(t, target.Expr)
			}
		}
	}

	return dash
}

// FlattenPanels flattens nested row panels into a single slice.
func FlattenPanels(panels []GrafanaPanel) []GrafanaPanel {
	var result []GrafanaPanel
	for _, p := range panels {
		result = append(result, p)
		if len(p.Panels) > 0 {
			result = append(result, FlattenPanels(p.Panels)...)
		}
	}
	return result
}

// checkBracketsBalanced verifies that parentheses, brackets, and braces are correctly matched and ordered.
func checkBracketsBalanced(expr string) error {
	var stack []rune
	inQuote := rune(0)

	for i, r := range expr {
		if inQuote != 0 {
			if r == inQuote {
				if i > 0 && rune(expr[i-1]) == '\\' {
					continue
				}
				inQuote = 0
			}
			continue
		}

		if r == '"' || r == '\'' || r == '`' {
			inQuote = r
			continue
		}

		switch r {
		case '(', '[', '{':
			stack = append(stack, r)
		case ')':
			if len(stack) == 0 || stack[len(stack)-1] != '(' {
				return fmt.Errorf("unexpected closing parenthesis ')' without matching '('")
			}
			stack = stack[:len(stack)-1]
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return fmt.Errorf("unexpected closing bracket ']' without matching '['")
			}
			stack = stack[:len(stack)-1]
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return fmt.Errorf("unexpected closing brace '}' without matching '{'")
			}
			stack = stack[:len(stack)-1]
		}
	}

	if inQuote != 0 {
		return fmt.Errorf("unclosed string literal %c", inQuote)
	}

	if len(stack) > 0 {
		return fmt.Errorf("unclosed delimiter '%c'", stack[len(stack)-1])
	}

	return nil
}

// AssertValidPromQL performs static validation on PromQL expressions.
func AssertValidPromQL(t *testing.T, expr string) {
	t.Helper()
	trimmed := strings.TrimSpace(expr)
	require.NotEmpty(t, trimmed, "PromQL expression must not be empty")

	err := checkBracketsBalanced(trimmed)
	require.NoError(t, err, "PromQL expression has unbalanced or inverted delimiters in expression: %s", expr)
}

// ValidateDerivedFieldRegex verifies that a Loki derived field regex accurately captures 32-char hex trace IDs.
func ValidateDerivedFieldRegex(t *testing.T, regexStr string) {
	t.Helper()
	re, err := regexp.Compile(regexStr)
	require.NoError(t, err, "Invalid derived field regex: %s", regexStr)

	testCases := []struct {
		input      string
		expectedID string
	}{
		{
			input:      `level=info msg="sub created" trace_id=4bf92f3577b34da6a3ce929d0e0e4736`,
			expectedID: "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			input:      `{"level":"info","msg":"test","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}`,
			expectedID: "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			input:      `traceparent=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`,
			expectedID: "4bf92f3577b34da6a3ce929d0e0e4736",
		},
	}

	matchCount := 0
	for _, tc := range testCases {
		matches := re.FindStringSubmatch(tc.input)
		if len(matches) > 1 {
			matchCount++
			var captured string
			for i := 1; i < len(matches); i++ {
				if matches[i] != "" {
					captured = matches[i]
					break
				}
			}
			if strings.HasPrefix(captured, "00-") {
				captured = strings.TrimPrefix(captured, "00-")
			}
			require.Contains(t, tc.expectedID, captured, "Captured trace ID mismatch for input: %s", tc.input)
		}
	}

	require.Greater(t, matchCount, 0, "Regex %s did not match any of the standard trace log formats", regexStr)
}
