---
id: TASK-019
title: "Require checker for workshop solution evaluation"
status: done
type: fix
description: "Require a working checker for workshop solution evaluation and report checker preparation errors."
priority: high
created_at: 2026-10-11
tags:
  - backend
---

# TASK-019: Workshop checker errors

## Context
Workshop TestSolution ignores checker source, dependency and compilation errors.
A configured checker can therefore be replaced silently by text comparison,
producing an incorrect verdict. Every solution evaluation must use a checker;
remove text comparison entirely. Fail before solution execution when the checker
is missing or its preparation fails.
CompileProblemComponent records shared lib dependencies by basename; TestSolution
must also find those dependencies, while preserving checker-relative files.
Start from origin/main including PR #13. Preserve untracked CODEBASE_REVIEW.md.

## Acceptance Criteria
- [x] Checker source and dependency read failures return contextual errors with the original cause and no test report.
- [x] Checker compilation failures return contextual errors preserving diagnostics and cancellation status; solution execution does not proceed.
- [x] Checker-relative dependencies and basename dependencies recorded from lib/ remain usable; storage errors other than not-found are not retried at an alternative path.
- [x] A working configured checker determines OK and WA verdicts even when its decision differs from text comparison; checker failure cannot produce a successful result.
- [x] Workspaces without a checker return an explicit error and no test report; text comparison is removed.
- [x] Regression unit tests and real go-judge workshop integration tests pass; task tasks:validate and task precommit pass.

## Implementation Notes
Limit changes to checker preparation in TestSolution and its regression tests.
Validator execution, manifest persistence, test identity and package publication
remain separate tasks. No API contract or frontend changes are needed.

## Delivery
Create the PR, obtain isolated review for the exact SHA and green CI; leave it unmerged.

## Verification
Regression unit tests reproduced ignored checker errors before the fix.
Nine unit scenarios pass, covering preparation failures, dependency locations,
cancellation/deadline status and missing checker. The full backend integration
suite passed, including the existing workshop workflow and six new real go-judge
checker scenarios. task tasks:validate and task precommit passed.
