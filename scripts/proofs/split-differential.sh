#!/usr/bin/env bash
# Store header/body split differential (docs/design/store-header-body-split-2026-11-01.md
# section 8; S3 on a paper copy, then d@e, then each ring before finalize).
#
#   split-differential.sh OLD_DB NEW_DB [OUTDIR]
#     OLD_DB   state.db.presplit (the pre-split file the migration kept)
#     NEW_DB   state.db (the migrated file)
#     OUTDIR   report directory (default: a fresh mktemp -d)
#
# Runs cmd/store-split-diff (read-only, streaming, nice/ionice) and writes
# OUTDIR/report.json. Exit 0 only when every check is equal.
#
# Keep set: when KEEPSET_BEFORE=DIR names a snapshot taken before the
# migration (scripts/cutover/keepset-snapshot.sh new DIR), this takes the
# "after" snapshot and diffs the two. The keep-set script reads the platform
# graph database and so only applies on Node B.
#
# Env:
#   STORE_SPLIT_DIFF  path to a built store-split-diff binary (default: go run)
#   AS_OF             RFC 3339 end of the 30-day retire window (default: now)
#   DAYS              retire window days (default 30)
set -euo pipefail

if [ "$#" -lt 2 ]; then
	sed -n '2,22p' "$0" >&2
	exit 2
fi
OLD_DB="$1"
NEW_DB="$2"
OUT="${3:-$(mktemp -d)}"
mkdir -p "$OUT"
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

LIMIT=()
if command -v ionice >/dev/null 2>&1; then
	LIMIT=(nice -n 19 ionice -c3)
elif command -v nice >/dev/null 2>&1; then
	LIMIT=(nice -n 19)
fi

ARGS=(--old "$OLD_DB" --new "$NEW_DB" --days "${DAYS:-30}")
if [ -n "${AS_OF:-}" ]; then
	ARGS+=(--as-of "$AS_OF")
fi

rc=0
if [ -n "${STORE_SPLIT_DIFF:-}" ]; then
	"${LIMIT[@]}" "$STORE_SPLIT_DIFF" "${ARGS[@]}" >"$OUT/report.json" || rc=$?
else
	(cd "$REPO" && "${LIMIT[@]}" go run ./cmd/store-split-diff "${ARGS[@]}") >"$OUT/report.json" || rc=$?
fi
echo "store-split-diff exit=$rc report=$OUT/report.json" >&2

if [ -n "${KEEPSET_BEFORE:-}" ]; then
	"$REPO/scripts/cutover/keepset-snapshot.sh" new "$OUT/keepset-after"
	"$REPO/scripts/cutover/keepset-snapshot.sh" diff "$KEEPSET_BEFORE" "$OUT/keepset-after" || rc=$?
fi

exit "$rc"
