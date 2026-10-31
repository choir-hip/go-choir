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
# which it never does. No upperdir: the store stays read-only as today.
{ lib, ... }:
{
  fileSystems."/nix/.ro-base" = {
    device = "/dev/disk/by-label/nix-base";
    fsType = "erofs";
    options = [ "ro" "x-systemd.after=systemd-modules-load.service" ];
    neededForBoot = true;
    noCheck = true;
  };
  fileSystems."/nix/.ro-app" = {
    device = "/dev/disk/by-label/nix-app";
    fsType = "erofs";
    options = [ "ro" "x-systemd.after=systemd-modules-load.service" ];
    neededForBoot = true;
    noCheck = true;
  };
  # Replaces microvm.nix's single-label erofs mount of /nix/store.
  fileSystems."/nix/store" = lib.mkForce {
    overlay.lowerdir = [ "/nix/.ro-app" "/nix/.ro-base" ];
    neededForBoot = true;
  };
  boot.initrd.availableKernelModules = [ "overlay" "erofs" ];
}
