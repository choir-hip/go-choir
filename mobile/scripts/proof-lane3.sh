#!/bin/sh
# Lane 3 proof: the Texture reader and the Textures place, on a simulator, in demo mode (fixtures, no network), light and dark.
# Runs the kit's TextureDocTests, builds, then drives the app by test id and takes a screenshot of every screen and state:
#   Textures place; a document (headings, lists, table, quote); its versions sheet and page menu; the prompt's aim;
#   a document mid-revision (the banner); a long story (header, citations, an open card, the source sheet, a transclusion opened as
#   its own page, Sources, More on this); read-along (sentence highlight); side notes (textflow); the Markdown atom-pill experiment.
# Last line: PROOF OK, or PROOF FAILED at <step>.
#   NOBUILD=1   skip the build        NOTEST=1   skip the swift test
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane3.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane3-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

if [ -z "${NOTEST:-}" ]; then
  echo "swift test (TextureDocTests)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter TextureDocTests > "$SCRATCH/swift-test.txt" 2>&1 \
    || { grep -E "error:|failed|Fatal" "$SCRATCH/swift-test.txt" | head -30; tail -15 "$SCRATCH/swift-test.txt"; fail "swift test"; }
  grep -E "Executed [0-9]+ tests" "$SCRATCH/swift-test.txt" | tail -1
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"

if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -E "error|Error|refused|lower-|syntax-|type-" "$SCRATCH/build.txt" | head -60; tail -15 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
[ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-lane3-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

# One drive per mode. A step is a few words; every `clock +N real` lets the app settle. Taps scroll the element into view.
L="clock +700 real"
launcher() { echo "tap bar-pages drag 0 -200"; echo "$L"; }
drive() { # <mode: light|dark> <attempt-file>
  M=$1
  "$HERE/exact.sh" agent ios --touch platform \
    "clock data" "clock +1500 real" "clock data" tree \
    "tap bar-pages drag 0 -200" "$L" "tap place-textures" "$L" "clock data" tree "screenshot $SHOTS/$M-01-textures.png" \
    "tap textures-doc-texture:demo-notes" "clock +1000 real" "clock data" tree "screenshot $SHOTS/$M-02-document-top.png" \
    "tap reader-block-b10" "$L" "clock data" tree "screenshot $SHOTS/$M-03-document-table.png" \
    "tap reader-versions" "$L" "clock data" tree "screenshot $SHOTS/$M-04-versions.png" \
    "tap reader-sheet-close" "$L" \
    "tap reader-menu" "$L" "clock data" tree "screenshot $SHOTS/$M-05-menu.png" \
    "tap reader-menu-aim" "$L" "clock data" tree "screenshot $SHOTS/$M-06-aim.png" \
    "tap reader-aim-clear" "$L" \
    "tap bar-pages drag 0 -200" "$L" "tap place-textures" "$L" "tap textures-doc-texture:demo-draft" "clock +1000 real" "clock data" tree "screenshot $SHOTS/$M-07-revising.png" \
    "tap bar-pages drag 0 -200" "$L" "tap place-paper" "$L" "tap paper-story-demo-grid-storage" "clock +1000 real" "clock data" tree "screenshot $SHOTS/$M-08-plain-story.png" \
    "tap reader-related-pub:demo-ev-charging" "clock +1200 real" "clock data" tree "screenshot $SHOTS/$M-09-story-top.png" \
    "tap reader-cite-a1" "$L" "clock data" tree "screenshot $SHOTS/$M-10-cite-open.png" \
    "tap reader-card-source-a1" "$L" "clock data" tree "screenshot $SHOTS/$M-11-source-sheet.png" \
    "tap reader-sheet-close" "$L" \
    "tap reader-ref-t1" "$L" "clock data" tree "screenshot $SHOTS/$M-12-transclusion.png" \
    "tap reader-card-open-t1" "clock +1200 real" "clock data" tree "screenshot $SHOTS/$M-13-transclusion-page.png" \
    "tap bar-pages drag 0 -200" "$L" "tap launcher-open-pub:demo-ev-charging" "clock +900 real" \
    "tap reader-source-3" "$L" "clock data" tree "screenshot $SHOTS/$M-14-source-from-list.png" \
    "tap reader-sheet-close" "$L" \
    "tap reader-sources" "$L" "clock data" tree "screenshot $SHOTS/$M-15-sources-list.png" \
    "tap reader-menu" "$L" "tap reader-menu-follow" "$L" "clock data" tree "screenshot $SHOTS/$M-16-read-along.png" \
    "tap reader-menu" "$L" "tap reader-menu-sidenotes" "$L" "tap reader-cite-a2" "$L" "clock data" tree "screenshot $SHOTS/$M-17-side-note.png" \
    "tap reader-menu" "$L" "tap reader-menu-sidenotes" "$L" \
    "tap reader-menu" "$L" "tap reader-menu-markdown" "$L" "clock data" tree "screenshot $SHOTS/$M-18-markdown-atoms.png" \
    "tap reader-menu" "$L" "tap reader-menu-markdown" "$L" \
    logs > "$OUT" 2>&1
}

attempt=1
while :; do
  drive light
  rc=$?
  if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
    attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
  fi
  break
done
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -20; fail "drive light (exit $rc)"; }
cp "$OUT" "$SHOTS/drive-light.txt"

must() { # <step name> <fixed text>
  grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"
  echo "ok: $1"
}

must "textures: search field" '[textures-search]'
must "textures: the edition is pinned" '[textures-doc-texture:demo-edition]'
must "textures: recent documents" '[textures-doc-texture:demo-notes]'
must "document: title" '[page-title] "Charging pilot: working notes"'
must "document: table block" '[reader-block-b10]'
must "versions sheet" '[reader-version-3]'
must "menu: versions" '[reader-menu-versions]'
must "aim: chip" '[reader-aim]'
must "revising: banner" '[reader-revising]'
must "revising: stop" '[reader-stop]'
must "plain story renders in the reader" '[page-title] "A regional grid adds its largest battery yet"'
must "related stories" '[reader-related-pub:demo-ev-charging]'
must "story: title" '[page-title] "A city rewires its kerbs for electric cars"'
must "story: a citation mark" '[reader-cite-a1]'
must "citation: card opens in place" '[reader-card-a1]'
must "citation: source sheet" '[reader-source-context]'
must "transclusion: card" '[reader-card-t1]'
must "transclusion: opens as its own page" '[page-title] "Charging pilot: working notes"'
must "sources list" '[reader-source-3]'

# Dark: the same screens again.
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode dark
[ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
sleep 3
drive dark
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -20; fail "drive dark (exit $rc)"; }
cp "$OUT" "$SHOTS/drive-dark.txt"
must "dark: story title" '[page-title] "A city rewires its kerbs for electric cars"'

[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 30 ] || fail "screenshots" "fewer than 30 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
