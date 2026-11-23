-- +goose NO TRANSACTION
-- +goose Up
-- Overlap index chosen by benchmark (ADR 0003, bench/RESULTS.md). Serves the
-- capacity function's "this person's assignments ending on or after the range
-- start" from the index alone (index-only scan). CONCURRENTLY so building it
-- on a large assignments table never blocks writes; that cannot run inside a
-- transaction, hence NO TRANSACTION.
CREATE INDEX CONCURRENTLY IF NOT EXISTS assignments_person_end_idx
  ON assignments (person_id, end_date) INCLUDE (start_date, hours_per_day);
