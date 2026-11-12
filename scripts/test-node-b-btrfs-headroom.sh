#!/usr/bin/env bash
# Behavioural test for scripts/node-b-btrfs-headroom with a fake btrfs that
# prints canned `btrfs filesystem usage -b` output and logs balance calls.
#
#   bash scripts/test-node-b-btrfs-headroom.sh
#
# Failure list, written before the script:
#   H1  metadata Size over 3x Used does not run the metadata balance
#   H2  metadata Size at or under 3x Used runs it (exactly 3x: no)
#   H3  low unallocated no longer runs the data balance, or healthy runs it
#   H4  the metadata balance runs without -musage=40,limit=10
#   H5  a failed data balance skips the metadata balance
#   H6  an unparsable metadata line balances blindly (must skip and exit 1)
#   H7  the nix unit does not run this script
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/node-b-btrfs-headroom"
fail=0; pass=0
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin"
GiB=1073741824
cat >"$root/bin/btrfs" <<'EOF2'
#!/usr/bin/env bash
if [ "$1 $2" = "filesystem usage" ]; then
cat <<OUT
Overall:
    Device size:		 510027366400
    Device unallocated:		 ${FAKE_UNALLOC}
Data,single: Size:430000000000, Used:340000000000 (79.07%)
${FAKE_META_LINE}
System,DUP: Size:33554432, Used:65536 (0.20%)
OUT
elif [ "$1 $2" = "balance start" ]; then
  echo "balance $*" >>"$FAKE_LOG"
  [ -n "${FAKE_DATA_FAIL:-}" ] && [[ "$3" == -d* ]] && exit 1
  exit 0
fi
EOF2
chmod +x "$root/bin/btrfs"

run() { # unalloc-GiB meta-size-GiB meta-used-GiB [env...]
  local ua="$1" ms="$2" mu="$3"; shift 3
  : >"$root/log"
  env PATH="$root/bin:$PATH" FAKE_LOG="$root/log" FAKE_UNALLOC=$((ua * GiB)) \
    FAKE_META_LINE="Metadata,DUP: Size:$((ms * GiB)), Used:$((mu * GiB)) (1.00%)" "$@" bash "$script" >"$root/out" 2>&1
  echo $? >"$root/rc"
}
check() { if eval "$2"; then pass=$((pass+1)); else fail=$((fail+1)); echo "FAIL: $1"; sed 's/^/    /' "$root/out" "$root/log"; fi; }

run 40 30 9
check "H1 size 30 > 3x used 9: metadata balance" 'grep -q "balance start -musage=40,limit=10 /" "$root/log"'
check "H4 exact arguments" '[ "$(grep -c musage=40,limit=10 "$root/log")" = 1 ]'
check "H3 unallocated 40 GiB: no data balance" '! grep -q -- "-dusage" "$root/log"'
run 40 27 9
check "H2 size exactly 3x used: no metadata balance" '! grep -q musage "$root/log" && [ "$(cat "$root/rc")" = 0 ]'
run 40 20 18
check "H2 healthy metadata: no balance" '[ ! -s "$root/log" ]'
run 29 20 18
check "H3 unallocated 29 GiB: data balance" 'grep -q "balance start -dusage=20,limit=20 /" "$root/log" && ! grep -q musage "$root/log"'
run 30 20 18
check "H3 unallocated exactly 30 GiB: no data balance" '[ ! -s "$root/log" ]'
run 10 30 5
check "H1 both run, data first" '[ "$(sed -n 1p "$root/log" | cut -d" " -f4)" = "-dusage=20,limit=20" ] && grep -q musage "$root/log"'
run 10 30 5 FAKE_DATA_FAIL=1
check "H5 failed data balance still tries metadata, exits 1" 'grep -q musage "$root/log" && [ "$(cat "$root/rc")" = 1 ]'
run 40 30 9 FAKE_META_LINE=
check "H6 no metadata line: skip, exit 1" '! grep -q balance "$root/log" && [ "$(cat "$root/rc")" = 1 ]'
unit="$(awk '/systemd.services.go-choir-btrfs-headroom = \{/,/^  \};/' "$here/../nix/node-b.nix")"
check "H7 unit runs the script" 'grep -q "scripts/node-b-btrfs-headroom" <<<"$unit"'

echo "node-b-btrfs-headroom: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
