{
  description = "go-choir: Distributed Multiagent Operating System";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    # Upstream microvm.nix for building NixOS guest VM images.
    # Used to generate the Firecracker-compatible kernel, initrd, rootfs,
    # and erofs store disk for sandbox VMs. The Go control plane
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
  };

  outputs = { self, nixpkgs, microvm, sbomnix, ... }:
    let
      # Packages are x86_64-linux only (deployment target)
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };

      # Go module version from go.mod
      goModuleVersion = "0.1.0";
      buildCommit = self.rev or self.dirtyRev or "local";
      buildDate = self.lastModifiedDate or "unknown";
      sourceRepoRemote = "https://github.com/choir-hip/go-choir.git";
      devSystems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forDevSystems = nixpkgs.lib.genAttrs devSystems;
      mkDevShell = devSystem:
        let
          devPkgs = import nixpkgs { system = devSystem; };
        in
        devPkgs.mkShell {
          packages = [
            devPkgs.git
            devPkgs.go
            devPkgs.pkg-config
            devPkgs.icu
            devPkgs.dolt
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

      # Keep source selection structural. Go owns the package dependency graph;
      # repeating it here as per-service internalDirs caused fallback builds to
      # omit new transitive imports that normal Go builds already understood.
      goServiceSrc = { subPackage, includeSkills ? false }:
        pkgs.lib.cleanSourceWith {
          src = ./.;
          filter = path: type:
            let
              full = toString path;
              rel = relPath path;
              isProductionFile = !(pkgs.lib.hasSuffix "_test.go" path);
              isRelevantDirectory =
                full == rootPath ||
                rel == "cmd" ||
                rel == subPackage ||
                pkgs.lib.hasPrefix (subPackage + "/") rel ||
                rel == "internal" ||
                pkgs.lib.hasPrefix "internal/" rel ||
                (includeSkills && (rel == "skills" || pkgs.lib.hasPrefix "skills/" rel));
            in
              (type == "directory" && isRelevantDirectory) ||
              (rel == "go.mod") ||
              (rel == "go.sum") ||
              (pkgs.lib.hasPrefix (subPackage + "/") rel && isProductionFile) ||
              (pkgs.lib.hasPrefix "internal/" rel && isProductionFile) ||
              (includeSkills && pkgs.lib.hasInfix "/skills/" path && pkgs.lib.hasSuffix "SKILL.md" path);
        };

      # Common buildGoModule args for all Go services
      commonGoArgs = {
        vendorHash = "sha256-JxOGfaZ3J71NVicFEhn1Vsgy5nOa1Sk74gQ0oroAhLA=";
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
      # with a checked-in package-lock.json for reproducibility in the sandbox.
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
              base == "svelte.config.js" ||
              base == "vite.config.js";
        };
        npmDepsHash = "sha256-1ivvmDrQmaHDTUu38BoEsyajT9TP9xdzie2gGU2DJtA=";
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

      rustyV8Archive = pkgs.fetchurl {
        url = "https://github.com/denoland/rusty_v8/releases/download/v137.3.0/librusty_v8_release_x86_64-unknown-linux-gnu.a.gz";
        hash = "sha256-omgf3lMBir0zZgGPEyYX3VmAAt948VbHvG0v9gi1ZWc=";
      };

      obscuraPkg = pkgs.rustPlatform.buildRustPackage {
        pname = "obscura";
        version = "0.1.0-choir-348a651";
        src = pkgs.fetchFromGitHub {
          owner = "yusefmosiah";
          repo = "obscura";
          rev = "348a651e287ad370546762e78fc2095a7d33dc93";
          hash = "sha256-+h05ieNUbfYCMqIoYuZLXwqhsZPsHHsXtnLzZEUaQMM=";
        };
        cargoHash = "sha256-q6bE+5p1nkxeuPdZ6eoLZ6eb274XPKaQASR9DCx4XD4=";
        nativeBuildInputs = [ pkgs.perl pkgs.pkg-config ];
        RUSTY_V8_ARCHIVE = rustyV8Archive;
        cargoBuildFlags = [ "-p" "obscura-cli" ];
        doCheck = false;
      };

      zotPkg = pkgs.buildGoModule {
        pname = "zot";
        version = "0.2.6";
        src = pkgs.fetchFromGitHub {
          owner = "patriceckhart";
          repo = "zot";
          rev = "v0.2.6";
          hash = "sha256-bWezjuuXh0e600KHxpABnLzA4XHYmef669IXbKLsZfA=";
        };
        vendorHash = "sha256-glcP2rMtb2mJooRhJqctPg18L9KjsJDyREL9WtFmmjE=";
        subPackages = [ "cmd/zot" ];
        ldflags = [
          "-s"
          "-w"
          "-X main.version=0.2.6"
          "-X main.commit=917da8c414e183118e68034e0e8c6f6b746f0132"
          "-X main.date=2026-05-30T17:33:08Z"
        ];
        doCheck = false;
      };

      # Build a single Go service binary
      mkGoService = { pname, subPackage, includeSkills ? false }:
        pkgs.buildGoModule (commonGoArgs // {
          inherit pname;
          version = goModuleVersion;
          src = goServiceSrc { inherit subPackage includeSkills; };
          subPackages = [ subPackage ];
          postInstall = ''
            mkdir -p $out/share/go-choir
            cat > $out/share/go-choir/build.json <<'EOF'
            {"schema_version":1,"artifact":"${pname}","version":"${goModuleVersion}","commit":"${buildCommit}","built_at":"${buildDate}"}
            EOF
            ${pkgs.lib.optionalString includeSkills ''
              mkdir -p $out/share/go-choir/skills
              cp -R skills/. $out/share/go-choir/skills/
            ''}
          '';
        });

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
        };
        sourcecycled = mkGoService {
          pname = "sourcecycled";
          subPackage = "cmd/sourcecycled";
        };
        sandbox = mkGoService {
          pname = "sandbox";
          subPackage = "cmd/sandbox";
          includeSkills = true;
        };
        frontend = frontendPkg;
        obscura = obscuraPkg;
        zot = zotPkg;
      };

    in
    let
      # ── Guest VM artifacts ──────────────────────────────────────────────
      # The sandbox guest VM is defined as a NixOS configuration using
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
      guestVmConfig = self.nixosConfigurations.go-choir-sandbox-vm.config;
      playwrightGuestVmConfig = self.nixosConfigurations.go-choir-sandbox-vm-playwright.config;

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
          # reference. With KSM on the host, identical pages are deduplicated.
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
      '';

      # Convenience packages that bundle guest artifacts together. The ordinary
      # image stays light and Obscura-backed; worker-playwright gets its own
      # image so high-fidelity screenshot/video proof does not inflate every VM.
      guest-image = mkGuestImage "go-choir-guest-image" guestVmConfig;
      guest-image-playwright = mkGuestImage "go-choir-guest-image-playwright" playwrightGuestVmConfig;

      # Desktop dev shell — for building Choir Desktop with Wails v3.
      # Separate from the default shell so it doesn't pull in ICU/Dolt
      # or interfere with the main Go dev environment.
      mkDesktopShell = devSystem:
        let
          devPkgs = import nixpkgs { system = devSystem; };
        in
        devPkgs.mkShell {
          packages = [
            devPkgs.go
            devPkgs.nodejs
            devPkgs.go-task
          ];
          shellHook = ''
            echo "Choir Desktop dev shell (Wails v3)"
            echo "  cd cmd/desktop && task deps && task dev"
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
        # Expose the guest image as a top-level package for easy building:
        #   nix build .#guest-image
        inherit guest-image guest-image-playwright;
      };

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

      # ── Sandbox guest VM NixOS configuration ──────────────────────────
      # This defines the guest VM that runs inside Firecracker on Node B.
      # Uses upstream microvm.nix to build the guest kernel, initrd, rootfs,
      # and erofs store disk. The Go vmmanager launches Firecracker with
      # these artifacts — it does NOT use the microvm runner scripts directly
      # because vmmanager needs per-VM networking, port assignment, and
      # lifecycle control.
      #
      # Key design (aligned with choiros-rs proven approach):
      #   - systemd as init (proper NixOS boot, not custom init script)
      #   - erofs for shared nix store with KSM deduplication
      #   - virtio-blk for data volumes (mutable sandbox state)
      #   - No virtiofs/9p shares (simpler, no host daemon needed)
      nixosConfigurations.go-choir-sandbox-vm = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit buildCommit sourceRepoRemote;
          includePlaywright = false;
        };
        modules = [
          microvm.nixosModules.microvm
          ./nix/sandbox-vm.nix
        ];
      };

      nixosConfigurations.go-choir-sandbox-vm-playwright = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit buildCommit sourceRepoRemote;
          includePlaywright = true;
        };
        modules = [
          microvm.nixosModules.microvm
          ./nix/sandbox-vm.nix
        ];
      };

      # ── Node B host configuration ─────────────────────────────────────
      nixosConfigurations.go-choir-b = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit buildCommit sourceRepoRemote;
          # Pass the guest VM runner artifacts to the host config so
          # the deploy pipeline can install them to /var/lib/go-choir/guest/.
          guestRunner = self.nixosConfigurations.go-choir-sandbox-vm.config.microvm.runner.firecracker;
        };
        modules = [
          ./nix/hardware.nix
          ./nix/disks.nix
          ./nix/node-b.nix
        ];
      };

      # ── Node A host configuration ─────────────────────────────────────
      # Full Choir mirror serving choir-ip.com. Imports node-b.nix for the
      # complete service stack, overrides hostname and Caddy virtualHosts.
      nixosConfigurations.go-choir-a = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        specialArgs = {
          goChoirPackages = goChoirPackages;
          inherit buildCommit sourceRepoRemote;
          guestRunner = self.nixosConfigurations.go-choir-sandbox-vm.config.microvm.runner.firecracker;
        };
        modules = [
          ./nix/node-a-hardware.nix
          ./nix/node-a-disks.nix
          ./nix/node-a.nix
        ];
      };

    };
}
