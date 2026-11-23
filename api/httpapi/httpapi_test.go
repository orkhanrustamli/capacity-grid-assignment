package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteErrorUsesAPIErrorStatusAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, &Error{Status: http.StatusConflict, Code: "capacity_conflict", Message: "changed",
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
	WriteError(rec, errors.New("pq: secret column detail"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}
