#!/usr/bin/env bash
# Behavioural test for scripts/node-b-deploy-disk-preflight's floor. It runs a
# copy of the real script beside a fake retention script, with fake df,
# curl, journalctl, nix and a fake nix.conf, so it needs no Nix and no root
# and reclaims nothing.
#
#   bash scripts/test-node-b-deploy-disk-preflight.sh
#
# Failure modes (docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md,
# review round 2 S-F2 / DS-1), written before the guard:
#   P1  a lowered floor admits a deploy while the live nix min-free is still
#       the old 80 GiB (both commits in one deploy, a revert of deploy 1
#       alone, or a system rollback): the build starts under min-free and
#       nix's auto-GC deletes its unrooted inputs (2026-09-17)
#   P2  an unreadable nix.conf, or one without a numeric min-free, is read
#       as "no min-free" instead of failing closed to the old 90 GiB floor
#   P3  DEPLOY_MIN_FREE_KIB set by hand lowers the floor under the guard
#   P4  the guard raises the floor when the live band is already the new one
#   P5  the floor in force is not in the log
#   P6  a df that exits 0 with a non-numeric reading, before or after the
#       reclaim, lets the deploy through (both [ tests error, the if is
#       false, exit 0; review round 2b DS2b-1)
#   P7  a non-numeric DEPLOY_MIN_FREE_KIB is not refused or guarded
#   P8  a numeric read too long for bash's 64-bit arithmetic (a 20-digit
#       floor or df reading makes every [ error, so the guard and the
#       refusal are skipped and the script exits 0; a min-free of 2^63
#       wraps negative) or with a leading zero (read as octal) admits the
#       deploy instead of failing closed (review round 3, R3-F1)
#   P9  the guard keeps a fixed 10 GiB over the live min-free instead of the
#       floor's terms (deploy 8 + compaction 14 + regrowth 10 + margin 4 =
#       36 GiB), so for a live min-free from 24 GiB up the overlapping work
#       no longer fits (review round 3, R3-F2)
# The fail-closed floor (an unreadable min-free or floor setting) is the
# old band's: 80 GiB min-free + 36 = 116 GiB.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
fail=0
pass=0
GiB_KIB=1048576

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/scripts"
cp "$here/node-b-deploy-disk-preflight" "$root/scripts/"
cat >"$root/scripts/node-b-nix-retention" <<'EOF'
#!/usr/bin/env bash
echo "fake retention"
EOF
cat >"$root/bin/df" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *--output=avail*)
    # FAKE_AVAIL_SEQ: one value per call ("a b": a before the reclaim, b after)
    if [ -n "${FAKE_AVAIL_SEQ+x}" ]; then
      n=$(cat "$FAKE_DIR/df.calls" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" >"$FAKE_DIR/df.calls"
      printf 'Avail\n%s\n' "$(echo "$FAKE_AVAIL_SEQ" | cut -d' ' -f"$n")"
    else
      printf 'Avail\n%s\n' "$FAKE_AVAIL_KIB"
    fi ;;
  *) echo "fake df $*" ;;
esac
EOF
for t in curl journalctl nix; do printf '#!/usr/bin/env bash\nexit 0\n' >"$root/bin/$t"; done
chmod +x "$root/bin/"* "$root/scripts/"*

