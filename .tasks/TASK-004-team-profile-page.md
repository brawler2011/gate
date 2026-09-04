---
id: TASK-004
title: "Implement dedicated team profile page"
status: todo
type: fix
description: "Add a dedicated team profile page displaying roster, statistics, and contest participation."
priority: normal
created_at: 2026-08-24
tags:
  - frontend
  - team
  - ui
---

# TASK-004: Implement dedicated team profile page

## Context
Imported from YouGile `TID-301` ("Страница команды"). The system currently lacks a dedicated public/member view for contest teams. Users and managers require a cohesive page to view team roster, captain, member statuses, and contest participation history.

## Acceptance Criteria
- [ ] Dedicated team route/page implemented in Next.js frontend
- [ ] Displays team name, avatar/badge, creator/captain, and current roster with user handles
- [ ] Lists active and past contest participations for the team
- [ ] Proper loading skeletons and error states (team not found, unauthorized)
- [ ] Automated tests written and passing (`bun test` and `task precommit:fe`)

## Implementation Notes
Adhere to `AGENTS.md` and `frontend/REFACTORING.md`: use Client Components with SWR for dynamic team state, Mantine components for UI, and avoid Server Actions.
