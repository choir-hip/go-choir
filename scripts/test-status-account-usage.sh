#!/usr/bin/env bash
# Behavioural test for scripts/status-account-usage: per-account usage from
# what Node B already records. The script splits fetching (one read-only ssh)
# from computing; this drives the computing through a --fixture file.
#
#   bash scripts/test-status-account-usage.sh
#
# Failure list, written before the code:
#  U1 a computer's calls are credited to the wrong account, or a computer
#     with no ownership row is dropped instead of shown as "(no owner)"
#  U2 an hour outside the last 24 hours is counted in "24h", or one outside
#     the window in "7d"; the hour line's own hour is used, not the log time
#  U3 an account with two computers is not one row with both summed
#  U4 dollars today and yesterday are not the spend file's per-computer
#     dollars summed over providers; an unreadable spend file reads as $0
#     instead of "unavailable"
#  U5 disk is not the newest growth-ledger value for the computer's data
#     image, or the ledger's "mixed" marker is read as a size
#  U6 accounts that own no computer are missing from the totals
#  U7 the report does not say how far back the call lines reach, so a short
#     journal reads as low use
#  U8 empty input crashes; any flag or non-zero exit exists (this is
#     monitoring, not a bound: the owner, 2026-11-24)
#  U9 rows are not ordered by calls in the last 24 hours, most first
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
S="$HERE/status-account-usage"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
pass=0; fail=0
check() { if eval "$2"; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "FAIL: $1"; fi; }

# now = 2026-11-24T12:30:00Z
NOW=1795523400
cat >"$T/f.json" <<JSON
{"now": $NOW,
 "users": "1097",
 "owners": "vm-aaa\tcomputer-1\tuser-alpha-0001\tactive\t2026-11-24T12:00:00Z\t1500000000\nvm-bbb\tcomputer-2\tuser-alpha-0001\tstopped\t2026-11-20T00:00:00Z\t0\nvm-ccc\tcomputer-3\tuser-beta-0002\tactive\t2026-11-24T11:00:00Z\t900000000",
 "calls": "2026/11/24 12:00:00 gateway: calls hour=2026-11-24T11:00Z computer=vm-aaa provider=devin calls=10 input_tokens=1000\n2026/11/24 12:00:00 gateway: calls hour=2026-11-24T11:00Z computer=vm-bbb provider=river calls=5 input_tokens=500\n2026/11/24 12:00:00 gateway: calls hour=2026-11-24T11:00Z computer=vm-ccc provider=devin calls=40 input_tokens=4000\n2026/11/23 12:00:00 gateway: calls hour=2026-11-23T11:00Z computer=vm-aaa provider=devin calls=7 input_tokens=700\n2026/11/24 12:00:00 gateway: calls hour=2026-11-10T11:00Z computer=vm-aaa provider=devin calls=900 input_tokens=90000\n2026/11/24 12:00:00 gateway: calls hour=2026-11-24T10:00Z computer=vm-zzz provider=search calls=3 input_tokens=0",
 "spend": "{\"day\":\"2026-11-24\",\"today\":{\"river\":{\"usd\":0.5,\"computers_usd\":{\"vm-bbb\":0.3,\"vm-ccc\":0.2}},\"bedrock\":{\"usd\":1,\"computers_usd\":{\"vm-aaa\":1.0}}},\"prev_day\":\"2026-11-23\",\"prev\":{\"river\":{\"usd\":2,\"computers_usd\":{\"vm-aaa\":2.0}}}}",
 "ledger": "{\"ts\":\"2026-11-24T11:00:00Z\",\"fs.vm-state.vm-aaa.data.img\":1073741824,\"fs.vm-state.vm-ccc.data.img\":\"mixed\"}\n{\"ts\":\"2026-11-24T12:00:00Z\",\"fs.vm-state.vm-aaa.data.img\":2147483648,\"fs.vm-state.vm-bbb.data.img\":3221225472}"}
