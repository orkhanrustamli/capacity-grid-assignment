// Package people serves PATCH /api/people/{id}: the handler that validates an
// edit of a person's weekly hours, and the transaction that stores it.
package people

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"capacity/api/database"
	"capacity/api/httpapi"
	"capacity/api/week"
)

const (
	maxPatchBodyBytes = 4 << 10
	maxWeeklyHours    = 168 // hours in a week
)

// weeklyHoursPattern is the accepted spelling of weekly_hours: up to three
// digits and at most two decimals, so the stored value is exactly what was
// typed and fits the 0–168 check.
var weeklyHoursPattern = regexp.MustCompile(`^\d{1,3}(\.\d{1,2})?$`)

// Handler serves PATCH /api/people/{id}.
//
// Body: {"weekly_hours": 32, "effective_from"?: "YYYY-MM-DD", "expected_weekly_hours"?: 40}.
// The new value applies from the Monday of effective_from (default: this
// week) until the person's next later change; earlier weeks keep their
// history. expected_weekly_hours, when sent, must equal the value in effect
// for that week, or the edit is refused with 409 (someone else changed it).
// Saving the value already in effect is a no-op that succeeds, so a retried
// save whose first response was lost does not conflict with itself.
type Handler struct {
	DB  database.DBTX
	Now func() time.Time
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.ParseID(r.PathValue("id"))
	if !ok {
		httpapi.WriteError(w, httpapi.BadRequest("invalid_person_id", "person id must be a positive integer"))
		return
	}
	update, err := h.parseCapacityUpdate(http.MaxBytesReader(w, r.Body, maxPatchBodyBytes))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	result, err := updateCapacity(r.Context(), h.DB, id, update, h.Now().UTC())
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) parseCapacityUpdate(body io.Reader) (capacityUpdate, error) {
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
	u := capacityUpdate{WeeklyHours: hours, EffectiveFrom: week.Start(h.Now().UTC())}

	if present(raw.EffectiveFrom) {
		var v string
		if err := json.Unmarshal(raw.EffectiveFrom, &v); err != nil {
			return capacityUpdate{}, invalidBody("effective_from must be a YYYY-MM-DD string")
		}
		d, err := week.ParseDate(v)
		if err != nil {
			return capacityUpdate{}, invalidBody("effective_from: " + err.Error())
		}
		u.EffectiveFrom = week.Start(d)
	}
	if present(raw.ExpectedWeeklyHours) {
		expected, ok := jsonNumber(raw.ExpectedWeeklyHours)
		if !ok || !weeklyHoursPattern.MatchString(expected) {
			return capacityUpdate{}, invalidBody("expected_weekly_hours must be a number with at most two decimals")
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
	return &httpapi.Error{Status: http.StatusUnprocessableEntity, Code: "invalid_weekly_hours",
		Message: fmt.Sprintf("weekly_hours must be between 0 and %d with at most two decimals", maxWeeklyHours)}
}

func invalidBody(message string) *httpapi.Error {
	return httpapi.BadRequest("invalid_body", message)
}
