package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

const (
	maxPatchBodyBytes = 4 << 10
	maxWeeklyHours    = 168 // hours in a week
)

// weeklyHoursPattern is the accepted spelling of weekly_hours: up to three
// digits and at most two decimals, so the stored value is exactly what was
// typed and fits the 0–168 check.
var weeklyHoursPattern = regexp.MustCompile(`^\d{1,3}(\.\d{1,2})?$`)

// handleUpdatePerson serves PATCH /api/people/{id}.
//
// Body: {"weekly_hours": 32, "effective_from"?: "YYYY-MM-DD", "expected_weekly_hours"?: 40}.
// The new value applies from the Monday of effective_from (default: this
// week) until the person's next later change; earlier weeks keep their
// history. expected_weekly_hours, when sent, must equal the value in effect
// for that week, or the edit is refused with 409 (someone else changed it).
func (s *server) handleUpdatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, badRequest("invalid_person_id", "person id must be a positive integer"))
		return
	}
	update, err := s.parseCapacityUpdate(http.MaxBytesReader(w, r.Body, maxPatchBodyBytes))
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := updateCapacity(r.Context(), s.db, id, update, s.now().UTC())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) parseCapacityUpdate(body io.Reader) (capacityUpdate, error) {
	var raw struct {
		WeeklyHours         json.RawMessage `json:"weekly_hours"`
		EffectiveFrom       json.RawMessage `json:"effective_from"`
		ExpectedWeeklyHours json.RawMessage `json:"expected_weekly_hours"`
	}
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return capacityUpdate{}, invalidBody(err.Error())
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return capacityUpdate{}, invalidBody("body must be a single JSON object")
	}

	hours, ok := jsonNumber(raw.WeeklyHours)
	if !ok {
		return capacityUpdate{}, invalidBody("weekly_hours is required and must be a number")
	}
	if err := validateWeeklyHours(hours); err != nil {
		return capacityUpdate{}, err
	}
	u := capacityUpdate{WeeklyHours: hours, EffectiveFrom: weekStart(s.now().UTC())}

	if present(raw.EffectiveFrom) {
		var v string
		if err := json.Unmarshal(raw.EffectiveFrom, &v); err != nil {
			return capacityUpdate{}, invalidBody("effective_from must be a YYYY-MM-DD string")
		}
		d, err := parseDate(v)
		if err != nil {
			return capacityUpdate{}, invalidBody("effective_from: " + err.Error())
		}
		u.EffectiveFrom = weekStart(d)
	}
	if present(raw.ExpectedWeeklyHours) {
		expected, ok := jsonNumber(raw.ExpectedWeeklyHours)
		if !ok {
			return capacityUpdate{}, invalidBody("expected_weekly_hours must be a number")
		}
		u.Expected = &expected
	}
	return u, nil
}

// jsonNumber returns the literal text of a JSON number, rejecting strings,
// null and every other JSON type. The decoder has already checked syntax, so
// a value starting with '-' or a digit is a number.
func jsonNumber(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || (raw[0] != '-' && (raw[0] < '0' || raw[0] > '9')) {
		return "", false
	}
	return string(raw), true
}

// present reports whether an optional field was sent with a value; an
// explicit null counts as absent.
func present(raw json.RawMessage) bool {
	return len(raw) > 0 && !bytes.Equal(raw, []byte("null"))
}

func validateWeeklyHours(v string) error {
	if weeklyHoursPattern.MatchString(v) {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f <= maxWeeklyHours {
			return nil
		}
	}
	return &apiError{Status: http.StatusUnprocessableEntity, Code: "invalid_weekly_hours",
		Message: fmt.Sprintf("weekly_hours must be between 0 and %d with at most two decimals", maxWeeklyHours)}
}

func invalidBody(message string) *apiError {
	return badRequest("invalid_body", message)
}
