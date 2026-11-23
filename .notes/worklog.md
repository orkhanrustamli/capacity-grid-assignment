# Worklog

Running notes on how this got built — decisions, assumptions, dead ends, and anything
left unfinished. Append as you go; a line or two per entry is right.

---

## 2026-11-23 — decisions from planning (before code)

- **Data:** every logical assignment is stored as 15 rows (14 × x + 1 × 2x). We sum every row; no dedupe. Verified against the seed: 8,413 logical assignments, 126,195 rows.
- **Working days:** Monday–Friday only. Weekend hours (~20% of booked hours in the seed) are summed separately and shown as a marker, not counted against capacity. Weekdays-only gives 1,626 over-allocated person-weeks; calendar days would give 4,779.
- **Over vs full:** "over" is strictly allocated > capacity; 2,040 person-weeks are exactly 100% and show as "full". Zero capacity with any work is "over", utilisation null.
- **Capacity history:** edits are effective-dated (from a chosen Monday onward); past weeks are editable on purpose (corrections), and weeks before the chosen one are never rewritten.
- **Empty weeks:** weeks come from the requested range, not the data — every third week (including the current week, 2026-11-23) has no assignments for anyone and must still render.
- **Index:** the existing `(start_date, end_date)` index only narrows `start_date <= to`; for a recent range Postgres seq-scans and discards 93% of rows (19 ms on 126 k rows, grows with history). Index choice deferred to a benchmark (Task 5).
- **Assumption:** nothing else writes `people.weekly_hours`; it is kept as a mirror of the current week's value.

## Task 1 — migrations and capacity history

- Decision: goose v3 (embedded SQL, advisory lock) applied at API startup; tests call the same `migrate()` (ADR 0004). Forward-only migrations.
- Decision: capacity history table `person_capacity_changes`, baseline `-infinity` row backfilled from `people.weekly_hours` (ADR 0002).
- Side effect: `go mod tidy` dropped the unused `sqlx` and bumped pgx v5.9.2 → v5.10.0 (goose's minimum).
- Verified: migration applied once; two API restarts applied nothing; CHECKs reject a Tuesday, -1, 169 and NaN.
- Environment note: host port 8080 is held by an unrelated local container on this machine, so `docker compose up api` can't publish it here; checked startup with `docker compose run` instead.

## Task 2 — range parsing

- Decision: `from` snaps back to its Monday and `to` forward to its Sunday; the response echoes the snapped range. A request covers at most 53 weeks (caps a response at people × 53 cells).
- Caught: the plan's "exactly 53 weeks" test range was really 52 weeks (off by one week); fixed the test data, not the code.

## Task 3 — the capacity function

- Decision: the rules live in one inlinable SQL function `capacity_person_weeks(from, to, ids[])` (ADR 0001). `EXPLAIN` confirms Postgres inlines it (no function scan; index on `person_id` used).
- Decision: golden test on seed people 1–5 **fails** (with "run make reset") rather than skips when the seed was edited — a skipped golden test reads as a green suite. Consequence: after editing people 1–5 in the UI, run `make reset` before the Go tests.
- Simplification: "zero capacity with work is over" needs no separate branch — allocated > 0 = capacity already satisfies `allocated > capacity`.

## Task 4 — GET /api/capacity

- Decision: keyset pagination on `(name, id)`, `over_only` filter and roster-wide `over_count` on the first page only (ADR 0006). `person_id=` requests also carry `over_count` so the post-edit row refresh can update the count.
- Dead end: the first pagination test assumed fixture names starting "zz-" sort last; the seed has Greek, Cyrillic, Hebrew, Arabic and CJK names, which sort after Latin in `en_US.utf8`. Test now checks the fixtures come first, in order, with no duplicates.
- Verified with curl on the demo range: people 1–5 match the golden table; 41 people are over-allocated in that range; reversed range → 400 `invalid_range`.

## Task 5 — benchmark and index

- Built `capacity_bench` (3,000 people, 1.5 M rows) from a dump of the seeded DB; the real DB is only read.
- Decision: B-tree `(person_id, end_date) INCLUDE (start_date, hours_per_day)` (ADR 0003, bench/RESULTS.md): 4–5× fewer buffers per page than the schema's indexes, same speed as GiST at 60% of its size, no extension.
- Bug caught by the benchmark: passing the roster as `(SELECT array_agg(id) FROM people)` in the function's arguments stops Postgres inlining it; switched the count and over_only to a `LATERAL` join.
- Known cost: whole-roster count/filter is 80–110 ms at production size regardless of index (aggregation). Pre-aggregation deferred.
- Overran the 15-minute box by ~10 minutes because of the inlining finding; worth it.

## Task 6 — PATCH /api/people/{id}
- Decision: an edit applies from a chosen Monday (default this week) until the person's next later change; earlier weeks keep their history. `people.weekly_hours` mirrors the value in effect this week, so a past edit can return a `weekly_hours` other than the one sent.
- Decision: optional `expected_weekly_hours` → 409 `capacity_conflict` with `current_weekly_hours` when someone else changed that week first; the person row is locked `FOR UPDATE` so two edits of one person serialise.
- Validation: `weekly_hours` must be a JSON number (a quoted "32" is 400), 0–168, at most 2 decimals, no exponent (422); unknown fields and trailing data are 400.
- Verified live: PATCH Ana 32 from 2026-01-12 → demo range shows 40, 40, 32 with the change marker on 01-12. Restored with a full DB reset (a PATCH back would leave an extra breakpoint row).
