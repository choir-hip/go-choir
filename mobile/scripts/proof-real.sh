#!/bin/sh
# Real-data proof: the app on a simulator of its own, signed in to the test computer (d@e) on staging.
# Sign-in: the real one is a browser device flow a script cannot approve, so the key file (REAL_KEY_FILE, default the QA key for d@e)
# is copied with `cp` into the app's Documents as `proof-key`; the simulator build reads it into the Keychain at launch and deletes it
# (AutoradioKit/ProofKey.swift, compiled only for the simulator). The key is never printed, logged or put on a command line.
# Reads only on the computer: the tour taps nothing that writes or spends (no prompts, no mail). The one local write is the theme choice on the phone (the Appearance tap; reset_state clears it). Set REAL_PROMPT=1 for the one optional
# prompt-bar question at the end (a model call on d@e).
# Assertions are on real conditions (a title is not empty, there is no error banner), never on fixture text.
# Last line: PROOF OK, or PROOF FAILED at <step>. Screenshots: $PROOF_SHOTS.
#   NOBUILD=1   skip the build        NOTESTS=1   skip swift test (the new decoders' tests only)
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-real-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
rm -f "$SHOTS"/real-*.png      # a reused PROOF_SHOTS must not satisfy the screenshot count with an earlier run's pictures
. "$HERE/proof-common.sh"
KEYFILE="${REAL_KEY_FILE:-$HOME/.config/choir-qa/keys/d-e.key}"
[ -r "$KEYFILE" ] || fail "key file" "cannot read the test key file"
echo "screenshots: $SHOTS"

# A clean app: no open pages, no first-run flags, the key file placed for the app to take at launch.
reset_state() {
  SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sleep 2
  # A fresh install has NO theme chosen: the default pair shows (Chrome Pastel in light, Liquid Metal in dark; themes.test.ts pins that). The simulator's preferences daemon keeps the OLD keys
  # after the plist removal below, and the one-time carry-over (themes.ts) reads them as the person's choice: both are deleted here (the carry-over drill seeds them again on purpose).
  for k in AutoradioThemeID AutoradioThemeMode; do SIM spawn "$EXACT_SIM" defaults delete $APP $k >/dev/null 2>&1; done
  for k in talkMode swipeLeft swipeRight; do SIM spawn "$EXACT_SIM" defaults delete $APP "AutoradioSetting.$k" >/dev/null 2>&1; done
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  [ -n "$CONT" ] || fail "reset" "no app container (is the app installed?)"
  S="$CONT/Library/Application Support/Autoradio"
  rm -f "$S/open-pages.json" "$S/onboarding.json" "$S/edition-topics.json" "$S/voice-setup.json" "$S/signals.jsonl" "$S/recent-files.json"
  rm -rf "$S/themes" "$CONT/Library/Caches/Autoradio/files"
  # The takes' data: the shared file (app:/data/takes.json, take-session.ts) and the Swift programme's copies (takes-own.json, takes-muted.json). Every drive starts without them; the takes drive seeds the old two below.
  rm -f "$S/takes-own.json" "$S/takes-muted.json" "$S/takes-copy-own.json" "$S/takes-copy-muted.json" "$CONT"/Library/Application\ Support/exact/*/data/takes.json "$CONT"/Library/Caches/exact/*/agent/*/data/takes.json
  rm -f "$CONT"/Library/Application\ Support/exact/*/data/edition-topics.json "$CONT"/Library/Caches/exact/*/agent/*/data/edition-topics.json   # the edition picks' own place now (edition-picks.ts: app:/data)
  rm -rf "$CONT"/Library/Application\ Support/exact/*/data/theme-choice.json "$CONT"/Library/Application\ Support/exact/*/data/themes   # app:/data on a device build
  rm -rf "$CONT"/Library/Caches/exact/*/agent/*/data/theme-choice.json "$CONT"/Library/Caches/exact/*/agent/*/data/themes   # the themes' own place now (themes.ts: app:/data, which an agent drive keeps under Library/Caches/exact/<app id>/agent/default/data), not UserDefaults and Autoradio/themes
  rm -f "$CONT/Library/Preferences/$APP.plist"
  mkdir -p "$CONT/Documents"
  if [ -n "${NOKEY:-}" ]; then rm -f "$CONT/Documents/proof-key"      # a launch with no key at all (the sign-out drill before it emptied the Keychain)
  else
    cp "$KEYFILE" "$CONT/Documents/proof-key" || fail "reset" "could not place the key file"
    chmod 600 "$CONT/Documents/proof-key"
  fi
  # The carry-over drill (SEED_LEGACY=1) seeds what an old build left, in THIS simulator's app container only: two preference keys (AutoradioThemeID=depth, AutoradioThemeMode=dark)
  # and one file, Application Support/Autoradio/themes/drill-user.json (a valid theme with the id drill-user). Nothing else; legacy_clean removes all three.
  # The takes drills (SEED_TAKES=1) seed what the old build kept, in THIS simulator's app container only: Application Support/Autoradio/takes-own.json (a handle and the own ids in TAKES_SEED_IDS, a
  # comma-separated list of quoted ids) and takes-muted.json (two muted voices). Whether shared code adopts them is its decision, made with the server's proof (take-session.ts refresh); the drills check both answers.
  if [ -n "${SEED_TAKES:-}" ]; then
    mkdir -p "$S"
    printf '{"handle":"voice-drill0001","ids":[%s]}' "$TAKES_SEED_IDS" > "$S/takes-own.json"
    printf '%s' '["voice-drillm1","voice-drillm2"]' > "$S/takes-muted.json"
  fi
  if [ -n "${SEED_LEGACY:-}" ]; then
    SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth; SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode dark
    mkdir -p "$S/themes"
    printf '%s' '{"schema_version":3,"id":"drill-user","name":"Drill user","schemes":["light","dark"],"colors":{"paper":{"light":"#FFFFFF","dark":"#101418"},"fg":{"light":"#111111","dark":"#F2F2F2"},"muted":{"light":"#444444","dark":"#C8C8C8"},"accent":{"light":"#0B5CAD","dark":"#7FB8F5"},"onAccent":{"light":"#FFFFFF","dark":"#101418"}}}' > "$S/themes/drill-user.json"
  fi
  # The kept first-run progress (app:/data/onboarding.json; shared code reads it, onboarding.ts parseProgress) is what says whether the radio may start: a launch whose first run is not over does not play
  # (radio-bridge.ts `ready`, by design). The drives before the first-run drive depended on what the PREVIOUS run left in the simulator (train2p3 began with the first run over and the radio played in every
  # drive; train2p4 began after a signed-out drive had removed it, `pulse ready=false` in 14 drives, no radio, "radio: playing a real story" missing). SEED_ONB_DONE=1 (set below, until the signed-out tour,
  # which asserts a first launch) writes "the edition is done" into each data directory that exists, so every drive starts from the same state whatever the run before it left.
  if [ -n "${SEED_ONB_DONE:-}" ]; then
    for d in "$CONT"/Library/Caches/exact/*/agent/*/data "$CONT"/Library/Application\ Support/exact/*/data; do
      [ -d "$d" ] && printf '%s' '{"offered":true,"editionDone":true}' > "$d/onboarding.json"
    done
  fi
  # The edition-picks drill (SEED_PICKS=1) seeds the OLD Swift file with one topic. This build never reads or deletes it: the drill asserts it is still there and nothing of it was adopted.
  if [ -n "${SEED_PICKS:-}" ]; then
    mkdir -p "$S"; printf '%s' '{"topics":["Zzdrillseed"]}' > "$S/edition-topics.json"
  fi
}
legacy_clean() {
  for k in AutoradioThemeMode AutoradioThemeID; do SIM spawn "$EXACT_SIM" defaults delete $APP $k >/dev/null 2>&1; done
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null); [ -n "$CONT" ] && rm -rf "$CONT/Library/Application Support/Autoradio/themes"
}

# ids found in a drive's tree dump: ids <file> <prefix>  -> every test id that starts with the prefix (one per line, in order)
ids() { grep -o "\[$2[^]]*\]" "$1" 2>/dev/null | sed 's/^\[//; s/\]$//' | awk '!seen[$0]++'; }

# Paper content is LIVE: the front page replaces its stories within minutes, and a tap on an id read from an earlier drive fails with "no view matches paper-story-..." (it did, in two lanes, in the read drive).
# So no drive taps a story or a play button picked by another drive. fresh_paper <name> runs a short drive of its own (open the paper, read the tree) right before the drive that taps, and sets
# F_STORY1, F_STORY2 (stories at the first card's depth that the server can resolve: a live front page sometimes holds one that answers 502 "failed to resolve publication", the server's, not the app's)
# and F_PLAY (the first play button). The caller copies what it needs into its own names.
# pick_stories <tree file> <how many>: the resolvable ids, in page order. (Only cards at the first card's depth: a card nested deeper sits in a smaller band, and a tap on one of those sometimes does
# not press, which is a script risk, not an app one.)
pick_stories() {
  first_indent=$(grep -m1 '\[paper-story-/pub/' "$1" | sed -E 's/^( *).*/\1/' | awk '{ print length($0) }')
  [ -n "$first_indent" ] || return 0
  n=0
  for id in $(awk -v n="$first_indent" '/\[paper-story-\/pub\// { match($0, /^ */); if (RLENGTH == n) { match($0, /\[paper-story-[^]]*\]/); id = substr($0, RSTART + 1, RLENGTH - 2); if (!seen[id]++) print id } }' "$1" | head -8); do
    [ "$n" -lt "$2" ] || break
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 "https://choir.news/api/platform/publications/resolve?route=${id#paper-story-}")
    [ "$code" = 200 ] && { echo "$id"; n=$((n + 1)); }
  done
}
fresh_paper() {
  step "clock data"; wait_ 6000
  place paper 2500
  step tree
  run_drive "$1" light "" --touch platform
  F_STORY1=""; F_STORY2=""
  PICKED=$(pick_stories "$SHOTS/drive-$1.txt" 2)
  F_STORY1=$(echo "$PICKED" | sed -n 1p); F_STORY2=$(echo "$PICKED" | sed -n 2p)
  F_PLAY=$(ids "$SHOTS/drive-$1.txt" "paper-play-" | sed -n 1p)
  [ -n "$F_STORY1" ] || echo "note: no top-band story resolved just before the $1 drive (staging answered non-200 for each, or the page drew none); its reader steps are skipped (docs/problems/staging-resolve-502-on-a-live-story-2026-11-19.md)"
}

