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
