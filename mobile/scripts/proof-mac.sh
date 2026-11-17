#!/bin/sh
# Rerunnable proof of the Mac app, driven through Exact's agent over the macOS host (no simulator, no person), in demo mode (bundled
# fixtures, no network). What a script can check, in one drive of a fresh app:
#   1. it launched on the Autoradio page (radio-page, radio-play), and Autopaper, reached from the Places row, shows story rows;
#   2. Play was pressed and, in ~/Library/Application Support/Autoradio/console.log (the app writes its AUTORADIO lines there):
#      at least two `AUTORADIO on air ... samples=<n>` lines with n > 0, and no `AUTORADIO paused reason=` line;
#   3. the models registered: `AUTORADIO models registered n=<n>` with n > 0 (skipped with ALLOW_NO_MODELS=1);
#   4. quitting the app leaves no running process.
# It then prints what a person must check by ear. Last line: MAC PROOF OK, or MAC PROOF FAILED at <step> (then a `FAILED <reason>` line).
# The proof does not play through the speakers: it sets the default AutoradioProofSilent (the same as the launch argument
# `-AutoradioProofSilent YES`), which mutes the main mixer's output while samples are still rendered and counted. That is a test
# affordance for this script only, not a feature or a setting; the script restores the defaults it changed when it ends.
#   NOBUILD=1          skip the build (use the installed app)
#   ALLOW_NO_MODELS=1  the app has no model libraries: the model check is skipped, not failed
# Test ids: radio-page, radio-play, radio-place-autopaper, paper-story-* (docs/design/autoradio-page-2026-11-17.md).
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-mac.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
APP=news.choir.autoradio.lab
SUPPORT="$HOME/Library/Application Support/Autoradio"
CONSOLE="$SUPPORT/console.log"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-mac-proof-$(date +%H%M%S)}"
mkdir -p "$SHOTS"; echo "screenshots: $SHOTS"

