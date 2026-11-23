-- Grows a copy of the seed (in the capacity_bench database, never the real one)
-- to production size: 3,000 people and about 2.6 years of history, ~1.5 M
-- assignment rows in the seed's own 15-fragment shape.

-- 5 more copies of every person and their bookings: 500 → 3,000 people.
INSERT INTO people (id, name, weekly_hours)
SELECT p.id + 1000 * k, p.name || ' #' || k, p.weekly_hours
FROM people p CROSS JOIN generate_series(1, 5) AS k;

INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
SELECT a.person_id + 1000 * k, a.project_id, a.start_date, a.end_date, a.hours_per_day
FROM assignments a CROSS JOIN generate_series(1, 5) AS k;

-- History depth: every booking again, 84 weeks earlier (same weekdays).
INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
SELECT person_id, project_id, start_date - 588, end_date - 588, hours_per_day FROM assignments;

INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
SELECT id, '-infinity', weekly_hours FROM people
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('people', 'id'), max(id)) FROM people;
SELECT setval(pg_get_serial_sequence('assignments', 'id'), max(id)) FROM assignments;
