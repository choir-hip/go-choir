#!/bin/sh
# The signed App Store .ipa of the production app ("Autoradio 1", app.json's app.id). The lead runs this in the build tree through the build lock:
#   EXACT_IDENTITY=<SHA-1 of the Apple Distribution identity> EXACT_PROFILE=<path of the App Store profile for the production id> \
#     /usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock mobile/scripts/archive-store.sh
# It refuses (plain message, nothing built) unless: no AR_VARIANT/AR_APP_ID is set (the store build is the production app), this is the build tree, EXACT_IDENTITY is a
# 40-hex SHA-1 of an Apple Distribution identity this Mac holds, and EXACT_PROFILE is an unexpired App Store profile for exactly the production id made for that identity
# (check-store-signing.py; it names what is wrong and prints no key material). It never searches for an identity or a profile.
# Then: the build number (below); the STORE staging of the Swift, made here every time immediately before the build (stage-app.sh <ios> store: sync-exact-package.sh leaves the developer
# launch hooks out, so nothing an earlier development build staged is what Exact reads); and Exact's own archive command, run by this script itself through exact-env.sh
# (exact.sh and build-lab.sh refuse every archive: this is the ONE route, so no marker or environment variable can make another)
#   bun exact.mjs ios --device --archive <ipa> --json      (reference.md "Build an iOS app on a CI machine"; --archive bakes production trust and Swift's release build)
# with EXACT_VERSION (app.json "version"), EXACT_BUILD_NUMBER, EXACT_NON_EXEMPT_ENCRYPTION=false (HTTPS and the system's encryption only; Exact writes ITSAppUsesNonExemptEncryption)
# and EXACT_DEV_MENU=0 (exact-env.sh gives it to every build: Exact's developer menu, four fingers, is off in every build of this app). Last, it reads the .ipa back and prints its
# path, bundle id, version and build number, and fails if they are not what was asked for, if any Mach-O of the app (the executable, the module dylib or framework, plug-ins) holds a name from store-dev-names.txt (the developer launch hooks), or
# if the signature is not an Apple Distribution one for the production id (check-store-ipa.py).
# The BUILD NUMBER (CFBundleVersion): by default whole minutes since 2026-01-01 00:00 UTC, or one more than the last number this script gave if that is higher ($OUTDIR/last-build),
# so a default number is never at or below the record, a re-upload of the same commit is a higher number (App Store Connect takes a build number once per version), and it has at
# most 9 digits for centuries. AR_BUILD_NUMBER=<n> (a whole number, 1 to 999999999) is used as given, even if it is at or below the record (App Store Connect refuses a number it
# already holds; the script cannot know one it was not given), and raises the record only when it is higher; later default numbers stay above the record. A number is CLAIMED
# atomically: the directory $OUTDIR/claims/<number> is made (mkdir fails if it exists), so two runs at once, or a run after the record was lost, never give one number twice, and an
# AR_BUILD_NUMBER this script gave before is refused. The caller holds the build lock (the command above); this script takes none.
# The .ipa goes to $AR_STORE_OUT_DIR (default ~/.cache/autoradio-store), outside the repo. EXACT_PROFILE and AR_STORE_OUT_DIR may be relative to where this is run; they are made absolute.
# ALLOW_NO_MODELS is refused (a store archive always carries the voice models).
set -eu
PATH="/usr/bin:/bin:/usr/sbin:/sbin"     # the system's tools only: a codesign, ditto or security earlier on the caller's PATH must not judge a store build (the checkers name /usr/bin themselves too)
HERE="$(cd "$(dirname "$0")/.." && pwd)"
no() { echo "archive-store: $1" >&2; exit 1; }
[ -z "${AR_VARIANT:-}${AR_APP_ID:-}${AR_APP_NAME:-}" ] || no "the store build is the production app: unset AR_VARIANT, AR_APP_ID and AR_APP_NAME (a development app is never uploaded)"
[ -z "${AR_APP_JSON:-}" ] || no "AR_APP_JSON is set: the store build reads the app's own app.json only (another identity file would ship another id, name or version); unset it"
"$HERE/scripts/build-tree.sh" is || no "this is not the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named)). Push your lane; the lead archives it there."
ID="$("$HERE/scripts/app-identity.sh" id)" || exit 1
NAME="$("$HERE/scripts/app-identity.sh" name)" || exit 1
VERSION="$("$HERE/scripts/app-identity.sh" version)" || exit 1
[ -n "${EXACT_IDENTITY:-}" ] || no "EXACT_IDENTITY is not set: name the Apple Distribution identity by its SHA-1 (40 hex digits)."
case "$EXACT_IDENTITY" in *[!0-9A-Fa-f]*) bad=1 ;; *) bad="" ;; esac     # a pattern on the whole value: grep matches one line of a multi-line value
[ -z "$bad" ] && [ "${#EXACT_IDENTITY}" -eq 40 ] || no "EXACT_IDENTITY must be the identity's SHA-1: 40 hex digits."
[ -n "${EXACT_PROFILE:-}" ] || no "EXACT_PROFILE is not set: name the App Store provisioning profile for $ID by its path."
[ -f "$EXACT_PROFILE" ] || no "EXACT_PROFILE is not a file."
[ -z "${ALLOW_NO_MODELS:-}" ] || no "ALLOW_NO_MODELS is set: a store archive without the on-device voice models is a defect, not a build (run scripts/ensure-deps.sh ios; unset ALLOW_NO_MODELS)"
# Paths mean the directory this script was started in, for the checks below and for Exact (exact.sh runs it from AutoradioLab/): made absolute once, here.
EXACT_PROFILE="$(cd "$(dirname "$EXACT_PROFILE")" && pwd -P)/$(basename "$EXACT_PROFILE")"
/usr/bin/python3 -I "$HERE/scripts/check-store-signing.py" "$ID" "$EXACT_IDENTITY" "$EXACT_PROFILE" || exit 1
OUTDIR="${AR_STORE_OUT_DIR:-$HOME/.cache/autoradio-store}"
mkdir -p "$OUTDIR"
OUTDIR="$(cd "$OUTDIR" && pwd -P)"
# Stage the store kind NOW, before a number is claimed (a staging that fails burns none), then run Exact on what was just staged: nothing between the two reads the tree for the build.
"$HERE/scripts/stage-app.sh" ios store || no "staging the store kind failed (see above); nothing was built"
# The build number: the minute count, but never at or below the last number this script gave (kept in $OUTDIR/last-build), and claimed atomically (see the header).
LASTF="$OUTDIR/last-build"; LAST=0
if [ -e "$LASTF" ]; then
  LAST="$(cat "$LASTF")"
  case "$LAST" in ''|0?*|*[!0-9]*) no "$LASTF holds no whole number (the last build number given): fix or delete it." ;; esac
  [ "${#LAST}" -le 9 ] || no "$LASTF holds no whole number (the last build number given): fix or delete it."
