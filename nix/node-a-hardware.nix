# Generic hardware configuration for Node A.
#
# Node A's physical hardware is not yet fixed. This config uses NixOS
# hardware scanning (not-detected.nix) so the config builds on any
# x86_64-linux machine. When Node A is provisioned on specific hardware,
# replace this with a hardware-specific config like nix/hardware.nix.
{ config, lib, pkgs, modulesPath, ... }:
{
  imports = [ (modulesPath + "/installer/scan/not-detected.nix") ];

  # Generic x86_64 kernel modules for common hardware.
  boot.initrd.availableKernelModules = [
    "ahci"
    "nvme"
    "sd_mod"
    "xhci_pci"
    "usb_storage"
    "usbhid"
  ];

  nixpkgs.hostPlatform = lib.mkDefault "x86_64-linux";
}
