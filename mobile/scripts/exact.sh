#!/bin/sh
# Run an Exact command for AutoradioLab outside the Nix dev shell, e.g.  mobile/scripts/exact.sh contract build app.contract
# No choice runs it for the production app ("Autoradio 1", app.json's app.id). AR_VARIANT=lab or lab-one (or AR_APP_ID=<its id>) runs it for a development app
# beside it (app-identity.sh; README, "The apps on the phone").
# BUN names a Bun 1.4.2 binary (default ~/.bun-1.4.2/bin/bun, installed by setup-exact.sh).
# EXACT_SIM (a simulator name or udid) is passed through: use scripts/sim-device.sh to get a simulator of your own.
# Exact allows one app per bundle id per simulator and a launch replaces a running copy, so two people driving
# the same simulator kill each other's app ("the app hung up (exit code 0)", "never opened its agent socket").
HERE="$(cd "$(dirname "$0")/.." && pwd)"
"$HERE/scripts/check-exact-pin.sh" || exit 1      # the wrong Exact must never build quietly (see exact.pin)
# Which app: AR_VARIANT or AR_APP_ID (app-identity.sh). Exact reads it as EXACT_VARIANT, which `env -i` below would drop unless it is passed on.
VARIANT="$("$HERE/scripts/app-identity.sh" variant)" || exit 1
# The sign-in callback scheme is the app's id (app.ts SIGNIN_SCHEME, grant auth.callback): given to Exact explicitly for every identity, so the default never rests on a
# literal fallback and an empty value in the caller's environment cannot make a dead grant (app.ts reads it with `??`; the build refuses `||`).
SCHEME="$("$HERE/scripts/app-identity.sh" id)" || exit 1
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
# What `env -i` below would drop and Exact reads from its environment (build-and-ship.md, "Archive, TestFlight, CI"): the signing pair, the version and build number,
# the export-compliance answer and the developer menu's build-time off. Passed only when set, so a development build is exactly as it was.
set -- "$BUN" exact.mjs "$@"
N=$#
for v in EXACT_IDENTITY EXACT_PROFILE EXACT_VERSION EXACT_BUILD_NUMBER EXACT_NON_EXEMPT_ENCRYPTION EXACT_DEV_MENU; do
  eval "isset=\${$v+set}"
  if [ -n "$isset" ]; then eval "val=\$$v"; set -- "$@" "$v=$val"; fi       # a value may hold spaces (a profile's path): one argument each
done
while [ "$N" -gt 0 ]; do a="$1"; shift; set -- "$@" "$a"; N=$((N - 1)); done   # the assignments first, then the command
"$HERE/scripts/gen-build-info.sh"                 # About names the commit: never build from the previous commit's build-info.gen.ts
cd "$HERE/AutoradioLab" || exit 1
exec env -i HOME="$HOME" USER="$USER" TMPDIR="${TMPDIR:-/tmp}" BUN="$BUN" ${EXACT_SIM:+EXACT_SIM="$EXACT_SIM"} ${VARIANT:+EXACT_VARIANT="$VARIANT"} SIGNIN_SCHEME="$SCHEME" \
  PATH="$(dirname "$BUN"):$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin" \
  DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer EXACT2="$HERE/.exact-src" EXACT_FEEDBACK=0 \
  "$@"
