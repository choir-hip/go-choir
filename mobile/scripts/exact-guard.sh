#!/bin/sh
# What Exact may be asked to do for this app, decided in ONE place: exact-guard.sh <caller> <the arguments of the Exact command>. exact.sh and build-lab.sh both call it
# (build-lab before it stages anything). It is a list of what runs, not a list of what is refused: a verb, a target or a flag that is not named below is refused, so a verb
# Exact adds later is refused until it is read and listed here. Flags are read by exact-args.sh in both spellings.
#
# NO ARCHIVE, EVER, THROUGH EXACT.SH OR BUILD-LAB.SH. `--archive` (either spelling, any identity, any environment) is refused: the store archive has ONE route, archive-store.sh, which
# stages the store kind of the Swift itself immediately before the build, runs Exact itself (exact-env.sh, not this guard) and reads the ipa's executable and signature back
# (check-store-ipa.py). An earlier version let exact.sh archive when an environment variable said so; the variable could be set after a development build had left the launch hooks
# staged, and every check was of the command line, none of the tree or the ipa.
#
# The production app is what runs with no development app chosen (app-identity.sh: no AR_VARIANT, no AR_APP_ID). It may reach a phone or the world only through the store
# (archive-store.sh, TestFlight), so with no choice only these run (Exact's verbs at the pin, scripts/app-cli.mjs appVerbs, each read):
#   no argument, help, --help, -h     prints help (help takes no argument after it; an EMPTY first argument is refused)
#   contract, forma, docs             compile, check and read: no bundle, no device
#   web, web-build, serve             the web host
#   test|agent <web|ios|macos|mac>    a drive of the web, the simulator or this Mac; never --device, --phone or --unsigned (the flags that reach a phone)
#   verify web                        the web leg of the check
#   ios [--run] [--json]              the simulator build; mac [--run] [--json]: this Mac
# Refused with no choice, by being unlisted: share (an iPhone ipa for AppDrop), deploy and push (publish), update, lock, add, hatch, snapback, backend, pattern, feedback,
# linux, windows, android, and every `--device` build (a development install of the production id).
# With a development app chosen (AR_VARIANT=lab or lab-one) the list is the development app's, and it is a list of REFUSALS: everything is run except an archive and `share`; a
# phone build needs the build tree. That is deliberate: a verb Exact adds later runs for a development app (its blast radius is a lab build, which is not the store app) and is
# refused for the production app until it is listed. A development app's variant is only AR_VARIANT / AR_APP_ID: `--variant` and EXACT_VARIANT are refused.
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
HAS_DEVICE=""; ea_has --device "$@" && HAS_DEVICE=1; ea_has --phone "$@" && HAS_DEVICE=1       # --phone names a phone as --device does: either is a phone build
in_tree() { "$HERE/scripts/build-tree.sh" is || no "$1 runs in the build tree ($("$HERE/scripts/build-tree.sh" path 2>/dev/null || echo none named)); push your lane, the lead builds it there"; }

# An empty first argument is no verb: `exact.sh "" ios --device` must not ride on "no verb prints help".
if [ "$#" -ge 1 ] && [ -z "$1" ]; then no "the first argument is empty: name a verb (an empty one is refused, not read as help)"; fi
VERB="${1:-}"
if ea_has --archive "$@"; then no "an archive is made only by archive-store.sh (it stages the store kind itself, runs Exact itself and reads the ipa back); exact.sh and build-lab.sh run none, for any app, identity or environment"; fi
if [ "$VERB" = share ] && [ -n "$VARIANT" ]; then no "share is not run for a development app: it publishes an iPhone build for testers; the production app reaches testers through TestFlight (archive-store.sh)"; fi
if [ -n "$VARIANT" ]; then
  [ -z "$HAS_DEVICE" ] || in_tree "a phone build"
  exit 0
fi

# ---- the production app: the list ----
case "$VERB" in
  '') exit 0 ;;
  help|--help|-h) [ "$#" -eq 1 ] || no "$VERB takes no argument after it (it prints help; a phone build is not asked through it)"; exit 0 ;;
  contract|forma|docs|web|web-build|serve) exit 0 ;;
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
