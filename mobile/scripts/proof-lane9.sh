#!/bin/sh
# Lane 9 proof: the pure-Contract TetraMark and the alive wait, on a simulator, demo mode, light and dark.
# Drives by test id: the bar's mark (idle, listening, playing), the motion gallery (`lab:motion`: every state at 26, 44 and
# 96 pt, the reduced-motion row, the wait, the 3D-box experiment), the flip after "Replay states", the wait page
# (`lab:wait`). Measures, as well as a simulator can: the frame counter the root clock keeps (frames per second actually
# presented) and the CPU and memory of the app process, sampled by pid with `top`, while idle, while the wait runs and
# during the gallery. SIMULATOR NUMBERS ARE NOT PHONE NUMBERS: the app runs on the Mac's CPU, the render server is the Mac's.
# Screenshots go to $PROOF_SHOTS. Last line: PROOF OK or PROOF FAILED at <step>.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane9.$$"
mkdir -p "$SCRATCH"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }
DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -v "^warning" "$SCRATCH/build.txt" | tail -40; fail "build"; }
fi

SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/lane9-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"
sim() { DEVELOPER_DIR=$DD /usr/bin/xcrun simctl "$@"; }
sim privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
fresh() { # <light|dark>
  sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode "$1"
  sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
}
CONT=$(sim get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
clean_pages() { [ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"; }

# Sample the app process (CPU and resident MB) once a second for N seconds into a file, after it appears. Background.
appid() { sim spawn "$EXACT_SIM" launchctl list 2>/dev/null | awk -v a="UIKitApplication:$APP" 'index($3, a) {print $1; exit}'; }
sampler() { # <file> <seconds>
  ( n=0; pid=""
    while [ $n -lt 40 ] && [ -z "$pid" ]; do pid=$(appid); case "$pid" in ''|-) pid=""; sleep 1; n=$((n+1));; esac; done
    [ -n "$pid" ] || { echo "nopid" > "$1"; exit 0; }
    : > "$1"; i=0
    while [ $i -lt "$2" ]; do
      v=$(top -l 2 -s 1 -pid "$pid" -stats cpu,mem 2>/dev/null | tail -1)
      echo "$(date +%s) $v" >> "$1"; i=$((i+1))
    done ) &
}
mean_cpu() { awk 'NR>3 && $2+0==$2 {s+=$2; n++; if ($2>m) m=$2} END {if (n) printf "mean %.1f%% max %.1f%% over %d samples", s/n, m, n; else print "no samples"}' "$1"; }

drive() { # <out> <ops...>  (retries a refused agent socket)
  out="$1"; shift; attempt=1
  while :; do
    "$HERE/exact.sh" agent ios --touch platform --timing platform "$@" > "$out" 2>&1; rc=$?
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$out" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
    fi
    return $rc
  done
}
must() { grep -qF -- "$3" "$2" || fail "$1" "missing: $3"; echo "ok: $1"; }

# ---- M1: baseline, the bar's mark idle on the paper page (5 Hz drift), 10 s ----
fresh light; clean_pages
sampler "$SCRATCH/cpu-idle.txt" 14
drive "$SCRATCH/m1.txt" "clock data" "clock +1500 real" "tree" "screenshot $SHOTS/01-bar-idle.png" "clock +9000 real" "clock data" \
  || { tail -15 "$SCRATCH/m1.txt"; fail "drive idle"; }
wait
must "bar: the mark replaces the placeholder glyph" "$SCRATCH/m1.txt" '[mark]'

# ---- D1: bar states, the gallery, the flip, the wait inside the gallery, then end on the wait page ----
fresh light; clean_pages
sampler "$SCRATCH/cpu-gallery.txt" 75
drive "$SCRATCH/d1.txt" "clock data" "clock +1200 real" \
  "tap bar-pages" "clock +600 real" "tap pile-places" "clock +600 real" "tap place-settings" "clock +600 real" tree "screenshot $SHOTS/02-settings.png" \
  "tap bar-voice down" "clock +900 real" tree "screenshot $SHOTS/03-bar-listening.png" "tap up" "clock +1500 real" \
  "tap settings-motion" "clock +1500 real" tree "screenshot $SHOTS/04-gallery-top.png" \
  "tap motion-replay" "clock +120 real" "screenshot $SHOTS/05-flip-a.png" "clock +150 real" "screenshot $SHOTS/05-flip-b.png" \
  "clock +150 real" "screenshot $SHOTS/05-flip-c.png" "clock +150 real" "screenshot $SHOTS/05-flip-d.png" "clock +1500 real" tree "screenshot $SHOTS/06-gallery-settled.png" \
  "tap motion-wait-replay" "clock +1800 real" tree "screenshot $SHOTS/07-wait-1.png" \
  "clock +3500 real" "screenshot $SHOTS/07-wait-2.png" "clock +3500 real" "screenshot $SHOTS/07-wait-3.png" \
  "clock +3200 real" "screenshot $SHOTS/07-wait-4.png" "clock +2500 real" tree "screenshot $SHOTS/07-wait-5-done.png" \
  "tap fins-clocked" "clock +900 real" tree "screenshot $SHOTS/08-gallery-bottom.png" "clock +2000 real" "screenshot $SHOTS/08-gallery-bottom-b.png" \
  "tap bar-pages" "clock +600 real" "tap pile-places" "clock +600 real" "tap place-settings" "clock +600 real" \
  "tap settings-wait" "clock +1500 real" tree "screenshot $SHOTS/09-wait-page.png" logs \
  || { grep -v "^warning" "$SCRATCH/d1.txt" | tail -20; fail "drive gallery"; }
wait
cp "$SCRATCH/d1.txt" "$SHOTS/drive-gallery.txt" 2>/dev/null

# ---- M2: the wait page alone (relaunch lands on it), 15 s: frames per second and CPU ----
sampler "$SCRATCH/cpu-wait.txt" 19
drive "$SCRATCH/m2.txt" "clock data" "clock +1500 real" "tap wait-replay" "clock +300 real" tree "screenshot $SHOTS/10-wait-start.png" "clock +15500 real" tree "screenshot $SHOTS/11-wait-end.png" \
  || { grep -v "^warning" "$SCRATCH/m2.txt" | tail -20; fail "drive wait"; }
wait
cp "$SCRATCH/m2.txt" "$SHOTS/drive-wait.txt" 2>/dev/null

# ---- D3: dark ----
fresh dark; clean_pages
drive "$SCRATCH/d3.txt" "clock data" "clock +1200 real" \
  "tap bar-pages" "clock +600 real" "tap pile-places" "clock +600 real" "tap place-settings" "clock +600 real" \
  "tap settings-motion" "clock +2500 real" tree "screenshot $SHOTS/12-gallery-dark.png" \
  "tap motion-wait-replay" "clock +4000 real" "screenshot $SHOTS/13-wait-dark.png" \
  || { grep -v "^warning" "$SCRATCH/d3.txt" | tail -20; fail "drive dark"; }

# ---- assertions ----
D="$SCRATCH/d1.txt"
must "settings: the motion gallery is reachable" "$D" '[settings-motion]'
must "gallery: a page" "$D" '[page-title] "Motion"'
for s in idle listening thinking speaking success error reduced; do must "gallery: row $s" "$D" "[motion-row-$s]"; done
for z in 26 44 96; do must "gallery: thinking mark at $z pt" "$D" "[mark-thinking-$z]"; done
must "gallery: the reduced row" "$D" '[mark-reduced-96]'
must "wait: first scripted line" "$D" 'Reading what you asked'
must "wait: a later scripted line" "$D" 'Drafting the revision'
must "wait: the end" "$D" 'Revision ready'
must "experiment: clocked 3D boxes" "$D" '[fins-clocked]'
must "experiment: keyframe 3D boxes" "$D" '[fins-keyframes]'
must "wait page: reachable" "$D" '[wait]'
# the pictures must move: consecutive screenshots of the wait differ
for p in 07-wait-1:07-wait-2 07-wait-2:07-wait-3 07-wait-3:07-wait-4 05-flip-a:05-flip-c; do
  a=${p%%:*}; b=${p##*:}
  [ -f "$SHOTS/$a.png" ] && [ -f "$SHOTS/$b.png" ] || fail "screenshots" "missing $a or $b"
  [ "$(md5 -q "$SHOTS/$a.png")" != "$(md5 -q "$SHOTS/$b.png")" ] || fail "motion" "$a and $b are identical: the mark did not move"
  echo "ok: $a and $b differ"
done
# frames per second from the root clock's counter on the wait page
FR=$(grep -o 'wait-clock\] "[0-9]* frames, [0-9]* ms"' "$SCRATCH/m2.txt" | sed 's/[^0-9 ]//g' | awk '{print $1, $2}')
echo "$FR" | awk 'NR==1 {f0=$1; t0=$2} {f1=$1; t1=$2} END {if (t1>t0) printf "MEASURE wait page: %d frames in %.1f s = %.1f frames/s presented to the mark (60 = none dropped)\n", f1-f0, (t1-t0)/1000, (f1-f0)/((t1-t0)/1000); else print "MEASURE: no clock readings"}'
echo "MEASURE app process CPU (simulator, Mac cores; not a phone), first 3 samples dropped:"
echo "  idle bar mark (5 Hz):   $(mean_cpu "$SCRATCH/cpu-idle.txt")"
echo "  gallery + wait + nav:   $(mean_cpu "$SCRATCH/cpu-gallery.txt")"
echo "  the wait page, 15 s:    $(mean_cpu "$SCRATCH/cpu-wait.txt")"
cp "$SCRATCH"/cpu-*.txt "$SHOTS/" 2>/dev/null
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 15 ] || fail "screenshots" "fewer than 15 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$D"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$D"; fi
echo "PROOF OK"
