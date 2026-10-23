#!/usr/bin/env bash
# Nix evaluation check of the host's Firecracker pin
# (docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md).
#
# Evaluates the go-choir-b host config only (no build) and asserts:
#   - pkgs.firecracker is the expected version (EXPECT_FIRECRACKER, default
#     1.17.0, the pin for every computer);
#   - vmctl's VM_FIRECRACKER_BIN and VM_JAILER_BIN come from that one
#     package (the VMM and its jailer move together);
#   - no second ("next") build is configured unless it is newer than the
#     pinned one;
#   - VM_DATA_DISCARD on (1, true, on or yes, any case: the set vmctl's
#     envFlagOn accepts) always comes with a non-empty VM_DATA_DISCARD_ONLY
#     (empty means every computer; the fleet-wide value is an explicit "*");
#   - every host assertion holds (choirFirecrackerBelow among them).
# Prints the pinned store path. With EXPECT_OUT set, the path must equal it
# (FP0: the path already built on Node B, so the deploy builds nothing).
# Needs nix, jq and a sort that supports -V (GNU coreutils; Node B and CI have
# it). Test: scripts/check-host-firecracker-pin-test.sh (a fake nix).
set -euo pipefail

cd "$(dirname "$0")/.."
host='.#nixosConfigurations.go-choir-b'
want="${EXPECT_FIRECRACKER:-1.17.0}"

fail() { echo "FAIL: $*" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq is required"

version="$(nix eval --raw "$host.pkgs.firecracker.version")"
out="$(nix eval --raw "$host.pkgs.firecracker.outPath")"
[ "$version" = "$want" ] || fail "host Firecracker is $version, expected $want"
[ -z "${EXPECT_OUT:-}" ] || [ "$out" = "$EXPECT_OUT" ] || fail "pinned store path is $out, expected $EXPECT_OUT"

env_json="$(nix eval --json "$host.config.systemd.services.go-choir-vmctl.serviceConfig.Environment")"
envval() { jq -r --arg k "$1=" '[.[] | select(startswith($k)) | ltrimstr($k)] | if length == 0 then "<unset>" else .[0] end' <<<"$env_json"; }

[ "$(envval VM_FIRECRACKER_BIN)" = "$out/bin/firecracker" ] || fail "VM_FIRECRACKER_BIN is $(envval VM_FIRECRACKER_BIN), not $out/bin/firecracker"
[ "$(envval VM_JAILER_BIN)" = "$out/bin/jailer" ] || fail "VM_JAILER_BIN is $(envval VM_JAILER_BIN), not $out/bin/jailer"

next="$(envval VM_FIRECRACKER_NEXT_BIN)"
if [ "$next" != "<unset>" ]; then
  next_version="${next%/bin/firecracker}"
  next_version="${next_version##*-firecracker-}"
  # sort -V is GNU coreutils (version order).
  newest="$(printf '%s\n%s\n' "$version" "$next_version" | sort -V | tail -n1)"
  [ "$next_version" != "$version" ] && [ "$newest" = "$next_version" ] \
    || fail "next build $next_version is not newer than the pinned $version"
fi

# The same truthy set as envFlagOn (internal/vmmanager/config.go): trimmed,
# case-insensitive 1, true, on, yes.
discard_flag="$(envval VM_DATA_DISCARD | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
case "$discard_flag" in 1 | true | on | yes)
  only="$(envval VM_DATA_DISCARD_ONLY)"
  [ "$only" != "<unset>" ] && [ -n "${only//[[:space:],]/}" ] \
    || fail "VM_DATA_DISCARD=$(envval VM_DATA_DISCARD) with an empty VM_DATA_DISCARD_ONLY (use \"*\" for every computer)"
  ;;
esac

failed="$(nix eval --json "$host.config.assertions" --apply 'as: map (a: a.message) (builtins.filter (a: !a.assertion) as)')"
[ "$failed" = "[]" ] || fail "host assertions fail: $failed"

echo "ok: host Firecracker $version at $out (VMM and jailer), next=${next}, discard_only=$(envval VM_DATA_DISCARD_ONLY)"
