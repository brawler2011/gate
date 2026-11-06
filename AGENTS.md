# Project Guidelines & Rules

## General Guidelines

### 1. Clarify & Challenge
- Do not assume or pick interpretations silently. If requirements or designs are ambiguous, ask.
- Push back constructively with concrete trade-offs when an approach is suboptimal or overengineered. Commit cleanly once decided.

### 2. Scope & Surgical Restraint
- Touch only what the task requires. Zero unrelated edits, no drive-by refactoring, and no adjacent formatting changes.
- Surgical restraint applies equally to test code: author new tests strictly for newly introduced logic, bug fixes, or explicit issue test requirements. Never build speculative test harnesses, mocks, or synthetic test suites for untouched pre-existing features.
- Do not create Markdown files or edit `README.md` unless explicitly requested.
- Clean up only your own orphans; leave pre-existing dead code untouched. Never create symlinks.

### 3. Architecture Over Diff Minimization
- Issue acceptance criteria are immutable: never drop, weaken, mock out, or invert requirements to minimize diff.
- Structural integrity, Single Responsibility, and idiomatic patterns take precedence over diff-line count.
- Do not overload existing functions or types with unrelated concerns or control flags (no lazy overloading); create focused components instead.
- No speculative code: no unrequested abstractions, configurability, or handling for impossible scenarios.

### 4. Goal-Driven Verification
- Ground tasks in verifiable success criteria (tests, commands). Loop until verified.
- Do not add tests merely because code changed or a bug was fixed. Prefer existing checks. For simple local changes, do not introduce dedicated fixtures, stubs, or test files unless explicitly required by the issue or necessary to verify a concrete, material risk.
- Distinguish change verification from invariant preservation: acceptance criteria requiring untouched features to remain operational are verified by running existing regression suites (`task test`, `task typecheck:fe`, `task lint:fe`), never by authoring new test suites for untouched code paths.
- Escalate test-environment impedance: if a test fails due to test-runner or mock limitations (e.g., headless DOM event quirks) rather than a code defect, verify whether the test itself violates surgical scope before attempting test-harness rewrites.

---

## Repository Layout

- `backend/`: Go application, migrations, SQL queries, and backend tests. The main Go module is at the repository root.
- `frontend/main.tsx`: React SPA entry point; `frontend/app/App.tsx`: React Router route definitions.
- `frontend/`: pages, components, hooks, API client, and Bun tests. The `@/` alias resolves here.
- `contracts/`: core OpenAPI and observer AsyncAPI specifications.
- `deploy/`: local and production configuration, Compose services, and environment examples.
- `package.json`, `bun.lock`, `vite.config.ts`, and `Taskfile.yml` live at the repository root. Run commands from there.

## Frontend Guidelines

These rules apply to `frontend/` and frontend configuration at the repository root.

1. **Use `bun` instead of `npm`**
   - Use `bun` for package management, scripts, and development commands. Do not use `npm`, `npx`, `yarn`, or `pnpm`.
2. **React/Vite SPA architecture**
   - Use React Router for routing and loaders, TanStack Query for query state, and Mantine for UI.
3. **Use the shared API client**
   - Import `api` and, when needed, `unwrap` from `@/lib/api` in components, hooks, and route loaders.
   - Browser requests use same-origin `/api` and `/ws`; Vite proxies these paths to `BACKEND_API_URL` during development.
4. **Never override or set fallback values for environment variables in code**
   - Local configuration is in `deploy/local/.env`; available variables are listed in `deploy/local/.env.example`.

---

## Backend Guidelines

When working with the backend codebase, adhere strictly to the following rules:

1. **Use `slog` exclusively for logging**
   - Use ONLY `slog`. No other loggers are allowed.

---

## Task Guidelines

- Use `task` instead of `go-task`. `task -a` lists available tasks.
- Install JavaScript dependencies with `bun install --frozen-lockfile`.
- Development: `task dev` runs the Go backend with Air and the Vite frontend; `task dev:be` and `task dev:fe` run them separately. Start required services separately using `task docker:up`.
- Database migrations: `task migrate:be` uses `deploy/local/.env`.
- Checks: `task test:be`, `task lint:be`, `task typecheck:fe`, `task lint:fe`, `task knip:fe`, and `bun run test`. `task test` runs backend and frontend tests.
- Full backend test suite: `task test:be:integration` includes integration tests and requires Docker for Testcontainers.
- Build: `task build` builds Go packages and the static SPA. `task build:fe:local` loads local environment settings; the application Docker image includes the Go binary and SPA bundle.
- `task precommit` runs contract lint, codegen freshness, backend lint/tests, and frontend typecheck/lint. It does not include frontend tests, Knip, integration tests, or the production build; run relevant checks separately.

## Generated Code

- Edit OpenAPI sources in `contracts/core/v1/openapi.yaml` and `contracts/observer/v1/openapi.yaml`, then run `task gen:contracts` to regenerate Go and TypeScript contracts.
- `task lint:contracts` validates both specifications using the pinned Vacuum version and `vacuum.yaml`; correctness errors block changes, while style/documentation warnings are advisory.
- Edit SQL queries in `backend/internal/postgres/*.sql` and migrations in `backend/migrations/`, then run `task gen:sqlc`.
- `task gen:mocks` regenerates mocks; `task gen` runs all generators.
- Do not hand-edit generated files in `backend/contracts/core/v1/`, `backend/contracts/observer/v1/`, `frontend/contracts/`, `backend/internal/postgres/sqlc/`, or `backend/tests/mocks/`.
- `task diff:codegen` regenerates code and fails if those generated paths have tracked or untracked changes.

