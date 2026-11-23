package main

import (
	"net/http"
	"net/url"
	"strconv"
)

const (
	defaultPageSize = 100
	maxPageSize     = 500
)

// handleCapacity serves GET /api/capacity?from=YYYY-MM-DD&to=YYYY-MM-DD
// [&limit=100][&cursor=…][&person_id=…][&over_only=true]
//
// One page of people in (name, id) order, each with allocated, weekend and
// capacity hours and a status for every week of the range. See README for the
// contract and ADR 0006 for pagination.
func (s *server) handleCapacity(w http.ResponseWriter, r *http.Request) {
	q, err := parseCapacityQuery(r.URL.Query())
	if err != nil {
		writeError(w, err)
		return
	}
	page, err := queryCapacity(r.Context(), s.db, q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseCapacityQuery(v url.Values) (capacityQuery, error) {
	rng, err := parseRange(v.Get("from"), v.Get("to"))
	if err != nil {
		return capacityQuery{}, err
	}
	q := capacityQuery{Range: rng, Limit: defaultPageSize}

	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			return capacityQuery{}, badRequest("invalid_limit", "limit must be an integer from 1 to 500")
		}
		q.Limit = n
	}
	if s := v.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			return capacityQuery{}, badRequest("invalid_cursor", "cursor is not one this API returned")
		}
		q.After = c
	}
	for _, s := range v["person_id"] {
		id, ok := parsePersonID(s)
		if !ok {
			return capacityQuery{}, badRequest("invalid_person_id", "person_id must be a positive integer")
		}
		q.PersonIDs = append(q.PersonIDs, id)
	}
	switch v.Get("over_only") {
	case "", "false":
	case "true":
		q.OverOnly = true
	default:
		return capacityQuery{}, badRequest("invalid_over_only", "over_only must be true or false")
	}
	return q, nil
}
