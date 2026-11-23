#!/bin/sh
# Upload a store .ipa to App Store Connect (TestFlight processing) with Apple's own tool, `xcrun altool --upload-app` (Xcode 27.1: present; altool is the supported uploader
# here, `xcrun altool --help`). The owner's act or the lead's, never an agent's on its own (docs/definitions/choir-shipping-the-app-2026-11-23.md, must_preserve).
#   ASC_KEY_ID=<API key id> ASC_ISSUER_ID=<issuer id> ASC_KEY_FILE=<path of the AuthKey .p8>  mobile/scripts/upload-store.sh <ipa>
# The key id and issuer are identifiers, the .p8 is the secret and is passed by PATH only (altool's --p8-file-path); none of the three is printed, and nothing goes through a
# password flag. Refuses unless the ipa is the production app (not a development one) and carries a provisioning profile. Run outside the Nix shell (xe.sh).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
PATH="$PATH:/usr/bin:/bin:/usr/sbin:/sbin"
no() { echo "upload-store: $1" >&2; exit 1; }
IPA="${1:-}"
[ -n "$IPA" ] && [ -f "$IPA" ] && case "$IPA" in *.ipa) true ;; *) false ;; esac || no "give the path of an .ipa file (archive-store.sh prints it)"
[ -n "${ASC_KEY_ID:-}" ] || no "ASC_KEY_ID is not set (the App Store Connect API key's id)"
[ -n "${ASC_ISSUER_ID:-}" ] || no "ASC_ISSUER_ID is not set (the issuer id on the API keys page)"
[ -n "${ASC_KEY_FILE:-}" ] && [ -f "$ASC_KEY_FILE" ] || no "ASC_KEY_FILE must be the path of the API key's .p8 file"
# group- or world-readable key file: say so, do not touch it
[ -z "$(find "$ASC_KEY_FILE" -perm -044 2>/dev/null)" ] || echo "upload-store: warning: ASC_KEY_FILE is readable by others; chmod 600 it" >&2
ID="$("$HERE/scripts/app-identity.sh" id)" || exit 1
[ -z "${AR_VARIANT:-}${AR_APP_ID:-}${AR_APP_NAME:-}" ] || no "the upload is for the production app: unset AR_VARIANT, AR_APP_ID and AR_APP_NAME"
/usr/bin/python3 -I - "$IPA" "$ID" <<'PY' || exit 1
import plistlib, sys, zipfile
ipa, bid = sys.argv[1:3]
def no(m): sys.stderr.write("upload-store: " + m + "\n"); sys.exit(1)
try: z = zipfile.ZipFile(ipa)
except Exception: no("the file is not an ipa (not a zip)")
plists = [n for n in z.namelist() if n.count("/") == 2 and n.startswith("Payload/") and n.endswith(".app/Info.plist")]
if len(plists) != 1: no("the ipa has no single app")
info = plistlib.loads(z.read(plists[0]))
if info.get("CFBundleIdentifier") != bid: no("the ipa is %r, not the production app %s" % (info.get("CFBundleIdentifier"), bid))
if plists[0][: -len("Info.plist")] + "embedded.mobileprovision" not in z.namelist(): no("the ipa carries no provisioning profile")
print("upload-store: %s %s (%s) build %s" % (bid, info.get("CFBundleShortVersionString"), "ipa ok", info.get("CFBundleVersion")))
PY
echo "upload-store: uploading to App Store Connect (credentials not shown)"
"$HERE/scripts/xe.sh" /usr/bin/xcrun altool --upload-app -f "$IPA" --api-key "$ASC_KEY_ID" --api-issuer "$ASC_ISSUER_ID" --p8-file-path "$ASC_KEY_FILE" || no "altool refused or failed (exit above); the build may still be processing, check App Store Connect before uploading again"
echo "upload-store: uploaded; App Store Connect now processes the build (minutes), then it appears in TestFlight"
