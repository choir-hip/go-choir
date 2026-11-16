#!/bin/sh
# Check out the pinned Exact source at mobile/.exact-src (gitignored) and install Bun 1.4.2 at ~/.bun-1.4.2.
# EXACT_REPO overrides the clone source (a local checkout is fine). No full `exact.mjs setup`.
set -eu
PIN=76cd50f9e863129b3fc75db1673d6ad4bbf79258
HERE="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$HERE/.exact-src"
REPO="${EXACT_REPO:-https://github.com/expo/exact.git}"
if [ ! -d "$SRC/.git" ]; then git clone -q "$REPO" "$SRC"; fi
if [ "$(git -C "$SRC" rev-parse HEAD)" != "$PIN" ]; then
  git -C "$SRC" fetch -q origin 2>/dev/null || true
  git -C "$SRC" checkout -q "$PIN"
fi
# Local patches (mobile/exact-patches/README.md): applied in order; an applied one is skipped; one that does not apply stops here.
for P in "$HERE"/exact-patches/*.patch; do
  [ -f "$P" ] || continue
  if git -C "$SRC" apply --check "$P" 2>/dev/null; then git -C "$SRC" apply "$P"; echo "patched: $(basename "$P")"
  elif git -C "$SRC" apply --check -R "$P" 2>/dev/null; then :
  else echo "patch does not apply to the pin: $P" >&2; exit 1; fi
done
BUNDIR="$HOME/.bun-1.4.2"
if [ ! -x "$BUNDIR/bin/bun" ]; then
  mkdir -p "$BUNDIR"
  BUN_INSTALL="$BUNDIR" curl -fsSL https://bun.sh/install | BUN_INSTALL="$BUNDIR" bash -s "bun-v1.4.2"
fi
echo "exact $(git -C "$SRC" rev-parse --short HEAD), bun $("$BUNDIR/bin/bun" --version)"
