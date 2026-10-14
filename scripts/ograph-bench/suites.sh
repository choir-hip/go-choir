#!/usr/bin/env bash
# Run query suites against a LOCAL dolt sql-server (127.0.0.1 only).
#   scripts/ograph-bench/suites.sh DB_NAME PARAMS_JSON [PORT] [SUITES...] [-- extra run flags]
set -euo pipefail
DBNAME=${1:?database name}
PARAMS=${2:?params.json}
PORT=${3:-3399}
shift 3 || true
SUITES=()
while [ $# -gt 0 ] && [ "$1" != "--" ]; do SUITES+=("$1"); shift; done
[ "${1:-}" = "--" ] && shift
[ ${#SUITES[@]} -eq 0 ] && SUITES=(baseline indexed fts ledger)
HERE=$(cd "$(dirname "$0")" && pwd)
BIN=${OGRAPH_BENCH_BIN:-}
if [ -z "$BIN" ]; then
  BIN=$(mktemp -d)/ograph-bench
  (cd "$HERE/../.." && go build -o "$BIN" ./scripts/ograph-bench)
fi
for s in "${SUITES[@]}"; do
  echo "### suite $s"
  "$BIN" run -dsn "root@tcp(127.0.0.1:${PORT})/${DBNAME}" -params "$PARAMS" -suite "$s" "$@"
done
