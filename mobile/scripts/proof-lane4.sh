#!/bin/sh
# Lane 4 proof (cards, launcher, signals), in demo mode on a simulator of its own, driven through Exact's agent socket by test id.
# Steps: the kit tests (CardsTests, SpineTests); a contract-only build (fails fast on a syntax error); the app build; then
#   A  two and three pages, the pile, a right swipe on a story ("More on River festival"), a left swipe (close, Undo pill),
#      a right swipe on a non-story (springs back), Reopen, the card menu and Pin, the story stack (swipe, buttons, Undo, the end)
#   C1 relaunch: what was told persisted, the front page in the shaded order, the launcher's live place lines and Recent, search
#   C2 the gesture meanings as data (cycled), the Undo pill after a close, Reset, the front page back in the paper's order
#   E  dark: pile, stack, launcher
#   D  mid-drag screenshots (the marks growing while the finger is down); best effort, a failure here is a note not a failure
# Screenshots go to $PROOF_SHOTS. Last line: PROOF OK, or PROOF FAILED at <step>.
#   NOBUILD=1   skip the build (use the installed app)     NOTESTS=1  skip the kit tests
set -u
S="$(cd "$(dirname "$0")" && pwd)"
M="$(cd "$S/.." && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane4.$$"
mkdir -p "$SCRATCH"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; for f in "$SCRATCH"/*.txt; do [ -f "$f" ] && cp "$f" "${PROOF_SHOTS:-/tmp}/drive-$(basename "$f")"; done; exit 1; }

if [ -z "${NOTESTS:-}" ]; then
  echo "== kit tests =="
  "$S/xe.sh" /usr/bin/swift test --package-path "$M/AutoradioKit" --filter "CardsTests|SpineTests" > "$SCRATCH/test.txt" 2>&1 \
    || { grep -E "error:|failed|XCTAssert" "$SCRATCH/test.txt" | head -30; fail "swift test"; }
  grep -E "Executed .* tests" "$SCRATCH/test.txt" | tail -1
fi

echo "== contract build =="
"$S/build-lab.sh" contract build app.contract > "$SCRATCH/contract.txt" 2>&1 || { tail -40 "$SCRATCH/contract.txt"; fail "contract build"; }
tail -3 "$SCRATCH/contract.txt"

export EXACT_SIM="${EXACT_SIM:-$("$S/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "== app build =="
  "$S/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -30 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
sim() { DEVELOPER_DIR=$DD /usr/bin/xcrun simctl "$@"; }
sim privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1
sim spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
sim spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
CONT=$(sim get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
if [ -n "$CONT" ]; then
  for f in open-pages.json what-i-told-the-paper.json signals.jsonl; do rm -f "$CONT/Library/Application Support/Autoradio/$f"; done
fi
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-lane4-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"
sim privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

# drive <name> <touch 0|1> ops... : one agent session; a refused launch is retried (the first launch can miss the agent socket).
drive() {
  name=$1; touch=$2; shift 2
  out="$SCRATCH/$name.txt"
  attempt=1
  while :; do
    if [ "$touch" = 1 ]; then "$S/exact.sh" agent ios --touch platform "$@" logs > "$out" 2>&1
    else "$S/exact.sh" agent ios "$@" logs > "$out" 2>&1; fi
    rc=$?
    if grep -q "never opened its agent socket\|the touch runner did not start\|not foreground-active\|before opening its agent socket\|the app hung up" "$out" && [ $attempt -lt 4 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt ($name)"; sleep 20
      sim terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
    fi
    break
  done
  cat "$out" >> "$SCRATCH/all.txt"
  return $rc
}
must() { # <file> <step name> <fixed text>
  grep -qF -- "$3" "$SCRATCH/$1.txt" || fail "$2" "missing in $1: $3"
  echo "ok: $2"
}
nomust() { # <file> <step name> <fixed text that must NOT appear>
  if grep -qF -- "$3" "$SCRATCH/$1.txt"; then fail "$2" "unexpected in $1: $3"; fi
  echo "ok: $2"
}
line_of() { grep -nF -- "$2" "$SCRATCH/$1.txt" | head -1 | cut -d: -f1; }

# ---------------------------------------------------------------- A
drive A 1 \
  "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-paper.png" \
  "tap paper-story-demo-grid-storage" "clock +800 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" tree "screenshot $SHOTS/02-pile-two-cards.png" \
  "tap pile-places" "clock +700 real" "clock data" tree "screenshot $SHOTS/03-launcher.png" \
  "tap place-paper" "clock +600 real" "clock data" \
  "tap paper-story-demo-river-festival" "clock +800 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" tree "screenshot $SHOTS/04-pile-three-cards.png" \
  "tap card-swipe-pub:demo-river-festival drag 150 0" "clock +400 real" "clock data" tree "screenshot $SHOTS/05-swipe-right-more.png" \
  "tap card-swipe-pub:demo-river-festival drag -150 0" "clock +700 real" "clock data" tree "screenshot $SHOTS/06-swipe-left-closed-undo-pill.png" \
  "tap card-swipe-paper:front drag 150 0" "clock +700 real" "clock data" tree "screenshot $SHOTS/07-nonstory-springs-back.png" \
  "tap pile-reopen" "clock +900 real" "clock data" tree "screenshot $SHOTS/08-reopened.png" \
  "tap card-menu-pub:demo-river-festival" "clock +700 real" "clock data" tree "screenshot $SHOTS/09-card-menu.png" \
  "tap menu-pin" "clock +700 real" "clock data" tree "screenshot $SHOTS/10-pinned.png" \
  "tap card-swipe-pub:demo-river-festival" "clock +800 real" "clock data" tree \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap card-close-pub:demo-grid-storage" "clock +700 real" "clock data" tree "screenshot $SHOTS/10b-closed-grid.png" \
  "tap pile-mode-stack" "clock +900 real" "clock data" tree "screenshot $SHOTS/11-stack.png" \
  "tap stack-swipe-demo-harbor-barrier drag 150 0" "clock +500 real" "clock data" tree "screenshot $SHOTS/12-stack-more.png" \
  "tap stack-aside" "clock +600 real" "clock data" tree "screenshot $SHOTS/13-stack-aside.png" \
  "tap stack-undo" "clock +600 real" "clock data" tree \
  "tap stack-more" "clock +600 real" "clock data" tree \
  "tap stack-aside" "clock +600 real" "clock data" tree \
  "tap stack-aside" "clock +800 real" "clock data" tree "screenshot $SHOTS/14-stack-end.png" \
  "tap stack-all" "clock +600 real" "clock data" tree
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/A.txt" | tail -15; fail "drive A (exit $rc)"; }

must A "demo: front page under the bar" '[page-title] "AUTOPAPER"'
must A "pile: a card per page" '[card-pub:demo-grid-storage]'
must A "pile: the card's touch surface" '[card-swipe-pub:demo-grid-storage]'
must A "pile: the card's menu button" '[card-menu-pub:demo-grid-storage]'
must A "pile: card for the paper" '[card-paper:front]'
must A "pile: three cards" '[bar-pages-count] "3"'
must A "pile: Pages and Stories modes" '[pile-mode-stack]'
must A "launcher: a place with its live line (mail)" '[place-line-mail]'
must A "launcher: the stack entry" '[launcher-stack]'
must A "launcher: your paper (count)" '[told-count]'
must A "launcher: gesture meanings are buttons" '[gesture-left]'
must A "swipe right on a story: the acknowledgement names the entity" 'More on River festival'
must A "swipe left: the card closes (Undo pill names it)" 'Closed The river festival returns'
must A "swipe left: Undo button" '[undo-close]'
must A "reopen: the page is back" '[card-pub:demo-river-festival]'
must A "card menu: pin" '[menu-pin]'
must A "card menu: more of this on a story" '[menu-more]'
must A "card menu: why this story" 'Ranked by the paper'
must A "pin: the card says Pinned" 'Pinned'
must A "stack: a card per story, in order" '[stack-card-demo-harbor-barrier]'
must A "stack: the count" '[stack-count] "1 of 4'
must A "stack: swipe right is more of this" 'More on Harbor Authority'
must A "stack: the next card" '[stack-count] "2 of 4'
must A "stack: set aside moves on" '[stack-count] "3 of 4'
must A "stack: undo goes back" '[stack-count] "2 of 4'
must A "stack: the end" '[stack-end]'
must A "stack: the end offers all stories" '[stack-all]'

# ---------------------------------------------------------------- C1: relaunch; what was told persisted and shades the front page
drive C1 1 \
  "clock data" "clock +1500 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap pile-places" "clock +800 real" "clock data" tree "screenshot $SHOTS/15-launcher-live.png" \
  "tap launcher-search" "clock +500 real" "clock data" \
  "type launcher-search harbor" "clock +900 real" "clock data" tree "screenshot $SHOTS/16-search.png" \
  "type launcher-search paper" "clock +900 real" "clock data" \
  "tap search-hit-paper:front" "clock +900 real" "clock data" tree "screenshot $SHOTS/17-front-shaded.png"
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/C1.txt" | tail -15; fail "drive C1 (exit $rc)"; }
must C1 "persisted: what was told survives a relaunch" '[told-count] "You have told the paper'
nomust C1 "persisted: not empty" 'You have not told the paper anything yet'
must C1 "launcher: Recent pages (the closed page)" '[launcher-recent-pub:demo-grid-storage]'
must C1 "search: a story matches" '[search-hit-pub:demo-harbor-barrier]'
must C1 "launcher: a place line is live" '[place-line-paper]'
fest=$(line_of C1 '[paper-story-demo-river-festival]'); grid=$(line_of C1 '[paper-story-demo-grid-storage]')
[ -n "$fest" ] && [ -n "$grid" ] || fail "front page: stories listed" "festival=$fest grid=$grid"
# the last tree of the drive is the front page after place-paper: compare the LAST occurrences
fest=$(grep -nF '[paper-story-demo-river-festival]' "$SCRATCH/C1.txt" | tail -1 | cut -d: -f1)
grid=$(grep -nF '[paper-story-demo-grid-storage]' "$SCRATCH/C1.txt" | tail -1 | cut -d: -f1)
[ "$fest" -lt "$grid" ] || fail "shading: the told story's relative rises above grid storage" "festival line $fest, grid line $grid"
echo "ok: shading: River festival now above Grid storage on the front page"

# ---------------------------------------------------------------- C2: gesture meanings, Undo after a close, Reset
drive C2 1 \
  "clock data" "clock +1500 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap pile-places" "clock +800 real" "clock data" \
  "tap gesture-left" "clock +700 real" "clock data" tree "screenshot $SHOTS/18-gesture-left-changed.png" \
  "tap gesture-right" "clock +700 real" "clock data" tree \
  "tap gesture-left" "clock +600 real" "clock data" tree \
  "tap gesture-left" "clock +600 real" "clock data" tree \
  "tap gesture-right" "clock +600 real" "clock data" tree \
  "tap told-open" "clock +900 real" "clock data" tree "screenshot $SHOTS/19-told-page.png" \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap card-close-texture:what-i-told-the-paper" "clock +700 real" "clock data" tree "screenshot $SHOTS/20-close-button-undo.png" \
  "tap undo-close" "clock +900 real" "clock data" tree \
  "tap pile-places" "clock +800 real" "clock data" \
  "tap told-reset" "clock +900 real" "clock data" tree "screenshot $SHOTS/21-reset.png" \
  "tap place-paper" "clock +900 real" "clock data" tree "screenshot $SHOTS/22-front-unshaded.png"
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/C2.txt" | tail -15; fail "drive C2 (exit $rc)"; }
must C2 "meanings: swipe left, close and less" 'Swipe left: Close, and less of this'
must C2 "meanings: swipe left, less in the stack only" 'Swipe left: Close; less of this in the stack'
must C2 "meanings: swipe right, nothing" 'Swipe right: Nothing'
must C2 "meanings: back to the defaults" 'Swipe right: More of this'
must C2 "close button: the Undo pill" 'Closed What I'
must C2 "undo: the page is back" '[card-texture:what-i-told-the-paper]'
must C2 "reset: nothing told" 'You have not told the paper anything yet'
fest=$(grep -nF '[paper-story-demo-river-festival]' "$SCRATCH/C2.txt" | tail -1 | cut -d: -f1)
grid=$(grep -nF '[paper-story-demo-grid-storage]' "$SCRATCH/C2.txt" | tail -1 | cut -d: -f1)
[ -n "$fest" ] && [ -n "$grid" ] && [ "$grid" -lt "$fest" ] || fail "reset: the paper's own order is back" "festival line $fest, grid line $grid"
echo "ok: reset: the paper's own order is back"

# ---------------------------------------------------------------- E: dark
drive E 1 \
  "clock data" "clock +1500 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap pile-places" "clock +800 real" "clock data" \
  "tap place-settings" "clock +700 real" "clock data" \
  "tap settings-mode-dark" "clock +900 real" "clock data" \
  "tap bar-pages" "clock +900 real" "clock data" tree "screenshot $SHOTS/23-pile-dark.png" \
  "tap pile-mode-stack" "clock +900 real" "clock data" tree "screenshot $SHOTS/24-stack-dark.png" \
  "tap pile-places" "clock +900 real" "clock data" tree "screenshot $SHOTS/25-launcher-dark.png" \
  "tap place-settings" "clock +700 real" "clock data" \
  "tap settings-mode-light" "clock +900 real" "clock data" tree
rc=$?
[ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/E.txt" | tail -15; fail "drive E (exit $rc)"; }
must E "dark: the pile renders" '[pile-mode-stack]'
must E "light again" 'Showing Depth (light)'

# ---------------------------------------------------------------- D: mid-drag (best effort)
drive D 1 \
  "clock data" "clock +1500 real" "clock data" \
  "tap bar-pages" "clock +700 real" "clock data" \
  "tap pile-mode-pages" "clock +700 real" "clock data" \
  "tap card-swipe-pub:demo-river-festival drag 90 0 hold 700 during \"screenshot $SHOTS/26-middrag-more.png\"" "clock +900 real" "clock data" \
  "tap card-swipe-pub:demo-river-festival drag -90 0 hold 700 during \"screenshot $SHOTS/27-middrag-close.png\"" "clock +900 real" "clock data" \
  "tap pile-mode-stack" "clock +900 real" "clock data" \
  "tap stack-swipe-demo-harbor-barrier drag 90 0 hold 700 during \"screenshot $SHOTS/28-middrag-stack-more.png\"" "clock +900 real" "clock data"
rc=$?
if [ $rc -eq 0 ] && [ -f "$SHOTS/26-middrag-more.png" ]; then echo "ok: mid-drag screenshots"; else echo "note: mid-drag screenshots not taken (exit $rc); the marks are judged from the after-throw screenshots"; grep -v "^warning" "$SCRATCH/D.txt" | tail -5; fi

[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 22 ] || fail "screenshots" "fewer than 22 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$SCRATCH/all.txt"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$SCRATCH/all.txt"; fi
echo "PROOF OK"
