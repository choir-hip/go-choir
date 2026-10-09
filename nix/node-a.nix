# Node A: a shared NixOS host (owner 2026-10-09). Choir uses it only as Node
# B's Nix remote builder (./modules/choir-nix-builder-host.nix). The rest of
# the machine is free for another project. The old Choir mirror that served
# choir-ip.com was removed the same day. Handoff notes:
# docs/node-a-shared-host.md.
{ pkgs, ... }:
{
  imports = [ ./modules/choir-nix-builder-host.nix ];

  boot.loader.efi.canTouchEfiVariables = true;
  boot.loader.efi.efiSysMountPoint = "/boot/efi";
  boot.loader.grub = {
    enable = true;
    efiSupport = true;
    devices = [ "nodev" ];
  };

  networking.hostName = "node-a";
  networking.useDHCP = true;
  networking.firewall = {
    enable = true;
    allowedTCPPorts = [ 22 ];
  };

  services.openssh = {
    enable = true;
    openFirewall = true;
    settings = {
      PermitRootLogin = "prohibit-password";
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
    };
  };

  users.users.root.openssh.authorizedKeys.keys = [
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILN3IIn6TzBBExWiJTJ7aDlA/LlEMXvjFlSfkKkV02TZ wiz@choiros-ovh"
  ];

  nix.settings.auto-optimise-store = true;
  nix.gc = {
    automatic = true;
    dates = "weekly";
    options = "--delete-older-than 14d";
  };

  environment.systemPackages = with pkgs; [ git vim curl htop ];

  time.timeZone = "UTC";
  system.stateVersion = "25.11";
}
