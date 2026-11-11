#!/bin/sh
# Lane 5 proof: Files and the viewers, on a simulator, in demo mode (bundled invented files, no network).
#   proof-lane5.sh check    only the fast steps: sync, interface, Contract build (prints the compiler's errors), no tests, no simulator
#   proof-lane5.sh          build, drive the screens, then the Swift tests (FilesPathTests, EpubTests, FilesServiceTests,
#                           ChoirFilesBackendTests); PROOF OK or PROOF FAILED at <step>
#   NOBUILD=1  skip the build   NOTESTS=1  skip the Swift tests
# Screenshots go to $PROOF_SHOTS. Elements are found by test id, never by position. File paths contain spaces, so those ids are quoted.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
MODE="${1:-all}"
SCRATCH="${TMPDIR:-/tmp}/proof-lane5.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane5-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

# ---- 1. prepare exactly as build-lab.sh does, then build the Contract alone (about 2 s) and show every error
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
"$HERE/sync-exact-package.sh" > "$SCRATCH/sync.txt" 2>&1 || { tail -20 "$SCRATCH/sync.txt"; fail "sync"; }
python3 -I "$HERE/gen-exact-interface.py" > "$SCRATCH/iface.txt" 2>&1 || { tail -20 "$SCRATCH/iface.txt"; fail "interface"; }
python3 -I "$HERE/check-strings.py" > "$SCRATCH/strings.txt" 2>&1 || { tail -20 "$SCRATCH/strings.txt"; fail "strings"; }
(cd "$HERE/../AutoradioLab" && rm -rf node_modules/@choir && "$BUN" install > "$SCRATCH/bun.txt" 2>&1) || { tail -20 "$SCRATCH/bun.txt"; fail "bun install"; }
"$HERE/exact.sh" contract build app.contract > "$SCRATCH/contract.txt" 2>&1 || { grep -v '^warning' "$SCRATCH/contract.txt" | head -60; fail "contract build"; }
grep -E "^(warning|error)" "$SCRATCH/contract.txt" | grep -i "files\|viewers\|fs[A-Z]\|fv[A-Z]" | head -20
echo "ok: contract build"
[ "$MODE" = "check" ] && { echo "PROOF OK (check only)"; exit 0; }

# ---- 3. build and drive
export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -E "error|Error" "$SCRATCH/build.txt" | head -30; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
if [ -n "$CONT" ]; then
  rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json" "$CONT/Library/Application Support/Autoradio/recent-files.json"
  rm -rf "$CONT/Library/Caches/Autoradio/files"
fi
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/lane5-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"

# Steps are collected one per line and handed to the driver as separate arguments (a step may hold spaces).
STEPS="$SCRATCH/steps.txt"
step() { printf '%s\n' "$1" >> "$STEPS"; }
wait_() { step "clock +${1:-700} real"; step "clock data"; }
shot() { step "screenshot $SHOTS/$1.png"; }
view() { wait_ "${2:-800}"; step tree; shot "$1"; }
# Switch to an open page from the pile: pile, Places, then the page's chip in the launcher.
goto() { step "tap bar-pages"; wait_ 600; step "tap pile-places"; wait_ 600; step "tap \"launcher-open-$1\""; wait_ 800; }
run() { # <drive name> [agent flags]
  name="$1"; shift
  set -f; OLDIFS="$IFS"; IFS='
'
  # shellcheck disable=SC2046
  "$HERE/exact.sh" agent ios "$@" $(cat "$STEPS") >> "$OUT" 2>&1
  rc=$?
  IFS="$OLDIFS"; set +f
  : > "$STEPS"
  return $rc
}
: > "$OUT"; : > "$STEPS"

