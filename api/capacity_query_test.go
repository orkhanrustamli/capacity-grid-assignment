package main

import (
	"testing"
)

type personWeek struct {
	Week                         string
	Allocated, Weekend, Capacity float64
	Changed                      bool
	Status                       string
}

// personWeeks reads capacity_person_weeks for the given people, grouped by person.
func personWeeks(t *testing.T, db DBTX, from, to string, ids ...int) map[int][]personWeek {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT person_id, week_start::text, round(allocated, 2)::float8, round(weekend, 2)::float8,
		       round(capacity, 2)::float8, capacity_changed, status
		FROM capacity_person_weeks($1::date, $2::date, $3::int[])
		ORDER BY person_id, week_start`, from, to, ids)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int][]personWeek{}
	for rows.Next() {
		var id int
		var w personWeek
		if err := rows.Scan(&id, &w.Week, &w.Allocated, &w.Weekend, &w.Capacity, &w.Changed, &w.Status); err != nil {
			t.Fatal(err)
		}
		out[id] = append(out[id], w)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// onlyWeek returns the single week of a one-week query.
func onlyWeek(t *testing.T, weeks []personWeek) personWeek {
	t.Helper()
	if len(weeks) != 1 {
		t.Fatalf("got %d weeks, want 1: %+v", len(weeks), weeks)
	}
	return weeks[0]
}

func assertWeek(t *testing.T, got personWeek, allocated, weekend, capacity float64, status string) {
	t.Helper()
	if got.Allocated != allocated || got.Weekend != weekend || got.Capacity != capacity || got.Status != status {
		t.Errorf("week %s = %v/%v weekend/%v %s, want %v/%v weekend/%v %s", got.Week,
			got.Allocated, got.Weekend, got.Capacity, got.Status, allocated, weekend, capacity, status)
	}
}

func TestFragmentsAreSummed(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Fragments", "40")
	insertFragmentedAssignment(t, tx, id, "2026-01-05", "2026-01-09", "8") // 15 rows summing to 8 h/day

	assertWeek(t, onlyWeek(t, personWeeks(t, tx, "2026-01-05", "2026-01-11", id)[id]), 40, 0, 40, "full")
}

func TestWeekendHoursAreSeparate(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Weekend", "40")
	insertAssignment(t, tx, id, "2026-01-05", "2026-01-11", "8") // Monday to Sunday

	assertWeek(t, onlyWeek(t, personWeeks(t, tx, "2026-01-05", "2026-01-11", id)[id]), 40, 16, 40, "full")
}

func TestAssignmentCrossingWeeksIsSplit(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Crossing", "40")
	insertAssignment(t, tx, id, "2026-01-09", "2026-01-12", "8") // Friday to Monday

	weeks := personWeeks(t, tx, "2026-01-05", "2026-01-18", id)[id]
	if len(weeks) != 2 {
		t.Fatalf("got %d weeks", len(weeks))
	}
	assertWeek(t, weeks[0], 8, 16, 40, "under")
	assertWeek(t, weeks[1], 8, 0, 40, "under")
}

func TestOverlappingAssignmentsAdd(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Overlap", "40")
	insertAssignment(t, tx, id, "2026-01-05", "2026-01-09", "6")
	insertAssignment(t, tx, id, "2026-01-07", "2026-01-09", "5")

	assertWeek(t, onlyWeek(t, personWeeks(t, tx, "2026-01-05", "2026-01-11", id)[id]), 45, 0, 40, "over")
}

func TestExactlyFullIsNotOver(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Exactly full", "40")
	insertAssignment(t, tx, id, "2026-01-05", "2026-01-09", "8")

	assertWeek(t, onlyWeek(t, personWeeks(t, tx, "2026-01-05", "2026-01-11", id)[id]), 40, 0, 40, "full")
}

func TestZeroCapacityWithWorkIsOver(t *testing.T) {
	tx := beginTx(t)
	busy := insertPerson(t, tx, "Zero busy", "0")
	idle := insertPerson(t, tx, "Zero idle", "0")
	insertAssignment(t, tx, busy, "2026-01-05", "2026-01-09", "4")

	got := personWeeks(t, tx, "2026-01-05", "2026-01-11", busy, idle)
	assertWeek(t, onlyWeek(t, got[busy]), 20, 0, 0, "over")
	assertWeek(t, onlyWeek(t, got[idle]), 0, 0, 0, "under")
}

func TestEmptyWeeksAndIdlePeopleAppear(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Idle", "40")

	weeks := personWeeks(t, tx, "2026-01-05", "2026-01-25", id)[id]
	want := []string{"2026-01-05", "2026-01-12", "2026-01-19"}
	if len(weeks) != len(want) {
		t.Fatalf("got %d weeks: %+v", len(weeks), weeks)
	}
	for i, w := range weeks {
		if w.Week != want[i] {
			t.Errorf("week %d = %s, want %s", i, w.Week, want[i])
		}
		assertWeek(t, w, 0, 0, 40, "under")
	}
}

func TestAssignmentPartlyOutsideRange(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Partly outside", "40")
	insertAssignment(t, tx, id, "2025-12-29", "2026-01-09", "8")

	assertWeek(t, onlyWeek(t, personWeeks(t, tx, "2026-01-05", "2026-01-11", id)[id]), 40, 0, 40, "full")
}

func TestCapacityBreakpointMidRange(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "Breakpoint", "40")
	mustExec(t, tx, `INSERT INTO person_capacity_changes (person_id, effective_from, weekly_hours)
		VALUES ($1, '2026-01-12', 32)`, id)

	weeks := personWeeks(t, tx, "2026-01-05", "2026-01-25", id)[id]
	wantCapacity := []float64{40, 32, 32}
	wantChanged := []bool{false, true, false}
	if len(weeks) != len(wantCapacity) {
		t.Fatalf("got %d weeks", len(weeks))
	}
	for i, w := range weeks {
		if w.Capacity != wantCapacity[i] || w.Changed != wantChanged[i] {
			t.Errorf("week %s: capacity %v changed %v, want %v %v", w.Week, w.Capacity, w.Changed, wantCapacity[i], wantChanged[i])
		}
	}
}

func TestPersonWithoutBaselineFallsBackToPeopleColumn(t *testing.T) {
	tx := beginTx(t)
	id := insertPersonWithoutBaseline(t, tx, "No baseline", "24")

	weeks := personWeeks(t, tx, "2026-01-05", "2026-01-18", id)[id]
	if len(weeks) != 2 {
		t.Fatalf("got %d weeks", len(weeks))
	}
	for _, w := range weeks {
		assertWeek(t, w, 0, 0, 24, "under")
	}
}
