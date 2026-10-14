#!/usr/bin/env bash
# Apply indexes.sql one statement at a time against a LOCAL dolt repo (no
# server running), timing each and recording repo size after a commit.
#   scripts/ograph-bench/stage.sh DB_DIR [SQL_FILE]
set -euo pipefail
DB=${1:?db dir}
HERE=$(cd "$(dirname "$0")" && pwd)
SQL=${2:-$HERE/indexes.sql}
cd "$DB"
grep -v '^--' "$SQL" | grep -v '^[[:space:]]*$' | while IFS= read -r stmt; do
  t1=$(python3 -c 'import time;print(time.time())')
  if dolt sql -q "$stmt" >/dev/null 2>"$DB/../stage.err"; then status=ok; else status="FAILED: $(head -c 300 "$DB/../stage.err")"; fi
  t2=$(python3 -c 'import time;print(time.time())')
  echo "$(python3 -c "print(f'{$t2-$t1:8.1f}s')") $status :: ${stmt:0:110} [size $(du -sh "$DB" | cut -f1)]"
  case "$stmt" in
    *FULLTEXT*) dolt gc >/dev/null 2>&1 || true; echo "         gc after fulltext [size $(du -sh "$DB" | cut -f1)]" ;;
  esac
done
dolt add -A >/dev/null && dolt commit -m "proposed indexes" >/dev/null || true
echo "size_after_indexes $(du -sh "$DB" | cut -f1)"
dolt gc >/dev/null 2>&1 || true
echo "size_after_gc $(du -sh "$DB" | cut -f1)"