attempt=1
while :; do
  # ---- drive 1: the place, light
  step "clock data"; wait_ 1500; step tree; shot 01-paper
  step "tap bar-pages"; wait_ 600; step "tap pile-places"; wait_ 600; step "tap place-files"; view 02-files-root 1200
  step "tap files-view-grid"; view 03-files-grid 500
  step "tap files-view-list"; wait_ 400
  step "tap files-sort-modified"; view 04-sorted-by-modified 500
  step "tap files-sort-size"; wait_ 400
  step "tap files-sort-name"; wait_ 400
  step "tap \"files-row-/Documents\""; view 05-documents-folder 800
  step "tap files-new-toggle"; wait_ 300; step "type files-new-input Projects"; step "tap files-new-go"; view 06-new-folder-banner 800
  step "tap \"files-more-/Documents/Projects\""; wait_ 300; step "type files-rename-input Plans"; step "tap files-rename-go"; view 07-folder-rename-refused 800
  step "tap files-upload"; view 08-upload-added 900
  step "tap \"files-more-/Documents/Added note.txt\""; wait_ 300; step "type files-rename-input Aardvark.txt"; step "tap files-rename-go"; view 09-renamed 800
  step "tap \"files-more-/Documents/Aardvark.txt\""; wait_ 300; step tree; shot 10-row-actions
  step "tap \"files-delete-/Documents/Aardvark.txt\""; view 11-deleted 800
  step "tap \"files-crumb-/\""; wait_ 600
  step "type files-search lantern"; step "type files-search key Enter"; view 12-search 1000
  step "tap files-search-clear"; wait_ 500
  step "tap files-mode-recent"; view 13-recent-empty 600
  step "tap files-mode-browse"; wait_ 500

  # ---- the viewers, one after the other; each returns to the Files page through the pile
  step "tap \"files-row-/Documents\""; wait_ 600
  step "tap \"files-row-/Documents/Council minutes.pdf\""; view 20-pdf 2500
  step "tap pdf-next"; view 21-pdf-page-2 1500
  step "type pdf-find lamps"; step "type pdf-find key Enter"; view 22-pdf-search 1500
  goto "file:/"
  step "tap \"files-row-/Documents/Harbor notes.md\""; view 23-markdown 1500
  goto "file:/"
  step "tap \"files-row-/Documents/Budget 2026.csv\""; view 24-csv 1500
  goto "file:/"
  step "type files-search Lamps"; step "type files-search key Enter"; wait_ 900
  step "tap \"files-row-/Documents/Lamps.swift\""; view 25-code 1500
  goto "file:/"; step "tap files-search-clear"; wait_ 500
  step "type files-search Survey"; step "type files-search key Enter"; wait_ 900
  step "tap \"files-row-/Documents/Survey readings.dat\""; view 26-cannot-preview 1200
  goto "file:/"; step "tap files-search-clear"; wait_ 500
  step "tap \"files-crumb-/\""; wait_ 600
  step "tap \"files-row-/Books\""; wait_ 600
  step "tap \"files-row-/Books/The Lantern Keeper.epub\""; view 27-epub 2000
  step "tap epub-toc"; view 28-epub-contents 700
  step "tap epub-chapter-1"; view 29-epub-chapter-2 900
  step "tap epub-next"; view 30-epub-chapter-3 900
  goto "file:/"
  step "tap \"files-crumb-/\""; wait_ 600
  step "tap \"files-row-/Pictures\""; wait_ 600
  step "tap \"files-row-/Pictures/Harbor test card.png\""; view 31-image 2000
  step "tap image-2x"; view 32-image-2x 900
  step "tap image-4x"; view 33-image-4x 900
  step "tap image-center"; wait_ 800
  goto "file:/"
  step "tap \"files-crumb-/\""; wait_ 600
  step "tap \"files-row-/Media\""; wait_ 600
  step "tap \"files-row-/Media/Tone 440.wav\""; view 34-audio 1800
  goto "file:/"
  step "tap \"files-row-/Media/Pattern.mp4\""; view 35-video 2000
  goto "file:/"
  step "tap files-mode-recent"; view 36-recent 800
  run drive1 --touch platform || rc=$?
  rc=${rc:-0}

  # ---- drive 2: pinch and drag on the image (the platform touch), then the dark theme
  if [ "$rc" -eq 0 ]; then
    step "clock data"; wait_ 1500
    goto "file:/Pictures/Harbor test card.png"
    step "tap image-handle pinch 2.5"; view 40-image-pinch 1000
    step "tap image-handle drag -60 -30"; view 41-image-pan 900
    step "tap bar-pages"; wait_ 600; step "tap pile-places"; wait_ 600; step "tap place-settings"; wait_ 800
    step "tap settings-mode-dark"; wait_ 900
    goto "file:/"
    step "tap files-mode-browse"; view 50-dark-files 900
    goto "file:/Pictures/Harbor test card.png"; view 51-dark-image 900
    goto "file:/Books/The Lantern Keeper.epub"; view 52-dark-epub 900
    goto "file:/Documents/Council minutes.pdf"; view 53-dark-pdf 1200
    goto "file:/Documents/Survey readings.dat"; view 54-dark-cannot-preview 800
    run drive2 --touch platform || rc=$?
    rc=${rc:-0}
  fi
  if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
    attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; : > "$OUT"; : > "$STEPS"; rc=0
    DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
  fi
  break
