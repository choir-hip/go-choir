# Node B disk/filesystem configuration (UUID-based, stable across reboots)
# Root btrfs on md RAID1, UUID 3b71f2a6-7820-47a1-ba22-c44c65e31ea1
# Adapted from choiros-rs nix/hosts/ovh-node-b-disks.nix
{ ... }:
{
  fileSystems."/" = {
    device = "/dev/disk/by-uuid/3b71f2a6-7820-47a1-ba22-c44c65e31ea1";
    fsType = "btrfs";
    options = [ "subvol=@" "compress=zstd" "noatime" ];
  };

  fileSystems."/data" = {
    device = "/dev/disk/by-uuid/3b71f2a6-7820-47a1-ba22-c44c65e31ea1";
    fsType = "btrfs";
    options = [ "subvol=@data" "compress=zstd" "noatime" ];
  };

  fileSystems."/boot" = {
    device = "/dev/disk/by-uuid/a20c8956-eb6c-4a04-ad90-c384f6089a5e";
    fsType = "ext4";
  };

  fileSystems."/boot/efi" = {
    device = "/dev/disk/by-uuid/F9E9-6E35";
    fsType = "vfat";
    options = [ "umask=0077" ];
  };

  fileSystems."/swap" = {
    device = "/dev/disk/by-uuid/3b71f2a6-7820-47a1-ba22-c44c65e31ea1";
    fsType = "btrfs";
    options = [ "subvol=@swap" "noatime" ];
  };

  # 4 GiB, was 16 GiB: it holds under 200 MiB with 33 GiB of RAM
  # (docs/problems/node-b-storage-clustering-assessment-2026-11-11.md A3).
  # NixOS creates the file through mkswap-swap-swapfile.service, which
  # restartIfChanged=false and which only the swap unit pulls in: the resize
  # happens at the next boot (the old file is removed and recreated while swap
  # is not yet active), not at activation. Rollback: revert, or size = 16384.
  swapDevices = [{
    device = "/swap/swapfile";
    size = 4 * 1024;
  }];
}
