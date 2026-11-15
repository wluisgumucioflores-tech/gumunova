#!/bin/sh
# Applies the pending migrations, then replaces this shell with the API (so it receives SIGTERM).
set -e
/app/migrate
exec /app/api
