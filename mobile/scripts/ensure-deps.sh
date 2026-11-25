#!/bin/sh
# Make sure the static xcframeworks the on-device models need (FluidAudio, llama.cpp) are in mobile/.deps/xcframeworks.
# A build without them has NO model stages: the app then shows only fallbacks ("the voice models don't load, only the
# fallbacks", owner, 2026-11-11). So the app build calls this first and fails if it cannot provide them.
#
# Order: already there; else copy from the per-machine cache ($AUTORADIO_DEPS_CACHE, default ~/.cache/autoradio-deps);
# else build them (build-xcframeworks.sh: about 10 minutes per FluidAudio slice, and llama.cpp's own script) and fill the
# cache so no other worktree builds them again.
#
# `ensure-deps.sh [ios|macos]` (default ios): the platform whose slice must be in each xcframework. An xcframework that has
# only the iOS slices is NOT present for macos (Exact: "has no macos arm64 slice in its Info.plist").
set -eu
PLATFORM="${1:-ios}"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
# The caller already holds the Exact build lock. Stage Tiny Voca's precompiled assets
# (models, phonemizer data, Dizione XCFramework) before the larger libraries.
"$HERE/scripts/prepare-tiny-voca.sh" --stage "$PLATFORM" "$HERE/AutoradioLab"
OUT="$HERE/.deps/xcframeworks"
CACHE="${AUTORADIO_DEPS_CACHE:-$HOME/.cache/autoradio-deps}/xcframeworks"
# Whether xcframework $1 has a slice for $PLATFORM (the LibraryIdentifier starts with the platform: ios-arm64, macos-arm64).
has_slice() {
  [ -f "$1/Info.plist" ] || return 1
  /usr/bin/plutil -extract AvailableLibraries json -o - "$1/Info.plist" 2>/dev/null | /usr/bin/python3 -I -c '
import json, sys
want = sys.argv[1]
libs = json.load(sys.stdin)
sys.exit(0 if any(l.get("SupportedPlatform") == want and l.get("SupportedPlatformVariant") is None for l in libs) else 1)' "$([ "$PLATFORM" = macos ] && echo macos || echo ios)"
}
have() { has_slice "$1/FluidAudio.xcframework" && has_slice "$1/llama.xcframework"; }

if have "$OUT"; then echo "deps: xcframeworks present ($PLATFORM slice)"; exit 0; fi
mkdir -p "$OUT"
if have "$CACHE"; then
  for fw in FluidAudio llama; do rm -rf "$OUT/$fw.xcframework"; cp -R "$CACHE/$fw.xcframework" "$OUT/"; done
  echo "deps: copied xcframeworks from $CACHE"
  exit 0
fi
echo "deps: building the xcframeworks (this takes a while, once per machine: about 10 minutes per FluidAudio slice, three slices, and llama.cpp's builds)"
has_slice "$OUT/FluidAudio.xcframework" || "$HERE/scripts/build-xcframeworks.sh" fluidaudio
has_slice "$OUT/llama.xcframework" || "$HERE/scripts/build-llama-xcframework.sh"
have "$OUT" || { echo "deps: the xcframeworks are still missing after the build" >&2; exit 1; }
mkdir -p "$CACHE"
for fw in FluidAudio llama; do rm -rf "$CACHE/$fw.xcframework"; cp -R "$OUT/$fw.xcframework" "$CACHE/"; done
echo "deps: cached in $CACHE"
