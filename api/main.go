// Command api wires the HTTP server: it connects to Postgres, applies the
// migrations and routes each endpoint to its package.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"capacity/api/capacity"
	"capacity/api/database"
	"capacity/api/httpapi"
	"capacity/api/people"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://capacity:capacity@localhost:5432/capacity?sslmode=disable"
	}

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	for i := 0; i < 30; i++ {
		if err = db.Ping(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatalf("ping: %v", err)
	}

	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth(db))
	mux.Handle("GET /api/capacity", &capacity.Handler{DB: db})
	mux.Handle("PATCH /api/people/{id}", &people.Handler{DB: db, Now: time.Now})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleHealth(db database.DBTX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count int
		if err := db.QueryRow(r.Context(), `SELECT count(*) FROM people`).Scan(&count); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "people": count})
	}
}
