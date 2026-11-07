# Phase 2: policy layer

The new API consists of `RequireOrganization`, `RequireTeam`, `RequireProblem`,
`RequireContest`, the `RequireGrant*` checks, and the owner transition functions.
All take typed values from `access/model`, perform no I/O, and use the supplied
`PolicyInput.Now`. They do not accept repositories, transactions or callbacks.
A failed facts load must be passed as `LoadError`; only confirmed missing grants
become empty roles. Expired actors fail with unauthenticated, including at expiry.
Unknown actions, roles and settings fail closed.

Each action evaluator checks the RFC §3 action gate. A `manage_members` gate is
not authorization for an arbitrary role change: use the corresponding
`RequireGrant*` function with both the current and requested role. The same check
applies when issuing and accepting invitations. Empty roles represent absent or
revoked grants. Resource-team links require the resource ACL check and a team
from the same organization; team edit alone is insufficient.

Contexts contain facts for the current actor and resource. Team facts must
represent direct active memberships, without parent-team inheritance. The
resolver takes the highest valid grant; inactive and foreign-organization grants
contribute nothing. Problem direct roles remain owner/moderator/viewer; team
read/write/admin roles use a distinct type. Contest teams cannot be owners.
Org/global admin contributes operational administration, never ownership.

`ContestConditions` supplies the matrix's T predicates. It intentionally does
not implement contest phases, settings normalization, freeze or snapshots in
this PR. A later context builder must derive these conditions from validated
settings and one server clock. Nil conditions deny gated operations. Conditions
must include the relevant audience/business restrictions; they must never come
from a client. Submission-specific checks also require the actual submission's
owner and contest IDs. Manager access does not bypass official submission gates
or the requirement for participant admission.

Owner changes require the before/after explicit active owner sets. Inherited
organization owners authorize resource ownership changes but do not count toward
the explicit resource owner invariant. A set must contain at least one distinct,
permanent, active owner; a primary owner, if used, must remain in that set. Grant
changes cannot change other owners at the same time. Transfer planning returns a
new set with the recipient added before the previous owner is removed and updates
the primary owner when present. The caller must persist the complete transition
atomically. Account expiry, deactivation and bulk removal must validate their
resulting owner sets too. Existing owner_id/direct membership inconsistencies and
legacy contest team owner data need auditing before adopting the new policies.

Metadata reads connect these checks through `MetadataPermissions`, using
pool-backed `PolicyFactsRepo` reads. Other actions still delegate to legacy
`PermissionsUseCase` pending the lifecycle and command migration. There is no new
DTO contract, revision schema or mutation guard. A pure owner precheck cannot enforce concurrent
mutation integrity or immediate revocation. Before connecting write paths, add
RFC §8 nonlocking facts loading, optimistic revisions/decision guards and the
serializable owner mutation protocol (with fresh external prechecks after retry).
Do not call authorization again inside a write transaction.

The shared RFC matrix rows live in `backend/tests/accessfixtures`; policy tests
exercise the role columns, public/closed branches and T gates. Separate tests
cover inheritance, admission, grant transitions, owner sets and transfer planning.
PostgreSQL parity tests in `visibility_integration_test.go` compare the same
persisted actor/resource facts against pure policies and SQL scopes, with explicit
expected results for every grant branch.

## Metadata and list reads

`visibility.sql` scopes problems, contests and teams before count and pagination
in one statement. Team-resource lists add a link filter to those same scopes;
viewing a closed team does not grant access to its private resources. NULL and
zero UUID viewers are guests; expired or unknown persisted actors match no rows.
Use cases reject expired identities with unauthenticated. `WithReadTime` fixes
one server clock for the request, used by policies and SQL expiry checks.

`total LEFT JOIN page` preserves totals past the last page. A nullable JSON
resource represents the empty-page sentinel, which the adapter removes before
mapping results. SQL grants use EXISTS, never per-row Go permission checks.
Legacy `unlisted` visibility is rejected, matching the typed policy's supported
public/private values. Unknown roles and participation modes also fail closed.

`PolicyFacts` exposes batch loaders for future capability projections: pass all
returned page IDs to the appropriate loader, then evaluate the pure policies
with one `PolicyInput`. Each nonempty batch uses one nonlocking SQL query;
missing IDs are absent and read errors propagate. Empty batches perform no I/O.
The metadata adapter uses the same loaders for individual checks. Contest
conditions and capability DTOs remain the responsibility of later PRs.