---

## GitHub Communication Guidelines

- Include only information that helps readers understand the task, make a decision, or fix a problem. Let the complexity of the change determine the length.
- Omit repetition, generic assurances of compliance, process narration, and empty sections. Do not create separate reports to replace removed template sections.
- In PR descriptions and reviews, report only check failures and material verification gaps. Do not list successful checks or repeat the contents of `task precommit`.

## Issue Guidelines

- GitHub Issues are the only task specification store. All issues, including those created by agents, MUST follow the appropriate form in `.github/ISSUE_TEMPLATE`.
- Use `bug`, `enhancement`, or `technical-task` as appropriate. The `backward-compatible` label requires preserving compatibility.
- Project statuses are Backlog → Ready → In progress → Review → Done. New proposals and unresolved design questions belong in Backlog; Ready requires defined scope and verifiable acceptance criteria.
- Update In progress and Review explicitly. PR creation or closure does not complete an issue. Closed issues become Done. When reopening an issue, move it to Backlog. Preserve the reason when closing as not planned.

## Git & Commit Guidelines

1. **Conventional Commits Format**
   - Format: `<type>(<scope>): <subject>` or `<type>: <subject>`
   - Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
   - Scope should be lowercase (e.g., `backend`, `frontend`, `contracts`, `tooling`, `ci`, `auth`).
   - Do NOT end the subject line with a period (`.`).

2. **Subject & PR Title Length Limit (Max 65 Chars)**
   - The first line (subject / PR title) MUST NOT exceed **65 characters**.
   - GitHub web UI truncates commit messages beyond ~65-72 characters in table views and adds ` (#<pr>)` on squash merges.
   - Keep the subject line concise and punchy.

3. **Separation of Header and Body**
   - Do NOT cram multiple changes or full changelogs into the subject line.
   - Leave the second line empty.
   - Put detailed descriptions, rationale, and bullet lists in the commit **body** or PR description.

## Pull Request & Agent Workflow Guidelines

1. **Issue Association & Branching**
   - Every PR MUST reference a GitHub issue. Use `Closes #N` when the PR completes the issue's scope and `Refs #N` for partial work.
   - Branch naming convention: `issue/<number>-<slug>`.
   - Never commit or open PRs directly from `main`.

2. **PR Creation**
   - Run `task precommit` successfully before creating the PR.
   - Validate the Conventional Commits title (<= 65 chars) with `printf '%s\n' '<title>' | task validate:commit`.
   - Follow `.github/pull_request_template.md`: include the issue link, result, and non-obvious rationale. Describe material breaking changes alongside the result.
   - Push the issue branch and create the PR with `gh pr create --body-file <path>`.

3. **Adversarial Code Review Workflow (Reviewer Agent)**
   - **Context Isolation**: Review in a fresh context and clean worktree at the PR head, without the author's conversation or claimed results.
   - **Reviewer Inputs**: Issue description, `AGENTS.md`, and `git diff <base-sha>...<head-sha>`. Allow access to repository code and tests needed to verify the change. Obtain the full PR head and current `main` base SHAs through GitHub and git to track the reviewed revision; publishing them is not required. Stop and report any failure to obtain the diff.
   - **Reviewer Audit Focus**:
     1. Acceptance criteria integrity: verify against the original issue; reject dropped/inverted requirements and confirmation-bias tests.
     2. Architecture & SRP: reject lazy overloading (cramming secondary concerns or control flags into existing helpers to save lines).
     3. `AGENTS.md` compliance: `slog` only, no env fallback values, React/Vite SPA architecture, no symlinks.
     4. Surgical scope: zero unrelated edits, no drive-by refactoring or formatting changes, no leftover dead code, and no test scope creep on untouched invariants.
     5. Verification and edge cases: verify changed behavior using appropriate existing checks. Missing new tests is not itself a finding. Request a new test only when explicitly required by the issue or when a concrete, material failure mode remains unverified; identify that requirement or failure mode. Reject speculative test suites for unmodified invariants.
   - **Independent Verification**: Run the checks needed to verify the reviewed revision. Do not treat author claims as verification.
   - **Verdict Format**: With no findings, output `APPROVE`. Otherwise, output `REQUEST_CHANGES` with concrete problems and actionable `file:line` pointers. Include material verification limitations when present. Post with `gh pr review --comment --body-file <path>`; `APPROVE` in a comment is an agent verdict, not a GitHub approval event.
   - Any head or base change invalidates prior review and check evidence; obtain a new diff, independent review and CI results.

4. **Semi-Autonomous Merge Policy**
   - Agents DO NOT auto-merge to `main`.
   - Final merge requires explicit human instruction and all of the following:
     1. Reviewer Agent verdict is `APPROVE` for the current head/base SHAs.
     2. All GitHub Actions CI checks (`ci.yml`, `pr-title.yml`) and required checks pass for that revision. If CI tests a synthetic merge, verify its parents match the reviewed head/base.
     3. Recheck remote head/base immediately before merging; restart verification if either changed.
     4. Use squash-and-merge: `gh pr merge --squash --delete-branch --match-head-commit <head-sha>`.

---

## Tags

1. **Breaking Changes (Default: Enabled)**
   - By default, tasks **ARE PERMITTED** to introduce breaking changes (API signature updates, contract modifications, DB schema changes) without maintaining backward compatibility layers.
   - If backward compatibility is required, the task will explicitly specify the **`backward-compatible`** issue label.
