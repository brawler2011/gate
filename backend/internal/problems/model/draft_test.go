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
				ID: uuid.New(), Role: "checker", Path: "checkers/check.cpp",
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
			d.Components = append(d.Components, DraftComponent{ID: uuid.New(), Role: "checker", Path: "checkers/check.cpp"})
		}, "duplicate component"},
		"dependency outside lib": {func(d *Draft) {
			d.Components[0].Dependencies = map[string]string{"en.md": "statements/en.md"}
		}, `"statements/en.md" is outside lib/`},
		"unknown source extension": {func(d *Draft) {
			d.Files["checkers/check.rs"] = ""
			delete(d.Files, "checkers/check.cpp")
			d.Components[0].Path = "checkers/check.rs"
		}, `"checkers/check.rs" has an unknown source extension`},
		"asset outside assets": {func(d *Draft) { d.Assets[0].Path = "images/pic.png" }, `"images/pic.png" is outside assets/`},
	} {
		t.Run(name, func(t *testing.T) {
			d := draft()
			tc.change(&d)
			require.ErrorContains(t, d.Validate(quotas, false), tc.err)
		})
	}
}

func TestDraftSingletonRoles(t *testing.T) {
	quotas := DraftQuotas{FileBytes: 1 << 20, TextBytes: 1 << 20, Files: 100, Tests: 100, Components: 100, Subtasks: 100}
	for role, folder := range map[string]string{"checker": "checkers", "validator": "validators", "interactor": "interactors"} {
		t.Run(role, func(t *testing.T) {
			d := Draft{ProblemID: uuid.New(), Revision: 1, Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256, Files: map[string]string{}}
			add := func(path string) {
				d.Files[path] = ""
				d.Components = append(d.Components, DraftComponent{ID: uuid.New(), Role: role, Path: path})
			}
			add(folder + "/a.cpp")
			require.NoError(t, d.Validate(quotas, false))
			require.Equal(t, d.Components[0].ID, d.Component(role).ID)
			add(folder + "/b.py")
			require.ErrorContains(t, d.Validate(quotas, false), folder+"/ holds more than one "+role)
		})
	}
}

func TestDraftLanguageFromExtension(t *testing.T) {
	for path, language := range map[string]string{
		"checkers/a.cpp": "cpp", "checkers/a.CC": "cpp", "checkers/a.py": "python", "checkers/a.go": "go", "checkers/a.java": "java", "checkers/a.rs": "", "checkers/a": "",
	} {
		require.Equal(t, language, LanguageForPath(path), path)
	}
}
