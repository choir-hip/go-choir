#!/bin/sh
# Real-data proof: the app on a simulator of its own, NOT in demo mode, signed in to the test computer (d@e) on staging.
# Sign-in: the real one is a browser device flow a script cannot approve, so the key file (REAL_KEY_FILE, default the QA key for d@e)
# is copied with `cp` into the app's Documents as `proof-key`; the simulator build reads it into the Keychain at launch and deletes it
# (AutoradioKit/ProofKey.swift, compiled only for the simulator). The key is never printed, logged or put on a command line.
# Reads only: the tour taps nothing that writes or spends (no prompts, no mail). Set REAL_PROMPT=1 for the one optional
# prompt-bar question at the end (a model call on d@e).
# Assertions are on real conditions (a title is not empty, there is no error banner), never on fixture text.
# Last line: PROOF OK, or PROOF FAILED at <step>. Screenshots: $PROOF_SHOTS.
#   NOBUILD=1   skip the build        NOTESTS=1   skip swift test (the new decoders' tests only)
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-real-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
. "$HERE/proof-common.sh"
KEYFILE="${REAL_KEY_FILE:-$HOME/.config/choir-qa/keys/d-e.key}"
[ -r "$KEYFILE" ] || fail "key file" "cannot read the test key file"
echo "screenshots: $SHOTS"

# A real-mode clean app: demo OFF, no open pages, no first-run flags, the key file placed for the app to take at launch.
reset_state() {
  SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sleep 2
  SIM spawn "$EXACT_SIM" defaults delete $APP AutoradioDemo >/dev/null 2>&1
  SIM spawn "$EXACT_SIM" defaults delete $APP AutoradioOpenOnboarding >/dev/null 2>&1
  for k in talkMode swipeLeft swipeRight; do SIM spawn "$EXACT_SIM" defaults delete $APP "AutoradioSetting.$k" >/dev/null 2>&1; done
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  [ -n "$CONT" ] || fail "reset" "no app container (is the app installed?)"
  S="$CONT/Library/Application Support/Autoradio"
  rm -f "$S/open-pages.json" "$S/onboarding.json" "$S/interview.json" "$S/voice-setup.json" "$S/interview.md" "$S/signals.jsonl" "$S/recent-files.json"
  rm -rf "$S/themes" "$CONT/Library/Caches/Autoradio/files"
  rm -f "$CONT/Library/Preferences/$APP.plist"
  mkdir -p "$CONT/Documents"
  cp "$KEYFILE" "$CONT/Documents/proof-key" || fail "reset" "could not place the key file"
  chmod 600 "$CONT/Documents/proof-key"
}

# ids found in a drive's tree dump: ids <file> <prefix>  -> every test id that starts with the prefix (one per line, in order)
ids() { grep -o "\[$2[^]]*\]" "$1" 2>/dev/null | sed 's/^\[//; s/\]$//' | awk '!seen[$0]++'; }

# ---- 1. new decoders' tests (the whole package runs in proof-app.sh)
export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOTESTS:-}" ]; then
  echo "swift test (RealShapeTests, ProofKeyTests)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter "RealShapeTests|ProofKeyTests" > "$SCRATCH/test.txt" 2>&1 \
    || { grep -E "error:|failed|XCTAssert" "$SCRATCH/test.txt" | cut -c1-400 | head -40; tail -6 "$SCRATCH/test.txt"; cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"; fail "swift test"; }
  grep -E "Executed [0-9]+ tests" "$SCRATCH/test.txt" | tail -1
  cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"
fi

# ---- 2. build
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -v '^warning' "$SCRATCH/build.txt" | head -60; tail -15 "$SCRATCH/build.txt"; cp "$SCRATCH/build.txt" "$SHOTS/build.txt"; fail "build"; }
fi
SIM privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

