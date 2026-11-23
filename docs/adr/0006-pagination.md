# ADR 0006 — Keyset pagination by (name, id), an over-only filter, and a roster-wide count

Status: accepted · 2026-11-23

## Context

Production rosters have a few thousand people. One person × 26 weeks is 26 cells; 3,000 people
is 78,000 cells per response — too much to compute, send and render for a screen that shows
perhaps 30 rows at a time. The manager's real question is "who is over-committed?", which
scrolling through 30 pages does not answer.

## Options

| Option | Cost |
|---|---|
| Return everyone | Simple. At 3,000 people × 26 weeks: one large aggregate, megabytes of JSON, a DOM the browser struggles with. |
| `LIMIT/OFFSET` | Each page rescans everything before it; a person added or removed between requests shifts rows, so someone is skipped or shown twice. |
| **Keyset on `(name, id)`, opaque cursor** | Each page starts exactly after the last row of the previous one, stable under inserts; cost does not grow with page number. |
| "Worst first" sorting | The question managers ask, but the sort key is computed per range, so a cursor can't be a stored column; every page must aggregate the whole roster. |

## Decision

- Keyset on `(name, id)` — `id` breaks ties between equal names. The cursor is base64url JSON of
  the last row (`{"n": name, "i": id}`); clients treat it as opaque. `limit` defaults to 100, max 500.
- `over_only=true` keeps people with at least one `over` week in the range, still keyset-paged.
  It answers "who is over-committed?" without a computed sort key.
- `over_count` (people over in at least one week, whole roster) is returned **only on the first
  page** (no cursor), including for `person_id=` requests — so the one-row refresh after an edit
  (ADR 0005) brings back a correct count without a second request.
- `person_id=` (repeatable) returns just those rows in the same shape.

## Consequences

- `over_only` and `over_count` aggregate the whole roster for the range; ADR 0003 measures it.
- Order follows the database collation (`en_US.utf8`): the seed's Greek, Cyrillic, Hebrew,
  Arabic and CJK names sort after Latin ones. Fine for one locale; a multi-locale product would
  sort by an ICU collation chosen per viewer.
- There is no index on `people (name, id)`: sorting a few thousand names is sub-millisecond. At
  tens of thousands of people it becomes worth adding.
- Worst-first sorting is deferred; if wanted, it would be a separate, non-paginated "top N" call.
- Page, cells and count are separate statements, not one snapshot, so a concurrent write between
  them can make the count disagree with the page by one edit for one response. Acceptable for a
  view that refreshes on every edit; a `REPEATABLE READ` read-only transaction would close it.
