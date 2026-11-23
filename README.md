# Capacity view

Managers need to see who is over-committed before the week starts, not after. This is a
small slice of that: allocation against capacity, per person, per week.

This capacity view will be one component of a team overview page, alongside a timeline
of the team's logged time. For this assignment, build only the capacity view. In
production this page serves rosters of a few thousand people and up to two years of
history. The seed is smaller. Design for the real one, and write down what you deferred.

Budget about ninety minutes. The scaffold runs already — `make up` gives you a working
database, a working API, and a working dev server. Three things are stubbed out and yours
to build.
You do not need Go, Node.js, or Postgres installed locally.

Build a grid a manager can use to understand the team's workload and make changes.
Spend most of your time on the backend: make sure the numbers are right and the
code is easy to change as the team's planning needs evolve.

## Requirements

- Docker with Docker Compose v2
- Make
- Ports 3000 and 8080 available

## Running it

From the repository root:

```bash
make up
```

Web on http://localhost:3000, API on http://localhost:8080.
`GET /api/health` works today and tells you the database is seeded.

`make up` stays in the foreground. Leave it running while you work and press `Ctrl-C` to
stop it. Frontend changes reload automatically. After a backend change, stop and run
`make up` again so the API is rebuilt.

Useful commands:

```bash
make down   # stop and remove the containers
make reset  # wipe the database and re-seed it
make logs   # follow API logs
make psql   # open a Postgres shell
```

## What to build

1. **`GET /api/capacity?from=&to=`** in `api/capacity.go` — a manager should be able to
   see allocated hours and capacity for anyone on their team, week by week, for the
   selected dates.
2. **`CapacityGrid`** in `web/src/CapacityGrid.tsx` — people down the side, weeks across
   the top, over-allocation obvious at a glance. Users should be able to navigate
   backward and forward by week and choose a date range from the UI.
3. **Editing capacity** — a manager can change someone's weekly hours from the grid.
   `PATCH /api/people/{id}` is stubbed in `api/people.go`. After saving, the numbers
   on screen must be correct without a full page reload — how they get there (refetch
   the range, patch what you already have, something optimistic) is a decision we'll
   ask about, so write it down. Saves can fail; what the manager sees when one does is
   part of the task.

The JSON shape between them is undefined on purpose. It's your API; design it.

## The data

`db/schema.sql` and `db/seed.sql` hold the team and their assignments. Read them.
They're fixed input — don't edit them.

## Ground rules

- **Use AI tools.** We do, all day. This task assumes it.
- **Use whatever dependencies help.** The API already includes `sqlx`, which we use in
  Focus API, but using it is optional.
- **Don't touch the run environment** — Compose files, Dockerfiles, the Makefile, the
  schema, or the seed. We rebuild submissions with our own copies of those, so changes
  there are lost anyway.
- **Commit as you go** on a feature branch, and don't squash. The history is useful to us.
- **Vitest is installed.** `docker compose exec web npm test` runs it. A test or two around
  the parts you'd be nervous to change is worth more to us than a full suite.

## When you're done

**Start it, open it, and look at the numbers.** Do they say what you expect? If something
looks off, either fix it or write down in `DECISIONS.md` that you saw it and why you left
it. Both are fine answers. Not looking is the one thing that isn't.

Then fill in `DECISIONS.md` — it's short, and it's the first thing we read.

## How we read this

We spend about ten minutes per submission, on:

- `DECISIONS.md`, in your words
- `.notes/worklog.md`, including decisions or verification that did not make the final summary
- The running grid, and whether the numbers hold up
- The grid as a manager experiences it: loading, editing, a failed save
- The API and query design, and how the view behaves after an edit
- The diff, as a piece of code someone else has to maintain
- Your commit history

Then we talk about it for an hour, live, and extend it together. Come ready to explain
what you decided and why — that conversation is the real point of this exercise, and it's
much easier when the notes are honest about what you didn't get to.

---

## Implementation notes

What was built, how to check it, and where the reasoning lives: the decisions are the ADRs
below, and the working history is in [`.notes/worklog.md`](.notes/worklog.md).

The brief above names `api/capacity.go` and `api/people.go`; that code now lives in the
`api/capacity/` and `api/people/` packages, and `web/src/CapacityGrid.tsx` is now
`web/src/components/CapacityGrid/CapacityGrid.tsx`.

### Decisions (ADRs)

| ADR | Decision |
|---|---|
| [0001](docs/adr/0001-working-days-and-weeks.md) | ISO weeks (Mon–Sun, named by the Monday); Mon–Fri working days; weekend hours shown, never counted; what "over" means, including zero capacity |
| [0002](docs/adr/0002-capacity-history.md) | Capacity edits are effective-dated breakpoints, so past weeks keep the capacity they had |
| [0003](docs/adr/0003-overlap-index.md) | One B-tree index on assignments, chosen by a benchmark at production scale ([db/bench/RESULTS.md](db/bench/RESULTS.md)) |
| [0004](docs/adr/0004-migrations-at-startup.md) | Database changes are embedded goose migrations the API applies at startup |
| [0005](docs/adr/0005-refresh-after-edit.md) | After an edit, the client re-reads that person's row; four explicit failure states |
| [0006](docs/adr/0006-pagination.md) | Keyset pagination by (name, id), an over-only filter and a roster-wide over count |

