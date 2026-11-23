# ADR 0003 — Overlap index on assignments, chosen by benchmark

Status: accepted · 2026-11-23

## Context

Every grid load asks "for these ~100 people, which assignments overlap [from, to]?". The schema
has `(person_id)` and `(start_date, end_date)`. The second only narrows `start_date <= to`; for
a recent range it matches almost all history (on the seed Postgres seq-scanned and discarded 93%
of rows). Production is a few thousand people and two years of history.

## Options (measured — [db/bench/RESULTS.md](../../db/bench/RESULTS.md), 3,000 people, 1.5 M rows, generic plans)

| Option | Page of 100, recent / old / 26 weeks | Size | Cost |
|---|---|---|---|
| Schema indexes only | 6.6 / 5.1 / 22.2 ms; 1,751 / 456 / 1,784 buffers | — | Work grows with roster × history, not with the page. |
| **B-tree `(person_id, end_date) INCLUDE (start_date, hours_per_day)`** | **4.2 / 4.9 / 17.2 ms; 328 / 357 / 356 buffers** | 59 MB | Index-only scan; exact on `end_date >= from`, old ranges read the person's later rows (bounded). No extension. |
| GiST `(person_id, daterange(start, end))` + `btree_gist` | 4.4 / 3.9 / 18.2 ms; 384 / 425 / 529 buffers | 99 MB | Exact on both bounds, but larger, slower to write, needs an extension and a rewritten predicate. |
| Pre-aggregated person-week table / materialised view | (not built) | — | Fastest reads, but must be invalidated on every assignment write; a project of its own. |

## Decision

The B-tree, created by migration 00003 with `CREATE INDEX CONCURRENTLY` (no write lock while it
builds). It touches 4–5× fewer buffers than the baseline on every page shape, matches GiST on time
at 60% of the size, and needs no extension. "Now and the coming weeks" — the manager's main use —
is its best case.

## Consequences

- The whole-roster aggregate behind `over_count` and `over_only` is aggregation work, index or
  not, and grows with the range: at 3,000 people about 110 ms for 8 weeks, 300 ms for 26 weeks
  and 600 ms for 53 weeks (the API maximum). The first version of the function joined every
  assignment to every week of the range and cost 2.3 s at 53 weeks; migration 00004 expands each
  assignment only into the weeks it overlaps (same results, [db/bench/RESULTS.md](../../db/bench/RESULTS.md)). Acceptable on a
  page load; the next step, if it is not, is a per-person-week summary maintained on assignment
  writes (deferred), or caching the count per range for a short time.
- `assignments_person_id_idx` is now redundant (the new index leads with `person_id`). Dropping it
  would save write cost; left in place because it belongs to the given schema.
- If a concurrent build fails it leaves an `INVALID` index that `IF NOT EXISTS` would then skip;
  the recovery is to drop it and restart the API. Noted for the runbook.
- Benchmarked on a laptop with a warm cache; absolute numbers are indicative, buffer counts are
  the comparable figure.
