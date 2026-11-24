#!/bin/sh
# What Exact may be asked to do for this app, decided in ONE place: exact-guard.sh <caller> <the arguments of the Exact command>. exact.sh and build-lab.sh both call it
# (build-lab before it stages anything). It is a list of what runs, not a list of what is refused: a verb, a target or a flag that is not named below is refused, so a verb
# Exact adds later is refused until it is read and listed here. Flags are read by exact-args.sh in both spellings.
#
# The production app is what runs with no development app chosen (app-identity.sh: no AR_VARIANT, no AR_APP_ID). It may reach a phone or the world only through the store
# (archive-store.sh, TestFlight), so with no choice only these run (Exact's verbs at the pin, scripts/app-cli.mjs appVerbs, each read):
#   help, --help, -h, no verb         prints help
#   contract, forma, docs             compile, check and read: no bundle, no device
#   web, web-build, serve             the web host
#   test|agent <web|ios|macos|mac>    a drive of the web, the simulator or this Mac; never --device, --phone, --archive or --unsigned (the only flags that reach a phone or write an ipa)
#   verify web                        the web leg of the check
#   ios [--run] [--json]              the simulator build; mac [--run] [--json]: this Mac
#   ios --device --archive <ipa> [--json]   the store archive, only with the marker archive-store.sh sets (AR_STORE_ARCHIVE=1) and the store's own settings, checked here again
# Refused with no choice, by being unlisted: share (an iPhone ipa for AppDrop), deploy and push (publish), update, lock, add, hatch, snapback, backend, pattern, feedback,
# linux, windows, android, and every `--device` build (a development install of the production id).
# With a development app chosen (AR_VARIANT=lab or lab-one): everything is run except an archive and share (neither is a development app's); a phone build needs the build tree.
# A development app's variant is only AR_VARIANT / AR_APP_ID: `--variant` and EXACT_VARIANT are refused (they would reach Exact without the sign-in scheme the choice carries).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
CALLER="${1:-exact-guard}"; shift
. "$HERE/scripts/exact-args.sh"
no() { echo "$CALLER: $1" >&2; exit 1; }
if ea_has --variant "$@"; then no "name the development app with AR_VARIANT=lab or AR_VARIANT=lab-one (or AR_APP_ID), not --variant: only AR_VARIANT also gives the build the sign-in scheme and the checks that go with the app"; fi
[ -z "${EXACT_VARIANT:-}" ] || no "EXACT_VARIANT is not read: name the development app with AR_VARIANT=lab or AR_VARIANT=lab-one (or AR_APP_ID)"
# Exact's own developer menu is off in every build of this app (exact.sh sets EXACT_DEV_MENU=0); a caller's other value is not honoured.
[ "${EXACT_DEV_MENU-0}" = 0 ] || no "EXACT_DEV_MENU is always 0 for this app (Exact's developer menu is off in every build); unset it"
VARIANT="$("$HERE/scripts/app-identity.sh" variant)" || exit 1
KIND="${AR_BUILD_KIND:-development}"
case "$KIND" in development|store) ;; *) no "AR_BUILD_KIND must be development or store" ;; esac
VERB="${1:-}"
HAS_ARCHIVE=""; ea_has --archive "$@" && HAS_ARCHIVE=1
HAS_DEVICE=""; ea_has --device "$@" && HAS_DEVICE=1
in_tree() { "$HERE/scripts/build-tree.sh" is || no "$1 runs in the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named)); push your lane, the lead builds it there"; }

if [ "$VERB" = share ] && [ -n "$VARIANT" ]; then no "share is not run for a development app: it publishes an iPhone build for testers; the production app reaches testers through TestFlight (archive-store.sh)"; fi

if [ "$KIND" = store ] && [ -z "$HAS_ARCHIVE" ]; then no "AR_BUILD_KIND=store is for --archive only"; fi
if [ -n "$VARIANT" ]; then
  [ -z "$HAS_ARCHIVE" ] || no "an archive is the production app's store build: unset AR_VARIANT and AR_APP_ID (a development app is never archived)"
  [ -z "$HAS_DEVICE" ] || in_tree "a phone build"
  exit 0
fi

