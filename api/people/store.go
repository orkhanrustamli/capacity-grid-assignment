package people

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"capacity/api/database"
	"capacity/api/httpapi"
	"capacity/api/week"
)

// capacityChange is one breakpoint of a person's capacity history.
type capacityChange struct {
	EffectiveFrom *string `json:"effective_from"` // null = baseline, in effect since forever
	WeeklyHours   float64 `json:"weekly_hours"`
}

// personCapacity is the PATCH response: the person's capacity history after
// the edit, plus the value in effect this week (mirrored into people.weekly_hours).
type personCapacity struct {
	ID              int              `json:"id"`
	Name            string           `json:"name"`
	WeeklyHours     float64          `json:"weekly_hours"`
	EffectiveFrom   string           `json:"effective_from"`
	CapacityChanges []capacityChange `json:"capacity_changes"`
}

// capacityUpdate is a validated edit. Numbers stay decimal strings and are
// cast to numeric in SQL, so no value passes through a float on the way in.
type capacityUpdate struct {
	WeeklyHours   string
	EffectiveFrom time.Time // a Monday
	Expected      *string   // the value the client saw for that week; nil = don't check
}

const (
	lockPersonSQL = `SELECT name FROM people WHERE id = $1 FOR UPDATE`

	// A person written by anything other than this endpoint may have no
	// history yet; their current value becomes the baseline before the first
	// change, so weeks before the change keep the capacity they had.
	ensureBaselineSQL = `
INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
SELECT id, '-infinity', weekly_hours FROM people WHERE id = $1
ON CONFLICT DO NOTHING`

	// The value in effect for the edited week, whether it already equals the
	// new value, and whether it equals what the client expected (true when the
	// client sent no expectation).
	capacityInEffectSQL = `
SELECT round(weekly_hours, 2)::float8,
       weekly_hours = $3::numeric,
       coalesce(weekly_hours = $4::numeric, true)
FROM person_capacity_changes
WHERE person_id = $1 AND effective_from <= $2::date
ORDER BY effective_from DESC
LIMIT 1`

	upsertChangeSQL = `
INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
VALUES ($1, $2::date, $3::numeric)
ON CONFLICT (person_id, effective_from)
DO UPDATE SET weekly_hours = EXCLUDED.weekly_hours, created_at = now()`

	// people.weekly_hours stays the value in effect this week, so readers of
	// the original column keep seeing the current capacity.
	mirrorCurrentSQL = `
UPDATE people SET weekly_hours = (
  SELECT weekly_hours FROM person_capacity_changes
  WHERE person_id = $1 AND effective_from <= $2::date
  ORDER BY effective_from DESC
  LIMIT 1)
WHERE id = $1
RETURNING round(weekly_hours, 2)::float8`

	listChangesSQL = `
SELECT CASE WHEN effective_from = '-infinity' THEN NULL ELSE to_char(effective_from, 'YYYY-MM-DD') END,
       round(weekly_hours, 2)::float8
FROM person_capacity_changes
WHERE person_id = $1
ORDER BY effective_from`
)

// updateCapacity sets a person's weekly hours from u.EffectiveFrom onward,
// until their next later change, in one transaction. The person row is locked
// first, so two edits of the same person apply one after the other.
func updateCapacity(ctx context.Context, db database.DBTX, personID int, u capacityUpdate, today time.Time) (personCapacity, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return personCapacity{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := personCapacity{ID: personID, EffectiveFrom: u.EffectiveFrom.Format(week.Layout)}
	if err := tx.QueryRow(ctx, lockPersonSQL, personID).Scan(&out.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return personCapacity{}, &httpapi.Error{Status: http.StatusNotFound, Code: "person_not_found",
				Message: fmt.Sprintf("no person with id %d", personID)}
		}
		return personCapacity{}, err
	}
	if _, err := tx.Exec(ctx, ensureBaselineSQL, personID); err != nil {
		return personCapacity{}, err
	}
	if err := applyChange(ctx, tx, personID, u); err != nil {
		return personCapacity{}, err
	}
	if err := tx.QueryRow(ctx, mirrorCurrentSQL, personID, week.Start(today).Format(week.Layout)).Scan(&out.WeeklyHours); err != nil {
		return personCapacity{}, err
	}
	if out.CapacityChanges, err = listChanges(ctx, tx, personID); err != nil {
		return personCapacity{}, err
	}
	return out, tx.Commit(ctx)
}

// applyChange writes the breakpoint unless the edited week already has the
// new value (nothing would change, and a retried save must not conflict with
// itself), and refuses it with 409 when the week's value is no longer the one
// the client last saw.
func applyChange(ctx context.Context, tx pgx.Tx, personID int, u capacityUpdate) error {
	effectiveFrom := u.EffectiveFrom.Format(week.Layout)
	var current float64
	var unchanged, asExpected bool
	err := tx.QueryRow(ctx, capacityInEffectSQL, personID, effectiveFrom, u.WeeklyHours, u.Expected).
		Scan(&current, &unchanged, &asExpected)
	if err != nil {
		return err
	}
	if unchanged {
		return nil
	}
	if !asExpected {
		return &httpapi.Error{Status: http.StatusConflict, Code: "capacity_conflict",
			Message: "capacity changed since it was loaded",
			Details: map[string]any{"current_weekly_hours": current}}
	}
	_, err = tx.Exec(ctx, upsertChangeSQL, personID, effectiveFrom, u.WeeklyHours)
	return err
}

func listChanges(ctx context.Context, tx pgx.Tx, personID int) ([]capacityChange, error) {
	rows, err := tx.Query(ctx, listChangesSQL, personID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (capacityChange, error) {
		var c capacityChange
		err := row.Scan(&c.EffectiveFrom, &c.WeeklyHours)
		return c, err
	})
}
