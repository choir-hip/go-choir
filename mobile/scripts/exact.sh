#!/bin/sh
# Run an Exact command for AutoradioLab outside the Nix dev shell, e.g.  mobile/scripts/exact.sh contract build app.contract
# No choice runs it for the production app ("Autoradio 1", app.json's app.id). AR_VARIANT=lab or lab-one (or AR_APP_ID=<its id>) runs it for a development app
# beside it (app-identity.sh; README, "The apps on the phone").
# BUN names a Bun 1.4.2 binary (default ~/.bun-1.4.2/bin/bun, installed by setup-exact.sh).
# EXACT_SIM (a simulator name or udid) is passed through: use scripts/sim-device.sh to get a simulator of your own.
# Exact allows one app per bundle id per simulator and a launch replaces a running copy, so two people driving
# the same simulator kill each other's app ("the app hung up (exit code 0)", "never opened its agent socket").
# The one guard (exact-guard.sh, also run by build-lab.sh before it stages anything): what Exact may be asked to do for this app is a LIST. With no development app chosen only
# the verbs and targets that cannot put a signed production-id app on a phone or publish it run; with a development app chosen `share` is refused. An archive is refused for
# every app: it is archive-store.sh's alone (it runs Exact through exact-env.sh itself). Anything not listed is refused, a verb Exact adds later included.
HERE="$(cd "$(dirname "$0")/.." && pwd)"
"$HERE/scripts/exact-guard.sh" exact.sh "$@" || exit 1
. "$HERE/scripts/exact-env.sh"
exact_run "$@"