# ---- 1. new decoders' tests (the whole package, the generators and the wiring greps run in scripts/check-all.sh, which needs no simulator; the lead runs it before a landing)
export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOTESTS:-}" ]; then
  echo "swift test (RealShapeTests, ProofKeyTests, TypedTurnTests)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter "RealShapeTests|ProofKeyTests|TypedTurnTests" > "$SCRATCH/test.txt" 2>&1 \
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
SEED_ONB_DONE=1
# The app's console for the whole proof (the simulator's log stream: no ring, unlike the journal the drive dumps read). run_drive adds each drive's window of it to that drive's dump (proof-common.sh).
PROOF_JSLOG="$SHOTS/proof-js-console.txt"
SIM spawn "$EXACT_SIM" log stream --level debug --style compact --predicate "process == \"$APP_NAME\" AND subsystem == \"$APP\"" > "$PROOF_JSLOG" 2>&1 &
PROOF_JSLOG_PID=$!

# ---- 3.0 WARM-UP drive: the first launch after an install showed Settings account, Files and Mail signed out for the whole drive in four of sixteen runs (the later drives of the same run were fine, with the
# same key). The cause is not proven (a first-launch race around the Keychain); the survey must not be that launch, so it is a throwaway one that only opens the app and waits.
step "clock data"; wait_ 6000
place settings 900; step "tap settings-nav-account"; view real-00w-warmup 4000
step logs
run_drive warm light "" --touch platform

# ---- 3a. SURVEY drive: every place once, trees dumped so the next drive can pick real ids
step "clock data"; wait_ 6000
view real-00-opens-on-radio 800   # the app opens on the Autoradio page (radio: is the first fixed page); there is no first-run page with a carried key
place paper 2500
view real-01-paper-front 1500
step "tap paper-scroll drag 0 -500"; view real-02-paper-scrolled 900
step "tap paper-scroll drag 0 -700"; view real-03-paper-scrolled-more 900
place settings 900; view real-04-settings 600
step "tap settings-nav-account"; view real-05-settings-account 3000
step "tap settings-back"; wait_ 500
step "tap settings-nav-computer"; view real-06-settings-computer 3500
step "tap settings-back"; wait_ 500
step "tap settings-nav-appearance"; view real-07-settings-appearance 900
# the themes are shared code (themes.ts): a tap on a non-default theme repaints the app, then the Default entry gives the pair back
step "tap settings-theme-lantern"; view real-07b-appearance-lantern 900
step "tap settings-theme-default"; wait_ 500
place textures 900; view real-08-textures-list 3500
place files 900; view real-09-files-root 3500
place mail 900; view real-10-mail-inbox 3500
place podcasts 900; view real-10b-podcasts 3500
place notebook 900; view real-10c-notebook 4000
step logs
run_drive survey light "" --touch platform

# ---- 3a2. SYSTEM DARK drive: the simulator's appearance is dark and the app has no choice and no mode (reset_state clears both), so the default pair follows the SYSTEM's scheme,
# read from exactViewport().prefersColorScheme in shared code (themes.ts). The appearance goes back to light when the drive ends or the script stops.
trap 'SIM ui "$EXACT_SIM" appearance light >/dev/null 2>&1; rm -rf "$SCRATCH"' EXIT
SIM ui "$EXACT_SIM" appearance dark >/dev/null 2>&1; sleep 2
echo "sysdark: the simulator's appearance is now: $(SIM ui "$EXACT_SIM" appearance 2>&1 | head -1)"
step "clock data"; wait_ 4000
place settings 900; view real-07c-system-dark 900; step logs
run_drive sysdark light "" --touch platform
SIM ui "$EXACT_SIM" appearance light >/dev/null 2>&1

