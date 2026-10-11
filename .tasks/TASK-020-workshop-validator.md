---
id: TASK-020
title: "Execute validators during workshop test validation"
status: done
type: fix
description: "Replace workshop file-existence checks with actual sandboxed validator execution."
priority: high
created_at: 2026-10-11
tags:
  - backend
---

# TASK-020: Workshop validator execution

## Context
ValidateAllTests currently reports existing input files as valid without executing
the configured validator, and returns an empty successful report without one.
Compile the configured validator once, then run it with each input on stdin.
Use the existing sandbox language configuration and checker dependency conventions.
Start from origin/main including PR #14. Preserve untracked CODEBASE_REVIEW.md.

## Acceptance Criteria
- [x] Missing validator, source/dependency failures and compiler diagnostics return contextual errors with no successful report, preserving original causes and cancellation.
- [x] Validator-relative and shared lib/ dependencies, including nested basename dependencies recorded by component compilation, work without masking storage failures.
- [x] Each declared test is executed with its actual input; exit zero with OK status is valid, rejection/runtime limits are invalid with diagnostics; absent input is invalid and storage/dispatch failures abort the report.
- [x] Manual input names are preserved during validation; generated inputs use the existing numbered workshop files. Test ordinals consistently use samples first, then other groups by name, matching judge ordering.
- [x] Unit regression tests and real go-judge integration tests cover acceptance, rejection, malformed/absent input, preparation failures, runtime limits and cancellation; task tasks:validate and task precommit pass.

## Implementation Notes
Validator execution uses a fixed 5-second CPU / 512-MB budget, independent of
solution limits. A sandbox internal error aborts validation. Empty declared test
sets produce an empty report only after validator preparation succeeds.
Share dependency loading with checker preparation to preserve existing behavior.
No API/frontend changes. Full test identity round-trip, generated-test production,
manifest persistence and publication remain separate tasks.

## Delivery
Create the PR, obtain isolated review for the exact SHA and green CI; leave it unmerged.

## Verification
Before the fix, regression tests reproduced successful validation without a validator
and with missing source/dependencies or unsupported compilation. Nineteen unit
scenarios now pass, including real sandbox-client dispatch against a test gRPC
executor for input order, infrastructure errors and execution cancellation.
Targeted workshop validator/checker tests also pass with the race detector.
Eleven real go-judge validator scenarios pass, including rejection of existing
input, malformed input, compiler diagnostics, runtime failure, time limit,
Python execution and relative/shared/nested dependencies.
The full backend integration suite passed (239.7 seconds), including existing
workshop and checker workflows. task precommit and task tasks:validate passed.
