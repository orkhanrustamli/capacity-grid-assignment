// Package httpapi holds what every handler shares: the JSON response writer,
// the error a client is meant to see, and parsing of id parameters.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// Error is an error the client is meant to see: a status, a stable
// machine-readable code, a human message, and optional extra fields that are
// merged into the error object (e.g. current_weekly_hours on a conflict).
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// BadRequest is a 400 with the given code and message.
func BadRequest(code, message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: code, Message: message}
}

// WriteJSON answers with v encoded as JSON and the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError answers with {"error": {"code", "message", ...details}}. Any
// error that is not an *Error is logged and reported as a bare 500, so
// database messages never reach the client.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		log.Printf("internal error: %v", err)
		apiErr = &Error{Status: http.StatusInternalServerError, Code: "internal", Message: "internal error"}
	}
	body := make(map[string]any, len(apiErr.Details)+2)
	for k, v := range apiErr.Details {
		body[k] = v
	}
	body["code"] = apiErr.Code
	body["message"] = apiErr.Message
	WriteJSON(w, apiErr.Status, map[string]any{"error": body})
}

// ParseID reads a person id: a positive integer that fits the int4 id column.
func ParseID(s string) (int, bool) {
	id, err := strconv.ParseInt(s, 10, 32)
	return int(id), err == nil && id > 0
}
