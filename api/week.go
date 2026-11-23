package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Weeks are ISO weeks, Monday to Sunday, identified by their Monday as
// YYYY-MM-DD. Never by week number: 2025-12-29 is week 1 of 2026, and 2026
// has a week 53. See ADR 0001.
const (
	dateLayout = "2006-01-02"
	// maxRangeWeeks bounds one request: a year of weeks per screen is plenty,
	// and it caps the response at people × 53 cells.
	maxRangeWeeks = 53
)

// parseDate reads a strict YYYY-MM-DD calendar date as UTC midnight.
func parseDate(s string) (time.Time, error) {
	d, err := time.Parse(dateLayout, s)
	if err != nil || d.Format(dateLayout) != s {
		return time.Time{}, fmt.Errorf("%q is not a YYYY-MM-DD date", s)
	}
	if d.Year() < 1 {
		// Go accepts year 0000; Postgres has no year 0 and would fail the query.
		return time.Time{}, fmt.Errorf("%q is before year 1", s)
	}
	return d, nil
}

// weekStart returns the Monday on or before d.
func weekStart(d time.Time) time.Time {
	daysSinceMonday := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -daysSinceMonday)
}

// dateRange is a run of whole weeks: From is a Monday, To is a Sunday.
type dateRange struct {
	From, To time.Time
}

func (r dateRange) weekCount() int {
	return int(r.To.Sub(r.From).Hours()/24+1) / 7
}

// parseRange validates the from/to query parameters and widens them to whole
// weeks: from snaps back to its Monday, to forward to its Sunday.
func parseRange(from, to string) (dateRange, error) {
	f, err := parseDate(from)
	if err != nil {
		return dateRange{}, badRequest("invalid_date", "from: "+err.Error())
	}
	t, err := parseDate(to)
	if err != nil {
		return dateRange{}, badRequest("invalid_date", "to: "+err.Error())
	}
	if f.After(t) {
		return dateRange{}, badRequest("invalid_range", "from must be on or before to")
	}
	r := dateRange{From: weekStart(f), To: weekStart(t).AddDate(0, 0, 6)}
	if r.weekCount() > maxRangeWeeks {
		return dateRange{}, badRequest("range_too_long", fmt.Sprintf("a range covers at most %d weeks", maxRangeWeeks))
	}
	return r, nil
}

// parsePersonID reads a person id: a positive integer that fits the int4 id column.
func parsePersonID(s string) (int, bool) {
	id, err := strconv.ParseInt(s, 10, 32)
	return int(id), err == nil && id > 0
}

func badRequest(code, message string) *apiError {
	return &apiError{Status: http.StatusBadRequest, Code: code, Message: message}
}

// weekStarts lists the Monday of every week in the range.
func (r dateRange) weekStarts() []string {
	out := make([]string, 0, r.weekCount())
	for d := r.From; d.Before(r.To); d = d.AddDate(0, 0, 7) {
		out = append(out, d.Format(dateLayout))
	}
	return out
}
