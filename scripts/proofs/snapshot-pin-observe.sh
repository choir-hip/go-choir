#!/bin/sh
# Watch one snapshot capture's read pin from outside, on a test computer (d@e),
# for S1 of docs/design/snapshot-without-scratch-copy-2026-11-20.md.
#
#   scripts/proofs/snapshot-pin-observe.sh <runtime base url> [seconds between samples] [samples]
#
# Each sample prints the UTC time, the status code and latency of GET /health
# (a 503 store_stalled is counted), the WAL file size, the last checkpoint after
# a long read (frames, how long it took, the WAL size after it) and the snapshot
# producer's last capture (outcome, pin, witness, upload, WAL frames the pin
# held back, scratch bytes), and while a capture runs the WAL its pin holds now
# (updated at the capture's own one-minute tick). Read-only: two GETs a sample. Needs curl and jq.
#
# The route that needs a credential takes it from the environment, never from
# this file: SNAPSHOT_OBSERVE_HEADER, a full header line such as
# "Authorization: Bearer ..." (set it in your own shell; do not commit it).
set -eu
base=${1:?usage: snapshot-pin-observe.sh <runtime base url> [seconds] [samples]}
every=${2:-30}
samples=${3:-240}
hdr=${SNAPSHOT_OBSERVE_HEADER:-}
stalled=0
run=0
longest=0
n=0
while [ "$n" -lt "$samples" ]; do
  now=$(date -u +%FT%TZ)
  health=$(curl -s -o /dev/null -w '%{http_code} %{time_total}s' --max-time 20 "$base/health" || echo "curl-failed")
  case "$health" in
    503*) stalled=$((stalled + 1)); run=$((run + 1)); [ "$run" -gt "$longest" ] && longest=$run ;;
    *) run=0 ;;
  esac
  if [ -n "$hdr" ]; then
    obs=$(curl -s --max-time 20 -H "$hdr" "$base/api/runtime/observability" || echo '{}')
  else
    obs=$(curl -s --max-time 20 "$base/api/runtime/observability" || echo '{}')
  fi
  line=$(printf '%s' "$obs" | jq -r '
    "wal_file=\(.wal.wal_bytes // "-")"
    + " ckpt_frames=\(.wal.last_checkpoint.log_frames // "-") ckpt_took=\(.wal.last_checkpoint.duration // "-") ckpt_wal_after=\(.wal.last_checkpoint.wal_bytes_after // "-")"
    + " in_flight=\(.snapshot_producer.in_flight) pin_wal_held_bytes_now=\(.snapshot_producer.pin_wal_held_bytes_now // "-")"
    + " outcome=\(.snapshot_producer.last_outcome // "-")"
    + " pin_s=\(.snapshot_producer.last_pin_seconds // "-") witness_s=\(.snapshot_producer.last_witness_seconds // "-") stream_s=\(.snapshot_producer.last_stream_seconds // "-") drain_s=\(.snapshot_producer.last_drain_seconds // "-")"
    + " pin_wal_frames=\(.snapshot_producer.last_pin_wal_frames // "-") pin_wal_frame_bytes=\(.snapshot_producer.last_pin_wal_frame_bytes // "-") frames_known=\(.snapshot_producer.last_pin_wal_frames_known)"
    + " pin_events=\(.snapshot_producer.last_pin_events // "-") scratch_bytes=\(.snapshot_producer.last_scratch_bytes // "-")" ' 2>/dev/null || echo "observability unreadable")
  echo "$now health=$health store_stalled_total=$stalled longest_stalled_run_samples=$longest (x ${every}s) $line"
  n=$((n + 1))
  sleep "$every"
done
