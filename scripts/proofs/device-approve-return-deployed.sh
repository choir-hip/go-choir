#!/usr/bin/env bash
# Deployed proof (sign-in sheet, server and web part): what a stranger can see
# of POST /auth/device/approve after it learned the optional `return` and
# `state` fields. No credentials, no account, no session; read-only in effect
# (every request is refused before it reads a body or touches a decision).
#   - without a session, a JSON post answers 401 with or without the fields,
#     and the two bodies are identical (the fields change nothing a stranger
#     can see; a bad hint is not a 400)
#   - a form-encoded post answers 415 (JSON only, so a cross-site form cannot post)
#   - GET /device serves the approval page (200, html). This check does not
#     distinguish the new page from the old one: the web half is proven only by
#     the owner's phone.
# What it cannot check: that an approval answers return_to. That needs the
# owner's passkey session and a device code, an owner act, covered by the
# Go tests and the owner's phone.
# CHOIR_HOST defaults to https://choir.news (scheme included).
set -euo pipefail
HOST=${CHOIR_HOST:-https://choir.news}
case "$HOST" in http://* | https://*) ;; *) HOST="https://$HOST" ;; esac
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok: would probe $HOST"; exit 0; fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fails=0
want() { # what got want
	if [ "$2" = "$3" ]; then echo "PASS $1: $2"; else echo "FAIL $1: got $2, want $3"; fails=$((fails + 1)); fi
}
post_json() { # outfile body
	curl -sS -m 15 -o "$1" -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
		--data "$2" "$HOST/auth/device/approve"
}
plain=$(post_json "$work/plain" '{"user_code":"ZZZZ-ZZZZ","decision":"approve"}')
hinted=$(post_json "$work/hinted" '{"user_code":"ZZZZ-ZZZZ","decision":"approve","return":"news.choir.autoradio.lab","state":"0123456789abcdef0123456789abcdef"}')
badhint=$(post_json "$work/badhint" '{"user_code":"ZZZZ-ZZZZ","decision":"approve","return":"https://example.invalid/x","state":"not a nonce"}')
want "no session, no fields" "$plain" 401
want "no session, return and state" "$hinted" 401
want "no session, bad return and state" "$badhint" 401
if cmp -s "$work/plain" "$work/hinted" && cmp -s "$work/plain" "$work/badhint"; then
	echo "PASS the three 401 bodies are identical"
else
	echo "FAIL the 401 bodies differ"
	fails=$((fails + 1))
fi
form=$(curl -sS -m 15 -o /dev/null -w '%{http_code}' -X POST \
	-H 'Content-Type: application/x-www-form-urlencoded' \
	--data 'user_code=ZZZZ-ZZZZ&decision=approve&return=news.choir.autoradio.lab' "$HOST/auth/device/approve")
want "form-encoded post" "$form" 415
page=$(curl -sS -m 15 -o "$work/page" -w '%{http_code} %{content_type}' "$HOST/device")
case "$page" in
	"200 text/html"*) echo "PASS GET /device: $page" ;;
	*) echo "FAIL GET /device: $page"; fails=$((fails + 1)) ;;
esac
[ "$fails" = 0 ] || { echo "$fails check(s) failed" >&2; exit 1; }
echo "all checks passed"
