package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRange(t *testing.T) {
	ok := []struct {
		name, from, to, wantFrom, wantTo string
		weeks                            int
	}{
		{"demo range, to snaps to Sunday", "2025-12-29", "2026-01-16", "2025-12-29", "2026-01-18", 3},
		{"mid-week day crossing the year", "2025-12-31", "2025-12-31", "2025-12-29", "2026-01-04", 1},
		{"ISO week 53 of 2026", "2026-12-28", "2027-01-03", "2026-12-28", "2027-01-03", 1},
		{"a single Monday", "2026-03-09", "2026-03-09", "2026-03-09", "2026-03-15", 1},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			r, err := parseRange(tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.From.Format(dateLayout); got != tc.wantFrom {
				t.Errorf("from = %s, want %s", got, tc.wantFrom)
			}
			if got := r.To.Format(dateLayout); got != tc.wantTo {
				t.Errorf("to = %s, want %s", got, tc.wantTo)
			}
			if got := r.weekCount(); got != tc.weeks {
				t.Errorf("weeks = %d, want %d", got, tc.weeks)
			}
		})
	}

	bad := []struct{ name, from, to, code string }{
		{"missing from", "", "2026-01-16", "invalid_date"},
		{"not zero-padded", "2026-1-5", "2026-01-16", "invalid_date"},
		{"impossible day", "2026-02-30", "2026-03-01", "invalid_date"},
		{"missing to", "2026-01-05", "", "invalid_date"},
		{"from after to", "2026-01-16", "2025-12-29", "invalid_range"},
		{"53 weeks plus one", "2025-01-06", "2026-01-12", "range_too_long"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRange(tc.from, tc.to)
			var apiErr *apiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *apiError", err)
			}
			if apiErr.Code != tc.code || apiErr.Status != http.StatusBadRequest {
				t.Errorf("got %d %s, want 400 %s", apiErr.Status, apiErr.Code, tc.code)
			}
		})
	}
}

func TestParseRangeAcceptsExactly53Weeks(t *testing.T) {
	r, err := parseRange("2025-01-06", "2026-01-11")
	if err != nil {
		t.Fatal(err)
	}
	if r.weekCount() != maxRangeWeeks {
		t.Errorf("weeks = %d, want %d", r.weekCount(), maxRangeWeeks)
	}
}

func TestWriteErrorUsesAPIErrorStatusAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, &apiError{Status: http.StatusConflict, Code: "capacity_conflict", Message: "changed",
		Details: map[string]any{"current_weekly_hours": 36}})

	var body struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || body.Error["code"] != "capacity_conflict" ||
		body.Error["message"] != "changed" || body.Error["current_weekly_hours"] != float64(36) {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWriteErrorHidesInternalErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errors.New("pq: secret column detail"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}
