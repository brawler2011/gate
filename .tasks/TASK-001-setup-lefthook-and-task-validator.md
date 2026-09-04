---
id: TASK-001
title: "Setup Lefthook pre-commit and markdown task validator"
status: done
type: chore
description: "Integrate Lefthook pre-commit hooks and Go-based markdown task specification validator."
priority: high
created_at: 2026-09-04
tags:
  - tooling
  - git-hooks
  - workflow
---

# TASK-001: Setup Lefthook pre-commit and markdown task validator

## Context
To maintain high engineering standards and seamless pair-programming / agentic development, tasks need to follow a strict markdown format with verifiable frontmatter schema and required context and acceptance criteria sections. Lefthook ensures that all modified task definitions and code comply with quality checks before every git commit.

## Acceptance Criteria
- [x] Go module `tools/taskvalidator` validates frontmatter against embedded JSON schema
- [x] File naming convention `TASK-NNN-slug.md` enforced with matching frontmatter `id`
- [x] Required markdown sections `## Context` and `## Acceptance Criteria` verified with checkbox items
- [x] Unit tests cover positive and negative validation scenarios
- [x] Task template `.tasks/TEMPLATE.md` created
- [x] Lefthook configured at repo root with fast staged checks
- [x] `Taskfile.yml` updated with `tasks:validate` and `hooks:install`

## Implementation Notes
The validator runs via `task tasks:validate` and is executed automatically by Lefthook whenever `.tasks/*.md` files are staged.
