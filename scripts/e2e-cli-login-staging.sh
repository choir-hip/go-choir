#!/usr/bin/env bash
# Deployed acceptance for choir login (docs/design/cli-agent-onboarding-2026-11-03.md,
# Acceptance): a person or agent at a terminal signs in with no key in hand.
#
#   scripts/e2e-cli-login-staging.sh [--dry-run] [out-dir]
#
# Env: CHOIR_BACKEND (default https://choir.news). Needs chromium for
# Playwright (cd frontend && npm ci && npx playwright install chromium) and
# open registration (a fresh example.com account is registered each run).
#
# Steps (each prints one PASS/FAIL line; the run writes <out>/acceptance.json):
#   L0 choir login (isolated XDG_CONFIG_HOME, no key in flags or env) prints
#      an approve link on this backend and a code
#   A1 frontend/tests/cli-login-approve-deployed.spec.js registers a fresh
#      account with a virtual passkey, opens the link signed out, signs in on
#      the page, sees the CLI's label, and approves
#   L1 choir login exits 0 and writes the credentials file, mode 0600
#   C1 the account's computer is listed (GET /api/compute/status with the key;
#      the first read by an owner-wide key creates it)
#   S1 choir self-dev list --computer <id> answers with no key flag or env
#   R1 choir logout revokes the key and removes the file; the old key now
#      answers 401
# The key is never printed: it lives in the credentials file under <out>,
# which the script deletes at the end.
set -euo pipefail

dry=0
while [[ "${1:-}" == --* ]]; do
  case "$1" in
    --dry-run) dry=1 ;;
    *) echo "unknown flag $1" >&2; exit 2 ;;
  esac
  shift
done
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend="${CHOIR_BACKEND:-https://choir.news}"
out="${1:-$(mktemp -d "${TMPDIR:-/tmp}/cli-login-e2e.XXXXXX")}"
mkdir -p "$out/logs"
spec="tests/cli-login-approve-deployed.spec.js"

if [[ $dry == 1 ]]; then
  echo "dry run: backend=$backend out=$out"
  (cd "$repo" && go build -o "$out/choir" ./cmd/choir) && echo "dry run: choir CLI builds" || { echo "FAIL choir CLI does not build"; exit 1; }
  XDG_CONFIG_HOME="$out/config" "$out/choir" login --help >/dev/null 2>&1 || true
  "$out/choir" help | grep -q '^  login' && echo "dry run: choir login is registered" || { echo "FAIL choir help lists no login"; exit 1; }
  if [[ -d "$repo/frontend/node_modules/@playwright" ]]; then
    (cd "$repo/frontend" && npx playwright test "$spec" --list >"$out/logs/spec-list.log" 2>&1) \
      && echo "dry run: approve spec loads" || { echo "FAIL approve spec does not load (see $out/logs/spec-list.log)"; exit 1; }
  else
    node --check "$repo/frontend/$spec" && echo "dry run: approve spec parses (frontend/node_modules absent; not loaded by Playwright)"
  fi
  echo "dry run: steps L0 A1 L1 C1 S1 R1; no network calls made"
  exit 0
fi

steps="$out/steps.jsonl"
: >"$steps"
fails=0
step() { # id pass|fail detail
  local id=$1 res=$2 detail=$3
  echo "$( [[ $res == pass ]] && echo PASS || echo FAIL ) $id: $detail"
  python3 -c 'import json,sys;print(json.dumps({"step":sys.argv[1],"result":sys.argv[2],"detail":sys.argv[3]}))' "$id" "$res" "$detail" >>"$steps"
  [[ $res == pass ]] || fails=$((fails + 1))
}
finish() {
  # Every exit path revokes the run's key: a key the run minted but did not
  # log out (a failed C1 or S1, an interrupt) must not outlive it.
  [[ -n "${login_pid:-}" ]] && kill "$login_pid" 2>/dev/null || true
  if [[ -e "${creds:-/nonexistent}" ]]; then
    if "$out/choir" logout >/dev/null 2>"$out/logs/exit-logout.err"; then
      echo "exit: revoked the run's key (choir logout)"
    else
      echo "WARN exit: could not revoke the run's key: $(tail -1 "$out/logs/exit-logout.err")" >&2
      fails=$((fails + 1))
    fi
  fi
  rm -rf "$out/config"
  python3 - "$out" "$backend" "$fails" <<'PY'
import json, sys, datetime
out, backend, fails = sys.argv[1], sys.argv[2], int(sys.argv[3])
steps = [json.loads(l) for l in open(f"{out}/steps.jsonl") if l.strip()]
json.dump({"acceptance": "choir-login-staging", "backend": backend,
           "at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
           "passed": fails == 0 and len(steps) == 6, "steps": steps},
          open(f"{out}/acceptance.json", "w"), indent=2)
PY
  echo "acceptance: $out/acceptance.json"
}
trap finish EXIT
trap 'exit 130' INT TERM

