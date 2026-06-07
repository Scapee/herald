#!/bin/bash
set -e

# Concatenate and run all up migrations in lexicographic order.
# This script is placed in /docker-entrypoint-initdb.d/ and executed once
# on first initialization by the postgres entrypoint.
cat /docker-entrypoint-initdb.d/migrations/*.up.sql \
  | psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB"
