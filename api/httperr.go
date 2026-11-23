package main

import (
	"errors"
	"log"
	"net/http"
)

// apiError is an error the client is meant to see: a status, a stable
// machine-readable code, a human message, and optional extra fields that are
// merged into the error object (e.g. current_weekly_hours on a conflict).
type apiError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

// writeError answers with {"error": {"code", "message", ...details}}. Any
// error that is not an *apiError is logged and reported as a bare 500, so
// database messages never reach the client.
func writeError(w http.ResponseWriter, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		log.Printf("internal error: %v", err)
		apiErr = &apiError{Status: http.StatusInternalServerError, Code: "internal", Message: "internal error"}
	}
	body := make(map[string]any, len(apiErr.Details)+2)
	for k, v := range apiErr.Details {
		body[k] = v
	}
	body["code"] = apiErr.Code
	body["message"] = apiErr.Message
	writeJSON(w, apiErr.Status, map[string]any{"error": body})
}
