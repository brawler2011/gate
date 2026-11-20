# Publication integrity

Builds use an immutable draft snapshot. Later saves cannot alter an in-flight
build, and rebuilding a revision may produce a separate immutable package.

Publication requires compiled components, materialized tests, successful validation
and answer generation. The package is an ordinary gfmt archive, written like a draft
export plus every test's input and answer, so it imports back as the draft. It holds
sources and tests, not binaries; judging compiles against the running judge image.

Input objects are verified against their registered size and SHA-256. Archive
uploads must enforce the expected SHA-256 at the storage server; acknowledging a
checksum header alone is insufficient. Failed uploads cannot become ready packages.
Storage must preserve bytes after acknowledging a write.

Publication checks attempt ownership and lease, then atomically commits the object,
package, its index (title, limits, statements, samples, assets) and successful build state. An ambiguous commit is
resolved from the durable package. Stale workers cannot publish or substitute a
newer draft, and failed attempts leave only unreferenced bytes.

Object references stay within their organization and prevent deletion while in
use. GC removes unreferenced rows before their bytes and sweeps abandoned uploads.
Statement reads use separately stored content and assets without opening archives.
