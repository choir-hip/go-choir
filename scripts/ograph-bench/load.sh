#!/usr/bin/env bash
# Build a LOCAL dolt database from ograph-bench CSVs, then build Store B's
# shipped indexes, timing each step and recording on-disk size. Never points
# at a platform database: it creates its own repo under DB_DIR.
#
#   scripts/ograph-bench/load.sh DATA_DIR DB_DIR [TABLE...]
#
# Large CSVs are imported in 1M-row chunks with `dolt gc` after each chunk:
# a single 10M-row import grew the repo's journal past 12 GB of garbage on a
# 1.5M-row table (gc then left 537 MB), which a nearly full laptop disk cannot
# hold. Tables that already have rows are skipped, so a run can resume.
set -euo pipefail
DATA=${1:?data dir}
DB=${2:?db dir}
shift 2
TABLES=("$@")
[ ${#TABLES[@]} -eq 0 ] && TABLES=(og_objects og_edges capture_index capture_blocks claims entities entity_aliases alias_trigrams world_events event_participants)
HERE=$(cd "$(dirname "$0")" && pwd)
CHUNK=${CHUNK_ROWS:-1000000}

size() { du -sh "$DB" | cut -f1; }
now() { python3 -c 'import time;print(time.time())'; }
elapsed() { python3 -c "print(f'{$2-$1:.1f}')"; }

mkdir -p "$DB"
cd "$DB"
if [ ! -d .dolt ]; then
  dolt init --name bench --email bench@localhost >/dev/null
fi
if ! dolt sql -r csv -q "SHOW TABLES" | grep -qx og_objects; then
  dolt sql < "$HERE/schema.sql"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
t0=$(now)
for tbl in "${TABLES[@]}"; do
  have=$(dolt sql -r csv -q "SELECT COUNT(*) FROM $tbl" | tail -1)
  if [ "$have" != "0" ]; then
    echo "import $tbl skipped ($have rows present)"
    continue
  fi
  t1=$(now)
  rm -f "$TMP"/part_*
  tail -n +2 "$DATA/$tbl.csv" | split -l "$CHUNK" - "$TMP/part_"
  header=$(head -1 "$DATA/$tbl.csv")
  for part in "$TMP"/part_*; do
    { echo "$header"; cat "$part"; } > "$TMP/chunk.csv"
    dolt table import -u "$tbl" "$TMP/chunk.csv" >/dev/null 2>"$TMP/err" || { cat "$TMP/err"; exit 1; }
    dolt gc >/dev/null 2>&1 || true
  done
  echo "import $tbl $(elapsed "$t1" "$(now)")s (incl gc) size $(size)"
done
dolt add -A >/dev/null && dolt commit -m "import" >/dev/null || true
echo "stage import $(elapsed "$t0" "$(now)")s $(size)"

t0=$(now)
dolt sql -q "CREATE INDEX idx_og_objects_kind_owner ON og_objects(object_kind, owner_id);
CREATE INDEX idx_og_objects_updated ON og_objects(updated_at);
CREATE INDEX idx_og_edges_from ON og_edges(from_id);
CREATE INDEX idx_og_edges_to ON og_edges(to_id);" >/dev/null
dolt add -A >/dev/null && dolt commit -m "store b indexes" >/dev/null
echo "stage store_b_indexes $(elapsed "$t0" "$(now)")s $(size)"
dolt gc >/dev/null 2>&1 || true
echo "stage gc $(size)"
