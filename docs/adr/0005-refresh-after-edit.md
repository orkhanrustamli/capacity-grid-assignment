# ADR 0005 — After an edit, re-read that person's row from the server

Status: accepted · 2026-11-23

## Context

Changing a person's weekly hours changes, for every week from the edit onward: their capacity,
utilization and status, and possibly whether they count as over-allocated (`over_count`, the
over-only list). The brief says every number that depends on the edit must be right after a save,
without a page reload. The grid may hold several pages of a range, and other ranges sit in the
query cache.

## Options

| Option | Cost |
|---|---|
| Refetch the whole range | Simplest and always right. Costs every loaded page (N × 100 people, each a roster-wide count) per edit, and the grid reshuffles under the manager's cursor. |
| Patch the cache from the PATCH response | One request. But the client would recompute capacity per week, utilization and status — the status rule would then live in Go-SQL *and* TypeScript, and drift. |
| Optimistic update | Feels instant. Shows numbers the server may reject (422, 409), and still needs the status rule on the client. Rolling back a wrong "under" to "over" is worse than waiting 50 ms. |
| **Re-read one row (`person_id=`) and write it into the cache** | One small extra request with the same endpoint and shape. The server stays the only owner of the arithmetic. |

## Decision

On a successful PATCH, the client requests `GET /api/capacity?from&to&person_id=<id>` for the
range the save **started in** (captured when Save is clicked, not read when it returns), replaces
that person in whichever loaded page holds them, and replaces page 0's `over_count` with the
roster-wide count the row request carries (ADR 0006). Every other cached range is marked stale
without refetching (`invalidateQueries({ refetchType: 'none' })`) and refetches when next shown.

The four failure states, each visible, none silent:

| What failed | What the manager sees |
|---|---|
| Validation (400/422) | The server's message under the input; the typed value stays for correction. |
| Conflict (409) | "Changed by someone else — now 36 h." The row is re-read so the cell shows the winning value. |
| Network / 5xx | "Couldn't save. Your change wasn't applied." with Retry (re-sends the same body); the cell keeps the last saved numbers. |
| Saved, but the re-read failed | The row is dimmed and says "Saved — couldn't refresh this row." with Retry. Old numbers are never shown as current. |

No numbers change while saving; the Save button shows "Saving…".

## Consequences

- One extra request per edit: one row's aggregate plus the roster-wide count. ADR 0003 measures
  the count at 3,000 people as ~110 ms for 8 weeks and ~600 ms for the 53-week maximum, so on a
  long range "Saving…" lasts up to about a second. Splitting the count into its own background
  request would shorten that; deferred.
- **Known gap:** in over-only mode, a person whose edit fixes their over-allocation stays listed
  (now showing "under") until the range is next loaded. Removing a row from under the manager's
  cursor right after they edited it is more jarring than one stale membership; the count above
  the grid is already correct.
- Load more after an edit continues from the cursor as before; the edited person does not move
  because order is by name.
- If two managers edit the same person, the second save gets the 409 path only when the client
  sent `expected_weekly_hours` — the grid always does.
