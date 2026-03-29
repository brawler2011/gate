# Workshop Storage Package

Storage abstraction used by the workshop use case.

## Implementations

- `S3Service`: primary implementation for production. Stores workspace files in S3 under `workspaces/{problemId}/{path}`.
- `NewInMemoryS3Service`: in-memory storage-backed `S3Service` constructor used by tests.

## Notes

- `manifest.json` is not stored by this package. Manifest is persisted in Postgres (`problems.manifest`).
- The package handles workspace files only (tests, solutions, checkers, validators, generators, interactors, media, README, etc.).
