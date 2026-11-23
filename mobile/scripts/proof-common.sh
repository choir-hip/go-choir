#!/bin/sh
# Shared helpers for the real-data proof (sourced by proof-real.sh).
# Needs HERE (the scripts directory) and SHOTS set by the caller. Provides: fail, SIM, reset_state, step/wait_/shot/view/place/goto,
# run_drive (a drive that starts from a clean app, retried when the agent socket is refused), must, nomust.
DD=/Applications/Xcode.app/Contents/Developer
APP="$("$HERE/app-identity.sh" id)" || exit 1   # AR_VARIANT / AR_APP_ID choose the second app
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

# blocks_have_text <tree-file> [n]: the reader's text is really on the page. For each of the first n (default 2) `reader-block-b<N>` lines of a tree dump, the block must carry a text of 20 or more characters: on its own line
# (`[reader-block-b2] "..."`, the old shape) or in a child line, deeper than the block and before the next line at its depth or less (`Text#859 "..."`, one per sentence run). A block with no such text fails, so an
# empty block cannot pass; fewer than n blocks fails too. Prints the failing block's id and returns 1.
blocks_have_text() {
  awk -v n="${2:-2}" '
    function indent(l) { match(l, /^ */); return RLENGTH }
    function close_block() { if (cur != "" && seen <= n && !ok) { print "no text in " cur; bad = 1 } cur = "" }
    match($0, /\[reader-block-b[0-9]+\]/) { close_block(); seen++; cur = substr($0, RSTART + 1, RLENGTH - 2); ind = indent($0); ok = ($0 ~ /\] "[^"]{20,}/); next }
    cur != "" && indent($0) <= ind { close_block() }
    cur != "" && $0 ~ /Text#[0-9]+ "[^"]{20,}/ { ok = 1 }
    END { close_block(); if (seen < n) { print "only " seen " blocks"; bad = 1 } exit bad }
  ' "$1"
}

DRIVES_FAILED=""
# run_drive <name> <mode> <onboarding?> <agent flags...>: reset, run the collected steps, retry a refused launch up to 3 times.
run_drive() {
  name="$1"; mode="$2"; ob="$3"; shift 3
  drive_started=$(date '+%a %-d %b')
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
    # A settle that gave up on a slow launch's requests in the first few ops (`clock settle: TimerFireLimit`: the radio's timers ran out before the requests of a launch that took 41 s came back; train2p1's
    # drill died so at op 4/79, the app stayed signed in and every later signed-out drive was a signed-in app) changes nothing yet: the drive is run again once from a clean app, logged and counted. Later, or twice, it fails.
    if [ $rc -ne 0 ] && [ $relaunched -eq 0 ] && grep -q "TimerFireLimit" "$SCRATCH/$name.txt"; then
      op=$(grep -o "op [0-9]*/[0-9]* ['\`][^'\`]*['\`]" "$SCRATCH/$name.txt" | head -1)
      opn=$(printf '%s' "$op" | sed -n 's/^op \([0-9]*\)\/.*/\1/p')
      if [ -n "$opn" ] && [ "$opn" -le 8 ]; then
        relaunched=1
        echo "retry: $name drive, $op"
        echo "$name: $op" >> "$SHOTS/retries.txt"
        SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
      fi
    fi
    break
  done
  : > "$STEPS"
  cat "$SCRATCH/$name.txt" >> "$OUT"
  cp "$SCRATCH/$name.txt" "$SHOTS/drive-$name.txt" 2>/dev/null
  # The date the app showed is today's date WHEN THE DUMP WAS TAKEN, not when the check runs (a run across local midnight). The drive's start and its end both: the dump is between them.
  echo "$drive_started" > "$SHOTS/drive-$name.date"; date '+%a %-d %b' >> "$SHOTS/drive-$name.date"
  if [ $rc -ne 0 ]; then
    grep -v "^warning" "$SCRATCH/$name.txt" | tail -15
    # PROOF_KEEP_GOING=1: every drive starts a fresh app, so a failed drive is recorded and the next one runs; the end prints every name. Without it the first failure stops the proof.
    [ "${PROOF_KEEP_GOING:-}" = 1 ] || fail "drive $name (exit $rc)"
    DRIVES_FAILED="$DRIVES_FAILED $name"; echo "drive $name FAILED (exit $rc)"; return 0
  fi
  echo "drive $name ok"
}

FAILS=""
must() { if grep -qF -- "$2" "$OUT"; then echo "ok: $1"; else echo "MISSING: $1: $2"; FAILS="$FAILS [$1]"; fi; }
# A negative check over a missing or empty file proves nothing (a drive that wrote nothing would pass every one): it fails.
nomust() { if [ ! -s "$OUT" ]; then echo "MISSING: $1: the drive output is missing or empty"; FAILS="$FAILS [$1]"; elif grep -qF -- "$2" "$OUT"; then echo "UNWANTED: $1: $2"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi; }
# chk/bad are defined HERE, before any drive: the first-run drive uses them, and a check called before its definition is "command not found" and silently no check at all
chk() { # name pattern file
  if grep -qE -- "$2" "$3"; then echo "ok: $1"; else echo "MISSING: $1"; FAILS="$FAILS [$1]"; fi
}
bad() { # name pattern file
  if [ ! -s "$3" ]; then echo "MISSING: $1: $3 is missing or empty, so there is nothing to check"; FAILS="$FAILS [$1]"; return 0; fi
  if grep -qE -- "$2" "$3"; then echo "UNWANTED: $1: $(grep -m1 -E -- "$2" "$3" | cut -c1-160)"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi
}
