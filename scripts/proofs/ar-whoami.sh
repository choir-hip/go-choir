#!/usr/bin/env bash
# Proof (Autoradio lane S, item 2): GET /auth/whoami answers a bearer key with
# its public handle, key id and computer binding; it does not
# answer without a key and never carries an email address. Needs CHOIR_HOST
# and CHOIR_API_KEY_FILE (a file holding the key; never echoed). Read-only.
# `--dry-run` only checks the inputs.
set -euo pipefail
: "${CHOIR_HOST:?set CHOIR_HOST}"
: "${CHOIR_API_KEY_FILE:?set CHOIR_API_KEY_FILE}"
[ -r "$CHOIR_API_KEY_FILE" ] || { echo "cannot read CHOIR_API_KEY_FILE" >&2; exit 2; }
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok"; exit 0; fi
out=$(mktemp)
trap 'rm -f "$out"' EXIT
anon=$(curl -sS -m 10 -o /dev/null -w '%{http_code}' "https://${CHOIR_HOST}/auth/whoami")
echo "anonymous: http $anon (want 401)"
# The key goes through a curl config on stdin, so it is never in argv.
code=$(printf 'header = "Authorization: Bearer %s"\n' "$(tr -d '\n\r ' < "$CHOIR_API_KEY_FILE")" |
  curl -sS -m 10 -K - -o "$out" -w '%{http_code}' "https://${CHOIR_HOST}/auth/whoami")
echo "bearer: http $code"
[ "$anon" = 401 ] && [ "$code" = 200 ] || exit 1
python3 -I - "$out" <<'PY'
import json, re, sys
r = json.load(open(sys.argv[1]))
print({k: r.get(k) for k in ("handle", "key_id", "computer_id")})
ok = "user_id" not in r and re.fullmatch(r"writer-[0-9a-f]{6}", r.get("handle", "")) is not None and bool(r.get("key_id"))
ok = ok and "@" not in json.dumps(r)
sys.exit(0 if ok else 1)
PY
