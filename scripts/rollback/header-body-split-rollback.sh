#!/usr/bin/env bash
# Rollback below the store header/body split
# (docs/design/store-header-body-split-2026-11-01.md section 6, ".presplit
# lifecycle"). Runs INSIDE the guest (or on its mounted persistent volume)
# while the runtime is stopped. It puts the pre-split file back as state.db
# and rewrites the store floor to 5, so the prior release (epoch 5) opens it;
# that binary boots at the presplit head and replays the tape tail (the tape
# format did not change). Floors never lower in code: this recipe is the only
# way down, and it is manual on purpose.
#
# Usage:
#   header-body-split-rollback.sh check  STATE_DIR   # read-only: what would happen
#   header-body-split-rollback.sh apply  STATE_DIR   # do it (runtime stopped)
#
# STATE_DIR is the directory holding state.db (and the store's "<base>.texture"
# sidecar directory with store-floor.json). After apply, install the prior
# release (the updater's restorePrior is fenced by the floor until this
# script lowered it) and start the runtime.
#
# Refuses when: state.db.presplit is missing (finalized or auto-dropped: the
# rollback is then the pre-migration snapshot, by the restore runbook), the
# runtime still has state.db open, or more than one sidecar directory holds a
# floor. The split file is kept as state.db.split-rolled-back, never deleted.
set -euo pipefail

mode=${1:-}
dir=${2:-}
[ -n "$mode" ] && [ -n "$dir" ] || { echo "usage: $0 check|apply STATE_DIR" >&2; exit 2; }
state="$dir/state.db"
pre="$state.presplit"
aside="$state.split-rolled-back"

fail() { echo "header-body-split-rollback: FAIL: $*" >&2; exit 1; }

[ -f "$pre" ] || fail "$pre is missing (finalized or auto-dropped); roll back from the pre-migration snapshot instead"
[ -f "$state" ] || fail "$state is missing; a boot of the split binary finishes or undoes an interrupted swap first"
if command -v fuser >/dev/null 2>&1 && [ -n "$(fuser "$state" 2>/dev/null | tr -d ' ')" ]; then
	fail "$state is open by a running process; stop the runtime first"
fi
floors=()
while IFS= read -r f; do floors+=("$f"); done < <(find "$dir" -maxdepth 2 -name store-floor.json -path '*.texture/*' 2>/dev/null)
[ "${#floors[@]}" -le 1 ] || fail "more than one store floor under $dir: ${floors[*]}"

echo "header-body-split-rollback: state=$state ($(stat -c %s "$state" 2>/dev/null || stat -f %z "$state") bytes)"
echo "header-body-split-rollback: presplit=$pre ($(stat -c %s "$pre" 2>/dev/null || stat -f %z "$pre") bytes)"
echo "header-body-split-rollback: floor=${floors[0]:-none}"
[ "$mode" = check ] && { echo "header-body-split-rollback: check ok (nothing changed)"; exit 0; }
[ "$mode" = apply ] || fail "unknown mode $mode"

# 1. The split file aside (with its WAL), the presplit file (with its WAL) in.
for s in "" -wal -shm; do
	[ -e "$state$s" ] && mv -f "$state$s" "$aside$s"
done
for s in "" -wal -shm; do
	[ -e "$pre$s" ] && mv -f "$pre$s" "$state$s"
done
# The split's layout marker and summary describe the split file.
[ -e "$state.layout.json" ] && mv -f "$state.layout.json" "$aside.layout.json"
[ -e "$state.presplit.json" ] && mv -f "$state.presplit.json" "$aside.presplit.json"
sync
# 2. The floor back to 5 (the epoch of the pre-split layout).
if [ -n "${floors[0]:-}" ]; then
	tmp="${floors[0]%/*}/.store-floor.tmp"
	printf '{"schema":"choir-store-floor-v1","version":5,"reason":"manual rollback below the store header/body split"}' >"$tmp"
	sync "$tmp" 2>/dev/null || sync
	mv -f "$tmp" "${floors[0]}"
	sync
fi
echo "header-body-split-rollback: applied; the split file is kept at $aside. Install the prior release, then start the runtime."
