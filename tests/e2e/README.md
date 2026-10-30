# Required checks and test boundaries

`task test:e2e` runs repository contract checks. The directory name is historical;
these tests do not start a deployed application.

| Check | Actual boundary | What it does not establish |
| --- | --- | --- |
| Deployment services/rotation | Parse checked-in Compose YAML and inspect services, volumes, ports and logging | Container startup, networking or log rotation at runtime |
| Traefik business routes | Parse/read checked-in route and middleware configuration | HTTP passing through Traefik or forwarded headers reaching the backend |
| Outbox locking/headers contract | Read SQL and generated query text | Concurrent database locking or message delivery |
| No Gate instrumentation | Scan Go/TS source for removed instrumentation imports | Telemetry export or browser behavior |
| CI required gate | Execute the actual `ci-status` Bash script from `ci.yml` for success and each dependency's failure/cancellation/skip; compare dependencies to all CI jobs | GitHub runner scheduling or a live merge attempt |

Real diagnostic delivery is covered by
`backend/tests/integration/submissions_test.go` (`TestIntegrationSuite/TestSubmissions`)
and `diagnostic_delivery_test.go`: the generated HTTP client calls an in-process
production HTTP handler with test authentication, writes PostgreSQL, and a real
outbox worker publishes to NATS JetStream. The production judge worker receives
submission/rejudge IDs and request IDs; a test double replaces sandbox judging.
This does not traverse a browser, Traefik, production authentication, or go-judge.

`TestJudgeFullFlowE2E` exercises production use cases/repositories against
PostgreSQL, JetStream, SeaweedFS S3 and go-judge containers. It calls use cases
directly and does not traverse HTTP, Traefik or the deployed frontend. Removed
synthetic frontend-to-Tempo and request-object header tests must not be recreated
as evidence of those boundaries. Frontend production/SSR/CSR coverage belongs to
TASK-013.

CI runs contracts/codegen, repository tool tests, backend lint/unit/integration,
frontend typecheck/lint/Knip/tests/build and a frontend Docker image/runtime
configuration check. `All CI Checks Passed` depends on every job and rejects any
non-success result even when other jobs succeed. The reusable deployment workflow
requires CI before image builds, and deployment requires those builds. The active
main ruleset requires this aggregate check with strict up-to-date validation;
repository rules are external settings and must be inspected separately from YAML.

Local Lefthook lint filters provide quick feedback on touched files. They are not
proof of full verification. `task precommit` runs contracts/codegen, tool tests,
backend lint/unit tests and frontend typecheck/lint before a PR; CI additionally
runs the integration, deployment contract, frontend test/build and image checks.
Hook/Task redesign and release artifacts are outside this audit (#57).
