#!/bin/sh
# Shared helpers for the real-data proof (sourced by proof-real.sh).
# Needs HERE (the scripts directory) and SHOTS set by the caller. Provides: fail, SIM, reset_state, step/wait_/shot/view/place/goto,
# run_drive (a drive that starts from a clean app, retried when the agent socket is refused), must, nomust.
DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
SCRATCH="${TMPDIR:-/tmp}/proof-app.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
STEPS="$SCRATCH/steps.txt"
: > "$OUT"; : > "$STEPS"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-app-last.txt" 2>/dev/null; [ -n "${SHOTS:-}" ] && cp "$OUT" "$SHOTS/drive.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }
SIM() { DEVELOPER_DIR=$DD /usr/bin/xcrun simctl "$@"; }

# reset_state is defined by the caller (proof-real.sh): a clean app, signed in with the key file.

# Steps are collected one per line and handed to the driver as separate arguments (a step may hold spaces).
step() { printf '%s\n' "$1" >> "$STEPS"; }
wait_() { step "clock +${1:-700} real"; step "clock data"; }
shot() { step "screenshot $SHOTS/$1.png"; }
view() { wait_ "${2:-800}"; step tree; shot "$1"; }   # settle, dump the tree, screenshot <name>
place() { step "tap bar-mark"; wait_ 600; step "tap place-$1"; wait_ "${2:-900}"; }
# Put the app in the default theme, light, by its own Settings controls (the proof never trusts what an earlier run left).
ensure_light() { place settings 900; step "tap settings-theme-default"; wait_ 500; step "tap settings-mode-light"; wait_ 700; }
goto() { step "tap bar-mark"; wait_ 600; step "tap \"launcher-open-$1\""; wait_ 900; }

# run_drive <name> <mode> <onboarding?> <agent flags...>: reset, run the collected steps, retry a refused launch up to 3 times.
run_drive() {
  name="$1"; mode="$2"; ob="$3"; shift 3
  cp "$STEPS" "$SCRATCH/steps.$name"
  attempt=1
  while :; do
    reset_state "$mode" "$ob"
    set -f; OLDIFS="$IFS"; IFS='
'
    # shellcheck disable=SC2046
    "$HERE/exact.sh" agent ios "$@" $(cat "$SCRATCH/steps.$name") > "$SCRATCH/$name.txt" 2>&1
    rc=$?
    IFS="$OLDIFS"; set +f
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$SCRATCH/$name.txt" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
    fi
    break
  done
  : > "$STEPS"
  cat "$SCRATCH/$name.txt" >> "$OUT"
  cp "$SCRATCH/$name.txt" "$SHOTS/drive-$name.txt" 2>/dev/null
  [ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/$name.txt" | tail -15; fail "drive $name (exit $rc)"; }
  echo "drive $name ok"
}

FAILS=""
must() { if grep -qF -- "$2" "$OUT"; then echo "ok: $1"; else echo "MISSING: $1: $2"; FAILS="$FAILS [$1]"; fi; }
nomust() { if grep -qF -- "$2" "$OUT"; then echo "UNWANTED: $1: $2"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi; }
