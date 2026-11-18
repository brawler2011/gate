# Publication integrity

Publication is `EnqueueBuild` → `Pipeline.Execute` → `PublishBuild`. The former
`PublishProblem` entry point and separate mutable manifest/test metadata are no
longer used. A problem UUID identifies the editable problem; a draft revision
identifies an atomic authoring save. A package UUID identifies an immutable build
of that revision, with its own archive SHA-256. Rebuilding a revision can create
another package; editing a problem does not change an existing package.

## Required files and metadata

`Draft.Validate` runs both on saves and, with required components enforced, when
enqueueing a build. All supported types require a nonblank title, positive time
and memory limits, at least one test, and valid component source files and declared
source dependencies. Paths must be safe and assets must not duplicate text paths.
Every path belongs to the draft layout: component sources in their role folder
(`checkers/`, `validators/`, `interactors/`, `generators/`, `solutions/`), shared
files and all source dependencies in `lib/`, statements in `statements/`, statement
resources in `assets/`.
A draft holds at most one checker, validator and interactor; the role folder
identifies each, so there is no selection. A component's language comes from its
file extension (`.cpp`, `.cc`, `.cxx`, `.py`, `.go`, `.java`) and must be available
on the judge image (checked at enqueue and again when the build runs, which uses
the languages of the image it runs on). The main solution is the only selected component.

- `pass-fail` and `scoring` require a checker. Each test requires either a provided
  answer object or a solution component that generates the answer. An optional
  main solution is checked against provided answers.
- `interactive` requires an interactor and a main solution. Its tests explicitly
  use interactive answer mode and must not reference ordinary answer objects or
  answer generators. The build runs the solution with the interactor, which is
  called as `interactor <input> <output> [<answer>]`; as in Polygon, its output
  on the main solution becomes the test answer. An optional checker then runs
  with `<input> <interactor output> <answer>`, during the build and when judging
  after the interactor accepts, and decides the verdict.
- Manual inputs reference stored objects in the same organization. Generated
  inputs reference a generator component and its arguments, passed as they are.
  The generator executes during the build; the package contains its resulting
  input.
- Test IDs and positive ordinals must be unique. Every test belongs to an existing
  group. Group IDs and positive ordinals are unique, policies are `complete` or
  `each`, and dependencies reference existing groups without duplicates or cycles.
  Empty groups are allowed. Scores are derived from authored test weights;
  scoring max score equals the group total. Other types have no max score.
- A validator is optional. When present, it runs on every materialized input.
  Required components must compile, generated files must be produced, and checker
  or interactor execution must succeed before the archive can be published.

The ready GFMT v2 archive contains `problem.yaml`, the author snapshot at
`.gate/snapshot.json`, source files/assets and every declared test input and answer. Generated tests have no
remaining execution recipe in the ready manifest. `gfmt.OpenBuiltPackage` checks
the final tree, including regular files, the checker or interactor source named in
`programs` (a checker, or an interactor for interactive problems), test modes and group graph,
before archiving. Missing or changed input objects fail materialization with the
object UUID and affected test input/answer or asset path.

A package holds no binaries and does not depend on a judge image. The judge
compiles the checker and interactor with its own image on first use. Builds and
the judge of a worker share compiled programs in `<work_dir>/programs/<key>`
(`sandbox.ProgramCache`), keyed by the source, language, library files and the
compiler (the image digest together with its language configuration): a rebuild
does not recompile unchanged programs, and the judge reuses what the build
compiled. Verdicts are reproducible against the current image only.

## Version and failure boundaries

Draft saves and paired uploads atomically replace files and metadata under an
expected revision check. An incomplete upload never becomes a draft reference.
Enqueue locks that revision, validates it, and persists the entire snapshot,
including text files, metadata, limits and immutable input object roots.
Later saves can proceed while the worker builds exclusively from that snapshot.
The worker verifies each input object's size and SHA-256 against its registry row.

The archive includes the snapshot, materialized tests and manifest from the same
execution. The worker uploads it to a fresh key and sends the archive's expected SHA-256
with S3 `PutObject` as a base64 `ChecksumSHA256`. The pinned aws-sdk-go-v2 S3
v1.68.0 does not send this checksum by default, so `UploadFileWithSHA256` sets it
explicitly from the digest already computed during archiving. Author uploads use
the same call with the digest computed while spooling the request body.

`TestSeaweedUploadChecksumEnforcement` proves that the real integration server,
`chrislusf/seaweedfs:4.53_full`, accepts matching SHA-256 and rejects mismatched
body/checksum pairs with the server's `BadDigest` error. Merely accepting a checksum
header would not satisfy this contract. The server validates the write, so a
successful publication requires no archive read-back. Truncated, empty or changed
upload bytes and failed upload acknowledgements are infrastructure failures
eligible for retry from the original snapshot; they never produce a ready package.
The publication regressions alter the body while preserving the expected digest,
require `BadDigest`, and forbid archive downloads during successful retries.

`PublishBuild` checks attempt ownership and lease, then atomically inserts the
output object's row, inserts the package with the build's revision and metadata,
and marks the build successful. The same
transaction stores the statement-facing content outside the archive: sample
inputs and answers, taken from the materialized tests, as `problem_packages.samples`,
and one `problem_package_assets` row per statement asset that references the
draft's stored object. Statement reads and image requests never open the archive,
and asset objects stay referenced until the package is pruned. Commit failure
rolls these changes back; an ambiguous commit is resolved by looking up the durable
package. Retries and late workers cannot splice in a newer draft or publish an
unowned output. Bytes of a failed or stale attempt never get a row; GC deletes
them as orphans (see below).
As with the existing immutable object model, storage must preserve objects after
the acknowledged write; this protocol does not provide recovery from subsequent data loss.

## Stored objects

A `stored_objects` row exists only for bytes that were uploaded with a verified
SHA-256, and it is inserted in the same transaction as its first reference (a
draft test or asset, or a package). Every reference is a composite foreign key
`(object_id, organization_id)` with `ON DELETE RESTRICT`, so Postgres rejects a
reference to another organization's object and refuses to delete a referenced row.
Builds, packages and template clones share objects through these references.

GC deletes rows that nothing references, one per transaction; a foreign key
violation means a reference was committed meanwhile and the row stays. It then
deletes the row's bytes. A sweep deletes bytes without a row that are older than
`object_retention`: keys are random UUIDs that are never reused, and a writer
commits its row only before its operation lease or build deadline, which
`object_retention` must exceed. An object is deleted on the first GC pass after
it loses its last reference; there is no grace period.

The `TestPublication*` integration regressions use real PostgreSQL, SeaweedFS S3
and go-judge. `TestProblemPipelineRealServices` additionally covers generated inputs
and answers with a validator. Run them from the repository root:

```sh
mise exec -- task be:test:integration -- ./backend/tests/integration -run TestSeaweedUploadChecksumEnforcement -count=1
mise exec -- task be:test:integration -- ./backend/tests/integration/postgres -run 'TestPublication|TestProblemPipelineRealServices' -count=1
```
