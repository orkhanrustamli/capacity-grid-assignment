# ADR 0002 — Capacity history as effective-dated breakpoints

Status: accepted · 2026-11-23

## Context

`people.weekly_hours` is one number applied to every week of up to two years of history.
Editing it in place changes every past week: a person who was over-allocated last March
silently stops being over-allocated, and no report can be reproduced.

## Options

| Option | Write | Read | Cost / risk |
|---|---|---|---|
| A. Overwrite `people.weekly_hours` | one `UPDATE` | trivial | Rewrites all history on every edit. |
| **B. `person_capacity_changes(person_id, effective_from, weekly_hours)`, PK `(person_id, effective_from)`** | upsert one row | `lead()` turns rows into periods, joined to weeks | Overlaps impossible by construction (one value per breakpoint). |
| C. Per-week override rows | one row per week | trivial join | A different feature (time off); can be layered on B later. |
| D. Validity ranges + `EXCLUDE USING gist` | split/merge ranges on every write | direct | Same expressiveness as B, much harder writes. |

## Decision

B. An edit means "from Monday *W* onward, until the next existing breakpoint". `effective_from`
is a Monday (CHECK) or `-infinity` for the baseline, which the migration backfills from today's
`people.weekly_hours`, so every existing week reads the same before and after. `weekly_hours`
is CHECKed to 0–168 (this also rejects `NaN`, which `numeric` otherwise accepts).

- Later breakpoints are kept, never deleted; the grid marks the week a capacity changes.
- Past weeks are editable on purpose (corrections), but only from the week the manager picks.
- People without a baseline (inserted by something other than our PATCH) fall back to
  `people.weekly_hours`; the PATCH inserts their baseline before writing, so an edit never
  rewrites their earlier weeks.

## Consequences

- `people.weekly_hours` stays as the "current value" for other readers: each PATCH sets it to
  the value in effect for the current week. A **future-dated** change is not mirrored until
  its week arrives — a known gap; a daily job or a view would close it.
- **Assumption:** nothing else writes `people.weekly_hours` (true in this repo). An HR sync that
  did would need a trigger turning such writes into breakpoints.
- No audit trail of *who* changed what: that needs authentication, deferred. `created_at` is
  kept so one can be added later.
