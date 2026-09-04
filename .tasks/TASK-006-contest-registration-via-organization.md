---
id: TASK-006
title: "Restrict contest participation to organization members"
status: todo
type: fix
description: "Evaluate and enforce policy allowing contest registration exclusively through organizations."
priority: normal
created_at: 2026-08-22
tags:
  - backend
  - frontend
  - contests
  - orgs
  - policy
---

# TASK-006: Restrict contest participation to organization members

## Context
Imported from YouGile TID-297 ("Сделать добавление на контест только через организацию? Подумать"). Currently, individual users can register for contests directly. For institutional contests or enterprise events, participation must be restricted so that participants must belong to an eligible organization. This task defines the policy, backend validation checks during registration, and corresponding frontend UI state.

## Acceptance Criteria
- [ ] Add contest configuration flag (e.g. require_organization_membership) in contest schema and API
- [ ] Backend validation rejects registration if participant is not a verified member of an allowed organization
- [ ] Frontend contest registration dialog displays organization selector and explains requirements when enabled
- [ ] API unit tests and frontend component tests verifying enforcement
- [ ] Both backend and frontend pre-commit checks pass cleanly

## Implementation Notes
Verify interaction with team-based contests where team members might belong to different orgs.
