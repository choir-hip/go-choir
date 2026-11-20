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
# Every entity a front-page story lists (the first 15) must answer 200 with
# that story among its stories, or 404 (a person without a page, by the
# owner-question-1 default). A 200 that lists no story fails, and so does a
# run in which no entity answers 200: the read must agree with the front list.
python3 -I - "$front" "$CHOIR_HOST" <<'PY'
import json, sys, urllib.request, urllib.error
front = json.load(open(sys.argv[1])); host = sys.argv[2]
seen, rows = set(), []
for s in front.get("stories") or front.get("items") or []:
    for e in s.get("entities") or []:
        if e["id"] not in seen and len(rows) < 15:
            seen.add(e["id"]); rows.append((e["id"], e["label"], s["publication_id"]))
if not rows:
    print("no front story lists an entity"); sys.exit(3)
def get(url):
    try:
        with urllib.request.urlopen(url, timeout=20) as r:
            return r.status, json.load(r)
    except urllib.error.HTTPError as e:
        return e.code, None
ok = bad = 0
for eid, label, pub in rows:
    code, p = get(f"https://{host}/api/platform/entities/{eid}?stories=50")
    if code == 404:
        print(f"404 {label} (a person without a page)"); continue
    if code != 200:
        print(f"FAIL {label}: status {code}"); bad += 1; continue
    ids = [x["publication_id"] for x in p["story_list"]]
    print(f"200 {label}: stories={p['stories']} central={p['central_stories']} listed={len(ids)} front story listed={pub in ids}")
    if not ids or pub not in ids:
        print(f"FAIL {label}: the front story {pub} is not among its stories"); bad += 1
    else:
        ok += 1
if bad or not ok:
    print(f"part A failed: {ok} entities agree with the front list, {bad} do not"); sys.exit(1)
PY
# Stranger bounds: more than 50 stories, and a malformed id.
eid=$(python3 -I -c 'import json,sys; f=json.load(open(sys.argv[1])); print(next(e["id"] for s in (f.get("stories") or f.get("items") or []) for e in (s.get("entities") or [])))' "$front")
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
