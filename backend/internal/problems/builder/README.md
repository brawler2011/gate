# Publication integrity

Publication is `EnqueueBuild` → `Pipeline.Execute` → `PublishBuild`. The former
`PublishProblem` entry point and separate mutable manifest/test metadata are no
longer used. A problem UUID identifies the editable problem; a draft revision
identifies an atomic authoring save. A package UUID identifies an immutable build
of that revision, with its own archive SHA-256. Rebuilding a revision can create
another package; editing a problem does not change an existing package.

## Required files and metadata

`Draft.Validate` runs both on saves and, with complete selections required, when
enqueueing a build. All supported types require a nonblank title, positive time
and memory limits, at least one test, and valid component source files and declared
source dependencies. Paths must be safe and assets must not duplicate text paths.
Component selections must reference components of the appropriate role, using a
language available in the pinned toolchain.

- `pass-fail` and `scoring` require a checker. Each test requires either a provided
  answer object or a solution component that generates the answer. An optional
  main solution is checked against provided answers.
- `interactive` requires an interactor and a main solution. Its tests explicitly
  use interactive answer mode and must not reference ordinary answer objects or
  answer generators. The build runs the solution with the interactor.
- Manual inputs reference available objects in the same organization. Generated
  inputs reference a generator component, explicit arguments and a seed. The
  generator executes during the build; the package contains its resulting input.
- Test IDs and positive ordinals must be unique. Every test belongs to an existing
  group. Group IDs and positive ordinals are unique, policies are `complete` or
  `each`, and dependencies reference existing groups without duplicates or cycles.
  Empty groups are allowed. Scores are derived from authored test weights;
  scoring max score equals the group total. Other types have no max score.
- A validator is optional. When selected, it runs on every materialized input.
  Required components must compile, generated files must be produced, and checker
  or interactor execution must succeed before the archive can be published.

The ready GFMT v2 archive contains `problem.yaml`, the author snapshot at
`.gate/snapshot.json`, source files/assets, compiled components and dependencies,
and every declared test input and noninteractive answer. Generated tests have no
remaining execution recipe in the ready manifest. `gfmt.OpenBuiltPackage` checks
the final tree, including regular files, selections, test modes and group graph,
before archiving. Missing or changed input objects fail materialization with the
object UUID and affected test input/answer or asset path.

## Version and failure boundaries

Draft saves and paired uploads atomically replace files and metadata under an
expected revision check. An incomplete upload never becomes a draft reference.
Enqueue locks that revision, validates it, and persists the entire snapshot,
including text files, metadata, limits, toolchain and immutable input object roots.
Later saves can proceed while the worker builds exclusively from that snapshot.
The worker verifies each input object's size and SHA-256 against its registry row.

The archive includes the snapshot, materialized tests and manifest from the same
execution. After registering and uploading it, the worker streams the stored
archive back and checks its size and SHA-256 before calling `PublishBuild`. This
adds one archive read per successful attempt, with bounded memory. A missing,
truncated or changed upload is an infrastructure failure eligible for retry from
the original snapshot; it never produces a ready package.

`PublishBuild` checks attempt ownership and lease, then atomically changes the
output object to available, inserts the package with the build's revision and
metadata, releases the output hold, and marks the build successful. Commit failure
rolls these changes back; an ambiguous commit is resolved by looking up the durable
package. Retries and late workers cannot splice in a newer draft or publish an
unowned output. Failed uploads remain under the existing retention/GC protocol.
As with the existing immutable object model, storage must preserve objects after
verification; this protocol does not provide recovery from subsequent data loss.

The `TestPublication*` integration regressions use real PostgreSQL, SeaweedFS S3
and go-judge. `TestProblemPipelineRealServices` additionally covers generated inputs
and answers with a validator. Run them from the repository root:

```sh
mise exec -- task test:be:integration -- ./backend/tests/integration/postgres -run 'TestPublication|TestProblemPipelineRealServices' -count=1
```