prior() { /usr/bin/defaults read $APP "$1" 2>/dev/null || echo "__unset__"; }
PRIOR_DEMO=$(prior AutoradioDemo); PRIOR_SILENT=$(prior AutoradioProofSilent)
restore1() { if [ "$2" = "__unset__" ]; then /usr/bin/defaults delete $APP "$1" >/dev/null 2>&1; else /usr/bin/defaults write $APP "$1" -bool "$([ "$2" = 1 ] && echo YES || echo NO)"; fi; }
quit_app() { /usr/bin/osascript -e "tell application id \"$APP\" to quit" >/dev/null 2>&1; sleep 2; }
cleanup() { quit_app; restore1 AutoradioDemo "$PRIOR_DEMO"; restore1 AutoradioProofSilent "$PRIOR_SILENT"; rm -rf "$SCRATCH"; }
trap cleanup EXIT
fail() { cp "$OUT" "$SHOTS/drive.txt" 2>/dev/null; echo "MAC PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "FAILED $2"; exit 1; }

if [ -z "${NOBUILD:-}" ]; then
  echo "building (build-lab.sh mac takes the Mac build lock itself)"
  "$HERE/build-lab.sh" mac > "$SCRATCH/build.txt" 2>&1 || { tail -20 "$SCRATCH/build.txt"; fail "build" "the Mac build failed"; }
fi

# Why the proof taps Play itself: the station starts by itself only after onboarding has finished (Station.isReady), so a fresh
# state (onboarding.json removed below, as proof-common.sh does on the phone) never auto-starts and the one tap is the only start.
# A fresh app: quit, demo mode, light, silent output, no open pages from an earlier run.
quit_app
/usr/bin/defaults write $APP AutoradioDemo -bool YES
/usr/bin/defaults write $APP AutoradioThemeMode light
/usr/bin/defaults write $APP AutoradioProofSilent -bool YES
/usr/bin/defaults delete $APP AutoradioThemeID >/dev/null 2>&1
rm -f "$SUPPORT/open-pages.json" "$SUPPORT/onboarding.json"
# The first `agent macos` of a checkout builds its own copy of the app (its log: "building for macOS: there is no build yet", minutes
# long) and exits 1 when that outlasts the driver: that was attempt 1's refusal. A warm-up drive absorbs it, so the proof drive below is
# a real run; its result is not read.
echo "warm-up drive (builds the agent's copy once)"
"$HERE/exact.sh" agent macos "clock data" > "$SCRATCH/warm.txt" 2>&1 || "$HERE/exact.sh" agent macos "clock data" > "$SCRATCH/warm.txt" 2>&1 || true
quit_app
rm -f "$SUPPORT/open-pages.json"
BEFORE=$(wc -l < "$CONSOLE" 2>/dev/null || echo 0)

attempt=1
while :; do
  # The app opens on the Autoradio page (radio-page, with radio-play). Play is pressed on the page; Autopaper is reached from the page's
  # Places row (radio-place-autopaper) and must show story rows. The Autopaper step comes last so leaving the page cannot disturb the audio check.
  "$HERE/exact.sh" agent macos \
    "clock data" "clock +2500 real" "clock data" tree "screenshot $SHOTS/01-radio-page.png window" \
    "tap radio-play" "clock +25000 real" "clock data" tree "screenshot $SHOTS/02-after-play.png window" \
    "tap radio-place-autopaper" "clock +2500 real" "clock data" tree "screenshot $SHOTS/03-autopaper.png window" logs > "$OUT" 2>&1
  rc=$?
  [ $rc -eq 0 ] && break
  cp "$OUT" "$SHOTS/drive-attempt-$attempt.txt"
  echo "drive attempt $attempt failed (exit $rc); first lines:"; head -5 "$OUT"
  [ $attempt -ge 2 ] && fail "drive (exit $rc)" "$(grep -m1 -E 'no view matches|never opened|refused' "$OUT" || tail -3 "$OUT")"
  # Reset before a retry: a retry on the first attempt's state fails at the first tap.
  attempt=$((attempt + 1)); quit_app; rm -f "$SUPPORT/open-pages.json" "$SUPPORT/onboarding.json"; BEFORE=$(wc -l < "$CONSOLE" 2>/dev/null || echo 0); sleep 3
done
echo "launched: the app answered the agent"

grep -q "radio-page" "$OUT" && grep -q "radio-play" "$OUT" || fail "radio page" "the launch tree has no radio-page with radio-play: the app did not open on the Autoradio page"
echo "radio page: radio-page and radio-play present"
grep -q "paper-story-" "$OUT" || fail "autopaper" "no paper-story row after radio-place-autopaper: the Places row did not open Autopaper"
echo "autopaper: reached from the Places row, story rows present"

[ -f "$CONSOLE" ] || fail "console log" "$CONSOLE does not exist: the app did not write its console file"
tail -n +$((BEFORE + 1)) "$CONSOLE" > "$SHOTS/console-after-play.txt"
AIR=$(grep -cE "AUTORADIO on air .* samples=[1-9][0-9]*" "$SHOTS/console-after-play.txt")
if grep -q "AUTORADIO paused reason=" "$SHOTS/console-after-play.txt"; then
  fail "audio" "paused: $(grep -m1 'AUTORADIO paused reason=' "$SHOTS/console-after-play.txt")"
fi
if [ "$AIR" -lt 2 ]; then
  why=$(grep -E "AUTORADIO on air .*FAILED|AUTORADIO (ran dry|entry finished)|AUTORADIO .*(error|failed)" "$SHOTS/console-after-play.txt" | head -3 | tr '\n' ';')
  fail "audio" "${AIR:-0} 'on air ... samples=' line(s), need 2; ${why:-nothing else on the console} (see $SHOTS/console-after-play.txt)"
fi
echo "audio: $AIR blocks on air with samples, no pause"

if [ -n "${ALLOW_NO_MODELS:-}" ]; then echo "models: skipped (ALLOW_NO_MODELS=1)"
else
  grep -hE "AUTORADIO models registered n=[1-9]" "$CONSOLE" | tail -1 | grep -q . || fail "models" "no 'AUTORADIO models registered n=<n>' line with n > 0 in console.log: the model libraries are not linked or did not register"
  echo "models: $(grep -hE 'AUTORADIO models registered' "$CONSOLE" | tail -1 | cut -c1-120)"
fi

quit_app
if [ "$(/usr/bin/osascript -e "application id \"$APP\" is running" 2>/dev/null)" = "true" ]; then fail "quit" "the app is still running after quit"; fi
echo "quit: no process left"

cat <<'TXT'

MAC PROOF OK (the parts a script can check). A person must still check, by ear and by eye (this run was silent):
  1. Play: the radio speaks through the Mac's speakers, and Pause stops it at once.
  2. One voice at a time: speaking to it pauses the programme; no two voices overlap.
  3. The microphone: turning voice on shows the system permission prompt; after allowing it, the Lab trace shows a heard turn.
  4. Echo: with speakers (no headphones) the radio does not answer its own voice.
  5. Sign in: the approval page opens in the browser, and the app signs in.
  6. Reader and Texture: open a PDF or an EPUB in Files; edit a Texture page, quit, relaunch, and it is kept.
  7. Layout: resize the window from phone width to wide; nothing is cut off at 360 by 560.
TXT
