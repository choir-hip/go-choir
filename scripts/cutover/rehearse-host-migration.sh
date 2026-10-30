#!/usr/bin/env bash
# C4 rehearsal / C5 run of the host migration: Store A (platform Dolt) -> the
# PostgreSQL database control, Store B (corpus Dolt) -> graph. Wraps
# choir-migrate-host (cmd/choir-migrate-host, internal/migrate/hostcopy), writes
# its JSON report, and prints the differential summary.
#
# The source servers are read-only (one REPEATABLE READ transaction per table),
# so this is safe against the LIVE Dolt servers; every target table is
# truncated first, so a rerun starts clean. Memory is bounded (streamed rows).
#
# Usage:
#   rehearse-host-migration.sh [--dry-run] [--tables a,b] [--parallel N] [--report PATH]
#
# Connection settings (environment; no password goes on a command line):
#   SOURCE_CONTROL_DSN  default choir@tcp(127.0.0.1:13306)/platform?parseTime=true
#   SOURCE_GRAPH_DSN    default choir@tcp(127.0.0.1:13307)/corpus?parseTime=true
#   TARGET_CONTROL_DSN  default postgres://choir@127.0.0.1:5432/control?sslmode=disable
#   TARGET_GRAPH_DSN    default postgres://choir@127.0.0.1:5432/graph?sslmode=disable
#   CHOIR_DOLT_USER / CHOIR_DOLT_PASSWORD_FILE   Dolt credential (internal/dbcred),
#     default user choir, password /var/lib/go-choir/secrets/dolt-app-password
#   CHOIR_PG_PASSWORD_FILE   default /var/lib/go-choir/secrets/pg-app-password
#   CHOIR_MIGRATE_HOST_BIN   the binary (default: choir-migrate-host on PATH,
#     else `go run ./cmd/choir-migrate-host` from the repo)
#
# A scratch PostgreSQL (rehearsal) is just different TARGET_* DSNs, for example
# a database created with `createdb -O choir control_rehearsal`.
#
# --dry-run counts rows, intersects columns and censuses text PostgreSQL cannot
# store (NUL bytes, invalid UTF-8, with sample keys); it fails if any is found.
#
# Exit status: 0 every table verified (row count and digest equal); 1 otherwise.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
report="${REPORT:-/var/lib/go-choir/migration-reports/host-$(date -u +%Y%m%dT%H%M%SZ).json}"
extra=()
while [ $# -gt 0 ]; do
  case "$1" in
    --report) report="$2"; shift 2 ;;
    --dry-run) extra+=(--dry-run); shift ;;
    --tables) extra+=(--tables "$2"); shift 2 ;;
    --parallel) extra+=(--parallel "$2"); shift 2 ;;
    -h|--help) sed -n 2,34p "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

export CHOIR_DOLT_USER="${CHOIR_DOLT_USER:-choir}"
export CHOIR_DOLT_PASSWORD_FILE="${CHOIR_DOLT_PASSWORD_FILE:-/var/lib/go-choir/secrets/dolt-app-password}"
export CHOIR_PG_PASSWORD_FILE="${CHOIR_PG_PASSWORD_FILE:-/var/lib/go-choir/secrets/pg-app-password}"
src_control="${SOURCE_CONTROL_DSN:-choir@tcp(127.0.0.1:13306)/platform?parseTime=true}"
src_graph="${SOURCE_GRAPH_DSN:-choir@tcp(127.0.0.1:13307)/corpus?parseTime=true}"
dst_control="${TARGET_CONTROL_DSN:-postgres://choir@127.0.0.1:5432/control?sslmode=disable}"
dst_graph="${TARGET_GRAPH_DSN:-postgres://choir@127.0.0.1:5432/graph?sslmode=disable}"

mkdir -p "$(dirname "$report")"
bin="${CHOIR_MIGRATE_HOST_BIN:-$(command -v choir-migrate-host || true)}"
if [ -n "$bin" ]; then
  cmd=("$bin")
else
  cmd=(go run "$repo/cmd/choir-migrate-host")
fi

echo "report: $report" >&2
rc=0
"${cmd[@]}" \
  --source-control-dsn "$src_control" --source-graph-dsn "$src_graph" \
  --target-control-dsn "$dst_control" --target-graph-dsn "$dst_graph" \
  --report "$report" ${extra[@]+"${extra[@]}"} || rc=$?

if command -v jq >/dev/null && [ -s "$report" ]; then
  echo
  echo "== differential (tables whose source and target digests differ or that failed) =="
  jq -r '[.control, .graph] | map(select(. != null)) | .[] | .tables[] | select(.ok != true) | "\(.name): \(.error)"' "$report" || true
  echo "== totals =="
  jq -r '[.control, .graph] | map(select(. != null)) | .[] | "\(.target): \(.tables | length) tables, \(.total_rows) rows, ok=\(.ok)"' "$report" || true
fi
exit "$rc"
