-- +goose Up
-- +goose StatementBegin

-- Same rules and result as 00002; only the allocation step changes. 00002
-- joined every assignment in the range to every week in the range and then
-- filtered, so the work grew with weeks × assignments: about 3 s for the
-- roster-wide count over 53 weeks at 3,000 people (bench/RESULTS.md). Here
-- each assignment is expanded only into the weeks it overlaps.
CREATE OR REPLACE FUNCTION capacity_person_weeks(p_from date, p_to date, p_person_ids int[])
RETURNS TABLE (person_id int, week_start date, allocated numeric, weekend numeric,
               capacity numeric, capacity_changed boolean, status text)
LANGUAGE sql STABLE AS $$
  WITH weeks AS (
    -- Plain date arithmetic: date_trunc('week', ...) returns timestamptz and
    -- depends on the session time zone.
    SELECT p_from + 7 * i AS week_start
    FROM generate_series(0, (p_to - p_from) / 7) AS i
  ),
  allocation AS (
    -- Every row counts: one logical booking is stored as 15 fragment rows.
    -- Each assignment visits only the weeks of the range it touches (index i
    -- counts weeks from p_from, a Monday); within a week, its days inside
    -- Monday–Friday and inside Saturday–Sunday, each clipped at zero.
    SELECT a.person_id, w.week_start,
           sum(a.hours_per_day * greatest(0, least(a.end_date, w.week_start + 4)
                                          - greatest(a.start_date, w.week_start) + 1)) AS weekday_hours,
           sum(a.hours_per_day * greatest(0, least(a.end_date, w.week_start + 6)
                                          - greatest(a.start_date, w.week_start + 5) + 1)) AS weekend_hours
    FROM assignments a
    CROSS JOIN LATERAL (
      SELECT p_from + 7 * i AS week_start
      FROM generate_series((greatest(a.start_date, p_from) - p_from) / 7,
                           (least(a.end_date, p_to) - p_from) / 7) AS i
    ) w
    WHERE a.person_id = ANY (p_person_ids)
      AND a.end_date >= p_from AND a.start_date <= p_to
    GROUP BY a.person_id, w.week_start
  ),
  periods AS (
    -- Capacity breakpoints (ADR 0002) turned into [effective_from, effective_until).
    SELECT c.person_id, c.effective_from, c.weekly_hours,
           lead(c.effective_from, 1, 'infinity') OVER (PARTITION BY c.person_id ORDER BY c.effective_from)
             AS effective_until
    FROM person_capacity_changes c
    WHERE c.person_id = ANY (p_person_ids)
  ),
  cells AS (
    SELECT p.id AS person_id, w.week_start,
           coalesce(al.weekday_hours, 0) AS allocated,
           coalesce(al.weekend_hours, 0) AS weekend,
           -- People inserted without a baseline row fall back to people.weekly_hours.
           coalesce(pr.weekly_hours, p.weekly_hours) AS capacity,
           coalesce(pr.effective_from = w.week_start, false) AS capacity_changed
    FROM people p
    CROSS JOIN weeks w
    LEFT JOIN allocation al ON al.person_id = p.id AND al.week_start = w.week_start
    LEFT JOIN periods pr ON pr.person_id = p.id
                        AND w.week_start >= pr.effective_from AND w.week_start < pr.effective_until
    WHERE p.id = ANY (p_person_ids)
  )
  SELECT c.person_id, c.week_start, c.allocated, c.weekend, c.capacity, c.capacity_changed,
         CASE
           -- Strictly greater: exactly 100% is "full", not "over". With zero
           -- capacity, any work is over and no work is under.
           WHEN c.allocated > c.capacity THEN 'over'
           WHEN c.allocated = c.capacity AND c.capacity > 0 THEN 'full'
           ELSE 'under'
         END
  FROM cells c
$$;

-- +goose StatementEnd
