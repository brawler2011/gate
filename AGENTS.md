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
   - **Context Isolation**: Start a fresh reviewer context/session (or isolated reviewer subagent) with no author conversation, implementation narrative, or claimed check results. Supply the issue description, this `AGENTS.md`, and the pinned diff. Give the reviewer a separate clean worktree at the reviewed head, with access to repository code, configuration, and tests needed to independently verify the change.
   - **Pin the input**: Record the full PR head SHA and fetched `main` base SHA. Prepare the worktree and diff with the following commands, replacing `123` with the PR number. Run in a shell where failure can terminate the preparation:

     ```sh
     PR=123
     REVIEW_HEAD=$(gh pr view "$PR" --json headRefOid --jq .headRefOid) || exit 1
     git fetch origin main "refs/pull/$PR/head" || exit 1
     REVIEW_FETCHED_HEAD=$(git rev-parse FETCH_HEAD) || exit 1
     if [ "$REVIEW_FETCHED_HEAD" != "$REVIEW_HEAD" ]; then
       echo 'PR head changed during preparation; restart review' >&2
       exit 1
     fi
     REVIEW_BASE=$(git rev-parse origin/main) || exit 1
     REVIEW_DIR=$(mktemp -d) || exit 1
     git worktree add --detach "$REVIEW_DIR" "$REVIEW_HEAD" || exit 1
     REVIEW_DIFF=$(mktemp) || exit 1
     if ! git diff "$REVIEW_BASE...$REVIEW_HEAD" -- > "$REVIEW_DIFF"; then
       echo 'Review blocked: failed to acquire pinned diff' >&2
       exit 1
     fi
     printf 'Head: %s\nBase: %s\nWorktree: %s\nDiff: %s\n' \
       "$REVIEW_HEAD" "$REVIEW_BASE" "$REVIEW_DIR" "$REVIEW_DIFF"
     ```

   - **Diff failures**: Report the failing command and error as a blocked review. Never substitute a working-tree diff, cached diff, or empty diff after an acquisition failure. Confirm the resulting diff covers the intended issue before reviewing.
   - **Reviewer Audit Focus**:
     1. Acceptance Criteria: Verify every item against the pinned code and independently run relevant checks/tests in the review worktree; inspect surrounding code where necessary.
     2. `AGENTS.md` compliance: `slog` only, no env fallback values, no server actions, untouched `next.config.mjs`, no symlinks.
     3. Surgical changes: no unrelated edits, no leftover dead code.
     4. Test coverage and edge cases.
   - **Evidence**: Record full head and base SHAs, each command actually run, its result/exit status, and relevant output or a log link. Explicitly distinguish failures, skipped checks, and checks that could not run. Verify `git rev-parse HEAD` still equals the pinned head and inspect `git status --short` after checks; reconcile generated or modified files before claiming results for that SHA. Never infer successful checks from an author statement, a PR checkbox, or an unexecuted command.
   - **Verdict Format**: Start with `APPROVE` or `REQUEST_CHANGES`, include the full reviewed head/base SHAs and independent evidence, and give actionable file:line pointers for findings. Post with `gh pr review <number> --comment --body-file <review-file>`; a comment containing `APPROVE` is the reviewer-agent verdict, not a GitHub approval event.
   - **Invalidation**: Before posting, compare the current `gh pr view <number> --json headRefOid --jq .headRefOid` with the reviewed head. If it differs, report the evidence as stale and restart at the new head. Every new commit (including rebases, merge commits, and review fixes) invalidates all prior local check evidence and verdicts; rerun checks, obtain a fresh isolated review, and wait for CI for the current head. A changed base requires renewed diff/review and CI for that base too.

4. **Semi-Autonomous Merge Policy**
   - Agents DO NOT auto-merge to `main`.
   - Final merge is executed by a human engineer (or an agent upon explicit human instruction) only after:
     1. Reviewer Agent verdict is `APPROVE` for the current full PR head SHA and current fetched `main` base SHA, with independently collected check evidence.
     2. All GitHub Actions CI checks (`ci.yml`, `pr-title.yml`) and repository-required checks pass for the current PR revision. Inspect `gh pr checks <number>` and `gh pr checks <number> --required`, retaining check/run links and results with the head SHA. Pending, failed, missing, or skipped required checks do not satisfy the gate. For CI on a synthetic merge commit, verify its parents match the reviewed head and base; never reuse a run from an older revision.
     3. Immediately before merge, recheck the remote PR head and `main` base against the evidence. Any mismatch requires the renewed checks/review above; do not merge on stale evidence.
     4. Merge strategy is squash-and-merge (`gh pr merge <number> --squash --delete-branch --match-head-commit <reviewed-head-sha>`). Never enable auto-merge.

---

## Tags

1. **Breaking Changes (Default: Enabled)**
   - By default, tasks **ARE PERMITTED** to introduce breaking changes (API signature updates, contract modifications, DB schema changes) without maintaining backward compatibility layers.
   - If backward compatibility is required, the task will explicitly specify the **`backward-compatible`** issue label.

