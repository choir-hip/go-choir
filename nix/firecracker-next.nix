# Firecracker 1.17 as a second VMM build, launched only for the computers
# VM_FIRECRACKER_NEXT_ONLY names (internal/vmmanager/vmm_next.go; NET-08 and
# RES-16 in docs/definitions/choir-launch-fp8-image-jailer-2026-10-21.md).
#
# Hashes computed by FP0 on the Mac (2026-10-22): the source with `nix flake
# prefetch github:firecracker-microvm/firecracker/v1.17.0`, the vendored
# crates from the hash mismatch of fetchCargoVendor's fixed-output staging
# step, which is the same on every system. v1.17.0 is the latest 1.17.x tag.
# nix/node-b.nix imports this for the computers VM_FIRECRACKER_NEXT_ONLY
# names.
#
# 1.17.0 (2026-09-10) adds opt-in virtio-blk discard for writable Sync
# drives (CHANGELOG 1.17.0, PR #6142). Check the latest 1.17.x patch release
# before filling the hashes, and drop this file once nixpkgs packages >= 1.17.
{ pkgs }:
pkgs.firecracker.overrideAttrs (finalAttrs: old: {
  version = "1.17.0";
  src = pkgs.fetchFromGitHub {
    owner = "firecracker-microvm";
    repo = "firecracker";
    rev = "v${finalAttrs.version}";
    hash = "sha256-1HL13XL8CvwGTcm8uIJxgW31hUXXLZ8F2+jV7FEgxEY=";
  };
  cargoDeps = pkgs.rustPlatform.fetchCargoVendor {
    inherit (finalAttrs) src;
    name = "firecracker-${finalAttrs.version}-vendor";
    hash = "sha256-TR5hi7iZbZe7BTvqgPT9N2i0WPYT7aVwcSzkCVlPzFk=";
  };
})
