#!/usr/bin/env bash
# The weekly prune keeps the last 2 snapshot sets. No network, no secrets:
# a fake restic that serves a snapshot list and records forget and prune.
#
# Failure modes (written before the code; owner 2026-10-31: weekly
# automatic prune, keep the last 2 daily sets, shards included):
#
#  1. A shard outlives or loses its main snapshot: never; sets are kept or
#     forgotten whole by their run tag.
#  2. The run in flight (main plus some shards) or a failed run is forgotten
#     or counted as a kept set: never; it stays and does not count.
#  3. Fewer than 2 complete sets exist and one is forgotten: never.
#  4. A set from before the split (main only, full tree) is forgotten while
#     it is one of the last 2 sets, or counted complete after the split
#     began and is missing shards: never.
#  5. A snapshot without a run tag (a hand-made one) is forgotten: never.
#  6. The prune uses the append-only credentials: never; it reads
#     prune-rest-password and prune-ssh-key.
#  7. A failed forget or prune reads as success: never; status.json says
#     failed with the cause and last_prune_epoch does not move.
#  8. A prune that never ran or is old goes unreported: never; freshness
#     fails once the backup is older than the maximum prune age.
#
# Run: env -u BASH_ENV bash scripts/off-host-backup-prune-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
# The shard mechanism, as it ran on 2026-11-01; the default is no shards now.
export OFFHOST_BIG_DIRS="pin-receipts:4 computer-event:2 computer-event-payload:2"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

# snapshots JSON: sets "run:shards" ...; shards beyond 0 are numbered
fixture() { # out, then run:shards ...
  local out="$1"; shift
  local spec run n i t=0
  {
    for spec in "$@"; do
      run="${spec%%:*}"; n="${spec##*:}"; t=$((t + 1))
      printf '{"id":"main-%s","time":"2026-10-%02dT02:46:00Z","tags":["daily","run-%s"]}\n' "$run" "$t" "$run"
      for (( i = 0; i < n; i++ )); do
        printf '{"id":"sh%s-%s","time":"2026-10-%02dT03:00:00Z","tags":["daily","run-%s","shard-x-%s"]}\n' "$i" "$run" "$t" "$run" "$i"
      done
    done
    printf '{"id":"hand-made","time":"2026-09-01T00:00:00Z","tags":["manual"]}\n'
  } | jq -sc '[{group_key:{}, snapshots: .}]' > "$out"
}
setup() { # name
  R="$WORK/$1"; mkdir -p "$R/bin" "$R/root/platform-artifacts" "$R/sec" "$R/root/offhost-backup"
  ST="$R/root/offhost-backup/status.json"; FIX="$R/snaps.json"; LOG="$R/restic.log"
  printf '#!/bin/sh\ncat >/dev/null\necho "[]"\n' > "$R/bin/curl"
  printf '#!/bin/sh\nexit 0\n' > "$R/bin/ssh"
  cat > "$R/bin/restic" <<EOF
#!/usr/bin/env bash
case " \$* " in
  *" snapshots "*) cat "$FIX" ;;
  *" forget "*) echo "forget \$*" >> "$LOG"; echo "pw=\$RESTIC_REST_PASSWORD" >> "$LOG"; [ -e "$R/forget-fails" ] && { echo boom >&2; exit 1; } ;;
  *" prune "*) echo "prune \$*" >> "$LOG"; [ -e "$R/prune-fails" ] && { echo boom >&2; exit 1; } ;;
esac
exit 0
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf daily-pw > "$R/sec/rest-password"
  printf prune-pw > "$R/sec/prune-rest-password"; printf k > "$R/sec/prune-ssh-key"
}
run() { # subcommand
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_NO_TUNNEL=1 OFFHOST_PRUNE_WAIT_SECONDS=0 bash "$SCRIPT" "$1" 2>&1)"; RC=$?
}
forgotten() { grep '^forget ' "$LOG" 2>/dev/null | sed 's/^forget .*forget //' | tr ' ' '\n' | sort | tr '\n' ' '; }
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC forgot=[$(forgotten)] out=$(printf '%s' "$OUT" | tail -3) status=$(cat "$ST" 2>/dev/null)"; fails=$((fails + 1)); fi; }
status() { jq -r ".$1 // \"\"" "$ST" 2>/dev/null; }

