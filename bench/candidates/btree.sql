DROP INDEX IF EXISTS assignments_person_end_idx, assignments_person_period_gist;
CREATE INDEX assignments_person_end_idx ON assignments (person_id, end_date) INCLUDE (start_date, hours_per_day);
