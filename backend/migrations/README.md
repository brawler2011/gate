# Schema

Gate has no users yet, so data is disposable. The schema is one editable file,
`20261112000000_init.sql`: change it in place, and do not add migrations.

`backend --migrate` (the Helm migrate Job before every deploy, `task migrate:be`
locally) hashes this directory and compares the hash with the one stored in the
database:

- same hash: nothing happens, data is kept;
- different or no hash: every S3 bucket is emptied, the Redis database flushed, the
  JetStream streams deleted, and the `public` schema dropped and created from scratch.
  The backend recreates streams and buckets on startup.

When Gate gets real users, freeze this file as the baseline and go back to
rollback-safe migrations (expand/contract, see #212).
