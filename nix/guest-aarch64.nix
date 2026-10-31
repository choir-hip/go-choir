# aarch64 guest for the Mac, booted by Apple Virtualization.framework
# (VZLinuxBootLoader: an uncompressed arm64 kernel Image, an initrd and a
# command line; raw images as virtio-blk; virtio console; virtio-net on the
# framework's NAT; no vsock-less assumptions beyond what the framework offers).
#
# Imported only by the aarch64 guest variants in flake.nix (aarch64Guest). The
# x86_64 Firecracker guest does not import it, and nix/autoputer-vm.nix is
# shared unchanged: this file only adds or overrides what differs on the Mac.
#
# What differs from the Firecracker guest:
#   - Console. Virtualization.framework gives a Linux guest a virtio console
#     (hvc0), not a 16550 on ttyS0. It is on the kernel command line; the
#     stock kernel builds virtio_console, virtio_pci, virtio_blk and virtio_net
#     in (checked in the built kernel config), so the first kernel messages
#     and a failed store mount are visible.
#   - Network. The framework's NAT answers DHCP and offers no point-to-point
#     /30, so a guest booted without the host's ip= parameter takes DHCP
#     (priority 20, so an ip= boot still gets its static network from the
#     runtime unit's 10-vm-runtime.network).
#   - Devices are PCI, not MMIO. microvm.nix already loads virtio_pci and
#     virtio_mmio in the initrd; nothing here is MMIO-only.
{ config, lib, pkgs, ... }:
{
  boot.kernelParams = [ "console=hvc0" ];

  # pdfplumber's test inputs (pandas-stubs, pyreadstat, narwhals, duckdb, ...)
  # are in its derivation, so building pdfplumber on aarch64 compiled duckdb
  # from source (no aarch64 cache hit at the pinned nixpkgs; 760 C++ units).
  # None of them is a runtime dependency: nixpkgs' own pdfplumber lists them
  # under nativeCheckInputs. Skip the check here. The guest's python still has
  # the same runtime closure; the x86_64 guest keeps its cached, checked build.
  nixpkgs.overlays = [
    (final: prev: {
      pythonPackagesExtensions = prev.pythonPackagesExtensions ++ [
        (pyFinal: pyPrev: {
          pdfplumber = pyPrev.pdfplumber.overridePythonAttrs (_: {
            doCheck = false;
            nativeCheckInputs = [ ];
          });
        })
      ];
    })
  ];

  # microvm.nix builds rootfs.ext4 with libguestfs, which exists only for
  # x86. The disk is never read by the guest (the root is a tmpfs, the store
  # is the erofs layers); the host still attaches it first, so the data drive
  # keeps its /dev/vdb position, and releases still carry the file. A GPT disk
  # with an empty FAT ESP of the same shape, 16 MiB, stands in for it.
  microvm.bootDisk = lib.mkForce (pkgs.runCommand "microvm-bootdisk.img" {
    nativeBuildInputs = [ pkgs.buildPackages.gptfdisk pkgs.buildPackages.dosfstools ];
  } ''
    truncate -s 16M $out
    sgdisk --new=1:2048:0 --typecode=1:EF00 --change-name=1:ESP $out >/dev/null
    truncate -s 14M esp.img
    mkfs.vfat -n ESP esp.img >/dev/null
    dd if=esp.img of=$out bs=512 seek=2048 conv=notrunc status=none
  '');

  # With no ip= on the command line the Firecracker guest's network unit has
  # nothing to configure and exits 1; skip it instead of failing the boot.
  systemd.services.go-choir-configure-network.unitConfig.ConditionKernelCommandLine = "ip";

  systemd.network.networks."20-vm-dhcp" = {
    matchConfig.Driver = "virtio_net";
    networkConfig = {
      DHCP = "ipv4";
      IPv6AcceptRA = false;
      LinkLocalAddressing = "no";
    };
  };
}
