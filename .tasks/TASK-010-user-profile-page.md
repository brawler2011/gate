---
id: TASK-010
title: "Design responsive user profile page with backend fields"
status: todo
type: feat
description: "Build user profile page reflecting real backend schema fields, contest history, and statistics."
priority: normal
created_at: 2025-11-16
tags:
  - frontend
  - user
  - profile
  - ui
---

# TASK-010: Design responsive user profile page with backend fields

## Context
Imported from YouGile TID-85 ("Сделать красивую страницу профиля пользователя", notes: "статус: в ожидании настоящих полей юзера с бекенда"). The user profile view requires a polished UI displaying user bio, organization affiliations, solved problems, contest ranking/rating trajectory, and activity heatmap once backend schema fields are finalized.

## Acceptance Criteria
- [ ] Synchronize frontend user contract with current backend user schema fields
- [ ] Build user profile page at /user/[username] with responsive Mantine UI
- [ ] Display user stats (rating, contests participated, problems solved, badges/achievements)
- [ ] Graceful fallback for optional profile fields (avatar, bio, social links)
- [ ] Unit tests for profile components and SWR data hook
- [ ] task precommit:fe passes cleanly

## Implementation Notes
Follow AGENTS.md frontend guidelines: client component with SWR, no server actions, bun run typecheck.
