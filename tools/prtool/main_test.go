package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePRTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"valid feat", "feat(tooling): add pr automation tool", false},
		{"valid fix", "fix: correct typo in taskfinder", false},
		{"valid docs", "docs: update workflow guidelines", false},
		{"valid chore", "chore(ci): update github actions", false},
		{"empty title", "", true},
		{"missing space after colon", "feat(tooling):add something", true},
		{"too long title", "feat(tooling): this is a very long title that exceeds the sixty five characters limit", true},
		{"uppercase scope", "feat(Tooling): invalid scope casing", true},
		{"invalid type", "invalid(tooling): not a conventional type", true},
		{"ends with dot", "feat(tooling): do not end with period.", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePRTitle(tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePRTitle(%q) err = %v, wantErr = %v", tt.title, err, tt.wantErr)
			}
		})
	}
}

func TestExtractTaskID(t *testing.T) {
	cases := map[string]string{
		"task/TASK-002-pr-automation": "TASK-002",
		"fix/TASK-001-setup":          "TASK-001",
		"TASK-999":                    "TASK-999",
		"feature/no-task-id":          "",
		"random-branch-name":          "",
	}

	for branch, want := range cases {
		got := ExtractTaskID(branch)
		if got != want {
			t.Errorf("ExtractTaskID(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestRenderPRBody(t *testing.T) {
	template := `## Task
Resolves / Task: <!-- e.g. .tasks/TASK-002-pr-automation-and-review-flow.md -->

## Description
<!-- Concise summary of changes and non-obvious rationale. Keep focused. -->

## Type of Change
- [ ] ` + "`feat`" + `: New feature
- [ ] ` + "`fix`" + `: Bug fix
- [ ] ` + "`chore`" + `: Maintenance, tooling, or workflow update
`

	meta := &TaskMeta{
		ID:          "TASK-002",
		Title:       "PR automation and reviewer workflow",
		Type:        "feat",
		Description: "Implement PR automation tool.",
		FilePath:    ".tasks/TASK-002-pr-automation-and-review-flow.md",
	}

	rendered := RenderPRBody(template, meta, "Custom body text.")

	if !strings.Contains(rendered, "TASK-002") {
		t.Errorf("expected TASK-002 in rendered body, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Implement PR automation tool.") {
		t.Errorf("expected description in rendered body, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Custom body text.") {
		t.Errorf("expected custom body in rendered body, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "- [x] `feat`:") {
		t.Errorf("expected feat to be checked, got:\n%s", rendered)
	}
}

func TestFindTaskFileAndParse(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot failed: %v", err)
	}

	taskPath, err := FindTaskFile(repoRoot, "TASK-001")
	if err != nil {
		t.Fatalf("FindTaskFile(TASK-001) failed: %v", err)
	}

	if !strings.Contains(taskPath, "TASK-001") {
		t.Errorf("unexpected taskPath: %s", taskPath)
	}

	meta, err := ParseTaskMeta(filepath.Join(repoRoot, taskPath))
	if err != nil {
		t.Fatalf("ParseTaskMeta failed: %v", err)
	}

	if meta.ID != "TASK-001" {
		t.Errorf("expected ID 'TASK-001', got %q", meta.ID)
	}
	if meta.Type != "chore" {
		t.Errorf("expected Type 'chore', got %q", meta.Type)
	}
}

func TestValidatePRTitle_ScopeCasingMessage(t *testing.T) {
	err := ValidatePRTitle("feat(Tooling): uppercase scope")
	if err == nil {
		t.Fatalf("expected error for uppercase scope, got nil")
	}
	if !strings.Contains(err.Error(), "must be lowercase") {
		t.Errorf("expected error message to contain 'must be lowercase', got: %v", err)
	}
}

func TestRenderPRBody_BreakingChangesAndVerification(t *testing.T) {
	template := `## Breaking Changes
- [ ] Yes (breaking change permitted by default)
- [ ] No (backward compatible)

## Verification Plan
<!-- Outline tests run and verification results (e.g., ` + "`task precommit`" + `, unit tests) -->
`

	// 1. Default (breaking allowed by default)
	metaDefault := &TaskMeta{
		ID:       "TASK-100",
		FilePath: ".tasks/TASK-100.md",
		Tags:     []string{"tooling"},
	}
	res1 := RenderPRBody(template, metaDefault, "")
	if !strings.Contains(res1, "- [x] Yes (breaking change permitted by default)") {
		t.Errorf("expected default to mark breaking change permitted, got:\n%s", res1)
	}
	if strings.Contains(res1, "- [x] No (backward compatible)") {
		t.Errorf("expected backward compatible NOT to be checked, got:\n%s", res1)
	}
	if !strings.Contains(res1, "- [x] Automated pre-commit verification passed (`task precommit`)") {
		t.Errorf("expected verification plan auto-filled, got:\n%s", res1)
	}

	// 2. Backward compatible tag
	metaCompat := &TaskMeta{
		ID:       "TASK-101",
		FilePath: ".tasks/TASK-101.md",
		Tags:     []string{"tooling", "backward-compatible"},
	}
	res2 := RenderPRBody(template, metaCompat, "")
	if !strings.Contains(res2, "- [x] No (backward compatible)") {
		t.Errorf("expected backward-compatible tag to check No, got:\n%s", res2)
	}
	if strings.Contains(res2, "- [x] Yes (breaking change permitted by default)") {
		t.Errorf("expected breaking change permitted NOT to be checked, got:\n%s", res2)
	}
}

func TestParseTaskMeta_RelativePathGuaranteed(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot failed: %v", err)
	}

	absPath := filepath.Join(repoRoot, ".tasks", "TASK-001-setup-lefthook-and-task-validator.md")
	meta, err := ParseTaskMeta(absPath)
	if err != nil {
		t.Fatalf("ParseTaskMeta failed: %v", err)
	}

	if filepath.IsAbs(meta.FilePath) {
		t.Errorf("expected relative FilePath, got absolute: %s", meta.FilePath)
	}
	if !strings.HasPrefix(meta.FilePath, ".tasks") {
		t.Errorf("expected FilePath to start with .tasks, got: %s", meta.FilePath)
	}
}

func TestParseTaskMeta_Tags(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot failed: %v", err)
	}

	meta, err := ParseTaskMeta(filepath.Join(repoRoot, ".tasks", "TASK-003-optimize-pr-and-review-workflow.md"))
	if err != nil {
		t.Fatalf("ParseTaskMeta failed: %v", err)
	}

	if !meta.HasTag("tooling") {
		t.Errorf("expected tag 'tooling' in meta.Tags: %v", meta.Tags)
	}
	if !meta.HasTag("workflow") {
		t.Errorf("expected tag 'workflow' in meta.Tags: %v", meta.Tags)
	}
}
