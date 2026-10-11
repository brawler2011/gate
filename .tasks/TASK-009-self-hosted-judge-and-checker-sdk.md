---
id: TASK-009
title: "Build SDK for self-hosted judge workers and custom checkers"
status: todo
type: feat
description: "Develop Go/Python SDK allowing self-hosted judge nodes and custom problem checkers to integrate with Gate."
priority: normal
created_at: 2026-08-20
tags:
  - backend
  - judge
  - sdk
  - architecture
---

# TASK-009: Build SDK for self-hosted judge workers and custom checkers

## Context
Imported from YouGile TID-280 ("SDK для self-hosted judge + checks"). To support distributed contests, custom problem environments (e.g. specialized hardware, custom grading harnesses, or external execution nodes), Gate needs an SDK and protocol specification for running self-hosted judge workers and custom checkers safely and reliably.

## Acceptance Criteria
- [ ] SDK interface defined for judge worker registration, heartbeat, job polling/dispatch, and result reporting
- [ ] Secure token/mTLS authentication mechanism between self-hosted judge and Gate backend
- [ ] Checker execution protocol supporting standard testlib formats and custom return codes
- [ ] Example runner implementation provided in SDK package
- [ ] Unit and integration tests covering dispatch, grading, and network failure recovery

## Implementation Notes
Ensure sandboxing guidelines and timeout limits are strictly enforced on client SDKs.
