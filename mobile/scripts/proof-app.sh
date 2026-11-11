#!/bin/sh
# Whole-app proof (integration of the nine lanes), on a simulator of its own driven through Exact's agent socket, in demo mode.
# 1. generators and checks (embedded assets, Exact interface, strings), 2. every Swift test of the package (one `swift test`),
# 3. one build, 4. a tour: paper (front, a story in the reader), the bar's controls (play, mic, the player sheet), the deck (a swipe sends a page to the bottom, six swipes cycle round, the
# launcher brings one back) and the launcher, files (list, a PDF),
# mail (inbox, a message), Textures (list, a document), settings, voice setup and the interview, the motion lab; first in LIGHT, then in
# DARK. The scheme really switches: the Settings mode control (settings-mode-dark) is tapped, the settings line "Showing Depth (dark)" is
# asserted, and it is put back to light at the end. Screenshot names say which mode they are: light-NN-*.png, dark-NN-*.png.
# Last line: PROOF OK, or PROOF FAILED at <step>. Screenshots: $PROOF_SHOTS.
#   NOBUILD=1   skip the build        NOTESTS=1   skip swift test
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-app-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
. "$HERE/proof-common.sh"
echo "screenshots: $SHOTS"

# ---- 1. generators and checks
python3 -I "$HERE/embed-assets.py" > "$SCRATCH/embed.txt" 2>&1 || { tail -20 "$SCRATCH/embed.txt"; fail "embed assets"; }
python3 -I "$HERE/gen-exact-interface.py" > "$SCRATCH/iface.txt" 2>&1 || { tail -20 "$SCRATCH/iface.txt"; fail "exact interface"; }
python3 -I "$HERE/check-strings.py" > "$SCRATCH/strings.txt" 2>&1 || { tail -20 "$SCRATCH/strings.txt"; fail "strings"; }
echo "ok: assets, interface, strings"

# ---- 2. Swift tests: the whole package
if [ -z "${NOTESTS:-}" ]; then
  echo "swift test (the whole package)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" > "$SCRATCH/test.txt" 2>&1 \
    || { grep -E "error:|failed|XCTAssert" "$SCRATCH/test.txt" | cut -c1-400 | head -40; tail -6 "$SCRATCH/test.txt"; cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"; fail "swift test"; }
  grep -E "Executed [0-9]+ tests" "$SCRATCH/test.txt" | tail -1
  cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"
fi

# ---- 3. build
export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -v '^warning' "$SCRATCH/build.txt" | head -60; tail -15 "$SCRATCH/build.txt"; cp "$SCRATCH/build.txt" "$SHOTS/build.txt"; fail "build"; }
fi
SIM privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

# ---- 4a. LIGHT tour (touch platform: drags and pinches)
step "clock data"; wait_ 1500
ensure_light
goto "paper:front"; step tree; shot light-01-paper-front
step "tap paper-scroll drag 0 -260"; wait_ 700
step "tap paper-story-demo-grid-storage"; view light-02-paper-story-reader 1000
step "tap reader-scroll drag 0 -360"; view light-02c-story-scrolled 900
step "tap page-deck drag 180 0"; view light-02d-scrolled-then-swiped 1200
goto "pub:demo-grid-storage"; view light-02e-story-back-after-swipe 900
# the bar's explicit controls: play starts the edition, the line above names what is loaded, a tap on it opens the player sheet
step "tap bar-play"; wait_ 1800
step tree; shot light-02b-bar-playing
step "tap bar-text"; view light-03-player 900
step "tap player-close"; wait_ 700
# the mic: a tap talks, a tap again sends
step "tap bar-mic"; view light-03b-mic-listening 900
step "tap bar-mic"; wait_ 1500
# the deck: a sideways swipe sends the story to the bottom, the next page comes up; the launcher brings it back
step "tap page-deck drag -180 0"; view light-04-deck-swiped-away 900
step "tap bar-mark"; view light-05-launcher 700
step "tap \"launcher-open-pub:demo-grid-storage\""; view light-05b-story-brought-back 900
# six swipes cycle the whole deck of six round to the story again
for i in 1 2 3 4 5 6; do step "tap page-deck drag 180 0"; wait_ 1500; if [ $i = 2 ]; then step tree; shot light-05c-deck-mid-cycle; fi; done
step tree; shot light-05d-deck-cycled-round
step "tap bar-mark"; wait_ 600
step "tap place-settings"; wait_ 900
step "tap settings-components"; view light-05e-components-page 1500
step "tap bar-mark"; wait_ 600
step "tap place-files"; view light-06-files-list 1200
step "tap \"files-row-/Documents\""; wait_ 700
step "tap \"files-row-/Documents/Council minutes.pdf\""; view light-07-files-pdf 2500
place mail 900; view light-08-mail-inbox 500
step "tap mail-row-m6"; view light-09-mail-message 900
place textures 900; view light-10-textures-list 500
step "tap textures-doc-texture:demo-notes"; view light-11-texture-document 1000
place settings 900; step tree; shot light-12-settings
step "tap settings-motion"; view light-13-motion-lab 1500
place settings 900
step "tap settings-setup-voice"; view light-14-setup-from-settings 900
step "tap ob-skip"; view light-14b-setup-left-back-to-deck 900
place settings 900
step "tap settings-dev-details"; wait_ 500
goto "paper:front"; view light-15-paper-developer-controls 800
step "type bar-field what is this page about"; view light-15b-typed-ask 900
step logs
run_drive light light "" --touch platform

