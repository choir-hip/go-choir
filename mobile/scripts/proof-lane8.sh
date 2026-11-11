#!/bin/sh
# Lane 8 proof: first run, on a simulator driven through Exact's agent socket, in demo mode (bundled fixtures, no network, no
# microphone, no model download). Shows, in light and dark: the welcome, voice setup (downloads with progress, one stall,
# one fallback chosen by the listener, one failed check that falls back by itself), the microphone asked in context with its
# live check, the hearing check, the interview (vague answer, an interruption, a too-loud pause and Continue, the read-back
# and a correction), the edition written (one desk failure and Retry), the edition page, the front page with the radio playing.
# Before the drive it runs the kit's tests for the interview and for setup (`swift test --filter`).
# Last line: PROOF OK, or PROOF FAILED at <step>. Screenshots: $PROOF_SHOTS.
#   NOBUILD=1   skip the build        NOTESTS=1   skip swift test
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane8.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
OUT2="$SCRATCH/drive2.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane8-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

# 1. The kit's tests (the failure lists of Interview.swift and VoiceSetup.swift), with the demo fixtures embedded first.
python3 -I "$HERE/embed-assets.py" >/dev/null || fail "embed assets"
if [ -z "${NOTESTS:-}" ]; then
  echo "swift test (InterviewTests, VoiceSetupTests)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter "InterviewTests|VoiceSetupTests|SpineTests" > "$SCRATCH/test.txt" 2>&1 || { grep "error:" "$SCRATCH/test.txt" | cut -c1-700; tail -8 "$SCRATCH/test.txt"; fail "swift test"; }
  grep -E "Executed [0-9]+ tests" "$SCRATCH/test.txt" | tail -1
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -40 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
SIM() { DEVELOPER_DIR=$DD /usr/bin/xcrun simctl "$@"; }
SIM privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-lane8-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"

fresh() { # <scheme>: demo mode on, the onboarding page opens (and the first-run state is reset) at launch
  SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioOpenOnboarding -bool YES
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode "$1"
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  if [ -n "$CONT" ]; then
    S="$CONT/Library/Application Support/Autoradio"
    rm -f "$S/open-pages.json" "$S/onboarding.json" "$S/interview.json" "$S/voice-setup.json" "$S/interview.md"
  fi
}

# A refused agent socket is retried (the first launch of a fresh install can miss it).
drive() { # <outfile> <ops...>
  of="$1"; shift
  attempt=1
  while :; do
    "$HERE/exact.sh" agent ios "$@" > "$of" 2>&1
    rc=$?
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$of" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
    fi
    return $rc
  done
}

W() { echo "clock +$1 real"; }

# 2. Drive one, light: the whole path.
fresh light
drive "$OUT" \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-welcome.png" \
  "tap ob-begin" "clock +1200 real" "clock data" tree "screenshot $SHOTS/02-setup-downloading.png" \
  "clock +3500 real" "clock data" tree "screenshot $SHOTS/03-setup-stalled.png" \
  "tap ob-fallback-transcriber" "clock +900 real" "clock data" tree "screenshot $SHOTS/04-setup-fallback.png" \
  "clock +7500 real" "clock data" tree "screenshot $SHOTS/05-mic-ask.png" \
  "tap ob-mic-ask" "clock +350 real" "clock data" tree "screenshot $SHOTS/06-mic-checking.png" \
  "clock +1800 real" "clock data" tree "screenshot $SHOTS/07-mic-heard.png" \
  "tap ob-mic-next" "clock +600 real" "clock data" tree "screenshot $SHOTS/08-hearing.png" \
  "tap ob-hearing-yes" "clock +600 real" "clock data" tree "screenshot $SHOTS/09-ready.png" \
  "tap ob-interview" "clock +800 real" "clock data" tree "screenshot $SHOTS/10-interview-q1.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/11-vague-offer.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/12-interrupted.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/13-paused-loud.png" \
  "tap ob-continue" "clock +700 real" "clock data" tree "screenshot $SHOTS/14-continued.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" \
  "tap ob-demo-next" "clock +1200 real" "clock data" \
  "tap ob-demo-next" "clock +1200 real" "clock data" \
  "tap ob-demo-next" "clock +1200 real" "clock data" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/15-readback.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/16-readback-corrected.png" \
  "tap ob-demo-next" "clock +900 real" "clock data" tree "screenshot $SHOTS/17-writing.png" \
  "clock +3200 real" "clock data" tree "screenshot $SHOTS/18-writing-failed.png" \
  "tap ob-edition-retry" "clock +1200 real" "clock data" tree "screenshot $SHOTS/19-writing-retry.png" \
  "clock +3500 real" "clock data" tree "screenshot $SHOTS/20-edition-done.png" \
  "tap ob-edition-open" "clock +900 real" "clock data" tree "screenshot $SHOTS/21-edition-texture.png" \
  "tap bar-pages" "clock +600 real" "clock data" "tap pile-places" "clock +600 real" "clock data" \
  "tap launcher-open-onboarding:setup" "clock +700 real" "clock data" \
  "tap ob-start-radio" "clock +1800 real" "clock data" tree "screenshot $SHOTS/22-front-page-playing.png" \
  logs
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -15; fail "drive 1 (exit $rc)"; }

