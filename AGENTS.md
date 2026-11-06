# Project instructions

## Working on changes

- Keep changes focused on the task, including necessary supporting changes. Do not add unrelated cleanup or features.
- Ask when a decision materially changes the requested behavior or scope; make routine implementation decisions independently.
- Breaking changes are allowed unless the issue has the `backward-compatible` label.

## Project conventions

- Run commands from the repository root using `task` and `bun`. Find development, build, check, and generation commands in `Taskfile.yml` and frontend scripts in `package.json`.
- Install dependencies with `bun install --frozen-lockfile`.
- Use `slog` for backend logging.
- Use React Router for routing, TanStack Query for query state, and Mantine for UI.
- Use `api` and, when needed, `unwrap` from `@/lib/api`. Browser requests use same-origin `/api` and `/ws`.
- Do not override environment variables or add fallback values in code. Local settings belong in `deploy/local/.env`; variables are listed in `.env.example` alongside it.

## Generated code

Edit sources and run the corresponding generator from `Taskfile.yml`; do not hand-edit generated files. Generated directories are listed in its `GENERATED_PATHS` variable.

## GitHub

Apply this workflow when working on a GitHub issue or PR.

- Use GitHub issues for task specifications and follow `.github/ISSUE_TEMPLATE/` when creating them.
- Use issue labels `bug`, `enhancement`, or `technical-task`. Move issues to In progress when starting work and Review when ready. New proposals and reopened issues go to Backlog; closed issues go to Done. Preserve the reason when closing as not planned.
- Work on `issue/<number>-<slug>` branches; do not commit or open PRs from `main`.
- Before opening a PR, run `task precommit` successfully. See `Taskfile.yml` for what each check covers.
- Commit subjects and PR titles use Conventional Commits, at most 65 characters, without a trailing period. Validate with `task validate:commit`.
- Follow `.github/pull_request_template.md`; use `Closes #N` for complete delivery or `Refs #N` for partial work. Create PRs with `gh pr create --body-file <path>`.
- Review in a fresh context and clean worktree against the issue and actual diff. Report concrete problems with file/line pointers; a request for more tests must identify unverified behavior. Post `APPROVE` or `REQUEST_CHANGES` using `gh pr review --comment --body-file <path>`.
- Merge only on explicit human instruction, after independent reviewer `APPROVE` and passing CI for the current PR head and base. Recheck both revisions before merging; changes require renewed review and checks. Use `gh pr merge --squash --delete-branch --match-head-commit <head-sha>`.
