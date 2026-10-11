---
id: TASK-016
title: "Enforce access rules in the public problem list"
status: todo
type: fix
description: "Filter problem lists and pagination by the caller's existing read access."
priority: critical
created_at: 2026-10-11
tags:
  - backend
  - security
---

# TASK-016: Enforce access rules in the public problem list

## Context
The public ListProblems endpoint exposes private and unlisted metadata when
organization_id is absent. Search filters must only narrow the accessible set;
they must never grant access. Use the existing GetProblem read rules: public visibility, global admin, problem
owner, direct problem member, assigned team member in the problem organization,
or organization owner/admin. Ordinary organization membership alone grants no access.
The user explicitly confirmed this resolution of the conflicting list rule.
Unlisted problems require the same grants as private problems. The owner filter
means owner_id equals the caller; guests requesting owner=true receive an empty list.
API pagination.total retains its existing meaning (number of pages).
Scope is CODEBASE_REVIEW.md issue 1 only; frontend and other access endpoints stay unchanged.

## Acceptance Criteria
- [ ] Guests receive only public problems with and without organization_id and with search, owner and template filters.
- [ ] Authenticated callers retain public, owner, direct, team and organization access under the agreed existing rules.
- [ ] Filters only narrow results; pagination and total operate on accessible rows before LIMIT/OFFSET, without duplicates.
- [ ] HTTP regression tests reproduce the original leak and verify access grants and revocation.
- [ ] Task validation, precommit checks, independent review and PR CI pass; PR remains unmerged.