# 3. Drive two, dark: welcome, setup (stall, fallback), microphone, hearing, interview start, the pause.
fresh dark
drive "$OUT2" \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/30-dark-welcome.png" \
  "tap ob-begin" "clock +4700 real" "clock data" tree "screenshot $SHOTS/31-dark-setup-stalled.png" \
  "tap ob-fallback-transcriber" "clock +8400 real" "clock data" tree "screenshot $SHOTS/32-dark-mic-ask.png" \
  "tap ob-mic-ask" "clock +2300 real" "clock data" tree "screenshot $SHOTS/33-dark-mic-heard.png" \
  "tap ob-mic-next" "clock +600 real" "clock data" tree "screenshot $SHOTS/34-dark-hearing.png" \
  "tap ob-hearing-yes" "clock +600 real" "clock data" "tap ob-interview" "clock +800 real" "clock data" \
  "tap ob-demo-next" "clock +1200 real" "clock data" "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/35-dark-interrupted.png" \
  "tap ob-demo-next" "clock +1200 real" "clock data" tree "screenshot $SHOTS/36-dark-paused.png" \
  logs
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT2" | tail -15; cp "$OUT2" "$OUT"; fail "drive 2 (exit $rc)"; }

must() { # <step> <fixed text> [file]
  f="${3:-$OUT}"
  grep -qF -- "$2" "$f" || fail "$1" "missing: $2"
  echo "ok: $1"
}
must "welcome: the page and its heading" '[page-title] "Make it yours"'
must "welcome: begin and skip" '[ob-begin]'
must "setup: the heading" '[page-title] "Set up your voice"'
must "setup: a row per model" '[ob-row-transcriber]'
must "setup: all four stages" '[ob-row-decision]'
must "setup: honest progress text" 'Downloading: '
must "setup: a stalled row says so" 'No progress for'
must "setup: the stalled row offers retry and the fallback" '[ob-fallback-transcriber]'
must "setup: a failed check falls back by itself" 'wrong length'
must "mic: asked in context" '[ob-mic-ask]'
must "mic: the live check" 'Say anything.'
must "mic: the words appear" '[ob-heard]'
must "hearing: the route" '[ob-route]'
must "hearing: the check" '[ob-hearing-yes]'
must "ready: start the interview" '[ob-interview]'
must "interview: the microphone is open" '[ob-listening]'
must "interview: the first question" 'every day'
must "interview: the vague answer offers sections" 'Today'
must "interview: an interruption is marked" 'cutting in'
must "interview: a loud room pauses it on its card" '[ob-paused]'
must "interview: continue" '[ob-continue]'
must "interview: the read-back" 'Here is what I have'
must "interview: the correction is read back again" 'I changed that'
must "edition: the alive wait slot" '[ob-wait-slot]'
must "edition: the desk failure is kept and retryable" '[ob-edition-retry]'
must "edition: shown" '[ob-edition]'
must "edition: open as a page" '[ob-edition-open]'
must "finish: start the radio" '[ob-start-radio]'
must "front page playing: the bar shows what plays" '[bar-text]'
must "dark: welcome" '[ob-begin]' "$OUT2"
must "dark: the pause" '[ob-paused]' "$OUT2"
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 28 ] || fail "screenshots" "fewer than 28 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT" "$OUT2"; then echo "note: errors in the app log:"; grep -h -m5 'ERROR\|Fatal error' "$OUT" "$OUT2"; fi
echo "PROOF OK"
