#!/bin/sh
# Sync the kit into the Exact package, refresh the app's copy of it (bun copies file: dependencies), then
# run an Exact command in the app:  build-lab.sh ios   |   build-lab.sh ios --device --phone <udid>
#                                   build-lab.sh mac --run     (the native AppKit app on this Mac; ALLOW_NO_MODELS=1 until the
#                                                               macOS slices of the model libraries exist: scripts/ensure-deps.sh macos)
# No choice builds the production app's id ("Autoradio 1"). AR_VARIANT=lab or lab-one (or AR_APP_ID=<its id>, app-identity.sh) builds a development app under its own
# bundle id: Exact's own variant, so the tracked app.json is not changed (nothing to restore). The identity is checked here, before the sync, and again by exact.sh.
# A build for a phone (--device) must name its development app: the production id reaches phones through the store (archive-store.sh), never as a development install.
# AR_BUILD_KIND=store is the store archive's (archive-store.sh): it stages the Swift without the developer launch hooks (sync-exact-package.sh) and goes with --archive only.
# The ios path takes no build lock: its caller holds ~/.cache/autoradio-mac-build.lock. The mac path takes it itself, for the
# build only (a `--run` app that stays open must not hold it), unless AUTORADIO_MAC_LOCK_HELD is set by a caller that holds it.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
# The app is compiled in one tree per Mac (scripts/build-tree.sh): its output is 12 GiB a copy.
"$HERE/scripts/build-tree.sh" is || { echo "build-lab: this is not the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named)). Push your lane; the lead builds it there." >&2; exit 1; }
# Never build against the wrong Exact: .exact-src must be the commit in exact.pin with the patches applied (prints both shas otherwise).
"$HERE/scripts/check-exact-pin.sh" || exit 1
# Which app (AR_VARIANT / AR_APP_ID, scripts/app-identity.sh): refuse a wrong choice before anything is synced or built. app.json is only read, never written.
"$HERE/scripts/app-identity.sh" id >/dev/null || exit 1
HAS_DEVICE=""; HAS_ARCHIVE=""
for a in "$@"; do case "$a" in --device) HAS_DEVICE=1 ;; --archive) HAS_ARCHIVE=1 ;; esac; done
if [ -n "$HAS_DEVICE" ] && [ -z "$HAS_ARCHIVE" ] && [ -z "$("$HERE/scripts/app-identity.sh" variant)" ]; then
  echo "build-lab: a device build must name its development app: AR_VARIANT=lab or AR_VARIANT=lab-one (the production id reaches a phone through TestFlight, archive-store.sh)" >&2; exit 1
fi
if [ -n "$HAS_ARCHIVE" ] && [ "${AR_BUILD_KIND:-}" != store ]; then echo "build-lab: an archive is the store kind: AR_BUILD_KIND=store (archive-store.sh sets it)" >&2; exit 1; fi
if [ "${AR_BUILD_KIND:-}" = store ] && [ -n "$("$HERE/scripts/app-identity.sh" variant)" ]; then echo "build-lab: the store kind is the production app: unset AR_VARIANT and AR_APP_ID (a development app is never archived)" >&2; exit 1; fi
if [ "${AR_BUILD_KIND:-}" = store ] && [ -z "$HAS_ARCHIVE" ]; then echo "build-lab: AR_BUILD_KIND=store is for --archive only" >&2; exit 1; fi
case "${AR_BUILD_KIND:-development}" in development|store) ;; *) echo "build-lab: AR_BUILD_KIND must be development or store" >&2; exit 1 ;; esac
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
