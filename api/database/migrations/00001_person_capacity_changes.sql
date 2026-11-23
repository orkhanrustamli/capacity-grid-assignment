-- +goose Up
-- Capacity history (ADR 0002). Each row says "from this Monday onward, until
-- the next row, this person has weekly_hours". '-infinity' is the baseline.
CREATE TABLE person_capacity_changes (
  person_id      int         NOT NULL REFERENCES people(id),
  effective_from date        NOT NULL,
  weekly_hours   numeric     NOT NULL CHECK (weekly_hours >= 0 AND weekly_hours <= 168),
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (person_id, effective_from),
  CHECK (effective_from = '-infinity' OR extract(isodow FROM effective_from) = 1)
);

-- Today's value becomes everyone's baseline, so existing history reads the
-- same before and after the migration.
INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
SELECT id, '-infinity', weekly_hours FROM people
ON CONFLICT DO NOTHING;
