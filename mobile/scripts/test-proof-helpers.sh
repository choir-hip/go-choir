#!/bin/sh
# Pure-shell test of the proof's pick helpers (proof-real.sh: ids, pick_stories) and of run_drive's one retry on a dispatch-log timeout (proof-common.sh), over a canned tree and stub programs. No simulator, no network.
#   sh mobile/scripts/test-proof-helpers.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
T="${TMPDIR:-/tmp}/test-proof-helpers.$$"; mkdir -p "$T/bin"; trap 'rm -rf "$T"' EXIT
fails=0; ok() { echo "ok: $1"; }; no() { echo "FAIL: $1"; fails=$((fails + 1)); }
# the helpers, cut out of the scripts
sed -n '/^ids() {/p' "$HERE/proof-real.sh" > "$T/h.sh"
sed -n '/^pick_stories() {/,/^}/p' "$HERE/proof-real.sh" >> "$T/h.sh"
. "$T/h.sh"
# a tree: two cards at depth 6, one nested at depth 10; the first card does not resolve
cat > "$T/tree.txt" <<'T'
      View#1 [paper-story-/pub/texture/gone]
      View#2 [paper-story-/pub/texture/one]
          View#3 [paper-story-/pub/texture/nested]
      View#4 [paper-story-/pub/texture/two]
      View#5 [paper-play-/pub/texture/one]
T
printf '#!/bin/sh\ncase "$*" in *texture/gone*) printf 404;; *) printf 200;; esac\n' > "$T/bin/curl"; chmod +x "$T/bin/curl"
got=$(PATH="$T/bin:$PATH" pick_stories "$T/tree.txt" 2 | tr '\n' ' ')
[ "$got" = "paper-story-/pub/texture/one paper-story-/pub/texture/two " ] && ok "pick_stories skips a 404 and a nested card, takes two" || no "pick_stories gave '$got'"
[ "$(ids "$T/tree.txt" paper-play- | sed -n 1p)" = "paper-play-/pub/texture/one" ] && ok "ids finds the play button" || no "ids"
[ -z "$(PATH="$T/bin:$PATH" pick_stories /dev/null 2)" ] && ok "an empty tree picks nothing" || no "empty tree"
# layout_scroll: the wheel distance that brings a node's middle to the wanted line, read from a `layout <id>` block
sed -n '/^layout_scroll() {/,/^}/p' "$HERE/proof-common.sh" > "$T/l.sh"; . "$T/l.sh"
cat > "$T/layout.txt" <<'L'
viewport 402×874 · clock 6000 ms
#12 [reader-scroll] ScrollView 0,0 402×874 scroll 0,0
node #761 [reader-cite-ref-a1] Text · site 5 · epoch 3 · incarnation 1
  space viewport 20,1204 20×22 · frame 20,300 20×22 (kernel, in the parent or containing block)
node #770 [reader-cite-ref-a2] Text · site 5 · epoch 3 · incarnation 1
  space viewport 20,200 20×22 · frame 20,200 20×22
node #780 [reader-cite-ref-a3] Text · site 5 · epoch 3 · incarnation 1
  frame 20,200 20×22
L
[ "$(layout_scroll "$T/layout.txt" reader-cite-ref-a1)" = "895" ] && ok "layout_scroll: a node below the fold: middle 1215 to the line 320 is 895" || no "layout_scroll below the fold: $(layout_scroll "$T/layout.txt" reader-cite-ref-a1)"
[ "$(layout_scroll "$T/layout.txt" reader-cite-ref-a1 500)" = "715" ] && ok "layout_scroll: another wanted line" || no "layout_scroll wanted line"
[ "$(layout_scroll "$T/layout.txt" reader-cite-ref-a2)" = "0" ] && ok "layout_scroll: a node already in view needs no scroll" || no "layout_scroll in view"
if out=$(layout_scroll "$T/layout.txt" reader-cite-ref-zz); then no "layout_scroll: a missing node printed '$out'"; else ok "layout_scroll: a missing node fails and prints nothing"; fi
if out=$(layout_scroll "$T/layout.txt" reader-cite-ref-a3); then no "layout_scroll: a node with no viewport space printed '$out'"; else ok "layout_scroll: a node with no viewport space fails"; fi
# blocks_have_text: sentence children, the old one-line shape, and an empty block
sed -n '/^blocks_have_text() {/,/^}/p' "$HERE/proof-common.sh" > "$T/b.sh"; . "$T/b.sh"
cat > "$T/blocks-ok.txt" <<'B'
              View#857
                Text#858 [reader-block-b2] (contextmenu)
                  Text#859 "Ukraine carried out a three-night campaign against the Kerch Bridge."
                  Text#860 [reader-cite-ref-1] " 1 " label="Source 1" (press)
                Text#872 [reader-block-b3] (contextmenu)
                  Text#873 "short"
                  Text#874 "The strikes ran from the night of 16 November."
