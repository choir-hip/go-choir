#!/usr/bin/env bash
# Deployed acceptance for sync slice 1 (lane/mac-follow): the follower
# (choir-bundle follow, the same internal/tapefollow syncer the Mac app runs)
# keeps a verified, read-only local copy of d@e's computer from staging.
#
#   scripts/e2e-mac-follow-staging.sh [--dry-run] [out-dir]
#
# Env: CHOIR_BACKEND (default https://choir.news), CHOIR_FOLLOW_COMPUTER
# (default d@e, computer-59c902c5d46caecc0a180a8557164e72), CHOIR_ENV_FILE
# (default /Users/wiz/Dev/go-choir/.env, read for CHOIR_TEST_D_E_API_KEY),
# FOLLOW_MAX_EVENTS (events per pulled bundle, default 2000),
# FOLLOW_MAX_BUNDLES (0, the default: pull until caught up).
#
# Steps (each prints one PASS/FAIL line; the run writes <out>/acceptance.json):
#   A1 unauthenticated GET refused 401
#   A2 the d@e key on another computer refused 403
#   A3 a write method refused 405
#   P1 a one-event bootstrap probe answers 200 with the range headers
#   S1 pull three pages (500 events each), then SIGKILL a pull mid-stream
#   S2 the store reopens to its last committed head and verifies as one tape
#   S3 resume and catch up (or FOLLOW_MAX_BUNDLES pages); verify again
#   S4 the local head is the server's (a 204 for have=head), or the server
#      moved on (200), never a 409
#   R1 every local bundle file is read-only
# The key is never printed: it reaches curl and choir-bundle only through
# process substitution (-H @<(printf ...), --header-file <(printf ...)).
set -euo pipefail

dry=0
if [[ "${1:-}" == "--dry-run" ]]; then dry=1; shift; fi
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend="${CHOIR_BACKEND:-https://choir.news}"
computer="${CHOIR_FOLLOW_COMPUTER:-computer-59c902c5d46caecc0a180a8557164e72}"
env_file="${CHOIR_ENV_FILE:-/Users/wiz/Dev/go-choir/.env}"
max_events="${FOLLOW_MAX_EVENTS:-2000}"
max_bundles="${FOLLOW_MAX_BUNDLES:-0}"
out="${1:-$(mktemp -d "${TMPDIR:-/tmp}/mac-follow-e2e.XXXXXX")}"
store="$out/store/$computer"
bin="$out/bin/choir-bundle"
mkdir -p "$out/bin" "$out/logs"
results=()
fails=0

pass() { echo "PASS $1: $2"; results+=("{\"step\":\"$1\",\"ok\":true,\"detail\":$(printf '%s' "$2" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')}"); }
fail() { echo "FAIL $1: $2"; fails=$((fails + 1)); results+=("{\"step\":\"$1\",\"ok\":false,\"detail\":$(printf '%s' "$2" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')}"); }

echo "building choir-bundle into $bin"
(cd "$repo" && go build -o "$bin" ./cmd/choir-bundle)

if [[ $dry == 1 ]]; then
  "$bin" follow 2>&1 | head -1 || true
  echo "dry run: backend=$backend computer=$computer out=$out max_events=$max_events max_bundles=$max_bundles"
  echo "dry run: steps A1 A2 A3 P1 S1 S2 S3 S4 R1 (no network calls made)"
  exit 0
fi

key="$(grep -E '^(export )?CHOIR_TEST_D_E_API_KEY=' "$env_file" | tail -1 | sed -E 's/^(export )?CHOIR_TEST_D_E_API_KEY=//; s/^["'\'']//; s/["'\'']$//')"
if [[ -z "$key" ]]; then
  echo "CHOIR_TEST_D_E_API_KEY not found in $env_file" >&2
  exit 2
fi
auth() { printf 'Authorization: Bearer %s\n' "$key"; }
route="$backend/api/computers/$computer/tape/bundle"

# A1
code="$(curl -sS -o /dev/null -w '%{http_code}' "$route?branch=main&max_events=1")"
[[ "$code" == 401 ]] && pass A1 "unauthenticated: $code" || fail A1 "unauthenticated: $code, want 401"

# A2
code="$(curl -sS -o /dev/null -w '%{http_code}' -H @<(auth) "$backend/api/computers/computer-00000000000000000000000000000000/tape/bundle?branch=main&max_events=1")"
[[ "$code" == 403 ]] && pass A2 "foreign computer: $code" || fail A2 "foreign computer: $code, want 403"

# A3
code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST -H @<(auth) "$route?branch=main")"
[[ "$code" == 405 ]] && pass A3 "POST: $code" || fail A3 "POST: $code, want 405"

