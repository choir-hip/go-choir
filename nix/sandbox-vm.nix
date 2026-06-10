# Sandbox guest NixOS config for Firecracker microVMs on Node B.
#
# This module defines the guest VM configuration using the upstream
# microvm.nix module (https://github.com/microvm-nix/microvm.nix).
# The upstream module handles kernel building, initrd generation,
# rootfs image creation, and Firecracker runner script generation.
#
# Key design choices (aligned with choiros-rs approach):
#   - Uses upstream microvm.nix (not the fork) for stability
#   - virtio-blk for data volumes (storeDiskInterface = "blk")
#   - erofs for the shared nix store disk (automatic when shares = [])
#   - systemd as init (proper NixOS boot instead of custom init script)
#   - Go control plane (vmmanager/vmctl) manages VM lifecycle externally
#
# Guest contains ONLY the sandbox runtime binary — no provider credentials,
# no auth signing keys, no gateway secrets (VAL-VM-011).
#
# The vmmanager package (internal/vmmanager) launches Firecracker with
# the kernel, rootfs, and store disk from the microvm runner outputs.
# It does NOT use the microvm runner scripts directly because vmmanager
# needs per-VM networking, port assignment, and lifecycle control.
{ config, lib, pkgs, goChoirPackages, sourceRepoRemote ? "https://github.com/choir-hip/go-choir.git", buildCommit ? "local", includePlaywright ? false, ... }:

let
  playwrightCli = pkgs.writeShellApplication {
    name = "playwright";
    runtimeInputs = [ pkgs.nodejs ];
    text = ''
      export PLAYWRIGHT_BROWSERS_PATH="''${PLAYWRIGHT_BROWSERS_PATH:-${pkgs.playwright-driver.browsers}}"
      export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD="''${PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD:-1}"
      exec ${pkgs.nodejs}/bin/node ${pkgs.playwright}/cli.js "$@"
    '';
  };

  playwrightCoreCli = pkgs.writeShellApplication {
    name = "playwright-core";
    runtimeInputs = [ pkgs.nodejs ];
    text = ''
      export PLAYWRIGHT_BROWSERS_PATH="''${PLAYWRIGHT_BROWSERS_PATH:-${pkgs.playwright-driver.browsers}}"
      export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD="''${PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD:-1}"
      exec ${pkgs.nodejs}/bin/node ${pkgs.playwright}/cli.js "$@"
    '';
  };

  playwrightNodeModules = pkgs.runCommand "go-choir-playwright-node-modules" { } ''
    mkdir -p "$out/lib/node_modules"
    ln -s ${pkgs.playwright} "$out/lib/node_modules/playwright-core"
    ln -s ${pkgs.playwright} "$out/lib/node_modules/playwright"
  '';

  playwrightTools = pkgs.symlinkJoin {
    name = "go-choir-playwright-tools";
    paths = [
      playwrightCli
      playwrightCoreCli
      playwrightNodeModules
    ];
  };

  documentPython = pkgs.python3.withPackages (ps: with ps; [
    beautifulsoup4
    ebooklib
    lxml
    pdfplumber
    pypdf
    python-docx
  ]);

  sandboxRuntimeInstall = pkgs.writeShellScript "go-choir-install-sandbox-runtime" ''
    set -euo pipefail

    env_file="/run/go-choir-sandbox.env"
    if [ -f "$env_file" ]; then
      set -a
      # shellcheck disable=SC1090
      . "$env_file"
      set +a
    fi

    runtime_url="''${RUNTIME_VMCTL_URL:-}"
    runtime_root="/mnt/persistent/runtime"
    current="$runtime_root/sandbox"
    previous="$runtime_root/sandbox-previous"
    next="$runtime_root/.sandbox-next"

    if [ -z "$runtime_url" ]; then
      echo "go-choir-sandbox: RUNTIME_VMCTL_URL is unavailable; using baked sandbox runtime" >&2
      exit 0
    fi

    mkdir -p "$runtime_root"
    rm -rf "$next"
    mkdir -p "$next"

    if ${pkgs.curl}/bin/curl -fsS --retry 3 --retry-delay 1 --retry-all-errors \
      -H "X-Internal-Caller: true" \
      "$runtime_url/internal/vmctl/runtime-package/sandbox" |
      ${pkgs.gnutar}/bin/tar -x -C "$next"; then
      if [ ! -x "$next/bin/sandbox" ]; then
        echo "go-choir-sandbox: downloaded runtime package lacks bin/sandbox; keeping current runtime" >&2
        rm -rf "$next"
        exit 0
      fi
      rm -rf "$previous"
      if [ -e "$current" ]; then
        mv "$current" "$previous"
      fi
      mv "$next" "$current"
      echo "go-choir-sandbox: installed host-provided sandbox runtime package"
    else
      echo "go-choir-sandbox: runtime package download failed; keeping current or baked runtime" >&2
      rm -rf "$next"
    fi
  '';

  sandboxRuntimeExec = pkgs.writeShellScript "go-choir-run-sandbox-runtime" ''
    set -euo pipefail

    dynamic="/mnt/persistent/runtime/sandbox/bin/sandbox"
    if [ -x "$dynamic" ]; then
      if [ -f /mnt/persistent/runtime/sandbox/choir-runtime.env ]; then
        set -a
        # shellcheck disable=SC1091
        . /mnt/persistent/runtime/sandbox/choir-runtime.env
        set +a
      fi
      export RUNTIME_SKILLS_ROOT="/mnt/persistent/runtime/sandbox/share/go-choir/skills"
      exec "$dynamic" "$@"
    fi

    export RUNTIME_SKILLS_ROOT="${goChoirPackages.sandbox}/share/go-choir/skills"
    exec ${goChoirPackages.sandbox}/bin/sandbox "$@"
  '';
