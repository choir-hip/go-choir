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
# run_drive: a first run that ends in the dispatch-log timeout is run again once and logged; a second timeout fails
mkdir -p "$T/shots"; SHOTS="$T/shots"; export SHOTS
printf '#!/bin/sh\nn=$(cat "%s/n" 2>/dev/null || echo 0); echo $((n + 1)) > "%s/n"\nif [ $n -lt "$FAILN" ]; then echo "op 2/9 `tap place-mail`: tap #1: the dispatch log did not answer within 1 s of the touch; the app'"'"'s carrier and the touch runner were stopped"; exit 1; fi\necho tree\n' "$T" "$T" > "$T/exact.sh"; chmod +x "$T/exact.sh"
run() { # <FAILN>
  rm -f "$T/n" "$T/shots/retries.txt"
  (
    HERE="$T"; export FAILN="$1"; EXACT_SIM=x; TMPDIR="$T"
    . "$(dirname "$0")/proof-common.sh" 2>/dev/null
    reset_state() { :; }; SIM() { :; }; sleep() { :; }; fail() { echo "FAILED $1"; exit 1; }
    step "tap place-mail"; run_drive d light ""
  ) 2>&1
}
out=$(run 1); echo "$out" | grep -q '^retry: d drive' && echo "$out" | grep -q 'drive d ok' && [ "$(wc -l < "$T/shots/retries.txt")" -eq 1 ] && ok "one timeout: the drive is run again, logged, and passes" || no "one timeout: $out"
out=$(run 2); echo "$out" | grep -q 'FAILED drive d' && ok "two timeouts: the drive fails" || no "two timeouts: $out"
[ "$fails" -eq 0 ] && echo "all ok" || { echo "$fails failed"; exit 1; }
