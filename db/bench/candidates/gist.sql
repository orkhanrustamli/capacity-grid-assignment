-- Needs the function's range filter rewritten as a daterange overlap (run.sh does it).
DROP INDEX IF EXISTS assignments_person_end_idx, assignments_person_period_gist;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE INDEX assignments_person_period_gist ON assignments USING gist (person_id, daterange(start_date, end_date, '[]'));
