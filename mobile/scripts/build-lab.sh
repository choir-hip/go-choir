#!/bin/sh
# Sync the kit into the Exact package, refresh the app's copy of it (bun copies file: dependencies), then
# run an Exact command in the app:  build-lab.sh ios   |   build-lab.sh ios --device --phone <udid>
#                                   build-lab.sh mac --run     (the native AppKit app on this Mac; ALLOW_NO_MODELS=1 until the
#                                                               macOS slices of the model libraries exist: scripts/ensure-deps.sh macos)
# The ios path takes no build lock: its caller holds ~/.cache/autoradio-mac-build.lock. The mac path takes it itself, for the
# build only (a `--run` app that stays open must not hold it), unless AUTORADIO_MAC_LOCK_HELD is set by a caller that holds it.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM=ios
case "${1:-}" in mac|macos) PLATFORM=macos ;; esac
if [ "$PLATFORM" = macos ] && [ -z "${AUTORADIO_MAC_LOCK_HELD:-}" ]; then
  LOCK="${AUTORADIO_MAC_BUILD_LOCK:-$HOME/.cache/autoradio-mac-build.lock}"
  mkdir -p "$(dirname "$LOCK")"
  VERB="$1"; shift
  RUN=""; REST=""
  for a in "$@"; do if [ "$a" = "--run" ]; then RUN=1; else REST="$REST $a"; fi; done
  # shellcheck disable=SC2086
  AUTORADIO_MAC_LOCK_HELD=1 /usr/bin/lockf -k -t 7200 "$LOCK" "$0" "$VERB" $REST
  [ -n "$RUN" ] || exit 0
  # The build is current: launching it needs no lock.
  AUTORADIO_MAC_LOCK_HELD=1 exec "$0" "$VERB" $REST --run
fi
# The model libraries first: a build without them has no model stages (only fallbacks), which is a defect, not a build.
[ -n "${ALLOW_NO_MODELS:-}" ] || "$HERE/scripts/ensure-deps.sh" "$PLATFORM"
"$HERE/scripts/sync-exact-package.sh"
python3 -I "$HERE/scripts/gen-exact-interface.py" >/dev/null
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
(cd "$HERE/AutoradioLab" && rm -rf node_modules/@choir && "$BUN" install >/dev/null)
exec "$HERE/scripts/exact.sh" "$@"
