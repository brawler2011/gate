---
id: TASK-018
title: "Protect observer access and frozen submission results"
status: done
type: fix
description: "Authorize observer delivery and prevent frozen result disclosure through WebSocket and REST."
priority: critical
created_at: 2026-10-11
tags:
  - backend
  - frontend
  - contracts
---

# TASK-018: Observer access and frozen results

## Context
Observer subscriptions currently lack contest authorization and share unrestricted payloads.
Use the existing PermissionsUC and contest freeze rules. Own results remain visible;
ManageContest allows moderators to see other users' frozen results.
Start from origin/main including PR #11. Preserve untracked CODEBASE_REVIEW.md.

## Acceptance Criteria
- [x] Handshake rejects guests (401), malformed filters (400), and missing/inaccessible contests (403); contest-less submissions require global admin.
- [x] Submission access follows ListOwnSubmissions, GetMonitor, ListUsersSubmissions; filters only narrow access and missing metadata never matches a specified filter.
- [x] Announcements require GetContest; clarifications are delivered only to author or ManageContest.
- [x] Before history, pending, and live delivery, recheck session, current permissions and contest; close with 1008 on access loss and 1011 on verification error, without database work under hub locks.
- [x] Shared REST/WS policy hides other users' results at/after freeze, all results for forced freeze without boundary; unfrozen and own/moderator views remain complete.
- [x] Frozen JSON retains metadata and frozen=true, omits result fields, source and test details; completed remains available and hidden intermediate events are suppressed. Source and progress require their existing permissions.
- [x] GetSubmission and ListContestSubmissions apply masking; state filtering during freeze requires moderator or exclusively own selection. Scoreboard counts hidden pending before verdict inspection.
- [x] Optional contracts are generated with normal commands; frontend displays frozen results, preserves own results, avoids duplicate pending and restores scoreboard after reconnection.
- [x] Regression tests cover handshake/JSON, role sources, history/pending/live, revocation/demotion/deletion/database errors and freeze boundary/forced/unfrozen cases.
- [x] Backend unit/integration, observer race, bun test, task tasks:validate and task precommit pass.

## Implementation Notes
Revalidate on each delivery; idle connections are checked on the next event.
Previously delivered messages cannot be revoked. Additional database queries are accepted.
Precommit metadata tests use temporary task fixtures because origin/main removed TASK-001/003.

## Delivery
Create the PR with task pr:create, obtain isolated APPROVE for the exact SHA and green CI; leave it unmerged.

## Verification
Backend unit and full integration suites passed; real PostgreSQL roles and session revocation passed.
Observer race tests passed; bun test passed (60 tests), including production DOM tests.
Contracts generated; task tasks:validate and full task precommit passed.
