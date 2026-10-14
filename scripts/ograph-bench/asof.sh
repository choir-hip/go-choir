#!/usr/bin/env bash
# Write one synthetic "day" into an indexed LOCAL bench repo and time each
# write and the Dolt commit, then leave tag c1 (before) and head (after) for
# the asof suite. Runs through `dolt sql` in the repo dir (which connects to
# the local sql-server when one is running).
#   scripts/ograph-bench/asof.sh DB_DIR
set -euo pipefail
DB=${1:?db dir}
cd "$DB"
t() {
  local label=$1; shift
  local t1 t2
  t1=$(python3 -c 'import time;print(time.time())')
  dolt sql -q "$*" >/dev/null
  t2=$(python3 -c 'import time;print(time.time())')
  echo "$(python3 -c "print(f'{$t2-$t1:8.2f}s')") $label"
}
echo "size_before $(du -sh "$DB" | cut -f1)"
t "tag c1" "CALL DOLT_TAG('c1')"
# A day at today's rate: ~4,500 captures, ~22,000 claims (5M claims / 220 days),
# ~36,000 blocks (8 per capture). Copies of existing rows with new keys.
t "insert 4500 og_objects (gencol)" "INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by) SELECT CONCAT(canonical_id, '~d2'), object_kind, owner_id, computer_id, version_id, content_hash, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE object_kind = 'choir.web_capture' LIMIT 4500"
t "insert 4500 capture_index (FT title if indexed)" "INSERT INTO capture_index SELECT CONCAT(canonical_id, '~d2'), owner_id, computer_id, url_hash, canonical_url, host, source_id, lang, fetched_at, published_at, title, title_ci FROM capture_index LIMIT 4500"
t "insert 36000 capture_blocks (FT text if indexed)" "INSERT INTO capture_blocks SELECT CONCAT(capture_id, '~d2'), block_ord, lang, text FROM capture_blocks LIMIT 36000"
t "insert 22000 claims" "INSERT INTO claims SELECT CONCAT(claim_id, '~d2'), capture_id, speaker_entity_id, proposition_id, event_id, modality, stance, selection, uttered_at, fetched_at, deadline, settled, lang, gloss FROM claims LIMIT 22000"
t "insert 40000 og_edges" "INSERT INTO og_edges SELECT CONCAT(edge_id, '~d2'), from_id, to_id, kind, metadata, created_at, tombstone FROM og_edges LIMIT 40000"
t "settle 1000 forecasts" "UPDATE claims SET settled = 1 WHERE settled = 0 AND modality = 'forecast' LIMIT 1000"
t "dolt commit day" "CALL DOLT_COMMIT('-Am', 'day 2')"
echo "size_after_day $(du -sh "$DB" | cut -f1)"
