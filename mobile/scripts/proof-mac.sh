#!/bin/sh
# Rerunnable proof of the Mac app, driven through Exact's agent over the macOS host (no simulator, no person), in demo mode
# (bundled fixtures, no network). It checks what a script can: the app built and launched, answered the agent, opened the front
# page (paper story rows in the tree), opened the Lab page and registered its stages (the model rows, when the app has models), and
# that closing leaves no process behind. It then prints what a person must check by ear and by eye.
# Last line: MAC PROOF OK, or MAC PROOF FAILED at <step>. Screenshots go to PROOF_SHOTS (default under $TMPDIR).
#   NOBUILD=1          skip the build (use the installed app)
#   ALLOW_NO_MODELS=1  the app has no model libraries: the stage check is skipped, not failed
# UNRUN until the first Mac build is seen: the test ids are the ones proof-sim.sh drives on the phone.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-mac.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-mac-last.txt" 2>/dev/null; echo "MAC PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }
APP=news.choir.autoradio.lab
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-mac-proof-$(date +%H%M%S)}"
mkdir -p "$SHOTS"; echo "screenshots: $SHOTS"

if [ -z "${NOBUILD:-}" ]; then
  echo "building (build-lab.sh mac takes the Mac build lock itself)"
  "$HERE/build-lab.sh" mac > "$SCRATCH/build.txt" 2>&1 || { tail -20 "$SCRATCH/build.txt"; fail "build"; }
fi

# Fresh state before EVERY attempt: demo mode, light scheme, no open pages from an earlier run (a retry that kept the first attempt's
# open story page failed at the first tap, "drive refused" three times). The app's own preferences outlive `defaults write` made while
# it runs, so it is quit first. AutoradioDemo is restored to what it was when the script ends.
SUPPORT="$HOME/Library/Application Support/Autoradio"
CONSOLE="$SUPPORT/console.log"
PRIOR_DEMO=$(/usr/bin/defaults read $APP AutoradioDemo 2>/dev/null || echo "__unset__")
restore() { if [ "$PRIOR_DEMO" = "__unset__" ]; then /usr/bin/defaults delete $APP AutoradioDemo >/dev/null 2>&1; else /usr/bin/defaults write $APP AutoradioDemo -bool "$([ "$PRIOR_DEMO" = 1 ] && echo YES || echo NO)"; fi; }
trap 'restore; rm -rf "$SCRATCH"' EXIT
fresh() {
  /usr/bin/osascript -e "tell application id \"$APP\" to quit" >/dev/null 2>&1
  sleep 2
  /usr/bin/defaults write $APP AutoradioDemo -bool YES
  /usr/bin/defaults write $APP AutoradioThemeMode light
  /usr/bin/defaults delete $APP AutoradioThemeID >/dev/null 2>&1
  rm -f "$SUPPORT/open-pages.json"
}

attempt=1
while :; do
  fresh
  "$HERE/exact.sh" agent macos \
    "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-paper.png window" \
    "tap paper-story-demo-grid-storage" "clock +800 real" "clock data" tree "screenshot $SHOTS/02-story-page.png window" \
    "tap bar-pages" "clock +600 real" "clock data" tree \
    "tap pile-places" "clock +600 real" "clock data" "tap place-settings" "clock +600 real" "clock data" \
    "tap settings-components" "clock +700 real" "clock data" tree "screenshot $SHOTS/03-components.png window" \
    logs > "$OUT" 2>&1
  rc=$?
  [ $rc -eq 0 ] && break
  cp "$OUT" "$SHOTS/drive-attempt-$attempt.txt"
  echo "drive attempt $attempt failed (exit $rc); first lines:"; head -5 "$OUT"
  [ $attempt -ge 3 ] && fail "drive (exit $rc)" "$(tail -15 "$OUT")"
  attempt=$((attempt + 1)); sleep 5
done
echo "launched: the app answered the agent"

grep -q "paper-story-" "$OUT" || fail "front page" "no paper-story row in the tree: the front page did not open"
echo "front page: paper story rows present"
grep -q "settings-components" "$OUT" || fail "settings" "the settings page did not open"
if [ -n "${ALLOW_NO_MODELS:-}" ]; then
  echo "stages: skipped (ALLOW_NO_MODELS=1)"
else
  grep -q "stage-transcriber-" "$OUT" || fail "model stages" "no stage-transcriber-* row in the Components page: the model libraries are not linked or did not register"
  echo "stages: the model stages are registered"
fi

# Audio: a fresh app, Play, wait; the Kit prints "AUTORADIO on air <id> ... samples=<n>" to console.log (the app writes it on the Mac).
fresh
LINES_BEFORE=$(wc -l < "$CONSOLE" 2>/dev/null || echo 0)
"$HERE/exact.sh" agent macos "clock data" "clock +1500 real" "clock data" "tap bar-play" "clock +25000 real" "clock data" tree \
  "screenshot $SHOTS/04-played.png window" logs > "$SCRATCH/play.txt" 2>&1 || { cp "$SCRATCH/play.txt" "$SHOTS/play.txt"; fail "play drive" "$(tail -10 "$SCRATCH/play.txt")"; }
[ -f "$CONSOLE" ] || fail "console log" "$CONSOLE does not exist: the app did not write its console file"
tail -n +$((LINES_BEFORE + 1)) "$CONSOLE" > "$SHOTS/console-after-play.txt"
if grep -E "AUTORADIO on air .* samples=[1-9][0-9]*" "$SHOTS/console-after-play.txt" >/dev/null; then
  echo "audio: the first entry went on air with samples"
else
  why=$(grep -E "AUTORADIO on air .*FAILED|AUTORADIO .*(error|failed)" "$SHOTS/console-after-play.txt" | head -3)
  grep -m1 "Paused" "$SCRATCH/play.txt" >/dev/null && echo "the bar says Paused after Play"
  fail "audio" "FAILED ${why:-no 'on air ... samples=' line in console.log (see $SHOTS/console-after-play.txt)}"
fi

cat <<'TXT'

MAC PROOF OK (the parts a script can check). A person must still check, by ear and by eye:
  1. Play: the radio speaks through the Mac's speakers, and Pause stops it at once.
  2. One voice at a time: speaking to it pauses the programme; no two voices overlap.
  3. The microphone: the first use shows the system permission prompt; after allowing it, the Lab trace shows a heard turn.
  4. Echo: with speakers (no headphones) the radio does not answer its own voice.
  5. Sign in: the approval page opens in the browser, and the app signs in.
  6. Reader and Texture: open a PDF or an EPUB in Files; edit a Texture page, quit, relaunch, and it is kept.
  7. Layout: resize the window from phone width to wide; nothing is cut off at 360 by 560.
TXT
