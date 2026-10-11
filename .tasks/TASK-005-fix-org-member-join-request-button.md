---
id: TASK-005
title: "Fix join request button visibility for existing org members"
status: todo
type: fix
description: "Prevent display of join request button on organization page when user is already a member."
priority: normal
created_at: 2026-08-24
tags:
  - frontend
  - orgs
  - permissions
  - auth
---

# TASK-005: Fix join request button visibility for existing org members

## Context
Imported from YouGile TID-300 ("У участника организации есть кнопка 'заявка на вступление' - доработать"). When an authenticated user who is already an approved member of an organization visits that organization's page, the UI incorrectly continues to render the "Request to Join" action button. The join request button must only be shown to non-members without an existing pending application.

## Acceptance Criteria
- [ ] Check user's membership status in the organization prior to rendering the join action button
- [ ] Hide or disable "Request to Join" button when the current user is already an accepted member
- [ ] Show appropriate status indicator (e.g. "Member" badge or "Pending Approval" if request is under review)
- [ ] Component unit tests added covering non-member, pending member, and existing member states
- [ ] Frontend pre-commit validation task precommit:fe passes cleanly

## Implementation Notes
Ensure check uses cached membership data from SWR or session context to prevent layout shift.
