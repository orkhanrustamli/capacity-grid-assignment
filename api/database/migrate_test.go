package database_test

import (
	"context"
	"testing"

	"capacity/api/database"
	"capacity/api/internal/testdb"
)

var ctx = context.Background()

func TestMain(m *testing.M) { testdb.Main(m) }

func TestMigrateBackfillsOneBaselinePerPerson(t *testing.T) {
	var people, missing int
	testdb.MustScan(t, testdb.Pool.QueryRow(ctx, `SELECT count(*) FROM people`), &people)
	testdb.MustScan(t, testdb.Pool.QueryRow(ctx, `
		SELECT count(*) FROM people p
		WHERE NOT EXISTS (SELECT 1 FROM person_capacity_changes c
		                  WHERE c.person_id = p.id AND c.effective_from = '-infinity')`), &missing)
	if people == 0 || missing != 0 {
		t.Fatalf("people=%d without baseline=%d", people, missing)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	if err := database.Migrate(ctx, testdb.Pool); err != nil { // second run: nothing to apply, no error
		t.Fatal(err)
	}
}

func TestCapacityChangesRejectsNonMondayAndOutOfRange(t *testing.T) {
	tx := testdb.Begin(t)
	id := testdb.InsertPerson(t, tx, "Check Constraint", "40")
	for _, tc := range []struct{ from, hours string }{
		{"2026-01-06", "40"},  // a Tuesday
		{"2026-01-05", "-1"},  // negative
		{"2026-01-05", "169"}, // more hours than a week has
		{"2026-01-05", "NaN"}, // numeric accepts NaN; the check must not
	} {
		testdb.MustExec(t, tx, `SAVEPOINT s`)
		_, err := tx.Exec(ctx, `INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
			VALUES ($1, $2::date, $3::numeric)`, id, tc.from, tc.hours)
		if err == nil {
			t.Errorf("accepted %+v", tc)
		}
		testdb.MustExec(t, tx, `ROLLBACK TO SAVEPOINT s`)
	}
}
