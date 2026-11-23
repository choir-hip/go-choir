#!/bin/sh
# The signed App Store .ipa of the production app ("Autoradio 1", app.json's app.id). The lead runs this in the build tree through the build lock:
#   EXACT_IDENTITY=<SHA-1 of the Apple Distribution identity> EXACT_PROFILE=<path of the App Store profile for the production id> \
#     /usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock mobile/scripts/archive-store.sh
# It refuses (plain message, nothing built) unless: no AR_VARIANT/AR_APP_ID is set (the store build is the production app), this is the build tree, EXACT_IDENTITY is a
# 40-hex SHA-1 of an Apple Distribution identity this Mac holds, and EXACT_PROFILE is an unexpired App Store profile for exactly the production id made for that identity
# (check-store-signing.py; it names what is wrong and prints no key material). It never searches for an identity or a profile.
# Then: the build number (below), the store kind of the build (AR_BUILD_KIND=store: build-lab.sh, sync-exact-package.sh), and Exact's own archive command
#   bun exact.mjs ios --device --archive <ipa> --json      (reference.md "Build an iOS app on a CI machine"; --archive bakes production trust and Swift's release build)
# with EXACT_VERSION (app.json "version"), EXACT_BUILD_NUMBER, EXACT_NON_EXEMPT_ENCRYPTION=false (HTTPS and the system's encryption only; Exact writes ITSAppUsesNonExemptEncryption)
# and EXACT_DEV_MENU=0 (Exact's iOS developer menu, four fingers, is ON in a release build unless the build says off: DevMenuIOS.swift). Last, it reads the .ipa back and prints its
# path, bundle id, version and build number, and fails if they are not what was asked for.
# The BUILD NUMBER (CFBundleVersion) is whole minutes since 2026-01-01 00:00 UTC, or one more than the last number this script gave if that is higher ($OUTDIR/last-build): it
# rises with every run, so a re-upload of the same commit is a higher number (App Store Connect takes a build number once per version), and it has at most 9 digits for
# centuries. AR_BUILD_NUMBER=<n> sets it (a whole number, 1 to 999999999) and raises the record; later default numbers stay above it. The build lock serializes runs.
# The .ipa goes to $AR_STORE_OUT_DIR (default ~/.cache/autoradio-store), outside the repo. EXACT_PROFILE and AR_STORE_OUT_DIR may be relative to where this is run; they are made absolute.
# ALLOW_NO_MODELS is refused (a store archive always carries the voice models).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
PATH="$PATH:/usr/bin:/bin:/usr/sbin:/sbin"
no() { echo "archive-store: $1" >&2; exit 1; }
[ -z "${AR_VARIANT:-}${AR_APP_ID:-}${AR_APP_NAME:-}" ] || no "the store build is the production app: unset AR_VARIANT, AR_APP_ID and AR_APP_NAME (a development app is never uploaded)"
"$HERE/scripts/build-tree.sh" is || no "this is not the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named)). Push your lane; the lead archives it there."
ID="$("$HERE/scripts/app-identity.sh" id)" || exit 1
NAME="$("$HERE/scripts/app-identity.sh" name)" || exit 1
VERSION="$("$HERE/scripts/app-identity.sh" version)" || exit 1
[ -n "${EXACT_IDENTITY:-}" ] || no "EXACT_IDENTITY is not set: name the Apple Distribution identity by its SHA-1 (40 hex digits)."
printf '%s' "$EXACT_IDENTITY" | grep -Eq '^[0-9A-Fa-f]{40}$' || no "EXACT_IDENTITY must be the identity's SHA-1: 40 hex digits."
[ -n "${EXACT_PROFILE:-}" ] || no "EXACT_PROFILE is not set: name the App Store provisioning profile for $ID by its path."
[ -f "$EXACT_PROFILE" ] || no "EXACT_PROFILE is not a file."
[ -z "${ALLOW_NO_MODELS:-}" ] || no "ALLOW_NO_MODELS is set: a store archive without the on-device voice models is a defect, not a build (run scripts/ensure-deps.sh ios; unset ALLOW_NO_MODELS)"
# Paths mean the directory this script was started in, for the checks below and for Exact (exact.sh runs it from AutoradioLab/): made absolute once, here.
EXACT_PROFILE="$(cd "$(dirname "$EXACT_PROFILE")" && pwd -P)/$(basename "$EXACT_PROFILE")"
/usr/bin/python3 -I "$HERE/scripts/check-store-signing.py" "$ID" "$EXACT_IDENTITY" "$EXACT_PROFILE" || exit 1
OUTDIR="${AR_STORE_OUT_DIR:-$HOME/.cache/autoradio-store}"
mkdir -p "$OUTDIR"
OUTDIR="$(cd "$OUTDIR" && pwd -P)"
# The build number: the minute count, but never at or below the last number this script gave (kept in $OUTDIR/last-build, written under the build lock), so a second run in
# one minute and a run after AR_BUILD_NUMBER=<a larger number> still rise. A given AR_BUILD_NUMBER is used as given and raises the record, never lowers it.
LASTF="$OUTDIR/last-build"; LAST=0
if [ -e "$LASTF" ]; then
  LAST="$(cat "$LASTF")"
  printf '%s' "$LAST" | grep -Eq '^[0-9]{1,9}$' || no "$LASTF holds no whole number (the last build number given): fix or delete it."
fi
if [ -n "${AR_BUILD_NUMBER:-}" ]; then BUILD="$AR_BUILD_NUMBER"
else BUILD=$(( ($(date -u +%s) - 1767225600) / 60 )); [ "$BUILD" -gt "$LAST" ] || BUILD=$((LAST + 1)); fi
printf '%s' "$BUILD" | grep -Eq '^[1-9][0-9]{0,8}$' || no "the build number must be a whole number from 1 to 999999999 (AR_BUILD_NUMBER, or the record in $LASTF)."
[ "$BUILD" -gt "$LAST" ] && printf '%s\n' "$BUILD" > "$LASTF"
IPA="$OUTDIR/$(printf '%s' "$NAME" | tr -c 'A-Za-z0-9.\n' '-')-$VERSION-$BUILD.ipa"
rm -f "$IPA"
LOG="$OUTDIR/archive-$BUILD.stdout"
echo "archive-store: $NAME $ID version $VERSION build $BUILD"
set +e
AR_BUILD_KIND=store EXACT_VERSION="$VERSION" EXACT_BUILD_NUMBER="$BUILD" EXACT_NON_EXEMPT_ENCRYPTION=false EXACT_DEV_MENU=0 \
  "$HERE/scripts/build-lab.sh" ios --device --archive "$IPA" --json > "$LOG"
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
    if j.get("trust") != "production": no("Exact baked %r trust, not production" % (j.get("trust"),))
except (OSError, IndexError, ValueError): no("Exact printed no JSON result line")
print("ExactBuildKind: archive, developer menu off, trust production")
PY
