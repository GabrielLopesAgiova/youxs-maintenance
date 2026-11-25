#!/bin/sh
# Runs once, on the first start of the database volume (research R10).
# Creates the test database and applies every migration, in lexical order,
# to both the application database and the test database.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "CREATE DATABASE youxs_test"

for db in "$POSTGRES_DB" youxs_test; do
    for f in /migrations/*.sql; do
        [ -e "$f" ] || continue
        echo "initdb: applying $f to $db"
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$db" -f "$f"
    done
done
