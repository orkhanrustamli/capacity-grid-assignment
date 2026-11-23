package week

import (
	"errors"
	"net/http"
	"testing"

	"capacity/api/httpapi"
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
			r, err := ParseRange(tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.From.Format(Layout); got != tc.wantFrom {
				t.Errorf("from = %s, want %s", got, tc.wantFrom)
			}
			if got := r.To.Format(Layout); got != tc.wantTo {
				t.Errorf("to = %s, want %s", got, tc.wantTo)
			}
			if got := r.Count(); got != tc.weeks {
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
			_, err := ParseRange(tc.from, tc.to)
			var apiErr *httpapi.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *httpapi.Error", err)
			}
			if apiErr.Code != tc.code || apiErr.Status != http.StatusBadRequest {
				t.Errorf("got %d %s, want 400 %s", apiErr.Status, apiErr.Code, tc.code)
			}
		})
	}
}

func TestParseRangeAcceptsExactly53Weeks(t *testing.T) {
	r, err := ParseRange("2025-01-06", "2026-01-11")
	if err != nil {
		t.Fatal(err)
	}
	if r.Count() != MaxRangeWeeks {
		t.Errorf("weeks = %d, want %d", r.Count(), MaxRangeWeeks)
	}
}
