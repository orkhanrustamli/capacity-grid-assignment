-- Only the schema's own indexes: (person_id) and (start_date, end_date).
DROP INDEX IF EXISTS assignments_person_end_idx, assignments_person_period_gist;
