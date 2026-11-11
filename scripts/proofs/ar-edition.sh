#!/usr/bin/env bash
# Proof (Autoradio lane S, item 4): GET /api/texture/edition finds the
# owner's "My edition" Texture and answers its ids and time, never its text.
# Read-only; it answers found:false (not an error) before the interview has
# written one, which is reported and exits 3. Needs CHOIR_HOST and
# CHOIR_API_KEY_FILE (never echoed); set CHOIR_COMPUTER_ID for an owner-wide
# key. `--dry-run` only checks the inputs.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
: "${CHOIR_API_KEY_FILE:?set CHOIR_API_KEY_FILE}"
[ -r "$CHOIR_API_KEY_FILE" ] || { echo "cannot read CHOIR_API_KEY_FILE" >&2; exit 2; }
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
out=$(mktemp)
trap 'rm -f "$out"' EXIT
computer_header=()
if [ -n "${CHOIR_COMPUTER_ID:-}" ]; then computer_header=(-H "X-Choir-Computer: ${CHOIR_COMPUTER_ID}"); fi
code=$(printf 'header = "Authorization: Bearer %s"\n' "$(tr -d '\n\r ' < "$CHOIR_API_KEY_FILE")" |
  curl -sS -m 15 -K - -o "$out" -w '%{http_code}' ${computer_header[@]+"${computer_header[@]}"} "https://${CHOIR_HOST}/api/texture/edition")
echo "edition: http $code"
[ "$code" = 200 ] || { cat "$out"; echo; exit 1; }
python3 -I - "$out" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
print(r)
if not r.get("found"):
    print("no edition document yet"); sys.exit(3)
sys.exit(0 if r.get("doc_id") and set(r) <= {"found", "doc_id", "revision_id", "updated_at"} else 1)
PY