JSON
"$S" --json --fixture "$T/f.json" >"$T/out.json" 2>"$T/err"; rc=$?
q() { python3 -c "import json,sys; d=json.load(open('$T/out.json')); print($1)" 2>/dev/null; }
check "U8 exit 0" '[ "$rc" = 0 ]'
check "U3 alpha is one row with two computers" '[ "$(q "[r[\"computers\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"]")" = "[2]" ]'
check "U1 U2 alpha calls 24h = 15" '[ "$(q "[r[\"calls_24h\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 15 ]'
check "U2 alpha calls 7d = 22 (the 11-10 hour is outside)" '[ "$(q "[r[\"calls_7d\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 22 ]'
check "U1 alpha input tokens 24h = 1500" '[ "$(q "[r[\"input_tokens_24h\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 1500 ]'
check "U1 beta calls 24h = 40" '[ "$(q "[r[\"calls_24h\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-beta-0002\"][0]")" = 40 ]'
check "U1 unowned computer is a row" '[ "$(q "[r[\"calls_24h\"] for r in d[\"accounts\"] if r[\"account\"]==\"(no owner)\"][0]")" = 3 ]'
check "U4 alpha usd today 1.3" '[ "$(q "round([r[\"usd_today\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0],4)")" = 1.3 ]'
check "U4 alpha usd yesterday 2.0" '[ "$(q "[r[\"usd_yesterday\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 2.0 ]'
check "U4 beta usd today 0.2" '[ "$(q "[r[\"usd_today\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-beta-0002\"][0]")" = 0.2 ]'
check "U5 alpha disk 5 GiB (2 newest + 3)" '[ "$(q "[r[\"disk_gib\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 5.0 ]'
check "U5 mixed is not a size" '[ "$(q "[r[\"disk_gib\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-beta-0002\"][0]")" = None ]'
check "U6 totals" '[ "$(q "(d[\"totals\"][\"accounts\"], d[\"totals\"][\"accounts_with_a_computer\"], d[\"totals\"][\"computers\"])")" = "(1097, 2, 3)" ]'
check "U7 window start is the oldest hour line" '[ "$(q "d[\"calls_reach_back_to\"]")" = "2026-11-10T11:00Z" ]'
check "U9 order by calls_24h" '[ "$(q "[r[\"account\"] for r in d[\"accounts\"]]")" = "['"'"'user-beta-0002'"'"', '"'"'user-alpha-0001'"'"', '"'"'(no owner)'"'"']" ]'
check "U1 active count" '[ "$(q "[r[\"active\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0]")" = 1 ]'

"$S" --fixture "$T/f.json" >"$T/out.txt" 2>&1; rc=$?
check "text exit 0" '[ "$rc" = 0 ]'
check "U7 text names the reach" 'grep -q "call lines reach back to 2026-11-10T11:00Z" "$T/out.txt"'
check "text has alpha row" 'grep -q "user-alpha-0001" "$T/out.txt"'

python3 - "$T/f.json" "$T/g.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); d["spend"]="not json"; json.dump(d,open(sys.argv[2],"w"))
PY
"$S" --json --fixture "$T/g.json" >"$T/out.json" 2>/dev/null; rc=$?
check "U4 unreadable spend file: exit 0" '[ "$rc" = 0 ]'
check "U4 unreadable spend file: unavailable, not zero" '[ "$(q "(d[\"spend_available\"], [r[\"usd_today\"] for r in d[\"accounts\"] if r[\"account\"]==\"user-alpha-0001\"][0])")" = "(False, None)" ]'

echo '{"now": '$NOW', "users": "", "owners": "", "calls": "", "spend": "", "ledger": ""}' >"$T/e.json"
"$S" --json --fixture "$T/e.json" >"$T/out.json" 2>/dev/null; rc=$?
check "U8 empty input: exit 0" '[ "$rc" = 0 ]'
check "U8 empty input: no rows" '[ "$(q "len(d[\"accounts\"])")" = 0 ]'
"$S" --fixture "$T/e.json" >/dev/null 2>&1; check "U8 empty text exit 0" '[ "$?" = 0 ]'

echo "status-account-usage: $pass passed, $fail failed"
[ "$fail" = 0 ]
