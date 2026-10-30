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
- Do not modify the root `README.md` unless explicitly requested by the user.
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
   - Validate the Conventional Commits title (<= 65 chars) with `task commit:validate -- --title '<title>'`.
   - Follow `.github/pull_request_template.md`, recording only checks actually performed and their results.
   - Push the issue branch and create the PR with `gh pr create --body-file <path>`.

3. **Adversarial Code Review Workflow (Reviewer Agent)**
   - **Context Isolation**: Review in a fresh context and clean worktree at the PR head, without the author's conversation or claimed results.
   - **Reviewer Inputs**: Issue description, `AGENTS.md`, and `git diff <base-sha>...<head-sha>`. Allow access to repository code and tests needed to verify the change. Record the full head SHA and current `main` base SHA; stop and report any failure to obtain the diff.
   - **Reviewer Audit Focus**:
     1. Acceptance Criteria: All items genuinely verified and tested.
     2. `AGENTS.md` compliance: `slog` only, no env fallback values, no server actions, untouched `next.config.mjs`, no symlinks.
     3. Surgical changes: no unrelated edits, no leftover dead code.
     4. Test coverage and edge cases.
   - **Evidence**: Record checks actually run and their results for the reviewed SHA pair; identify failures, skipped checks and modified files. Do not treat author claims as verification.
   - **Verdict Format**: Output `APPROVE` or `REQUEST_CHANGES`, the reviewed head/base SHAs, and actionable file:line pointers. Post with `gh pr review --comment --body-file <path>`; `APPROVE` in a comment is an agent verdict, not a GitHub approval event.
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

