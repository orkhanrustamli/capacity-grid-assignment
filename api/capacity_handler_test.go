package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

const demoRange = "from=2025-12-29&to=2026-01-16"

func fixedNow() time.Time { return time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC) }

// getCapacity calls the handler directly and decodes a 200 response.
func getCapacity(t *testing.T, db DBTX, query string) capacityPage {
	t.Helper()
	rec := serveCapacity(db, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET ?%s = %d %s", query, rec.Code, rec.Body.String())
	}
	var page capacityPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func serveCapacity(db DBTX, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	(&server{db: db, now: fixedNow}).handleCapacity(rec, httptest.NewRequest(http.MethodGet, "/api/capacity?"+query, nil))
	return rec
}

// afterSeed is a cursor positioned just before the "zz-test" fixture people,
// so pagination tests walk only the people they created.
func afterSeed() string { return url.QueryEscape(encodeCursor(pageCursor{Name: "zz-test", ID: 0})) }

func names(p capacityPage) []string {
	out := []string{}
	for _, person := range p.People {
		out = append(out, person.Name)
	}
	return out
}

func TestCapacityResponseShape(t *testing.T) {
	requireFreshSeed(t)
	page := getCapacity(t, testPool, demoRange+"&person_id=1")

	if page.From != "2025-12-29" || page.To != "2026-01-18" {
		t.Errorf("range = %s..%s", page.From, page.To)
	}
	if fmt.Sprint(page.Weeks) != "[2025-12-29 2026-01-05 2026-01-12]" {
		t.Errorf("weeks = %v", page.Weeks)
	}
	if len(page.People) != 1 || page.People[0].Name != "Ana Ferreira" || len(page.People[0].Weeks) != 3 {
		t.Fatalf("people = %+v", page.People)
	}
	got := page.People[0].Weeks[0]
	if got.Week != "2025-12-29" || got.AllocatedHours != 40 || got.CapacityHours != 40 || got.WeekendHours != 16 ||
		got.Utilization == nil || *got.Utilization != 1 || got.Status != "full" || got.CapacityChanged {
		t.Errorf("Ana's first week = %+v", got)
	}
}

func TestUtilizationNullForZeroCapacity(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test zero", "0")
	insertAssignment(t, tx, id, "2026-01-05", "2026-01-09", "4")

	rec := serveCapacity(tx, fmt.Sprintf("%s&person_id=%d", demoRange, id))
	var raw struct {
		People []struct {
			Weeks []map[string]any `json:"weeks"`
		} `json:"people"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	week := raw.People[0].Weeks[1]
	if v, present := week["utilization"]; !present || v != nil || week["status"] != "over" {
		t.Errorf("week of 2026-01-05 = %v", week)
	}
}

func TestKeysetPaginationIsStableAndComplete(t *testing.T) {
	tx := beginTx(t)
	for i := 5; i >= 1; i-- { // inserted out of order on purpose
		insertPerson(t, tx, fmt.Sprintf("zz-test %d", i), "40")
	}

	// Walk from just before the fixtures to the end of the roster. Non-Latin seed
	// names (Greek, Cyrillic, Hebrew, Arabic, CJK) sort after "zz", so the
	// fixtures come first but are not last.
	var seen []string
	cursor := afterSeed()
	for pages := 1; ; pages++ {
		page := getCapacity(t, tx, demoRange+"&limit=2&cursor="+cursor)
		if pages > 1 && page.OverCount != nil {
			t.Error("a page requested with a cursor carries over_count")
		}
		seen = append(seen, names(page)...)
		if page.NextCursor == nil {
			break
		}
		cursor = url.QueryEscape(*page.NextCursor)
		if pages > 100 {
			t.Fatal("pagination does not terminate")
		}
	}
	if len(seen) < 5 || fmt.Sprint(seen[:5]) != "[zz-test 1 zz-test 2 zz-test 3 zz-test 4 zz-test 5]" {
		t.Errorf("seen = %v", seen)
	}
	unique := map[string]bool{}
	for _, n := range seen {
		if unique[n] {
			t.Errorf("%q appears on two pages", n)
		}
		unique[n] = true
	}
}

func TestOverOnlyAndOverCount(t *testing.T) {
	tx := beginTx(t)
	before := getCapacity(t, tx, demoRange+"&limit=1")
	if before.OverCount == nil {
		t.Fatal("first page has no over_count")
	}

	over := insertPerson(t, tx, "zz-test over", "40")
	insertAssignment(t, tx, over, "2026-01-05", "2026-01-09", "9") // 45 h
	insertPerson(t, tx, "zz-test fine", "40")

	after := getCapacity(t, tx, demoRange+"&limit=1")
	if *after.OverCount != *before.OverCount+1 {
		t.Errorf("over_count %d → %d, want +1", *before.OverCount, *after.OverCount)
	}

	filtered := getCapacity(t, tx, demoRange+"&over_only=true&cursor="+afterSeed())
	if fmt.Sprint(names(filtered)) != "[zz-test over]" {
		t.Errorf("over_only returned %v", names(filtered))
	}
	if filtered.OverCount != nil {
		t.Errorf("a page requested with a cursor carries over_count %d", *filtered.OverCount)
	}
}

func TestPersonIDFilterStillReturnsRosterOverCount(t *testing.T) {
	tx := beginTx(t)
	id := insertPerson(t, tx, "zz-test single", "40")
	roster := getCapacity(t, tx, demoRange+"&limit=1")
	single := getCapacity(t, tx, fmt.Sprintf("%s&person_id=%d", demoRange, id))

	if len(single.People) != 1 || single.People[0].ID != id {
		t.Fatalf("people = %+v", single.People)
	}
	if single.OverCount == nil || *single.OverCount != *roster.OverCount {
		t.Errorf("over_count = %v, want roster-wide %d", single.OverCount, *roster.OverCount)
	}
}

func TestCapacityValidation(t *testing.T) {
	for _, tc := range []struct{ query, code string }{
		{"to=2026-01-16", "invalid_date"},
		{demoRange + "&limit=0", "invalid_limit"},
		{demoRange + "&limit=501", "invalid_limit"},
		{demoRange + "&limit=x", "invalid_limit"},
		{demoRange + "&cursor=!!", "invalid_cursor"},
		{demoRange + "&person_id=-1", "invalid_person_id"},
		{demoRange + "&person_id=abc", "invalid_person_id"},
		{demoRange + "&over_only=yes", "invalid_over_only"},
		{"from=2025-01-06&to=2026-03-01", "range_too_long"},
		// Values Postgres cannot hold must be rejected here, not fail as a 500 there.
		{demoRange + "&person_id=99999999999", "invalid_person_id"},
		{demoRange + "&cursor=" + encodeCursor(pageCursor{Name: "a", ID: 99999999999}), "invalid_cursor"},
		{"from=0000-01-03&to=0000-01-09", "invalid_date"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			rec := serveCapacity(testPool, tc.query)
			var body struct {
				Error struct{ Code string } `json:"error"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if rec.Code != http.StatusBadRequest || body.Error.Code != tc.code {
				t.Errorf("got %d %s, want 400 %s", rec.Code, rec.Body.String(), tc.code)
			}
		})
	}
}