run_preflight() { # avail-GiB conf-contents|-  [env...]
  local avail="$1" conf="$2"; shift 2
  rm -f "$root/nix.conf" "$root/df.calls"
  [ "$conf" != "-" ] && printf '%s\n' "$conf" >"$root/nix.conf"
  env PATH="$root/bin:$PATH" FAKE_DIR="$root" FAKE_AVAIL_KIB=$((avail * GiB_KIB)) \
    PREFLIGHT_NIX_CONF="$root/nix.conf" "$@" bash "$root/scripts/node-b-deploy-disk-preflight" >"$root/out" 2>&1
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

old='min-free = 85899345920
max-free = 107374182400'
new='auto-optimise-store = true
min-free = 25769803776
max-free = 42949672960'
f56="DEPLOY_MIN_FREE_KIB=$((56 * GiB_KIB))"
f60="DEPLOY_MIN_FREE_KIB=$((60 * GiB_KIB))"
closed=$((116 * GiB_KIB))

run_preflight 70 "$old" "$f56"
check "P1 live min-free 80, floor 56: 70 GiB free is refused" '[ "$(cat "$root/rc")" = 1 ]'
check "P5 the guarded floor (80 + 36 = 116 GiB) is logged" 'grep -q "live_min_free_kib=83886080 min_required_kib=$closed" "$root/out"'
run_preflight 100 "$old" "$f56"
check "P1 live min-free 80: 100 GiB free is refused (P9)" '[ "$(cat "$root/rc")" = 1 ]'
run_preflight 120 "$old" "$f56"
check "P1 live min-free 80: 120 GiB free passes" '[ "$(cat "$root/rc")" = 0 ]'

run_preflight 60 "$new" "$f56"
check "P4 live min-free 24, hand-set floor 56: raised to 24 + 36 = 60 (P9), 60 GiB free passes" '[ "$(cat "$root/rc")" = 0 ] && grep -q "min_required_kib=$((60 * GiB_KIB))" "$root/out"'
run_preflight 50 "$new" "$f56"
check "P4 live min-free 24, floor 56: 50 GiB free is refused" '[ "$(cat "$root/rc")" = 1 ]'

run_preflight 70 - "$f56"
check "P2 no nix.conf: fails closed to 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read min-free" "$root/out" && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 70 "max-free = 42949672960" "$f56"
check "P2 nix.conf without min-free: fails closed to 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 70 "min-free = lots" "$f56"
check "P2 non-numeric min-free: fails closed to 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "min_required_kib=$closed" "$root/out"'

run_preflight 40 "$old" DEPLOY_MIN_FREE_KIB=$((10 * GiB_KIB))
check "P3 a hand-set 10 GiB floor stays guarded at 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "min_required_kib=$closed" "$root/out"'

# ---- P6 every free-space read fails closed ------------------------------------
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ="garbage"
check "P6 non-numeric free space before the reclaim: refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read free space" "$root/out"'
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ=""
check "P6 empty free space before the reclaim: refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read free space" "$root/out"'
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ="$((10 * GiB_KIB)) garbage"
check "P6 non-numeric free space after the reclaim: refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read free space after the reclaim" "$root/out"'
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ="$((10 * GiB_KIB)) $((70 * GiB_KIB))"
check "P6 a numeric reclaim to 70 GiB passes" '[ "$(cat "$root/rc")" = 0 ]'

# ---- P7 the floor setting itself ------------------------------------------------
run_preflight 70 "$new" DEPLOY_MIN_FREE_KIB=lots
check "P7 non-numeric DEPLOY_MIN_FREE_KIB: 116 GiB and logged" '[ "$(cat "$root/rc")" = 1 ] && grep -q "DEPLOY_MIN_FREE_KIB" "$root/out" && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 120 "$new" DEPLOY_MIN_FREE_KIB=lots
check "P7 non-numeric DEPLOY_MIN_FREE_KIB: 120 GiB passes" '[ "$(cat "$root/rc")" = 0 ]'

# ---- P8 numbers too long for the arithmetic, or octal-looking -----------------
big=99999999999999999999
run_preflight 70 "$new" DEPLOY_MIN_FREE_KIB=$big
check "P8 a 20-digit DEPLOY_MIN_FREE_KIB: 116 GiB, 70 GiB free is refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ="$big"
check "P8 a 20-digit free-space reading before the reclaim: refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read free space" "$root/out"'
run_preflight 0 "$new" "$f60" FAKE_AVAIL_SEQ="$((10 * GiB_KIB)) $big"
check "P8 a 20-digit free-space reading after the reclaim: refused" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read free space after the reclaim" "$root/out"'
run_preflight 70 "min-free = 9223372036854775808" "$f60"
check "P8 min-free 2^63 (wraps negative): fails closed to 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "cannot read min-free" "$root/out" && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 70 "min-free = 99999999999999999999" "$f60"
check "P8 a 20-digit min-free: fails closed to 116 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "min_required_kib=$closed" "$root/out"'
run_preflight 70 "min-free = 042949672960" "$f60"
check "P8 min-free with a leading zero is decimal (40 GiB): floor 76 GiB" '[ "$(cat "$root/rc")" = 1 ] && grep -q "live_min_free_kib=41943040 min_required_kib=$((76 * GiB_KIB))" "$root/out"'
run_preflight 70 "$new" DEPLOY_MIN_FREE_KIB=062914560
check "P8 a floor with a leading zero is decimal (60 GiB): 70 GiB free passes" '[ "$(cat "$root/rc")" = 0 ] && grep -q "min_required_kib=62914560" "$root/out"'

# ---- P9 the guard is the floor's terms over the live min-free ------------------
run_preflight 65 "min-free = 42949672960" "$f60"
check "P9 live min-free 40, floor 60: 65 GiB free is refused (floor 76)" '[ "$(cat "$root/rc")" = 1 ] && grep -q "live_min_free_kib=41943040 min_required_kib=$((76 * GiB_KIB))" "$root/out"'
run_preflight 77 "min-free = 42949672960" "$f60"
check "P9 live min-free 40: 77 GiB free passes" '[ "$(cat "$root/rc")" = 0 ]'
run_preflight 61 "$new" "$f60"
check "P9 live min-free 24: the floor stays 60 (24 + 36)" '[ "$(cat "$root/rc")" = 0 ] && grep -q "live_min_free_kib=25165824 min_required_kib=62914560" "$root/out"'
run_preflight 59 "$new" "$f60"
check "P9 live min-free 24: 59 GiB free is refused" '[ "$(cat "$root/rc")" = 1 ]'

echo "node-b-deploy-disk-preflight: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
