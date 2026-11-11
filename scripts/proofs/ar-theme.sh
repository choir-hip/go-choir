#!/usr/bin/env bash
# Proof (Autoradio lane S, item 3): GET /api/texture/theme?doc= serves a
# validated version 3 theme from a Texture that holds one fenced choir-theme
# block, and refuses a missing document. Read-only. Needs CHOIR_HOST,
# CHOIR_API_KEY_FILE (never echoed) and CHOIR_THEME_DOC (the id of a theme
# Texture on the key's computer; make one by hand first, this script writes
# nothing). Set CHOIR_COMPUTER_ID for an owner-wide key.
# `--dry-run` only checks the inputs.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
: "${CHOIR_API_KEY_FILE:?set CHOIR_API_KEY_FILE}"
: "${CHOIR_THEME_DOC:?set CHOIR_THEME_DOC}"
[ -r "$CHOIR_API_KEY_FILE" ] || { echo "cannot read CHOIR_API_KEY_FILE" >&2; exit 2; }
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
out=$(mktemp)
trap 'rm -f "$out"' EXIT
computer_header=()
if [ -n "${CHOIR_COMPUTER_ID:-}" ]; then computer_header=(-H "X-Choir-Computer: ${CHOIR_COMPUTER_ID}"); fi
get() { # path -> http code, body in $out; the key goes through curl config on stdin
  printf 'header = "Authorization: Bearer %s"\n' "$(tr -d '\n\r ' < "$CHOIR_API_KEY_FILE")" |
    curl -sS -m 15 -K - -o "$out" -w '%{http_code}' ${computer_header[@]+"${computer_header[@]}"} "https://${CHOIR_HOST}$1"
}
code=$(get "/api/texture/theme?doc=${CHOIR_THEME_DOC}")
echo "theme: http $code"
[ "$code" = 200 ] || { cat "$out"; echo; exit 1; }
python3 -I - "$out" <<'PY' || exit 1
import json, sys
r = json.load(open(sys.argv[1]))
t = r.get("theme", {})
print({"doc_id": r.get("doc_id"), "revision_id": r.get("revision_id"), "id": t.get("id"), "schemes": t.get("schemes"), "ignored": r.get("ignored")})
sys.exit(0 if t.get("schema_version") == 3 and t.get("id") and t.get("colors") else 1)
PY
code=$(get "/api/texture/theme?doc=does-not-exist")
echo "missing doc: http $code (want 404)"
[ "$code" = 404 ]
