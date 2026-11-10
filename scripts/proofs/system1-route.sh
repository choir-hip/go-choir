#!/usr/bin/env bash
# Proof: a bearer API key asks System 1 two choice questions through its own
# computer (POST /api/system1/judgments). Needs CHOIR_HOST (e.g. choir.news)
# and CHOIR_API_KEY_FILE (a file holding the key; never echoed). Prints the
# answers and timings; exits non-zero unless both answers are among the offered
# choices and total_ms is present. `--dry-run` only checks the inputs.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
: "${CHOIR_API_KEY_FILE:?set CHOIR_API_KEY_FILE}"
[ -r "$CHOIR_API_KEY_FILE" ] || { echo "cannot read CHOIR_API_KEY_FILE" >&2; exit 2; }
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
body='{"label":"proof","state":{"context":"A listener asks for something calm to hear while cooking dinner."},"questions":{"mood":{"type":"choice","instructions":"Which mood fits the request best?","choices":["calm","energetic","somber"]},"talk":{"type":"choice","instructions":"Is the listener asking for spoken word or music?","choices":["speech","music","either"]}}}'
out=$(mktemp)
trap 'rm -f "$out"' EXIT
# The key goes through a curl config on stdin, so it is never in argv.
code=$(printf 'header = "Authorization: Bearer %s"\n' "$(tr -d '\n\r ' < "$CHOIR_API_KEY_FILE")" |
  curl -sS -m 10 -K - -o "$out" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' --data "$body" \
    "https://${CHOIR_HOST}/api/system1/judgments")
echo "http $code"
cat "$out"
echo
[ "$code" = 200 ] || exit 1
python3 -I - "$out" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
offered = {"mood": {"calm", "energetic", "somber"}, "talk": {"speech", "music", "either"}}
ok = "total_ms" in r and "upstream_ms" in r
for name, choices in offered.items():
    a = r.get("answers", {}).get(name, {})
    print(f"{name}: {a.get('choice')} (confidence {a.get('confidence')})")
    ok = ok and a.get("choice") in choices
print(f"upstream_ms={r.get('upstream_ms')} total_ms={r.get('total_ms')}")
sys.exit(0 if ok else 1)
PY
