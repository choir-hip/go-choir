#!/bin/sh
# Stage the app for a build: stage-app.sh <ios|macos|android> <development|store>. build-lab.sh stages `development` before every
# build it runs; archive-store.sh stages `store` itself before every archive, so what Exact reads is what this script just made, never what an earlier build left.
# Android has no Swift app module or xcframeworks: it refreshes only the shared Exact package and JavaScript dependencies, leaving the Apple staging untouched.
# The kind is an argument, not something read from the caller's environment (a stale AR_BUILD_KIND must change nothing): it reaches sync-exact-package.sh as AR_BUILD_KIND.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM="${1:-}"; KIND="${2:-}"
case "$PLATFORM" in ios|macos|android) ;; *) echo "stage-app: the platform is ios, macos or android" >&2; exit 1 ;; esac
case "$KIND" in development|store) ;; *) echo "stage-app: the kind is development or store" >&2; exit 1 ;; esac
# Never build against the wrong Exact: .exact-src must be the commit in exact.pin with the patches applied (prints both shas otherwise).
"$HERE/scripts/check-exact-pin.sh" || exit 1
if [ "$PLATFORM" = android ]; then
  /usr/bin/python3 -I "$HERE/scripts/gen-exact-interface.py" >/dev/null
else
  # The model libraries first: a build without them has no model stages (only fallbacks), which is a defect, not a build.
  [ -n "${ALLOW_NO_MODELS:-}" ] || "$HERE/scripts/ensure-deps.sh" "$PLATFORM"
  AR_BUILD_KIND="$KIND" "$HERE/scripts/sync-exact-package.sh"
  /usr/bin/python3 -I "$HERE/scripts/gen-exact-interface.py" >/dev/null
fi
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
(cd "$HERE/AutoradioLab" && rm -rf node_modules/@choir && "$BUN" install >/dev/null)
