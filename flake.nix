{
  description = "go-choir: Distributed Multiagent Operating System";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    # Upstream microvm.nix for building NixOS guest VM images.
    # Used to generate the Firecracker-compatible kernel, initrd, rootfs,
    # and erofs store disk for autoputer VMs. The Go control plane
    # (vmmanager/vmctl) launches Firecracker with these artifacts.
    # Not using the fork — upstream is stable and well-maintained.
    microvm = {
      url = "github:microvm-nix/microvm.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    # sbomnix for CycloneDX SBOM generation from Nix flakes.
    # Generates a Software Bill of Materials for each package, listing
    # every dependency and transitive input. This is the machine-readable
    # inventory of choir_code in the equation computer = choir_code(artifact_program).
    # Path to FlakeBOM (Determinate Systems) when we switch to Determinate Nix
    # for enterprise sales — the CycloneDX format is compatible.
    sbomnix = {
      url = "github:tiiuae/sbomnix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    # Kernel-only nixpkgs (docs/problems/host-kernel-216-days-unpatched-
    # 2026-10-15.md). Used ONLY for boot.kernelPackages on the host
    # (go-choir-b) and the guest (go-choir-autoputer-vm), so a kernel
    # security update never drags the rest of the system along. Same
    # lineage as the base: main nixpkgs is a frozen nixos-unstable snapshot
    # (NixOS 26.11 pre-release, 2026-07-01), and this input is the current
    # nixos-unstable, so the kernel package matches the NixOS modules that
    # consume it (a 26.05 release-branch kernel needed an attribute shim).
    # Channel revisions are Hydra-built, so the kernel is a cache hit, not a
    # compile on Node B. Update it alone (the kernel half of the patch
    # cadence) with
    #   nix flake update nixpkgs-kernel
    # The full base bump belongs to the patch pipeline's weekly lane.
    nixpkgs-kernel.url = "github:NixOS/nixpkgs/nixos-unstable";
    # Qdrant-only nixpkgs (docs/problems/weekly-nixpkgs-bump-qdrant-build-
    # failure-2026-10-19.md). Pinned to the nixpkgs revision that production
    # runs, and deliberately NOT updated by the weekly bump (which moves only
    # `nixpkgs`). Used ONLY for pkgs.qdrant on the host, and only while the
    # base nixpkgs builds rustc against an LLVM older than 22 (see
    # choirQdrantModule). Delete this input when that module retires.
    nixpkgs-qdrant.url = "github:NixOS/nixpkgs/ec84054698e3875e23d6057a10283eb9dfa41f1b";
  };

  outputs = { self, nixpkgs, nixpkgs-kernel, nixpkgs-qdrant, microvm, sbomnix, ... }:
    let
      # Packages are x86_64-linux only (deployment target)
      system = "x86_64-linux";
      pkgs = importPkgs system;

      # One kernel line for host and guest, from the kernel-only input above.
      # Pinned to the 6.18 LTS series, not linuxPackages, whose default can
      # move to the next LTS: a series change (and its guest kernel config
      # review) is a deliberate edit here. Security floor 6.18.57: the
      # guest-reachable virtio-net checksum fix "ipv4: validate checksum_start
      # before completing checksum" (6.18.57; docs/problems/kernel-pin-behind-
      # guest-reachable-checksum-fix-2026-10-22.md), Vghost (6.18.45),
      # CVE-2026-53362 and Januscape (6.18.38), CVE-2026-53266 (6.18.36),
      # Copy Fail (6.18.22). The assertion keeps a lock regression from
      # silently shipping an older kernel. The floor lives in one file,
      # nix/kernel-security-floor, which scripts/node_b_host_reboot.py also
      # reads as its --min-kernel default. Evaluation fails unless the file is
      # exactly MAJOR.MINOR.PATCH: versionAtLeast against "", " 6.18.57",
      # "v6.18.57" or "6.18" is true, which would silently drop the floor.
      choirKernelSecurityFloor =
        let floor = nixpkgs.lib.fileContents ./nix/kernel-security-floor;
        in if builtins.match "[0-9]+\\.[0-9]+\\.[0-9]+" floor != null then floor
        else throw "nix/kernel-security-floor must be exactly MAJOR.MINOR.PATCH (one line), got \"${floor}\"";
      choirKernelPackages = (import nixpkgs-kernel { inherit system; }).linuxPackages_6_18;
      choirKernelModule = { lib, ... }: {
        boot.kernelPackages = choirKernelPackages;
        assertions = [{
          assertion = lib.versionAtLeast choirKernelPackages.kernel.version choirKernelSecurityFloor;
          message = "choir kernel ${choirKernelPackages.kernel.version} is below the ${choirKernelSecurityFloor} security floor (nix/kernel-security-floor; docs/problems/kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md)";
        }];
      };

      # Go toolchain lane (docs/reports/patch-lanes-plan-2026-10-15.md,
      # slice 3). Go point releases carry only security and critical fixes,
      # and they are the cheapest patch in the inventory: 1.26.9 closed 22
      # reachable standard-library govulncheck findings that the base's
      # 1.26.4 ships; 1.26.10 closes GO-2026-6809 (crypto/x509), reachable
      # (docs/problems/patch-watch-silent-and-open-security-findings-2026-10-26.md). The base's go_1_26 recipe with a newer version and
      # source hash (the same recipe nixos-unstable uses for 1.26.8, which
      # differs only in those two fields). It applies only while the given
      # nixpkgs is older, so the weekly base bump retires it. One compiler
      # everywhere: every Choir Go build, the dev shells, the host's and the
      # guest's `go`, and CI (go.mod's toolchain directive, ci.yml's pins).
      # Next point release: bump version + hash here, `toolchain` in go.mod,
      # and the go-version pins in .github/workflows/ci.yml.
      choirGoRelease = {
        version = "1.26.10";
        hash = "sha256-/WPog7ovK/ypPBZGsx6+9j4rHk0rMXyXidl2po2B828=";
      };
      mkChoirGo = p:
        if p.lib.versionAtLeast p.go_1_26.version choirGoRelease.version then p.go_1_26
        else p.go_1_26.overrideAttrs (_: {
          inherit (choirGoRelease) version;
          src = p.fetchurl {
            url = "https://go.dev/dl/go${choirGoRelease.version}.src.tar.gz";
            inherit (choirGoRelease) hash;
          };
        });
      choirGo = mkChoirGo pkgs;
      buildChoirGoModule = pkgs.buildGoModule.override { go = choirGo; };

      # Dolt lane (docs/reports/dolt-upgrade-compat-2026-10-15.md). The sql-server
      # for Store A and Store B, the host `dolt` CLI and the dev shell all use
      # pkgs.dolt; the locked nixpkgs ships 2.1.9. Same pattern as choirGo: the
      # base's own recipe with a newer version, source hash and vendor hash
      # (taken from nixpkgs' pkgs/by-name/do/dolt/package.nix at 2.4.2), applied
      # only while the given nixpkgs is older, so the weekly base bump retires
      # it. The embedded Go module in go.mod is a separate form of Dolt and is
      # pinned there.
      choirDoltRelease = {
        version = "2.4.2";
        hash = "sha256-4/VvjwV7N99lraNnc2vJpW17lIe5+1yShudGsZ2r39w=";
        vendorHash = "sha256-wJqA2gWhwEfHQim3a4HJpDsSj1Igeiyrg6xn8pPLrA4=";
      };
      mkChoirDolt = p:
        if p.lib.versionAtLeast p.dolt.version choirDoltRelease.version then p.dolt
        else p.dolt.overrideAttrs (_: {
          inherit (choirDoltRelease) version vendorHash;
          src = p.fetchFromGitHub {
            owner = "dolthub";
            repo = "dolt";
            tag = "v${choirDoltRelease.version}";
            inherit (choirDoltRelease) hash;
          };
        });
      choirDoltModule = { ... }: {
        nixpkgs.overlays = [ (final: prev: { dolt = mkChoirDolt prev; }) ];
      };

      # Firecracker lane. The host's VMM and its jailer are both
      # pkgs.firecracker (nix/node-b.nix VM_FIRECRACKER_BIN, VM_JAILER_BIN),
      # and the jailed VMM is its static build, pkgs.pkgsStatic.firecracker
      # (VM_JAILER_FIRECRACKER_BIN);
      # the locked nixpkgs ships 1.16.1. The host pins 1.17.0 for every
      # computer (docs/problems/firecracker-1-17-for-every-computer-
      # 2026-10-22.md): it carries every 1.16.2 fix (docs/problems/
      # firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md) and adds
      # opt-in virtio-blk discard for the data drive (RES-16). Every
      # version change voids every hibernate snapshot (vmctl refuses them as
      # firecracker_version_mismatch and cold boots). (Hotplug is not
      # affected: every version since 1.15.1 puts its region at 512 GiB,
      # past the E-2236's 39-bit limit, docs/problems/memory-hotplug-cannot-
      # work-on-node-b-39-bit-cpu-2026-10-22.md.) Same pattern as
      # choirDolt: the base's recipe with a newer version, source hash and
      # vendor hash, applied only while the given nixpkgs is older.
      #
      # The hashes are the ones the per-computer next build used (NET-08's
      # trial in D3, every computer in D4; nix/firecracker-next.nix, removed
      # with this pin): the source from
      # `nix flake prefetch github:firecracker-microvm/firecracker/v1.17.0`
      # on the Mac; the vendored crates from the hash mismatch of
      # fetchCargoVendor's fixed-output staging step on Node B's builder
      # with this flake's own package set (FP0, 2026-10-22). This recipe
      # evaluates to the same store path FP0 built on Node B for that
      # build (/nix/store/jbzvr6c965zyby7ldj73yi0zvibgd65b-firecracker-1.17.0),
      # so the computers it launched keep the same binary and banner.
      # Compute a vendor hash with the flake's pkgs on Linux, never with a
      # local nixpkgs (the Mac's first value differed).
      choirFirecrackerRelease = {
        version = "1.17.0";
        hash = "sha256-1HL13XL8CvwGTcm8uIJxgW31hUXXLZ8F2+jV7FEgxEY=";
        cargoHash = "sha256-rqKOD/4cqOLAOIRAdHvxurMB7JPQktWb6fr9KTs84RI=";
      };
      mkChoirFirecracker = p:
        if p.lib.versionAtLeast p.firecracker.version choirFirecrackerRelease.version then p.firecracker
        else p.firecracker.overrideAttrs (finalAttrs: _: {
          inherit (choirFirecrackerRelease) version;
          src = p.fetchFromGitHub {
            owner = "firecracker-microvm";
            repo = "firecracker";
            tag = "v${choirFirecrackerRelease.version}";
            inherit (choirFirecrackerRelease) hash;
          };
          cargoDeps = p.rustPlatform.fetchCargoVendor {
            inherit (finalAttrs) src;
            name = "firecracker-${choirFirecrackerRelease.version}-vendor";
            hash = choirFirecrackerRelease.cargoHash;
          };
        });
      # Upper bound: mkChoirFirecracker takes any nixpkgs firecracker at or
      # above 1.17.0, so a base bump that brings 1.18 would ship it host-wide
      # with no review. Every version change voids every hibernate snapshot;
      # a new minor version is staged per computer first (a fresh
      # nix/firecracker-next.nix and VM_FIRECRACKER_NEXT_BIN/_ONLY,
      # internal/vmmanager/vmm_next.go). Raise the bound here, deliberately,
      # when the host itself moves.
      choirFirecrackerBelow = "1.18";
      choirFirecrackerModule = { lib, pkgs, ... }: {
        nixpkgs.overlays = [ (final: prev: { firecracker = mkChoirFirecracker prev; }) ];
        # The jailer's exec file is the same pin built static
        # (pkgs.pkgsStatic.firecracker: this overlay reaches pkgsStatic, so
        # it is mkChoirFirecracker of the musl package set; nix/node-b.nix
        # VM_JAILER_FIRECRACKER_BIN). The jail holds no loader, so the
        # glibc build cannot exec there (docs/problems/jailed-firecracker-
        # cannot-exec-dynamic-binary-2026-10-23.md). vmctl refuses a jailed
        # exec file of another version at start; this catches it at eval.
        assertions = [{
          assertion = pkgs.pkgsStatic.firecracker.version == pkgs.firecracker.version;
          message = "the static Firecracker for jailed launches is ${pkgs.pkgsStatic.firecracker.version}, the host Firecracker is ${pkgs.firecracker.version}: both come from mkChoirFirecracker and must match (vmctl refuses the jailer otherwise).";
        } {
          assertion = lib.versionOlder pkgs.firecracker.version choirFirecrackerBelow;
          message = "host Firecracker ${pkgs.firecracker.version} is not below ${choirFirecrackerBelow}: a Firecracker version change voids every hibernate snapshot (firecracker_version_mismatch, every computer cold boots), and a new minor version is staged per computer first (internal/vmmanager/vmm_next.go; docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md). Keep the host on 1.17.x or raise choirFirecrackerBelow in flake.nix deliberately.";
        }];
      };

      # Qdrant lane (docs/problems/weekly-nixpkgs-bump-qdrant-build-failure-
      # 2026-10-19.md). nixos-unstable's rustc 1.98 is linked against LLVM 21
      # but its stdarch declares the LLVM 22 shape of
      # llvm.x86.avx512.vpdpbusd.512, so qdrant's `quantization` crate fails
      # to compile ("intrinsic signature mismatch"). Until the base's rustc
      # uses LLVM 22 or newer, take qdrant (same 1.18.2, the deployed build,
      # a binary-cache hit) from the pinned `nixpkgs-qdrant` input. The
      # condition flips by itself on a later weekly bump; then remove the
      # input and this module.
      choirQdrantModule = { ... }: {
        nixpkgs.overlays = [
          (final: prev: {
            qdrant =
              if prev.lib.versionAtLeast prev.rustc.llvm.version "22"
              then prev.qdrant
              else nixpkgs-qdrant.legacyPackages.${system}.qdrant;
          })
        ];
      };

      # Go module version from go.mod
      goModuleVersion = "0.1.0";
      buildCommit = self.rev or self.dirtyRev or "local";
      buildDate = self.lastModifiedDate or "unknown";
      sourceRepoRemote = "https://github.com/choir-hip/go-choir.git";
      # No x86_64-darwin: nixpkgs 26.11 throws on import for that system
      # (docs/problems/flakehub-publish-fails-after-nixpkgs-bump-2026-10-19.md),
      # and `nix flake show --all-systems` (flakehub-push) evaluates every
      # system named here. Re-add it only if the base moves to a nixpkgs
      # that supports x86_64-darwin again.
      devSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forDevSystems = nixpkgs.lib.genAttrs devSystems;
      # nixpkgs tracks nixos-unstable; the locked revision ships Go >= 1.26.2,
      # which the Dolt v2 graph (driver/v2 v2.2.0 go.mod directive) requires.
      importPkgs = system:
        import nixpkgs { inherit system; };
      mkDevShell = devSystem:
        let
          devPkgs = importPkgs devSystem;
        in
        devPkgs.mkShell {
          packages = [
            devPkgs.git
            (mkChoirGo devPkgs)
            devPkgs.pkg-config
            devPkgs.icu
            (mkChoirDolt devPkgs)
          ];
          shellHook = ''
            export PKG_CONFIG_PATH="${devPkgs.icu.dev}/lib/pkgconfig:${devPkgs.icu}/lib/pkgconfig''${PKG_CONFIG_PATH:+:''${PKG_CONFIG_PATH}}"
            export LD_LIBRARY_PATH="${devPkgs.icu}/lib''${LD_LIBRARY_PATH:+:''${LD_LIBRARY_PATH}}"
            export CGO_CFLAGS="$(pkg-config --cflags icu-i18n icu-uc 2>/dev/null) ''${CGO_CFLAGS:-}"
            export CGO_CXXFLAGS="$(pkg-config --cflags icu-i18n icu-uc 2>/dev/null) ''${CGO_CXXFLAGS:-}"
            export CGO_LDFLAGS="$(pkg-config --libs icu-i18n icu-uc 2>/dev/null) ''${CGO_LDFLAGS:-}"
          '';
        };

      rootPath = toString ./.;
      relPath = path:
        let
          full = toString path;
          prefix = rootPath + "/";
        in
          if pkgs.lib.hasPrefix prefix full then pkgs.lib.removePrefix prefix full else full;

      # Common buildGoModule args for all Go services
      commonGoArgs = {
        vendorHash = "sha256-VcX33AW1t9TO6vPVPKqmZqYmpPgy0bEGc/YI+VAhVrk=";
        nativeBuildInputs = [ pkgs.pkg-config ];
        buildInputs = [ pkgs.icu ];
        ldflags = [
          "-X github.com/yusefmosiah/go-choir/internal/buildinfo.Version=${goModuleVersion}"
          "-X github.com/yusefmosiah/go-choir/internal/buildinfo.Commit=${buildCommit}"
          "-X github.com/yusefmosiah/go-choir/internal/buildinfo.BuiltAt=${buildDate}"
        ];
        doCheck = false; # Tests run separately in CI
      };

      # Frontend package — built Svelte SPA via buildNpmPackage.
      # Local development uses pnpm (pnpm-lock.yaml); the Nix build uses npm
      # with a checked-in package-lock.json for reproducibility in the autoputer.
      # npmDepsHash was computed with `nix run nixpkgs#prefetch-npm-deps --
      # frontend/package-lock.json`. If dependencies change, re-run the
      # prefetch command (or set npmDepsHash to "" and read the correct hash
      # from the first Nix build error, just like Go's vendorHash).
      frontendPkg = pkgs.buildNpmPackage {
        pname = "go-choir-frontend";
        version = goModuleVersion;
        src = pkgs.lib.cleanSourceWith {
          src = ./frontend;
          filter = path: type:
            let
              base = baseNameOf path;
            in
            if type == "directory" then
              base != "node_modules" && base != "test-results" && base != ".cache"
            else
              (pkgs.lib.hasSuffix ".js" path) ||
              (pkgs.lib.hasSuffix ".mjs" path) ||
              (pkgs.lib.hasSuffix ".ts" path) ||
              (pkgs.lib.hasSuffix ".svelte" path) ||
              (pkgs.lib.hasSuffix ".css" path) ||
              (pkgs.lib.hasSuffix ".html" path) ||
              base == "package.json" ||
              base == "package-lock.json" ||
              base == "choir-package.json" ||
              base == "svelte.config.js" ||
              base == "vite.config.js";
        };
        npmDepsHash = "sha256-oA1Qf3x5Ff2kHBLJwhcVvy/d0n5GWlOXrXbue/HJ5UQ=";
        npmBuildScript = "build";
        VITE_CHOIR_BUILD_VERSION = goModuleVersion;
        VITE_CHOIR_BUILD_SHA = buildCommit;
        VITE_CHOIR_BUILD_TIME = buildDate;
        # Playwright downloads browsers during postinstall, which fails in the
        # Nix sandbox.  We only need it for e2e tests (not the build), so skip.
        PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
        installPhase = ''
          cp -r dist $out
        '';
      };

      # The frontend's locked dependencies alone (node_modules plus the two
      # package files they were built from), for capsule frontend builds
      # without the package registry: a Claude delegate's capsule links them
      # when its package files match (docs/design/delegate-one-round-build-2026-10-21.md).
      # Same lockfile, so the same npmDepsHash as frontendPkg.
      frontendDepsPkg = pkgs.buildNpmPackage {
        pname = "go-choir-frontend-deps";
        version = goModuleVersion;
        src = pkgs.lib.cleanSourceWith {
          src = ./frontend;
          filter = path: type:
            let
              base = baseNameOf path;
            in
            type != "directory" && (base == "package.json" || base == "package-lock.json");
        };
        npmDepsHash = frontendPkg.npmDepsHash;
        dontNpmBuild = true;
        PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
        installPhase = ''
          mkdir -p $out
          cp package.json package-lock.json $out/
          cp -r node_modules $out/node_modules
        '';
      };

      # One Go compile for every service (docs/problems/ci-deploy-latency-per-
      # service-go-builds-2026-10-09.md). Fifteen per-service buildGoModule
      # derivations each compiled the whole dependency graph (embedded Dolt
      # included, ~94 s each on Node B); this derivation builds every cmd with
      # one build cache. mkGoService copies one binary out and keeps the
      # per-service identity the deploy verifies (build.json, skills, frontend).
      goServiceSubPackages = [
        "cmd/auth" "cmd/proxy" "cmd/maild" "cmd/maildctl" "cmd/vmctl"
        "cmd/gateway" "cmd/corpusd" "cmd/checkpointd" "cmd/projection-compact"
        "cmd/platform-dolt-squash"
        "cmd/choir-updater" "cmd/choir-receipt-signer" "cmd/capsule-broker"
        "cmd/autoputer" "cmd/choir-builder" "cmd/goproxy"
        # capsule package mirror (station FP10)
        "cmd/pkgmirror"
        # claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md)
        "cmd/choir-delegate"
        # visual capture (docs/design/visual-evidence-slice-2026-10-15.md)
        "cmd/choir-capture"
        # the desk-session worker without Dolt (RES-17)
        "cmd/desk-worker"
        # operator tools of station FP9 (escrow key sealing; the copy-only
        # tape repack and census); installed beside corpusd (extraBins)
        "cmd/escrowkey" "cmd/taperepack"
      ];
      # Keep source selection structural. Go owns the package dependency graph;
      # repeating it here as per-service internalDirs caused fallback builds to
      # omit new transitive imports that normal Go builds already understood.
      goServicesSrc = pkgs.lib.cleanSourceWith {
        src = ./.;
        filter = path: type:
          let
            full = toString path;
            rel = relPath path;
            isProductionFile = !(pkgs.lib.hasSuffix "_test.go" path);
            inSubPackage = pkgs.lib.any (sp: rel == sp || pkgs.lib.hasPrefix (sp + "/") rel) goServiceSubPackages;
          in
            (type == "directory" && (full == rootPath || rel == "cmd" || inSubPackage || rel == "internal" || pkgs.lib.hasPrefix "internal/" rel)) ||
            (rel == "go.mod") ||
            (rel == "go.sum") ||
            (type != "directory" && inSubPackage && isProductionFile) ||
            (pkgs.lib.hasPrefix "internal/" rel && isProductionFile);
      };
      goServicesAll = buildChoirGoModule (commonGoArgs // {
        pname = "go-choir-services";
        version = goModuleVersion;
        src = goServicesSrc;
        subPackages = goServiceSubPackages;
      });
      skillsSrc = pkgs.lib.cleanSourceWith {
        src = ./.;
        filter = path: type:
          let
            full = toString path;
            rel = relPath path;
          in
            (type == "directory" && (full == rootPath || rel == "skills" || pkgs.lib.hasPrefix "skills/" rel)) ||
            (pkgs.lib.hasInfix "/skills/" path && pkgs.lib.hasSuffix "SKILL.md" path);
      };

      # A service package: its binary from the shared build plus identity.
      mkGoService = { pname, subPackage, includeSkills ? false, includeFrontend ? false, includeGoSum ? false, extraBins ? [ ] }:
        assert pkgs.lib.elem subPackage goServiceSubPackages;
        pkgs.stdenvNoCC.mkDerivation {
          inherit pname;
          version = goModuleVersion;
          dontUnpack = true;
          dontFixup = true;
          installPhase = ''
            mkdir -p $out/bin $out/share/go-choir
            cp ${goServicesAll}/bin/${baseNameOf subPackage} $out/bin/
            ${pkgs.lib.concatMapStrings (b: "cp ${goServicesAll}/bin/${b} $out/bin/\n") extraBins}
            cat > $out/share/go-choir/build.json <<'EOF'
            {"schema_version":1,"artifact":"${pname}","version":"${goModuleVersion}","commit":"${buildCommit}","built_at":"${buildDate}"}
            EOF
            ${pkgs.lib.optionalString includeSkills ''
              mkdir -p $out/share/go-choir/skills
              cp -R ${skillsSrc}/skills/. $out/share/go-choir/skills/
            ''}
            ${pkgs.lib.optionalString includeFrontend ''
              mkdir -p $out/frontend
              cp -R ${frontendPkg}/. $out/frontend/
            ''}
            ${pkgs.lib.optionalString includeGoSum ''
              cp ${./go.sum} $out/share/go-choir/go.sum
            ''}
          '';
        };

      # All packages
      goChoirPackages = {
        auth = mkGoService {
          pname = "auth";
          subPackage = "cmd/auth";
        };
        proxy = mkGoService {
          pname = "proxy";
          subPackage = "cmd/proxy";
        };
        maild = mkGoService {
          pname = "maild";
          subPackage = "cmd/maild";
        };
        maildctl = mkGoService {
          pname = "maildctl";
          subPackage = "cmd/maildctl";
        };
        vmctl = mkGoService {
          pname = "vmctl";
          subPackage = "cmd/vmctl";
        };
        gateway = mkGoService {
          pname = "gateway";
          subPackage = "cmd/gateway";
        };
        corpusd = mkGoService {
          pname = "corpusd";
          subPackage = "cmd/corpusd";
          # FP9's operator tools act on corpusd's escrow key and artifacts
          # root; on Node B they are at
          # /var/lib/go-choir/services/corpusd/bin/{escrowkey,taperepack}.
          extraBins = [ "escrowkey" "taperepack" ];
        };
        checkpointd = mkGoService {
          pname = "checkpointd";
          subPackage = "cmd/checkpointd";
        };
        projectionCompact = mkGoService {
          pname = "projection-compact";
          subPackage = "cmd/projection-compact";
        };
        # Store A weekly history squash (nix/node-b.nix, scripts/platform-dolt-squash.sh).
        platformDoltSquash = mkGoService {
          pname = "platform-dolt-squash";
          subPackage = "cmd/platform-dolt-squash";
        };
        updater = mkGoService {
          pname = "choir-updater";
          subPackage = "cmd/choir-updater";
        };
        receiptSigner = mkGoService {
          pname = "choir-receipt-signer";
          subPackage = "cmd/choir-receipt-signer";
        };
        capsuleBroker = mkGoService {
          pname = "capsule-broker";
          subPackage = "cmd/capsule-broker";
        };
        # claude delegate: the cell-side client of a claude-mode capsule's
        # broker (docs/design/claude-delegate-slice1-2026-10-14.md).
        choirDelegate = mkGoService {
          pname = "choir-delegate";
          subPackage = "cmd/choir-delegate";
        };
        # visual capture: the capsule job that screenshots the release
        # frontend (docs/design/visual-evidence-slice-2026-10-15.md).
        choirCapture = mkGoService {
          pname = "choir-capture";
          subPackage = "cmd/choir-capture";
        };
        autoputer = mkGoService {
          pname = "autoputer";
          subPackage = "cmd/autoputer";
          includeSkills = true;
          includeFrontend = true;
          extraBins = [ "desk-worker" ];
        };
        frontend = frontendPkg;
        frontendDeps = frontendDepsPkg;
        # Host Go module proxy for engineering capsules; carries the deployed
        # go.sum it pre-warms and verifies on start
        # (docs/reports/capsule-go-module-proxy-design-2026-10-14.md).
        goproxy = mkGoService {
          pname = "goproxy";
          subPackage = "cmd/goproxy";
          includeGoSum = true;
        };
        # Host package mirror for engineering capsules (npm, PyPI, crates, git;
        # station FP10, docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md).
        pkgmirror = mkGoService {
          pname = "pkgmirror";
          subPackage = "cmd/pkgmirror";
        };
      };

    in
    let
      # Host builder (S2): produces layered app-layer releases (nar +
      # builder-receipt.json) for tracking-computer pushes. Deployed to
      # Node B as a service pointer; no host systemd unit.
      choirBuilder = mkGoService {
        pname = "choir-builder";
        subPackage = "cmd/choir-builder";
      };

      # ── Guest VM artifacts ──────────────────────────────────────────────
      # The autoputer guest VM is defined as a NixOS configuration using
      # microvm.nix. From it we extract the individual artifacts that
      # vmmanager needs to launch Firecracker:
      #   - vmlinux (kernel)
      #   - boot disk (ext4 root filesystem)
      #   - initrd (for systemd module loading)
      #   - store disk (erofs for the nix store closure)
      #
      # The guest-image package bundles these for deployment. Replace live
      # guest artifacts atomically; running VMs may hold read-only image files
      # open and must not see those files truncated in place.
      #   nix build .#guest-image
      #   install to a temp dir, then mv artifacts into /var/lib/go-choir/guest/
      guestVmConfig = self.nixosConfigurations.go-choir-autoputer-vm.config;

      # The guest NixOS system for a given commit stamp. The deployed guest uses
      # buildCommit; checks.guest-floor-commit-free builds it under two other
      # stamps to prove the floor digest ignores the stamp.
      mkGuestVm = commit: nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit choirGo;
          buildCommit = commit;
          inherit sourceRepoRemote;
        };
        modules = [
          microvm.nixosModules.microvm
          ./nix/autoputer-vm.nix
          choirKernelModule
        ];
      };

      # ── Guest release metadata (docs/design/density-d2-snapshots-survive-
      # deploys-2026-10-26.md 3.1) ───────────────────────────────────────────
      # release.json is built into every guest image. floor_digest covers what
      # a snapshot's memory depends on that engineering cannot change: the
      # guest kernel and initrd, the locked nixpkgs and microvm revisions, the
      # Go toolchain and the floor source fileset (nix/guest-floor-paths.txt,
      # no tests, no testdata). None of it contains the commit stamp, so a
      # stamp-only change leaves the digest alone (checks.x86_64-linux.
      # guest-floor-commit-free). security_epoch, guest_abi and guest_abi_min
      # come from nix/guest-release-policy.json (rules in the sibling .md).
      guestReleasePolicy = builtins.fromJSON (builtins.readFile ./nix/guest-release-policy.json);
      guestFloorPaths =
        let
          lines = pkgs.lib.splitString "\n" (builtins.readFile ./nix/guest-floor-paths.txt);
          stripped = map (l: pkgs.lib.trim (pkgs.lib.head (pkgs.lib.splitString "#" l))) lines;
        in
          map (pkgs.lib.removeSuffix "/") (builtins.filter (l: l != "") stripped);
      guestFloorSrc = pkgs.lib.cleanSourceWith {
        name = "guest-floor-source";
        src = ./.;
        filter = path: type:
          let
            full = toString path;
            rel = relPath path;
            isTestOnly = pkgs.lib.hasSuffix "_test.go" rel || pkgs.lib.hasInfix "/testdata/" ("/" + rel + "/");
            under = e: rel == e || pkgs.lib.hasPrefix (e + "/") rel;
            above = e: type == "directory" && pkgs.lib.hasPrefix (rel + "/") e;
          in
            full == rootPath || (!isTestOnly && pkgs.lib.any (e: under e || above e) guestFloorPaths);
      };
      # The commit-free parts of a guest, as strings (kept apart from the hash
      # so the check can compare them and name the part that moved).
      guestFloorInputs = vmConfig: {
        kernel = "${vmConfig.boot.kernelPackages.kernel}";
        kernelConfig = "${vmConfig.boot.kernelPackages.kernel.configfile}";
        initrd = "${vmConfig.system.build.initialRamdisk}";
        nixpkgs = nixpkgs.rev or nixpkgs.narHash;
        microvm = microvm.rev or microvm.narHash;
        go = choirGo.version;
        floorSource = "${guestFloorSrc}";
      };
      guestFloorDigest = vmConfig:
        "sha256:" + builtins.hashString "sha256" (builtins.toJSON (guestFloorInputs vmConfig));
      guestReleaseJsonFor = commit: vmConfig: builtins.toJSON {
        schema = 1;
        floor_digest = guestFloorDigest vmConfig;
        security_epoch = guestReleasePolicy.security_epoch;
        guest_abi = guestReleasePolicy.guest_abi;
        guest_abi_min = guestReleasePolicy.guest_abi_min;
        build_commit = commit;
      };
      guestReleaseJson = guestReleaseJsonFor buildCommit guestVmConfig;

      mkGuestImage = name: vmConfig:
        let
          # Guest kernel (vmlinux ELF binary for Firecracker).
          guestKernel = vmConfig.boot.kernelPackages.kernel.dev;

          # Guest boot disk (root filesystem image).
          guestBootDisk = vmConfig.microvm.bootDisk;

          # Guest initrd (contains ext4, erofs, virtio modules needed by systemd).
          guestInitrd = vmConfig.system.build.initialRamdisk;

          # Guest store disk (erofs image containing the nix store closure).
          # This is the shared read-only nix store that VMs of this image class
          # reference. Guests share the file's host page cache; KSM is off on
          # the host (SEC-09), so guest memory pages are never merged.
          guestStoreDisk = vmConfig.microvm.storeDisk;
        in pkgs.runCommand name { } ''
        mkdir -p $out
        cp ${guestKernel}/vmlinux $out/vmlinux
        cp ${guestBootDisk} $out/rootfs.ext4
        cp ${guestInitrd}/${vmConfig.system.boot.loader.initrdFile} $out/initrd
        cp ${guestStoreDisk} $out/storedisk.erofs
        cat > $out/build.json <<'EOF'
{"schema_version":1,"artifact":"${name}","version":"${goModuleVersion}","commit":"${buildCommit}","built_at":"${buildDate}"}
EOF
        cat > $out/kernel-params <<'EOF'
