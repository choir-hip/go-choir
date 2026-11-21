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
step "clock data"; wait_ 4000
place settings 900; view real-07c-system-dark 900
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
rm -f "$SHOTS/takes-data.json" "$SHOTS/takes-copy-own.json" "$SHOTS/takes-copy-muted.json" "$SHOTS/takes-old-own.json" "$SHOTS/takes-old-muted.json"
[ -n "$TK_FILE" ] && cp "$TK_FILE" "$SHOTS/takes-data.json"
cp "$TKS/takes-copy-own.json" "$SHOTS/takes-copy-own.json" 2>/dev/null
cp "$TKS/takes-copy-muted.json" "$SHOTS/takes-copy-muted.json" 2>/dev/null
cp "$TKS/takes-own.json" "$SHOTS/takes-old-own.json" 2>/dev/null
cp "$TKS/takes-muted.json" "$SHOTS/takes-old-muted.json" 2>/dev/null
tseg() { awk -v m="$2" '/^screenshot /{ if (index($0, m)) { printf "%s", buf; exit } buf=""; next } { buf = buf $0 "\n" }' "$1"; }
tseg "$SHOTS/drive-takes.txt" real-46-takes-old-files > "$SHOTS/seg-takes.txt"

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
[ -n "$PLAY1" ] && { to_top; step "tap \"$PLAY1\""; wait_ 5000; step tree; shot real-16-playing; step "tap bar-now"; view real-17-radio-page 1200; }
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
# and dragged back up before Science is tapped again. Each target is also asserted to be in the dumped tree below, so a row that was off screen fails there, not silently.
step "tap ob-topic-Science"; wait_ 700
step "tap ob-topic-open-Culture"; wait_ 700; step "tap edit-edition-scroll drag 0 -300"; wait_ 500; step tree; step "tap ob-topic-Film"; wait_ 700
step "tap edit-edition-scroll drag 0 -700"; wait_ 500
step "type ob-topic-input Zzdrillchess"; step "tap ob-topic-add"; view real-72-topics-picked 1200
step "tap edit-edition-scroll drag 0 1500"; wait_ 500; step "tap ob-topic-Science"; view real-73-topics-one-off 1200
step "tap edit-edition-scroll drag 0 -1500"; wait_ 500; step tree; step "tap edit-edition-done"; wait_ 2500
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

# ---- 3f3. BAR drive (reads only: it plays and pauses a real story): the bar's page model is shared code (bar.ts) over the native facts. On a page: the aim line names the page and the deck count; one story plays: the
# bar says NOW PLAYING with the story's title and a progress rule; play pauses it (PAUSED, same title); play resumes it; the run sheet page (the radio) lists the entry on air.
fresh_paper pick-bar
PLAY_BAR=$F_PLAY
step "clock data"; wait_ 6000
place paper 2500
step tree
view real-66-bar-on-page 600
[ -n "$PLAY_BAR" ] && { to_top; step "tap \"$PLAY_BAR\""; wait_ 5000; view real-67-bar-playing 800; step "tap bar-play"; wait_ 1500; view real-68-bar-paused 800; step "tap bar-play"; wait_ 3000; view real-69-bar-resumed 800; step "tap bar-now"; view real-70-bar-run-sheet 1500; }
step logs
run_drive bar light "" --touch platform

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
LEFT=$(find "$CONT" \( -name 'signout-pending' -o -name 'account-stamp' \) 2>/dev/null | wc -l | tr -d ' ')

# ---- 3h. SIGNED-OUT TOUR (the drill above left the Keychain empty): launch with no key. The first screen (the radio) says what is true and offers the one way in; the paper lists the public stories and a
# story reads; the radio offers to read the public paper; Settings > Account has a Sign in button. Nothing here needs the owner's computer.
export NOKEY=1
fresh_paper pick-signedout
STORY_SO=$F_STORY1
step "clock data"; wait_ 6000
# the account card is answered in REAL time (the key read, the host fact): give the first screen several settle steps before it is read
wait_ 6000; wait_ 6000; wait_ 6000
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
unset NOKEY
SO="$SHOTS/drive-signedout.txt"
# the FIRST screen's dump only: the text of the drive up to the first screenshot step (real-50), not the whole drive
SO1="$SHOTS/signedout-first.txt"; awk '{ print } /real-50-so-first-screen/ { exit }' "$SO" > "$SO1"

