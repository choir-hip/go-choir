#!/bin/sh
# Make sure the static xcframeworks the on-device models need (FluidAudio, llama.cpp) are in mobile/.deps/xcframeworks.
# A build without them has NO model stages: the app then shows only fallbacks ("the voice models don't load, only the
# fallbacks", owner, 2026-11-11). So the app build calls this first and fails if it cannot provide them.
#
# Order: already there; else copy from the per-machine cache ($AUTORADIO_DEPS_CACHE, default ~/.cache/autoradio-deps);
# else build them (build-xcframeworks.sh: about 10 minutes per FluidAudio slice, and llama.cpp's own script) and fill the
# cache so no other worktree builds them again.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$HERE/.deps/xcframeworks"
CACHE="${AUTORADIO_DEPS_CACHE:-$HOME/.cache/autoradio-deps}/xcframeworks"
have() { [ -d "$1/FluidAudio.xcframework" ] && [ -d "$1/llama.xcframework" ]; }

if have "$OUT"; then echo "deps: xcframeworks present"; exit 0; fi
mkdir -p "$OUT"
if have "$CACHE"; then
  for fw in FluidAudio llama; do rm -rf "$OUT/$fw.xcframework"; cp -R "$CACHE/$fw.xcframework" "$OUT/"; done
  echo "deps: copied xcframeworks from $CACHE"
  exit 0
fi
echo "deps: building the xcframeworks (this takes a while, once per machine)"
[ -d "$OUT/FluidAudio.xcframework" ] || "$HERE/scripts/build-xcframeworks.sh" fluidaudio
[ -d "$OUT/llama.xcframework" ] || "$HERE/scripts/build-llama-xcframework.sh"
have "$OUT" || { echo "deps: the xcframeworks are still missing after the build" >&2; exit 1; }
mkdir -p "$CACHE"
for fw in FluidAudio llama; do rm -rf "$CACHE/$fw.xcframework"; cp -R "$OUT/$fw.xcframework" "$CACHE/"; done
echo "deps: cached in $CACHE"
