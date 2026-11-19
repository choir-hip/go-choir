#!/bin/sh
# Lane 9 proof: the pure-Contract TetraMark and the alive wait, on a simulator, demo mode, light and dark.
# Drives by test id: the bar's mark idle on the paper page. (The motion gallery, the mark gallery and the wait page are gone,
# owner 2026-11-19.) Measures, as well as a simulator can: the CPU of the app process, sampled by pid with `top`, while idle. SIMULATOR NUMBERS ARE NOT PHONE NUMBERS: the app runs on the Mac's CPU, the render server is the Mac's.
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

# ---- assertions ----
echo "MEASURE app process CPU (simulator, Mac cores; not a phone), first 3 samples dropped:"
echo "  idle bar mark (5 Hz):   $(mean_cpu "$SCRATCH/cpu-idle.txt")"
cp "$SCRATCH"/cpu-*.txt "$SHOTS/" 2>/dev/null
[ -f "$SHOTS/01-bar-idle.png" ] || fail "screenshots" "no 01-bar-idle.png in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$SCRATCH/m1.txt"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$SCRATCH/m1.txt"; fi
echo "PROOF OK"
