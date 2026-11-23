package main

import "testing"

// requireFreshSeed stops a test that reads the hand-built seed people 1–5 when
// someone has edited them (e.g. through the UI). A skip would read as green
// and hide the golden check, so this fails loudly with the fix instead.
func requireFreshSeed(t *testing.T) {
	t.Helper()
	var hours string
	var edits int
	mustScan(t, testPool.QueryRow(ctx, `
		SELECT string_agg(weekly_hours::text, ',' ORDER BY id) FROM people WHERE id BETWEEN 1 AND 5`), &hours)
	mustScan(t, testPool.QueryRow(ctx, `
		SELECT count(*) FROM person_capacity_changes
		WHERE person_id BETWEEN 1 AND 5 AND effective_from <> '-infinity'`), &edits)
	if hours != "40,40,20,40,0" || edits != 0 {
		t.Fatalf("this test needs a freshly seeded database (people 1–5 were edited: weekly_hours %s, %d capacity changes): run make reset", hours, edits)
	}
}

// The hand-built demo people, checked by hand against db/seed.sql.
func TestSeedPeople1To5DemoRange(t *testing.T) {
	requireFreshSeed(t)

	type want struct{ allocated, weekend, capacity float64; status string }
	expected := map[int][3]want{
		1: {{40, 16, 40, "full"}, {0, 0, 40, "under"}, {30, 0, 40, "under"}},  // Ana: Mon→Sun booking
		2: {{0, 0, 40, "under"}, {32, 16, 40, "under"}, {8, 0, 40, "under"}},  // Bo: Fri→Mon crossing
		3: {{0, 0, 20, "under"}, {4, 0, 20, "under"}, {12, 0, 20, "under"}},   // Cem: part-time
		4: {{0, 0, 40, "under"}, {45, 0, 40, "over"}, {40, 0, 40, "full"}},    // Dee: overlap, then exactly full
		5: {{0, 0, 0, "under"}, {20, 0, 0, "over"}, {0, 0, 0, "under"}},       // Eli: zero capacity
	}
	weeks := []string{"2025-12-29", "2026-01-05", "2026-01-12"}

	got := personWeeks(t, testPool, "2025-12-29", "2026-01-18", 1, 2, 3, 4, 5)
	for id, exp := range expected {
		if len(got[id]) != len(weeks) {
			t.Fatalf("person %d: got %d weeks", id, len(got[id]))
		}
		for i, w := range got[id] {
			if w.Week != weeks[i] {
				t.Errorf("person %d week %d = %s, want %s", id, i, w.Week, weeks[i])
			}
			assertWeek(t, w, exp[i].allocated, exp[i].weekend, exp[i].capacity, exp[i].status)
		}
	}
}
