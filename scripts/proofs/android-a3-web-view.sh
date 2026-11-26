#!/bin/sh
# Physical A3 proof for the package-first Android source WebView and external hand-off.
# Run from the lead's build tree. The phone must be ZY22H5K8QR; no credentials are used.
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
export AR_VARIANT=lab-one ANDROID_SERIAL="$SERIAL"
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export ANDROID_HOME=${ANDROID_HOME:-$HOME/Library/Android/sdk}
mkdir -p "$OUT"

[ "$("$ADB" -s "$SERIAL" shell getprop ro.serialno | tr -d '\r')" = "$SERIAL" ] || { echo "wrong Android device" >&2; exit 1; }
APK=$ROOT/mobile/AutoradioLab/target/android.lab-one/app/outputs/apk/release/app-release.apk
[ -f "$APK" ] || { echo "missing $APK; build in the lead tree first" >&2; exit 1; }
ACTUAL=$(shasum -a 256 "$APK" | awk '{print $1}')
[ "$ACTUAL" = "$APK_SHA" ] || { echo "APK SHA-256 $ACTUAL, expected $APK_SHA" >&2; exit 1; }
"$APKSIGNER" verify --print-certs "$APK" > "$OUT/apksigner.txt"
grep -q 'dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7' "$OUT/apksigner.txt"

cd "$ROOT"
STORAGE=a3-web-view-proof
mobile/scripts/exact.sh agent android --storage "$STORAGE" --bind autoradio \
  'clock +6000 real' 'clock data' 'tap bar-mark' \
  'clock +600 real' 'clock data' 'tap place-paper' \
  'clock +5000 real' 'clock data' tree logs > "$OUT/discover.log" 2>&1
STORY=$(sed -n 's/.*\[\(paper-story-[^]]*\)\].*/\1/p' "$OUT/discover.log" | head -1)
[ -n "$STORY" ] || { echo "no paper story in the physical tree" >&2; exit 1; }
open_story() {
  mobile/scripts/exact.sh agent android --storage "$STORAGE" --bind autoradio \
    'clock +6000 real' 'clock data' 'tap bar-mark' \
    'clock +600 real' 'clock data' 'tap place-paper' \
    'clock +5000 real' 'clock data' "tap $STORY" \
    'clock +5000 real' 'clock data' "$@"
}
open_story 'tap reader-source-5' 'clock +8000 real' 'clock data' tree \
  "screenshot $OUT/web.png" logs > "$OUT/web.log" 2>&1
grep -q 'NativeView.*\[reader-web-view\]' "$OUT/web.log"
HOST=$(sed -n 's/.*\[reader-web-host\] "\([^"]*\)".*/\1/p' "$OUT/web.log" | head -1)
[ -n "$HOST" ] || { echo "the source WebView did not report its host" >&2; exit 1; }
printf 'source_host=%s\n' "$HOST" > "$OUT/observed.txt"

open_story 'tap reader-source-5' 'clock +8000 real' 'clock data' \
  'tap content-toggle' 'clock +1000 real' 'clock data' \
  'tap reader-web-safari' 'clock +3000 real' logs > "$OUT/external.log" 2>&1
"$ADB" -s "$SERIAL" shell dumpsys activity activities > "$OUT/external-activity.txt"
grep -q 'topResumedActivity=.*com.android.chrome/org.chromium.chrome.browser.ChromeTabbedActivity' "$OUT/external-activity.txt"
grep -q 'act=android.intent.action.VIEW' "$OUT/external-activity.txt"

open_story 'tap reader-source-5' 'clock +8000 real' 'clock data' \
  'tap reader-web-close' 'clock +2000 real' 'clock data' \
  tree "screenshot $OUT/closed.png" logs > "$OUT/close.log" 2>&1
if grep -q 'NativeView.*\[reader-web-view\]' "$OUT/close.log"; then
  echo "the in-app source view remained after close" >&2
  exit 1
fi

"$ADB" -s "$SERIAL" shell am force-stop "$PACKAGE" > "$OUT/relaunch.txt"
"$ADB" -s "$SERIAL" shell am start -W -n "$PACKAGE/dev.exact.app.MainActivity" >> "$OUT/relaunch.txt"
grep -q 'Status: ok' "$OUT/relaunch.txt"
grep -q 'LaunchState: COLD' "$OUT/relaunch.txt"
"$ADB" -s "$SERIAL" shell pidof "$PACKAGE" >> "$OUT/relaunch.txt"

echo "PASS: in-app WebView, external hand-off, close and cold relaunch"
echo "evidence: $OUT"
