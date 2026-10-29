## Issue
<!-- Use Closes #N for complete delivery, or Refs #N for partial work. -->

## Description
<!-- Concise summary of changes and non-obvious rationale. Keep focused. -->

## Type of Change
- [ ] `feat`: New feature
- [ ] `fix`: Bug fix
- [ ] `refactor`: Code refactoring without behavioral change
- [ ] `chore`: Maintenance, tooling, or workflow update
- [ ] `docs`: Documentation updates

## Breaking Changes
- [ ] Yes (breaking change permitted by default)
- [ ] No (backward compatible)

## Guidelines Compliance (`AGENTS.md`)
- [ ] Surgical changes: only relevant files and lines touched, no extraneous formatting
- [ ] Frontend (if applicable): `next.config.mjs` and `next-env.d.ts` untouched
- [ ] Frontend (if applicable): No fallback values for environment variables in code
- [ ] Frontend (if applicable): No Server Actions (`'use server'`)
- [ ] Backend (if applicable): `slog` used exclusively for logging
- [ ] No symbolic links (symlinks) created

## Verification
<!-- List checks actually run and their results, including task precommit. State failures, skipped checks, and limitations. Never prefill success. -->
- Checked head SHA (full):
- Commands and results (exit status; output/log links):

## Independent Review Evidence
<!-- Reviewer fills a separate review comment from a fresh context/worktree using AGENTS.md. Do not copy author results as independent evidence. -->
- Reviewed head SHA / base SHA (full):
- Review comment and independent check evidence:
- CI check/run links for this revision:
<!-- Any new head commit invalidates the evidence and requires renewed checks/review/CI. A changed main base also requires renewed review/CI. Human squash merge only after current-SHA APPROVE and green required CI. -->
