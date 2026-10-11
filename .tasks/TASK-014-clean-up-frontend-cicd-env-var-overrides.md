---
id: TASK-014
title: "Eliminate frontend environment variable override hacks in CI/CD"
status: todo
type: refactor
description: "Refactor frontend build configuration to cleanly handle runtime and build-time env vars without CI hacks."
priority: normal
created_at: 2026-08-21
tags:
  - ci
  - frontend
  - env
  - build
---

# TASK-014: Eliminate frontend environment variable override hacks in CI/CD

## Context
Imported from YouGile TID-290 ("Костыль с переопределняими переменных окружения фронтенда в cicd"). Previous CI/CD workflows used ad-hoc sed/echo scripts to inject or rewrite environment variables before Next.js builds. Per AGENTS.md, environment variable overrides in code are prohibited. CI/CD must use standard Next.js build arguments and Docker multi-stage environment passing.

## Acceptance Criteria
- [ ] Audit all frontend CI/CD scripts and Dockerfile steps rewriting .env or build arguments
- [ ] Standardize build-time variables (NEXT_PUBLIC_*) via clean Docker build args in .github/workflows/
- [ ] Ensure runtime configuration strictly conforms to AGENTS.md (no fallback values, no ad-hoc string replacements)
- [ ] Clean build test passing in both local Docker builds and CI workflow
- [ ] task precommit:fe passes cleanly

## Implementation Notes
Never touch next.config.mjs or next-env.d.ts per AGENTS.md rules.
