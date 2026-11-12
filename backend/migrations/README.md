# Migrations

A failed release is rolled back by image only: the previous code then runs on the
**new** schema. Every migration must therefore work with the previous release's code.

## Rules

1. **Expand only.** Add tables, nullable columns, columns with a constant default,
   indexes. `DROP`, `RENAME`, type changes and `SET NOT NULL` wait for a later release,
   once the previous release no longer uses the object.
2. **No Down section.** Fix a bad migration with a new one.
3. **Merged migrations are immutable.** Never edit, rename or delete them; a new
   migration's version must be newer than the latest on the base branch.

Rules apply after the baseline `20261112000000_init.sql`.

## Removing or renaming a column

Release N adds the new column, writes both and backfills it; release N+1 uses only the new one;
release N+2 drops the old one, marked for the linter:

```sql
-- +goose Up
-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN email;
```

Create indexes on populated tables `CONCURRENTLY`, in a file marked
`-- +goose NO TRANSACTION`.

Checked by `task lint:migrations` and the Migration Rollback Test CI job; run the
rollback scenario locally with `task test:migrations:rollback BASE=origin/main`.
