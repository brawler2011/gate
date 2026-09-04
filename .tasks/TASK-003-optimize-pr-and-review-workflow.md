---
id: TASK-003
title: "Optimize PR template and review workflow"
status: done
type: refactor
description: "Deduplicate automated checks in PR template and code review prompt, prevent path leakage, and automate breaking changes selection."
priority: normal
created_at: 2026-09-04
tags:
  - tooling
  - workflow
  - git
  - review
---

# TASK-003: Optimize PR template and review workflow

## Context
Initial implementation of PR automation and adversarial review workflow included redundant checks that duplicate automated CI and CLI gates (such as `task precommit`, conventional commit validation, and task completion validation). In addition, local absolute paths could leak into PR descriptions, and breaking changes flags required manual formatting. This task optimizes the workflow by delegating mechanical checks entirely to automated tooling and CI, keeping human and LLM review focused on code semantics, architectural compliance, and edge cases.

## Acceptance Criteria
- [x] Redundant automated checks (`task precommit`, title validation, task criteria completion) removed from `.github/pull_request_template.md`
- [x] Redundant pre-commit / conventional commits check removed from Reviewer Agent checklist prompt in `tools/prtool`
- [x] Task file path in `tools/prtool` guaranteed to be repository-relative (`.tasks/...`) regardless of input path
- [x] `TaskMeta` parses tags and `RenderPRBody` auto-selects Breaking Changes status based on `backward compatible` tags
- [x] `RenderPRBody` populates `Verification Plan` with pre-commit confirmation
- [x] Unit tests for `tools/prtool` updated and covering new behavior
- [x] `task tasks:validate` and `task precommit` pass cleanly

## Implementation Notes
Focus code review on semantic correctness, surgical changes, and AGENTS.md architectural rules (slog, no symlinks, no server actions, no env fallback). Let CI and tooling enforce mechanical gates.
