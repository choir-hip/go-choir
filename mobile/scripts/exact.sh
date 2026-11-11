#!/bin/sh
# Run an Exact command for AutoradioLab outside the Nix dev shell, e.g.  mobile/scripts/exact.sh contract build app.contract
# BUN names a Bun 1.4.2 binary (default ~/.bun-1.4.2/bin/bun, installed by setup-exact.sh).
# EXACT_SIM (a simulator name or udid) is passed through: use scripts/sim-device.sh to get a simulator of your own.
# Exact allows one app per bundle id per simulator and a launch replaces a running copy, so two people driving
# the same simulator kill each other's app ("the app hung up (exit code 0)", "never opened its agent socket").
HERE="$(cd "$(dirname "$0")/.." && pwd)"
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
cd "$HERE/AutoradioLab" || exit 1
exec env -i HOME="$HOME" USER="$USER" TMPDIR="${TMPDIR:-/tmp}" BUN="$BUN" ${EXACT_SIM:+EXACT_SIM="$EXACT_SIM"} \
  PATH="$(dirname "$BUN"):$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin" \
  DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer EXACT2="$HERE/.exact-src" EXACT_FEEDBACK=0 \
  "$BUN" exact.mjs "$@"
