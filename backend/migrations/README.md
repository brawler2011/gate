# Schema

Edit `20261112000000_init.sql` in place; do not add migrations.

`backend --migrate` (the Helm migrate Job or local `task be:migrate`) compares the
migration hash with the database. An unchanged hash preserves data. A changed or
missing hash empties all S3 buckets, deletes JetStream streams and
recreates the PostgreSQL `public` schema. Startup recreates streams and buckets.

Replace this reset policy with rollback-safe migrations before retaining user data.
