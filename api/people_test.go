package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// patchRange spans the weeks the PATCH tests look at: 2025-12-29 .. 2026-02-02.
var patchRange = dateRange{
	From: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
	To:   time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC),
}

func servePatch(db DBTX, id, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/people/"+id, strings.NewReader(body))
	req.SetPathValue("id", id)
	(&server{db: db, now: fixedNow}).handleUpdatePerson(rec, req)
	return rec
}

// patch sends a PATCH that must succeed and decodes the response.
func patch(t *testing.T, db DBTX, id int, body string) personCapacity {
	t.Helper()
	rec := servePatch(db, strconv.Itoa(id), body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH %d %s = %d %s", id, body, rec.Code, rec.Body.String())
	}
	var p personCapacity
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// weekCapacities returns week → capacity cell for one person over patchRange.
func weekCapacities(t *testing.T, db DBTX, personID int) map[string]weekCell {
	t.Helper()
	page, err := queryCapacity(ctx, db, capacityQuery{Range: patchRange, Limit: 1, PersonIDs: []int{personID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.People) != 1 {
		t.Fatalf("person %d: got %d rows", personID, len(page.People))
	}
	out := map[string]weekCell{}
	for _, c := range page.People[0].Weeks {
		out[c.Week] = c
	}
	return out
}

func peopleWeeklyHours(t *testing.T, db DBTX, id int) float64 {
	t.Helper()
	var h float64
	mustScan(t, db.QueryRow(ctx, `SELECT weekly_hours::float8 FROM people WHERE id = $1`, id), &h)
	return h
}

func assertCapacities(t *testing.T, cells map[string]weekCell, want map[string]float64) {
	t.Helper()
	for week, hours := range want {
		if got := cells[week].CapacityHours; got != hours {
			t.Errorf("week %s capacity = %v, want %v", week, got, hours)
		}
	}
}

func TestPatchAppliesFromSnappedWeekAndKeepsHistory(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	got := patch(t, tx, id, `{"weekly_hours": 32, "effective_from": "2026-01-07"}`)

	if got.EffectiveFrom != "2026-01-05" || got.WeeklyHours != 32 || got.ID != id {
		t.Errorf("response = %+v", got)
	}
	cells := weekCapacities(t, tx, id)
	assertCapacities(t, cells, map[string]float64{"2025-12-29": 40, "2026-01-05": 32, "2026-01-12": 32})
	for week, c := range cells {
		if c.CapacityChanged != (week == "2026-01-05") {
			t.Errorf("week %s capacity_changed = %v", week, c.CapacityChanged)
		}
	}
	if h := peopleWeeklyHours(t, tx, id); h != 32 {
		t.Errorf("people.weekly_hours = %v, want 32", h)
	}
}

func TestPatchDefaultsToCurrentWeek(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	if got := patch(t, tx, id, `{"weekly_hours": 30}`); got.EffectiveFrom != "2026-01-05" {
		t.Errorf("effective_from = %s, want 2026-01-05", got.EffectiveFrom)
	}
}

func TestPatchPreservesLaterBreakpoint(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	patch(t, tx, id, `{"weekly_hours": 20, "effective_from": "2026-02-02"}`)
	patch(t, tx, id, `{"weekly_hours": 32, "effective_from": "2026-01-05"}`)

	assertCapacities(t, weekCapacities(t, tx, id), map[string]float64{
		"2025-12-29": 40, "2026-01-05": 32, "2026-01-12": 32, "2026-01-19": 32, "2026-01-26": 32, "2026-02-02": 20,
	})
}

func TestPatchPastEditMirrorsCurrentWeekValue(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	patch(t, tx, id, `{"weekly_hours": 32, "effective_from": "2026-01-05"}`)
	got := patch(t, tx, id, `{"weekly_hours": 36, "effective_from": "2025-12-01"}`)

	if got.WeeklyHours != 32 {
		t.Errorf("response weekly_hours = %v, want 32 (value in effect this week)", got.WeeklyHours)
	}
	if h := peopleWeeklyHours(t, tx, id); h != 32 {
		t.Errorf("people.weekly_hours = %v, want 32", h)
	}
	assertCapacities(t, weekCapacities(t, tx, id), map[string]float64{"2025-12-29": 36, "2026-01-05": 32})
}

func TestPatchFutureEditDoesNotMirror(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	got := patch(t, tx, id, `{"weekly_hours": 20, "effective_from": "2026-02-02"}`)

	if got.WeeklyHours != 40 {
		t.Errorf("response weekly_hours = %v, want 40", got.WeeklyHours)
	}
	if h := peopleWeeklyHours(t, tx, id); h != 40 {
		t.Errorf("people.weekly_hours = %v, want 40", h)
	}
}

func TestPatchConflictOnStaleExpected(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	rec := servePatch(tx, strconv.Itoa(id), `{"weekly_hours": 32, "expected_weekly_hours": 36}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code               string  `json:"code"`
			CurrentWeeklyHours float64 `json:"current_weekly_hours"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "capacity_conflict" || body.Error.CurrentWeeklyHours != 40 {
		t.Errorf("error = %+v", body.Error)
	}
	var changes int
	mustScan(t, tx.QueryRow(ctx, `SELECT count(*) FROM person_capacity_changes WHERE person_id = $1 AND effective_from <> '-infinity'`, id), &changes)
	if changes != 0 {
		t.Errorf("%d change rows written on conflict", changes)
	}
	if h := peopleWeeklyHours(t, tx, id); h != 40 {
		t.Errorf("people.weekly_hours = %v after conflict", h)
	}
}

func TestPatchMatchingExpectedSucceeds(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	if got := patch(t, tx, id, `{"weekly_hours": 32, "expected_weekly_hours": 40.00}`); got.WeeklyHours != 32 {
		t.Errorf("weekly_hours = %v", got.WeeklyHours)
	}
}

func TestPatchPersonWithoutBaselineKeepsEarlierWeeks(t *testing.T) {
	tx := beginTx(t)
	id := insertPersonWithoutBaseline(t, tx, "zz-test patch", "24")

	patch(t, tx, id, `{"weekly_hours": 32, "effective_from": "2026-01-05"}`)

	assertCapacities(t, weekCapacities(t, tx, id), map[string]float64{"2025-12-29": 24, "2026-01-05": 32})
}

func TestPatchReplacesSameWeek(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	patch(t, tx, id, `{"weekly_hours": 32, "effective_from": "2026-01-05"}`)
	got := patch(t, tx, id, `{"weekly_hours": 30, "effective_from": "2026-01-05"}`)

	nonBaseline := 0
	for _, c := range got.CapacityChanges {
		if c.EffectiveFrom != nil {
			nonBaseline++
			if *c.EffectiveFrom != "2026-01-05" || c.WeeklyHours != 30 {
				t.Errorf("change = %s %v", *c.EffectiveFrom, c.WeeklyHours)
			}
		}
	}
	if nonBaseline != 1 || len(got.CapacityChanges) != 2 {
		t.Errorf("capacity_changes = %+v", got.CapacityChanges)
	}
}

func TestPatchValidation(t *testing.T) {
	tx := beginTx(t)
	id := strconv.Itoa(insertPerson(t, tx, "zz-test patch", "40"))

	cases := []struct {
		name, id, body string
		status         int
		code           string
	}{
		{"negative", id, `{"weekly_hours": -1}`, 422, "invalid_weekly_hours"},
		{"over a week", id, `{"weekly_hours": 168.5}`, 422, "invalid_weekly_hours"},
		{"three decimals", id, `{"weekly_hours": 32.125}`, 422, "invalid_weekly_hours"},
		{"exponent", id, `{"weekly_hours": 3.2e1}`, 422, "invalid_weekly_hours"},
		{"whole week", id, `{"weekly_hours": 168}`, 200, ""},
		{"zero", id, `{"weekly_hours": 0}`, 200, ""},
		{"quoted number", id, `{"weekly_hours": "32"}`, 400, "invalid_body"},
		{"null", id, `{"weekly_hours": null}`, 400, "invalid_body"},
		{"missing", id, `{}`, 400, "invalid_body"},
		{"unknown field", id, `{"weekly_hours": 32, "foo": 1}`, 400, "invalid_body"},
		{"not json", id, `not json`, 400, "invalid_body"},
		{"trailing data", id, `{"weekly_hours": 32} {}`, 400, "invalid_body"},
		{"bad date", id, `{"weekly_hours": 32, "effective_from": "2026-13-01"}`, 400, "invalid_body"},
		{"bad expected", id, `{"weekly_hours": 32, "expected_weekly_hours": "40"}`, 400, "invalid_body"},
		{"year zero", id, `{"weekly_hours": 32, "effective_from": "0000-01-05"}`, 400, "invalid_body"},
		{"expected overflows", id, `{"weekly_hours": 32, "expected_weekly_hours": 1e999999}`, 400, "invalid_body"},
		{"expected three decimals", id, `{"weekly_hours": 32, "expected_weekly_hours": 40.001}`, 400, "invalid_body"},
		{"id not a number", "abc", `{"weekly_hours": 32}`, 400, "invalid_person_id"},
		{"id beyond int4", "99999999999", `{"weekly_hours": 32}`, 400, "invalid_person_id"},
		{"id zero", "0", `{"weekly_hours": 32}`, 400, "invalid_person_id"},
		{"unknown person", "999999999", `{"weekly_hours": 32}`, 404, "person_not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := servePatch(tx, c.id, c.body)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, c.status, rec.Body.String())
			}
			if c.status == 200 {
				return
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != c.code || body.Error.Message == "" {
				t.Errorf("error = %+v, want code %s", body.Error, c.code)
			}
		})
	}
}

func nonBaselineChanges(t *testing.T, db DBTX, id int) int {
	t.Helper()
	var n int
	mustScan(t, db.QueryRow(ctx, `SELECT count(*) FROM person_capacity_changes WHERE person_id = $1 AND effective_from <> '-infinity'`, id), &n)
	return n
}

// Saving the value already in effect changes no week, so it must not write a
// breakpoint: one would show a false "capacity changes" marker and cut short
// a later edit of an earlier week.
func TestPatchSameValueWritesNothing(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")

	got := patch(t, tx, id, `{"weekly_hours": 40.00, "effective_from": "2026-01-05", "expected_weekly_hours": 40}`)

	if got.WeeklyHours != 40 || len(got.CapacityChanges) != 1 {
		t.Errorf("response = %+v", got)
	}
	if n := nonBaselineChanges(t, tx, id); n != 0 {
		t.Errorf("%d change rows written for an unchanged value", n)
	}
	if weekCapacities(t, tx, id)["2026-01-05"].CapacityChanged {
		t.Error("week 2026-01-05 marked as a capacity change")
	}
}

// A save whose response was lost (timeout, dropped connection) is retried with
// the same body. The retry must succeed, not report the first attempt as
// someone else's conflicting change.
func TestPatchRetryOfAppliedSaveSucceeds(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test patch", "40")
	body := `{"weekly_hours": 32, "effective_from": "2026-01-05", "expected_weekly_hours": 40}`

	patch(t, tx, id, body)
	got := patch(t, tx, id, body)

	if got.WeeklyHours != 32 || nonBaselineChanges(t, tx, id) != 1 {
		t.Errorf("retry: response %+v, %d change rows", got, nonBaselineChanges(t, tx, id))
	}
}
