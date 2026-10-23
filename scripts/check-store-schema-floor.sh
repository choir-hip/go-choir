#!/usr/bin/env bash
# The store floor on the deploy path (docs/design/choir-event-table-f-c-2026-10-22.md
# 4.5; review finding H on F-c deploy 1).
#
# A guest whose store holds events stamped for choir_events must never run a
# binary below storeschema.Version 2. The guest and the updater refuse such a
# binary when they see a floor (storeschema/floor.go), but a plain git revert
# shipped by CI's deploy-refresh boots the older image's binary directly, and
# that revert also removes every fence inside the reverted code. This check
# lives outside that code: CI fails a push or pull request that LOWERS
# storeschema.Version, unless the head commit carries the trailer
#
#   Store-Floor-Override: <the pre-rollback check, design 4.5, and its result>
#
# which says, in the commit itself, that no computer has a store floor above
# the new epoch (each computer's /api/runtime/observability event_table
# store_floor). Raising or keeping the epoch always passes.
#
#   scripts/check-store-schema-floor.sh [--base REF] [--head REF]
#
# --base defaults to $STORE_FLOOR_BASE, then HEAD~1. An all-zero base (a new
# branch) passes: there is nothing to compare. Any other base that cannot be
# read FAILS CLOSED (bunch 148, Devin 1 and DeepSeek P2): the guard first
# fetches it once from origin (depth 1, then a deeper retry), and if it is
# still unreadable it exits 1, because a guard that passes when it cannot see
# the base is open exactly when a fetch fails or history was rewritten.
set -euo pipefail

base=${STORE_FLOOR_BASE:-}
head=HEAD
while [ $# -gt 0 ]; do
	case $1 in
	--base) base=$2; shift 2 ;;
	--head) head=$2; shift 2 ;;
	*) echo "usage: $0 [--base REF] [--head REF]" >&2; exit 2 ;;
	esac
done
[ -n "$base" ] || base="$head~1"

# have_commit REV: REV names a commit this checkout can read.
have_commit() { git cat-file -e "$1^{commit}" 2>/dev/null; }

# ensure_base REV: fetch REV from origin once if absent, then once deeper.
ensure_base() {
	have_commit "$1" && return 0
	git remote get-url origin >/dev/null 2>&1 || return 1
	git fetch --no-tags --depth=1 origin "$1" >/dev/null 2>&1 || true
	have_commit "$1" && return 0
	git fetch --no-tags --depth=200 origin "$1" >/dev/null 2>&1 || true
	have_commit "$1"
}

epoch_at() {
	{ git show "$1:internal/storeschema/schema.go" 2>/dev/null || true; } |
		sed -n 's/^const Version uint64 = \([0-9][0-9]*\).*/\1/p' | head -n 1
}

if [ -z "$(printf '%s' "$base" | tr -d '0')" ]; then
	echo "store-floor: no base commit (new branch); nothing to compare"
	exit 0
fi
head_epoch=$(epoch_at "$head")
if [ -z "$head_epoch" ]; then
	echo "store-floor: FAIL: cannot read storeschema.Version at $head" >&2
	exit 1
fi
if ! ensure_base "$base"; then
	echo "store-floor: FAIL: base $base cannot be read here after a fetch retry (shallow checkout, rewritten history or a fetch failure); head epoch $head_epoch; the guard cannot prove no decrease" >&2
	exit 1
fi
base_epoch=$(epoch_at "$base")
if [ -z "$base_epoch" ]; then
	echo "store-floor: FAIL: storeschema.Version unreadable at base $base (head epoch $head_epoch); the guard cannot prove no decrease" >&2
	exit 1
fi
if [ "$head_epoch" -ge "$base_epoch" ]; then
	echo "store-floor: PASS: storeschema.Version $base_epoch -> $head_epoch"
	exit 0
fi
override=$(git log -1 --format=%B "$head" | sed -n 's/^Store-Floor-Override:[[:space:]]*\(..*\)$/\1/p' | head -n 1)
if [ -n "$override" ]; then
	echo "store-floor: PASS (override): storeschema.Version $base_epoch -> $head_epoch; Store-Floor-Override: $override"
	exit 0
fi
cat >&2 <<MSG
store-floor: FAIL: storeschema.Version falls $base_epoch -> $head_epoch.
A computer whose store has a floor above $head_epoch (events stamped for
choir_events) would boot a binary that hides them and can hand out their
numbers again (design 4.5). The supported rollback is deploy 1 (epoch 2).
If every computer was checked first (design 4.5's pre-rollback check; each
computer's /api/runtime/observability event_table.store_floor), say so in
the head commit:
  Store-Floor-Override: <what was checked, and the result>
MSG
exit 1