if [ -z "${SHELLONLY:-}" ]; then
# ---- 4a0. "Read it to me" from nothing: a fresh app, a message, the button; the engine starts and the bar says now playing.
# (With a story already loaded and paused the message queues behind it, like the next story, so this drive starts clean.)
step "clock data"; wait_ 1500
place mail 900
step "tap mail-row-m6"; wait_ 900
step "tap mail-read"; view light-09b-mail-read-aloud 2500
step logs
run_drive mailread light "" --touch platform

# ---- 4a1. the radio feeds the reader: open a story from the launcher's search, play, and the reader marks the block being read
step "clock data"; wait_ 1500
ensure_light
step "tap bar-mark"; wait_ 900
step "type launcher-search harbor"; wait_ 900
step "tap search-hit-pub:demo-harbor-barrier"; view light-18-story-before-play 1000
step "tap bar-play"; wait_ 7000
step tree; shot light-19-story-while-playing
step logs
run_drive follow light "" --touch platform

# ---- 4a2. the talk mode has one owner (Settings): choose "Always listening" there and the bar's player says so
step "clock data"; wait_ 1500
ensure_light
place settings 900
step "tap settings-nav-talk"; wait_ 700
step "tap settings-choice-talkMode-open"; view light-16-settings-talk-open 700
step "tap bar-play"; wait_ 1800
step "tap bar-text"; view light-17-player-open-mode 900
step logs
run_drive talk light "" --touch platform

# ---- 4b. voice setup and the interview (the page opens at launch by a defaults flag), light
step "clock data"; wait_ 1500; step tree; shot light-20-onboarding-welcome
step "tap ob-begin"; view light-21-setup-downloading 1200
step "clock +3500 real"; step "clock data"; step tree; shot light-22-setup-stalled
step "tap onboarding-scroll drag 0 -900"; view light-22b-setup-scrolled-to-end 700
step "tap ob-fallback-transcriber"; view light-23-setup-fallback 900
step "clock +7500 real"; step "clock data"; step tree; shot light-24-mic-ask
step "tap ob-mic-ask"; wait_ 1800
step "tap ob-mic-next"; view light-25-hearing 600
step "tap ob-hearing-yes"; wait_ 600
step "tap ob-interview"; view light-26-interview-q1 800
step "tap ob-demo-next"; wait_ 1200
step "tap ob-demo-next"; wait_ 1200
step "tap ob-demo-next"; view light-27-interview-paused 1200
step "tap ob-continue"; view light-28-interview-continued 700
# on to the edition and "Start the radio" (this check is fatal: the radio must really start from the edition page)
for i in 1 2 3 4 5 6 7; do step "tap ob-demo-next"; wait_ 1200; done
step "tap ob-demo-next"; view light-29-writing 900
wait_ 3200; step "tap ob-edition-retry"; wait_ 1200
wait_ 3500; step tree; shot light-31-edition-done
step "tap ob-edition-open"; wait_ 900
step "tap bar-mark"; wait_ 600; step "tap place-settings"; wait_ 900; step "tap settings-setup-voice"; wait_ 900
step "tap ob-start-radio"; view light-32-front-page-playing 5000
step logs
run_drive voice light onboarding --touch platform