B
bhas() { blocks_have_text "$@" > "$T/b.out" 2>&1; }
bhas "$T/blocks-ok.txt" 2 && ok "blocks_have_text: sentence children carry the text" || no "blocks_have_text sentence children: $(cat "$T/b.out")"
cat > "$T/blocks-old.txt" <<'B'
        Text#10 [reader-block-b1] "A paragraph of the old one-line shape, long enough."
        Text#11 [reader-block-b2] "Another paragraph of the old one-line shape, as well."
B
bhas "$T/blocks-old.txt" 2 && ok "blocks_have_text: the old one-line shape passes" || no "blocks_have_text old shape"
cat > "$T/blocks-empty.txt" <<'B'
              Text#858 [reader-block-b2] (contextmenu)
                Text#859 "Ukraine carried out a three-night campaign against the Kerch Bridge."
              Text#872 [reader-block-b3] (contextmenu)
              Text#900 "A paragraph at the block's own depth does not count for it, however long it is."
B
if bhas "$T/blocks-empty.txt" 2; then no "blocks_have_text: an empty block passed"; else grep -q "no text in reader-block-b3" "$T/b.out" && ok "blocks_have_text: a block with no child text fails and is named" || no "blocks_have_text empty: $(cat "$T/b.out")"; fi
if bhas "$T/blocks-ok.txt" 3; then no "blocks_have_text: fewer blocks than asked passed"; else ok "blocks_have_text: fewer blocks than asked fails"; fi
# bad and nomust: a negative check over a missing or empty file fails (it used to pass: a drive that wrote nothing passed every one)
sed -n '/^FAILS=""/,/^}/p;/^nomust() {/p;/^bad() {/,/^}/p;/^chk() {/,/^}/p' "$HERE/proof-common.sh" > "$T/n.sh"
FAILS=""; OUT="$T/out.txt"; . "$T/n.sh"
printf 'a line\n' > "$T/has.txt"; : > "$T/empty.txt"
FAILS=""; bad "t" 'nothing like this' "$T/has.txt" >/dev/null; [ -z "$FAILS" ] && ok "bad: a file without the pattern passes" || no "bad: a clean file failed"
FAILS=""; bad "t" 'a line' "$T/has.txt" >/dev/null; [ -n "$FAILS" ] && ok "bad: a file with the pattern fails" || no "bad: the pattern was not found"
FAILS=""; bad "t" 'x' "$T/empty.txt" >/dev/null; [ -n "$FAILS" ] && ok "bad: an empty file fails" || no "bad: an empty file passed"
FAILS=""; bad "t" 'x' "$T/absent.txt" >/dev/null; [ -n "$FAILS" ] && ok "bad: a missing file fails" || no "bad: a missing file passed"
: > "$OUT"; FAILS=""; nomust "t" "x" >/dev/null; [ -n "$FAILS" ] && ok "nomust: an empty drive output fails" || no "nomust: an empty output passed"
rm -f "$OUT"; FAILS=""; nomust "t" "x" >/dev/null; [ -n "$FAILS" ] && ok "nomust: a missing drive output fails" || no "nomust: a missing output passed"
echo "a line" > "$OUT"; FAILS=""; nomust "t" "other" >/dev/null; [ -z "$FAILS" ] && ok "nomust: output without the text passes" || no "nomust: clean output failed"
FAILS=""; nomust "t" "a line" >/dev/null; [ -n "$FAILS" ] && ok "nomust: output with the text fails" || no "nomust: text not found"
# run_drive: a first run that ends in the dispatch-log timeout is run again once and logged; a second timeout fails
mkdir -p "$T/shots"; SHOTS="$T/shots"; export SHOTS
printf '#!/bin/sh\necho test.app.id\n' > "$T/app-identity.sh"; chmod +x "$T/app-identity.sh"      # proof-common.sh asks it for the app id (this stub directory is its HERE)
printf '#!/bin/sh\nn=$(cat "%s/n" 2>/dev/null || echo 0); echo $((n + 1)) > "%s/n"\nif [ $n -lt "$FAILN" ]; then echo "op 2/9 `tap place-mail`: tap #1: the dispatch log did not answer within 1 s of the touch; the app'"'"'s carrier and the touch runner were stopped"; exit 1; fi\necho tree\n' "$T" "$T" > "$T/exact.sh"; chmod +x "$T/exact.sh"
run() { # <FAILN>
  rm -f "$T/n" "$T/shots/retries.txt"
  (
    HERE="$T"; export FAILN="$1"; EXACT_SIM=x; TMPDIR="$T"
    . "$(dirname "$0")/proof-common.sh"
    reset_state() { :; }; SIM() { :; }; sleep() { :; }; fail() { echo "FAILED $1"; exit 1; }
    step "tap place-mail"; run_drive d light ""
  ) 2>&1
}
out=$(run 1); echo "$out" | grep -q '^retry: d drive' && echo "$out" | grep -q 'drive d ok' && [ "$(wc -l < "$T/shots/retries.txt")" -eq 1 ] && ok "one timeout: the drive is run again, logged, and passes" || no "one timeout: $out"
[ "$(wc -l < "$T/shots/drive-d.date" | tr -d ' ')" = 2 ] && [ "$(sed -n 1p "$T/shots/drive-d.date")" = "$(date '+%a %-d %b')" ] && ok "run_drive stamps the day the drive started and ended on (the date line is checked against it)" || no "run_drive wrote no date stamp"
out=$(run 2); echo "$out" | grep -q 'FAILED drive d' && ok "two timeouts: the drive fails" || no "two timeouts: $out"
# PROOF_KEEP_GOING=1: a failed drive is recorded and the next one runs; without it the proof stops at the first
out=$(PROOF_KEEP_GOING=1 run 2); echo "$out" | grep -q 'drive d FAILED' && ! echo "$out" | grep -q '^FAILED' && ok "keep going: a failed drive is named and the proof goes on" || no "keep going: $out"
# a settle that gave up at the start of a drive (`clock settle: TimerFireLimit`, first ops: requests still in flight from a slow launch; train2p1 drill, op 4/79) is run again once, logged; one later in the drive, or a second one, fails
printf '#!/bin/sh\nn=$(cat "%s/n" 2>/dev/null || echo 0); echo $((n + 1)) > "%s/n"\nif [ $n -lt "$FAILN" ]; then echo "op ${FAILOP:-4}/79 '"'"'clock settle'"'"': clock: clock: TimerFireLimit { limit: 4096 }"; exit 1; fi\necho tree\n' "$T" "$T" > "$T/exact.sh"; chmod +x "$T/exact.sh"
out=$(run 1); echo "$out" | grep -q '^retry: d drive, op 4/79' && echo "$out" | grep -q 'drive d ok' && ok "TimerFireLimit at op 4: the drive is run again once and passes" || no "TimerFireLimit at op 4: $out"
out=$(run 2); echo "$out" | grep -q 'FAILED drive d' && ok "TimerFireLimit twice: the drive fails" || no "TimerFireLimit twice: $out"
out=$(FAILOP=60 run 1); echo "$out" | grep -q 'FAILED drive d' && ! echo "$out" | grep -q '^retry:' && ok "TimerFireLimit at op 60: no retry (state may have changed)" || no "TimerFireLimit at op 60: $out"
# ---- the proof's sign-in (proof-key.sh): a disposable key minted with the QA key, seeded into Exact's scratch secret store; fakes for curl and simctl, no network, no simulator
cat > "$T/bin/curl" <<'C'
#!/bin/sh
# fake curl: logs its argv and what its -H @file holds; exits 99 when any argument holds a key (a key on a command line)
echo "ARGV $*" >> "$FAKE_LOG"
case "$*" in *choir_sk_*) echo "a key on the command line" >> "$FAKE_LOG.leak"; exit 99;; esac
out=""; hdr=""; method=GET; url=""
while [ $# -gt 0 ]; do
  case "$1" in -o) out=$2; shift;; -H) case "$2" in @*) hdr=${2#@};; esac; shift;; -X) method=$2; shift;; -w|--max-time|--data-binary|-d) shift;; -*) ;; *) url=$1;; esac
  shift
done
[ -n "$hdr" ] && echo "HDR mode=$(stat -f %Lp "$hdr") qa=$(grep -c 'Bearer choir_sk_qa_FAKEQA$' "$hdr")" >> "$FAKE_LOG"
case "$method $url" in
  "POST "*/auth/api-keys)
    code=${FAKE_MINT_CODE:-201}
    if [ "$code" = 201 ]; then
      if [ -n "${FAKE_MINT_BODY:-}" ]; then printf '%s' "$FAKE_MINT_BODY" > "$out"
      else printf '%s' '{"id":"key-123","label":"x","secret":"choir_sk_minted_FAKEMINT"}' > "$out"; fi
    else printf '%s' '{"error":"api key requires manage:keys or admin to create keys"}' > "$out"; fi
    printf '%s' "$code";;
  "DELETE "*/auth/api-keys/*) echo "REVOKE ${url##*/}" >> "$FAKE_LOG"; [ -n "$out" ] && : > "$out"; printf '%s' "${FAKE_REVOKE_CODE:-204}";;
  *) printf 000;;
