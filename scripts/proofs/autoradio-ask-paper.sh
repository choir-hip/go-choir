#!/usr/bin/env bash
# Proof: a listener's ask is answered from a search of the paper (the Autoradio
# desk's turn, POST /api/autoradio/turns; internal/agentcore/api_autoradio_paper.go).
#
# The turn is sent with NO candidates, so the only stories the desk may name in a
# queue op are the ones its own search of the paper found. The proof asks for a
# topic the paper holds (default "Taiwan and Asia") and passes only if the done
# event's `paper` account says the search ran (state found), gave at least one
# story, the vector half answered, nothing was partial, the top story is named,
# every queue op route is one of the stories given, and something was said. A queue op naming a story route is
# printed too (a route could not be there unless the search found it) but the
# model's choice to write one is not what is proved.
#
# Needs CHOIR_HOST (e.g. choir.news) and CHOIR_API_KEY_FILE (a file holding the
# drill account's key; never echoed). An owner-wide key names no computer, so set
# CHOIR_COMPUTER_ID (sent as X-Choir-Computer). CHOIR_ASK_TOPIC overrides the
# topic. Run it against the drill account only, never the paper's or the owner's
# computer. `--dry-run` only checks the inputs.
#
# The same facts are the computer's log line per turn,
#   autoradio: turn=<id> paper found=N given=M search_ms=T named=K vector="..." capped=false
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
: "${CHOIR_API_KEY_FILE:?set CHOIR_API_KEY_FILE}"
[ -r "$CHOIR_API_KEY_FILE" ] || { echo "cannot read CHOIR_API_KEY_FILE" >&2; exit 2; }
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
topic=${CHOIR_ASK_TOPIC:-Taiwan and Asia}
turn="proof-ask-$(date -u +%Y%m%d%H%M%S)-$$"
words="Play me more news about ${topic}, next."
body=$(python3 -I - "$turn" "$words" <<'PY'
import json, sys
print(json.dumps({
    "turn_id": sys.argv[1],
    "words": sys.argv[2],
    "page": {"address": "pub:front@proof", "title": "Front page", "text": "The front page."},
    "queue": {"version": 1, "current": None, "upcoming": []},
    "candidates": [],
}))
PY
)
out=$(mktemp)
trap 'rm -f "$out"' EXIT
computer_header=()
if [ -n "${CHOIR_COMPUTER_ID:-}" ]; then computer_header=(-H "X-Choir-Computer: ${CHOIR_COMPUTER_ID}"); fi
# The key goes through a curl config on stdin, so it is never in argv.
code=$(printf 'header = "Authorization: Bearer %s"\n' "$(tr -d '\n\r ' < "$CHOIR_API_KEY_FILE")" |
  curl -sS -m 60 -K - -o "$out" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' ${computer_header[@]+"${computer_header[@]}"} --data "$body" \
    "https://${CHOIR_HOST}/api/autoradio/turns")
echo "http $code turn=$turn words=\"$words\""
[ "$code" = 200 ] || { cat "$out"; echo; exit 1; }
python3 -I - "$out" <<'PY'
import json, sys
speech, ops, done = [], [], {}
name = None
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.rstrip("\n")
    if line.startswith("event: "):
        name = line[7:]
    elif line.startswith("data: ") and name:
        d = json.loads(line[6:])
        if name == "speech":
            speech.append(d["text"])
        elif name == "queue_ops":
            ops.extend(d.get("ops") or [])
        elif name == "done":
            done = d
print("speech:", " ".join(speech))
routes = [o["item"]["route"] for o in ops if o.get("item") and o["item"].get("route")]
print("queue_ops routes (no candidates were sent, so these came from the search):", routes)
print("status:", done.get("status"), "timings_ms:", done.get("timings_ms"), "usage:", done.get("usage"))
paper = done.get("paper") or {}
print("paper:", paper)
top, given_routes = paper.get("top"), paper.get("routes") or []
print("top story by the ranking:", top, "of", given_routes)
# the top story must be the first of the stories given, every op route must be
# one of them (no candidates were sent), and the speech must be non-empty
ops_ok = all(r in given_routes for r in routes)
ok = (done.get("status") == "completed" and paper.get("state") == "found" and paper.get("given", 0) > 0
      and paper.get("vector") == "answered" and not paper.get("capped") and not paper.get("partial")
      and bool(top) and given_routes[:1] == [top] and ops_ok and bool(speech))
if not ok:
    print("FAIL: want status completed and paper state found, given > 0, vector answered, index not capped", file=sys.stderr)
sys.exit(0 if ok else 1)
PY
