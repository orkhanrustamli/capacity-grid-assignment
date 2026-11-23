# ADR 0001 — Weeks, working days, and what "over-allocated" means

Status: accepted · 2026-11-23

## Context

The brief asks for allocated hours against capacity per person per week, but does not say
which days count, where a week starts, or where "over" begins. The seed makes each choice
visible: about 20% of booked hours fall on Saturday or Sunday, 2,040 person-weeks are exactly
100%, Eli (id 5) has zero capacity and 20 h booked, and 2025-12-29 is ISO week 1 of 2026.

## Decision

- **Week:** ISO week, Monday to Sunday, identified and labelled by its Monday (`2026-01-05`),
  never by week number (week 1 of 2026 starts in December 2025; 2026 has a week 53).
- **Working days:** Monday to Friday. An assignment adds `hours_per_day` for each weekday it
  covers. Weekend hours are **not dropped**: they are summed separately (`weekend_hours`) and
  shown as a marker, but they do not count against capacity, which is a working-week figure.
- **Every row counts:** one logical booking is stored as 15 fragment rows; they are summed,
  never deduplicated.
- **Status:** `over` when allocated > capacity (strictly); `full` when equal and capacity > 0;
  otherwise `under`. Zero capacity with any work is `over`; utilisation is `null` (no division
  by zero).
- **One owner:** all of this lives in one SQL function, `capacity_person_weeks`, created by a
  migration. The page query, the over-allocated count and the `over_only` filter all call it,
  so the rule cannot drift between them. It is a single `LANGUAGE sql STABLE` SELECT, which
  Postgres inlines into the caller (checked with `EXPLAIN`: no function scan, real index use).

## Alternatives

| Option | Cost |
|---|---|
| Count calendar days | Over-allocated person-weeks go from 1,626 to 4,779 on the seed; Ana's Monday-to-Sunday week reads 56/40 for a person working a normal week. |
| Drop weekend hours silently | Simple, but hides real bookings — a manager can't see someone was booked on Saturday. |
| `>=` for over | 2,040 exactly-full weeks would all look like problems. |
| Per-person working patterns (e.g. Tue–Sat) | The right model eventually; the schema has no data for it. Deferred. |
| Compute in Go | Pulls every assignment row over the wire, and the count/filter would need the same rule a second time. |

## Consequences

- Holidays and time off are not modelled (2026-01-01 is inside the demo range and counts as a
  working day). A per-week override layer on top of ADR 0002 would add them.
- Changing a rule is a reviewed migration (`CREATE OR REPLACE FUNCTION`), covered by the
  fixture tests and the golden test on people 1–5.