esac
C
chmod +x "$T/bin/curl"
printf 'choir_sk_qa_FAKEQA\n' > "$T/qa.key"
PK_MINTED=choir_sk_minted_FAKEMINT
sk() { # <snippet> : run it with the sign-in helpers, the fake curl first on PATH and a fake container
  ( PATH="$T/bin:$PATH"; SHOTS="$T/shots"; SCRATCH="$T/sc"; APP=test.app.id; KEYFILE="$T/qa.key"; PROOF_BASE_URL=http://fake.invalid
    EXACT_SIM=x; CONT="$T/cont"; FAKE_LOG="$T/curl.log"; export FAKE_LOG
    mkdir -p "$SCRATCH" "$CONT"
    SIM() { [ "$1" = get_app_container ] && echo "$CONT"; return 0; }
    fail() { echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }
    . "$HERE/proof-key.sh" && eval "$1" ) 2>&1
}
fresh() { rm -rf "$T/cont" "$T/sc" "$T/curl.log" "$T/curl.log.leak"; mkdir -p "$T/cont"; }
SECDIR="$T/cont/Library/Caches/exact/test.app.id/agent/default/secrets"; DATADIR="$T/cont/Library/Caches/exact/test.app.id/agent/default/data"
fresh; mkdir -p "$DATADIR" "$SECDIR"; : > "$DATADIR/account-stamp"; : > "$DATADIR/signout-pending"; : > "$DATADIR/pending-revoke"; : > "$SECDIR/choir.computer-id"
out=$(sk 'proof_mint && proof_seed "$CONT"')
[ "$(ls "$SECDIR" | tr '\n' ' ')" = "choir.key choir.key-id " ] && ok "seed: the secrets directory holds choir.key and choir.key-id and nothing else (an old computer id is gone)" || no "seed: secrets are '$(ls "$SECDIR" 2>&1 | tr '\n' ' ')': $out"
[ "$(cat "$SECDIR/choir.key")" = "$PK_MINTED" ] && [ "$(wc -c < "$SECDIR/choir.key" | tr -d ' ')" = "${#PK_MINTED}" ] && ok "seed: choir.key is the minted key, bytes exact, no newline" || no "seed: choir.key content"
[ "$(cat "$SECDIR/choir.key-id")" = "key-123" ] && [ "$(wc -c < "$SECDIR/choir.key-id" | tr -d ' ')" = 7 ] && ok "seed: choir.key-id is the id the mint answered" || no "seed: choir.key-id"
[ "$(stat -f %Lp "$SECDIR/choir.key")" = 600 ] && [ "$(stat -f %Lp "$SECDIR/choir.key-id")" = 600 ] && [ "$(stat -f %Lp "$SECDIR")" = 700 ] && ok "seed: files are 0600 and the directory 0700" || no "seed: modes"
[ ! -e "$DATADIR/account-stamp" ] && [ ! -e "$DATADIR/signout-pending" ] && [ ! -e "$DATADIR/pending-revoke" ] && ok "seed: a stale stamp, marker and pending-revoke list are removed" || no "seed: stale data files stay"
grep -q 'choir_sk_qa_FAKEQA' "$SECDIR/choir.key" && no "seed: the QA key itself was seeded" || ok "seed: the seeded key is not the QA key"
grep -q '^HDR mode=600 qa=1$' "$T/curl.log" && [ ! -e "$T/curl.log.leak" ] && ok "mint: the QA key goes in a 0600 header file (-H @file), never in argv" || no "mint: header file or argv: $(cat "$T/curl.log" "$T/curl.log.leak" 2>&1)"
grep -q 'ARGV.*-X POST.*/auth/api-keys' "$T/curl.log" && ok "mint: one POST /auth/api-keys" || no "mint: no POST"
# the mint runs once however many drives seed
fresh; out=$(sk 'proof_mint; proof_seed "$CONT"; proof_seed "$CONT"; proof_seed "$CONT"'); [ "$(grep -c 'X POST' "$T/curl.log")" = 1 ] && ok "mint: three seeds, one mint" || no "mint count"
# refused: stops with the message and seeds nothing
fresh; out=$(FAKE_MINT_CODE=403 sk 'proof_mint; echo NOT-STOPPED'); echo "$out" | grep -qxF 'PROOF FAILED at sign-in: the test key may not mint keys (403)' && ! echo "$out" | grep -q NOT-STOPPED && [ ! -d "$T/cont/Library" ] && ok "mint refused: stops at once with the message and seeds nothing" || no "mint refused: $out"
echo "$out" | grep -q 'choir_sk_\|manage:keys' && no "mint refused: the reply was echoed" || ok "mint refused: the server's reply is not echoed"
# a reply with no id stops before any file is written
fresh; out=$(FAKE_MINT_BODY='{"secret":"choir_sk_minted_FAKEMINT"}' sk 'proof_mint; proof_seed "$CONT"; echo NOT-STOPPED'); echo "$out" | grep -q '^PROOF FAILED at sign-in' && ! echo "$out" | grep -q 'NOT-STOPPED' && [ ! -d "$T/cont/Library" ] && ok "mint: a reply with no id stops before any file is written" || no "mint no id: $out"
# the key never reaches output, a trace or argv
fresh; out=$(sk 'set -x; proof_mint; proof_seed "$CONT"; proof_key_cleanup; set +x')
echo "$out" | grep -q 'choir_sk_minted_FAKEMINT\|choir_sk_qa_FAKEQA' && no "leak: a key is in the output or the trace" || ok "leak: neither key is in stdout, stderr or an sh -x trace"
[ ! -e "$T/curl.log.leak" ] && ! grep -q 'choir_sk_' "$T/curl.log" && ok "leak: no curl argument held a key" || no "leak: argv"
# the exit trap revokes by id, with the QA key from a file; the seeded files go
fresh; out=$(sk 'trap proof_key_cleanup EXIT; proof_mint; proof_seed "$CONT"; exit 0')
grep -q '^REVOKE key-123$' "$T/curl.log" && ok "trap: the exit trap revokes the minted key by id" || no "trap: no revoke: $out"
[ ! -e "$SECDIR/choir.key" ] && [ ! -e "$SECDIR/choir.key-id" ] && ok "trap: the seeded files are removed at exit" || no "trap: files left"
fresh; out=$(sk 'proof_mint; proof_seed "$CONT"; proof_key_cleanup; proof_key_cleanup'); [ "$(grep -c '^REVOKE' "$T/curl.log")" = 1 ] && ok "trap: cleanup run twice revokes once" || no "trap: revoked twice"
# a killed proof still cleans up: the proof's own first two trap lines (EXIT, then the signals), a TERM while a step sleeps
sed -n '/^trap .* EXIT$/{p;q;}' "$HERE/proof-real.sh" > "$T/traps.sh"; sed -n "/^trap 'exit 143' TERM INT HUP/p" "$HERE/proof-real.sh" >> "$T/traps.sh"
fresh; ( sk '. "$T/traps.sh"; proof_mint; proof_seed "$CONT"; echo $(exec sh -c "echo \$PPID") > "$T/pid"; sleep 20 & wait' > "$T/term.out" ) & tw=$!
i=0; while [ ! -s "$T/pid" ] && [ $i -lt 50 ]; do sleep 0.1; i=$((i + 1)); done
[ -s "$T/pid" ] && kill -TERM "$(cat "$T/pid")" 2>/dev/null; wait $tw 2>/dev/null
grep -q '^REVOKE key-123$' "$T/curl.log" 2>/dev/null && [ ! -e "$T/sc" ] && ok "signal: a TERM runs the exit trap (the key is revoked, the scratch folder with the QA header is gone)" || no "signal: a killed proof left the key live: $(cat "$T/term.out" 2>/dev/null)"
fresh; out=$(sk 'proof_key_cleanup; echo "status=$?"'); echo "$out" | grep -q 'status=0' && ! grep -q REVOKE "$T/curl.log" 2>/dev/null && ok "trap: cleanup with no key minted does nothing" || no "cleanup with no key: $out"
# a revoke that failed is said loudly with the id, not the key; a revoke that finds the key gone (404) is fine
fresh; out=$(FAKE_REVOKE_CODE=500 sk 'proof_mint; proof_seed "$CONT"; proof_key_cleanup; echo "status=$?"')
echo "$out" | grep -q 'key-123' && echo "$out" | grep -qi 'revoke' && ! echo "$out" | grep -q 'choir_sk_' && echo "$out" | grep -q 'status=1' && ok "revoke failed: said with the key id only, and the status is not zero" || no "revoke failed: $out"
fresh; out=$(FAKE_REVOKE_CODE=404 sk 'proof_mint; proof_seed "$CONT"; proof_key_cleanup; echo "status=$?"'); echo "$out" | grep -q 'status=0' && ok "revoke: 404 (the sign-out drill revoked it already) is fine" || no "revoke 404: $out"
# the signed-out tour clears the seed
fresh; out=$(sk 'proof_mint; proof_seed "$CONT"; proof_unseed "$CONT"'); [ -z "$(ls "$SECDIR")" ] && [ ! -e "$DATADIR/account-stamp" ] && ok "unseed: the signed-out launch has no secrets and no stamp" || no "unseed: $(ls "$SECDIR")"
# the signed-in and signed-out assertions
printf '%s\n' 'View#1 [settings-account-signed] "No"' 'console: api: session found=false key=false computer=false' 'You are signed out' > "$T/so.txt"
printf '%s\n' 'View#1 [settings-account-signed] "Yes"' 'console: api: session found=true key=true computer=true' > "$T/si.txt"
printf '%s\n' 'View#1 [settings-account-signed] "Yes"' > "$T/si-noconsole.txt"
fresh; out=$(sk "proof_assert_signed_in '$T/so.txt'; echo NOT-STOPPED"); echo "$out" | grep -qxF 'PROOF FAILED at sign-in: the app is signed out' && ! echo "$out" | grep -q NOT-STOPPED && ok "signed-in assertion: fails on a signed-out tree with the message" || no "signed-in on signed-out: $out"
out=$(sk "proof_assert_signed_in '$T/si.txt'; echo PASSED"); echo "$out" | grep -q PASSED && ok "signed-in assertion: passes on a signed-in tree" || no "signed-in on signed-in: $out"
out=$(sk "proof_assert_signed_in '$T/si-noconsole.txt'; echo PASSED"); echo "$out" | grep -q 'PROOF FAILED at sign-in' && ok "signed-in assertion: the console line is needed too" || no "signed-in without the console line: $out"
out=$(sk "proof_assert_signed_in '$T/absent.txt'; echo PASSED"); echo "$out" | grep -q 'PROOF FAILED at sign-in' && ok "signed-in assertion: a missing dump fails" || no "signed-in on a missing file"
fresh; out=$(sk "proof_assert_signed_out '$T/so.txt'; echo PASSED"); echo "$out" | grep -q PASSED && ok "signed-out assertion: passes on a signed-out tree with no secrets" || no "signed-out on signed-out: $out"
out=$(sk "proof_assert_signed_out '$T/si.txt'; echo PASSED"); echo "$out" | grep -q 'PROOF FAILED at sign-in' && ok "signed-out assertion: fails on a signed-in tree" || no "signed-out on signed-in: $out"
fresh; mkdir -p "$SECDIR"; printf x > "$SECDIR/choir.key"; out=$(sk "proof_assert_signed_out '$T/so.txt'; echo PASSED"); echo "$out" | grep -q 'PROOF FAILED at sign-in' && ok "signed-out assertion: a choir.key left on disk fails" || no "signed-out with a key left: $out"
# the proof scripts keep the drives from emptying the store, and no longer copy the old file
if grep -v '^ *#' "$HERE"/proof-real.sh "$HERE"/proof-common.sh "$HERE"/proof-key.sh | grep -qE -- '--reset-storage|--compare'; then no "a drive empties the store"; else ok "no drive passes --reset-storage or --compare (it would empty the seeded secrets)"; fi
if grep -v '^ *#' "$HERE"/proof-real.sh | grep -q 'Documents/proof-key'; then no "the proof-key copy is still there"; else ok "the proof no longer copies a key into Documents"; fi
# sim-device.sh: the newest runtime that supports the device type, from simctl's JSON
cat > "$T/bin/simctl-fake" <<'C'
#!/bin/sh
case "$*" in
  "list -j runtimes") cat "$FAKE_RUNTIMES";;
  "list devices") ;;
  create*) echo "$*" > "$FAKE_CREATED"; echo AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA;;
  boot*) ;;