done
[ "${rc:-0}" -eq 0 ] || { grep -v "^warning" "$OUT" | tail -25; fail "drive (exit $rc)"; }

FAILS=""
must() { if grep -qF -- "$2" "$OUT"; then echo "ok: $1"; else echo "MISSING: $1: $2"; FAILS="$FAILS [$1]"; fi; }

must "Files place: heading" '[page-title] "Files"'
must "Files place: a folder row" '[files-row-/Documents]'
must "Files place: a file row at the root" '[files-row-/Readme.txt]'
must "Files place: search field" '[files-search]'
must "Files place: breadcrumbs" '[files-crumb-/]'
must "grid view" '[files-grid]'
must "folder: breadcrumb shows the folder" '[files-crumb-/Documents]'
must "new folder: banner" 'Made the folder'
must "folder rename refused: banner" 'Folders cannot be renamed here yet'
must "rename: banner" 'Renamed to'
must "delete: banner" 'Deleted'
must "search: finds the book across folders" '[files-row-/Books/The Lantern Keeper.epub]'
must "recent: the files opened appear" '[files-row-/Documents/Council minutes.pdf]'
must "upload: an added file" 'Added'
must "PDF: page label" '[pdf-page] "Page 1 of 3"'
must "PDF: next page" '[pdf-page] "Page 2 of 3"'
must "PDF: the frame" '[pdf-frame]'
must "PDF: search hit on page 2" '[pdf-hit-2]'
must "Markdown viewer" '[text-body]'
must "CSV viewer" '[csv-scroll]'
must "code viewer" '[code-card]'
must "cannot preview: honest message" 'cannot be previewed here'
must "EPUB: contents" '[epub-chapter-2]'
must "EPUB: a chapter title" 'Three visitors'
must "EPUB: text" '[epub-text]'
must "image viewer: frame" '[image-handle]'
must "image viewer: zoom control" '[image-4x]'
must "audio viewer: element and state" '[audio-state]'
must "video viewer: element" '[video-view]'
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 28 ] || FAILS="$FAILS [fewer than 28 screenshots]"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT"; fi
cp "$OUT" "$SHOTS/drive.txt" 2>/dev/null
# ---- last: Swift tests of this lane (after the drive, so one run shows both)
if [ -z "${NOTESTS:-}" ]; then
  LOG="$SCRATCH/tests.txt"
  if "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter FilesPathTests --filter EpubTests --filter FilesServiceTests --filter ChoirFilesBackendTests > "$LOG" 2>&1; then
    grep -E "Executed .* tests" "$LOG" | tail -1
    echo "ok: swift tests"
  else
    grep -E "error:|failed|XCTAssert" "$LOG" | head -40; cp "$LOG" "$SHOTS/swift-tests.txt" 2>/dev/null; FAILS="$FAILS [swift tests]"
  fi
fi
[ -z "$FAILS" ] || fail "checks:$FAILS"

echo "PROOF OK"