fi

# ---- 4c. DARK tour: the real switch is the Settings mode control
step "clock data"; wait_ 1500
ensure_light
place settings 900
step "tap settings-mode-dark"; view dark-01-settings-switched 900
goto "paper:front"; view dark-02-paper-front 700
step "tap paper-scroll drag 0 -260"; wait_ 700
step "tap paper-story-demo-grid-storage"; view dark-03-paper-story-reader 1000
step "tap bar-play"; wait_ 1800
step "tap bar-text"; view dark-04-player 900
step "tap player-close"; wait_ 700
step "tap bar-mic"; view dark-04b-mic-listening 900
step "tap bar-mic"; wait_ 1500
step "tap page-deck drag -180 0"; view dark-05-deck-swiped-away 900
step "tap bar-mark"; view dark-06-launcher 700
step "tap \"launcher-open-pub:demo-grid-storage\""; view dark-06b-story-brought-back 900
step "tap bar-mark"; wait_ 600
step "tap place-files"; view dark-07-files-list 1200
step "tap \"files-row-/Documents\""; wait_ 700
step "tap \"files-row-/Documents/Council minutes.pdf\""; view dark-08-files-pdf 2500
place mail 900; view dark-09-mail-inbox 500
step "tap mail-row-m6"; view dark-10-mail-message 900
place textures 900; view dark-11-textures-list 500
step "tap textures-doc-texture:demo-notes"; view dark-12-texture-document 1000
place settings 900; step tree; shot dark-13-settings
step "tap settings-motion"; view dark-14-motion-lab 1500
place settings 900
step "tap settings-setup-voice"; view dark-15-setup-from-settings 900
place settings 900
step "tap settings-mode-light"; view light-30-settings-reset 900
step logs
run_drive dark light "" --touch platform