# P1
hdrs="$out/logs/probe.headers"
code="$(curl -sS -o "$out/logs/probe.tar" -D "$hdrs" -w '%{http_code}' -H @<(auth) "$route?branch=main&max_events=1")"
src_head="$(grep -i '^x-choir-source-head-seq:' "$hdrs" | tr -d '\r' | awk '{print $2}')"
if [[ "$code" == 200 ]] && grep -qi '^x-choir-bundle-head-digest:' "$hdrs" && tar -tf "$out/logs/probe.tar" | grep -qx manifest.json; then
  pass P1 "bootstrap probe 200, source head $src_head, $(wc -c <"$out/logs/probe.tar" | tr -d ' ') bytes"
else
  fail P1 "bootstrap probe $code (headers in $hdrs)"
fi

follow() { "$bin" follow --backend "$backend" --computer "$computer" --store "$store" --header-file <(auth) "$@"; }

# S1
if follow --max-bundles 3 --max-events 500 >"$out/logs/s1.jsonl" 2>"$out/logs/s1.err"; then
  pass S1a "three pages: $(tail -1 "$out/logs/s1.jsonl")"
else
  fail S1a "three pages: $(tail -3 "$out/logs/s1.err" "$out/logs/s1.jsonl" | tr '\n' ' ')"
fi
before="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["head_seq"])' "$store/follow.json" 2>/dev/null || echo -1)"
follow --once --max-events "$max_events" >"$out/logs/s1-killed.jsonl" 2>"$out/logs/s1-killed.err" &
pid=$!
sleep 3
kill -9 "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true
pass S1b "SIGKILLed a pull at local head $before"

# S2
if "$bin" follow --computer "$computer" --store "$store" --verify >"$out/logs/s2.json" 2>"$out/logs/s2.err"; then
  pass S2 "reopened and verified after the kill: $(cat "$out/logs/s2.json")"
else
  fail S2 "verify after the kill: $(cat "$out/logs/s2.err")"
fi

# S3
s3args=(--once --max-events "$max_events")
if [[ "$max_bundles" != 0 ]]; then s3args=(--max-bundles "$max_bundles" --max-events "$max_events"); fi
t0=$(date +%s)
if follow "${s3args[@]}" >"$out/logs/s3.jsonl" 2>"$out/logs/s3.err"; then
  pass S3a "resumed in $(( $(date +%s) - t0 ))s: $(tail -1 "$out/logs/s3.jsonl")"
else
  fail S3a "resume: $(tail -3 "$out/logs/s3.err" "$out/logs/s3.jsonl" | tr '\n' ' ')"
fi
if "$bin" follow --computer "$computer" --store "$store" --verify >"$out/logs/s3-verify.json" 2>"$out/logs/s3-verify.err"; then
  pass S3b "verified as one tape: $(cat "$out/logs/s3-verify.json")"
else
  fail S3b "verify: $(cat "$out/logs/s3-verify.err")"
fi

# S4
read -r hseq hdig < <(python3 -c 'import json,sys;s=json.load(open(sys.argv[1]));print(s["head_seq"], s["head_digest"])' "$store/follow.json")
code="$(curl -sS -o /dev/null -D "$out/logs/s4.headers" -w '%{http_code}' -H @<(auth) "$route?branch=main&max_events=1&have_seq=$hseq&have_head=$hdig")"
srv="$(grep -i '^x-choir-source-head-seq:' "$out/logs/s4.headers" | tr -d '\r' | awk '{print $2}')"
case "$code" in
  204) pass S4 "local head $hseq is the server's head ($srv)" ;;
  200) pass S4 "local head $hseq is on the server's tape; the server moved on to $srv" ;;
  *) fail S4 "have=$hseq answered $code (409 means the local copy is not on the tape)" ;;
esac

# R1
writable="$(find "$store/bundles" -type f -perm -u+w | head -5)"
[[ -z "$writable" ]] && pass R1 "$(find "$store/bundles" -type f | wc -l | tr -d ' ') local files, none writable" || fail R1 "writable: $writable"

du_bytes="$(du -sk "$store" | awk '{print $1*1024}')"
{
  printf '{"backend":"%s","computer_id":"%s","store":"%s","store_bytes":%s,"fails":%d,"steps":[' "$backend" "$computer" "$store" "$du_bytes" "$fails"
  (IFS=,; printf '%s' "${results[*]}")
  printf '],"follow_state":'
  cat "$store/follow.json"
  printf '}\n'
} >"$out/acceptance.json"
echo "artifact: $out/acceptance.json"
[[ $fails == 0 ]]
