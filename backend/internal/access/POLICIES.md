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

`ApplyContestLifecycle` derives `ContestConditions` from persisted settings and
one server clock. Official submissions require participant admission and the
start-inclusive, end-exclusive window, including for managers. Freeze ends at
the contest end; only managers may request live scoreboard projections. Nil conditions deny gated operations. Conditions
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
`PermissionsUseCase` for unmigrated actions. Contest lifecycle reads and commands
use `MetadataPermissions`; DTO capabilities remain for the contracts PR. A pure owner precheck cannot enforce concurrent
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
The metadata adapter uses the same loaders for individual checks. Capability DTOs remain the responsibility of the contracts PR.


## Contest commands, snapshots and freeze

Attach/relink policies require destination management, source use permission,
matching package ownership and ready status. Source and package facts are read
through the pool before the write transaction. Contest problem reads join only
the link and pinned package, including catalog metadata captured at publication.
Statements and samples never fall back to workspace for a supplied package ID.
Submission creation captures the pinned package in the INSERT; judging and
rejudging retain that package even after a contest relink. Referenced packages
cannot be deleted; a linked source problem must first be detached.

Migrated contest commands and submission creation capture a conservative global
security revision before their external policy checks. Statement triggers bump
it atomically on changes to actors, grants, resources, links and problem blocks.
A SERIALIZABLE write transaction compares that revision without row locks and
checks the account/contest deadline using the database clock before commit.
Serialization failures restart eligible commands with a fresh external precheck;
there is no policy evaluation inside their write transaction. The shared revision
can reject commands after unrelated security changes; resource-specific revisions
would reduce such conflicts. Legacy ACL mutation authorization still needs its
separate migration; this guard does not claim to repair those older write paths.

Submission state history records verdict observation time. Masked standings use
states observed strictly before the freeze boundary, preventing late verdicts or
rejudges of pre-freeze submissions from leaking into the frozen board. Existing
rows have no trustworthy historical observation time, so migration records them
at migration time and masks them conservatively for older freeze boundaries.
Manual freezes record their boundary in server-owned contest settings.