# ---- 4. assertions on real conditions
if grep -lE "choir_sk_[A-Za-z0-9]" "$SHOTS"/*.txt "$OUT" >/dev/null 2>&1; then fail "key leak" "a key-shaped string is in the drive output"; fi
SV="$SHOTS/drive-survey.txt"; RD="$SHOTS/drive-read.txt"
chk() { # name pattern file
  if grep -qE -- "$2" "$3"; then echo "ok: $1"; else echo "MISSING: $1"; FAILS="$FAILS [$1]"; fi
}
bad() { # name pattern file
  if grep -qE -- "$2" "$3"; then echo "UNWANTED: $1: $(grep -m1 -E -- "$2" "$3" | cut -c1-160)"; FAILS="$FAILS [$1]"; else echo "ok: $1"; fi
}
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
chk "radio: playing a real story" 'NOW PLAYING|Now playing,' "$RD"
[ -n "$DOC1" ] && chk "texture document: opened" '\[reader-block-' "$RD"
if [ -n "${REAL_PROMPT:-}" ]; then
  chk "typed ask: the engine took the typed words as the turn" 'top story on the front page' "$SHOTS/drive-ask.txt"
  bad "typed ask: no sign-in failure" 'Sign in to talk to your computer' "$SHOTS/drive-ask.txt"
  chk "conversation: the question is a row (shared code over the native entries)" '\[conversation-row-' "$SHOTS/drive-ask.txt"
fi
if [ -n "$THEME_DOC" ]; then
  TH="$SHOTS/drive-theme.txt"
  chk "theme: the Texture's theme is listed" '\[settings-theme-depth-teal-test\]' "$TH"
  bad "theme: no problems reported" '\[settings-theme-problems\]' "$TH"
fi
# the takes drill (nothing is carried from the pre-release build; Record and Publish are not driven, see 3a4)
chk "takes: the Takes section drew" '\[radio-takes\]' "$SHOTS/seg-takes.txt"
chk "takes: the record control is there" '\[takes-record\]' "$SHOTS/seg-takes.txt"
bad "takes: no old muted voice is listed" 'Never play takes by voice-drillm' "$SHOTS/seg-takes.txt"
if [ -f "$SHOTS/takes-data.json" ]; then bad "takes: nothing of the old files is in the kept file" 'take-drill|voice-drill' "$SHOTS/takes-data.json"; else echo "info: takes: no kept file (d@e has no live take), nothing to check in it"; fi
chk "takes: the programme's copy was pushed by shared code (the file exists)" '"ids"' "$SHOTS/takes-copy-own.json"
bad "takes: nothing of the old files is in the programme's copy" 'take-drill|voice-drill' "$SHOTS/takes-copy-own.json"
chk "takes: the programme's muted copy is empty" '^\[\]$' "$SHOTS/takes-copy-muted.json"
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
  TODAY=$(date '+%a %-d %b')   # the simulator shares the Mac's clock and zone: a date from 1970 (no clock) or the wrong day fails
  chk "radio page: the date line is today ($TODAY)" "\\[radio-date\\] \"$TODAY\"" "$BR"
  bad "radio page: the on-air title is not the kind word (a title the play dropped)" '\[radio-onair-title\] "(Story|Opening|Page|Podcast|Take|Book|Mail|Result|Answer|Transition|Audio|Clip)"' "$BR"
  bad "radio page: Built on does not show a raw route" 'radio-builton-0\] label="Open /?pub/' "$BR"
  chk "radio page: the status line says On air or Paused" '\[radio-status\] "(On air|Paused)"' "$BR"
  bad "radio page: the status line is not an error" '\[radio-status\] "Can.t speak' "$BR"
  bad "bar: no error line held the bar" '\[bar-text\] "(Could not|Something went wrong|The voice stopped)' "$BR"
fi
# the spine drive
SP="$SHOTS/drive-spine.txt"
chk "bar: the field, the microphone and play" '\[bar-field\]' "$SP"
chk "bar: voice button" '\[bar-mic\]' "$SP"
chk "bar: play is there" '\[bar-play\]' "$SP"
chk "launcher: the mark opens the layer" '\[launcher-layer\]' "$SP"
chk "launcher: the paper is in the deck" '\[launcher-open-paper:front\]' "$SP"
chk "launcher: places" '\[place-settings\]' "$SP"
chk "themes: a fresh install shows the default pair, light (Chrome Pastel)" 'Showing Chrome Pastel in light' "$SP"
chk "themes: the default pair, dark (Liquid Metal)" 'Showing Liquid Metal in dark' "$SP"
chk "themes: Newsprint light" 'Showing Newsprint in light' "$SP"
chk "themes: Newsprint dark" 'Showing Newsprint in dark' "$SP"
[ -n "$FILE1" ] && chk "files: a file opened" '\[(page-title|viewer-[a-z]+|csv-card|image-view)\]' "$SHOTS/drive-tour.txt"
chk "podcasts: the place opened" '\[(podcasts-[a-z]+|podcast-row-[^]]*)\]' "$SHOTS/drive-tour.txt"
chk "notebook: the page opened" '\[notebook-(page|title)\]' "$SHOTS/drive-tour.txt"
chk "settings account again: signed in" '\[settings-account-signed\][^"]*"Yes"' "$SHOTS/drive-tour.txt"
# the first shared answer after a launch must not run over the device's 100 ms turn (the app logs the failure that says so)
bad "no resource over the 100 ms turn" 'over the 100 ms' "$OUT"
# the shared twins are the Android path: on iOS the shared reader must never fetch (it logs "reader: story blocks=" when it does)
bad "iOS: the shared reader twin never fetched" 'console: reader: story blocks=' "$OUT"
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
T0=$(awk '/\[radio-public-play-0\]/{f=1} f && /Text#[0-9]+ "/{print; exit}' "$SO" | grep -o 'Text#[0-9]* "[^"]*"' | head -1 | sed -E 's/^Text#[0-9]+ "//; s/"$//' | cut -c1-24)
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
[ "$LEFT" = 0 ] && echo "ok: drill: the wipe marker and the stamp are gone from the app's files" || { echo "MISSING: drill: marker or stamp left ($LEFT)"; FAILS="$FAILS [drill files]"; }
n=$(ls "$SHOTS"/real-*.png 2>/dev/null | wc -l)
[ "$n" -ge 18 ] || FAILS="$FAILS [screenshots: $n]"
echo "ok: screenshots in $SHOTS ($n)"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT" | sed -E 's/choir_sk_[^[:space:]"'"'"'<>]+/<key>/g' | cut -c1-240; fi
[ -z "$FAILS" ] || fail "checks:$FAILS"
[ -s "$SHOTS/retries.txt" ] && { echo "note: $(wc -l < "$SHOTS/retries.txt" | tr -d ' ') drive(s) were run again after the dispatch-log timeout (retry: lines above):"; cat "$SHOTS/retries.txt"; }
echo "PROOF OK"
