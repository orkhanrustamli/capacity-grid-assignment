// Package testdb is the shared setup for tests that need Postgres. It is not a
// _test.go file so that every package's tests can import it; internal/ keeps
// it out of the API's importable surface.
//
// Tests run against the Compose Postgres (DATABASE_URL), migrated exactly as
// the API migrates it at startup. Each test that writes works inside its own
// transaction, rolled back on cleanup, so the seed is never modified.
package testdb

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"capacity/api/database"
)

var ctx = context.Background()

// Pool is the migrated connection pool, set by Main.
var Pool *pgxpool.Pool

// Main connects, migrates once, runs the package's tests and exits. Call it
// from each package's TestMain.
func Main(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://capacity:capacity@localhost:5432/capacity?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	Pool = pool
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// FixedNow is the clock handlers get in tests: Wednesday 2026-01-07, noon UTC.
func FixedNow() time.Time { return time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC) }

// Begin opens a transaction that is rolled back when the test ends.
func Begin(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return tx
}

func MustScan(t *testing.T, row pgx.Row, dest ...any) {
	t.Helper()
	if err := row.Scan(dest...); err != nil {
		t.Fatal(err)
	}
}

func MustExec(t *testing.T, db database.DBTX, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

// InsertPerson creates a person together with the '-infinity' baseline row the
// migration backfill gives every existing person.
func InsertPerson(t *testing.T, db database.DBTX, name string, weeklyHours string) int {
	t.Helper()
	id := InsertPersonWithoutBaseline(t, db, name, weeklyHours)
	MustExec(t, db, `INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
		VALUES ($1, '-infinity', $2::numeric)`, id, weeklyHours)
	return id
}

// InsertPersonWithoutBaseline creates a person the way any writer other than
// our PATCH would: no capacity history at all.
func InsertPersonWithoutBaseline(t *testing.T, db database.DBTX, name string, weeklyHours string) int {
	t.Helper()
	var id int
	MustScan(t, db.QueryRow(ctx, `INSERT INTO people (name, weekly_hours) VALUES ($1, $2::numeric) RETURNING id`,
		name, weeklyHours), &id)
	return id
}

func InsertAssignment(t *testing.T, db database.DBTX, personID int, start, end string, hoursPerDay string) {
	t.Helper()
	MustExec(t, db, `INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
		VALUES ($1, 1, $2::date, $3::date, $4::numeric)`, personID, start, end, hoursPerDay)
}

// InsertFragmentedAssignment stores one logical booking the way the seed does:
// 15 rows, 14 carrying daily/16 and one carrying daily/8, summing to daily.
func InsertFragmentedAssignment(t *testing.T, db database.DBTX, personID int, start, end string, dailyHours string) {
	t.Helper()
	MustExec(t, db, `INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
		SELECT $1, 1, $2::date, $3::date, CASE WHEN i = 15 THEN $4::numeric / 8 ELSE $4::numeric / 16 END
		FROM generate_series(1, 15) AS i`, personID, start, end, dailyHours)
}
