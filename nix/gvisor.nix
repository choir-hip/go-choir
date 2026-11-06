# gVisor's runsc for desk workers (MA6.4, owner rulings R39 and R40;
# internal/yaegikernel/desk_gvisor.go). Pinned to the release Phase 0
# measured (docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md, x86_64;
# docs/evidence/gvisor-arm64-mac-guest-2026-11-04.md, aarch64): the release
# tarball, checked against its published sha512. runsc finds its sidecar
# binaries in gvisor-bin/ beside its own (resolved) path, so both sit in bin/.
# The guest runtime gets runsc's store path as CHOIR_RUNSC.
{ lib, stdenvNoCC, fetchurl }:
let
  version = "20261102.0";
  srcs = {
    x86_64-linux = fetchurl {
      url = "https://storage.googleapis.com/gvisor/releases/release/${version}/x86_64/gvisor.tar.bz2";
      hash = "sha512-LmZxuy0DO4ccWBGrV+FqxXCYiw4AVMUl+RHPrxkbqzyk77COCpjapLnn0kj55Xdu8u8vimuC2ClkakKSTJi7BQ==";
    };
    aarch64-linux = fetchurl {
      url = "https://storage.googleapis.com/gvisor/releases/release/${version}/aarch64/gvisor.tar.bz2";
      hash = "sha512-V0tXdbRBdhj2PIUG5rsjfjcN+FcigypdDDgxOCFV5V5Z1M+pk6Hpu+GxzwQ0b/1Isgqqh5xu2aUsaA92L9GZgA==";
    };
  };
in
stdenvNoCC.mkDerivation {
  pname = "gvisor-runsc";
  inherit version;
  src = srcs.${stdenvNoCC.hostPlatform.system} or (throw "gvisor-runsc: no release for ${stdenvNoCC.hostPlatform.system}");
  sourceRoot = ".";
  dontConfigure = true;
  dontBuild = true;
  # Static Go binaries from the release: nothing to patch or strip.
  dontPatchELF = true;
  dontStrip = true;
  dontFixup = true;
  installPhase = ''
    runHook preInstall
    mkdir -p $out/bin
    install -m 0555 runsc $out/bin/runsc
    cp -r gvisor-bin $out/bin/gvisor-bin
    chmod -R a-w,a+rX $out/bin/gvisor-bin
    runHook postInstall
  '';
  meta = {
    description = "gVisor runsc, the release Choir measured, for desk worker sandboxes";
    homepage = "https://gvisor.dev";
    license = lib.licenses.asl20;
    platforms = [ "x86_64-linux" "aarch64-linux" ];
  };
}
