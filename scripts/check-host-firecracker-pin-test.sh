#!/usr/bin/env bash
# Test of scripts/check-host-firecracker-pin.sh against a fake `nix` (no
# evaluation, no build, no host access). The fake answers the script's four
# nix eval calls from canned values; the host environment list it returns is
# the case under test (docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md,
# review finding 4: the discard guard must recognise the same truthy set as
# envFlagOn in internal/vmmanager/config.go: 1, true, on, yes, any case).
#
# Run: bash scripts/check-host-firecracker-pin-test.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/check-host-firecracker-pin.sh"
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required (a run without it proves nothing)" >&2; exit 1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
OUT=/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-firecracker-1.17.0
mkdir -p "$TMP/bin"
cat >"$TMP/bin/nix" <<'FAKE'
#!/usr/bin/env bash
# Fake nix: version, outPath, the vmctl Environment list, and assertions.
case "$*" in
  *pkgs.firecracker.version*) printf '%s' 1.17.0 ;;
  *pkgs.firecracker.outPath*) printf '%s' "$FAKE_OUT" ;;
  *serviceConfig.Environment*) printf '%s\n' "$FAKE_ENV_JSON" ;;
  *config.assertions*) printf '%s\n' '[]' ;;
  *) echo "fake nix: unexpected call: $*" >&2; exit 99 ;;
esac
FAKE
chmod +x "$TMP/bin/nix"

failures=0
# run_case NAME EXPECT(pass|fail) DISCARD ONLY
run_case() {
  local name="$1" expect="$2" discard="$3" only="$4" env_json status
  env_json="$(jq -nc --arg out "$OUT" --arg d "$discard" --arg o "$only" '
    ["VM_FIRECRACKER_BIN=\($out)/bin/firecracker", "VM_JAILER_BIN=\($out)/bin/jailer"]
    + (if $d == "-" then [] else ["VM_DATA_DISCARD=\($d)"] end)
    + (if $o == "-" then [] else ["VM_DATA_DISCARD_ONLY=\($o)"] end)')"
  PATH="$TMP/bin:$PATH" FAKE_OUT="$OUT" FAKE_ENV_JSON="$env_json" \
    bash "$SCRIPT" >"$TMP/stdout" 2>"$TMP/stderr"
  status=$?
  if { [ "$expect" = pass ] && [ "$status" -eq 0 ]; } || { [ "$expect" != pass ] && [ "$status" -ne 0 ]; }; then
    echo "ok: $name"
  else
    echo "FAIL: $name: expected $expect, exit $status: $(cat "$TMP/stderr" "$TMP/stdout" | head -n 2 | tr '\n' ' ')"
    failures=$((failures + 1))
  fi
}

run_case "discard unset"                         pass -    -
run_case "discard=1, selector *"                 pass 1    '*'
run_case "discard=1, empty selector"             fail 1    ''
run_case "discard=1, selector unset"             fail 1    -
run_case "discard=on, empty selector"            fail on   ''
run_case "discard=ON, selector unset"            fail ON   -
run_case "discard=true, empty selector"          fail true ''
run_case "discard=True, blank selector"          fail True ' , '
run_case "discard=yes, empty selector"           fail yes  ''
run_case "discard= Yes  (padded), no selector"   fail ' Yes ' -
run_case "discard=on, selector *"                pass on   '*'
run_case "discard=0, empty selector"             pass 0    ''
run_case "discard=off, empty selector"           pass off  ''

[ "$failures" -eq 0 ] || { echo "$failures case(s) failed" >&2; exit 1; }
echo "all cases passed"