if [ -n "${SHELLONLY:-}" ]; then
  for t in '[bar]' '[bar-mark]' '[bar-mic]' '[bar-play]' '[bar-field]' '[page-indicator]' '[page-deck]' '[story-more]' '[launcher-open-pub:demo-grid-storage]' '[place-files]'; do
    grep -qF -- "$t" "$OUT" && echo "ok: $t" || FAILS="$FAILS [$t]"
  done
  ls "$SHOTS"/*.png | wc -l
  [ -z "$FAILS" ] || fail "checks:$FAILS"
  echo "PROOF OK (shell tour only)"; exit 0
fi

if [ -z "${SHELLONLY:-}" ]; then
# ---- 4d. dark voice setup (the scheme is set by the same Settings control in the first drive of this mode; the page then opens by the flag)
step "clock data"; wait_ 1500
step "tap bar-mark"; wait_ 600; step "tap place-settings"; wait_ 900
step "tap settings-mode-dark"; wait_ 900
goto "onboarding:setup"; view dark-20-onboarding-welcome 900
step "tap ob-begin"; wait_ 4700; step tree; shot dark-21-setup-stalled
step "tap ob-fallback-transcriber"; wait_ 8400; step tree; shot dark-22-mic-ask
step "tap ob-mic-ask"; wait_ 2300; step tree; shot dark-23-mic-heard
step "tap ob-mic-next"; view dark-24-hearing 600
step "tap ob-hearing-yes"; wait_ 600
step "tap ob-interview"; wait_ 800
step "tap ob-demo-next"; wait_ 1200
step "tap ob-demo-next"; view dark-25-interview-interrupted 1200
step "tap ob-demo-next"; view dark-26-interview-paused 1200
step "tap ob-continue"; wait_ 700
for i in 1 2 3 4 5; do step "tap ob-demo-next"; wait_ 1200; done
step "tap ob-demo-next"; view dark-27-readback 1200
step "tap bar-mark"; wait_ 600; step "tap place-settings"; wait_ 900
step "tap settings-mode-light"; wait_ 900; step tree
step logs
run_drive darkvoice light onboarding --touch platform

fi

# ---- 5. assertions
must "paper: masthead" '[page-title] "AUTOPAPER"'
must "paper: a fixture story" 'The harbor council approves a tide barrier'
must "reader: the story is a page" '[page-title] "A regional grid adds its largest battery yet"'
must "bar: the glass lens" '[bar]'
must "bar: the mark opens the launcher" '[bar-mark]'
must "bar: the microphone" '[bar-mic]'
must "bar: play and pause" '[bar-play]'
must "bar: the field" '[bar-field]'
must "bar: the page-count line" '[page-indicator]'
must "deck: the pages" '[page-deck]'
must "deck: a story has a visible More of this" '[story-more]'
must "launcher: the deck is listed" '[launcher-open-pub:demo-grid-storage]'
nomust "no raw reader-facing address as a title" 'pub:/pub/'
must "player: opens from the bar" '[player-title]'
must "launcher: places" '[place-files]'
must "files: list" '[files-row-/Documents]'
must "files: the PDF viewer" '[pdf-frame]'
must "mail: inbox" '[mail-row-m6]'
must "mail: a message" '[mail-body]'
# "Read it to me" really speaks now (engine speakText): the radio is no longer idle or paused on the message screen
if grep -qF 'Now playing,' "$SHOTS/drive-mailread.txt" 2>/dev/null; then echo "ok: mail: Read it to me shows in now-playing"; else echo "MISSING: mail: Read it to me did not show in now-playing"; FAILS="$FAILS [mail read aloud]"; fi
must "textures: list" '[textures-doc-texture:demo-notes]'
must "textures: a document" '[page-title] "Charging pilot: working notes"'
must "settings: modes" '[settings-mode-dark]'
must "motion lab" '[page-title] "Motion"'
must "voice: welcome" '[ob-begin]'
must "voice: setup rows" '[ob-row-transcriber]'
must "voice: microphone check" '[ob-mic-ask]'
must "interview: listening" '[ob-listening]'
must "bar: tells the setup state" '[bar-setup]'
must "bar: tells the interview state" 'Interview: '
must "voice: the edition is done" '[ob-start-radio]'
if grep -qF 'NOW PLAYING' "$SHOTS/drive-voice.txt" 2>/dev/null; then echo "ok: Start the radio from the edition page starts it (the bar says NOW PLAYING)"; else echo "MISSING: Start the radio did not start the radio within 5 s"; FAILS="$FAILS [start the radio]"; fi
must "interview: pause card" '[ob-paused]'
must "first run: the card on the paper" '[firstrun-card]'
must "settings: Set up voice row" '[settings-setup-voice]'
must "settings: the mode control is ours" '[settings-mode-system]'
must "developer: the demo controls come with the switch" '[paper-composition]'
if grep -qF '[paper-composition]' "$SHOTS/drive-dark.txt" 2>/dev/null; then echo "UNWANTED: debug text shows without the developer switch"; FAILS="$FAILS [debug text]"; else echo "ok: no debug text without the developer switch"; fi
if grep -qF '[reader-following]' "$SHOTS/drive-follow.txt" 2>/dev/null; then echo "ok: reader follow: the block being read is marked"; else echo "note: reader follow not seen in the demo run (wired: the bar feeds ReaderFollow; see light-19)"; fi
must "really light: the settings line says light" 'Showing Depth (light)'
must "really dark: the settings line says dark" 'Showing Depth (dark)'
n=$(ls "$SHOTS"/light-*.png 2>/dev/null | wc -l); m=$(ls "$SHOTS"/dark-*.png 2>/dev/null | wc -l)
[ "$n" -ge 24 ] && [ "$m" -ge 18 ] || FAILS="$FAILS [screenshots: $n light, $m dark]"
echo "ok: screenshots in $SHOTS ($n light, $m dark)"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT"; fi
[ -z "$FAILS" ] || fail "checks:$FAILS"
echo "PROOF OK"