${builtins.concatStringsSep " " vmConfig.microvm.kernelParams}
EOF
        cp ${vmConfig.environment.etc."choir-guest-image-manifest".source} $out/guest-image-manifest
        cp ${pkgs.writeText "release.json" (guestReleaseJsonFor buildCommit vmConfig)} $out/release.json
      '';

      # Convenience package that bundles the canonical guest artifacts.
      guest-image = mkGuestImage "go-choir-guest-image" guestVmConfig;

      # Desktop dev shell — for building Choir Desktop with Wails v3.
      # Separate from the default shell so it doesn't pull in ICU/Dolt
      # or interfere with the main Go dev environment.
      mkDesktopShell = devSystem:
        let
          devPkgs = importPkgs devSystem;
        in
        devPkgs.mkShell {
          # Go only: the app embeds no frontend, and cmd/desktop/build.sh
          # compiles cgo with Xcode's clang and SDK, not a Nix toolchain.
          packages = [
            (mkChoirGo devPkgs)
          ];
          shellHook = ''
            echo "Choir Desktop dev shell (Wails v3)"
            echo "  cmd/desktop/build.sh run"
          '';
        };
    in
    {
      devShells = forDevSystems (devSystem: {
        default = mkDevShell devSystem;
        desktop = mkDesktopShell devSystem;
      });

      packages.${system} = goChoirPackages // {
        default = self.packages.${system}.auth;
        choir-builder = choirBuilder;
        # The Go toolchain lane's compiler (the declared Go for Patch Watch).
        choir-go = choirGo;
        # Expose the guest image as a top-level package for easy building:
        #   nix build .#guest-image
        inherit guest-image;
        # The release.json of this commit's guest image, without building the
        # image (the deploy-level floor measurement reads it per commit; the
        # same text is lib.guestReleaseJson for pure evaluation).
        guest-release-json = pkgs.writeText "release.json" guestReleaseJson;
      };

      lib.guestReleaseJson = guestReleaseJson;

      # D2 S1 test: the floor digest does not move on a commit-only change.
      # Evaluates the guest under two different commit stamps; a difference in
      # the kernel, initrd or floor source fails the evaluation.
      checks.${system}.guest-floor-commit-free =
        let
          a = guestFloorInputs (mkGuestVm "0000000000000000000000000000000000000001").config;
          b = guestFloorInputs (mkGuestVm "0000000000000000000000000000000000000002").config;
          moved = builtins.filter (k: a.${k} != b.${k}) (builtins.attrNames a);
        in
          assert pkgs.lib.assertMsg (moved == [ ])
            "the guest floor digest moves with the commit stamp: ${builtins.toJSON moved} differ between two buildCommit values. Take an input that carries the commit out of the digest and record why (docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md 3.1, S1 test 2).";
          pkgs.runCommand "guest-floor-commit-free" { } "echo ${builtins.toJSON a} > $out";

      # ── SBOM outputs ─────────────────────────────────────────────────
      # CycloneDX SBOMs for each Go service package. These are the
      # machine-readable bills of materials for choir_code, listing every
      # dependency and transitive input. Used for auditability, compliance,
      # and the artifact program doctrine's proof of determinism.
      #   nix build .#sbom.auth
      #   nix build .#sbom.proxy
      # The output is a CycloneDX JSON file at $out/sbom.json.
      # When we switch to Determinate Nix + FlakeBOM, the format is compatible.
      sbom.${system} = let
        sbomnixCli = sbomnix.packages.${system}.default;
        mkSbom = name: pkg:
          pkgs.runCommand "sbom-${name}" { nativeBuildInputs = [ sbomnixCli ]; } ''
            mkdir $out
            sbomnix --cdx "$out/sbom.json" "${pkg}"
          '';
      in builtins.mapAttrs mkSbom goChoirPackages;

      # ── Autoputer guest VM NixOS configuration ──────────────────────────
      # This defines the guest VM that runs inside Firecracker on Node B.
      # Uses upstream microvm.nix to build the guest kernel, initrd, rootfs,
      # and erofs store disk. The Go vmmanager launches Firecracker with
      # these artifacts — it does NOT use the microvm runner scripts directly
      # because vmmanager needs per-VM networking, port assignment, and
      # lifecycle control.
      #
      # Key design (aligned with choiros-rs proven approach):
      #   - systemd as init (proper NixOS boot, not custom init script)
      #   - erofs for a shared read-only nix store (one file; no KSM, SEC-09)
      #   - virtio-blk for data volumes (mutable autoputer state)
      #   - No virtiofs/9p shares (simpler, no host daemon needed)
      nixosConfigurations.go-choir-autoputer-vm = mkGuestVm buildCommit;


      # ── Node B host configuration ─────────────────────────────────────
      nixosConfigurations.go-choir-b = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit choirGo;
          inherit buildCommit sourceRepoRemote;
          # Bind the exact immutable guest image into the host system closure;
          # node-b activation advances /var/lib/go-choir/guest atomically.
          guestImage = guest-image;
        };
        modules = [
          ./nix/hardware.nix
          ./nix/disks.nix
          ./nix/node-b.nix
          choirKernelModule
          choirDoltModule
          choirFirecrackerModule
          choirQdrantModule
        ];
      };

    };
}
