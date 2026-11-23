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
# The BUILD NUMBER (CFBundleVersion) is whole minutes since 2026-01-01 00:00 UTC: it rises with every run, so a re-upload of the same commit is a higher number (App Store
# Connect takes a build number once per version), and it has at most 9 digits for centuries. AR_BUILD_NUMBER=<n> sets it (a whole number, 1 to 999999999), e.g. when App Store
# Connect's next number is wanted. Two runs inside one minute would collide: the build lock makes that impossible.
# The .ipa goes to $AR_STORE_OUT_DIR (default ~/.cache/autoradio-store), outside the repo.
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
/usr/bin/python3 -I "$HERE/scripts/check-store-signing.py" "$ID" "$EXACT_IDENTITY" "$EXACT_PROFILE" || exit 1
if [ -n "${AR_BUILD_NUMBER:-}" ]; then BUILD="$AR_BUILD_NUMBER"; else BUILD=$(( ($(date -u +%s) - 1767225600) / 60 )); fi
printf '%s' "$BUILD" | grep -Eq '^[1-9][0-9]{0,8}$' || no "AR_BUILD_NUMBER must be a whole number from 1 to 999999999 (CFBundleVersion rises with every upload)."
OUTDIR="${AR_STORE_OUT_DIR:-$HOME/.cache/autoradio-store}"
mkdir -p "$OUTDIR"
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
/usr/bin/python3 -I - "$IPA" "$ID" "$VERSION" "$BUILD" "$LOG" <<'PY'
import json, plistlib, sys, zipfile
ipa, bid, ver, build, log = sys.argv[1:6]
def no(m): sys.stderr.write("archive-store: the .ipa is not what was asked for: " + m + "\n"); sys.exit(1)
z = zipfile.ZipFile(ipa)
names = z.namelist()
plists = [n for n in names if n.count("/") == 2 and n.startswith("Payload/") and n.endswith(".app/Info.plist")]
if len(plists) != 1: no("it has no single Payload/<app>.app/Info.plist")
info = plistlib.loads(z.read(plists[0]))
app = plists[0][: -len("Info.plist")]
if info.get("CFBundleIdentifier") != bid: no("bundle id is %r, wanted %r" % (info.get("CFBundleIdentifier"), bid))
if info.get("CFBundleShortVersionString") != ver: no("version is %r, wanted %r" % (info.get("CFBundleShortVersionString"), ver))
if info.get("CFBundleVersion") != build: no("build number is %r, wanted %r" % (info.get("CFBundleVersion"), build))
if info.get("ITSAppUsesNonExemptEncryption") is not False: no("ITSAppUsesNonExemptEncryption is %r, wanted false" % (info.get("ITSAppUsesNonExemptEncryption"),))
if info.get("ExactDevMenu") is not False: no("the developer menu is not stamped off (ExactDevMenu is %r)" % (info.get("ExactDevMenu"),))
if info.get("ExactBuildKind") != "archive": no("ExactBuildKind is %r, wanted 'archive'" % (info.get("ExactBuildKind"),))
dev = sorted(k for k in info if k.startswith("ExactDevelopment"))
if dev: no("it carries a development client's keys: " + ", ".join(dev))
if app + "embedded.mobileprovision" not in names: no("it has no embedded provisioning profile (the .ipa is not signed for the store)")
try:
    last = [l for l in open(log).read().splitlines() if l.strip()][-1]
    j = json.loads(last)
    if j.get("trust") != "production": no("Exact baked %r trust, not production" % (j.get("trust"),))
except (OSError, IndexError, ValueError): no("Exact printed no JSON result line")
print("ipa: " + ipa)
print("bundle id: " + info["CFBundleIdentifier"])
print("version: " + info["CFBundleShortVersionString"])
print("build number: " + info["CFBundleVersion"])
print("ITSAppUsesNonExemptEncryption: false")
print("ExactBuildKind: archive, developer menu off, trust production")
PY
