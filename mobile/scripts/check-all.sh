#!/bin/sh
# Every check of the mobile app that needs NO simulator and NO native build: a lane may run it before it pushes. It replaces what the deleted proof-app.sh ran
# before its build (the generators and checks, the wiring greps, the whole Swift package) and adds the shared-code loop. Last line: CHECKS OK, or CHECKS FAILED at <step>.
#   NOSWIFT=1   skip `swift test` (it runs through the Mac build lock, one at a time; a whole run is about two minutes plus the wait)
#   NOBUN=1     skip the Exact loop (it needs EXACT2=<the lead's build tree>/mobile/.exact-src; see CLAUDE.md "Build output lives in one place")
# What this does NOT run, because it needs the lead's build tree: the built-app check (check-no-weights.sh on the built Autoradio.app) and every drive of the app
# (proof-real.sh, queue mode `real`, which builds first). A generator that changes a tracked file fails the check: commit what it wrote.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
M="$HERE/.."
fail() { echo "CHECKS FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }
cd "$M" || fail "cd"

# 1. generators, which must leave the tree as it was (a stale generated file is a failure) ...
python3 -I "$HERE/gen-exact-interface.py" >/dev/null 2>&1 || fail "gen-exact-interface"
python3 -I "$HERE/gen-tetramark.py" --check >/dev/null 2>&1 || fail "tetramark drift" "run scripts/gen-tetramark.py"
[ -z "$(git status --short -- exact-package/exact.json exact-package/package.json 2>/dev/null | grep -v '^??' | head -1)" ] || {
  git diff --stat -- exact-package/exact.json | tail -3; fail "generated files" "a generator changed a tracked file: commit what it wrote (or stash your own edit first: this check cannot tell them apart)"; }
# ... and the checks
python3 -I "$HERE/check-strings.py" >/dev/null 2>&1 || { python3 -I "$HERE/check-strings.py" 2>&1 | tail -10; fail "strings"; }
python3 -I "$HERE/check-no-loading.py" 2>&1 | tail -10; python3 -I "$HERE/check-no-loading.py" >/dev/null 2>&1 || fail "a Loading or Opening string"
python3 -I "$HERE/check-tetramark-projection.py" >/dev/null 2>&1 || fail "tetramark projection bound"
echo "ok: generators, strings, no-loading, tetramark"

# 2. the wiring the contract and the Kit depend on
C="$M/AutoradioLab"
grep -q 'send up = choirTalkCancel()' "$C/spine.contract" || fail "wiring" "micCancel does not call choirTalkCancel (a cancelled turn would be sent)"
grep -q 'choirAsk(text=ask, aim=pg.top)' "$C/spine.contract" || fail "wiring" "a typed ask does not carry the aim"
if grep -rIniE --exclude-dir=target --exclude-dir=node_modules --exclude-dir=.exact 'hold to talk|hold or tap|modeLabel|setLensStyle|lens-[abc]\b' "$C" "$M/exact-package/interface" "$M/AutoradioKit/Sources" >/dev/null 2>&1; then
  fail "wiring" "hold-to-talk wording, RadioInfo.modeLabel or the lens-variant switch is still in the app"
fi
for f in "$M"/themes/*.json; do grep -q '"lens"' "$f" || fail "wiring" "$(basename "$f") has no lens data"; done
echo "ok: cancel, aim, no hold wording, lens is theme data"

# 3. the native boundary only falls
python3 "$M/../scripts/check-autoradio-native-boundary" >/dev/null 2>&1 || { python3 "$M/../scripts/check-autoradio-native-boundary" 2>&1 | tail -4; fail "native boundary"; }
echo "ok: native boundary"

# 4. the Exact loop: contract build, the web host's tests, the whole bun suite
if [ -z "${NOBUN:-}" ]; then
  [ -n "${EXACT2:-}" ] || fail "exact loop" "set EXACT2=<the lead's build tree>/mobile/.exact-src (or NOBUN=1 to skip)"
  sh "$HERE/check-exact-pin.sh" || fail "exact pin" "EXACT2 is not at mobile/exact.pin"
  # bun install rewrites bun.lock; the committed file is restored afterwards, which is only safe when it had no change of its own: refuse to run with one.
  [ -z "$(git status --porcelain -- "$C/bun.lock")" ] || fail "bun.lock" "AutoradioLab/bun.lock has an uncommitted change: commit or revert it, then run again (this check restores it after bun install)"
  ALLOW_NO_MODELS=1 "$HERE/sync-exact-package.sh" >"${TMPDIR:-/tmp}/check-all-sync.txt" 2>&1 || { tail -8 "${TMPDIR:-/tmp}/check-all-sync.txt"; fail "sync-exact-package"; }
  ( cd "$C" && bun install >/dev/null 2>&1 ) || fail "bun install"
  [ -z "$(git status --porcelain -- "$C/bun.lock")" ] || git checkout -q -- "$C/bun.lock" || fail "bun.lock restore"
  ( cd "$C" && bun exact.mjs contract build app.contract >"${TMPDIR:-/tmp}/check-all-contract.txt" 2>&1 ) || { grep -i error "${TMPDIR:-/tmp}/check-all-contract.txt" | head -10; fail "contract build"; }
  ( cd "$C" && bun exact.mjs test web >"${TMPDIR:-/tmp}/check-all-web.txt" 2>&1 ); if grep -qE '[1-9][0-9]* failed' "${TMPDIR:-/tmp}/check-all-web.txt" || ! grep -q 'passed' "${TMPDIR:-/tmp}/check-all-web.txt"; then tail -8 "${TMPDIR:-/tmp}/check-all-web.txt"; fail "exact test web"; fi
  ( cd "$C" && bun test 2>&1 | tail -4 | tee "${TMPDIR:-/tmp}/check-all-bun.txt" | grep -E ' (pass|fail)' ); grep -qE '^ *0 fail' "${TMPDIR:-/tmp}/check-all-bun.txt" || fail "bun test"
  echo "ok: contract build, exact test web, bun test"
fi

# 5. the whole Swift package, through the lock
if [ -z "${NOSWIFT:-}" ]; then
  echo "swift test (the whole package, through the lock)"
  /usr/bin/lockf -k -t 7200 "$HOME/.cache/autoradio-mac-build.lock" swift test --package-path "$M/AutoradioKit" >"${TMPDIR:-/tmp}/check-all-swift.txt" 2>&1 \
    || { grep -aE "error: -\[" "${TMPDIR:-/tmp}/check-all-swift.txt" | sed -E 's/.*error: -\[([^]]*)\].*/\1/' | sort -u | head -20; fail "swift test" "full output: ${TMPDIR:-/tmp}/check-all-swift.txt"; }
  grep -aE "Executed [0-9]+ tests" "${TMPDIR:-/tmp}/check-all-swift.txt" | tail -1
fi
echo "CHECKS OK"
