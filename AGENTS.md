# Project instructions

Gate is a single-owner project with no users yet. Optimize for small, correct
changes delivered quickly; skip ceremony that does not catch real problems.

## Scope

- Do what the task asks plus the supporting changes it needs. No unrelated
  cleanup, refactors, or features.
- Decide routine implementation details yourself. Ask only when a choice
  changes behavior or scope the owner would notice.
- Breaking changes are fine unless the issue has the `backward-compatible` label.

## Conventions

- Run commands from the repository root with `task` and `bun`; `task --list`
  shows what exists. Install dependencies with `bun install --frozen-lockfile`.
- Backend logging: `slog`.
- Frontend: React Router for routing, TanStack Query for server state, Mantine
  for UI. Call the backend through `api` (and `unwrap` when needed) from
  `@/lib/api`; browsers use same-origin `/api` and `/ws`.
- Database schema lives only in `backend/migrations/20261112000000_init.sql`.
  Edit it in place; do not add migrations. Changing it wipes all data on the next
  deploy (see `backend/migrations/README.md`).
- No env var overrides or fallback values in code. Local settings go in
  `deploy/local/.env`; list new variables in `deploy/local/.env.example`.
- Generated code: edit the source and run the generator (`task gen` or the
  specific `*:gen` task). Never hand-edit paths listed in `GENERATED_PATHS` in
  `Taskfile.yml`.

## Tests

A test must be able to fail for a reason someone would care about. Write one when:

- fixing a bug — reproduce it first, so the test fails without the fix;
- adding logic with real branching: parsing, scoring, permissions, state
  transitions, concurrency, protocol/format handling.

Do not write tests that:

- restate the implementation, or assert constants, config, generated code, or
  types the compiler already checks;
- mostly verify mocks or that a function was called;
- render a component just to check that markup exists;
- duplicate an existing test with different literals.

Prefer one test through a real boundary (HTTP handler, DB, judge) over several
unit tests of internals. Removing a test that checks nothing is welcome. A change
without new tests is normal when nothing above applies.

## Checks

- While working, run only what covers the change: the affected Go package
  (`go test ./backend/<pkg>/...`), the affected frontend test file, or
  `task fe:typecheck`. Do not run the full suite after every edit.
- Once, before pushing a PR: `task check`. The pre-commit hook already lints
  staged files, and the commit-msg hook validates the subject.
- Integration tests (`task be:test:integration`) are slow; run them locally only
  when the change touches judging, queues, storage, or SQL they exercise.
  Otherwise let CI run them.
- After review fixes, rerun only the checks the fix touches; CI covers the rest.
- Docs, comments, and CI/deploy config changes need no test run beyond what
  hooks and CI do.

## GitHub workflow

**Size.** A change is *small* if it is obviously correct from the diff: docs,
config, copy, dependency bumps, or a localized fix of roughly under 100 changed
lines with no schema, API-contract, security, or judging changes. Everything
else is *normal*.

**Issues.** Normal changes start from an issue; small ones may go straight to a
PR. Write issues with the templates in `.github/ISSUE_TEMPLATE/`: the problem
and the intended result, plus constraints or open questions if any. Do not
invent acceptance checklists. Labels: `bug`, `enhancement`, or
`technical-task`.

**Branches and commits.** Never commit to `main`. Branch as
`issue/<number>-<slug>` (or `<type>/<slug>` with no issue). Commit subjects and
PR titles follow Conventional Commits, at most 65 characters, no trailing period.

**PRs.** Create with `gh pr create --body-file <path>`. The body links the issue
(`Closes #N`, or `Refs #N` for partial work) and explains only what the diff
does not: why, trade-offs, breaking changes, checks that failed or were skipped.
Do not list the checks that passed.

**Review.** Normal PRs get one independent review in a fresh context against the
issue and the diff. Small PRs need no review. The reviewer reports only real
problems — bugs, behavior that misses the issue, risky design — each with a
file/line pointer. No style nits the linters accept; ask for a test only when
naming a concrete behavior that could break unnoticed. Post the verdict
(`APPROVE` or `REQUEST_CHANGES`) with `gh pr review --comment --body-file <path>`.
Fixes that only address the review's findings do not need another review;
re-review only when the fix changes the design or adds new behavior.

**Merge** only when the owner says so, with green CI on the current head (and an
approval for normal PRs): `gh pr merge --squash --delete-branch
--match-head-commit <head-sha>`.
