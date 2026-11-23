# ADR 0004 — Database changes ship as embedded goose migrations applied at startup

Status: accepted · 2026-11-23

## Context

`db/schema.sql` and `db/seed.sql` are fixed, and reviewers rebuild with their own copies of
them and of the Compose files. Any table, index or function we need must therefore be created
by the API itself.

## Options

| Option | Cost |
|---|---|
| **goose v3 Provider, SQL files embedded with `embed.FS`, run on the existing pgx pool** | One dependency. Versioned, one transaction per file, state in `goose_db_version`, Postgres advisory lock so two replicas don't race. |
| golang-migrate | Same capability; its own driver model rather than our pgx pool. No advantage here. |
| Hand-rolled runner (~40 lines + advisory lock) | No dependency, but we would own ordering, locking and half-applied state — code a reviewer has to trust. |
| DDL in a script or in tests only | Reviewers' rebuild would not have the changes. |

## Decision

goose. `migrate()` runs after the database ping and before the server listens; the API refuses
to start if a migration fails. Tests call the same `migrate()` in `TestMain`, so they run
against exactly the schema the API serves.

## Consequences

- Migrations are **forward-only** (no `Down` sections). A rollback is a new forward migration;
  that is also what we would do in production, where a `Down` that drops a populated table is
  rarely safe to run.
- Index builds use `-- +goose NO TRANSACTION` with `CREATE INDEX CONCURRENTLY`, so a deploy on
  a large `assignments` table never blocks writes.
- `go mod tidy` removed the unused `sqlx` and moved pgx from v5.9.2 to v5.10.0 (goose's minimum).
- Migrating at startup suits one service owning its schema; with several services or long
  migrations we would move it to a deploy step that runs before the new version rolls out.
