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
place() { step "clock settle"; step "tap bar-mark"; wait_ 600; step "clock settle"; step "tap place-$1"; wait_ "${2:-900}"; }
# Put the app in the default theme, light, by its own Settings controls (the proof never trusts what an earlier run left).
ensure_light() { place settings 900; step "tap settings-theme-default"; wait_ 500; step "tap settings-mode-light"; wait_ 700; }
goto() { step "clock settle"; step "tap bar-mark"; wait_ 600; step "clock settle"; step "tap \"launcher-open-$1\""; wait_ 900; }

# layout_scroll <drive-log> <testId> [mid_y]: how many points to wheel the page's scroller so the node's middle sits at mid_y (default 320, clear of the bar), read from the `layout <testId>` step's output in
# the drive log: the viewport's height (its first line) and the node's own "space viewport X,Y WxH". Prints a whole number (0 when the node is already at or above mid_y). Prints nothing and returns 1 when the log
# has no such node block, or it has no viewport space, or the middle is farther than 30000 points below the first screen (a bound, not a guess): the caller says so and fails.
layout_scroll() {
  awk -v id="$2" -v want="${3:-320}" '
    /^viewport [0-9]+/ { split($2, v, "×"); vh = v[2] + 0 }
    /^node #/ { on = (index($0, "[" id "]") > 0); next }
    on && /^  space viewport / { split($3, p, ","); split($4, d, "×"); mid = p[2] + d[2] / 2; found = 1; on = 0 }
    END { if (!found || vh == 0 || mid > 30000) exit 1; n = int(mid - want + 0.5); print (n > 0 ? n : 0) }
  ' "$1"
}

# run_drive <name> <mode> <onboarding?> <agent flags...>: reset, run the collected steps, retry a refused launch up to 3 times.
run_drive() {
  name="$1"; mode="$2"; ob="$3"; shift 3
  cp "$STEPS" "$SCRATCH/steps.$name"
  attempt=1; relaunched=0
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
    # The dispatch log did not answer within 1 s of a real touch (the app's main thread was busy: Exact's touches.mjs, "realTap" barrier). Exact stops the app's carrier and the touch runner, so the one tap
    # cannot be played again in the same drive: the drive is run again from a clean app, once, and the retry is logged and counted ("retry: <op>", and in the summary). A second timeout fails.
    if [ $rc -ne 0 ] && [ $relaunched -eq 0 ] && grep -q "the dispatch log did not answer within 1 s of the touch" "$SCRATCH/$name.txt"; then
      relaunched=1
      op=$(grep -o 'op [0-9]*/[0-9]* `[^`]*`' "$SCRATCH/$name.txt" | head -1)
      echo "retry: $name drive, $op"
      echo "$name: $op" >> "$SHOTS/retries.txt"
      SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
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
