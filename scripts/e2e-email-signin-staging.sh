#!/usr/bin/env bash
# Deployed acceptance for email sign-in (docs/design/email-sign-in-2026-10-31.md,
# "Automated test (no person)"): a dedicated test account on choir.news signs in
# by emailed code through the Mac app's own native sign-in, the code read from
# the account's own mailbox through the product mail API.
#
#   scripts/e2e-email-signin-staging.sh [--dry-run] [--bootstrap-only] [out-dir]
#
# Env: CHOIR_BACKEND (default https://choir.news), CHOIR_E2E_STATE (default
# ~/.config/choir-e2e/email-signin.json; mode 0600, outside the repo),
# E2E_NODE_B_SSH (default node-b, used once by the bootstrap).
#
# Bootstrap (once; skipped when the state file says stage "done"):
#   frontend/tests/email-signin-bootstrap-deployed.spec.js registers the test
#   account at a reserved numbered test address (ssh node-b maildctl
#   reserve-test-number) with a virtual WebAuthn passkey, binds the number to
#   the account (maildctl bind-number, between register/begin and finish; the
#   public receive policy quarantines, the key reads the quarantine folder),
#   mints a read:runtime
#   key from the passkey session, completes verify-address, and waits for the
#   account's computer. Needs: ssh access to node-b, chromium for Playwright
#   (cd frontend && npm ci && npx playwright install chromium), registration
#   open (AUTH_REGISTRATION_CAP unset or above the count; a capped deploy answers
#   registration_waitlist and the operator raises the cap).
#
# Each run (cmd/desktop TestE2EEmailSignInStaging, native handlers + cookie jar):
#   I0 the scoped key reads the test inbox
#   B1 native begin (POST /auth/email/begin) answers an attempt id
#   M1 the new sign-in mail is read from the inbox and its code extracted
#   V0 a wrong code is refused
#   V1 native verify with the code signs in
#   V2 replaying the code is refused (single use)
#   S1 GET /auth/session shows the test account; the access token method is email_*
#   G1 POST /auth/api-keys with the email session answers 403
#   F1 follow sync of the account's computer to the server head with that session
#   F2 the local copy verifies as one tape
#   F3 the local head is the server head
# Each step prints one PASS/FAIL line; the run writes <out>/acceptance.json.
# The key, code, link, cookies and address are never printed: they live in the
# state file and in process memory.
set -euo pipefail

dry=0
bootstrap_only=0
while [[ "${1:-}" == --* ]]; do
  case "$1" in
    --dry-run) dry=1 ;;
    --bootstrap-only) bootstrap_only=1 ;;
    *) echo "unknown flag $1" >&2; exit 2 ;;
  esac
  shift
done
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend="${CHOIR_BACKEND:-https://choir.news}"
state="${CHOIR_E2E_STATE:-$HOME/.config/choir-e2e/email-signin.json}"
out="${1:-$(mktemp -d "${TMPDIR:-/tmp}/email-signin-e2e.XXXXXX")}"
mkdir -p "$out/logs"

stage=""
if [[ -f "$state" ]]; then
  mode="$(stat -c '%a' "$state" 2>/dev/null || stat -f '%Lp' "$state")"
  if [[ "$mode" != 600 ]]; then
    echo "state file mode is $mode, want 600: chmod 600 it" >&2
    exit 2
  fi
  stage="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1])).get("stage",""))' "$state")"
fi

if [[ $dry == 1 ]]; then
  echo "dry run: backend=$backend out=$out"
  echo "dry run: state file $state: ${stage:-absent} (bootstrap $([[ "$stage" == done ]] && echo skipped || echo would run))"
  echo "dry run: compiling the Go harness (cmd/desktop, no network)"
  (cd "$repo/cmd/desktop" && go test -c -o "$out/desktop.test" . >"$out/logs/compile.log" 2>&1) \
    && echo "dry run: harness compiles" || { echo "FAIL harness does not compile (see $out/logs/compile.log)"; exit 1; }
  (cd "$repo/cmd/desktop" && go test -run 'TestE2E(InboxNewestCode|AccessMethodReadsTheJar)' -count=1 . >"$out/logs/selftest.log" 2>&1) \
    && echo "dry run: harness selftest passes" || { echo "FAIL harness selftest (see $out/logs/selftest.log)"; exit 1; }
  if [[ -d "$repo/frontend/node_modules/@playwright" ]]; then
    (cd "$repo/frontend" && npx playwright test tests/email-signin-bootstrap-deployed.spec.js --list >"$out/logs/spec-list.log" 2>&1) \
      && echo "dry run: bootstrap spec loads" || { echo "FAIL bootstrap spec does not load (see $out/logs/spec-list.log)"; exit 1; }
  else
    node --check "$repo/frontend/tests/email-signin-bootstrap-deployed.spec.js" && echo "dry run: bootstrap spec parses (frontend/node_modules absent; not loaded by Playwright)"
  fi
  echo "dry run: steps (bootstrap) register, alias, key, verify-address, computer; (run) I0 B1 M1 V0 V1 V2 S1 G1 F1 F2 F3"
  echo "dry run: no network calls made"
  exit 0
