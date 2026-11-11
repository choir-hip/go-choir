#!/bin/sh
# Lane 1 proof (bar, radio, talk), on a simulator of its own in demo mode, driven by test id.
# Swift tests first (BarStateTests), then the build, then three drives:
#   A light: idle, typing and send, play from the bar, the expanded player (follow-along, jump, speed, pause), the paused bar,
#            hold to talk (talking, then heard/holding), after release
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
  "tap bar-text" "clock +900 real" "clock data" tree "screenshot $SHOTS/05-player.png" \
  "clock +3500 real" "clock data" tree "screenshot $SHOTS/05b-player-later.png" \
  "tap player-seg-2" "clock +900 real" "clock data" tree "screenshot $SHOTS/05c-player-jumped.png" \
  "tap player-speed-1.25" "clock +500 real" "clock data" tree \
  "tap player-play" "clock +800 real" "clock data" tree "screenshot $SHOTS/06-player-paused.png" \
  "tap player-close" "clock +800 real" "clock data" tree "screenshot $SHOTS/07-paused-bar.png" \
  "tap bar-voice down" "clock +1000 real" "clock data" tree "screenshot $SHOTS/08-talking.png" "tap up" \
  "clock +300 real" "clock data" tree "screenshot $SHOTS/09-after-release.png" \
  "clock +1800 real" "clock data" tree "screenshot $SHOTS/10-later.png" \
  logs
must() { grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"; echo "ok: $1"; }
must "idle: the Ask field with its aim (first run: the invitation)" 'label="Make it yours'
must "typing: Send replaces the voice button" '[bar-send]'
must "typing: the aim line above the field" '[bar-aim]'
must "playing: the bar says what plays" 'NOW PLAYING'
must "player: title" '[player-title]'
must "player: the passage, sentence by sentence" '[player-sentence-0]'
must "player: programme segments" '[player-seg-0]'
must "player: speed choices" '[player-speed-1.25]'
must "player: the programme rows" '[player-item-0]'
must "paused: the bar says paused" 'PAUSED'
must "talk: the bar shows the listening state" 'Listening'

# ---- B: dark
fresh dark
run_drive B \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/20-dark-idle.png" \
  "tap bar-voice down" "tap up" "clock +1800 real" "clock data" tree "screenshot $SHOTS/21-dark-playing.png" \
  "tap bar-text" "clock +900 real" "clock data" tree "screenshot $SHOTS/22-dark-player.png" \
  "tap player-close" "clock +700 real" \
  "tap bar-voice down" "clock +1000 real" "clock data" tree "screenshot $SHOTS/23-dark-talking.png" "tap up" "clock +1500 real" "clock data" tree "screenshot $SHOTS/24-dark-after.png" \
  logs

# ---- C: a scripted turn through the Components page, the bar visible; shots at intervals (best effort)
fresh light
run_drive C \
  "clock data" "clock +1500 real" "clock data" \
  "tap bar-pages" "clock +600 real" "tap pile-places" "clock +600 real" "tap place-components" "clock +800 real" "clock data" \
  "tap stage-addressing-hybrid" "tap stage-transcriber-scripted" "tap stage-system1-fixed" "tap stage-holder-local-ack" \
    "tap stage-desk-stub-echo" "tap stage-programme-fixture" "tap ack-immediate" "clock data" \
  "tap start" "clock +1500 real" "clock data" \
  "tap inject" "clock +1200 real" "clock data" tree "screenshot $SHOTS/30-turn-1.png" \
  "clock +1500 real" "clock data" tree "screenshot $SHOTS/31-turn-2.png" \
  "clock +2000 real" "clock data" tree "screenshot $SHOTS/32-turn-3.png" \
  "clock +3000 real" "clock data" tree "screenshot $SHOTS/33-turn-4.png" \
  "clock +5000 real" "clock data" tree "screenshot $SHOTS/34-turn-5.png" \
  logs

[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 18 ] || fail "screenshots" "fewer than 18 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'Fatal error' "$OUT"; then echo "note: fatal error in the app log:"; grep -m5 'Fatal error' "$OUT"; fi
echo "PROOF OK"
