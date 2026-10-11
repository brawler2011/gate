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
- [x] Real sandbox tests reuse the built compiler image while keeping separate containers; the full integration suite passes without increasing the test timeout, with measured build times.

## Implementation Notes
Validator execution uses a fixed 5-second CPU / 512-MB budget, independent of
solution limits. A sandbox internal error aborts validation. Empty declared test
sets produce an empty report only after validator preparation succeeds.
Share dependency loading with checker preparation to preserve existing behavior.
No API/frontend changes. Full test identity round-trip, generated-test production,
manifest persistence and publication remain separate tasks.
Keep the locally built go-judge image so subsequent isolated containers reuse
compiler layers. Removing it with PruneChildren after each test forced repeated
image builds and caused the first PR CI run to reach Go's 10-minute suite timeout
before validator execution. Preserve the existing GOJUDGE_TEST_IMAGE override.

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

CI initially hit Go's unchanged 10-minute suite timeout during the validator
sandbox image build. Earlier CI image rebuilds took about 105 seconds each.
After preserving the built image (KeepImage: true), all five local image builds
completed within 0-1 seconds at log timestamp resolution. Each test still started
and terminated its own container. The full local suite passed in 125.049 seconds
versus the prior 239.734-second run (about 48% faster in this single comparison).
This measurement uses the existing local Docker layer cache; a fresh CI runner
still needs its first compiler-image build. No test or timeout was removed or
relaxed. task precommit passed again after the cache change.
