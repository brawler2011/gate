package problemsmodel

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDraftLayout(t *testing.T) {
	quotas := DraftQuotas{FileBytes: 1 << 20, TextBytes: 1 << 20, Files: 100, Tests: 100, Components: 100, Subtasks: 100}
	draft := func() Draft {
		return Draft{
			ProblemID: uuid.New(), Revision: 1, Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256,
			Files: map[string]string{"checkers/check.cpp": "", "lib/testlib.h": "", "statements/en.md": ""},
			Components: []DraftComponent{{
				ID: uuid.New(), Role: "checker", Path: "checkers/check.cpp", Language: "cpp",
				Dependencies: map[string]string{"testlib.h": "lib/testlib.h"},
			}},
			Assets: []DraftAsset{{Path: "assets/pic.png", ObjectID: uuid.New()}},
		}
	}
	valid := draft()
	require.NoError(t, valid.Validate(quotas, false))

	for name, tc := range map[string]struct {
		change func(*Draft)
		err    string
	}{
		"file in the root":              {func(d *Draft) { d.Files["notes.txt"] = "" }, `"notes.txt" is outside`},
		"file in an unknown folder":     {func(d *Draft) { d.Files["libs/x.h"] = "" }, `"libs/x.h" is outside`},
		"role folder file without role": {func(d *Draft) { d.Files["checkers/old.cpp"] = "" }, `"checkers/old.cpp" is not a component source`},
		"component outside its folder": {func(d *Draft) {
			d.Files["solutions/check.cpp"] = ""
			d.Components[0].Path = "solutions/check.cpp"
		}, `"solutions/check.cpp" is outside checkers/`},
		"two components share a source": {func(d *Draft) {
			d.Components = append(d.Components, DraftComponent{ID: uuid.New(), Role: "checker", Path: "checkers/check.cpp", Language: "cpp"})
		}, "duplicate component"},
		"dependency outside lib": {func(d *Draft) {
			d.Components[0].Dependencies = map[string]string{"en.md": "statements/en.md"}
		}, `"statements/en.md" is outside lib/`},
		"asset outside assets": {func(d *Draft) { d.Assets[0].Path = "images/pic.png" }, `"images/pic.png" is outside assets/`},
	} {
		t.Run(name, func(t *testing.T) {
			d := draft()
			tc.change(&d)
			require.ErrorContains(t, d.Validate(quotas, false), tc.err)
		})
	}
}
