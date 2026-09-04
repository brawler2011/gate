---
id: TASK-002
title: "Agentic PR automation and code review workflow"
status: done
type: feat
description: "Implement PR creation automation tool, standardized PR template, and adversarial code review workflow."
priority: high
created_at: 2026-09-04
tags:
  - tooling
  - workflow
  - git
  - review
---

# TASK-002: Agentic PR automation and code review workflow

## Context
To improve code quality, streamline pair programming with AI agents, and minimize token usage and hallucinations, the repository needs an automated, reproducible workflow for creating pull requests, enforcing pre-commit quality gates, and conducting independent code reviews.

## Acceptance Criteria
- [x] Pull request template `.github/pull_request_template.md` created with AGENTS.md compliance checklist
- [x] Go CLI tool `tools/prtool` implemented supporting `create` and `review-prompt` subcommands
- [x] `prtool create` verifies non-main branch, associates task in `.tasks/`, validates title via Conventional Commits (<=65 chars), and executes pre-commit gates before creating PR
- [x] `Taskfile.yml` updated with `pr:create`, `pr:review-prompt`, and `pr:status`
- [x] Agent workflow guidelines and Reviewer Agent responsibilities documented in `AGENTS.md`
- [x] Unit tests for `tools/prtool` passing cleanly
- [x] `task precommit` passes cleanly

## Implementation Notes
`tools/prtool` provides programmatic automation around `gh pr create` and `task precommit`, ensuring that every pull request is bound to a validated specification in `.tasks/` and respects repository rules.
