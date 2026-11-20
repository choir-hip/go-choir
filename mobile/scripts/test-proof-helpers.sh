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
