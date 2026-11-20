#!/usr/bin/env bash
# Proof for station E2 of docs/design/entity-layer-2026-11-20.md, run by the
# lead after the deploy reaches staging (CHOIR_HOST, e.g. choir.news).
#
# Part A (public, no key): the entity read serves a paper entity with its
# stories. It takes the first central entity of the newest front stories.
# Part B (needs d@e's own guest or host access; run on the host that can
# reach corpusd): the neighbours route answers d@e's captures with
# candidates and `waiting`. Set CORPUSD_URL, OWNER_ID and COMPUTER_ID, and
# run on Node B as an internal caller (the route is not public).
#
# `--dry-run` only checks the inputs. Read-only; writes nothing.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
front=$(mktemp)
trap 'rm -f "$front"' EXIT
curl -sS -m 20 -o "$front" "https://${CHOIR_HOST}/api/platform/publications/front?limit=60"
eid=$(python3 -I - "$front" <<'PY'
import json, sys
f = json.load(open(sys.argv[1]))
for s in f.get("stories") or f.get("items") or []:
    for e in s.get("entities") or []:
        print(e["id"]); sys.exit(0)
sys.exit(3)
PY
)
echo "entity: $eid"
page=$(curl -sS -m 20 -w '\n%{http_code}' "https://${CHOIR_HOST}/api/platform/entities/${eid}?stories=5")
code=${page##*$'\n'}
body=${page%$'\n'*}
echo "GET /api/platform/entities/${eid}?stories=5 -> $code"
# A person without a Wikidata id and fewer than two central stories answers
# 404 by design; any other entity must be 200 with at least one story.
python3 -I - "$code" "$body" <<'PY'
import json, sys
code, body = sys.argv[1], sys.argv[2]
if code == "404":
    print("404: a person without a page by the owner-question-1 default (try another entity)"); sys.exit(0)
p = json.loads(body)
print(f"name={p['name']!r} type={p['type']} mentions={p['mentions']} central={p['central_mentions']} listed={len(p['story_list'])} co={[c['name'] for c in p['co_entities']]}")
sys.exit(0 if p["story_list"] and p["story_list"][0].get("role") else 1)
PY
# Stranger bounds: more than 50 stories, and a malformed id.
test "$(curl -s -o /dev/null -w '%{http_code}' "https://${CHOIR_HOST}/api/platform/entities/${eid}?stories=51")" = 400
test "$(curl -s -o /dev/null -w '%{http_code}' "https://${CHOIR_HOST}/api/platform/entities/ent:nothex")" = 404
echo "part A ok"

if [ -n "${CORPUSD_URL:-}" ] && [ -n "${OWNER_ID:-}" ] && [ -n "${COMPUTER_ID:-}" ]; then
  # Part B. From Node B, as an internal caller (hostauth credential as the
  # host's other internal calls carry it; see docs/reference for the header).
  ready=$(mktemp)
  trap 'rm -f "$front" "$ready"' EXIT
  curl -sS -m 40 -X POST "${CORPUSD_URL}/internal/platform/captures/neighbours" \
    -H 'Content-Type: application/json' -H 'X-Internal-Caller: true' \
    ${CORPUSD_INTERNAL_AUTH_HEADER:+-H "$CORPUSD_INTERNAL_AUTH_HEADER"} \
    -d "{\"owner_id\":\"${OWNER_ID}\",\"computer_id\":\"${COMPUTER_ID}\",\"ready\":true,\"ready_limit\":20,\"limit\":12}" -o "$ready"
  python3 -I - "$ready" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))["ready"]
caps = r["captures"]
print("ready=%d waiting=%d scanned=%d more=%s unavailable=%r" % (len(caps), r["waiting"], r["scanned"], r["more"], r.get("candidates_unavailable", "")))
for c in caps[:5]:
    print(c["vector"], c.get("outlet"), repr(c["title"][:60]), [(m["phrase"], [x["name"] for x in m["candidates"]][:3], "ambiguous" if m.get("ambiguous") else "") for m in c["mentions"][:4]])
sys.exit(0 if "waiting" in r else 1)
PY
  echo "part B ok"
else
  echo "part B skipped (set CORPUSD_URL, OWNER_ID, COMPUTER_ID on Node B)"
fi
