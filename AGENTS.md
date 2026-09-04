# Project Guidelines & Rules

## General Guidelines

Behavioral guidelines to reduce common LLM coding mistakes.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.
- **Never create symbolic links (symlinks).** Use standard files instead.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

### 5. Rigorous Discussion & Critical Pushback

**Never agree blindly. Provide trade-offs, risks, and real-world precedents.**

During active discussion and planning:
- Do not be a sycophant or validate flawed ideas just to be agreeable. Act as an objective, critical engineering peer.
- Support technical recommendations with concrete arguments: pros, cons, failure modes, and trade-offs.
- Ground arguments in real-world production practices and operational realities (e.g., maintenance cost, failure risks, scalability).
- If a proposed approach is suboptimal or overengineered, push back constructively and suggest a battle-tested alternative.
- Once debate concludes and a final decision is made by the user, commit fully and execute cleanly.

---

## Frontend Guidelines

When working with the frontend codebase (`/frontend` directory), adhere strictly to the following rules:

1. **Never modify `next.config.mjs`**
2. **Never modify `next-env.d.ts`**
3. **Use `bun` instead of `npm`**
   - Always use `bun` for package management, running scripts, and development commands (e.g., `bun run dev`, `bun add <package>`, `bun install`, `bun run typecheck`, `bun run build:local`). Do not use `npm`, `npx`, `yarn`, or `pnpm`.
4. **Never override or set fallback values for environment variables in code**
   - Do not use fallback values for environment variables (e.g., `return process.env.BACKEND_API_URL || "http://localhost:8080";`).
5. **Do not use or create Server Actions (`'use server'`)**
   - In this architecture, Server Actions are unnecessary and prohibited.
   - **Client Components (`"use client"`)**: Perform direct API calls via `@/lib/api` (`api.<method>(...)`).
   - **Server Components (RSC)**: Fetch data directly during component/page render using `@/lib/api` (e.g., `await unwrap(api.<method>)(...)` or `await api.<method>(...)`).

---

## Backend Guidelines

When working with the backend codebase, adhere strictly to the following rules:

1. **Use `slog` exclusively for logging**
   - Use ONLY `slog`. No other loggers are allowed.

---

## Task Guidelines

1. **Use `task` instead of `go-task`**
   - Always invoke Task commands as `task` (e.g., `task build`, `task test`). Do not use `go-task`.

---

## Task Specification Guidelines (.tasks/)

When managing or working with tasks in `.tasks/`:

1. **Task Lifecycle: `draft` -> `todo` -> `done`**
   - **Closed state machine**: Tasks have only three valid states: `draft`, `todo`, `done`.
   - `draft`: Task definition is being formulated or under discussion. Not ready for execution.
   - `todo`: Task specification is complete with clear context and criteria. Ready for development.
   - `done`: All acceptance criteria are fulfilled and verified.
   - **Execution tracking**: Tasks are static specifications, not an issue tracker. Work-in-progress and review states are tracked exclusively via Git branches and Pull Requests.

2. **Definition of Done Enforcement**
   - When marking `status: done`, ALL items in `## Acceptance Criteria` MUST be marked as completed (`- [x]`).
   - If any criteria remain unchecked (`- [ ]`), validation will fail.

3. **Validation and Pre-Commit**
   - Always ensure modified tasks pass `task tasks:validate` before committing.

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

---

## Tags

1. **Breaking Changes (Default: Enabled)**
   - By default, tasks **ARE PERMITTED** to introduce breaking changes (API signature updates, contract modifications, DB schema changes) without maintaining backward compatibility layers.
   - If backward compatibility is required, the task will explicitly specify the **`BACKWARD COMPATIBLE`** tag.
