#!/usr/bin/env bash
# Builds capacity_bench (a scaled copy of the seeded database) and prints
# EXPLAIN (ANALYZE, BUFFERS) for the API's real statements under one index
# candidate. Only reads the capacity database.
#
#   bash bench/run.sh build                 # (re)create capacity_bench
#   bash bench/run.sh explain <candidate>   # baseline | btree | gist
set -euo pipefail
cd "$(dirname "$0")/.."

psql() { docker compose exec -T db psql -v ON_ERROR_STOP=1 -q -U capacity "$@"; }

case "${1:-}" in
  build)
    psql capacity -c "DROP DATABASE IF EXISTS capacity_bench" -c "CREATE DATABASE capacity_bench"
    docker compose exec -T db sh -c "pg_dump -U capacity capacity | psql -q -v ON_ERROR_STOP=1 -U capacity capacity_bench" >/dev/null
    psql capacity_bench -f - < bench/scale.sql >/dev/null
    psql capacity_bench -c "VACUUM ANALYZE"
    psql capacity_bench -Atc "SELECT 'people ' || count(*) FROM people UNION ALL SELECT 'assignments ' || count(*) FROM assignments"
    ;;
  explain)
    candidate="${2:?candidate}"
    psql capacity_bench -f - < "bench/candidates/$candidate.sql"
    # Install the function exactly as last migrated (the newest migration that
    # defines it); the gist candidate gets its range filter rewritten as a
    # daterange overlap so the GiST index can serve it.
    fn_file=$(grep -l 'FUNCTION capacity_person_weeks' api/migrations/*.sql | sort | tail -1)
    fn=$(sed 's/^CREATE FUNCTION/CREATE OR REPLACE FUNCTION/' "$fn_file")
    if [ "$candidate" = gist ]; then
      fn=$(printf '%s' "$fn" | sed "s/AND a.end_date >= p_from AND a.start_date <= p_to/AND daterange(a.start_date, a.end_date, '[]') \&\& daterange(p_from, p_to, '[]')/")
    fi
    printf '%s\n' "$fn" | psql capacity_bench -f -
    psql capacity_bench -c "VACUUM ANALYZE assignments"
    psql capacity_bench -f - < bench/explain.sql
    ;;
  *) echo "usage: $0 build | explain <baseline|btree|gist>" >&2; exit 2 ;;
esac
