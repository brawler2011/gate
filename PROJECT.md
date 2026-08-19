# Project: Gate Scoreboard Freeze

## Architecture
- **Contracts (`contracts/core/v1/openapi.yaml`)**:
  - `ContestModel` & `UpdateContestRequestModel`: `freeze_duration_minutes` (int32), `freeze_status` (`auto`, `frozen`, `unfrozen`).
  - `ScoreboardResponseModel`: `is_frozen` (bool), `freeze_time` (date-time nullable).
  - `ScoreboardProblemResultModel`: `pending_attempts` (int32).
  - `GET /contests/{contest_id}/scoreboard`: `unfrozen` (bool query param).
- **Backend (`backend/`)**:
  - `internal/domain/models/contest.go`: `GetFreezeDurationMinutes()`, `GetFreezeStatus()`, `GetFreezeTime()`, `IsFrozenAt(t time.Time)`.
  - `internal/domain/models/`: `ScoreboardResponse` (`IsFrozen`, `FreezeTime`), `ContestProblemResult` (`PendingAttempts`).
  - `internal/usecase/contests.go`: `GetContestScoreboard` calculates freeze cutoff, hides verdicts post-freeze, computes `pending_attempts`, handles `unfrozen=true` for managers, keeps standings/penalties/ranks frozen.
  - `internal/transport/rest/core/contests.go`, `dto.go`: REST handlers, DTO mapping, manager permission check for `unfrozen=true`.
  - `internal/usecase/contests_scoreboard_test.go`: Unit tests for freeze calculation, timer boundaries, manual overrides, manager vs participant views.
- **Frontend (`frontend/`)**:
  - `components/contests/SettingsSection.tsx`: Form inputs for `freeze_duration_minutes` (NumberInput) and `freeze_status` (CustomSelect).
  - `components/contests/ContestMonitorTable.tsx`: "Монитор заморожен" badge, organizer toggle switch ("Замороженный вид" / "Реальный монитор"), cell rendering matrix (`?k`, `-k p?`, `+ p?`), summary rows ("Сдали", "Пытались"), leak-free WebSocket listener for `submissions.completed`.
  - `components/contests/ContestMonitorTable.module.css`: `.cellFrozenPending` styling.

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | OpenAPI Schema Updates | Add freeze fields to models and query param to scoreboard endpoint | M1 (DONE) | ORIGINAL_REQUEST §R1, R2, R4 |
| 2 | Contract Code Generation | Run `task gen` to generate Go and TypeScript contracts | M1 (DONE) | ORIGINAL_REQUEST §R4 |
| 3 | Domain Freeze Logic | Add helper methods (`IsFrozenAt`, `GetFreezeTime`, etc.) on `Contest` model | M2 (DONE) | ORIGINAL_REQUEST §R1 |
| 4 | Scoreboard Calculation & Freeze Rules | Compute frozen results, pending attempts, and manager `unfrozen` view | M2 (DONE) | ORIGINAL_REQUEST §R2 |
| 5 | Manager Authorization for Live Scoreboard | Enforce manager permissions for `unfrozen=true` query param | M2 (DONE) | ORIGINAL_REQUEST §R2 |
| 6 | My Submissions Integrity | Ensure participants see real verdicts in `/contests/{contest_id}/mysubmissions` | M2 (DONE) | ORIGINAL_REQUEST §R2 |
| 7 | Backend Unit Tests | Comprehensive tests in `contests_scoreboard_test.go` and domain tests | M2 (DONE) | ORIGINAL_REQUEST §R4 |
| 8 | Contest Settings UI | Input for `freeze_duration_minutes` and selector for `freeze_status` in `SettingsSection` | M3 (DONE) | ORIGINAL_REQUEST §R1 |
| 9 | Freeze Badge Display | "Монитор заморожен" badge when scoreboard is frozen in `ContestMonitorTable` | M3 (DONE) | ORIGINAL_REQUEST §R3 |
| 10 | Organizer Monitor View Toggle | Toggle switch between live and frozen view for contest managers | M3 (DONE) | ORIGINAL_REQUEST §R3 |
| 11 | Scoreboard Cell Formatting & Styling | Render `?k`, `-k p?`, `+ p?` / `+k p?` with `.cellFrozenPending` CSS styling | M3 (DONE) | ORIGINAL_REQUEST §R3 |
| 12 | Summary Rows Freeze Logic | "Сдали" and "Пытались" respect frozen results in `ContestMonitorTable` | M3 (DONE) | ORIGINAL_REQUEST §R3 |
| 13 | Real-time WebSocket Updates | Increment `pending_attempts` without leaking verdicts during freeze | M3 (DONE) | ORIGINAL_REQUEST §R3 |
| 14 | Frontend Typechecking Verification | Ensure `bun run typecheck` passes with zero errors | M3 (DONE) | ORIGINAL_REQUEST §R4 |
| 15 | Multi-agent Gate Verification | Full verification via Reviewers, Challengers, and Forensic Auditor | M4 (DONE) | ORIGINAL_REQUEST Acceptance Criteria |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | OpenAPI Contracts & Codegen | Update `openapi.yaml`, run `task gen`, verify generated Go/TS types | None | DONE |
| 2 | Backend Domain & UseCase Implementation | Models, freeze calculation, permissions, REST handlers, unit tests | M1 | DONE |
| 3 | Frontend Settings, Monitor Table & WebSocket | SettingsSection, ContestMonitorTable, CSS, WS listener, typecheck | M1 | DONE |
| 4 | Final Integration, Gate Audit & Verification | Multi-agent review, challenger tests, forensic audit, build/test pass | M1, M2, M3 | DONE |

## Interface Contracts
### Contracts ↔ Backend
- `GetContestScoreboardParams`: `Unfrozen *bool`
- `ContestModel`: `FreezeDurationMinutes *int32`, `FreezeStatus ContestModelFreezeStatus`
- `ScoreboardResponseModel`: `IsFrozen bool`, `FreezeTime *time.Time`
- `ScoreboardProblemResultModel`: `PendingAttempts int32`

### Contracts ↔ Frontend
- `DefaultService.getContestScoreboard({contestId, unfrozen})`
- `ScoreboardResponseModel`: `is_frozen: boolean`, `freeze_time?: string | null`
- `ScoreboardProblemResultModel`: `pending_attempts: number`
- `ContestModel` / `UpdateContestRequestModel`: `freeze_duration_minutes?: number`, `freeze_status?: 'auto' | 'frozen' | 'unfrozen'`

## Code Layout
- `contracts/core/v1/openapi.yaml`
- `backend/contracts/core/v1/core.go` (generated)
- `backend/internal/domain/models/contest.go`
- `backend/internal/usecase/contests.go`
- `backend/internal/usecase/contests_scoreboard_test.go`
- `backend/internal/transport/rest/core/contests.go`
- `backend/internal/transport/rest/core/dto.go`
- `frontend/contracts/core/v1/` (generated)
- `frontend/components/contests/SettingsSection.tsx`
- `frontend/components/contests/ContestMonitorTable.tsx`
- `frontend/components/contests/ContestMonitorTable.module.css`
- `frontend/app/contests/[contest_id]/monitor/page.tsx`