in
{
  networking.hostName = if includePlaywright then "go-choir-playwright-worker" else "go-choir-sandbox";

  # ── microvm configuration ────────────────────────────────────────────
  microvm = {
    # Firecracker as the hypervisor. The actual hypervisor binary is not
    # used from the microvm runner — vmmanager launches firecracker directly.
    # But this tells microvm.nix to generate Firecracker-compatible artifacts.
    hypervisor = "firecracker";

    # Guest resources (overridden by vmmanager at launch time via
    # Firecracker config, but used for the build-time artifact generation).
    vcpu = 2;
    mem = 512;

    # No tap interfaces defined here — vmmanager creates per-VM tap
    # devices and networking at runtime. The guest uses DHCP or
    # kernel ip= parameter for network config.
    interfaces = [];

    # Mutable sandbox state on a virtio-blk volume (/dev/vdb).
    # vmmanager creates the actual data.img per-VM at runtime from
    # the VM state directory. This declaration tells microvm.nix to
    # include virtio-blk support in the guest kernel/initrd.
    volumes = [{
      image = "data.img";
      mountPoint = "/mnt/persistent";
      size = 2048;
    }];

    # Use upstream microvm.nix API for the nix store disk.
    # erofs provides a shared nix store that can be shared
    # across VMs with KSM deduplication on the host.
    storeOnDisk = true;
    storeDiskType = "erofs";
    # Favor deploy-loop speed over maximum image compaction. microvm.nix's
    # default EROFS flags include fragments/dedupe on newer kernels, which
    # force the single-threaded mkfs.erofs path; keeping only fast LZ4 lets the
    # builder use the multithread-capable tool. CI lets Node B build these guest
    # images, including selected ordinary/playwright roots in parallel, so its
    # persistent Nix store absorbs the cost instead of rebuilding and copying
    # large image outputs from each ephemeral runner.
    storeDiskErofsFlags = [ "-zlz4" ];

    # No virtiofs or 9p shares. With shares = [], microvm.nix
    # automatically generates an erofs disk for the nix store closure.
    # This is more efficient than virtiofs for our use case because:
    # - No virtiofsd daemon needed on the host
    # - erofs disk is a single shared file referenced by all VMs
    # - Combined with KSM (shared=off), identical pages are deduplicated
    shares = [];
  };

  # ── Guest services ───────────────────────────────────────────────────

  # Configure the guest network from the vmmanager-provided kernel ip=
  # parameter before systemd-networkd starts. We cannot rely on DHCP here:
  # vmmanager owns the host tap, but it does not run a DHCP server.
  systemd.services.go-choir-configure-network = {
    description = "Configure guest network from kernel cmdline";
    wantedBy = [ "sysinit.target" ];
    before = [ "systemd-networkd.service" ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
    };
    script = ''
      set -euo pipefail
      client_ip=""
      gateway_ip=""

      for param in $(cat /proc/cmdline); do
        case "$param" in
          ip=*)
            value="''${param#ip=}"
            IFS=':' read -r client_ip _ gateway_ip _ _ _ _ <<EOF
$value
EOF
            ;;
        esac
      done

      if [ -z "$client_ip" ] || [ -z "$gateway_ip" ]; then
        echo "go-choir-configure-network: missing ip= kernel param" >&2
        exit 1
      fi

      mkdir -p /run/systemd/network
      cat > /run/systemd/network/10-vm-runtime.network <<EOF
[Match]
Driver=virtio_net

[Network]
Address=$client_ip/30
Gateway=$gateway_ip
DHCP=no
IPv6AcceptRA=no
LinkLocalAddressing=no
EOF
    '';
  };

  # Extract per-VM bootstrap settings into an env file before the sandbox
  # service starts. Runtime parameters come from kernel cmdline, while the
  # gateway token is read from the persistent data volume vmmanager owns.
  systemd.services.go-choir-extract-cmdline = {
    description = "Extract go-choir secrets from kernel cmdline";
    wantedBy = [ "multi-user.target" ];
    before = [ "go-choir-sandbox.service" ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
    };
    script = ''
      set -euo pipefail
      ENV_FILE="/run/go-choir-sandbox.env"
      : > "$ENV_FILE"

      # Parse kernel cmdline parameters from vmmanager.
      for param in $(cat /proc/cmdline); do
        case "$param" in
          guest_port=*)
            echo "SANDBOX_PORT=''${param#guest_port=}" >> "$ENV_FILE"
            ;;
          vm_id=*)
            echo "SANDBOX_ID=''${param#vm_id=}" >> "$ENV_FILE"
            ;;
          epoch=*)
            echo "VM_EPOCH=''${param#epoch=}" >> "$ENV_FILE"
            ;;
          choir.gateway_url=*)
            echo "RUNTIME_GATEWAY_URL=''${param#choir.gateway_url=}" >> "$ENV_FILE"
            ;;
          choir.vmctl_url=*)
            echo "RUNTIME_VMCTL_URL=''${param#choir.vmctl_url=}" >> "$ENV_FILE"
            ;;
          choir.maild_url=*)
            echo "RUNTIME_MAILD_URL=''${param#choir.maild_url=}" >> "$ENV_FILE"
            ;;
          choir.wire_publish_url=*)
            echo "RUNTIME_WIRE_PUBLISH_URL=''${param#choir.wire_publish_url=}" >> "$ENV_FILE"
            ;;
          choir.source_service_url=*)
            echo "SOURCE_SERVICE_BASE_URL=''${param#choir.source_service_url=}" >> "$ENV_FILE"
            ;;
          choir.source_service_runtime_url=*)
            echo "SOURCE_SERVICE_RUNTIME_BASE_URL=''${param#choir.source_service_runtime_url=}" >> "$ENV_FILE"
            ;;
          choir.source_service_runtime_owner_id=*)
            echo "SOURCE_SERVICE_RUNTIME_OWNER_ID=''${param#choir.source_service_runtime_owner_id=}" >> "$ENV_FILE"
            ;;
          choir.computer_kind=*)
            echo "CHOIR_COMPUTER_KIND=''${param#choir.computer_kind=}" >> "$ENV_FILE"
            ;;
          choir.owner_id=*)
            echo "CHOIR_OWNER_ID=''${param#choir.owner_id=}" >> "$ENV_FILE"
            ;;
          choir.desktop_id=*)
            echo "CHOIR_DESKTOP_ID=''${param#choir.desktop_id=}" >> "$ENV_FILE"
            ;;
          choir.worker_id=*)
            echo "CHOIR_WORKER_ID=''${param#choir.worker_id=}" >> "$ENV_FILE"
            ;;
          choir.candidate_id=*)
            echo "CHOIR_CANDIDATE_ID=''${param#choir.candidate_id=}" >> "$ENV_FILE"
            ;;
          choir.gateway_token=*)
            echo "RUNTIME_GATEWAY_TOKEN=''${param#choir.gateway_token=}" >> "$ENV_FILE"
            ;;
        esac
      done

      # Older vmmanager bootstraps had vmctl/gateway tap URLs before maild was
      # added. Derive the same tap host on port 8087 so refreshed active
      # computers converge Email draft persistence without a manual VM restart.
      if ! grep -q '^RUNTIME_MAILD_URL=' "$ENV_FILE"; then
        vmctl_url="$(sed -n 's/^RUNTIME_VMCTL_URL=//p' "$ENV_FILE" | tail -n1)"
        if [ -n "$vmctl_url" ]; then
          printf 'RUNTIME_MAILD_URL=%s\n' "$(printf '%s' "$vmctl_url" | sed 's/:8083$/:8087/')" >> "$ENV_FILE"
        fi
      fi

      if ! grep -q '^RUNTIME_WIRE_PUBLISH_URL=' "$ENV_FILE"; then
        vmctl_url="$(sed -n 's/^RUNTIME_VMCTL_URL=//p' "$ENV_FILE" | tail -n1)"
        if [ -n "$vmctl_url" ]; then
          printf 'RUNTIME_WIRE_PUBLISH_URL=%s\n' "$(printf '%s' "$vmctl_url" | sed 's/:8083$/:8082/')" >> "$ENV_FILE"
        fi
      fi

      # Back-compat fallback for older bootstraps that wrote only the host-side
      # token file expectation. Kernel cmdline now provides first-boot truth.
      if [ -f /mnt/persistent/gateway-token ]; then
        printf 'RUNTIME_GATEWAY_TOKEN=%s\n' "$(cat /mnt/persistent/gateway-token)" >> "$ENV_FILE"
      fi

      chmod 0640 "$ENV_FILE"
    '';
  };

  # Sandbox runtime service.
  # Runs the Go sandbox binary which listens for runtime API requests
  # inside the VM. Provider credentials are never in the guest (VAL-VM-011).
  # LLM calls route through the host-side gateway using the extracted token.
  systemd.services.go-choir-sandbox = {
    description = "go-choir Sandbox Runtime (VM guest)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-extract-cmdline.service" ];
    wants = [ "network-online.target" ];
    requires = [ "go-choir-extract-cmdline.service" ];
    environment = {
      # Direct Go exec paths (for example, git-backed export tools) do not
      # run through an interactive shell. Give the sandbox service an explicit
      # guest PATH so tool implementations and shell tools see the same basic
      # runtime utilities.
      PATH = lib.mkForce (lib.makeBinPath ((with pkgs; [
        bash
        coreutils
        findutils
        curl
        gcc
        git
        go
        gnumake
        gnugrep
        gnused
        icu
        nodejs
        perl
        pkg-config
        gnutar
        systemd
        procps
        iproute2
        documentPython
        libreoffice
        pandoc
        poppler-utils
        goChoirPackages.obscura
        goChoirPackages.zot
      ]) ++ lib.optionals includePlaywright (with pkgs; [
        playwrightTools
      ])));
      # VM health checks and host forwarding reach the guest via its tap IP,
      # so the sandbox must listen on all guest interfaces, not loopback only.
      SERVER_HOST = "0.0.0.0";
      # Default port; overridden by guest_port= in kernel cmdline.
      SANDBOX_PORT = "8085";
      SANDBOX_ID = "sandbox-guest";
      # Persistent state directory on the virtio-blk data volume.
      RUNTIME_STORE_PATH = "/mnt/persistent/state";
      # Files app data must use the same persistent data volume. Without this
      # the sandbox falls back to its process-local default, which can disappear
      # across guest reboot/recovery even when runtime DB state survives.
      SANDBOX_FILES_ROOT = "/mnt/persistent/files";
      # Dolt/go-mysql-server transitively requires ICU headers for CGO builds.
      # Worker verification commands are plain `go test`; surface the Nix ICU
      # dev output through standard compiler/pkg-config environment variables.
      CGO_CFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_CXXFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_LDFLAGS = "-L${pkgs.icu}/lib";
      PKG_CONFIG_PATH = "${pkgs.icu.dev}/lib/pkgconfig";
      RUNTIME_SKILLS_ROOT = "${goChoirPackages.sandbox}/share/go-choir/skills";
      RUNTIME_WORKER_REPO_REMOTE = sourceRepoRemote;
      RUNTIME_WORKER_REPO_BASE_SHA = buildCommit;
      RUNTIME_PROMOTION_SOURCE_REPO = sourceRepoRemote;
      RUNTIME_PROMOTION_WORKSPACE_ROOT = "/mnt/persistent/promotion-workspaces";
      # Guest health is part of staging acceptance. Stamp the source revision
      # into the VM runtime so refreshed active computers can prove which guest
      # image they are serving, even though they do not mount host deploy.env.
      CHOIR_DEPLOYED_COMMIT = buildCommit;
      # Worker candidate repos need non-interactive commits for export proof.
      GIT_AUTHOR_NAME = "Choir Worker";
      GIT_AUTHOR_EMAIL = "worker@choir.local";
      GIT_COMMITTER_NAME = "Choir Worker";
      GIT_COMMITTER_EMAIL = "worker@choir.local";
      # Keep Go build/module caches on the writable data volume. The shared
      # Nix store is intentionally read-only inside guest worker VMs.
      GOPATH = "/mnt/persistent/go";
      GOMODCACHE = "/mnt/persistent/go/pkg/mod";
      GOCACHE = "/mnt/persistent/go-build-cache";
      GOTOOLCHAIN = "local";
      # Worker VMs use Obscura as their lightweight VM-local browser,
      # extraction, and bounded-control substrate. Heavy Chrome/Playwright
      # browser bundles stay out of ordinary user/candidate VMs. The separate
      # worker-playwright image is the opt-in evidence/verifier exception.
      CHOIR_OBSCURA_BIN = "${goChoirPackages.obscura}/bin/obscura";
      OBSCURA_BIN = "${goChoirPackages.obscura}/bin/obscura";
      CHOIR_ZOT_PATH = "${goChoirPackages.zot}/bin/zot";
      # Explicit runtime-selected model. Provider credentials remain host-side;
      # guest LLM calls route through the gateway token above.
      RUNTIME_LLM_PROVIDER = "fireworks";
      RUNTIME_LLM_MODEL = "accounts/fireworks/models/deepseek-v4-flash";
      RUNTIME_LLM_REASONING_EFFORT = "low";
    } // lib.optionalAttrs includePlaywright {
      CHOIR_WORKER_BROWSER_CLASS = "playwright";
      CHOIR_PLAYWRIGHT_BIN = "${playwrightTools}/bin/playwright";
      NODE_PATH = "${playwrightTools}/lib/node_modules";
      PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
      PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";
      PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
    };
    serviceConfig = {
      ExecStartPre = "${sandboxRuntimeInstall}";
      ExecStart = "${sandboxRuntimeExec}";
      Restart = "on-failure";
      RestartSec = 1;
      # App adoption builds run as sandbox child processes. If a child build
      # exceeds the guest memory envelope, keep the runtime alive so it can
      # persist a blocked verifier result instead of losing terminal evidence.
      OOMPolicy = "continue";
      StandardOutput = "journal+console";
      StandardError = "journal+console";
      EnvironmentFile = [ "-/run/go-choir-sandbox.env" ];
    };
  };

  # Allow sandbox port through firewall
  networking.firewall.allowedTCPPorts = [ 8085 ];

  # ── Networking ───────────────────────────────────────────────────────
  # Use systemd-networkd for interface bring-up. The actual per-VM static
  # address is generated at boot by go-choir-configure-network from the
  # vmmanager-provided kernel ip= parameter.
  networking.useDHCP = false;
  systemd.network.enable = true;

  # ── System packages ──────────────────────────────────────────────────
  # Minimal set for debugging and runtime support.
  environment.systemPackages = (with pkgs; [
    coreutils
    curl
    gnutar
    gcc
    git
    go
    gnumake
    gnugrep
    gnused
    icu
    nodejs
    perl
    pkg-config
    procps
    iproute2
    documentPython
    libreoffice
    pandoc
    poppler-utils
    bash
  ]) ++ lib.optionals includePlaywright (with pkgs; [
    playwrightTools
  ]);

  system.stateVersion = "25.11";
}
