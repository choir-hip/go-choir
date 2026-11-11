#!/bin/sh
# End-to-end proof of the spine on a simulator, driven through Exact's agent socket (no human, no device), in demo mode
# (bundled fixtures, no network, no microphone): the glass bar over a full-bleed page; two pages open and the pile shows
# two cards; switch; one closes by swipe; the theme switches (Depth, Newsprint, dark); play/pause and a hold-to-talk on
# the bar; the Components page (a talk turn through the injected fixture path, with the stub stages) and the Trace page.
# Screenshots go to a directory the script prints (PROOF_SHOTS=dir to choose). Last line: PROOF OK, or PROOF FAILED at <step>.
# Elements are found by test id, never by position.
#   NOBUILD=1      skip the build (use the installed app)
#   EXACT_SIM=...  a simulator name or udid; default: a simulator of our own (scripts/sim-device.sh)
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-sim.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-sim-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"

if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -20 "$SCRATCH/build.txt"; fail "build"; }
fi

# One drive, many operations. `tap start` then the first frame: the app opens its agent socket only after its first
# frame, in the foreground (Exact: AgentFrontIOS.swift:29, ExactIOS/main.swift:134), and the driver waits 20 s for it
# (agent.mjs:567): a cold launch on a loaded machine misses that, so a refused socket is retried.
# A permission alert (the speech recognizer asks at its first use) takes the foreground from the app: the driver then
# reads "scene not foreground-active" and a later launch never opens its socket. Grant everything first; that also
# clears an alert left by an earlier run.
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all news.choir.autoradio.lab >/dev/null 2>&1

# Fresh state: demo mode on, light scheme, Depth, no open pages from an earlier run.
DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
[ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

# The first launch of a fresh install can miss the agent socket; a refused socket is retried.
attempt=1
while :; do
  "$HERE/exact.sh" agent ios --touch platform \
    "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-paper.png" \
    "tap paper-story-demo-grid-storage" "clock +800 real" "clock data" tree "screenshot $SHOTS/02-story-page.png" \
    "tap bar-pages" "clock +600 real" "clock data" tree "screenshot $SHOTS/03-pile-two-cards.png" \
    "tap pile-places" "clock +600 real" "clock data" "tap launcher-open-paper:front" "clock +600 real" "clock data" tree "screenshot $SHOTS/03b-switched-to-paper.png" \
    "tap bar-pages" "clock +600 real" "clock data" \
    "tap card-swipe-paper:front drag -150 0" "clock +800 real" "clock data" tree "screenshot $SHOTS/04-pile-after-swipe.png" \
    "tap pile-places" "clock +600 real" "clock data" tree "screenshot $SHOTS/05-launcher.png" \
    "tap place-settings" "clock +600 real" "clock data" tree "screenshot $SHOTS/06-settings-depth.png" \
    "tap settings-spike3d" "clock +900 real" "clock data" tree "screenshot $SHOTS/06b-spike-3d.png" \
    "tap settings-theme-newsprint" "clock +600 real" "clock data" tree "screenshot $SHOTS/07-settings-newsprint.png" \
    "tap settings-mode-dark" "clock +800 real" "clock data" tree "screenshot $SHOTS/08-newsprint-dark.png" \
    "tap settings-theme-depth" "clock +600 real" "clock data" tree "screenshot $SHOTS/09-depth-dark.png" \
    "tap settings-mode-light" "clock +800 real" "clock data" tree "screenshot $SHOTS/10-depth-light.png" \
    "tap bar-pages drag 0 -200" "clock +900 real" "clock data" tree "screenshot $SHOTS/10b-pull-launcher.png" \
    "tap place-settings" "clock +600 real" "clock data" \
    "tap bar-voice down" "tap up" "clock +1500 real" "clock data" tree "screenshot $SHOTS/11-playing.png" \
    logs > "$OUT" 2>&1
  rc=$?
  # Second drive, plain taps (under --touch platform a held press is one scripted touch): hold to talk, Components, Trace.
  # The open pages and the theme are kept from the first drive.
  [ $rc -eq 0 ] && "$HERE/exact.sh" agent ios \
    "clock data" "clock +1500 real" "clock data" \
    "tap bar-field" "clock +800 real" "clock data" "screenshot $SHOTS/12a-bar-field-focused.png window" \
    "tap bar-voice down" "clock +900 real" "clock data" tree "screenshot $SHOTS/12-talking.png" "tap up" "clock +800 real" "clock data" \
    "tap settings-components" "clock +700 real" "clock data" tree "screenshot $SHOTS/13-components.png" \
    "tap stage-addressing-hybrid" "tap stage-transcriber-scripted" "tap stage-system1-fixed" "tap stage-holder-fixed-lines" \
      "tap stage-desk-stub-echo" "tap stage-programme-fixture" "tap ack-immediate" "clock data" \
    "tap start" "clock +1500 real" "clock data" tree \
    "tap lab-talk down" "clock +700 real" "clock data" tree "tap up" "clock +1500 real" \
    "tap inject" "clock +3000 real" "clock data" tree \
    "clock +9000 real" "clock data" "tap open-trace" "clock +700 real" "clock data" tree "screenshot $SHOTS/14-trace.png" \
    logs >> "$OUT" 2>&1
  rc=$?
  if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
    attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
  fi
  break
done
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -15; fail "drive (exit $rc)"; }

