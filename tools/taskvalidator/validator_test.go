package main

import (
	"testing"
)

func TestValidator_Valid(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	validContent := `---
id: TASK-001
title: "Implement Lefthook integration"
status: todo
type: feat
description: "Integrate lefthook for automated pre-commit task checks"
priority: high
created_at: 2026-09-04
tags:
  - tooling
  - git
---

# TASK-001: Implement Lefthook integration

## Context
We need to ensure all task specifications are validated before commit.

## Acceptance Criteria
- [ ] Install lefthook
- [x] Configure lefthook.yml
`

	if err := v.ValidateContent("TASK-001-lefthook.md", []byte(validContent)); err != nil {
		t.Errorf("expected valid content, got error: %v", err)
	}
}

func TestValidator_TemplateIgnored(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	templateContent := `---
id: TASK-XXX
title: "[Type] Short title"
status: draft
---
Template body with no criteria
`

	if err := v.ValidateContent("TEMPLATE.md", []byte(templateContent)); err != nil {
		t.Errorf("expected template file to be ignored, got error: %v", err)
	}
}

func TestValidator_InvalidSchema(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		file    string
		content string
	}{
		{
			name: "missing required title",
			file: "TASK-001.md",
			content: `---
id: TASK-001
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "invalid id pattern",
			file: "TASK-1.md",
			content: `---
id: TASK-1
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "invalid status enum",
			file: "TASK-001.md",
			content: `---
id: TASK-001
title: "Valid title"
status: unknown_status
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "deprecated ready status",
			file: "TASK-001.md",
			content: `---
id: TASK-001
title: "Valid title"
status: ready
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "deprecated in_progress status",
			file: "TASK-001.md",
			content: `---
id: TASK-001
title: "Valid title"
status: in_progress
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "deprecated review status",
			file: "TASK-001.md",
			content: `---
id: TASK-001
title: "Valid title"
status: review
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "short description",
			file: "TASK-001.md",
			content: `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Too short"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateContent(tc.file, []byte(tc.content))
			if err == nil {
				t.Errorf("expected error for case %q, but validation succeeded", tc.name)
			}
		})
	}
}

func TestValidator_FilenameMismatch(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	content := `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some context

## Acceptance Criteria
- [ ] Criterion
`

	err = v.ValidateContent("TASK-002-feature.md", []byte(content))
	if err == nil {
		t.Errorf("expected error due to ID mismatch between filename and frontmatter, got nil")
	}
}

func TestValidator_MissingSections(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "missing context",
			content: `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "empty context",
			content: `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context

## Acceptance Criteria
- [ ] Criterion
`,
		},
		{
			name: "missing criteria",
			content: `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some valid context.
`,
		},
		{
			name: "criteria without checkboxes",
			content: `---
id: TASK-001
title: "Valid title"
status: todo
type: feat
description: "Valid description here"
created_at: 2026-09-04
---

## Context
Some valid context.

## Acceptance Criteria
Just some text without check boxes
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateContent("TASK-001.md", []byte(tc.content))
			if err == nil {
				t.Errorf("expected error for case %q, but validation succeeded", tc.name)
			}
		})
	}
}

func TestValidator_DoneStatusAndLifecycle(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	t.Run("valid done task with all criteria completed", func(t *testing.T) {
		content := `---
id: TASK-001
title: "Valid completed task"
status: done
type: feat
description: "Everything in this task was verified and completed"
created_at: 2026-09-04
---

## Context
Task context here.

## Acceptance Criteria
- [x] Criterion 1
- [X] Criterion 2

## Implementation Notes
Notes go here.
`
		if err := v.ValidateContent("TASK-001.md", []byte(content)); err != nil {
			t.Errorf("expected valid done task, got error: %v", err)
		}
	})

	t.Run("invalid done task with unchecked criteria", func(t *testing.T) {
		content := `---
id: TASK-001
title: "Incomplete done task"
status: done
type: feat
description: "Task marked done but has unfinished items"
created_at: 2026-09-04
---

## Context
Task context here.

## Acceptance Criteria
- [x] Done item
- [ ] Still incomplete item
`
		err := v.ValidateContent("TASK-001.md", []byte(content))
		if err == nil {
			t.Errorf("expected error for done task with unchecked item, got nil")
		}
	})

	t.Run("valid todo task with unchecked criteria", func(t *testing.T) {
		content := `---
id: TASK-001
title: "Todo task"
status: todo
type: feat
description: "Task ready to be worked on"
created_at: 2026-09-04
---

## Context
Task context here.

## Acceptance Criteria
- [ ] Not yet started
- [x] Pre-completed requirement
`
		if err := v.ValidateContent("TASK-001.md", []byte(content)); err != nil {
			t.Errorf("expected valid todo task, got error: %v", err)
		}
	})

	t.Run("valid draft task", func(t *testing.T) {
		content := `---
id: TASK-001
title: "Draft task"
status: draft
type: feat
description: "Draft task undergoing discussion"
created_at: 2026-09-04
---

## Context
Initial draft idea.

## Acceptance Criteria
- [ ] Preliminary idea
`
		if err := v.ValidateContent("TASK-001.md", []byte(content)); err != nil {
			t.Errorf("expected valid draft task, got error: %v", err)
		}
	})
}

