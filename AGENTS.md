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

## Task Workflow & Tags

Tasks may include explicit tags to define execution constraints and lifecycle:

### Tags & Defaults

1. **Breaking Changes (Default: Enabled)**
   - By default, tasks **ARE PERMITTED** to introduce breaking changes (API signature updates, contract modifications, DB schema changes) without maintaining backward compatibility layers.
   - If backward compatibility is required, the task will explicitly specify the **`BACKWARD COMPATIBLE`** tag.

2. **`WORKTREE`**
   - Signals that the task is executed in an isolated git worktree branch.
   - When `WORKTREE` is active (or when performing autonomous task execution), the agent **MUST** follow a strict TDD & verification workflow:
     1. **Scope & DoD Definition**: The agent explicitly analyzes the requirements and outputs:
        - **Scope:** Precise list of files/modules to modify.
        - **Boundaries:** Files/modules that must NOT be touched.
        - **Definition of Done (DoD):** Specific acceptance criteria and verification commands.
     2. **Test-First (TDD):** Write or update unit/integration tests covering the required behavior *before* writing the implementation. Verify that the new tests fail as expected.
     3. **Implementation:** Write the minimal code necessary to make the tests pass.
     4. **Pre-commit Verification:** Run the standard pre-commit verification command (see below).
     5. **Commit:** Create a clean conventional git commit in the current branch (do not switch branches or push).

---

## Pre-commit Verification Protocol

Before making any git commit, run the appropriate Task verification command based on the scope of changes:

- **Full Project / Cross-cutting changes:**
  ```bash
  task precommit
  ```
- **Backend-only changes:**
  ```bash
  task precommit:be
  ```
- **Frontend-only changes:**
  ```bash
  task precommit:fe
  ```

Never commit code with failing tests, type errors, or unresolved linter violations.
