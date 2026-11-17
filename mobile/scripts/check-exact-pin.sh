#!/bin/sh
# Refuse to go on unless mobile/.exact-src is exactly the pinned Exact commit (mobile/exact.pin) with every local patch applied.
# A checkout left at an older pin once built "successfully" against the wrong Exact; every build entry (build-lab.sh, exact.sh) calls this.
HERE="$(cd "$(dirname "$0")/.." && pwd)"
SRC="${EXACT2:-$HERE/.exact-src}"
PIN="$(tr -d ' \n' < "$HERE/exact.pin" 2>/dev/null)"
HEAD="$(git -C "$SRC" rev-parse HEAD 2>/dev/null)"
bad() { echo "exact pin check FAILED: $1" >&2; echo "  pin (mobile/exact.pin): $PIN" >&2; echo "  .exact-src HEAD:        ${HEAD:-none}" >&2; echo "  run: sh $HERE/scripts/setup-exact.sh" >&2; exit 1; }
[ -n "$PIN" ] || bad "mobile/exact.pin is missing or empty"
[ "$HEAD" = "$PIN" ] || bad "the Exact checkout is not at the pin"
for P in "$HERE"/exact-patches/*.patch; do
  [ -f "$P" ] || continue
  git -C "$SRC" apply --check -R "$P" 2>/dev/null || bad "patch not applied: $(basename "$P")"
done
exit 0