fi

# ---- bootstrap ----
if [[ "$stage" != done ]]; then
  if [[ -n "$stage" ]]; then
    echo "state file holds a half-finished bootstrap (stage $stage): its passkey lived only in the browser that registered it." >&2
    echo "move $state aside and run again; a new address is chosen." >&2
    exit 2
  fi
  mkdir -p "$(dirname "$state")"
  chmod 700 "$(dirname "$state")"
  # A numbered TEST address (10,000,000..99,999,999) on the public receive
  # policy: it needs no sender trust (the plus-code trusted-workflow policy
  # needs an aligned authserv-id that is not configured). Reserved here; the
  # spec binds it to the account between register/begin and register/finish,
  # before the auto-assign reconciler can give the account a user number.
  node_b="${E2E_NODE_B_SSH:-node-b}"
  address="$(ssh -o ConnectTimeout=15 -o BatchMode=yes "$node_b" 'maildctl reserve-test-number' \
    | python3 -c 'import json,sys;print(json.load(sys.stdin)["address"])')" \
    || { echo "FAIL could not reserve a test number on $node_b"; exit 1; }
  [[ "$address" =~ ^[0-9]{8}@choir\.news$ ]] || { echo "FAIL reserve-test-number answered an unexpected address shape"; exit 1; }
  echo "bootstrap: reserved a test number; registering the test account (address and ids go to the state file only)"
  set +e
  (cd "$repo/frontend" && E2E_ADDRESS="$address" E2E_STATE_FILE="$state" BASE_URL="$backend" \
    npx playwright test tests/email-signin-bootstrap-deployed.spec.js --project=chromium --workers=1 --reporter=list) \
    >"$out/logs/bootstrap.log" 2>&1
  rc=$?
  set -e
  address=""
  if [[ $rc != 0 ]]; then
    echo "FAIL bootstrap (log $out/logs/bootstrap.log):"
    tail -25 "$out/logs/bootstrap.log" | sed -E 's/[0-9]{8}/<8-digit>/g'
    exit 1
  fi
  stage="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1])).get("stage",""))' "$state")"
  [[ "$stage" == done ]] || { echo "FAIL bootstrap ended at stage $stage"; exit 1; }
  echo "PASS BOOT: test account registered, aliased, keyed, address verified, computer ready"
else
  echo "bootstrap: skipped (state file says done)"
fi
[[ $bootstrap_only == 1 ]] && exit 0

# ---- the run ----
set +e
(cd "$repo/cmd/desktop" && CHOIR_E2E_EMAIL_SIGNIN_STATE="$state" CHOIR_E2E_BACKEND="$backend" CHOIR_E2E_OUT="$out" \
  go test -v -run '^TestE2EEmailSignInStaging$' -count=1 -timeout 40m .) >"$out/logs/run.log" 2>&1
rc=$?
set -e
grep -E '^(PASS|FAIL) [A-Z][0-9]+:' "$out/logs/run.log" || true
if [[ ! -s "$out/steps.jsonl" ]]; then
  echo "FAIL run produced no steps (log $out/logs/run.log):"
  grep -v '^ld: warning' "$out/logs/run.log" | tail -15
  exit 1
fi

python3 - "$out" "$backend" "$rc" "$state" <<'PY'
import json, sys
out, backend, rc, state = sys.argv[1:5]
steps = [json.loads(l) for l in open(out + "/steps.jsonl") if l.strip()]
st = json.load(open(state))
fails = sum(1 for s in steps if not s["ok"])
json.dump({"backend": backend, "computer_id": st.get("computer_id", ""),
           "fails": fails, "go_test_exit": int(rc), "steps": steps},
          open(out + "/acceptance.json", "w"), indent=2)
PY
echo "artifact: $out/acceptance.json"
fails="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["fails"])' "$out/acceptance.json")"
[[ "$fails" == 0 && $rc == 0 ]]
