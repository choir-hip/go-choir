#!/usr/bin/env bash
# The flat tape directories are backed up in shards (one restic process
# each) so no process holds all 11M files' trees. No network, no restic:
# `off-host-backup.sh plan` on a fixture, and the shard patterns matched
# with bash globs ([^..] read as [!..]).
#
# Failure modes (written before the shards; memory record: restic peaked
# at 10 GB over one 11M-file walk, 2026-10-30 and -31):
#
#  1. A file in a big directory falls in no shard (silent loss from the
#     backup): never; every name, hex or not, is in exactly one shard.
#  2. A name is in two shards (double upload, double memory): never.
#  3. The main run still walks a big directory: never; plan lists each
#     under excluded.
#  4. A planned shard's path is not its directory: never.
#
# Run: env -u BASH_ENV bash scripts/off-host-backup-shards-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi; }

R="$WORK/root"
mkdir -p "$R/platform-artifacts/sha256" "$R/vm-state" "$R/secrets"
OUT="$(OFFHOST_DATA_ROOT="$R" bash "$SCRIPT" plan 2>&1)"
A="$R/platform-artifacts/sha256"

# 1, 2, 4
names="0a 3f 4b 7z 8c bd c0 ff 9 Z x.json .hidden"
for b in pin-receipts:4 computer-event:2 computer-event-payload:2; do
  d="${b%%:*}"; n="${b##*:}"; bad=0
  for name in $names; do
    hits=0
    for (( i = 0; i < n; i++ )); do
      excluded=0
      while read -r pat; do
        [ -n "$pat" ] || continue
        p="${pat//\[^/[!}"
        # shellcheck disable=SC2254
        case "$A/$d/$name" in $p) excluded=1 ;; esac
      done < <(printf '%s\n' "$OUT" | sed -n "/shard-$d-$i:/,/^  shard-\|^[a-z]/p" | sed -n 's/^    exclude //p')
      [ "$excluded" = 0 ] && hits=$((hits + 1))
    done
    [ "$hits" = 1 ] || { bad=1; echo "     $d/$name in $hits shards"; }
  done
  check "$d: each name in exactly one of $n shards" "$bad"
  check "$d: shard paths planned" "$([ "$(printf '%s\n' "$OUT" | grep -c "shard-$d-[0-9]*: $A/$d\$")" = "$n" ]; echo $?)"
  # 3
  check "$d: excluded from the main run" "$(printf '%s\n' "$OUT" | grep -qx "  $A/$d"; echo $?)"
done

[ "$fails" = 0 ] && echo "all ok" || { echo "$fails failed"; exit 1; }
