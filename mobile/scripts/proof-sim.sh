#!/bin/sh
# End-to-end proof of the spine on a simulator, driven through Exact's agent socket (no human, no device), in demo mode
# (bundled fixtures, no network, no microphone): the glass bar over a full-bleed page; the Autoradio page first (the first fixed page),
# then the launcher (the mark) opens Paper and a story; the launcher lists both; one is removed; the theme switches (Depth, Newsprint, dark).
# No audio is started: the bar's play button is only looked for. (The Components and Trace pages are deleted.)
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
    "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-radio-first.png" \
    "tap bar-mark" "clock +600 real" "clock data" tree "screenshot $SHOTS/01a-launcher.png" \
    "tap place-paper" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01b-paper.png" \
    "tap paper-story-demo-grid-storage" "clock +800 real" "clock data" tree "screenshot $SHOTS/02-story-page.png" \
    "tap bar-mark" "clock +600 real" "clock data" tree "screenshot $SHOTS/03-launcher-two-pages.png" \
    "tap launcher-open-paper:front" "clock +800 real" "clock data" tree "screenshot $SHOTS/03b-switched-to-paper.png" \
    "tap bar-mark" "clock +600 real" "clock data" "tap launcher-close-pub:demo-grid-storage" "clock +800 real" "clock data" tree "screenshot $SHOTS/04-after-close.png" \
    "tap place-settings" "clock +600 real" "clock data" tree "screenshot $SHOTS/06-settings-depth.png" \
    "tap settings-theme-newsprint" "clock +600 real" "clock data" tree "screenshot $SHOTS/07-settings-newsprint.png" \
    "tap settings-mode-dark" "clock +800 real" "clock data" tree "screenshot $SHOTS/08-newsprint-dark.png" \
    "tap settings-theme-depth" "clock +600 real" "clock data" tree "screenshot $SHOTS/09-depth-dark.png" \
    "tap settings-mode-light" "clock +800 real" "clock data" tree "screenshot $SHOTS/10-depth-light.png" \
    "tap bar-mark" "clock +900 real" "clock data" tree "screenshot $SHOTS/10b-launcher.png" \
    "clock +300 real" "clock data" tree "screenshot $SHOTS/11-bar.png" \
    logs > "$OUT" 2>&1
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

must "the app opens on the Autoradio page (the first fixed page)" '[radio-play]'
must "bar: the mark, the field, the microphone, play" '[bar-mark]'
must "bar: prompt field" '[bar-field]'
must "bar: voice button" '[bar-mic]'
must "launcher: the mark opens it with the places" '[place-paper]'
must "demo mode: the paper page under the bar" '[page-title] "AUTOPAPER"'
must "demo mode: a fixture story on the front page" 'The harbor council approves a tide barrier'
must "open a page: the story is a page" '[page-title] "A regional grid adds its largest battery yet"'
must "launcher: the story is in the deck" '[launcher-open-pub:demo-grid-storage]'
must "launcher: the paper is in the deck" '[launcher-open-paper:front]'
must "launcher: a remove button on the opened story" '[launcher-close-pub:demo-grid-storage]'
must "launcher: places" '[place-settings]'
must "settings: themes and modes" '[settings-theme-newsprint]'
must "theme switch: Newsprint" 'Showing Newsprint (light)'
must "theme switch: dark" 'Showing Newsprint (dark)'
must "theme switch: Depth dark" 'Showing Depth (dark)'
must "theme switch: back to Depth light" 'Showing Depth (light)'
must "launcher: the mark opens the layer" '[launcher-layer]'
must "bar: play is there (this proof starts no audio)" '[bar-play]'
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 11 ] || fail "screenshots" "fewer than 11 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
