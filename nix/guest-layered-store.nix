# Layered guest store, prototype (docs/design/layered-guest-image-2026-10-31.md).
#
# Imported only by the layered guest variants in flake.nix (the
# guest-image-layered package and its base probe). The deployed guest
# (guest-image, nixosConfigurations.go-choir-autoputer-vm) does not import it
# and its derivation is unchanged by this file.
#
# The guest's /nix/store is a read-only overlay of two erofs drives:
#   - nix-base: the slow closure (nixpkgs, kernel modules, chromium,
#     claude-code, libreoffice, the frontend's locked deps), keyed by the base
#     probe's derivation and reused across deploys;
#   - nix-app: the rest of this commit's closure (the go-choir binaries, the
#     NixOS system glue that names them), rebuilt each deploy.
# The two are disjoint by construction (app = closure(real) - closure(probe)),
# so lower order only matters for a store path name the builder put in both,
# which it never does. No upperdir: the store stays read-only as today. (A
# tool layer may repeat a path the base also carries: the same store path,
# the same bytes, so the order of those lowers does not matter either.)
#
# Tool layers (docs/design/guest-tool-layers-2026-10-31.md): one more erofs
# drive per name in nix/guest-tools.nix, label nix-tool-<name>, mounted at
# /nix/.ro-tool-<name> and appended to the store overlay's lowers. vmctl
# attaches EVERY slot: the tool's layer for a computer whose profile has it,
# else that slot's empty placeholder (same label, no store paths), so the
# mount set never depends on the profile. A short device timeout turns a
# missing drive into a fast boot failure instead of a long wait.
{ lib, guestToolLayers ? true, ... }:
let
  toolNames = if guestToolLayers then (import ./guest-tools.nix).names else [ ];
  toolMount = name: "/nix/.ro-tool-${name}";
  toolFileSystems = lib.listToAttrs (map (name: lib.nameValuePair (toolMount name) {
    device = "/dev/disk/by-label/nix-tool-${name}";
    fsType = "erofs";
    options = [ "ro" "x-systemd.after=systemd-modules-load.service" "x-systemd.device-timeout=20s" ];
    neededForBoot = true;
    noCheck = true;
  }) toolNames);
in
{
  fileSystems = toolFileSystems // {
  "/nix/.ro-base" = {
    device = "/dev/disk/by-label/nix-base";
    fsType = "erofs";
    options = [ "ro" "x-systemd.after=systemd-modules-load.service" ];
    neededForBoot = true;
    noCheck = true;
  };
  "/nix/.ro-app" = {
    device = "/dev/disk/by-label/nix-app";
    fsType = "erofs";
    options = [ "ro" "x-systemd.after=systemd-modules-load.service" ];
    neededForBoot = true;
    noCheck = true;
  };
  # Replaces microvm.nix's single-label erofs mount of /nix/store.
  "/nix/store" = lib.mkForce {
    overlay.lowerdir = [ "/nix/.ro-app" "/nix/.ro-base" ] ++ map toolMount toolNames;
    neededForBoot = true;
  };
  };
  boot.initrd.availableKernelModules = [ "overlay" "erofs" ];
}
