# Gate diagnostic logs and removal of the observability stack

Gate writes sanitized JSON through one `slog` handler to stdout in every environment.
There is no Gate OTel registration, exporter, distributed tracing, metrics provider,
collector, centralized log store or Grafana deployment. Next.js still reports ordinary
server errors; the API facade logs only operation, status, validated requestId and a
safe error type. `NEXT_TELEMETRY_DISABLED=1` continues to disable Next.js telemetry.

## Fields and investigation

HTTP records have `event=http_request_completed`, `request_id`, `method`, `route`,
`status` and `duration_ms`. The route is the ogen path pattern, `/ws/*`, `/files/*`
(local storage), or `unmatched`. Request paths, queries, headers, bodies and IPs are
not recorded. There is one completion record per request; a separate operation
error may also appear. A WebSocket record appears when its HTTP handler returns
(normally when the connection closes), with status 101.

`X-Request-ID` is returned to callers and included as `request_id` in API error
responses. Valid input is 1–128 ASCII letters, digits or `._:-`; other input gets a
new UUID. A supplied ID is a diagnostic label, not authenticated identity or a
uniqueness guarantee. It can be shared deliberately across a user's requests.

Submission records use these events:

| Event | Meaning |
| --- | --- |
| `submission_accepted` | Persisted/committed submission; immediately disqualified submissions include state and have no queue event |
| `submission_rejudge_requested` | Rejudge transaction committed, one record per affected submission |
| `submission_queued` | JetStream acknowledged publication, before marking outbox completed |
| `submission_judging_started` | Worker is about to call `JudgeSubmission` |
| `submission_result` | Usecase saved the verdict and attempted business result notifications |
| `submission_judging_completed` | Worker received successful return from the usecase |
| `submission_judging_failed` | Usecase returned an error; includes operation, error type/kind and duration |

Submission logs carry `submission_id`, available `request_id`, and async `event_id`
when known. Worker logs include `delivery_attempt` from JetStream metadata when
available. The outbox error record also has `aggregate_id`. IDs are carried in the
existing persisted outbox headers and in NATS `X-Request-ID`,
`Gate-Outbox-Event-ID`, `Gate-Queued-At` (RFC3339Nano). Business JSON, database schema,
consumer settings, locking, retries, deadlines and Ack/Nak decisions are unchanged.
Duplicates can occur after publish succeeds but marking completed fails, or on
redelivery; use event ID and delivery attempt when interpreting records.

`queue_wait_ms` measures from **outbox event creation** to worker start. It includes
outbox polling, NATS waiting and retries. A rejudge has a new outbox creation time;
it does not use the submission's original creation time. Legacy or malformed
metadata does not block processing. Unknown queue wait is omitted, with
`queue_wait_unavailable_reason=missing_queued_at|invalid_queued_at`; no age or
identifier is invented. `duration_ms` on worker completion/failure covers the
whole `JudgeSubmission` call, including loading, preparation and result persistence.

For a known request ID, save a snapshot before investigating:

```sh
docker compose -f deploy/prod/docker-compose.yml logs --no-color --since 1h backend > gate-backend.log
rg 'request-123|submission-uuid|event-uuid' gate-backend.log
```

Docker prefixes may precede the JSON; use `--no-log-prefix` for JSON-only backend
output. Other services and frontend output are not guaranteed to be JSON. Restrict
access to log snapshots: key/group redaction and Bearer/Basic prefix masking are
not a general scrubber for arbitrary message text, URLs or error objects. New
request/queue logs deliberately avoid serializing those values.

All remaining services in local/prod Compose and shared Traefik use `json-file`,
`max-size=10m`, `max-file=3`, approximately 30 MB per container. This is a size
limit, with no promise of days retained. It applies to newly created/recreated
containers; a restart alone does not update the driver options. Verify actual
settings after rollout:

```sh
docker inspect --format '{{json .HostConfig.LogConfig}}' gate-backend-prod
docker inspect --format '{{json .HostConfig.LogConfig}}' traefik-global
```

Local backend/frontend run outside Compose; their stdout is **not** Docker logs.
Capture their terminal output separately if needed. Logs disappear on rotation,
container deletion or host loss unless saved elsewhere. Removed capabilities
include distributed waterfalls, historical RED/runtime/queue/verdict graphs,
percentiles, centralized search, logs↔traces navigation and browser telemetry.
External availability checks/notifications are a separate task; `/health` is a
static HTTP response, not proof that dependencies or judge work.