(cd "$repo" && go build -o "$out/choir" ./cmd/choir)
export XDG_CONFIG_HOME="$out/config"
unset CHOIR_API_KEY CHOIR_API_KEY_FILE CHOIR_HOST CHOIR_COMPUTER
creds="$XDG_CONFIG_HOME/choir/credentials"
label="e2e cli-login $(date -u +%Y%m%dT%H%M%SZ)"

# L0: start the login in the background and wait for its link.
"$out/choir" login --host "$backend" --label "$label" >"$out/login.json" 2>"$out/logs/login.err" &
login_pid=$!
link=""
for _ in $(seq 1 60); do
  link="$(grep -Eo "${backend}/device\?code=[A-Z]{4}-[A-Z]{4}" "$out/logs/login.err" | head -1 || true)"
  [[ -n "$link" ]] && break
  kill -0 "$login_pid" 2>/dev/null || break
  sleep 1
done
if [[ -z "$link" ]]; then
  step L0 fail "choir login printed no approve link: $(tail -3 "$out/logs/login.err" | tr '\n' ' ')"
  kill "$login_pid" 2>/dev/null || true
  exit 1
fi
step L0 pass "choir login printed an approve link on $backend"
printf '%s\n' "$link" >"$out/link.txt"

# A1: approve in the browser.
set +e
(cd "$repo/frontend" && BASE_URL="$backend" CLI_LOGIN_LINK_FILE="$out/link.txt" CLI_LOGIN_LABEL="$label" \
  CLI_LOGIN_RESULT_FILE="$out/account.json" \
  npx playwright test "$spec" --project=chromium --workers=1 --reporter=list) >"$out/logs/approve.log" 2>&1
rc=$?
set -e
if [[ $rc != 0 ]]; then
  step A1 fail "approve spec failed (log $out/logs/approve.log): $(grep -E 'Error|expect' "$out/logs/approve.log" | head -3 | tr '\n' ' ')"
  kill "$login_pid" 2>/dev/null || true
  exit 1
fi
step A1 pass "a fresh passkey account saw the label \"$label\" on the approve page and approved"

# L1: the CLI collects the key.
set +e
wait "$login_pid"
rc=$?
set -e
mode="$(stat -c '%a' "$creds" 2>/dev/null || stat -f '%Lp' "$creds" 2>/dev/null || echo none)"
if [[ $rc == 0 && "$mode" == 600 ]]; then
  step L1 pass "choir login exited 0; credentials file mode $mode; key $(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["key_id"])' "$out/login.json")"
else
  step L1 fail "choir login exit $rc, credentials mode $mode: $(tail -2 "$out/logs/login.err" | tr '\n' ' ')"
  exit 1
fi

# C1: find the account's computer with the saved key (never printed).
computer=""
# A first sign-in creates the computer on the first status read (the proxy
# starts it in the background); a cold first boot takes minutes, so give the
# listing and the first answer 10 minutes each.
for _ in $(seq 1 120); do
  computer="$(python3 - "$creds" <<'PY' || true
import json, sys, urllib.request
c = json.load(open(sys.argv[1]))
req = urllib.request.Request(c["host"] + "/api/compute/status", headers={"Authorization": "Bearer " + c["api_key"]})
try:
    body = json.load(urllib.request.urlopen(req, timeout=30))
except Exception:
    sys.exit(0)
for comp in body.get("computers") or []:
    if comp.get("computer_id"):
        print(comp["computer_id"]); break
PY
)"
  [[ -n "$computer" ]] && break
  sleep 5
done
if [[ -z "$computer" ]]; then
  step C1 fail "no computer listed for the account within 10 minutes"
  exit 1
fi
step C1 pass "the account's computer is $computer"

# S1: choir self-dev list with no key flag or env.
ok=0
for _ in $(seq 1 120); do
  if "$out/choir" self-dev list --computer "$computer" >"$out/selfdev-list.json" 2>"$out/logs/selfdev.err"; then
    ok=1; break
  fi
  sleep 5
done
if [[ $ok == 1 ]] && python3 -c 'import json,sys;json.load(open(sys.argv[1]))' "$out/selfdev-list.json"; then
  step S1 pass "choir self-dev list --computer $computer answered JSON with no key flag or env"
else
  step S1 fail "choir self-dev list: $(tail -2 "$out/logs/selfdev.err" | tr '\n' ' ')"
fi

# R1: logout revokes.
old_key="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["api_key"])' "$creds")"
if "$out/choir" logout >"$out/logout.json" 2>"$out/logs/logout.err" && [[ ! -e "$creds" ]]; then
  code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $old_key" "$backend/auth/api-keys")"
  if [[ "$code" == 401 ]]; then
    step R1 pass "choir logout removed the file; the old key answers 401"
  else
    step R1 fail "after logout the old key answers HTTP $code"
  fi
else
  step R1 fail "choir logout failed: $(tail -2 "$out/logs/logout.err" | tr '\n' ' ')"
fi
old_key=""
[[ $fails == 0 ]]
