// Package week parses dates and snaps them to weeks.
//
// Weeks are ISO weeks, Monday to Sunday, identified by their Monday as
// YYYY-MM-DD. Never by week number: 2025-12-29 is week 1 of 2026, and 2026
// has a week 53. See ADR 0001.
package week

import (
	"fmt"
	"time"

	"capacity/api/httpapi"
)

const (
	// Layout is the YYYY-MM-DD form every date takes on the wire and in SQL.
	Layout = "2006-01-02"
	// MaxRangeWeeks bounds one request: a year of weeks per screen is plenty,
	// and it caps the response at people × 53 cells.
	MaxRangeWeeks = 53
)

// ParseDate reads a strict YYYY-MM-DD calendar date as UTC midnight.
func ParseDate(s string) (time.Time, error) {
	d, err := time.Parse(Layout, s)
	if err != nil || d.Format(Layout) != s {
		return time.Time{}, fmt.Errorf("%q is not a YYYY-MM-DD date", s)
	}
	if d.Year() < 1 {
		// Go accepts year 0000; Postgres has no year 0 and would fail the query.
		return time.Time{}, fmt.Errorf("%q is before year 1", s)
	}
	return d, nil
}

// Start returns the Monday on or before d.
func Start(d time.Time) time.Time {
	daysSinceMonday := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -daysSinceMonday)
}

// Range is a run of whole weeks: From is a Monday, To is a Sunday.
type Range struct {
	From, To time.Time
}

// Count is the number of weeks in the range.
func (r Range) Count() int {
	return int(r.To.Sub(r.From).Hours()/24+1) / 7
}

// Starts lists the Monday of every week in the range.
func (r Range) Starts() []string {
	out := make([]string, 0, r.Count())
	for d := r.From; d.Before(r.To); d = d.AddDate(0, 0, 7) {
		out = append(out, d.Format(Layout))
	}
	return out
}

// ParseRange validates the from/to query parameters and widens them to whole
// weeks: from snaps back to its Monday, to forward to its Sunday.
func ParseRange(from, to string) (Range, error) {
	f, err := ParseDate(from)
	if err != nil {
		return Range{}, httpapi.BadRequest("invalid_date", "from: "+err.Error())
	}
	t, err := ParseDate(to)
	if err != nil {
		return Range{}, httpapi.BadRequest("invalid_date", "to: "+err.Error())
	}
	if f.After(t) {
		return Range{}, httpapi.BadRequest("invalid_range", "from must be on or before to")
	}
	r := Range{From: Start(f), To: Start(t).AddDate(0, 0, 6)}
	if r.Count() > MaxRangeWeeks {
		return Range{}, httpapi.BadRequest("range_too_long", fmt.Sprintf("a range covers at most %d weeks", MaxRangeWeeks))
	}
	return r, nil
}
