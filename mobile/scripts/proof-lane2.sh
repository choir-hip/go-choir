#!/bin/sh
# Lane 2 (Paper) proof on a simulator, demo mode, driven through Exact's agent socket. First the layout engine's unit tests
# (swift test --filter LayoutTests), then for light and for dark: the default edition, then the three demo editions (a
# big-news day, an even day, a thin day) each at the top and further down, a section page, a story opened from the page,
# play from here, and pull to refresh. Screenshots go to $PROOF_SHOTS. Last line: PROOF OK, or PROOF FAILED at <step>.
#   NOBUILD=1   skip the build        NOTESTS=1   skip the Swift tests
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane2.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane2-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

if [ -z "${NOTESTS:-}" ]; then
  echo "swift test --filter LayoutTests"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter LayoutTests > "$SCRATCH/tests.txt" 2>&1 || { tail -40 "$SCRATCH/tests.txt"; fail "layout tests"; }
  tail -3 "$SCRATCH/tests.txt"
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -v "^ *[0-9]* |" "$SCRATCH/build.txt" | head -60; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-lane2-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

: > "$OUT"
for MODE in light dark; do
  DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode $MODE
  DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
  CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  [ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
  attempt=1
  while :; do
    "$HERE/exact.sh" agent ios --touch platform \
      "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/$MODE-01-default.png" \
      "tap paper-edition-big" "clock +900 real" "clock data" tree "screenshot $SHOTS/$MODE-02-big-top.png" \
      "tap paper drag 0 -560" "clock +700 real" "clock data" tree "screenshot $SHOTS/$MODE-03-big-mid.png" \
      "tap paper drag 0 -560" "clock +700 real" "clock data" tree "screenshot $SHOTS/$MODE-04-big-low.png" \
      "tap paper drag 0 -560" "clock +700 real" "clock data" tree "screenshot $SHOTS/$MODE-05-big-end.png" \
      "tap paper drag 0 1500" "clock +700 real" "clock data" \
      "tap paper-edition-even" "clock +900 real" "clock data" tree "screenshot $SHOTS/$MODE-06-even-top.png" \
      "tap paper drag 0 -560" "clock +700 real" "clock data" tree "screenshot $SHOTS/$MODE-07-even-mid.png" \
      "tap paper drag 0 1500" "clock +700 real" "clock data" \
      "tap paper-edition-thin" "clock +900 real" "clock data" tree "screenshot $SHOTS/$MODE-08-thin.png" \
      "tap paper-edition-big" "clock +900 real" "clock data" \
      "tap paper-play-demo-harbor-barrier" "clock +1500 real" "clock data" tree "screenshot $SHOTS/$MODE-09-play-from-here.png" \
      "tap paper-section-culture" "clock +900 real" "clock data" tree "screenshot $SHOTS/$MODE-10-section.png" \
      "tap paper-section-front" "clock +900 real" "clock data" \
      "tap paper-story-demo-grid-storage" "clock +900 real" "clock data" tree "screenshot $SHOTS/$MODE-11-story-page.png" \
      "tap bar-pages" "clock +600 real" "clock data" "tap pile-places" "clock +600 real" "clock data" \
      "tap launcher-open-paper:front" "clock +900 real" "clock data" \
      "tap paper drag 0 400" "clock +250 real" "screenshot $SHOTS/$MODE-12-pull-to-refresh.png" "clock +1500 real" "clock data" tree \
      logs >> "$OUT" 2>&1
    rc=$?
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; : > "$OUT"; continue
    fi
    break
  done
  [ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -20; fail "drive $MODE (exit $rc)"; }
done

must() { grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"; echo "ok: $1"; }
must "masthead" '[page-title] "AUTOPAPER"'
must "default edition: stories as a composed page" '[paper-story-demo-grid-storage]'
must "default edition composition" 'Composition: pair+river'
must "big day: lead, cluster, a stacked pair, briefs, river, opinion" 'Composition: lead+cluster+pair(stack)+briefs+river+opinion'
must "big day: the lead" '[paper-story-demo-harbor-barrier]'
must "big day: Developing cluster follow-up" '[paper-story-demo-big-barrier-cost]'
must "even day: two leads and a side-by-side pair" 'Composition: colead+pair+briefs+river+opinion'
must "thin day: no lead" 'Composition: pair+river'
must "sections: a chip per section" '[paper-section-culture]'
must "play from here" '[paper-play-demo-harbor-barrier]'
must "section page" '"Culture"'
must "story opens as a page" '[page-title] "A regional grid adds its largest battery yet"'
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 20 ] || fail "screenshots" "fewer than 20 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
