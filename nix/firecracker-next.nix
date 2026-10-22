# Firecracker 1.17 as a second VMM build, launched only for the computers
# VM_FIRECRACKER_NEXT_ONLY names (internal/vmmanager/vmm_next.go; NET-08 and
# RES-16 in docs/definitions/choir-launch-fp8-image-jailer-2026-10-21.md).
#
# UNEVALUATED. Written in a lane with no Nix builds: both hashes are
# lib.fakeHash, so building this fails and prints the real hash, once for the
# source and once for the vendored crates. Nothing imports this file yet;
# importing it into nix/node-b.nix (the FP0 step) is what builds it.
#
# 1.17.0 (2026-09-10) adds opt-in virtio-blk discard for writable Sync
# drives (CHANGELOG 1.17.0, PR #6142). Check the latest 1.17.x patch release
# before filling the hashes, and drop this file when the host itself moves to
# 1.17 (raise choirFirecrackerBelow in flake.nix). Hotplug is no reason to
# avoid it: every version since 1.15.1 puts the virtio-mem region at 512 GiB,
# past Node B's 39-bit limit (firecracker #6255; docs/problems/memory-hotplug-
# cannot-work-on-node-b-39-bit-cpu-2026-10-22.md).
{ pkgs }:
pkgs.firecracker.overrideAttrs (finalAttrs: old: {
  version = "1.17.0";
  src = pkgs.fetchFromGitHub {
    owner = "firecracker-microvm";
    repo = "firecracker";
    rev = "v${finalAttrs.version}";
    hash = pkgs.lib.fakeHash;
  };
  cargoDeps = pkgs.rustPlatform.fetchCargoVendor {
    inherit (finalAttrs) src;
    name = "firecracker-${finalAttrs.version}-vendor";
    hash = pkgs.lib.fakeHash;
  };
})