### API

Every error our handlers return has the same body (unknown routes and methods get the router's plain-text 404/405), and database errors are logged, never returned:

```json
{ "error": { "code": "invalid_range", "message": "from must be on or before to" } }
```

**`GET /api/capacity?from=YYYY-MM-DD&to=YYYY-MM-DD`** — allocated hours and capacity per person
per week. `from` snaps back to its Monday, `to` forward to its Sunday; at most 53 weeks.
Optional: `limit` (1–500, default 100), `cursor` (from `next_cursor`), `person_id` (repeatable;
just those rows), `over_only=true` (people over in at least one week).

```json
{
  "from": "2025-12-29", "to": "2026-01-18",
  "weeks": ["2025-12-29", "2026-01-05", "2026-01-12"],
  "people": [{
    "id": 4, "name": "Dee Okafor",
    "weeks": [
      { "week": "2026-01-05", "allocated_hours": 45, "capacity_hours": 40, "weekend_hours": 0,
        "utilization": 1.125, "status": "over", "capacity_changed": false }
    ]
  }],
  "next_cursor": null,
  "over_count": 41
}
```

`status` is `over` (allocated > capacity, or any hours against zero capacity), `full`
(allocated = capacity > 0) or `under`. `utilization` is `null` when capacity is 0. Every person
has one entry per week, zeros included. `over_count` is roster-wide and only on the first page.
Errors: 400 `invalid_date`, `invalid_range`, `range_too_long`, `invalid_limit`, `invalid_cursor`,
`invalid_person_id`, `invalid_over_only`.

**`PATCH /api/people/{id}`** — set weekly hours from a week onward.

```json
{ "weekly_hours": 32, "effective_from": "2026-01-05", "expected_weekly_hours": 40 }
```

`weekly_hours` is required (a JSON number, 0–168, at most two decimals). `effective_from` is
optional (default: this week; any day snaps to its Monday); the value applies until the person's
next later change. `expected_weekly_hours` is optional: if the value in effect that week is no
longer that, the edit is refused with 409. Saving the value already in effect for that week
changes nothing and returns 200, so retrying a save whose response was lost is safe.
`people.weekly_hours` keeps the value in effect this week.

```json
{ "id": 4, "name": "Dee Okafor", "weekly_hours": 32, "effective_from": "2026-01-05",
  "capacity_changes": [ { "effective_from": null, "weekly_hours": 40 },
                        { "effective_from": "2026-01-05", "weekly_hours": 32 } ] }
```

Errors: 400 `invalid_person_id`, `invalid_body` (malformed JSON, unknown field, missing or quoted
number, bad date); 404 `person_not_found`; 409 `capacity_conflict` (with `current_weekly_hours`);
422 `invalid_weekly_hours`.

### Tests

```bash
docker compose run --rm --build api go test ./...       # Go: needs the db service up
docker compose run --rm --no-deps web sh -c "npm install --no-audit --no-fund && npm test"
docker compose run --rm --no-deps web sh -c "npm install --no-audit --no-fund && npm run tsc"
```

The Go tests run against the Compose Postgres, each inside a transaction that is rolled back.
The golden test checks the hand-built seed people 1–5 and **fails** with "run make reset" if
they were edited through the UI. Web tests run under `TZ=America/Los_Angeles`, so date bugs that
only show west of UTC fail here.

### Benchmark

```bash
bash db/bench/run.sh build                # capacity_bench: 3,000 people, ~1.5 M assignment rows
bash db/bench/run.sh explain btree        # EXPLAIN (ANALYZE, BUFFERS) for each API statement
```

Candidates are `baseline`, `btree` and `gist`; cases cover 8-, 26- and 53-week ranges. Results
and the choice are in
[db/bench/RESULTS.md](db/bench/RESULTS.md).

### Deferred

- Authentication, teams, and who may edit whom.
- Public holidays and per-week time off (capacity is weekly hours only).
- An audit log of capacity changes (only `created_at` today) and undo.
- "Worst first" sorting (would be a separate top-N call; see ADR 0006).
- Row virtualisation for loading the whole roster at once.
- A scheduled job to keep `people.weekly_hours` mirroring the current week when a future change
  takes effect (today it is updated on each edit only).
- A trigger for writers of `people.weekly_hours` other than this API, so their changes reach the
  capacity history.
- Refreshing over-only list membership after an edit (ADR 0005).
