# Gate

## Installation / Prerequisites

- [Task Installation](https://taskfile.dev/docs/installation)

## Diagnostics

Gate uses sanitized JSON backend logs and container log rotation (10 MB × 3).
The local Compose runs business dependencies; backend/frontend run separately.
See [logging and production transition runbook](docs/logging-runbook.md) for
correlation fields, troubleshooting, diagnostic losses and rollback/volume policy.

## Issues and pull requests

[Gate Project](https://github.com/users/brawler2011/projects/2) tracks work from
[GitHub Issues](https://github.com/brawler2011/gate/issues), the sole task store.
Use the Bug report, Feature request, or Technical task form, including for issues
created by agents. Apply `backward-compatible` when compatibility is required.
New proposals and unresolved design questions start in Backlog; Ready requires
defined scope and verifiable acceptance criteria. Update In progress and Review
explicitly. Closed issues appear as Done in History. When reopening an issue, move it to
Backlog. PR events alone do not complete an issue.

Create a branch named `issue/<number>-<slug>`. Run `task precommit`, validate the
Conventional Commits PR title (65 characters maximum) with
`task commit:validate -- --title '<title>'`, and follow the PR template. Create
the PR with `gh pr create --body-file <path>`, using `Closes #N` for complete
scope or `Refs #N` for partial work. Record only checks actually performed.
An isolated reviewer receives the issue description, `git diff origin/main...HEAD`
and `AGENTS.md`, and posts the verdict with `gh pr review --comment --body-file <path>`.
A human squash-merges after APPROVE and passing CI. See
[migration history](https://github.com/brawler2011/gate/issues/17) for the original
specifications, completed work, cancellations, and the archived review.