## Dependency boundary and verification

Gate's source/generated Go code has no OTel imports. `contracts/ogen.yml` disables
`ogen/otel`; `task contracts:gen-openapi` explicitly loads it. Generated OTel-specific
Go options are removed. External HTTP and WS contracts remain unchanged.

`go list -deps ./...` in backend contains no OTel packages. The test graph
(`go list -deps -test ./...`) still includes OTel through testcontainers → Docker
client's HTTP instrumentation. `go mod why -m` can also include dependency-owned
tests: gRPC tests require the OTel API, Docker client tests require OTLP trace
exporter/proto modules. Ogen itself declares OTel in its own module graph, but
uninstrumented generated contracts neither import nor require it directly.
Next.js retains its internal tracing API/optional OTel peer declaration. None of
these library-owned dependencies registers Gate exporters; replacing/forking the
libraries is outside scope. `go mod tidy` and `bun install` manage manifests/locks.

The revised `task test:e2e` performs repository deployment/contract checks; it does
not simulate production delivery. Backend unit tests exercise actual sanitizer,
HTTP streaming/WS, worker Ack/Nak and outbox publication behavior. Integration tests
exercise real PostgreSQL/NATS delivery from an HTTP-created submission with the
sandbox usecase replaced at the worker boundary; separate existing sandbox and
strategy tests cover actual judging. CI runs frontend tests and deployment checks.

## Production transition (separate, manually scheduled operation)

Agree on maintenance window, acceptable interruption, rollback duration and
archive storage/access/retention before proceeding. No production deployment or
volume deletion is part of TASK-021 implementation.

1. Inventory the **actual** Compose project, container IDs, immutable image
   digests, mounts, configurations and logging settings. Compare to the repository.
   Store environment securely; do not print secrets in reports.
2. Back up old Compose, Collector/Grafana/Tempo/Loki configuration, shared Traefik
   middleware and environment to protected storage. Pin the previous image digests
   and ensure those images remain pullable. Save new image digests as well.
3. The normal workflow uses mutable environment tags, `up --remove-orphans` and
   image prune, with no automatic rollback. For this transition use a manual,
   digest-pinned Compose override and do not run prune during acceptance/rollback.
   Do not use the normal workflow to bypass the staged teardown below.
4. Keep the old telemetry services/configs available while upgrading backend and
   frontend to the new immutable digests. Check `/health`, authenticated API, WS
   message transfer, S3 upload/download, successful and failed submission judging,
   rejudge and application restart. HTTP `ok` alone is insufficient.
5. After smoke succeeds, stop **only** `otel-collector`, `tempo`, `loki`,
   `victoriametrics`, `grafana` in the inventoried old project. Apply final Compose
   without blind `--remove-orphans`, then remove only those stopped container IDs.
   Preserve all volumes. Do not stop the shared Traefik service wholesale.
6. Update the separately delivered shared Traefik dynamic configuration. Confirm
   API/WS/S3 work and Collector/Grafana routes no longer exist. SCP overwrite does
   not delete stale files: after backup, remove only the agreed obsolete telemetry
   configuration files on the host.
7. Confirm JSON backend records, actual rotation options via inspect, absence of
   exporter errors/old containers/routes, and correct request/submission/rejudge
   correlation. Save acceptance logs. After the agreed rollback window, remove
   obsolete OTel/Grafana variables and secrets.

Rollback: restore old configs/environment/Traefik first, then bring the five
services back on their original volumes and restore backend/frontend by the
saved immutable digests. Check API, WS and real judge completion. No database
migration is required; older code can ignore the new diagnostic headers.

## Telemetry volumes: separate explicit deletion decision

Repository names are `gate-tempo-data-prod`, `gate-loki-data-prod`,
`gate-victoriametrics-data-prod`, `gate-grafana-data-prod`. Confirm actual mounts
and project names; these are expected names, not authorization to delete them.
After stopping services, archive each actual volume, calculate and record checksums,
and test restoration into fresh isolated volumes using old configs/images. Retain
original volumes until the rollback window ends. Record archive locations,
permissions, checksums and restore results. Removal requires a separate decision
with an explicit list of names. Never use `down -v` or volume prune; business
volumes are outside scope. Removing declarations from Compose does not delete data.