# ---- the production app: the list ----
if [ -n "$HAS_ARCHIVE" ]; then
  [ "${AR_STORE_ARCHIVE:-}" = 1 ] && [ "$KIND" = store ] || no "an archive is made only by archive-store.sh (it sets AR_BUILD_KIND=store and the store marker); run that, with EXACT_IDENTITY and EXACT_PROFILE"
  [ "$VERB" = ios ] || no "the store archive is the ios verb's"
  # exactly: ios --device --archive <ipa> [--json]  (Exact reads `--archive <path>` only)
  [ "$#" -ge 4 ] && [ "$2" = --device ] && [ "$3" = --archive ] && [ -n "$4" ] || no "the store archive is exactly: ios --device --archive <ipa> [--json]"
  if [ "$#" -ge 5 ]; then [ "$#" -eq 5 ] && [ "$5" = --json ] || no "the store archive is exactly: ios --device --archive <ipa> [--json]; nothing else may follow (no --unsigned, --phone, --run)"; fi
  in_tree "an archive"
  ID="$("$HERE/scripts/app-identity.sh" id)"; VERSION="$("$HERE/scripts/app-identity.sh" version)"
  [ "${EXACT_VERSION:-}" = "$VERSION" ] || no "the archive needs EXACT_VERSION=$VERSION (app.json's version)"
  case "${EXACT_BUILD_NUMBER:-}" in ''|0*|*[!0-9]*) no "the archive needs EXACT_BUILD_NUMBER, a whole number from 1 to 999999999" ;; esac
  [ "${#EXACT_BUILD_NUMBER}" -le 9 ] || no "the archive needs EXACT_BUILD_NUMBER, a whole number from 1 to 999999999"
  [ "${EXACT_NON_EXEMPT_ENCRYPTION:-}" = false ] || no "the archive needs EXACT_NON_EXEMPT_ENCRYPTION=false (the export-compliance answer)"
  case "${EXACT_IDENTITY:-}" in ''|*[!0-9A-Fa-f]*) no "the archive needs EXACT_IDENTITY: the Apple Distribution identity's SHA-1 (40 hex digits)" ;; esac
  [ "${#EXACT_IDENTITY}" -eq 40 ] || no "the archive needs EXACT_IDENTITY: the Apple Distribution identity's SHA-1 (40 hex digits)"
  [ -n "${EXACT_PROFILE:-}" ] && [ -f "$EXACT_PROFILE" ] || no "the archive needs EXACT_PROFILE: the App Store profile's path"
  PATH="$PATH:/usr/bin:/bin:/usr/sbin:/sbin"
  SIGN="$(/usr/bin/python3 -I "$HERE/scripts/check-store-signing.py" "$ID" "$EXACT_IDENTITY" "$EXACT_PROFILE" 2>&1)" || no "${SIGN#archive-store: }"
  exit 0
fi
case "$VERB" in
  ''|help|--help|-h|contract|forma|docs|web|web-build|serve) exit 0 ;;
  test|agent|verify)
    HOST="${2:-}"
    case "$VERB:$HOST" in test:web|test:ios|test:macos|test:mac|agent:web|agent:ios|agent:macos|agent:mac|verify:web) ;; *) no "$VERB runs with no development app only for a named host (test|agent web, ios, macos, mac; verify web), not '$HOST'" ;; esac
    for f in --device --phone --unsigned; do ea_has "$f" "$@" && no "$VERB $f puts a build on a phone: name the development app, AR_VARIANT=lab or AR_VARIANT=lab-one (the production app reaches a phone through TestFlight, archive-store.sh)"; done
    exit 0 ;;
  ios|mac)
    shift
    ea_only --run --json --help -h -- "$@" || no "$VERB with no development app is the simulator or this Mac build only (--run, --json); a phone build, --unsigned and the rest need a development app: AR_VARIANT=lab or AR_VARIANT=lab-one (the production app reaches a phone through TestFlight, archive-store.sh)"
    exit 0 ;;
  *) no "$VERB is not run with no development app chosen (it could put the production app on a phone or publish it); the production app reaches testers through archive-store.sh and TestFlight. For a development app: AR_VARIANT=lab or AR_VARIANT=lab-one" ;;
esac
