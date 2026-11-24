#!/bin/sh
# The negative control of the store check: build a DEVELOPMENT app the way the store app is built (the same Exact archive command, identity and profile), and require that
# check-store-ipa.py REFUSES it for a listed developer hook, naming the file that holds it. The lead runs this once, before trusting the first store archive, in the build tree
# through the build lock; nothing is typed by hand:
#   EXACT_IDENTITY=<SHA-1 of the Apple Distribution identity> EXACT_PROFILE=<path of the App Store profile> \
#     /usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock mobile/scripts/store-negative-control.sh
# It stages the DEVELOPMENT kind (stage-app.sh ios development), runs Exact's archive through exact-env.sh with version, build number 1, the export answer and the signing pair,
# and then, whatever happened, stages the STORE kind again so the tree is never left holding the developer hooks. It cannot upload: it never calls altool, takes no App Store
# Connect credential, and deletes the .ipa it made (a development app signed for the store, named NEVER-UPLOAD, is no artifact to keep) once it has read it.
# Exit 0 only if the check refused the ipa with "holds developer-only names": it prints that refusal, and then every Mach-O of the unpacked app with the listed names each holds, so
# the lead sees where the hooks live in a real build and which listed names survive a release compile. Exit 1, saying which: Exact failed; the check did NOT refuse (the check is
# inert for release binaries: do not upload anything from this tree; stage the store kind (stage-app.sh ios store), archive from Xcode, then run archive-store.sh's checks by hand on that
# archive: check-store-ipa.py, `strings -a` on every Mach-O for each name of store-dev-names.txt, codesign -dvv and -d --entitlements); or it refused for another reason (read it).
# ALLOW_NO_MODELS is refused (the control must be built as the store app is).
set -eu
PATH="/usr/bin:/bin:/usr/sbin:/sbin"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
no() { echo "store-negative-control: $1" >&2; exit 1; }
[ -z "${AR_VARIANT:-}${AR_APP_ID:-}${AR_APP_NAME:-}" ] || no "the control is of the production app: unset AR_VARIANT, AR_APP_ID and AR_APP_NAME"
[ -z "${AR_APP_JSON:-}" ] || no "AR_APP_JSON is set: unset it"
[ -z "${ALLOW_NO_MODELS:-}" ] || no "ALLOW_NO_MODELS is set: the control is built as the store app is, with the models; unset it"
"$HERE/scripts/build-tree.sh" is || no "this is not the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named))."
ID="$("$HERE/scripts/app-identity.sh" id)" || exit 1
VERSION="$("$HERE/scripts/app-identity.sh" version)" || exit 1
[ -n "${EXACT_IDENTITY:-}" ] || no "EXACT_IDENTITY is not set: name the Apple Distribution identity by its SHA-1 (40 hex digits)."
case "$EXACT_IDENTITY" in *[!0-9A-Fa-f]*) bad=1 ;; *) bad="" ;; esac
[ -z "$bad" ] && [ "${#EXACT_IDENTITY}" -eq 40 ] || no "EXACT_IDENTITY must be the identity's SHA-1: 40 hex digits."
[ -n "${EXACT_PROFILE:-}" ] && [ -f "$EXACT_PROFILE" ] || no "EXACT_PROFILE must be the path of the App Store provisioning profile for $ID."
EXACT_PROFILE="$(cd "$(dirname "$EXACT_PROFILE")" && pwd -P)/$(basename "$EXACT_PROFILE")"
/usr/bin/python3 -I "$HERE/scripts/check-store-signing.py" "$ID" "$EXACT_IDENTITY" "$EXACT_PROFILE" || exit 1
OUTDIR="${AR_STORE_OUT_DIR:-$HOME/.cache/autoradio-store}/negative-control"
mkdir -p "$OUTDIR"; OUTDIR="$(cd "$OUTDIR" && pwd -P)"
IPA="$OUTDIR/NEVER-UPLOAD-negative-control.ipa"; LOG="$OUTDIR/exact.stdout"; UNP="$OUTDIR/unpacked"; REFUSAL="$OUTDIR/refusal.txt"
rm -rf "$IPA" "$UNP" "$REFUSAL"
export EXACT_VERSION="$VERSION" EXACT_BUILD_NUMBER=1 EXACT_NON_EXEMPT_ENCRYPTION=false
restage() { "$HERE/scripts/stage-app.sh" ios store >/dev/null 2>&1 || echo "store-negative-control: WARNING: restaging the store kind failed; run stage-app.sh ios store before anything else" >&2; }
echo "store-negative-control: staging the DEVELOPMENT kind, then Exact's archive (a development app; it is deleted after it is read)"
"$HERE/scripts/stage-app.sh" ios development || { restage; no "staging the development kind failed (see above)"; }
. "$HERE/scripts/exact-env.sh"
set +e
exact_run ios --device --archive "$IPA" --json > "$LOG"
RC=$?
set -e
restage
[ "$RC" -eq 0 ] || { tail -5 "$LOG" >&2; no "Exact's archive failed (exit $RC): nothing was judged"; }
[ -f "$IPA" ] || no "Exact reported success but wrote no .ipa"
set +e
/usr/bin/python3 -I "$HERE/scripts/check-store-ipa.py" negative-control "$IPA" "$ID" "$VERSION" 1 > "$OUTDIR/check.stdout" 2> "$REFUSAL"
CHECK=$?
set -e
if [ "$CHECK" -eq 0 ]; then
  rm -f "$IPA"
  no "NOT REFUSED: check-store-ipa.py accepted a development app. The check is inert for release binaries: do not upload anything. Stage the store kind (stage-app.sh ios store), archive from Xcode, and on that archive run check-store-ipa.py, strings -a on every Mach-O for each name of store-dev-names.txt, codesign -dvv and codesign -d --entitlements :- by hand."
fi
grep -q "holds developer-only names" "$REFUSAL" || { cat "$REFUSAL" >&2; rm -f "$IPA"; no "the development app was refused for ANOTHER reason (above), not for the developer hooks: the name search was not shown to work"; }
echo "store-negative-control: REFUSED, as it must be:"; cat "$REFUSAL"
# Where the names live, for the lead: every Mach-O of the unpacked app with the listed names it holds.
mkdir -p "$UNP"
/usr/bin/ditto -x -k "$IPA" "$UNP" || no "could not unpack the control to list its files"
echo "store-negative-control: the listed names each Mach-O of the development app holds:"
find "$UNP" -type f \( -name '*.dylib' -o -perm -u+x \) | sort | while read -r f; do
  r="$(/usr/bin/python3 -I "$HERE/scripts/check-store-ipa.py" --executable "$f" 2>&1 || true)"
  case "$r" in *"holds developer-only names"*) echo "  ${f#"$UNP"/}: ${r#*names: }" ;; esac
done
rm -rf "$IPA" "$UNP"
echo "store-negative-control: the development ipa is deleted; the store kind is staged again. Now run archive-store.sh (the same variables, no build number to give)."