# Assertions on the one transcript. The tree lists every tab's panel in a fixed order, so the order of lines inside
# one tree says nothing: each text must simply appear, and each is a text that exists only after its step.
must() { # <step name> <fixed text>
  grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"
  echo "ok: $1"
}

must "demo mode: the paper page under the bar" '[page-title] "AUTOPAPER"'
must "demo mode: a fixture story on the front page" 'The harbor council approves a tide barrier'
must "bar: floats over the page" '[bar] (pan'
must "bar: pages button, prompt, voice button" '[bar-pages]'
must "bar: prompt field with the aim" 'label="Ask about this page'
must "bar: voice button" '[bar-voice]'
must "open a page: the story is a page" '[page-title] "A regional grid adds its largest battery yet"'
must "open a page: the bar counts two" '[bar-pages-count] "2"'
must "pile: a card per page (story)" '[card-pub:demo-grid-storage]'
must "pile: a card per page (paper)" '[card-paper:front]'
must "switch: the launcher's open chip brings the first page forward" '[page-title] "AUTOPAPER"'
must "close by swipe: the count is back to one" '[bar-pages-count] "1"'
must "launcher: places" '[place-settings]'
must "settings: themes and modes" '[settings-theme-newsprint]'
must "theme switch: Newsprint" 'Showing Newsprint (light)'
must "theme switch: dark" 'Showing Newsprint (dark)'
must "theme switch: Depth dark" 'Showing Depth (dark)'
must "theme switch: back to Depth light" 'Showing Depth (light)'
must "spike 3D: rotate about y with perspective is accepted and applied" 'label="Rotated 50 degrees about y"'
must "pull the bar up: the launcher opens" '[launcher-layer]'
must "play: tap on the voice button starts the radio (bar shows what plays)" '[bar-text]'
must "hold to talk: the bar shows the listening state" 'Listening'
must "components: reachable as a page" '[page-title] "Components"'
must "components: talk control" '[lab-talk]'
must "components: acknowledgement timing choice" '[ack-immediate]'
must "components: stub configuration selected explicitly" '"● System 1'
must "components: audio state line" '[audio-state]'
must "components: policy choices" '[policy-alwaysTalking]'
must "components: inject control" '[inject]'
must "trace: reachable as a page" '[page-title] "Trace"'
must "trace: export control" '[trace-export]'
must "start audio: listening claim" 'State listening'
must "start audio: virtual engine" 'engine virtual'
must "start audio: microphone on" 'Phase: playing'
must "hold to talk (lab): phase leaves playing" 'Phase: addressed'
must "audio switch events recorded" 'audio switch listening -> talking in'
grep -qE '\[trace-id\] "turn-' "$OUT" || fail "trace: turn record" "no live turn record in Trace"
echo "ok: trace: live turn record"
grep -qE 'Acknowledgement [0-9]+ ms' "$OUT" || fail "trace: acknowledgement time" "no acknowledgement time in the record"
echo "ok: trace: acknowledgement time"
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 12 ] || fail "screenshots" "fewer than 12 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
