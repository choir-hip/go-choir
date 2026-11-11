#!/usr/bin/env bash
# Proof (Autoradio lane S, item 1): the public front list and a story bundle
# carry each story's central entities as [{id, label}]. Read-only, public, no
# key. Needs CHOIR_HOST (e.g. choir.news). Exits non-zero unless every
# entity has a non-empty id and label and the bundle agrees with the front
# list for the first story that has entities (a freshly deployed paper may
# have none yet: that is reported and exits 3, not a pass).
# `--dry-run` only checks the inputs.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
front=$(mktemp)
trap 'rm -f "$front"' EXIT
curl -sS -m 20 -o "$front" "https://${CHOIR_HOST}/api/platform/publications/front?limit=60"
python3 -I - "$front" "$CHOIR_HOST" <<'PY'
import json, sys, urllib.request
front = json.load(open(sys.argv[1]))
host = sys.argv[2]
stories = front.get("stories") or front.get("items") or []
withents = [s for s in stories if s.get("entities")]
print(f"stories={len(stories)} with_entities={len(withents)}")
for s in withents:
    for e in s["entities"]:
        if not e.get("id") or not e.get("label"):
            print("bad entity", e); sys.exit(1)
if not withents:
    print("no story carries entities yet"); sys.exit(3)
s = withents[0]
route = s["route"]["bare"]
with urllib.request.urlopen(f"https://{host}/api/platform/publications/resolve?route={route}", timeout=20) as r:
    b = json.load(r)
bundle = b.get("entities") or []
print(f"{route}: front={[e['label'] for e in s['entities']]} bundle={[e['label'] for e in bundle]}")
sys.exit(0 if bundle == s["entities"] else 1)
PY
