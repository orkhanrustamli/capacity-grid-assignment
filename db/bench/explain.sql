-- The API's real statements (api/capacity/query.go), prepared and forced onto
-- the generic plan: pgx prepares statements and Postgres may switch to a
-- generic plan after five runs, so the generic plan is the one that must be good.
SET plan_cache_mode = force_generic_plan;

PREPARE cells(date, date, int[]) AS
  SELECT person_id, week_start::text,
         round(allocated, 2)::float8, round(capacity, 2)::float8, round(weekend, 2)::float8,
         CASE WHEN capacity = 0 THEN NULL ELSE round(allocated / capacity, 4)::float8 END,
         status, capacity_changed
  FROM capacity_person_weeks($1, $2, $3)
  ORDER BY person_id, week_start;

PREPARE over_count(date, date) AS
  SELECT count(DISTINCT w.person_id)::int
  FROM (SELECT array_agg(id) AS ids FROM people) roster,
       LATERAL capacity_person_weeks($1, $2, roster.ids) w
  WHERE w.status = 'over';

PREPARE over_only_page(text, int, bool, date, date, int) AS
  SELECT p.id, p.name FROM people p
  WHERE (p.name, p.id) > ($1, $2)
    AND (NOT $3 OR p.id IN (
          SELECT w.person_id
          FROM (SELECT array_agg(id) AS ids FROM people) roster,
               LATERAL capacity_person_weeks($4, $5, roster.ids) w
          WHERE w.status = 'over'))
  ORDER BY p.name, p.id
  LIMIT $6;

-- A first page: the first 100 people in (name, id) order.
SELECT array_agg(id)::text AS page_ids FROM (SELECT id FROM people ORDER BY name, id LIMIT 100) s \gset

\echo '== (a) cells, 100 people, recent 8 weeks 2026-11-23..2027-01-17'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE cells('2026-11-23', '2027-01-17', :'page_ids');
\echo '== (b) cells, 100 people, old 8 weeks 2024-03-04..2024-04-28'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE cells('2024-03-04', '2024-04-28', :'page_ids');
\echo '== (c) cells, 100 people, 26 weeks 2026-06-01..2026-11-29'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE cells('2026-06-01', '2026-11-29', :'page_ids');
\echo '== (d) over_count, whole roster, recent 8 weeks'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE over_count('2026-11-23', '2027-01-17');
\echo '== (e) over_only first page, whole roster, recent 8 weeks'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE over_only_page('', 0, true, '2026-11-23', '2027-01-17', 101);
\echo '== (f) over_count, whole roster, 26 weeks 2026-06-01..2026-11-29'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE over_count('2026-06-01', '2026-11-29');
\echo '== (g) over_count, whole roster, 53 weeks (the API maximum) 2025-12-01..2026-12-06'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE over_count('2025-12-01', '2026-12-06');
\echo '== (h) over_only first page, whole roster, 53 weeks'
EXPLAIN (ANALYZE, BUFFERS) EXECUTE over_only_page('', 0, true, '2025-12-01', '2026-12-06', 101);