esac
C
chmod +x "$T/bin/simctl-fake"
python3 -I - "$T" <<'PY'
import json, sys
t = sys.argv[1]
dev = "com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro"
duo = "com.apple.CoreSimulator.SimDeviceType.iPhone-Duo"
def rt(v, types): return {"isAvailable": True, "version": v, "identifier": "com.apple.CoreSimulator.SimRuntime.iOS-" + v.replace(".", "-"), "platform": "iOS", "supportedDeviceTypes": [{"identifier": i} for i in types]}
json.dump({"runtimes": [rt("27.0", [dev, duo]), rt("27.1", [duo])]}, open(t + "/rt-both.json", "w"))
json.dump({"runtimes": [rt("27.1", [duo])]}, open(t + "/rt-none.json", "w"))
json.dump({"runtimes": [rt("9.9", [dev]), rt("27.0", [dev]), rt("27.10", [dev]), {**rt("30.0", [dev]), "isAvailable": False}]}, open(t + "/rt-numeric.json", "w"))
PY
simdev() { AR_SIMCTL="$T/bin/simctl-fake" AR_SIM_NAME=autoradio-lab FAKE_RUNTIMES="$1" FAKE_CREATED="$T/created.txt" sh "$HERE/sim-device.sh" 2>&1; }
rm -f "$T/created.txt"; out=$(simdev "$T/rt-both.json"); grep -q 'SimRuntime.iOS-27-0' "$T/created.txt" 2>/dev/null && ok "sim-device: picks iOS 27.0 when 27.1 cannot host the iPhone 18 Pro" || no "sim-device pick: $out $(cat "$T/created.txt" 2>&1)"
rm -f "$T/created.txt"; out=$(simdev "$T/rt-numeric.json"); grep -q 'SimRuntime.iOS-27-10' "$T/created.txt" 2>/dev/null && ok "sim-device: versions compare as numbers, an unavailable runtime is skipped" || no "sim-device numeric: $out $(cat "$T/created.txt" 2>&1)"
rm -f "$T/created.txt"; if out=$(simdev "$T/rt-none.json"); then no "sim-device: no runtime fits, yet it succeeded"; else echo "$out" | grep -q 'iPhone-18-Pro' && [ ! -e "$T/created.txt" ] && ok "sim-device: no runtime supports the type: fails naming it, creates nothing" || no "sim-device none: $out"; fi
[ "$fails" -eq 0 ] && echo "all ok" || { echo "$fails failed"; exit 1; }
