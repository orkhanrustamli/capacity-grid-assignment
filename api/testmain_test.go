package main

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tests run against the Compose Postgres (DATABASE_URL), migrated exactly as
// the API migrates it at startup. Each test that writes works inside its own
// transaction, rolled back on cleanup, so the seed is never modified.
var (
	ctx      = context.Background()
	testPool *pgxpool.Pool
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://capacity:capacity@localhost:5432/capacity?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	testPool = pool
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func beginTx(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return tx
}

func mustScan(t *testing.T, row pgx.Row, dest ...any) {
	t.Helper()
	if err := row.Scan(dest...); err != nil {
		t.Fatal(err)
	}
}

func mustExec(t *testing.T, db DBTX, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

// insertPerson creates a person together with the '-infinity' baseline row the
// migration backfill gives every existing person.
func insertPerson(t *testing.T, db DBTX, name string, weeklyHours string) int {
	t.Helper()
	id := insertPersonWithoutBaseline(t, db, name, weeklyHours)
	mustExec(t, db, `INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
		VALUES ($1, '-infinity', $2::numeric)`, id, weeklyHours)
	return id
}

// insertPersonWithoutBaseline creates a person the way any writer other than
// our PATCH would: no capacity history at all.
func insertPersonWithoutBaseline(t *testing.T, db DBTX, name string, weeklyHours string) int {
	t.Helper()
	var id int
	mustScan(t, db.QueryRow(ctx, `INSERT INTO people (name, weekly_hours) VALUES ($1, $2::numeric) RETURNING id`,
		name, weeklyHours), &id)
	return id
}

func insertAssignment(t *testing.T, db DBTX, personID int, start, end string, hoursPerDay string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
		VALUES ($1, 1, $2::date, $3::date, $4::numeric)`, personID, start, end, hoursPerDay)
}

// insertFragmentedAssignment stores one logical booking the way the seed does:
// 15 rows, 14 carrying daily/16 and one carrying daily/8, summing to daily.
func insertFragmentedAssignment(t *testing.T, db DBTX, personID int, start, end string, dailyHours string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO assignments (person_id, project_id, start_date, end_date, hours_per_day)
		SELECT $1, 1, $2::date, $3::date, CASE WHEN i = 15 THEN $4::numeric / 8 ELSE $4::numeric / 16 END
		FROM generate_series(1, 15) AS i`, personID, start, end, dailyHours)
}
