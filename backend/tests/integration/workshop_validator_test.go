//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkshopValidatorExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	sb := newWorkshopSandbox(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name         string
		source       string
		path         string
		input        string
		files        map[string]string
		deps         []models.Dependency
		compileFirst bool
		valid        bool
		wantError    string
		wantInvalid  string
	}{
		{name: "accepts stdin", source: "#include <iostream>\nint main(){int x; return (std::cin >> x) && x == 42 ? 0 : 1;}", input: "42\n", valid: true},
		{name: "rejects existing input", source: "#include <cstdio>\nint main(){std::fputs(\"validator_rejected_input\", stderr); return 1;}", input: "42\n", wantInvalid: "validator_rejected_input"},
		{name: "rejects malformed input", source: "#include <iostream>\nint main(){int x; return std::cin >> x ? 0 : 1;}", input: "invalid\n", wantInvalid: "exit code=1"},
		{name: "missing input", source: "int main(){return 0;}", wantInvalid: "test file not found"},
		{name: "compiler diagnostics", source: "#error validator_regression_diagnostic", input: "42\n", wantError: "validator_regression_diagnostic"},
		{name: "runtime failure", source: "#include <cstdlib>\nint main(){std::abort();}", input: "42\n", wantInvalid: "status=RE"},
		{name: "time limit", source: "int main(){for(;;){}}", input: "42\n", wantInvalid: "status=TLE"},
		{name: "interpreted validator", path: "validators/validator.py", source: "import sys\nsys.exit(0 if sys.stdin.read().strip() == '42' else 1)\n", input: "42\n", valid: true},
		{name: "relative dependency", source: "#include \"validation.h\"\nint main(){return VALIDATOR_EXIT;}", input: "42\n", deps: []models.Dependency{{Filename: "validation.h"}}, files: map[string]string{"validators/validation.h": "#define VALIDATOR_EXIT 0", "lib/validation.h": "#define VALIDATOR_EXIT 1"}, valid: true},
		{name: "shared dependency", source: "#include \"validation.h\"\nint main(){return VALIDATOR_EXIT;}", input: "42\n", deps: []models.Dependency{{Filename: "validation.h"}}, files: map[string]string{"lib/validation.h": "#define VALIDATOR_EXIT 1"}, wantInvalid: "exit code=1"},
		{name: "compiled nested dependency", source: "#include \"validation.h\"\nint main(){return VALIDATOR_EXIT;}", input: "42\n", files: map[string]string{"lib/nested/validation.h": "#define VALIDATOR_EXIT 0"}, compileFirst: true, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			path := tc.path
			if path == "" {
				path = "validators/validator.cpp"
			}
			repo := newMockProblemsRepo()
			manifest := models.ProblemManifest{
				TimeLimitMs:   1, // Validator budget must be independent of solution limits.
				MemoryLimitMb: 1,
				FilesMetadata: []models.FileMetadata{{Type: "validator", Filename: path, Dependencies: tc.deps}},
			}
			data, err := json.Marshal(manifest)
			require.NoError(t, err)
			repo.manifests[id] = data
			ws := usecase.NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "validator-execution")
			require.NoError(t, ws.WriteFile(ctx, id, "problem.yaml", []byte("subtasks:\n  samples:\n    tests:\n      - manual: 01.in\n")))
			require.NoError(t, ws.WriteFile(ctx, id, path, []byte(tc.source)))
			if tc.input != "" {
				require.NoError(t, ws.WriteFile(ctx, id, "tests/01.in", []byte(tc.input)))
			}
			for path, content := range tc.files {
				require.NoError(t, ws.WriteFile(ctx, id, path, []byte(content)))
			}
			uc := usecase.NewWorkshopUseCase(repo, ws, sb, &mockTxManager{})
			if tc.compileFirst {
				compiled, err := uc.CompileProblemComponent(ctx, models.CompileComponentRequest{ProblemID: id, ComponentType: "validator"})
				require.NoError(t, err)
				require.True(t, compiled.Success, compiled.CompileError)
			}
			report, err := uc.ValidateAllTests(ctx, id, uuid.Nil)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				require.Nil(t, report)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, report.TotalTests)
			require.Len(t, report.Results, 1)
			require.Equal(t, tc.valid, report.Results[0].Valid)
			if tc.valid {
				require.Equal(t, 1, report.ValidTests)
				require.Zero(t, report.InvalidTests)
			} else {
				require.Equal(t, 1, report.InvalidTests)
				require.Zero(t, report.ValidTests)
				require.Contains(t, report.Results[0].Error, tc.wantInvalid)
			}
		})
	}
}
