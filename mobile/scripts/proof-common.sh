#!/bin/sh
# Shared helpers for the whole-app proof (sourced by proof-app.sh; the lane proofs keep their own copies).
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

# A clean app: demo mode, Depth, the scheme given (default light), no open pages, no first-run flags, default settings.
reset_state() { # [light|dark] [onboarding]
  SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sleep 2
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode "${1:-light}"
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
  if [ "${2:-}" = onboarding ]; then SIM spawn "$EXACT_SIM" defaults write $APP AutoradioOpenOnboarding -bool YES
  else SIM spawn "$EXACT_SIM" defaults delete $APP AutoradioOpenOnboarding >/dev/null 2>&1; fi
  for k in talkMode swipeLeft swipeRight; do SIM spawn "$EXACT_SIM" defaults delete $APP "AutoradioSetting.$k" >/dev/null 2>&1; done
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  if [ -n "$CONT" ]; then
    S="$CONT/Library/Application Support/Autoradio"
    rm -f "$S/open-pages.json" "$S/onboarding.json" "$S/interview.json" "$S/voice-setup.json" "$S/interview.md" "$S/signals.jsonl" "$S/recent-files.json"
    rm -rf "$S/themes" "$CONT/Library/Caches/Autoradio/files"
  fi
}

# Steps are collected one per line and handed to the driver as separate arguments (a step may hold spaces).
step() { printf '%s\n' "$1" >> "$STEPS"; }
wait_() { step "clock +${1:-700} real"; step "clock data"; }
shot() { step "screenshot $SHOTS/$1.png"; }
view() { wait_ "${2:-800}"; step tree; shot "$1"; }   # settle, dump the tree, screenshot <name>
place() { step "tap bar-pages"; wait_ 600; step "tap pile-places"; wait_ 600; step "tap place-$1"; wait_ "${2:-900}"; }
goto() { step "tap bar-pages"; wait_ 600; step "tap pile-places"; wait_ 600; step "tap \"launcher-open-$1\""; wait_ 900; }

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
