# Benchmark — overlap index for the capacity query

Run 2026-11-23 on the Compose Postgres 17 (laptop, Docker Desktop), `capacity_bench` built by
`bash bench/run.sh build`: **3,000 people, 1,514,340 assignment rows** (the seed ×6 people,
then every booking repeated 84 weeks earlier, ~2.6 years of history). Statements are the API's
own (`api/capacity_query.go`), prepared, with `plan_cache_mode = force_generic_plan` — the plan
pgx-prepared statements end up on. Each candidate ran twice; times are warm (second run) unless
noted. Raw plans are reproducible with `bash bench/run.sh explain <candidate>`.

## Page of 100 people (the request every grid load makes)

| Case | Baseline (schema indexes only) | **B-tree `(person_id, end_date) INCLUDE (start_date, hours_per_day)`** | GiST `(person_id, daterange)` + `btree_gist` |
|---|---|---|---|
| (a) recent 8 weeks | 6.6 ms (40.5 cold), 1,751 buffers | **4.2 ms, 328 buffers** | 4.4 ms, 384 buffers |
| (b) old 8 weeks (2024) | 5.1 ms, 456 buffers | **4.9 ms, 357 buffers** | 3.9 ms, 425 buffers |
| (c) 26 weeks | 22.2 ms, 1,784 buffers | **17.2 ms, 356 buffers** | 18.2 ms, 529 buffers |

Baseline reads the `(start_date, end_date)` index for *every* person with `start_date <= to`
(55 k–234 k entries) and intersects it with the person index; its cost grows with history and
roster size, not with the page. The B-tree serves the page with an index-only scan: cost grows
with the page only.

## Whole roster (over_count, over_only)

| Case | Baseline | B-tree | GiST |
|---|---|---|---|
| (d) over_count, 8 weeks | 118 ms | 84 ms | 106 ms |
| (e) over_only first page, 8 weeks | 105 ms | 79 ms | 95 ms |

Here the index barely matters: the time goes to aggregating ~55 k rows into 24 k person-weeks
(sort spills ~11 MB to disk at the default `work_mem`). The lever for this is pre-aggregation or
caching, not an index (ADR 0003).

## Sizes (1.5 M rows; table 87 MB)

B-tree 59 MB · GiST 99 MB · existing `(person_id)` 13 MB, `(start_date, end_date)` 12 MB.

## Finding along the way

The first version passed the whole roster as `capacity_person_weeks(from, to, (SELECT array_agg(id) FROM people))`.
Postgres never inlines a set-returning SQL function whose arguments contain a sub-select, so the
count ran as an opaque function call (plan showed `Sort Key: capacity_person_weeks.person_id`).
Passing the array through a `LATERAL` join lets the planner inline it; the API uses that form.
