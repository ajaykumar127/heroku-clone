# control-plane/migrations

Plain SQL migration files for the control plane database.

## How migrations work

Files are numbered sequentially and must be applied in order:

```
001_initial_schema.sql   — apps, releases, api_tokens, config_vars
002_runtimes.sql         — Runtime Plane registry
003_jobs.sql             — job queue (build/deploy work items)
004_app_runtime_fk.sql   — runtime_id columns on apps and releases
```

Each file is idempotent (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`,
`ALTER TABLE … ADD COLUMN IF NOT EXISTS`) so it is safe to re-run against an
already-migrated database — with one exception noted below.

## Applying migrations

### PostgreSQL (production)

```bash
psql $DATABASE_URL -f migrations/001_initial_schema.sql
psql $DATABASE_URL -f migrations/002_runtimes.sql
psql $DATABASE_URL -f migrations/003_jobs.sql
psql $DATABASE_URL -f migrations/004_app_runtime_fk.sql
```

Or apply all at once:

```bash
for f in migrations/00*.sql; do
  echo "Applying $f …"
  psql $DATABASE_URL -f "$f"
done
```

### SQLite (local development / CI)

```bash
for f in migrations/00*.sql; do
  echo "Applying $f …"
  sqlite3 dev.db < "$f"
done
```

> **Note:** Migration 004 contains the PostgreSQL `ALTER TABLE … ADD COLUMN IF NOT EXISTS`
> statements by default. The equivalent SQLite statements are present but commented out.
> For SQLite, uncomment those lines and comment out the PostgreSQL lines, or handle the
> "duplicate column name" error in your migration runner.

## SQLite compatibility notes

1. **`"commit"` is always quoted** — `commit` is a reserved keyword in SQLite.
   All migration files quote it as `"commit"` for portability.

2. **No `IF NOT EXISTS` on `ALTER TABLE ADD COLUMN`** — SQLite does not support
   this clause. Migration 004 documents the two variants side-by-side.

3. **No `ADD CONSTRAINT` after table creation** — SQLite does not support adding
   constraints via `ALTER TABLE`. Foreign-key relationships are therefore modelled
   as plain `TEXT` columns ("soft FKs") and enforced at the application layer.

4. **`TIMESTAMP` type** — SQLite stores timestamps as `TEXT` in ISO-8601 format.
   The application layer must handle the difference in serialisation between
   SQLite and PostgreSQL.
