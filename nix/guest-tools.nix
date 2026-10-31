# Guest tool layers (docs/design/guest-tool-layers-2026-10-31.md).
#
# The heavy tools every guest carries, grouped into layers. One file names
# them for both sides, so the guest (nix/autoputer-vm.nix) and the layer
# builder (flake.nix) use the same package instances:
#   - names: the layer slots, sorted. Their order is the Firecracker drive
#     order after the base and the overlay lower order after app and base.
#     nix/guest-layered-store.nix mounts one /nix/.ro-tool-<name> per name.
#   - forPkgs { pkgs, choirGo, stub }: the tool packages, the roots of each
#     layer, the floor and the exec-logged binaries. stub = true replaces
#     every tool with an empty package (the core probe of the base layer).
#
# A layer image is closure(roots) - closure(floor), built from nixpkgs only,
# so its store hash (its id on the host) moves with nixpkgs or this file,
# never with the guest config or Go code. The floor only trims the layers:
# the base keeps every path the core probe needs, so a floor package the
# core lacks costs bytes in the base, never a missing path.
{
  names = [ "browser" "delegate" "office" "toolchain" ];

  forPkgs = { pkgs, choirGo, stub ? false }:
    let
      pick = name: p:
        if stub then pkgs.runCommand "${name}-tool-stub" { } "mkdir -p $out/bin" else p;
      packages = {
        # visual capture (docs/design/visual-evidence-slice-2026-10-15.md):
        # full Chromium, on no PATH; only a granted computer's capture job
        # gets its path.
        chromium = pick "chromium" pkgs.chromium;
        # claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md):
        # pinned Claude Code, on no PATH; only the allowlisted computer's
        # claude-mode capsule broker runs it.
        claudeCode = pick "claude-code" pkgs.claude-code;
        # No Go caller (2026-10-31 inventory); exec-logged to measure use.
        libreoffice = pick "libreoffice" pkgs.libreoffice;
        # Engineering capsule builds (go build / go test, cgo through gcc).
        gcc = pick "gcc" pkgs.gcc;
        binutils = pick "binutils" pkgs.binutils;
        go = pick "go" choirGo;
      };
    in
    {
      inherit packages;
      layers = {
        browser = [ packages.chromium ];
        delegate = [ packages.claudeCode ];
        office = [ packages.libreoffice ];
        toolchain = [ packages.gcc packages.binutils packages.go ];
      };
      # Packages every guest's core carries (PATH and systemPackages in
      # nix/autoputer-vm.nix, and NixOS itself).
      floor = with pkgs; [ glibc bash coreutils systemd util-linux curl git nodejs perl python3 openssl zlib ];
      # The binaries the exec log wraps, per layer: the office suite's whole
      # bin directory, and the toolchain's entry points. Browser and delegate
      # are called by absolute path and have their own grant logs.
      # bins = null wraps every executable in the package's bin directory.
      logged = [
        { layer = "office"; package = packages.libreoffice; bins = null; }
        { layer = "toolchain"; package = packages.go; bins = [ "go" ]; }
        { layer = "toolchain"; package = packages.gcc; bins = [ "gcc" "cc" "g++" "c++" ]; }
      ];
    };
}
