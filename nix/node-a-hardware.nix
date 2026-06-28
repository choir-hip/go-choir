# Hardware configuration for disposable go-choir Node A design lab.
{ config, lib, pkgs, modulesPath, ... }:
{
  imports = [ (modulesPath + "/installer/scan/not-detected.nix") ];

  boot.initrd.availableKernelModules = [ "ahci" "nvme" "usb_storage" "usbhid" ];
  boot.kernelModules = [ "kvm-intel" ];
  boot.swraid.enable = true;
  boot.swraid.mdadmConf = "MAILADDR root";

  nixpkgs.hostPlatform = lib.mkDefault "x86_64-linux";
  hardware.cpu.intel.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
}
