#!/usr/bin/env bash
# Which guest images the off-host backup clones. No network, no secrets:
# a fixture vm-state directory and `off-host-backup.sh plan`.
#
# Failure modes (written before the selection; problem record
# docs/problems/no-off-host-backup-2026-10-20.md section 13 item 10):
#
#  1. With OFFHOST_IMAGE_COMPUTERS set, an image of a computer not named
#     is cloned (a test computer's disk in the nightly run): never.
#  2. A named computer's image is silently missing (no ownership, no
#     data.img): never; plan and backup name it, and backup ends partial.
#  3. A named computer with several VM directories (a fleet candidate and
#     a vm) loses one: never; every image of that computer is selected.
#  4. Unset, the selection changes: never; every image, as before.
#  5. ownerships.json is unreadable while the list is set: never silent;
#     no image is selected and the run names the cause.
#  6. A computer's snapshot chunks leave the run while its image is not in
#     it (its snapshots were the only copy off host): never; only a
#     computer whose image was selected has file-cas-chunks/<c> excluded.
#
# Run: bash scripts/off-host-backup-images-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

fixture() { # root
  local r="$1" v
  for v in vm-paper candidate-owner vm-owner vm-test1 vm-test2; do
    mkdir -p "$r/vm-state/$v"; : > "$r/vm-state/$v/data.img"
  done
  mkdir -p "$r/platform-artifacts" "$r/platform-dolt/.choir-backup/platform" "$r/secrets"
  cat > "$r/vm-state/ownerships.json" <<'EOF'
{"ownerships":[
 {"vm_id":"vm-paper","computer_id":"computer-paper"},
 {"vm_id":"candidate-owner","computer_id":"computer-owner"},
 {"vm_id":"vm-owner","computer_id":"computer-owner"},
 {"vm_id":"vm-test1","computer_id":"computer-test1"},
 {"vm_id":"vm-test2","computer_id":"computer-test2"}
]}
EOF
}
plan() { # name, computers ("-" = unset); sets OUT RC
  local root="$WORK/$1"; fixture "$root"
  if [ "$2" = - ]; then
    OUT="$(OFFHOST_DATA_ROOT="$root" bash "$SCRIPT" plan 2>&1)"; RC=$?
  else
    OUT="$(OFFHOST_DATA_ROOT="$root" OFFHOST_IMAGE_COMPUTERS="$2" bash "$SCRIPT" plan 2>&1)"; RC=$?
  fi
  ROOT="$root"
}
images() { printf '%s\n' "$OUT" | sed -n '/^guest images:/,/^[a-z]/p' | grep -o 'vm-state/[^/]*' | sort | tr '\n' ' '; }
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC images=$(images) out=$OUT"; fails=$((fails + 1)); fi; }

# 1, 3
plan named "computer-paper,computer-owner"
check "only the named computers' images" "$([ "$(images)" = "vm-state/candidate-owner vm-state/vm-owner vm-state/vm-paper " ]; echo $?)"

# 6
excl() { printf '%s\n' "$OUT" | sed -n '/^excluded:/,/^absent:/p' | grep -o 'file-cas-chunks/[^ ]*' | sort | tr '\n' ' '; }
check "imaged computers' snapshot chunks are excluded" "$([ "$(excl)" = "file-cas-chunks/computer-owner file-cas-chunks/computer-paper " ]; echo $?)"

# 4
plan unset -
check "unset excludes no snapshot chunks" "$([ -z "$(excl)" ]; echo $?)"
check "unset selects every image" "$([ "$(images)" = "vm-state/candidate-owner vm-state/vm-owner vm-state/vm-paper vm-state/vm-test1 vm-state/vm-test2 " ]; echo $?)"

# 2: a named computer with no ownership
plan missing "computer-paper computer-gone"
check "a named computer with no image is reported" "$(printf '%s' "$OUT" | grep -q 'computer-gone'; echo $?)"
check "a named computer with no image fails the plan" "$([ "$RC" = 3 ]; echo $?)"
check "a named computer with no image keeps its snapshot chunks" "$([ "$(excl)" = "file-cas-chunks/computer-paper " ]; echo $?)"

# 2: an ownership whose image file is absent
plan noimg "computer-paper"
rm -f "$ROOT/vm-state/vm-paper/data.img"
OUT="$(OFFHOST_DATA_ROOT="$ROOT" OFFHOST_IMAGE_COMPUTERS="computer-paper" bash "$SCRIPT" plan 2>&1)"; RC=$?
check "a named computer whose data.img is absent is reported" "$(printf '%s' "$OUT" | grep -q 'computer-paper'; [ "$RC" = 3 ]; echo $?)"

# 5
plan unreadable "computer-paper"
printf 'not json' > "$ROOT/vm-state/ownerships.json"
OUT="$(OFFHOST_DATA_ROOT="$ROOT" OFFHOST_IMAGE_COMPUTERS="computer-paper" bash "$SCRIPT" plan 2>&1)"; RC=$?
check "unreadable ownerships selects nothing and says why" "$([ -z "$(images)" ] && printf '%s' "$OUT" | grep -q 'ownerships.json' && [ "$RC" = 3 ]; echo $?)"

[ "$fails" = 0 ] && echo "all image selection checks passed" || { echo "$fails failed"; exit 1; }
