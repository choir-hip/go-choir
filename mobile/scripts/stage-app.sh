#!/bin/sh
# Stage the app's Swift for a build, and refresh the app's copy of the kit: stage-app.sh <ios|macos> <development|store>. build-lab.sh stages `development` before every
# build it runs; archive-store.sh stages `store` itself before every archive, so what Exact reads is what this script just made, never what an earlier build left.
# The kind is an argument, not something read from the caller's environment (a stale AR_BUILD_KIND must change nothing): it reaches sync-exact-package.sh as AR_BUILD_KIND.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM="${1:-}"; KIND="${2:-}"
case "$PLATFORM" in ios|macos) ;; *) echo "stage-app: the platform is ios or macos" >&2; exit 1 ;; esac
case "$KIND" in development|store) ;; *) echo "stage-app: the kind is development or store" >&2; exit 1 ;; esac
# Never build against the wrong Exact: .exact-src must be the commit in exact.pin with the patches applied (prints both shas otherwise).
"$HERE/scripts/check-exact-pin.sh" || exit 1
# The model libraries first: a build without them has no model stages (only fallbacks), which is a defect, not a build.
[ -n "${ALLOW_NO_MODELS:-}" ] || "$HERE/scripts/ensure-deps.sh" "$PLATFORM"
AR_BUILD_KIND="$KIND" "$HERE/scripts/sync-exact-package.sh"
python3 -I "$HERE/scripts/gen-exact-interface.py" >/dev/null
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
(cd "$HERE/AutoradioLab" && rm -rf node_modules/@choir && "$BUN" install >/dev/null)
