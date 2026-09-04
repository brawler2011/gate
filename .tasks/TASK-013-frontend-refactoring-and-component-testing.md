---
id: TASK-013
title: "Refactor frontend components and establish SSR/CSR testing harness"
status: todo
type: refactor
description: "Decompose monolithic components, extract SWR domain hooks, and introduce Bun Test component harness."
priority: normal
created_at: 2026-08-20
tags:
  - frontend
  - testing
  - refactor
  - nextjs
---

# TASK-013: Refactor frontend components and establish SSR/CSR testing harness

## Context
Imported from YouGile TID-276 ("Рефакторинг компонентов. Покрыть фронтенд тестами. SSR-CSR.") and detailed in frontend/REFACTORING.md. Several critical components suffer from duplicated state management, monolithic files, and lack of UI component regression tests. This task implements Phase 1 and Phase 2 of the refactoring plan.

## Acceptance Criteria
- [ ] Set up @testing-library/react and happy-dom test runner for bun test in frontend/
- [ ] Decompose monolithic components/problems/Problem.tsx into compound subcomponents
- [ ] Unify duplicated entity management tables (ContestTeamsManagement, OrgMembersManagement) into shared hooks/components
- [ ] Add regression tests verifying component rendering and state transitions
- [ ] Strict typecheck (bun run typecheck) and linter (bun run lint) pass with zero regressions

## Implementation Notes
Reference frontend/REFACTORING.md for phased plan and compound component architecture.
