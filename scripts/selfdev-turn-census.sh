#!/usr/bin/env bash
# Census receipt: turn outcomes vs revision outcomes for self-development
# supervision trajectories on staging (Definition: scheduling-and-candidate-proof-2026-08-21,
# Phase 2 census gate).
#
# Usage: CHOIR_API_KEY=... ./scripts/selfdev-turn-census.sh [computer_id]
# Requires curl + python3. Read-only against the owner product path.
set -euo pipefail

HOST="${CHOIR_HOST:-https://choir.news}"
KEY="${CHOIR_API_KEY:?CHOIR_API_KEY required}"
CID="${1:-computer-03335285269bdba4f94377e56879f9e6}"

api() { # method path [body]
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS --max-time 60 -X "$method" -H "Authorization: Bearer $KEY" \
      -H "X-Choir-Computer: $CID" -H "Content-Type: application/json" \
      -d "$body" "$HOST$path"
  else
    curl -sS --max-time 60 -H "Authorization: Bearer $KEY" \
      -H "X-Choir-Computer: $CID" "$HOST$path"
  fi
}

echo "== computer: $CID"
DOCS_JSON="$(api GET /api/texture/documents)"
SELFDEV_DOCS="$(printf '%s' "$DOCS_JSON" | python3 -c '
import json,sys
d=json.load(sys.stdin)
for doc in d.get("documents",[]):
    if "elf-development" in doc.get("title",""):
        print(doc["doc_id"])')"
DOC_COUNT=$(printf '%s\n' "$SELFDEV_DOCS" | grep -c . || true)
echo "== selfdev supervision documents: $DOC_COUNT"

TOTAL_TURNS=0; TOTAL_REV=0; TOTAL_WAIT=0; TOTAL_BLOCK=0; TOTAL_NSC=0
{
  printf '%s\n' "$SELFDEV_DOCS" | while read -r doc; do
    api GET "/api/texture/documents/$doc/revisions" | python3 -c '
import json,sys
d=json.load(sys.stdin)
revs=d.get("revisions",[])
agent=[r for r in revs if r.get("author_kind")=="appagent"]
print(f"{len(revs)} revisions, {len(agent)} appagent-authored")'
  done
} || true

TRAJ_JSON="$(api GET /api/trajectories?limit=200)"
printf '%s' "$TRAJ_JSON" | python3 - "$CID" <<'PYEOF'
import json,sys,subprocess,os
cid=sys.argv[1]
data=json.load(sys.stdin)
trajs=data.get("trajectories",[])
host=os.environ.get("HOST_BASE","https://choir.news")
key=os.environ["CHOIR_API_KEY"]
def api(path):
    out=subprocess.run(["curl","-sS","--max-time","60","-H",f"Authorization: Bearer {key}",
        "-H",f"X-Choir-Computer: {cid}", f"https://choir.news{path}"],capture_output=True,text=True)
    return json.loads(out.stdout)
turns={}; rev=wait=block=nsc=0; turn_total=0
for t in trajs:
    tid=t.get("trajectory_id"); 
    if not tid: continue
    page=api(f"/api/trajectories/{tid}/events?limit=500")
    for e in page.get("events",[]):
        k=e.get("kind","")
        if k=="texture_turn_committed":
            turn_total+=1
            reason=(e.get("reason") or "")
            # outcome is in the command receipt; approximate by artifact refs
            refs=e.get("artifact_refs") or []
            turns[tid]=turns.get(tid,0)+1
    rev+=sum(1 for e in page.get("events",[]) if e.get("kind")=="artifact_head_advanced")
print(f"selfdev-relevant trajectories scanned: {len(trajs)}")
print(f"texture_turn_committed events: {turn_total}")
print(f"artifact head advances (revisions): {rev}")
PYEOF
