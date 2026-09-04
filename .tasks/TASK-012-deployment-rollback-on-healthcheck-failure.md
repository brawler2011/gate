---
id: TASK-012
title: "Implement automated deployment rollback on healthcheck failure"
status: todo
type: impr
description: "Add automated rollback in CI/CD deployment pipelines if service healthchecks fail."
priority: high
created_at: 2026-08-20
tags:
  - ci
  - deploy
  - traefik
  - infra
---

# TASK-012: Implement automated deployment rollback on healthcheck failure

## Context
Imported from YouGile TID-286 ("Откат деплоя при healthcheck fail и тп"). The current deployment workflow (.github/workflows/deploy.yml) updates containers via Docker Compose / Traefik. If a new version fails its healthcheck after rollout, the environment can remain degraded. An automated rollback step must restore the previous stable container tags and routing configuration.

## Acceptance Criteria
- [ ] Post-deployment healthcheck verification loop added in deploy workflow
- [ ] On healthcheck failure, workflow automatically rolls back to previous stable container images
- [ ] Traefik routing stays pointed at healthy instances during rollover
- [ ] Failure alert and rollback notification dispatched in deployment logs
- [ ] Workflow syntax and pre-commit checks pass

## Implementation Notes
Maintain previous image digest/tag in deploy job state for seamless zero-downtime rollback.
