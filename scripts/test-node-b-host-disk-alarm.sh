#!/usr/bin/env bash
# Behavioural test for scripts/node-b-host-disk-alarm. It drives the real
# script with fake df and btrfs, so it needs no btrfs and no root.
#
#   bash scripts/test-node-b-host-disk-alarm.sh
#
# Failure modes (docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md),
# written before the script:
#   A1  free under the deploy floor (60 GiB) is not a warning
#   A2  btrfs unallocated under 20 GiB is not a warning
#   A3  free under nix min-free (24 GiB) or unallocated under 10 GiB is not
#       an error-priority line
#   A4  an unreadable df or btrfs reading is silence instead of an error line
#   A5  a healthy host writes nothing, so a missing line cannot be told from
#       a timer that never ran
#   A6  the unit fails on a low disk (a failed oneshot is not the alarm; the
#       journal priority is) or the script changes anything
#   A7  a value exactly at a threshold alarms (S-F5: -lt vs -le)
#   A8  an unparsable metadata line is an all-clear (DS-4)
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/node-b-host-disk-alarm"
fail=0
pass=0

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin"
GiB=1073741824

cat >"$root/bin/df" <<'EOF'
#!/usr/bin/env bash
[ -n "${FAKE_DF_FAIL:-}" ] && exit 1
printf 'Avail\n%s\n' "$FAKE_AVAIL_BYTES"
EOF
cat >"$root/bin/btrfs" <<'EOF'
#!/usr/bin/env bash
[ -n "${FAKE_BTRFS_FAIL:-}" ] && { echo "ERROR: not a btrfs filesystem" >&2; exit 1; }
cat <<OUT
Overall:
    Device size:		 510027366400
    Device allocated:		 469225570304
    Device unallocated:		 ${FAKE_UNALLOC_BYTES}
    Device missing:		            0
    Used:			 360000000000
Data,single: Size:430000000000, Used:340000000000 (79.07%)
$( [ -z "${FAKE_NO_META:-}" ] && echo "Metadata,DUP: Size:19993198592, Used:${FAKE_META_USED:-16794286817} (84.00%)" )
System,DUP: Size:33554432, Used:65536 (0.20%)
OUT
EOF
chmod +x "$root/bin/df" "$root/bin/btrfs"

run_alarm() { # avail-GiB unalloc-GiB [env...]
  local avail="$1" unalloc="$2"; shift 2
  env PATH="$root/bin:$PATH" FAKE_AVAIL_BYTES=$((avail * GiB)) FAKE_UNALLOC_BYTES=$((unalloc * GiB)) \
    "$@" bash "$script" >"$root/out" 2>&1
  echo $? >"$root/rc"
}

check() {
  local name="$1" cond="$2"
  if eval "$cond"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    echo "FAIL: $name"; echo "  condition: $cond"; sed 's/^/    /' "$root/out"
  fi
}

run_alarm 139 38
check "A5 healthy: one info line with the numbers" '[ "$(wc -l <"$root/out")" = 1 ] && grep -q "^<6>host disk ok: free 139 GiB, btrfs unallocated 38 GiB, metadata 84% of 18.6 GiB" "$root/out"'
check "A6 healthy exits 0" '[ "$(cat "$root/rc")" = 0 ]'

run_alarm 60 20
check "A7 free 60 and unallocated 20 exactly: ok" 'grep -q "^<6>host disk ok: free 60 GiB, btrfs unallocated 20 GiB" "$root/out"'
run_alarm 24 10
check "A7 free 24 and unallocated 10 exactly: warning, not error" 'grep -q "^<4>host disk WARNING: " "$root/out" && ! grep -q "^<3>" "$root/out"'

run_alarm 100 38 FAKE_NO_META=1
check "A8 no metadata line: warning" 'grep -q "^<4>host disk WARNING: cannot read btrfs metadata" "$root/out"  && grep -q "metadata unknown$" "$root/out" && [ "$(cat "$root/rc")" = 0 ]'

run_alarm 59 38
check "A1 free 59 < 60: warning priority" 'grep -q "^<4>host disk WARNING: free 59 GiB below 60 GiB" "$root/out"'
check "A6 warning exits 0" '[ "$(cat "$root/rc")" = 0 ]'

run_alarm 100 19
check "A2 unallocated 19 < 20: warning" 'grep -q "^<4>host disk WARNING: .*btrfs unallocated 19 GiB below 20 GiB" "$root/out"'

run_alarm 23 38
check "A3 free 23 < 24: error priority" 'grep -q "^<3>host disk ERROR: free 23 GiB below 24 GiB" "$root/out"'
check "A6 error exits 0" '[ "$(cat "$root/rc")" = 0 ]'
run_alarm 60 9
check "A3 unallocated 9 < 10: error priority" 'grep -q "^<3>host disk ERROR: .*btrfs unallocated 9 GiB below 10 GiB" "$root/out"'
run_alarm 30 15
check "A1+A2 both named in one warning line" '[ "$(grep -c "^<4>" "$root/out")" = 1 ] && grep -q "free 30 GiB below 60 GiB" "$root/out" && grep -q "unallocated 15 GiB below 20 GiB" "$root/out"'

run_alarm 100 38 HOST_DISK_WARN_FREE_GIB=120
check "thresholds come from the environment" 'grep -q "^<4>host disk WARNING: free 100 GiB below 120 GiB" "$root/out"'

run_alarm 100 38 FAKE_BTRFS_FAIL=1
check "A4 unreadable btrfs: error line" 'grep -q "^<3>host disk ERROR: cannot read btrfs" "$root/out"'
check "A4 exits 0" '[ "$(cat "$root/rc")" = 0 ]'
run_alarm 100 38 FAKE_DF_FAIL=1
check "A4 unreadable df: error line" 'grep -q "^<3>host disk ERROR: cannot read free space" "$root/out"'
check "A6 unreadable df exits 0" '[ "$(cat "$root/rc")" = 0 ]'

echo "node-b-host-disk-alarm: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