# 1: four complete sets (default shards: 4 + 2 + 2 = 8): the oldest two go whole.
setup four; fixture "$FIX" r1:8 r2:8 r3:8 r4:8; run prune
want="main-r1 main-r2 $(for i in 0 1 2 3 4 5 6 7; do printf 'sh%s-r1 sh%s-r2 ' $i $i; done)"
want="$(printf '%s' "$want" | tr ' ' '\n' | sort | tr '\n' ' ')"
check "oldest two sets forgotten whole, shards with their main" "$([ "$(forgotten)" = "$want" ]; echo $?)"
check "then prune --max-unused 5%" "$(grep -q '^prune .*--max-unused 5%' "$LOG"; echo $?)"
check "forget and prune ran with the prune password" "$([ "$(grep -c '^pw=prune-pw$' "$LOG")" = 1 ]; echo $?)"
check "status: ok, forgotten count, kept runs" "$([ "$(status last_prune_result)" = ok ] && [ "$(status last_prune_forgotten)" = 18 ] && [ "$(status last_prune_kept_runs)" = '["run-r3","run-r4"]' ] || [ "$(jq -c .last_prune_kept_runs "$ST")" = '["run-r3","run-r4"]' ]; echo $?)"
check "hand-made snapshot untouched" "$(forgotten | grep -q hand-made; [ $? = 1 ]; echo $?)"

# 2: an incomplete newest set stays and does not count.
setup flight; fixture "$FIX" r1:8 r2:8 r3:8 r4:3; run prune
check "in-flight set kept, 2 complete sets kept, r1 forgotten" "$([ "$(forgotten | tr ' ' '\n' | grep -c 'r1')" = 9 ] && ! forgotten | grep -q -E 'r[234]'; echo $?)"

# 3: fewer than 2 complete sets: nothing forgotten (a lone complete set + partial).
setup one; fixture "$FIX" r1:8 r2:2; run prune
check "one complete set and a partial: nothing forgotten" "$([ -z "$(forgotten)" ]; echo $?)"
check "prune still runs" "$(grep -q '^prune ' "$LOG"; echo $?)"
setup none; fixture "$FIX" r1:3 r2:1; run prune
check "no complete set: nothing forgotten" "$([ -z "$(forgotten)" ]; echo $?)"

# 4: pre-split full snapshots.
setup legacy1; fixture "$FIX" r0:0 r1:8; run prune
check "legacy set plus one split set are the last 2: nothing forgotten" "$([ -z "$(forgotten)" ]; echo $?)"
setup legacy2; fixture "$FIX" r0:0 r1:8 r2:8; run prune
check "legacy set older than 2 split sets is forgotten" "$([ "$(forgotten)" = "main-r0 " ]; echo $?)"
setup legacy3; fixture "$FIX" r0:0 r1:8 r2:8 r3:2; run prune
check "a split-era main with missing shards is not a legacy set" "$([ "$(forgotten)" = "main-r0 " ]; echo $?)"

# 7: a failing forget or prune.
setup fail; fixture "$FIX" r1:8 r2:8 r3:8; : > "$R/forget-fails"; run prune
check "failed forget: nonzero, status failed, no prune" "$([ "$RC" != 0 ] && [ "$(status last_prune_result)" = failed ] && [ -z "$(status last_prune_epoch)" ] && ! grep -q '^prune ' "$LOG"; echo $?)"
setup fail2; fixture "$FIX" r1:8 r2:8 r3:8; : > "$R/prune-fails"; run prune
check "failed prune: nonzero, status failed" "$([ "$RC" != 0 ] && [ "$(status last_prune_result)" = failed ] && [ -z "$(status last_prune_epoch)" ]; echo $?)"

# 8: freshness reports prune age.
setup fresh
t="$(date -u +%s)"
printf '{"last_attempt_result":"ok","last_success_epoch":%s,"first_success_epoch":%s,"last_verify_result":"ok","last_verify_epoch":%s,"prune_since_epoch":%s}\n' "$((t - 3600))" "$((t - 86400 * 20))" "$((t - 3600))" "$((t - 86400 * 12))" > "$ST"
run freshness
check "no prune in 20 days is stale" "$([ "$RC" != 0 ] && printf '%s' "$OUT" | grep -q 'no successful prune'; echo $?)"
printf '{"last_attempt_result":"ok","last_success_epoch":%s,"first_success_epoch":%s,"last_verify_result":"ok","last_verify_epoch":%s,"last_prune_epoch":%s,"last_prune_result":"ok"}\n' "$((t - 3600))" "$((t - 86400 * 20))" "$((t - 3600))" "$((t - 86400))" > "$ST"
run freshness
check "a prune a day ago is fresh" "$([ "$RC" = 0 ]; echo $?)"

# prune-keygen refuses to overwrite.
setup kg
check "prune-keygen refuses to overwrite existing keys" "$(run prune-keygen; [ "$RC" = 2 ] || [ "$RC" = 5 ]; echo $?)"

[ "$fails" = 0 ] && echo "all prune checks passed" || { echo "$fails failed"; exit 1; }