fi
okbuild() { case "$1" in ''|0*|*[!0-9]*) return 1 ;; esac; [ "${#1}" -le 9 ]; }
CLAIMS="$OUTDIR/claims"; mkdir -p "$CLAIMS"
if [ -n "${AR_BUILD_NUMBER:-}" ]; then
  BUILD="$AR_BUILD_NUMBER"
  okbuild "$BUILD" || no "the build number must be a whole number from 1 to 999999999 (AR_BUILD_NUMBER, or the record in $LASTF)."
  mkdir "$CLAIMS/$BUILD" 2>/dev/null || { [ -d "$CLAIMS/$BUILD" ] || no "cannot claim a build number in $CLAIMS (is it writable?)"; no "build number $BUILD was already given by this script (its claim is in $CLAIMS): App Store Connect takes a number once per version; give another."; }
else
  BUILD=$(( ($(date -u +%s) - 1767225600) / 60 )); [ "$BUILD" -gt "$LAST" ] || BUILD=$((LAST + 1))
  while :; do
    okbuild "$BUILD" || no "the build number must be a whole number from 1 to 999999999 (AR_BUILD_NUMBER, or the record in $LASTF)."
    mkdir "$CLAIMS/$BUILD" 2>/dev/null && break
    [ -d "$CLAIMS/$BUILD" ] || no "cannot claim a build number in $CLAIMS (is it writable?)"      # a mkdir that failed for another reason than "taken" is not a number to skip
    BUILD=$((BUILD + 1))
  done
fi
# The record only goes up: re-read under a short lock (a directory made and removed), so a run holding a lower number cannot write over a higher one another run just wrote.
RLOCK="$OUTDIR/last-build.lock"; _w=0
until mkdir "$RLOCK" 2>/dev/null; do _w=$((_w + 1)); [ "$_w" -lt 100 ] || no "$RLOCK is held (another run is recording a number; if none is running, remove that directory)"; sleep 0.1; done
trap 'rmdir "$RLOCK" 2>/dev/null' EXIT INT TERM       # an interrupt between the mkdir and the rmdir leaves no stale lock
CUR=0; [ -e "$LASTF" ] && CUR="$(cat "$LASTF")"
case "$CUR" in ''|*[!0-9]*) CUR=0 ;; esac
if [ "$BUILD" -gt "$CUR" ]; then printf '%s\n' "$BUILD" > "$LASTF.$$" && mv -f "$LASTF.$$" "$LASTF"; fi
rmdir "$RLOCK"; trap - EXIT INT TERM
IPA="$OUTDIR/$(printf '%s' "$NAME" | tr -c 'A-Za-z0-9.\n' '-')-$VERSION-$BUILD.ipa"
rm -f "$IPA"
LOG="$OUTDIR/archive-$BUILD.stdout"
echo "archive-store: $NAME $ID version $VERSION build $BUILD"
export EXACT_VERSION="$VERSION" EXACT_BUILD_NUMBER="$BUILD" EXACT_NON_EXEMPT_ENCRYPTION=false
. "$HERE/scripts/exact-env.sh"
set +e
exact_run ios --device --archive "$IPA" --json > "$LOG"
RC=$?
set -e
[ "$RC" -eq 0 ] || { tail -5 "$LOG" >&2; no "Exact's archive failed (exit $RC); its log is above on stderr, its JSON line is in $LOG"; }
[ -f "$IPA" ] || no "Exact reported success but wrote no .ipa at the path asked for"
/usr/bin/python3 -I "$HERE/scripts/check-store-ipa.py" archive-store "$IPA" "$ID" "$VERSION" "$BUILD" || exit 1
/usr/bin/python3 -I - "$LOG" <<'PY'
import json, sys
def no(m): sys.stderr.write("archive-store: the .ipa is not what was asked for: " + m + "\n"); sys.exit(1)
try:
    j = json.loads([l for l in open(sys.argv[1]).read().splitlines() if l.strip()][-1])
    if not isinstance(j, dict): raise ValueError
    if j.get("trust") != "production": no("Exact baked %r trust, not production" % (j.get("trust"),))
except (OSError, IndexError, ValueError): no("Exact printed no JSON result line")
print("ExactBuildKind: archive, developer menu off, trust production")
PY
