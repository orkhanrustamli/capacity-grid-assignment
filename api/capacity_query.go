package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// capacityQuery is a validated GET /api/capacity request.
type capacityQuery struct {
	Range     dateRange
	Limit     int
	After     *pageCursor // nil on the first page
	PersonIDs []int       // non-empty: just these people (used to refresh one row)
	OverOnly  bool
}

// pageCursor is the last person of the previous page in (name, id) order.
type pageCursor struct {
	Name string `json:"n"`
	ID   int    `json:"i"`
}

func encodeCursor(c pageCursor) string {
	b, _ := json.Marshal(c) // a struct of a string and an int always marshals
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*pageCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var c pageCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.ID < 0 {
		return nil, errors.New("negative id")
	}
	return &c, nil
}

type weekCell struct {
	Week            string   `json:"week"`
	AllocatedHours  float64  `json:"allocated_hours"`
	CapacityHours   float64  `json:"capacity_hours"`
	WeekendHours    float64  `json:"weekend_hours"`
	Utilization     *float64 `json:"utilization"` // null when capacity is 0
	Status          string   `json:"status"`      // over | full | under
	CapacityChanged bool     `json:"capacity_changed"`
}

type personRow struct {
	ID    int        `json:"id"`
	Name  string     `json:"name"`
	Weeks []weekCell `json:"weeks"`
}

type capacityPage struct {
	From       string      `json:"from"`
	To         string      `json:"to"`
	Weeks      []string    `json:"weeks"`
	People     []personRow `json:"people"`
	NextCursor *string     `json:"next_cursor"`
	// OverCount is the number of people over capacity in at least one week of
	// the range, across the whole roster. Only on the first page (no cursor).
	OverCount *int `json:"over_count,omitempty"`
}

// Two page statements chosen in Go, rather than one with "$ids IS NULL OR …":
// pgx caches prepared statements and Postgres switches to a generic plan after
// five runs, and a generic plan cannot pick an index per branch.
//
// over_only keeps people with at least one over week; the set is computed once
// for the range from the same function the cells and the count use.
const (
	pageByCursorSQL = `
		SELECT p.id, p.name FROM people p
		WHERE (p.name, p.id) > ($1::text, $2::int)
		  AND (NOT $3::bool OR p.id IN (
		        SELECT w.person_id FROM capacity_person_weeks($4::date, $5::date, (SELECT array_agg(id) FROM people)) w
		        WHERE w.status = 'over'))
		ORDER BY p.name, p.id
		LIMIT $6`

	pageByIDsSQL = `
		SELECT p.id, p.name FROM people p
		WHERE p.id = ANY ($1::int[])
		  AND (NOT $2::bool OR p.id IN (
		        SELECT w.person_id FROM capacity_person_weeks($3::date, $4::date, $1::int[]) w
		        WHERE w.status = 'over'))
		ORDER BY p.name, p.id
		LIMIT $5`

	// Rounded here, at the edge; everything before stays numeric.
	cellsSQL = `
		SELECT person_id, week_start::text,
		       round(allocated, 2)::float8, round(capacity, 2)::float8, round(weekend, 2)::float8,
		       CASE WHEN capacity = 0 THEN NULL ELSE round(allocated / capacity, 4)::float8 END,
		       status, capacity_changed
		FROM capacity_person_weeks($1::date, $2::date, $3::int[])
		ORDER BY person_id, week_start`

	overCountSQL = `
		SELECT count(DISTINCT person_id)::int
		FROM capacity_person_weeks($1::date, $2::date, (SELECT array_agg(id) FROM people))
		WHERE status = 'over'`
)

// queryCapacity returns one page of people, ordered by (name, id), each with
// one cell per week of the range.
func queryCapacity(ctx context.Context, db DBTX, q capacityQuery) (capacityPage, error) {
	from, to := q.Range.From.Format(dateLayout), q.Range.To.Format(dateLayout)
	page := capacityPage{From: from, To: to, Weeks: q.Range.weekStarts(), People: []personRow{}}

	people, err := queryPagePeople(ctx, db, q, from, to)
	if err != nil {
		return capacityPage{}, err
	}
	if len(people) > q.Limit {
		people = people[:q.Limit]
		last := people[len(people)-1]
		next := encodeCursor(pageCursor{Name: last.Name, ID: last.ID})
		page.NextCursor = &next
	}
	if err := fillWeeks(ctx, db, people, from, to); err != nil {
		return capacityPage{}, err
	}
	page.People = people

	if q.After == nil {
		var n int
		if err := db.QueryRow(ctx, overCountSQL, from, to).Scan(&n); err != nil {
			return capacityPage{}, err
		}
		page.OverCount = &n
	}
	return page, nil
}

// queryPagePeople fetches up to Limit+1 people; the extra one tells us there
// is a next page.
func queryPagePeople(ctx context.Context, db DBTX, q capacityQuery, from, to string) ([]personRow, error) {
	var sql string
	var args []any
	if len(q.PersonIDs) > 0 {
		sql, args = pageByIDsSQL, []any{q.PersonIDs, q.OverOnly, from, to, q.Limit + 1}
	} else {
		after := pageCursor{}
		if q.After != nil {
			after = *q.After
		}
		sql, args = pageByCursorSQL, []any{after.Name, after.ID, q.OverOnly, from, to, q.Limit + 1}
	}
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []personRow{}
	for rows.Next() {
		var p personRow
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

// fillWeeks loads the cells for the given people in one query.
func fillWeeks(ctx context.Context, db DBTX, people []personRow, from, to string) error {
	if len(people) == 0 {
		return nil
	}
	ids := make([]int, len(people))
	byID := make(map[int]*personRow, len(people))
	for i := range people {
		ids[i] = people[i].ID
		byID[people[i].ID] = &people[i]
	}
	rows, err := db.Query(ctx, cellsSQL, from, to, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var c weekCell
		if err := rows.Scan(&id, &c.Week, &c.AllocatedHours, &c.CapacityHours, &c.WeekendHours,
			&c.Utilization, &c.Status, &c.CapacityChanged); err != nil {
			return err
		}
		byID[id].Weeks = append(byID[id].Weeks, c)
	}
	return rows.Err()
}
