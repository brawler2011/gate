---
id: TASK-008
title: "Add judge worker pool dashboard to Grafana"
status: todo
type: feat
description: "Expose Prometheus metrics for judge worker pool and create Grafana dashboard."
priority: normal
created_at: 2026-08-20
tags:
  - observability
  - grafana
  - prometheus
  - judge
---

# TASK-008: Add judge worker pool dashboard to Grafana

## Context
Imported from YouGile TID-284 ("В grafana показывать пул воркеров."). The testing system relies on judge worker pools to execute and grade submissions. Operators currently lack visibility into pool capacity, active vs idle workers, queue latency, and worker failure rates. A dedicated Grafana dashboard and supporting Prometheus metrics are needed for observability.

## Acceptance Criteria
- [ ] Prometheus metrics exported for worker pool: total workers, active workers, idle workers, queue length, and evaluation duration
- [ ] Grafana dashboard JSON created/updated in deploy/common/grafana/ visualizing worker status and load
- [ ] Alerting thresholds configured for worker pool exhaustion and high queue delays
- [ ] Tier observability or unit tests in tests/e2e/helpers/grafana_validator.go verified
- [ ] Pre-commit validation task precommit passes

## Implementation Notes
Reuse existing Prometheus metric naming conventions in observer/v1 contracts.
