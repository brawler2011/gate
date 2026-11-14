# Optional fields in updates

Investigation for #46. This describes representative user and contest update paths,
not a universal PATCH or JSON Merge Patch protocol. The OpenAPI schema in
`contracts/core/v1/openapi.yaml` decides whether a field accepts JSON null;
a nullable database column alone does not grant an API operation that ability.

## API decode, domain input, and persistence

Ogen decodes optional, non-nullable scalars into `Opt*` (`Set`, `Value`):
omission leaves `Set=false`, explicit null fails decoding with HTTP 400, and a
value sets `Set=true`, including `false`, `0`, or `""`. Validation may then reject
the value. Nullable optional scalars use `OptNil*` (`Set`, `Null`, `Value`):
omitted is unset, null is set and null, and a value is set and non-null.
Generated types must stay at the API boundary; do not infer presence from a zero
value or use `Or(default)` when mapping a patch.

| Representative field | Omitted | JSON null | Explicit value | Domain and SQL mapping |
| --- | --- | --- | --- | --- |
| User `username`, `email`, `role` | Preserve | HTTP 400 | Replace if valid; empty username/email fail validation | REST maps `IsSet()` to a pointer in `UpdateUserInput`; nil becomes SQL NULL and `COALESCE` preserves the column |
| Contest `description` | Preserve | HTTP 400 | Replace, including `""` | Pointer plus `COALESCE` |
| Contest `enable_drafts` | Preserve | HTTP 400 | Store `true` or `false` | Present key in `Settings` patch map; merged into JSONB |
| Contest `freeze_duration_minutes` | Preserve key | Delete key | Store nonnegative integer, including `0` | Null becomes a present map key with nil value, marshals as JSON null; JSONB merge then `jsonb_strip_nulls` removes it |
| Contest `start_time`, `end_time` | Preserve | Clear to SQL NULL | Replace timestamp | Pointer plus independent `StartTimeSet` / `EndTimeSet`; SQL `CASE WHEN ..._set THEN ... ELSE ... END` |
| Contest `freeze_status` | Preserve | HTTP 400 | Store `auto`, `frozen`, or `unfrozen` | Present key in the settings patch; no update-time default |

The mapping is in `internal/transport/rest/core/users.go` and `contests.go`.
`internal/identity/users.go` validates and forwards the user input unchanged
through its account transaction; `UpdateUserParams` aliases `UpdateUserInput`.
`internal/contests/contests.go` forwards the contest input through its authorized
transaction and invalidates cached reads after success. Repositories in
`internal/postgres/users.go` and `contests.go` map these inputs to SQLC arguments;
the source queries are `users.sql` and `contests.sql` in that directory.

A nil contest `Settings` pointer supplies no JSON patch. A non-nil map supplies
only changed keys; unrelated keys survive the JSONB concatenation. An empty map
has no replacements. `jsonb_strip_nulls` acts on the merged object, so stored
object null members are removed as well. Clearing a setting removes its stored
override; read-time policy can then apply its default. This is different from
storing numeric zero or boolean false.

For dates, the set flag is authoritative: false preserves even if a caller supplies
a non-nil timestamp. True with nil clears; true with a timestamp replaces. A plain
`*time.Time` would lose the difference between omission and clearing. REST checks
ordering when both non-null dates are supplied.

## Internal optional user fields

`UpdateUserInput` also has `AvatarUrl`, `ExpiresAt`, and `IsEmailVerified` pointers;
these are not properties of `UpdateUserRequestModel`. For repository updates,
nil preserves the existing value and a non-nil pointer writes its value. In
particular, `&false` resets email verification and `&""` writes an empty avatar
string. `identity/avatars.go` uses that empty string sentinel for avatar deletion;
read paths treat nil and empty as absent. It does not store SQL NULL on deletion.

Although email, avatar, and expiry can be NULL in storage and nil on reads,
the generic user update cannot clear them to SQL NULL: its pointers encode only
preserve/replace. No public nullable update contract promises such clearing.
Adding that operation would require an explicit contract and presence marker
through the domain and SQL, rather than changing pointer types in isolation.

## Evidence and decision

- `tests/integration/optional_updates_test.go`: raw HTTP user patches verify
  omission, replacement, null rejection, empty-value validation and rejected mixed
  updates without writes. Repository round trips verify nil preservation,
  explicit false, and the avatar empty string sentinel against PostgreSQL.
- `tests/integration/contest_dates_test.go`: all nine omitted/null/value date
  combinations round trip through HTTP and PostgreSQL; settings checks cover
  omission, null deletion, zero, false, empty description, and freeze mode presence.
- `internal/transport/rest/core/patch_fields_test.go`: ordinary Go tests exercise
  generated HTTP decoding and mapping for boolean patches, preservation of the
  existing freeze mode, and problem metadata optional fields.

The investigation found that `UpdateContestRequestModel.freeze_status` had an
OpenAPI default of `auto`. Ogen applied it during decode, marking an omitted
field present and resetting a frozen/unfrozen contest on an unrelated PATCH.
Removing that update default preserves omission; explicit `auto` still replaces
the mode. Regression tests seed a frozen mode and check both cases.

Retain the current domain representations. No optimization or allocation claim
is proposed, and no broad model replacement or benchmark is warranted by this
behavioral investigation. A pointer alone is not evidence of heap allocation;
any later performance proposal needs allocation/escape measurements and measured
impact on a representative path before changing models.
