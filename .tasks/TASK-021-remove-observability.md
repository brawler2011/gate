---
id: TASK-021
title: "Replace observability stack with safe JSON logs"
status: done
type: refactor
description: "Remove Gate OpenTelemetry integration and infrastructure while preserving diagnostic correlation and business behavior."
priority: normal
created_at: 2026-10-12
tags:
  - backend
  - frontend
  - infrastructure
---

# TASK-021: Replace observability stack with safe JSON logs

## Context
Remove Gate's OTel SDKs, exporters, instrumentation, W3C propagation, generated contract instrumentation, and Collector/Tempo/Loki/VictoriaMetrics/Grafana deployment. Retain library-owned transitive dependencies, business services and interfaces, outbox headers/schema, retries and Ack/Nak semantics. TASK-008 and existing CODEBASE_REVIEW.md are outside scope. No production deployment or volume deletion is authorized by this task.

## Acceptance Criteria
- [x] Backend uses one sanitized JSON stdout slog handler initialized before configuration loading; startup errors use slog.
- [x] Public HTTP requests log exactly one completion with safe route, method, request_id, status and duration_ms; request IDs validate 1–128 ASCII letters/digits/._:-; streaming and WebSocket upgrade work.
- [x] Request IDs survive persisted outbox and NATS delivery. Submission event ID and outbox creation time reach judge; legacy/invalid metadata does not affect processing.
- [x] Accepted/rejudge events occur after persistence/commit; queued after confirmed publication; judge start/completion/failure include identifiers and timings. Queue wait uses outbox time, including rejudge; unknown wait has a reason.
- [x] Gate source and regenerated contracts have no OTel imports; ogen configuration reproducibly disables ogen/otel; manifests and lockfiles are tidied and remaining transitive dependencies explained.
- [x] Frontend instrumentation and OTel env/dependencies are removed; API error logs exclude request/body and retain ApiResult.requestId; protected Next.js files are untouched.
- [x] Both application Compose files and shared Traefik remove telemetry services/routes/volumes/env and SeaweedFS metrics endpoints; every remaining service has json-file rotation at 10m × 3.
- [x] Meaningful logging, HTTP/WS, async/rollback and infrastructure tests pass alongside business tests; CI runs frontend tests and revised deployment checks.
- [x] Codegen, precommit, integration, frontend tests/knip/build, contracts compilation, images and isolated Compose smoke are verified; limitations recorded accurately.
- [x] Runbook documents log fields, duration semantics, rotation limits, staged production transition, digest rollback, config backups and separate volume archival/deletion; isolated review approves and PR CI passes.


## Implementation Notes
Business API and WS payloads do not change. New diagnostic NATS headers are X-Request-ID, Gate-Outbox-Event-ID and Gate-Queued-At (RFC3339Nano). Old messages receive no invented identifiers. Production rollout remains separate: archive/restore/checksum actual telemetry volumes, preserve them throughout the agreed rollback window, and obtain explicit deletion approval. No down -v, volume prune or auto-merge.


## Verification
2026-10-12: backend lint (0 issues), backend tests including real sandbox, full integration suite, targeted HTTP → PostgreSQL → outbox → JetStream → judge worker and rejudge, frontend 53 tests, frontend lint/Knip/production build without OTel env, contracts compilation, and local/prod/shared-Traefik Compose config validation pass. Generated diffs remain identical after task gen. Protected frontend files and TASK-008 are unchanged. Runtime go list -deps contains no OTel packages; test graph retains Docker/testcontainers instrumentation. See docs/logging-runbook.md for dependency provenance and operational limits. Full precommit and backend/frontend image builds pass. Isolated production-topology Compose smoke verifies actual API → PostgreSQL → outbox → JetStream → go-judge Accepted/Wrong Answer and rejudge, S3 transfer, WS 101/events, JSON correlation and all twelve container rotation settings. Chromium loaded 47 requests with no OTLP. Saved config/immutable image restoration preserves volumes and repeats the business smoke successfully. Isolated reviewer verdict is APPROVE; PR #16 CI passes (run 38228947651: codegen, backend lint/unit/integration, frontend and PR title). Actual production baseline and telemetry-volume archive/restore remain rollout prerequisites.
