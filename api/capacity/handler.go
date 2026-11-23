// Package capacity serves GET /api/capacity: the capacity query, its keyset
// cursor, and the handler that parses the request.
package capacity

import (
	"net/http"
	"net/url"
	"strconv"

	"capacity/api/database"
	"capacity/api/httpapi"
	"capacity/api/week"
)

const (
	defaultPageSize = 100
	maxPageSize     = 500
)

// Handler serves GET /api/capacity?from=YYYY-MM-DD&to=YYYY-MM-DD
// [&limit=100][&cursor=…][&person_id=…][&over_only=true]
//
// One page of people in (name, id) order, each with allocated, weekend and
// capacity hours and a status for every week of the range. See README for the
// contract and ADR 0006 for pagination.
type Handler struct {
	DB database.DBTX
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q, err := parseQuery(r.URL.Query())
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	page, err := Load(r.Context(), h.DB, q)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func parseQuery(v url.Values) (Query, error) {
	rng, err := week.ParseRange(v.Get("from"), v.Get("to"))
	if err != nil {
		return Query{}, err
	}
	q := Query{Range: rng, Limit: defaultPageSize}

	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			return Query{}, httpapi.BadRequest("invalid_limit", "limit must be an integer from 1 to 500")
		}
		q.Limit = n
	}
	if s := v.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			return Query{}, httpapi.BadRequest("invalid_cursor", "cursor is not one this API returned")
		}
		q.After = c
	}
	for _, s := range v["person_id"] {
		id, ok := httpapi.ParseID(s)
		if !ok {
			return Query{}, httpapi.BadRequest("invalid_person_id", "person_id must be a positive integer")
		}
		q.PersonIDs = append(q.PersonIDs, id)
	}
	switch v.Get("over_only") {
	case "", "false":
	case "true":
		q.OverOnly = true
	default:
		return Query{}, httpapi.BadRequest("invalid_over_only", "over_only must be true or false")
	}
	return q, nil
}