# ---- 3a. SURVEY drive: every place once, trees dumped so the next drive can pick real ids
step "clock data"; wait_ 6000
view real-00-first-run 800
step "tap ob-skip"; wait_ 2500
view real-01-paper-front 1500
step "tap paper-scroll drag 0 -500"; view real-02-paper-scrolled 900
step "tap paper-scroll drag 0 -700"; view real-03-paper-scrolled-more 900
place settings 900; view real-04-settings 600
step "tap settings-nav-account"; view real-05-settings-account 3000
step "tap settings-back"; wait_ 500
step "tap settings-nav-computer"; view real-06-settings-computer 3500
step "tap settings-back"; wait_ 500
step "tap settings-nav-appearance"; view real-07-settings-appearance 900
place textures 900; view real-08-textures-list 3500
place files 900; view real-09-files-root 3500
place mail 900; view real-10-mail-inbox 3500
step logs
run_drive survey light "" --touch platform

STORY1=$(ids "$SHOTS/drive-survey.txt" "paper-story-" | sed -n 1p)
STORY2=$(ids "$SHOTS/drive-survey.txt" "paper-story-" | sed -n 4p)
PLAY1=$(ids "$SHOTS/drive-survey.txt" "paper-play-" | sed -n 2p)
DOC1=$(ids "$SHOTS/drive-survey.txt" "textures-doc-" | sed -n 3p)
# a document by its title: the id of the row whose card holds the title text (the tree lists the row, then its texts)
docid() { awk -v t="$2" '/\[textures-doc-texture:/ { match($0, /\[textures-doc-texture:[^]]*\]/); id=substr($0, RSTART+1, RLENGTH-2) } index($0, "\"" t "\"") && id != "" { print id; exit }' "$1"; }
THEME_ROW=$(docid "$SHOTS/drive-survey.txt" "Autoradio theme test")
BIG_ROW=$(docid "$SHOTS/drive-survey.txt" "Master document")
DIR1=$(ids "$SHOTS/drive-survey.txt" "files-row-" | sed -n 1p)
MAIL1=$(ids "$SHOTS/drive-survey.txt" "mail-row-" | sed -n 1p)
THEME_DOC=${THEME_ROW#textures-doc-texture:}
echo "picked: theme=${THEME_DOC:+yes} big=${BIG_ROW:+yes} story1=${STORY1:+yes} story2=${STORY2:+yes} play=${PLAY1:+yes} doc=${DOC1:+yes} dir=${DIR1:+yes} mail=${MAIL1:+yes}"

# ---- 3b. READ drive: two real stories in the reader, one played, one Texture, a folder, a message
step "clock data"; wait_ 6000
step "tap ob-skip"; wait_ 2500
[ -n "$STORY1" ] && { step "tap \"$STORY1\""; view real-11-story-1 4000; step "tap reader-scroll drag 0 -600"; view real-12-story-1-scrolled 900; step "tap reader-scroll drag 0 -900"; view real-13-story-1-more 900; }
goto "paper:front"; wait_ 1200
[ -n "$STORY2" ] && { step "tap \"$STORY2\""; view real-14-story-2 4000; step "tap reader-scroll drag 0 -2500"; view real-15-story-2-end 900; }
goto "paper:front"; wait_ 1200
[ -n "$PLAY1" ] && { step "tap \"$PLAY1\""; wait_ 5000; step tree; shot real-16-playing; step "tap bar-text"; view real-17-player 1200; step "tap player-close"; wait_ 700; }
place textures 900; wait_ 2500
[ -n "$DOC1" ] && { step "tap \"$DOC1\""; view real-18-texture-document 4000; }
place files 900; wait_ 2500
[ -n "$DIR1" ] && { step "tap \"$DIR1\""; view real-19-files-folder 3500; }
place mail 900; wait_ 2500
[ -n "$MAIL1" ] && { step "tap \"$MAIL1\""; view real-20-mail-message 3500; }
step logs
run_drive read light "" --touch platform

# ---- 3c. THEME drive: a theme from a Texture, loaded through Settings (the document "Autoradio theme test" on d@e holds one choir-theme block)
if [ -n "$THEME_DOC" ]; then
  step "clock data"; wait_ 5000
  step "tap ob-skip"; wait_ 2000
  place settings 900
  step "tap settings-nav-appearance"; wait_ 700
  step "type settings-theme-doc $THEME_DOC"; wait_ 500
  step "tap settings-theme-load"; view real-21-theme-loaded 3500
  step "tap settings-back"; wait_ 600; view real-22-settings-with-theme 700
  step logs
  run_drive theme light "" --touch platform
fi

# ---- 3d. BIG drive: the largest document on d@e (hundreds of versions) opens, or says plainly why not
if [ -n "$BIG_ROW" ]; then
  step "clock data"; wait_ 5000
  step "tap ob-skip"; wait_ 2000
  place textures 900; wait_ 2500
  step "tap \"$BIG_ROW\""; view real-23-big-document 9000
  step logs
  run_drive big light "" --touch platform
fi

# ---- 4. assertions on real conditions
if grep -lE "choir_sk_[A-Za-z0-9]" "$SHOTS"/*.txt "$OUT" >/dev/null 2>&1; then fail "key leak" "a key-shaped string is in the drive output"; fi
SV="$SHOTS/drive-survey.txt"; RD="$SHOTS/drive-read.txt"
chk() { # name pattern file
  if grep -qE -- "$2" "$3"; then echo "ok: $1"; else echo "MISSING: $1"; FAILS="$FAILS [$1]"; fi
}
bad() { # name pattern file
  if grep -qE -- "$2" "$3"; then echo "UNWANTED: $1: $(grep -m1 -E -- "$2" "$3" | cut -c1-160)"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi
}
chk "not demo mode: no demo switch on the settings page" '\[settings-nav-account\]' "$SV"
bad "not demo mode: no demo fixture story" 'harbor council approves a tide barrier' "$SV"
chk "paper: the masthead" '\[page-title\] "AUTOPAPER"' "$SV"
chk "paper: real stories (route ids)" '\[paper-story-/pub/' "$SV"
if grep -A3 '\[paper-story-/pub/' "$SV" | grep -qE 'Text#[0-9]+ "[^"]{10,}"'; then echo "ok: paper: a story title is not empty"; else echo "MISSING: paper: a story title is not empty"; FAILS="$FAILS [story title]"; fi
bad "paper: no error banner" '\[paper-status\] "(Could not|Offline)' "$SV"
bad "paper: no sign-in card" '\[paper-signin-card\]' "$SV"
chk "settings: the account shows a handle" '\[settings-account-handle\][^"]*"writer-' "$SV"
bad "settings: account has no failure message" '\[settings-account-message\] "(This|Could|The server)' "$SV"
chk "textures: documents listed" '\[textures-doc-texture:' "$SV"
chk "textures: no edition yet shows the make-your-edition card" '\[textures-make-edition\]' "$SV"
bad "textures: no error" '\[textures-message\] "(Could|Sign in)' "$SV"
chk "files: root listing" '\[files-row-/' "$SV"
chk "mail: reached the mail place" '\[page-title\] "(Mail|Inbox)"' "$SV"
bad "mail: no error screen" '\[mail-status\] "(Could not|You are offline)' "$SV"
[ -n "$STORY1" ] && chk "reader: story 1 has a title" '\[page-title\] "[^"]{3,}"' "$RD"
bad "reader: no error message" '\[reader-message\] "(Could not|Sign in)' "$RD"
chk "reader: body blocks" '\[reader-block-' "$RD"
chk "reader: more on this" '\[reader-related-pub:' "$RD"
chk "radio: playing a real story" 'NOW PLAYING|Now playing,' "$RD"
[ -n "$DOC1" ] && chk "texture document: opened" '\[reader-block-' "$RD"
if [ -n "$THEME_DOC" ]; then
  TH="$SHOTS/drive-theme.txt"
  chk "theme: the Texture's theme is listed" '\[settings-theme-depth-teal-test\]' "$TH"
  bad "theme: no problems reported" '\[settings-theme-problems\]' "$TH"
fi
n=$(ls "$SHOTS"/real-*.png 2>/dev/null | wc -l)
[ "$n" -ge 18 ] || FAILS="$FAILS [screenshots: $n]"
echo "ok: screenshots in $SHOTS ($n)"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT" | sed -E 's/choir_sk_[A-Za-z0-9]+/<key>/g' | cut -c1-240; fi
[ -z "$FAILS" ] || fail "checks:$FAILS"
echo "PROOF OK"
