#!/bin/sh
# Physical A2 proof: the Android deck plays the radio, keeps sounding with the screen off, and
# answers the media session's pause, play and next, on the connected Motorola.
#
#   scripts/proofs/android-a2-deck.sh <evidence-directory> <expected-apk-sha256>
#
# Run from the lead's build tree. The phone must be ZY22H5K8QR; no credentials are used (the
# station plays signed out, from the public paper).
set -eu

OUT=${1:-}
APK_SHA=${2:-}
if [ -z "$OUT" ] || [ -z "$APK_SHA" ]; then
  echo "usage: $0 <evidence-directory> <expected-apk-sha256>" >&2
  exit 2
fi
case "$OUT" in /*) ;; *) OUT=$(pwd)/$OUT ;; esac
ROOT=${BUILD_TREE:-$(cd "$(dirname "$0")/../.." && pwd)}
ADB=${ADB:-/Users/wiz/Library/Android/sdk/platform-tools/adb}
SERIAL=${ANDROID_SERIAL:-ZY22H5K8QR}
PACKAGE=news.choir.autoradio.lab.one
APKSIGNER=${APKSIGNER:-$(find "$HOME/Library/Android/sdk/build-tools" -name apksigner -type f | sort -V | tail -1)}
mkdir -p "$OUT"

[ "$("$ADB" -s "$SERIAL" shell getprop ro.serialno | tr -d '\r')" = "$SERIAL" ] || { echo "wrong Android device" >&2; exit 1; }
APK=$ROOT/mobile/AutoradioLab/target/android.lab-one/app/outputs/apk/release/app-release.apk
[ -f "$APK" ] || { echo "missing $APK; build in the lead tree first" >&2; exit 1; }
ACTUAL=$(shasum -a 256 "$APK" | awk '{print $1}')
[ "$ACTUAL" = "$APK_SHA" ] || { echo "APK SHA-256 $ACTUAL, expected $APK_SHA" >&2; exit 1; }
"$APKSIGNER" verify --print-certs "$APK" > "$OUT/apksigner.txt"
grep -q 'dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7' "$OUT/apksigner.txt"

adb() { "$ADB" -s "$SERIAL" "$@"; }
session() { adb shell dumpsys media_session 2>/dev/null; }
state() { session | sed -n 's/.*state=PlaybackState {state=\([A-Z]*\)(\([0-9]*\)), position=\([0-9]*\).*/\1 \3/p' | head -1; }
description() { session | sed -n 's/.*description=//p' | head -1; }

wait_state() { # state seconds
  n=0
  while [ "$n" -lt "$2" ]; do
    got=$(state | awk '{print $1}')
    [ "$got" = "$1" ] && return 0
    sleep 1; n=$((n + 1))
  done
  echo "the media session did not reach $1 in $2 s (last: $(state))" >&2
  return 1
}

echo "== cold launch"
adb shell am force-stop "$PACKAGE"
adb shell am start -W -n "$PACKAGE/dev.exact.app.MainActivity" > "$OUT/launch.txt"
grep -q 'Status: ok' "$OUT/launch.txt"
wait_state PLAYING 40
# The session reports PLAYING as soon as the item is claimed; the first words start after the
# on-device speech is made. Wait for rendered progress, not only the state.
n=0; P1=0
while [ "$n" -lt 60 ]; do
  P1=$(state | awk '{print $2}')
  [ "${P1:-0}" -ge 1000 ] && break
  sleep 1; n=$((n + 1))
done
[ "${P1:-0}" -ge 1000 ] || { echo "no rendered position after 60 s" >&2; exit 1; }
state > "$OUT/playing.txt"
description > "$OUT/description.txt"
echo "playing at ${P1}ms: $(cat "$OUT/description.txt")"

echo "== background"
# A spoken story is one player item per chunk, so its position restarts at each chunk: progress is
# either a later position in the same item or a later item. Sample once a second for ten seconds.
item_id() { session | sed -n 's/.*active item id=\([0-9]*\).*/\1/p' | head -1; }
FIRST_ID=$(item_id)
adb shell input keyevent KEYCODE_HOME
n=0; LAST_ID=$FIRST_ID; P2=$P1; moved=0
while [ "$n" -lt 10 ]; do
  sleep 1
  ID=$(item_id); P2=$(state | awk '{print $2}')
  if [ "${ID:-0}" -gt "${FIRST_ID:-0}" ] 2>/dev/null; then moved=1; fi
  [ "${P2:-0}" -ge $((P1 + 5000)) ] && moved=1
  LAST_ID=$ID
  n=$((n + 1))
done
state > "$OUT/background.txt"
printf 'first_item=%s last_item=%s last_position=%s\n' "$FIRST_ID" "$LAST_ID" "$P2" >> "$OUT/background.txt"
[ "$(state | awk '{print $1}')" = "PLAYING" ] || { echo "the deck stopped when the screen went" >&2; exit 1; }
[ "$moved" = 1 ] || { echo "the radio did not advance in the background (item $FIRST_ID, position $P1 -> ${P2})" >&2; exit 1; }
echo "background: item $FIRST_ID -> $LAST_ID, position now ${P2}ms"

echo "== media session pause and play"
adb shell cmd media_session dispatch pause
wait_state PAUSED 10
P3=$(state | awk '{print $2}')
sleep 2
P4=$(state | awk '{print $2}')
[ "$P3" = "$P4" ] || { echo "paused but the position moved ($P3 -> $P4)" >&2; exit 1; }
state > "$OUT/paused.txt"
adb shell cmd media_session dispatch play
wait_state PLAYING 10
state > "$OUT/played.txt"
echo "paused at ${P3}ms; playing again"

echo "== next"
item_id() { session | sed -n 's/.*active item id=\([0-9]*\).*/\1/p' | head -1; }
BEFORE=$(description); BEFORE_ID=$(item_id)
adb shell cmd media_session dispatch next
n=0; AFTER=$BEFORE; AFTER_ID=$BEFORE_ID
while [ "$n" -lt 25 ]; do
  AFTER=$(description); AFTER_ID=$(item_id)
  if [ -n "$AFTER" ] && { [ "$AFTER_ID" != "$BEFORE_ID" ] || [ "$AFTER" != "$BEFORE" ]; }; then break; fi
  sleep 1; n=$((n + 1))
done
[ -n "$AFTER" ] && { [ "$AFTER_ID" != "$BEFORE_ID" ] || [ "$AFTER" != "$BEFORE" ]; } || { echo "next did not change the item ($BEFORE / item $BEFORE_ID)" >&2; exit 1; }
printf 'before: %s (item %s)\nafter: %s (item %s)\n' "$BEFORE" "$BEFORE_ID" "$AFTER" "$AFTER_ID" > "$OUT/next.txt"
echo "next: $AFTER (item $AFTER_ID)"

echo "== a cold start resumes the station"
adb shell am force-stop "$PACKAGE"
adb shell am start -W -n "$PACKAGE/dev.exact.app.MainActivity" > "$OUT/relaunch.txt"
grep -q 'Status: ok' "$OUT/relaunch.txt"
wait_state PLAYING 40
state > "$OUT/relaunched.txt"
echo "relaunched, playing at $(state | awk '{print $2}')ms"

session > "$OUT/media-session.txt"
echo "PASS: the deck plays, survives the screen going, and obeys the media session"
echo "evidence: $OUT"