# ---- 3a3. CARRY-OVER drill: an old build's choice (theme depth, mode dark) and one old user theme file are SEEDED (reset_state, SEED_LEGACY), the app launches, and the themes arrive in the new place:
# Depth in dark is shown, the user theme is listed, and the kept choice file holds them. The old keys and file are removed again afterwards.
trap 'SIM ui "$EXACT_SIM" appearance light >/dev/null 2>&1; legacy_clean; rm -rf "$SCRATCH"' EXIT
SEED_LEGACY=1; export SEED_LEGACY
step "clock data"; wait_ 5000
place settings 900; view real-07d-carried-over 900
wait_ 8000; view real-07e-carried-over-later 600     # the carry-over runs apart from the first paint: the later view is the one that must show it
step logs
run_drive migrate light "" --touch platform
unset SEED_LEGACY
CHK="$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)"
MIGRATED_FILE=$(ls "$CHK"/Library/Caches/exact/*/agent/*/data/theme-choice.json "$CHK"/Library/Application\ Support/exact/*/data/theme-choice.json 2>/dev/null | head -1)
[ -n "$MIGRATED_FILE" ] && cp "$MIGRATED_FILE" "$SHOTS/migrated-theme-choice.json"
ls "$CHK"/Library/Caches/exact/*/agent/*/data/themes/ "$CHK"/Library/Application\ Support/exact/*/data/themes/ 2>/dev/null > "$SHOTS/migrated-themes-dir.txt"
legacy_clean

# ---- 3a4. TAKES drill. Nothing is carried from the pre-release build (owner's ruling, 2026-11-20): its two files (takes-own.json with a handle and own ids, takes-muted.json with two muted voices) are SEEDED by
# reset_state (SEED_TAKES, TAKES_SEED_IDS) in the drive that is the first signed-in launch to see them, and the build must neither read, adopt from nor delete them: no muted voice is listed, nothing of them is
# in the kept file (app:/data/takes.json, which exists only when the server's `mine` gave this account a live take) or in the programme's copy (pushed by shared code, so it exists after the first refresh), and both
# old files are still there with their contents. The account wipe (the sign-out drill below) is what removes them.
# NOT covered: Record, Stop, Preview, Publish. The simulator has no microphone input the scripts can drive (the agent's microphone track is unused: testing-and-debugging.md, "Voice flows for agent tests"), and
# Publish would write a take to d@e. Those are in take-session.test.ts (S1 to S8, U3, B1) against a fake recorder, and on a phone by hand. The account-change case cannot be driven with one key.
SEED_TAKES=1; export SEED_TAKES
TAKES_SEED_IDS='"take-drilla","take-drillb"'; export TAKES_SEED_IDS
step "clock data"; wait_ 5000
place radio 1500
wait_ 9000                                            # the refresh runs apart from the first paint (a task, then the signed-in request)
for i in 1 2 3; do step "tap radio-scroll drag 0 -1500"; wait_ 300; done
view real-46-takes-old-files 900
step logs
run_drive takes light "" --touch platform
unset SEED_TAKES TAKES_SEED_IDS
CHK="$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)"
TKS="$CHK/Library/Application Support/Autoradio"
TK_FILE=$(ls -t "$CHK"/Library/Caches/exact/*/agent/*/data/takes.json "$CHK"/Library/Application\ Support/exact/*/data/takes.json 2>/dev/null | head -1)
rm -f "$SHOTS/takes-data.json" "$SHOTS/takes-old-own.json" "$SHOTS/takes-old-muted.json"
[ -n "$TK_FILE" ] && cp "$TK_FILE" "$SHOTS/takes-data.json"
cp "$TKS/takes-own.json" "$SHOTS/takes-old-own.json" 2>/dev/null
cp "$TKS/takes-muted.json" "$SHOTS/takes-old-muted.json" 2>/dev/null
tseg() { awk -v m="$2" '/^screenshot /{ if (index($0, m)) { printf "%s", buf; exit } buf=""; next } { buf = buf $0 "\n" }' "$1"; }
tseg "$SHOTS/drive-takes.txt" real-46-takes-old-files > "$SHOTS/seg-takes.txt"

# ---- 3a5. IMPORTED DOCUMENTS (the PDFs and e-books the person imports; files on their computer, AutoradioLab/imported.ts). Open the place from the launcher (its place-imported key): the list comes from the computer (the Imported
# documents folder, through the Files client), so on d@e it is empty or lists what is there; either way the page must draw its title and the import button, and say none of the old Reader's words.
# NOT covered: the system file picker (the simulator's picker cannot be driven by the scripts) and reading aloud (no audio); imported.test.ts drives the import, the open, the search and the words handed to the player
# against a fake computer, and the open of a real document is checked by hand on a phone.
step "clock data"; wait_ 5000
place imported 900
view real-47-imported 2500
step logs
run_drive imported light "" --touch platform
tseg "$SHOTS/drive-imported.txt" real-47-imported > "$SHOTS/seg-imported.txt"

# Fail loudly rather than quietly tour nothing: no story card at all means the paper page did not draw, which the tour must not pass over.
grep -q '\[paper-story-/pub/' "$SHOTS/drive-survey.txt" || fail "survey" "no paper-story- line in the survey tree: the paper page showed no story card"
# (Stories and the play button are NOT taken from the survey: it is minutes old by the time they are tapped. Each drive that taps paper content picks it fresh: fresh_paper.)
DOC1=$(ids "$SHOTS/drive-survey.txt" "textures-doc-" | sed -n 3p)
# a document by its title: the id of the row whose card holds the title text (the tree lists the row, then its texts)
docid() { awk -v t="$2" '/\[textures-doc-texture:/ { match($0, /\[textures-doc-texture:[^]]*\]/); id=substr($0, RSTART+1, RLENGTH-2) } index($0, "\"" t "\"") && id != "" { print id; exit }' "$1"; }
THEME_ROW=$(docid "$SHOTS/drive-survey.txt" "Autoradio theme test")
BIG_ROW=$(docid "$SHOTS/drive-survey.txt" "Master document")
DIR1=$(ids "$SHOTS/drive-survey.txt" "files-row-" | sed -n 1p)
MAIL1=$(ids "$SHOTS/drive-survey.txt" "mail-row-" | sed -n 1p)
# a file that opens WITHOUT writing anything: a table or a picture (a text or Markdown file is opened as a Texture document by the computer, which writes; a PDF the same)
FILE1=$(ids "$SHOTS/drive-survey.txt" "files-row-" | grep -iE '\.(csv|tsv|png|jpe?g|gif)$' | sed -n 1p)
POD1=$(ids "$SHOTS/drive-survey.txt" "podcast-row-" | sed -n 1p)
THEME_DOC=${THEME_ROW#textures-doc-texture:}
echo "picked: file=${FILE1:+yes} podcast=${POD1:+yes} theme=${THEME_DOC:+yes} big=${BIG_ROW:+yes} doc=${DOC1:+yes} dir=${DIR1:+yes} mail=${MAIL1:+yes}"

# Rows of the live front page sit wherever today's page puts them, and the bar's text field can cover one. Before a row is tapped the
# list is dragged back to its top (twice, so a long page is covered), where the first card is clear of the bar; the second card's title sits under the bar there, so its tap scrolls it up first.
to_top() { step "tap paper-scroll drag 0 700"; wait_ 400; step "tap paper-scroll drag 0 700"; wait_ 600; }

# ---- 3b. READ drive: two real stories in the reader, one played, one Texture, a folder, a message
fresh_paper pick-read
STORY1=$F_STORY1; STORY2=$F_STORY2; PLAY1=$F_PLAY
echo "picked (just before the read drive): story1=${STORY1:+yes} story2=${STORY2:+yes} play=${PLAY1:+yes}"
step "clock data"; wait_ 6000
place paper 2500
[ -n "$STORY1" ] && { to_top; step "tap \"$STORY1\""; view real-11-story-1 4000; step "tap reader-scroll drag 0 -600"; view real-12-story-1-scrolled 900; step "tap reader-scroll drag 0 -900"; view real-13-story-1-more 900; }
goto "paper:front"; wait_ 1200
[ -n "$STORY2" ] && { to_top; step "tap paper-scroll drag 0 -350"; wait_ 1800; step "tap \"$STORY2\""; view real-14-story-2 4000; step "tap reader-scroll drag 0 -2500"; view real-15-story-2-end 900; }
goto "paper:front"; wait_ 1200
[ -n "$PLAY1" ] && { to_top; step "tap \"$PLAY1\""; wait_ 5000; step tree; shot real-16-playing; step logs; step "tap bar-now"; view real-17-radio-page 1200; }
place textures 900; wait_ 2500
[ -n "$DOC1" ] && { step "tap \"$DOC1\""; view real-18-texture-document 4000; }
place files 900; wait_ 2500
[ -n "$DIR1" ] && { step "tap \"$DIR1\""; view real-19-files-folder 3500; }
place mail 900; wait_ 2500
[ -n "$MAIL1" ] && { step "tap \"$MAIL1\""; view real-20-mail-message 3500; }
step logs
run_drive read light "" --touch platform

# ---- 3b2. READER drive (shared code, reader.ts): one real story read twice. Part one opens it and dumps the page (title, text, citation marks, the Sources list, related); part two opens it again, reads which citation
# marks part one found, taps the first one (its card opens under the paragraph), then the page menu and its Versions sheet. Nothing is written to the computer. The ids tapped come from part one's own tree, not from a guess.
READER_STORY=${STORY1:-$STORY2}
if [ -n "$READER_STORY" ]; then
  step "clock data"; wait_ 6000
  place paper 2500; to_top; step "tap \"$READER_STORY\""; view real-r1-reader-story 5000
  step "tap reader-scroll drag 0 -1500"; view real-r2-reader-sources 1200
  step logs
  run_drive reader1 light "" --touch platform
  CITE1=$(ids "$SHOTS/drive-reader1.txt" "reader-cite-" | sed -n 1p)
  # Part two: where the citation sits. The agent's tap does not scroll a target into view at this pin (the drive failed twice on "outside the viewport", with and without a real touch), so the page is scrolled
  # explicitly, by the distance read from the node's own frame, to the middle of the screen (430), so a live story's next version, which moves the mark a few lines between the two launches, still leaves it in reach (train2p3's reader2 found the mark out of reach after 188 from a measure taken in another launch; its node ids showed 3 fewer nodes above the mark); the layout step before the tap leaves the frame it tapped at: `layout <id>` prints its "space viewport X,Y WxH" at the scroll the page opens at.
  CITE_SCROLL=""
  if [ -n "$CITE1" ]; then
    step "clock data"; wait_ 6000
    place paper 2500; to_top; step "tap \"$READER_STORY\""; wait_ 5000
    step "layout \"$CITE1\""
    run_drive reader1b light "" --touch platform
    CITE_SCROLL=$(layout_scroll "$SHOTS/drive-reader1b.txt" "$CITE1" 430) || fail "reader" "the layout of $CITE1 in $SHOTS/drive-reader1b.txt has no viewport frame (or the node is more than 30000 points down): the drive cannot scroll to the citation"
  fi
  step "clock data"; wait_ 6000
  place paper 2500; to_top; step "tap \"$READER_STORY\""; wait_ 5000
  [ -n "$CITE1" ] && { step "tap reader-scroll wheel 0 $CITE_SCROLL"; step "clock settle"; step "layout \"$CITE1\""; step "tap \"$CITE1\""; view real-r3-reader-citation-card 1500; step "tap reader-scroll wheel 0 -30000"; step "clock settle"; }
  step "tap reader-menu"; view real-r4-reader-menu 1200
  step "tap reader-menu-versions"; view real-r5-reader-versions 1500
  step logs
  run_drive reader2 light "" --touch platform
fi

# ---- 3c. THEME drive: a theme from a Texture, loaded through Settings (the document "Autoradio theme test" on d@e holds one choir-theme block)
if [ -n "$THEME_DOC" ]; then
  step "clock data"; wait_ 5000
  place paper 2000
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
  place paper 2000
  place textures 900; wait_ 6000        # the list of hundreds of documents is slow on a cold launch (the drive tapped it at 2.5 s and found no row)
  step "tap \"$BIG_ROW\""; view real-23-big-document 9000
  step logs
  run_drive big light "" --touch platform
fi

# ---- 3e. ASK drive (REAL_PROMPT=1 only: it is a model call on d@e): a typed question in the bar runs one turn on the computer
if [ -n "${REAL_PROMPT:-}" ]; then
  step "clock data"; wait_ 5000
  place paper 2000
  step "type bar-field \"What is the top story on the front page right now?\""; wait_ 500
  view real-24-asked 400
  step "tap bar-send"; wait_ 4000; step tree; shot real-25-asked-sent
  step "clock +40000 real"; step "clock data"; step tree; shot real-26-answer
  step "tap bar-answer-text"; wait_ 1500; view real-27-conversation 600       # the conversation page (conversation.ts rows over the native entries)
  step logs
  run_drive ask light "" --touch platform
fi

# ---- 3f. TOUR drive: a file that opens read only, the podcasts list and a show (no Subscribe), the notebook, the Settings account page once more
step "clock data"; wait_ 5000
place files 900; wait_ 2500
[ -n "$FILE1" ] && { step "tap \"$FILE1\""; view real-30-files-file 4000; }
place podcasts 900; view real-31-podcasts 3500
[ -n "$POD1" ] && { step "tap \"$POD1\""; view real-32-podcast-show 3500; }
place notebook 900; view real-33-notebook 4000
place settings 900; step "tap settings-nav-account"; view real-34-settings-account 3500
step "tap settings-back"; wait_ 500
step "tap settings-nav-about"; view real-35-settings-about 1500
step "tap settings-back"; wait_ 500
step "tap settings-nav-activity"; view real-36-settings-activity 3500
place mail 900; wait_ 2500
step "tap mail-folder-sent"; view real-37-mail-sent 3000
step "tap mail-folder-quarantine"; view real-38-mail-quarantine 3000
step logs
run_drive tour light "" --touch platform

# ---- 3f1. EDITION PICKS drive (shared code, edition-picks.ts; local writes only on the phone: nothing is written to the computer). The OLD Swift file is SEEDED with one topic and must come out
# of the drive UNREAD and still there (the build never reads or deletes it; an upgraded phone starts with no picks). The topic screen is opened from My edition (the page, not an assumption, says which
# control: with no picks it is "Choose topics"), three picks are made (a category, a subtopic under another, a typed one) and one is taken off. The stored file is copied out and read.
SEED_PICKS=1; export SEED_PICKS
step "clock data"; wait_ 6000
place paper 2500
step "tap paper-section-mine"; view real-70-edition-mine 1500
step "tap paper-choose-topics"; view real-71-topics-empty 1500
# A tap needs its target in the viewport (the driver cannot tap what the bar or the fold covers): Science sits near the top; after Culture opens the list is dragged up before Film, the field and Done are tapped,
# and dragged back up before Science is tapped again. Each target is also asserted to appear in the drive's dumped trees below (presence in the log, not proof of the viewport at the tap: a tap that missed shows up as a pick that is not stored).
step "tap ob-topic-Science"; wait_ 700
# `clock settle` after each drag: a real touch played while the list is still moving was lost ("no touch reached the app's window", the picks drive, tap Film); `layout` says where the target is if a tap is lost again.
# (the layout the run printed: after Culture opens the page has already scrolled it to the top, and a drag of -300 put Film at screen y -11, under the status bar: the drag is +150 now, Film comes down into the middle.)
step "tap ob-topic-open-Culture"; wait_ 700; step "tap edit-edition-scroll drag 0 150"; wait_ 500; step "clock settle"; step tree; step "layout ob-topic-Film"; step "tap ob-topic-Film"; wait_ 700
step "tap edit-edition-scroll drag 0 -700"; wait_ 500; step "clock settle"
# settle before the add tap: the keyboard is still moving after the typing, and the touch was lost once (trainp4: "no touch reached the app's window")
step "type ob-topic-input Zzdrillchess"; step "clock settle"; step "tap ob-topic-add"; view real-72-topics-picked 1200
# (the run: after the drag to the top Science sat under the bar's field, so the list is dragged back up 400 to put it clear of the bar.)
step "tap edit-edition-scroll drag 0 1500"; wait_ 500; step "tap edit-edition-scroll drag 0 -400"; wait_ 500; step "clock settle"; step "layout ob-topic-Science"; step "tap ob-topic-Science"; view real-73-topics-one-off 1200
step "tap edit-edition-scroll drag 0 -1500"; wait_ 500; step "clock settle"; step tree; step "tap edit-edition-done"; wait_ 2500
place paper 2000
step "tap paper-section-mine"; view real-74-edition-mine-after 2500
step logs
run_drive picks light "" --touch platform
unset SEED_PICKS

CHK="$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)"
PICKS_FILE=$(ls "$CHK"/Library/Caches/exact/*/agent/*/data/edition-topics.json "$CHK"/Library/Application\ Support/exact/*/data/edition-topics.json 2>/dev/null | head -1)
[ -n "$PICKS_FILE" ] && cp "$PICKS_FILE" "$SHOTS/picks-stored.json"
OLD_FILE="$CHK/Library/Application Support/Autoradio/edition-topics.json"
if [ -n "$CHK" ] && [ -f "$OLD_FILE" ]; then cp "$OLD_FILE" "$SHOTS/picks-old-file.json"; else rm -f "$SHOTS/picks-old-file.json"; fi

# ---- 3d2. FIRST RUN, COMPUTER, ACTIVITY AND ABOUT: shared code over the native facts (onboarding.ts, computer.ts, activity.ts, settings.ts aboutPage). A fresh app; the Settings pages once each, then the edit page.
step "clock data"; wait_ 6000
place settings 900
step "tap settings-nav-computer"; view real-80-computer 4500
step "tap settings-back"; wait_ 500
step "tap settings-nav-activity"; view real-81-activity 3500
place settings 900          # Activity is a page of its own (activity.contract has no back button): Settings is opened again from the launcher
step "tap settings-nav-about"; view real-82-about 1500
step "tap settings-back"; wait_ 500
place paper 2500
step "tap paper-section-mine"; wait_ 1500
step "tap paper-choose-topics"; view real-83-edit-page 1500
step "tap ob-topic-Science"; wait_ 700
step "tap edit-edition-done"; wait_ 2500
step logs
run_drive firstrun light "" --touch platform
FR="$SHOTS/drive-firstrun.txt"
chk "settings computer (shared code): the computer's id is shown" '\[settings-computer-id\] "[^-"]' "$FR"
bad "settings computer: the state is not still checking" '\[settings-computer-state\] "Checking' "$FR"
bad "settings computer: no failure message" '\[settings-computer-message\] "(Could|Not signed|This phone|Your account)' "$FR"
chk "settings activity (shared code, native facts): the radio's now line is listed" '\[activity-row-now-radio\]' "$FR"
chk "settings activity: the Now section is there" '\[activity-section-now\]' "$FR"
chk "settings about: the app's version is the bundle's (a native fact), not a dash" '\[settings-about-app\] "[^-"]' "$FR"
chk "first run: the edit page lists the topic tree (onboarding.ts editing)" '\[ob-topic-Science\]' "$FR"

# ---- 3f1. NOTEBOOK drive (lane exact-notebook-content): the notebook page and the scorecard from shared code under app:/data, a Refresh press that reads again, and the content sync that
# starts at launch (shared code: content-run.ts logs one line per kind, counts only). Reads nothing it writes: no tap on a line, whose ids depend on the account's own notebook.
step "clock data"; step logs          # the console is a ring of the last 4096 lines: the launch's (the content sync's) is read before the page's own traffic pushes it out
place notebook 900; view real-47-notebook 5000
step "tap notebook-refresh"; view real-48-notebook-refreshed 5000
step logs
run_drive notebook light "" --touch platform

# ---- 3f2. SPINE drive (local UI only, nothing is written to the computer): the bar, the launcher and its deck, the default themes and the mode switch.
# Ported from the demo-mode proofs (proof-sim, proof-lane1, proof-lane4, proof-lane7, proof-default-themes, proof-app), which are deleted with demo mode.
fresh_paper pick-spine
STORY_SPINE=$F_STORY1
step "clock data"; wait_ 5000
view real-60-bar 800
place paper 2500
[ -n "$STORY_SPINE" ] && { to_top; step "tap \"$STORY_SPINE\""; wait_ 3500; }
step "tap bar-mark"; wait_ 900; step tree; shot real-61-launcher
step "tap \"launcher-open-paper:front\""; wait_ 900
place settings 900; step "tap settings-nav-appearance"; wait_ 600
# the mode starts at "system" (the simulator may be in dark): choose light, which keeps the default pair (no theme is chosen)
step "tap settings-mode-light"; view real-62-default-light 900
step "tap settings-mode-dark"; view real-63-default-dark 1000
step "tap settings-mode-light"; wait_ 700
step "tap settings-theme-newsprint"; view real-64-newsprint-light 900
step "tap settings-mode-dark"; view real-65-newsprint-dark 900
step logs
run_drive spine light "" --touch platform

# ---- 3f2b. SEARCH drives (reads only: the launcher's paper-wide search, shared code (retrieval-run.ts `launcherSearch`, the resource `psr`) over the server's public route).
# One query the paper answers ("the": a substring of nearly every story) and one nothing answers. Each starts a fresh app and types once, then waits past the pause in typing (350 ms) and the answer.
step "clock data"; wait_ 4000
step "tap bar-mark"; wait_ 900
step "type launcher-search \"the\""; wait_ 1500; step logs; wait_ 1500; step logs; wait_ 1500; step tree; shot real-71-search-hits
step logs          # `logs` reads what is new: read in turn, the answer's line is taken before the ring (3000 lines, a busy second or two) drops it
run_drive search-hit light "" --touch platform
step "clock data"; wait_ 4000
step "tap bar-mark"; wait_ 900
step "type launcher-search \"zzqxjvwk\""; wait_ 1500; step logs; wait_ 1500; step logs; wait_ 1500; step tree; shot real-72-search-none
step logs
run_drive search-none light "" --touch platform

# ---- 3f2c. CARDS drive (writes only on this phone and one steer tap to the computer): the story cards and the launcher are shared code (cards.ts). "More of this" on a story in front says "Noted" and the paper's
# learned list holds the act; the launcher finds the Paper place by a typed word; Reset empties the list. Its own fresh story: no other drive's pick.
fresh_paper pick-cards
STORY_CARDS=$F_STORY1
if [ -n "$STORY_CARDS" ]; then
  step "clock data"; wait_ 4000
  place paper 2500
  to_top; step "tap \"$STORY_CARDS\""; wait_ 3500
  step "tap story-more"; wait_ 1500; step tree
  step "tap bar-mark"; wait_ 900; step "type launcher-search Paper"; view real-66-launcher-search 1200
  step "tap \"search-hit-paper:front\""; wait_ 600       # a search shows hits (search-hit-<address>); launcher-open-<address> is the deck's row, not on screen while a query is typed
  # the launcher keeps the typed query "Paper" (it lists hits, not the places): the field is emptied before Settings is picked
  step "clock settle"; step "tap bar-mark"; wait_ 600; step "type launcher-search \"\""; wait_ 800; step "clock settle"; step "tap place-settings"; wait_ 900; step "tap settings-nav-learned"; view real-67-learned 1500
  step "tap settings-learned-reset"; view real-68-learned-reset 1200
  step logs
  run_drive cards light "" --touch platform
fi

# ---- 3f3. BAR drive (reads only: it plays and pauses a real story): the bar's page model is shared code (bar.ts) over the native facts. On a page: the aim line names the page and the deck count; one story plays: the
# bar says NOW PLAYING with the story's title and a progress rule; play pauses it (PAUSED, same title); play resumes it; the run sheet page (the radio) lists the entry on air.
fresh_paper pick-bar
PLAY_BAR=$F_PLAY
step "clock data"; wait_ 6000
place paper 2500
step tree
view real-66-bar-on-page 600
[ -n "$PLAY_BAR" ] && { to_top; step "tap \"$PLAY_BAR\""; wait_ 5000; view real-67-bar-playing 800; step "tap bar-play"; wait_ 1500; view real-68-bar-paused 800; step "tap bar-play"; wait_ 3000; view real-69-bar-resumed 800; step "tap bar-now"; view real-70-bar-run-sheet 1500; view real-70b-elapsed-a 200; view real-70c-elapsed-b 5000; }   # the last two views: no tap between them, five seconds apart (the position must move by itself)
step logs
run_drive bar light "" --touch platform

# ---- 3f4. BACKGROUND drive (the radio plays on with the app not in front): the radio starts by itself (a signed-in computer, onboarding done) and the Home screen is put over the app for BG_SECONDS.
# Checks that can fail: at least five entries START on the native console (`on air <entry> voice=`) after the app left the front. What this proves: the radio (shared code's pump and hold, and the deck) keeps
# starting entries while the app is covered. What it cannot prove: the simulator does not suspend an app, so frozen shared code (an iPhone with the screen locked and no sounding audio) is NOT shown, and the
# order of the sheet is not read (the deck journal that said it is pruned when shared code acknowledges it). The held-entries-while-frozen case is owed on the owner's locked phone (docs/design/autoradio-in-exact-2026-11-18.md).
BG_SECONDS="${BG_SECONDS:-300}"
# A driven app cannot be put in the background: the driver calls it "suspended" and every op fails (run 59d6d809d: `clock +10000 real`: "the app left the foreground ... so it is suspended"), and the app exits
# when the driver's socket closes (Agent.exitAfterLanding, ExactKit/Agent.swift:198). So the radio is started by a PLAIN launch (no driver; what a phone does), the Home screen is put over it, and what is counted
# is the NATIVE console's `on air <entry> voice=` lines after that moment. A first drive (agent) shows the radio starts; this one shows it starts and goes on with no driver.
step "clock data"; wait_ 8000
view real-71-radio-starts 3000
step logs
run_drive background light "" --touch platform
reset_state light ""
BGCONSOLE="$SHOTS/bg-app-console.txt"
# Shared code's `console.log` lines go to the platform log of a native app (Exact js/src/console.js: a line with a method is written by the host); a plain launch has no driver, so they are read from
# the simulator's log stream (harness only: read-only, no app change).
BGJSLOG="$SHOTS/bg-js-console.txt"
SIM spawn "$EXACT_SIM" log stream --level debug --style compact --predicate "process == \"$APP_NAME\"" > "$BGJSLOG" 2>&1 &
BGSTREAM=$!
SIM launch --console "$EXACT_SIM" $APP > "$BGCONSOLE" 2>&1 &
BGLAUNCH=$!
sleep 45
BGCONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
BGLOG="$BGCONT/Library/Application Support/Autoradio/console.log"
echo "background: plain launch, 45 s in: $(grep -c 'AUTORADIO on air ' "$BGLOG") on-air lines in the native console; the app's own output so far:"; tail -8 "$BGCONSOLE" | cut -c1-220
echo "background: shared code's console in the plain launch (the simulator's log stream, $BGJSLOG: $(wc -l < "$BGJSLOG" | tr -d ' ') lines), the radio/onboarding/writer/deck/api lines:"; grep -E "(radio|onboarding|writer|deck|runsheet|api):" "$BGJSLOG" | tail -25 | cut -c1-260
BGDIR=$(ls -d "$BGCONT"/Library/Application\ Support/exact/*/data "$BGCONT"/Library/Caches/exact/*/agent/*/data 2>/dev/null | head -2 | tr '\n' ' '); echo "background: data dirs: $BGDIR; files: $(ls $BGDIR 2>/dev/null | tr '\n' ' ')"
BGT0=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
SIM launch "$EXACT_SIM" com.apple.springboard >/dev/null 2>&1           # the Home screen over the app: it is not frontmost
sleep "$BG_SECONDS"
kill $BGLAUNCH $BGSTREAM 2>/dev/null
sleep 2          # the log stream is flushed to its file as it stops
echo "background: laps 50 ms to 3 s apart (lap.ts: the gap between two consecutive lap lines of the shared console, from the log stream's own timestamps):"
awk '/console\] lap: / { split($2, t, ":"); ms = (t[1]*3600 + t[2]*60 + t[3]) * 1000; if (prev != "" && ms - pms > 50 && ms - pms < 3000) printf "  %d ms  from [%s] to [%s]\n", ms - pms, prev, substr($0, index($0, "lap: ") + 5); prev = substr($0, index($0, "lap: ") + 5); pms = ms }' "$BGJSLOG" | head -30
echo "background: the radio/hold/writer lines of shared code after the $BG_SECONDS s:"; grep -E "(radio|writer|deck|runsheet):" "$BGJSLOG" | tail -25 | cut -c1-260
if [ ! -f "$BGLOG" ]; then echo "MISSING: background: no native console ($BGLOG)"; FAILS="$FAILS [background journal]"
else
  BGN=$(awk -v t0="$BGT0" '$1 >= t0 && /AUTORADIO on air / && / voice=/' "$BGLOG" | wc -l | tr -d ' ')
  # Desk entries run 60 to 75 s, so 300 s holds four or five: three is the floor that still shows the radio advancing by
  # itself more than once. What silence would look like is checked beside it: after the first entry of the window, no
  # entry may start after a gap (native logs gap_ms only when an entry begins after silence).
  BGGAPS=$(awk -v t0="$BGT0" '$1 >= t0 && /AUTORADIO on air / && / gap_ms=/' "$BGLOG" | wc -l | tr -d ' ')
  if [ "$BGN" -ge 3 ]; then echo "ok: background: $BGN entries started on air after the app left the front (the Home screen over a plain-launched app, $BG_SECONDS s)"
  else echo "MISSING: background: needs at least 3 entries started on air after the app left the front; the native console shows $BGN since $BGT0"; FAILS="$FAILS [background entries]"; fi
  if [ "$BGGAPS" -le 1 ]; then echo "ok: background: $BGGAPS entries began after silence in the window (at most the opening's hand-off)"
  else echo "MISSING: background: $BGGAPS entries began after silence since $BGT0 (gap_ms lines in the native console)"; FAILS="$FAILS [background gaps]"; fi
fi
echo "not driven: background: no remote next (a script cannot press the lock screen), and no frozen shared code (the simulator does not suspend it): only the owner's locked phone shows those"
SIM launch "$EXACT_SIM" $APP >/dev/null 2>&1

# ---- 3g. SIGN-OUT DRILL, LAST (local: it signs the simulator's app out; the proof key has no key id, so nothing is revoked on the server and nothing else is written there).
# Data is on the pages first; then Sign out; then the account page says signed out, Files and Mail show no data, and the wipe marker and the stamp are gone from the app's files.
step "clock data"; wait_ 5000
place paper 2000
place files 900; view real-40-files-before 3000
place settings 900; step "tap settings-nav-account"; view real-41-account-before 3500
step "tap settings-signout"; wait_ 5000; step tree; shot real-42-account-after
place files 900; view real-43-files-after 3000
place mail 900; view real-44-mail-after 3000
place settings 900; step "tap settings-nav-account"; view real-45-account-after-again 3500
step logs
run_drive drill light "" --touch platform
DR="$SHOTS/drive-drill.txt"
AFTER="$SHOTS/drill-after.txt"; awk '/\[settings-account-signed\][^"]*"No"/ { f = 1 } f' "$DR" > "$AFTER"
CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
LEFT_FILES=$(find "$CONT" \( -name 'signout-pending' -o -name 'account-stamp' \) 2>/dev/null)
LEFT=$(printf '%s' "$LEFT_FILES" | grep -c . | tr -d ' ')

# A failed drill never signed out: the Keychain still holds the key, so the signed-out tour below launches a SIGNED-IN app and every check of it fails for that reason (train2p1). Say so once, so the cause is not sought in the tour.
case "$DRIVES_FAILED" in *drill*) echo "note: the sign-out drill failed, so the app below is still signed in: read the drill's failure first, not the signed-out checks";; esac

# ---- 3h. SIGNED-OUT TOUR (the drill above left the Keychain empty): launch with no key. The first screen (the radio) says what is true and offers the one way in; the paper lists the public stories and a
# story reads; the radio offers to read the public paper; Settings > Account has a Sign in button. Nothing here needs the owner's computer.
export NOKEY=1
unset SEED_ONB_DONE          # the signed-out tour is a first launch: the kept progress is removed below and not put back
fresh_paper pick-signedout
STORY_SO=$F_STORY1
# The first run is offered ONCE (onboarding.ts offer: "offered" is kept in app:/data/onboarding.json), and the pick drive above was a signed-out launch that used the offer: reset_state clears the old Swift
# file only. The signed-out tour is a FIRST launch, so the kept progress is removed here (the check below asserts what a first signed-out launch shows, as it always did).
SOCONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
[ -n "$SOCONT" ] || fail "signed out" "no app container: the kept first-run progress cannot be removed (the glob would have started at /Library)"
rm -f "$SOCONT"/Library/Caches/exact/*/agent/*/data/onboarding.json "$SOCONT"/Library/Application\ Support/exact/*/data/onboarding.json
step "clock data"; wait_ 6000
# the account card is answered in REAL time (the key read, the host fact): give the first screen several settle steps before it is read
wait_ 6000; wait_ 6000; wait_ 6000
step logs          # the launch's console (the first run's offer, the account state) before the 4096-line ring pushes it out
view real-50-so-first-screen 2500
place paper 2500; view real-51-so-paper 3500
# the end of the page clears the card: the last row is readable above it (read the screenshot)
for i in 1 2 3 4 5 6; do step "tap paper-scroll drag 0 -1500"; wait_ 300; done; view real-51b-so-paper-end 900; for i in 1 2 3 4 5 6; do step "tap paper-scroll drag 0 1500"; wait_ 300; done
[ -n "$STORY_SO" ] && { to_top; step "tap \"$STORY_SO\""; view real-52-so-story 4000; step "tap reader-menu"; wait_ 900; place radio 1500; view real-52b-so-after-sheet 2500; }
place radio 1500; view real-53-so-radio 2500
step "tap radio-play-public"; wait_ 6000; step tree; shot real-53b-so-playing
place settings 900; step "tap settings-nav-account"; view real-54-so-account 3000
place files 900; view real-55-so-files 2500
place textures 900; view real-56-so-textures 2500
step logs
run_drive signedout light "" --touch platform
kill "$PROOF_JSLOG_PID" 2>/dev/null; PROOF_JSLOG=
unset NOKEY
SO="$SHOTS/drive-signedout.txt"
# the FIRST screen's dump only: the text of the drive up to the first screenshot step (real-50), not the whole drive
SO1="$SHOTS/signedout-first.txt"; awk '{ print } /real-50-so-first-screen/ { exit }' "$SO" > "$SO1"

# ---- 4. assertions on real conditions
if grep -lE "choir_sk_[A-Za-z0-9]" "$SHOTS"/*.txt "$OUT" >/dev/null 2>&1; then fail "key leak" "a key-shaped string is in the drive output"; fi
SV="$SHOTS/drive-survey.txt"; RD="$SHOTS/drive-read.txt"
chk "settings: the account row is there" '\[settings-nav-account\]' "$SV"
chk "paper: the masthead" '\[page-title\] "AUTOPAPER"' "$SV"
chk "paper: real stories (route ids)" '\[paper-story-/pub/' "$SV"
if grep -A3 '\[paper-story-/pub/' "$SV" | grep -qE 'Text#[0-9]+ "[^"]{10,}"'; then echo "ok: paper: a story title is not empty"; else echo "MISSING: paper: a story title is not empty"; FAILS="$FAILS [story title]"; fi
bad "paper: no error banner" '\[paper-status\] "(Could not|Offline)' "$SV"
bad "paper: no sign-in card" '\[paper-signin-card\]' "$SV"
chk "settings: the account shows a handle" '\[settings-account-handle\][^"]*"writer-' "$SV"
bad "settings: account has no failure message" '\[settings-account-message\] "(This|Could|The server)' "$SV"
for T in default chrome-pastel depth lantern liquid-metal newsprint signal; do chk "themes (shared code): $T is listed" "\\[settings-theme-$T\\]" "$SV"; done
chk "themes (shared code): a non-default theme is applied by a tap (Lantern, light)" '\[settings-current\] "Showing Lantern in light' "$SV"
bad "themes (shared code): no theme problems on the survey" '\[settings-theme-problems\]' "$SV"
chk "themes (shared code): the system is dark, no choice: the dark default (Liquid Metal) shows" 'Showing Liquid Metal (\(dark\)| in dark)' "$SHOTS/drive-sysdark.txt"
bad "themes (shared code): the system is dark: the light default does not show" 'Showing Chrome Pastel' "$SHOTS/drive-sysdark.txt"
chk "themes carry-over: the old choice arrived (Depth, dark)" 'Showing Depth \(dark\)' "$SHOTS/drive-migrate.txt"
chk "themes carry-over: the old user theme is listed" '\[settings-theme-drill-user\]' "$SHOTS/drive-migrate.txt"
chk "themes carry-over: the kept choice file says theme depth, mode dark, migrated" '"theme":"depth","mode":"dark","migrated":true' "$SHOTS/migrated-theme-choice.json"
chk "themes carry-over: the user theme file is in the new place" 'drill-user.json' "$SHOTS/migrated-themes-dir.txt"
# the edition picks (shared code): every check reads a file or a tree and FAILS when it is missing
[ -s "$SHOTS/picks-stored.json" ] || { echo "MISSING: edition picks: the stored file app:/data/edition-topics.json was not found in the app container"; FAILS="$FAILS [picks file]"; }
fixed() { # name needle file : the file contains the fixed text
  if [ -s "$3" ] && grep -qF -- "$2" "$3"; then echo "ok: $1"; else echo "MISSING: $1: $2"; FAILS="$FAILS [$1]"; fi
}
nofixed() { # name needle file : the file exists and does not contain the fixed text
  if [ ! -s "$3" ]; then echo "MISSING: $1: $3 is not there"; FAILS="$FAILS [$1]"; elif grep -qF -- "$2" "$3"; then echo "UNWANTED: $1: $2"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi
}
fixed "edition picks: the stored file holds the subtopic (Film)" '"Film"' "$SHOTS/picks-stored.json"
fixed "edition picks: the stored file holds the typed topic (Zzdrillchess)" '"Zzdrillchess"' "$SHOTS/picks-stored.json"
nofixed "edition picks: the category taken off (Science) is not stored" '"Science"' "$SHOTS/picks-stored.json"
nofixed "edition picks: nothing of the seeded old file was adopted" 'Zzdrillseed' "$SHOTS/picks-stored.json"
fixed "edition picks: the old Swift file is still there, unread and unchanged" 'Zzdrillseed' "$SHOTS/picks-old-file.json"
fixed "edition picks: with no picks the first My edition says to choose topics (nothing was seeded)" '[paper-choose-topics]' "$SHOTS/drive-picks.txt"
fixed "edition picks: the topic screen listed the tree (Science in the viewport)" '[ob-topic-Science]' "$SHOTS/drive-picks.txt"
fixed "edition picks: the subtopic row (Film) was in the viewport when tapped" '[ob-topic-Film]' "$SHOTS/drive-picks.txt"
fixed "edition picks: the typed-topic field was on the screen" '[ob-topic-input]' "$SHOTS/drive-picks.txt"
fixed "edition picks: Done was on the screen when tapped" '[edit-edition-done]' "$SHOTS/drive-picks.txt"
fixed "edition picks: three picks made, the screen says it follows 3 topics" 'Your edition follows 3 topics' "$SHOTS/drive-picks.txt"
fixed "edition picks: one taken off, the screen says it follows 2 topics" 'Your edition follows 2 topics' "$SHOTS/drive-picks.txt"
fixed "edition picks: My edition, built from the saved picks, says where it is from" '[paper-mine-from]' "$SHOTS/drive-picks.txt"
chk "textures: documents listed" '\[textures-doc-texture:' "$SV"
if grep -qE '\[textures-make-edition\]' "$SV"; then echo "info: textures: the make-your-edition card shows (the drill computer has no edition)"; else echo "info: textures: no make-your-edition card (the drill computer has an edition now; that is state, not a defect)"; fi
bad "textures: no error" '\[textures-message\] "(Could|Sign in)' "$SV"
chk "files: root listing" '\[files-row-/' "$SV"
chk "mail: reached the mail place" '\[page-title\] "(Mail|Inbox)"' "$SV"
bad "mail: no error screen" '\[mail-status\] "(Could not|You are offline)' "$SV"
[ -n "$STORY1" ] && chk "reader: story 1 has a title" '\[page-title\] "[^"]{3,}"' "$RD"
bad "reader: no error message" '\[reader-message\] "(Could not|Sign in)' "$RD"
chk "reader: body blocks" '\[reader-block-' "$RD"
# "more on this" depends on the live stories (some have no related items: two runs lacked it with everything else fine), so it is told, not required
if grep -qE '\[reader-related-pub:' "$RD"; then echo "ok: reader: more on this"; else echo "info: reader: neither story had a 'more on this' list (live content, not a defect)"; fi
# "Hear it" depends on the live stories (a story has clips only when a podcast discussed it), so it is told, not required; but when the heading is drawn it must carry its rows
# (the shared storyClips answers it, not the native reader), each with its Play button, so a heading over nothing fails here.
# The public clip set says whether the story the drive opened (STORY1) has clips: when it does the heading is REQUIRED (a broken hearClips wiring or a refused row fails here, not skipped); when it does not, or the route is
# unreachable, it is told. The rows each draw a Play button (the tap itself is a drive of its own: it needs a podcast clip with audio, which only some stories have).
HEAR_ROUTE=${STORY1#paper-story-}
if [ -n "$STORY1" ] && curl -s --max-time 15 "https://choir.news/api/platform/podcast-clips?limit=200" | tr -d '\n ' | grep -qE "\"route\":\"/?${HEAR_ROUTE#/}\""; then
  chk "reader: hear it is drawn for a story the server has clips for" '\[reader-hear-it\]' "$RD"; chk "reader: hear it has its clip rows" '\[clip-clip-[0-9a-f]+\]' "$RD"; chk "reader: hear it rows draw a Play button" '\[clip-play-clip-[0-9a-f]+\]' "$RD"
elif grep -qE '\[reader-hear-it\]' "$RD"; then chk "reader: hear it has its clip rows" '\[clip-clip-[0-9a-f]+\]' "$RD"; chk "reader: hear it rows draw a Play button" '\[clip-play-clip-[0-9a-f]+\]' "$RD"; else echo "info: reader: the story has no podcast clips on the server (live content, not a defect)"; fi
chk "radio: playing a real story" 'NOW PLAYING|Now playing,' "$RD"
[ -n "$DOC1" ] && chk "texture document: opened" '\[reader-block-' "$RD"
if [ -n "$READER_STORY" ]; then
  R1="$SHOTS/drive-reader1.txt"; R2="$SHOTS/drive-reader2.txt"
  # the console lines of THESE drives only (the earlier `read` drive writes the same lines to $OUT, so $OUT would pass without them)
  chk "reader (shared code): the story was read by reader.ts in the reader drive" 'reader: story blocks=[1-9]' "$R1"
  chk "reader (shared code): the page in front was handed to the native voice and native held it" 'reader: handed to the voice blocks=[1-9]' "$R1"
  bad "reader: native did not refuse the hand-over" 'reader: the voice did not take the page' "$R1"
  chk "reader: the story has a title" '\[page-title\] "[^"]{3,}"' "$R1"
  bad "reader: no error message" '\[reader-message\] "(Could|Sign in)' "$R1"
  # a block's text is its sentence runs, children of the block's own line (`Text#N "..."`); blocks_have_text (proof-common.sh) needs 20 characters under each of the first two, so an empty block cannot pass
  if out=$(blocks_have_text "$R1" 2); then echo "ok: reader: the text is there (the first two blocks carry sentences)"; else echo "MISSING: reader: the text is there: $out"; FAILS="$FAILS [reader text]"; fi
  chk "reader: the kicker and the disclosure (a story from the paper)" '\[reader-disclosure\] "Written by AI' "$R1"
  chk "reader: a citation mark is in the text" '\[reader-cite-' "$R1"
  chk "reader: the Sources list is there and has a row" '\[reader-source-1\]' "$R1"
  if grep -qE '\[reader-related-pub:' "$R1"; then echo "ok: reader: related stories listed"; else echo "info: reader: this story had no related list (live content)"; fi
  [ -n "$CITE1" ] && chk "reader: tapping a citation opens its card" '\[reader-card-[^]]*\]' "$R2"
  chk "reader: the page menu opens" '\[reader-menu-versions\]' "$R2"
  chk "reader: the Versions sheet lists a version or says there is one" '\[reader-version-[0-9]+\]|This page has one version' "$R2"
fi
if [ -n "${REAL_PROMPT:-}" ]; then
  chk "typed ask: the engine took the typed words as the turn" 'top story on the front page' "$SHOTS/drive-ask.txt"
  bad "typed ask: no sign-in failure" 'Sign in to talk to your computer' "$SHOTS/drive-ask.txt"
  chk "conversation: the question is a row (shared code over the native entries)" '\[conversation-row-' "$SHOTS/drive-ask.txt"
  chk "retrieval: shared code answered the turn's ask for passages (retrieval-run.ts through retrievalAnswer)" 'AUTORADIO retrieval ask answered' "$SHOTS/drive-ask.txt"
  bad "retrieval: the ask did not time out (the source was never read, or the answer was lost)" 'AUTORADIO retrieval ask no-answer' "$SHOTS/drive-ask.txt"
fi
if [ -n "$THEME_DOC" ]; then
  TH="$SHOTS/drive-theme.txt"
  chk "theme: the Texture's theme is listed" '\[settings-theme-depth-teal-test\]' "$TH"
  bad "theme: no problems reported" '\[settings-theme-problems\]' "$TH"
fi
# the imported documents drill
chk "imported: the page is titled for what it holds" 'Imported documents' "$SHOTS/seg-imported.txt"
chk "imported: the import control is there (an error or a skeleton has none)" '\[reader-import\]' "$SHOTS/seg-imported.txt"
chk "imported: the list or the empty words drew" '\[reader-(empty|list)\]' "$SHOTS/seg-imported.txt"
bad "imported: no word of the old Reader or its shelf" 'Remove from Reader|Your books|Add an EPUB or PDF|[Ss]helf' "$SHOTS/seg-imported.txt"
bad "imported: no failure line" '\[reader-error\]' "$SHOTS/seg-imported.txt"
# the takes drill (nothing is carried from the pre-release build; Record and Publish are not driven, see 3a4)
chk "takes: the Takes section drew" '\[radio-takes\]' "$SHOTS/seg-takes.txt"
chk "takes: the record control is there" '\[takes-record\]' "$SHOTS/seg-takes.txt"
bad "takes: no old muted voice is listed" 'Never play takes by voice-drillm' "$SHOTS/seg-takes.txt"
if [ -f "$SHOTS/takes-data.json" ]; then bad "takes: nothing of the old files is in the kept file" 'take-drill|voice-drill' "$SHOTS/takes-data.json"; else echo "info: takes: no kept file (d@e has no live take), nothing to check in it"; fi
chk "takes: the old own-ids file is still there, unread" 'take-drilla' "$SHOTS/takes-old-own.json"
chk "takes: the old muted file is still there, unread" 'voice-drillm1' "$SHOTS/takes-old-muted.json"
# the tour
chk "settings: About names the app" '\[settings-about-app\]' "$SHOTS/drive-tour.txt"
# About's Build row is the commit the app was built from (build-info.gen.ts, written at build time) and it is the commit under test; the server row reads /health
BUILT9="$(git -C "$(dirname "$0")" rev-parse HEAD 2>/dev/null | cut -c1-9)"
bad "settings: About's Build is not unknown" '\[settings-about-build\] "unknown"' "$SHOTS/drive-tour.txt"
if [ -n "$BUILT9" ]; then chk "settings: About's Build is the built commit $BUILT9" "\\[settings-about-build\\] \"$BUILT9" "$SHOTS/drive-tour.txt"; else echo "info: no git here, About's Build not compared"; fi
bad "settings: About's server build was read" '\[settings-about-server\] "not reported"' "$SHOTS/drive-tour.txt"
chk "settings: About's server build is a commit" '\[settings-about-server\] "[0-9a-f]{9}' "$SHOTS/drive-tour.txt"
chk "settings: Activity page opened" '\[page-title\] "Activity"' "$SHOTS/drive-tour.txt"
chk "mail: the Sent folder tab" '\[mail-folder-sent\]' "$SHOTS/drive-tour.txt"
bad "mail: Sent and Quarantine show no error screen" '\[mail-error\]' "$SHOTS/drive-tour.txt"
# the bar drive (bar.ts over the native facts)
BR="$SHOTS/drive-bar.txt"
chk "bar: on a page the line names the page and counts the deck" '\[page-indicator\] "On: [^"]+ · [0-9]+ in the deck"' "$BR"
[ -n "$PLAY_BAR" ] || fail "bar" "no paper-play- id was picked just before the bar drive: the bar's playing, pausing and run sheet checks would be skipped"
if [ -n "$PLAY_BAR" ]; then
  chk "bar: a story is playing (NOW PLAYING with a title)" 'NOW PLAYING|Now playing, [^.]{3,}' "$BR"
  chk "bar: the progress rule is drawn" '\[bar-progress\]' "$BR"
  chk "bar: play pauses it (the bar says PAUSED)" 'PAUSED|Paused, [^.]{3,}' "$BR"
  chk "bar: the run sheet page shows the entry on air" '\[radio-onair-title\] "[^"]{3,}"' "$BR"
  # the run sheet page (radio-page.ts over the native facts): the date line, the status line while a story plays
  # the simulator shares the Mac's clock and zone: a date from 1970 (no clock) or the wrong day fails. The expected day is the one the drive ran on (run_drive wrote it when the dump was taken), so a run that crosses local midnight does not fail it.
  TODAY=$(sed -n 1p "$SHOTS/drive-bar.date" 2>/dev/null); TODAY_END=$(sed -n 2p "$SHOTS/drive-bar.date" 2>/dev/null)
  [ -n "$TODAY" ] || fail "bar" "the bar drive left no date stamp"
  chk "radio page: the date line is the day of the drive ($TODAY)" "\\[radio-date\\] \"($TODAY|$TODAY_END)\"" "$BR"
  bad "radio page: the on-air title is not the kind word (a title the play dropped)" '\[radio-onair-title\] "(Story|Opening|Page|Podcast|Take|Book|Mail|Result|Answer|Transition|Audio|Clip)"' "$BR"
  bad "radio page: Built on does not show a raw route" 'radio-builton-0\] label="Open /?pub/' "$BR"
  chk "radio page: the status line says On air or Paused" '\[radio-status\] "(On air|Paused)"' "$BR"
  bad "radio page: the status line is not an error" '\[radio-status\] "Can.t speak' "$BR"
  # The position moves while it plays with NO tap between two reads (the simulator proof only ever read after a tap, which a stale position passes: owner, on a phone, 2026-11-22). Two separate dumps of the
  # drive (tseg: the text before each screenshot), taken five seconds apart with nothing pressed between them. The SAME element is read in both (a story shows radio-elapsed, spoken, an estimate from the
  # voice's clock; podcast audio queue-elapsed), on the SAME entry (its title in both: the story must not have ended and another begun in the window), and an absent element fails.
  tseg "$BR" real-70b-elapsed-a > "$SHOTS/seg-elapsed-a.txt"; tseg "$BR" real-70c-elapsed-b > "$SHOTS/seg-elapsed-b.txt"
  ELID=""; E1=""; E2=""
  for id in radio-elapsed queue-elapsed; do
    A=$(grep -oE "\\[$id\\] \"[0-9:]+\"" "$SHOTS/seg-elapsed-a.txt" | head -1 | sed -E 's/^[^"]*"//; s/"$//'); B=$(grep -oE "\\[$id\\] \"[0-9:]+\"" "$SHOTS/seg-elapsed-b.txt" | head -1 | sed -E 's/^[^"]*"//; s/"$//')
    if [ -n "$A" ] && [ -n "$B" ]; then ELID=$id; E1=$A; E2=$B; break; fi
  done
  # The progress line carries its value too (radio.contract: the aria-label of radio-progress, "Progress 12.3 percent"): an entry with no estimate has no elapsed label, and its progress still moves.
  P1=$(grep -oE '\[radio-progress\][^"]*"Progress [0-9.]+ percent"' "$SHOTS/seg-elapsed-a.txt" | head -1 | sed -E 's/^[^"]*"//; s/"$//'); P2=$(grep -oE '\[radio-progress\][^"]*"Progress [0-9.]+ percent"' "$SHOTS/seg-elapsed-b.txt" | head -1 | sed -E 's/^[^"]*"//; s/"$//')
  T1=$(grep -oE '\[(radio-onair-title|bar-text)\] "[^"]*"' "$SHOTS/seg-elapsed-a.txt" | head -1); T2=$(grep -oE '\[(radio-onair-title|bar-text)\] "[^"]*"' "$SHOTS/seg-elapsed-b.txt" | head -1)
  if [ -z "$ELID" ] && { [ -z "$P1" ] || [ -z "$P2" ]; }; then echo "MISSING: position: neither an elapsed label (radio-elapsed or queue-elapsed) nor the progress value (radio-progress) is in BOTH of the two dumps taken with no tap between"; FAILS="$FAILS [position moves while playing]"
  elif [ -z "$T1" ] || [ "$T1" != "$T2" ]; then echo "MISSING: position: the entry on air is not the same in both dumps, or its title was not read ([$T1] / [$T2]), so the values cannot be compared"; FAILS="$FAILS [position moves while playing]"
  elif { [ -n "$ELID" ] && [ "$E1" != "$E2" ]; } || { [ -n "$P1" ] && [ -n "$P2" ] && [ "$P1" != "$P2" ]; }; then echo "ok: position: moved with no tap between two dumps of $T1 (${ELID:-no label}: $E1 -> $E2; $P1 -> $P2)"
  else echo "MISSING: position: nothing moved in five seconds with no tap (${ELID:-no label}: $E1 -> $E2; $P1 -> $P2)"; FAILS="$FAILS [position moves while playing]"; fi
  chk "position: the page still says On air at the second read" '\[radio-status\] "On air"' "$SHOTS/seg-elapsed-b.txt"
  rm -f "$SHOTS/seg-elapsed-a.txt" "$SHOTS/seg-elapsed-b.txt"
  bad "bar: no error line held the bar" '\[bar-text\] "(Could not|Something went wrong|The voice stopped)' "$BR"
fi
# the search drives: the paper-wide rows land under the typed query, and a query nothing matches says so (the answer is for THIS query: a stale or never-merged answer shows neither)
SH="$SHOTS/drive-search-hit.txt"; SN="$SHOTS/drive-search-none.txt"
chk "search: a query the paper answers lists a story row (search-hit-pub:)" '\[search-hit-pub:' "$SH"
chk "search: the paper itself answered and added rows beyond the local ones (the log line of shared code, not the local rows)" 'search: paper done found=[1-9][0-9]* extra=[1-9]' "$SH"
bad "search: the answered query does not stay on Searching the paper" '\[launcher-search-note\] "Searching the paper' "$SH"
bad "search: the answered query is not Nothing found" '\[launcher-(search-note|nohits)\] "(Nothing found|Could not search)' "$SH"
chk "search: a query nothing matches says Nothing found in the paper" '\[launcher-search-note\] "Nothing found in the paper' "$SN"
bad "search: the empty answer lists no story row" '\[search-hit-pub:' "$SN"
chk "search: the paper answered the nonsense query with nothing (done, found=0)" 'search: paper done found=0 extra=0' "$SN"
# the spine drive
SP="$SHOTS/drive-spine.txt"
chk "bar: the field, the microphone and play" '\[bar-field\]' "$SP"
chk "bar: voice button" '\[bar-mic\]' "$SP"
chk "bar: play is there" '\[bar-play\]' "$SP"
chk "launcher: the mark opens the layer" '\[launcher-layer\]' "$SP"
chk "launcher: the paper is in the deck" '\[launcher-open-paper:front\]' "$SP"
chk "launcher: places" '\[place-settings\]' "$SP"
# the cards drive: each check reads this drive's own log and fails on the screen it names
CD="$SHOTS/drive-cards.txt"
if [ -n "$STORY_CARDS" ]; then
  chk "cards: More of this on the story in front says Noted (a story-more tap that recorded nothing says More of this)" 'Noted' "$CD"
  chk "cards: the launcher finds the Paper place by a typed word (search-hit row, not the places grid)" '\[search-hit-paper:front\]' "$CD"
  chk "cards: what the paper has learned holds the act (a story-more that wrote nothing leaves no row)" '\[settings-learned-row\]' "$CD"
  chk "cards: Reset empties the list (the empty line appears only with no rows)" '\[settings-learned-empty\]' "$CD"
fi
chk "themes: a fresh install shows the default pair, light (Chrome Pastel)" 'Showing Chrome Pastel in light' "$SP"
chk "themes: the default pair, dark (Liquid Metal)" 'Showing Liquid Metal in dark' "$SP"
chk "themes: Newsprint light" 'Showing Newsprint in light' "$SP"
chk "themes: Newsprint dark" 'Showing Newsprint in dark' "$SP"
[ -n "$FILE1" ] && chk "files: a file opened" '\[(page-title|viewer-[a-z]+|csv-card|image-view)\]' "$SHOTS/drive-tour.txt"
chk "podcasts: the place opened" '\[(podcasts-[a-z]+|podcast-row-[^]]*)\]' "$SHOTS/drive-tour.txt"
chk "notebook: the page opened" '\[notebook-(page|title)\]' "$SHOTS/drive-tour.txt"
NB="$SHOTS/drive-notebook.txt"
chk "notebook: the page, from shared code" '\[notebook-page\]' "$NB"
chk "notebook: the held list is complete (its last row, 13)" '\[notebook-held-13\]' "$NB"
chk "notebook: the notebook was read (a section, or a plain notice that there is none or it is offline)" '\[notebook-(section-[a-z-]+|doc-notice)\]' "$NB"
# (Two "not stuck loading" checks were deleted: the only `loading` the page draws is its idle answer, so they could not catch a stuck read; the positive checks above are the proof of the read.)
# The sync starts at EVERY launch and logs when it ends. Two places can show it. A drive's dump reads Exact's journal ring, which a busy launch overflows before the dump (train2p3: 3947 to 27519 earlier lines
# dropped per drive; the sync's lines ran at launch, were logged, and were pushed out), so a drive may not hold them. The background drive's plain launch has no ring: its console is the simulator's log stream
# ($BGJSLOG, the same `console.log` lines, whole), which is where train2p3's lines are (03:48:13 story, 03:49:54 document). The sync ran at launch if either source shows it.
cat "$SHOTS"/drive-*.txt "$BGJSLOG" > "$SCRATCH/all-drives.txt" 2>/dev/null
chk "content sync: the shared sync ran at launch (stories: count and bytes logged)" 'content sync story: fetched [0-9]+, held [0-9]+, failed [0-9]+, [0-9]+ bytes' "$SCRATCH/all-drives.txt"
chk "content sync: documents ran too" 'content sync document: fetched [0-9]+, held [0-9]+, failed [0-9]+, [0-9]+ bytes' "$SCRATCH/all-drives.txt"
bad "content sync: stories did not stop on an error" 'content sync story: .*stopped: error' "$NB"
bad "content sync: the file walk is not a launch job" 'content sync file:' "$NB"
chk "settings account again: signed in" '\[settings-account-signed\][^"]*"Yes"' "$SHOTS/drive-tour.txt"
# the first shared answer after a launch must not run over the device's 100 ms turn (the app logs the failure that says so)
bad "no resource over the 100 ms turn" 'over the 100 ms' "$OUT"
# the reader is shared code on both platforms: it logs a count line (never a title) each time it reads a story or a document
# signed in: the sign-in card never shows on the signed-in drives
bad "signed in: no sign-in card on the survey" '\[signedout-banner\]' "$SV"
bad "signed in: no sign-in card on the read drive" '\[signedout-banner\]' "$RD"
bad "signed in: no sign-in card on the tour" '\[signedout-banner\]' "$SHOTS/drive-tour.txt"
# every Sign in action is the account state's: none of them may show on a signed-in phone (paper, onboarding, banner, settings)
for D in "$SV" "$RD" "$SHOTS/drive-tour.txt"; do
  bad "signed in: no Sign in action ($(basename "$D"))" '\[(paper-signin|paper-signin-card|signin|signedout-signin)\]' "$D"
done
# a momentary empty account state must not hide the write controls unnoticed: signed in, they ARE there
chk "signed in: Mail has New message" '\[mail-compose\]' "$SV"
chk "signed in: Files has Add a file" '\[files-upload\]' "$SV"
# the signed-out tour
# the first-run page signed out is its own step: a signed-out first launch opens on it, and the card must stand on it
chk "signed out: the first-run page is the first screen" 'Make it yours' "$SO1"
chk "signed out: the first screen has the banner" '\[signedout-banner\]' "$SO1"
chk "signed out: the first screen has the one way in" '\[signedout-signin\]' "$SO1"
chk "signed out: the radio says what it is" 'This is the public paper. Sign in to hear your own programme.' "$SO"
chk "signed out: the paper lists public stories" '\[paper-story-/pub/' "$SO"
[ -n "$STORY_SO" ] && chk "signed out: a story reads" '\[reader-block-' "$SO"
chk "signed out: the radio offers the public paper" '\[radio-play-public\]' "$SO"
# public playback was STARTED: the now-playing line names one of the public stories listed on the radio page
T0=$(awk '/\[radio-public-play-0\]/{f=1; next} f && /Text#[0-9]+ "/{ t=$0; f=0 } /tapped .*"radio-play-public"/{ exit } END{}  { if (t != "") last=t } END{ print last }' "$SO" | grep -o 'Text#[0-9]* "[^"]*"' | head -1 | sed -E 's/^Text#[0-9]+ "//; s/"$//' | cut -c1-24)   # the first public row as the LAST dump before the tap showed it (the paper's front changes live: the first dump of the drive can be minutes old)
if [ -n "$T0" ] && grep -E 'NOW PLAYING|Now playing,' "$SO" | grep -qF "$T0"; then echo "ok: signed out: public playback started (now playing: a public story)"; else echo "MISSING: signed out: public playback started (title '$T0')"; FAILS="$FAILS [public playback]"; fi
# the tree a screenshot was taken of: the lines between the previous screenshot line and this one
seg() { awk -v m="$1" '/^screenshot /{ if (index($0, m)) { printf "%s", buf; exit } buf=""; next } { buf = buf $0 "\n" }' "$SO"; }
seg real-51b-so-paper-end > "$SHOTS/seg-paper-end.txt"; seg real-52b-so-after-sheet > "$SHOTS/seg-after-sheet.txt"
chk "signed out: the end of the paper shows its last line (the count)" 'stories . pull down to refresh' "$SHOTS/seg-paper-end.txt"
chk "signed out: the card is there at the end of the paper" '\[signedout-banner\]' "$SHOTS/seg-paper-end.txt"
# the reader's menu sheet left open must not hide the card on the next page
if [ -n "$STORY_SO" ]; then chk "signed out: after a reader sheet the next page still has the card" '\[signedout-banner\]' "$SHOTS/seg-after-sheet.txt"; fi
# the card is the first child of the bar's own column, before the bar (so it sits whole above the bar at any bar height, the now-playing strip included). The tree dump carries no frames, so this is
# the structure, not a measured bottom edge; the screenshots are where the placement is read.
PLACED=$(awk '/\[bar-stack\]/{st=1;bar=0;next} /\[bar\]/{bar=1} /\[signedout-banner\]/{if(st&&!bar)ok++;else bad++} END{print (ok+0)","(bad+0)}' "$SO")
case "$PLACED" in 0,*) echo "MISSING: signed out: the card is placed above the bar ($PLACED)"; FAILS="$FAILS [card placement]";; *,0) echo "ok: signed out: the card is the bar column's first child on every screen ($PLACED)";; *) echo "MISSING: signed out: a card is not above the bar ($PLACED)"; FAILS="$FAILS [card placement]";; esac
bad "signed out: the paper has no second sign-in line" '\[(paper-signin|paper-signin-away|paper-signin-card)\]' "$SO"
bad "signed out: Files has no write actions" '\[(files-upload|files-new-toggle|files-search)\]' "$SO"
bad "signed out: Mail has no compose" '\[mail-compose\]' "$SO"
chk "signed out: Settings > Account has Sign in" '\[settings-signin\]' "$SO"
chk "signed out: Textures says sign in (shared code)" '\[textures-message\] "Sign in to see your documents' "$SO"
chk "signed out: Files says sign in" 'Sign in to see your files' "$SO"
bad "signed out: no locked-phone sentence" 'Unlock your phone' "$SO"
# the sign-out drill
chk "drill: signed in before" '\[settings-account-signed\][^"]*"Yes"' "$DR"
chk "drill: the account page says signed out after" '\[settings-account-signed\][^"]*"No"' "$DR"
bad "drill: no failure note on the account page" '\[settings-signout-message\]' "$AFTER"
bad "drill: Files shows no rows after sign-out" '\[files-row-/' "$AFTER"
bad "drill: Mail shows no rows after sign-out" '\[mail-row-' "$AFTER"
[ "$LEFT" = 0 ] && echo "ok: drill: the wipe marker and the stamp are gone from the app's files" || { echo "MISSING: drill: marker or stamp left ($LEFT): $(printf '%s' "$LEFT_FILES" | sed "s|$CONT||" | tr '\n' ' ')"; FAILS="$FAILS [drill files]"; }
n=$(ls "$SHOTS"/real-*.png 2>/dev/null | wc -l)
[ "$n" -ge 18 ] || FAILS="$FAILS [screenshots: $n]"
echo "ok: screenshots in $SHOTS ($n)"
# A fatal error in the app's own log is a crash of the app under proof: it fails the proof (ERROR lines stay a note: the app logs handled errors that way).
if grep -q 'Fatal error' "$OUT"; then echo "UNWANTED: a fatal error in the app's log: $(grep -m1 'Fatal error' "$OUT" | sed -E 's/choir_sk_[^[:space:]"'"'"'<>]+/<key>/g' | cut -c1-200)"; FAILS="$FAILS [fatal error in the app log]"; fi
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT" | sed -E 's/choir_sk_[^[:space:]"'"'"'<>]+/<key>/g' | cut -c1-240; fi
if [ -n "$DRIVES_FAILED" ]; then echo "PROOF FAILED:$DRIVES_FAILED${FAILS:+ (checks:$FAILS)}"; cp "$OUT" "$SHOTS/drive.txt" 2>/dev/null; exit 1; fi
[ -z "$FAILS" ] || fail "checks:$FAILS"
[ -s "$SHOTS/retries.txt" ] && { echo "note: $(wc -l < "$SHOTS/retries.txt" | tr -d ' ') drive(s) were run again after the dispatch-log timeout (retry: lines above):"; cat "$SHOTS/retries.txt"; }
echo "PROOF OK"
