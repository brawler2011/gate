---
id: TASK-007
title: "Resolve SSG build failure due to backend unavailability"
status: todo
type: fix
description: "Eliminate build-time backend dependencies causing SSG failures during container and CI builds."
priority: high
created_at: 2026-08-20
tags:
  - frontend
  - cicd
  - ssg
  - nextjs
---

# TASK-007: Resolve SSG build failure due to backend unavailability

## Context
Imported from YouGile TID-278 ("тк время билда нету доступа к бекенду, не работает SSG"). During Docker image builds and CI/CD pipelines, Next.js static site generation (SSG / generateStaticParams) fails or hangs because the backend API is not running or unreachable in the build environment. Pages requiring dynamic backend data should be rendered with dynamic server rendering or client-side SWR fetching, decoupling the static frontend asset compilation from a live backend instance.

## Acceptance Criteria
- [ ] Identify all routes attempting static data fetching during build time (generateStaticParams / pre-rendering with API calls)
- [ ] Transition dynamic data fetching to client-side SWR hooks or dynamic RSC request-time execution
- [ ] Verify bun run build:local and Docker build succeed in an isolated environment without backend access
- [ ] No fallback URLs or hardcoded dummy endpoints added to runtime code (complying with AGENTS.md)
- [ ] CI build step succeeds reliably without spinning up backend services

## Implementation Notes
Check layout and route configs in frontend/app/[slug] to ensure dynamic params or runtime headers are respected.
