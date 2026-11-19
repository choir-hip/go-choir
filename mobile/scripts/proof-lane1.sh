#!/bin/sh
# Lane 1 proof (bar, radio, talk), on a simulator of its own in demo mode, driven by test id.
# Swift tests first (BarStateTests), then the build, then three drives:
#   A light: idle, typing and send, play from the bar, the expanded player (follow-along, jump, speed, pause), the paused bar,
#            the bar's Voice control (a tap)
#   B dark:  the same bar states in dark
#   C light: a scripted turn through the Components page (stub stages) with the bar visible, shots at intervals
# Screenshots go to $PROOF_SHOTS. Last line: PROOF OK or PROOF FAILED at <step>. NOBUILD=1 skips the build.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane1.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
: > "$OUT"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane1-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

if "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter BarStateTests > "$SCRATCH/swift.txt" 2>&1; then
  echo "ok: swift tests (BarStateTests)"
else tail -30 "$SCRATCH/swift.txt"; fail "swift tests"; fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -40 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
sim() { DEVELOPER_DIR=$DD /usr/bin/xcrun simctl "$@"; }
sim privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-lane1-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"

fresh() { # <light|dark>
  sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode "$1"
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
  CONT=$(sim get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  [ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
  sim privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
}

# run_drive <name> <ops...>: one drive; a refused agent socket is retried (as proof-sim.sh does)
run_drive() {
  name="$1"; shift
  attempt=1
  while :; do
    "$HERE/exact.sh" agent ios "$@" > "$SCRATCH/$name.txt" 2>&1
    rc=$?
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$SCRATCH/$name.txt" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
    fi
    break
  done
  cat "$SCRATCH/$name.txt" >> "$OUT"
  [ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/$name.txt" | tail -15; fail "drive $name (exit $rc)"; }
}

# ---- A: light
fresh light
run_drive A \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-idle.png" \
  "tap bar-field" "clock +700 real" "type bar-field How did the battery do on its first week" "clock +500 real" "clock data" tree "screenshot $SHOTS/02-typing.png window" \
  "tap bar-send" "clock +700 real" "clock data" tree "screenshot $SHOTS/03-after-send.png window" \
  "tap bar-voice down" "tap up" "clock +1800 real" "clock data" tree "screenshot $SHOTS/04-playing.png" \
  "tap bar-now" "clock +900 real" "clock data" tree "screenshot $SHOTS/05-radio-page.png" \
  "clock +3500 real" "clock data" tree "screenshot $SHOTS/05b-radio-page-later.png" \
  "tap radio-speed-up" "clock +500 real" "clock data" tree \
  "tap radio-play" "clock +800 real" "clock data" tree "screenshot $SHOTS/06-radio-paused.png" \
  "tap bar-voice down" "clock +1000 real" "clock data" tree "screenshot $SHOTS/08-talking.png" "tap up" \
  "clock +300 real" "clock data" tree "screenshot $SHOTS/09-after-release.png" \
  "clock +1800 real" "clock data" tree "screenshot $SHOTS/10-later.png" \
  logs
must() { grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"; echo "ok: $1"; }
must "idle: the Ask field with its aim (first run: the invitation)" 'label="Make it yours'
must "typing: Send replaces the voice button" '[bar-send]'
must "typing: the aim line above the field" '[bar-aim]'
must "playing: the bar says what plays" 'NOW PLAYING'
must "radio page: title" '[radio-onair-title]'
must "radio page: the sentence being spoken" '[radio-onair-sentence]'
must "radio page: speed" '[radio-speed]'
must "paused: the bar says paused" 'PAUSED'
must "talk: the bar shows the listening state" 'Listening'

# ---- B: dark
fresh dark
run_drive B \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/20-dark-idle.png" \
  "tap bar-voice down" "tap up" "clock +1800 real" "clock data" tree "screenshot $SHOTS/21-dark-playing.png" \
  "tap bar-now" "clock +900 real" "clock data" tree "screenshot $SHOTS/22-dark-radio-page.png" \
  "tap bar-voice down" "clock +1000 real" "clock data" tree "screenshot $SHOTS/23-dark-talking.png" "tap up" "clock +1500 real" "clock data" tree "screenshot $SHOTS/24-dark-after.png" \
  logs

[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 12 ] || fail "screenshots" "fewer than 12 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'Fatal error' "$OUT"; then echo "note: fatal error in the app log:"; grep -m5 'Fatal error' "$OUT"; fi
echo "PROOF OK"
