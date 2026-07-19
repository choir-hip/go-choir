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
{ config, lib, pkgs, goChoirPackages, sourceRepoRemote ? "https://github.com/choir-hip/go-choir.git", buildCommit ? "local", ... }:

let

  documentPython = pkgs.python3.withPackages (ps: with ps; [
    beautifulsoup4
    ebooklib
    lxml
    pdfplumber
    pypdf
    python-docx
  ]);
  guestImageManifest = pkgs.writeText "choir-immutable-guest-image-manifest" ''
    contract=choir-guest-image-v1
    build_commit=${buildCommit}
    sandbox=${goChoirPackages.sandbox}
    updater=${goChoirPackages.updater}
    capsule_broker=${goChoirPackages.capsuleBroker}
    kernel=${config.boot.kernelPackages.kernel}
    kernel_config=${config.boot.kernelPackages.kernel.configfile}
  '';


  sandboxRuntimeExec = pkgs.writeShellScript "go-choir-run-sandbox-runtime" ''
    set -euo pipefail

    if [ -f /run/go-choir-sandbox.env ]; then
      set -a
      # shellcheck disable=SC1091
      . /run/go-choir-sandbox.env
      set +a
    fi

    if [ -z "''${RUNTIME_WIRE_PUBLISH_URL:-}" ]; then
      for param in $(cat /proc/cmdline); do
        case "$param" in
          choir.wire_publish_url=*)
            export RUNTIME_WIRE_PUBLISH_URL="''${param#choir.wire_publish_url=}"
            ;;
          choir.vmctl_url=*)
            vmctl_url="''${param#choir.vmctl_url=}"
            if [ -n "$vmctl_url" ]; then
              export RUNTIME_WIRE_PUBLISH_URL="$(printf '%s' "$vmctl_url" | sed 's/:8083$/:8082/')"
            fi
            ;;
        esac
      done
    fi

    if [ -n "''${RUNTIME_WIRE_PUBLISH_URL:-}" ]; then
      echo "go-choir-sandbox: wire publish URL configured"
    else
      echo "go-choir-sandbox: wire publish URL not configured" >&2
    fi

    current="/mnt/persistent/choir-updater/current"
    dynamic="$current/bin/sandbox"
    if [ -x "$dynamic" ]; then
      export RUNTIME_SKILLS_ROOT="$current/share/go-choir/skills"
      export CHOIR_UPDATER_ROOT="/mnt/persistent/choir-updater"
      exec "$dynamic" "$@"
    fi
    export CHOIR_UPDATER_ROOT="/mnt/persistent/choir-updater"
    export RUNTIME_SKILLS_ROOT="${goChoirPackages.sandbox}/share/go-choir/skills"
    exec ${goChoirPackages.sandbox}/bin/sandbox "$@"
  '';
in
{
  networking.hostName = "go-choir-sandbox";

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

  # Per-realization bootstrap credential disk. vmmanager creates this tiny
  # ext4 device outside kernel argv; the trusted guest core consumes and
  # unlinks its sole mode-0400 envelope before agent runtime starts.
  fileSystems."/run/choir-bootstrap" = {
    device = "/dev/disk/by-label/CHOIR_CRED";
    fsType = "ext4";
    options = [ "rw" "nosuid" "nodev" "noexec" ];
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
    before = [ "go-choir-updater.service" "go-choir-sandbox.service" ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
    };
    script = ''
      set -euo pipefail
      ENV_FILE="/run/go-choir-sandbox.env"
      : > "$ENV_FILE"
      UPDATER_ENV_FILE="/run/go-choir-updater.env"
      : > "$UPDATER_ENV_FILE"

      # Parse kernel cmdline parameters from vmmanager.
      for param in $(cat /proc/cmdline); do
        case "$param" in
          guest_port=*)
            echo "SANDBOX_PORT=''${param#guest_port=}" >> "$ENV_FILE"
            ;;
          vm_id=*)
            echo "SANDBOX_ID=''${param#vm_id=}" >> "$ENV_FILE"
            echo "SANDBOX_ID=''${param#vm_id=}" >> "$UPDATER_ENV_FILE"
            ;;
          choir.computer_id=*)
            echo "CHOIR_COMPUTER_ID=''${param#choir.computer_id=}" >> "$ENV_FILE"
            echo "CHOIR_COMPUTER_ID=''${param#choir.computer_id=}" >> "$UPDATER_ENV_FILE"
            ;;
          choir.realization_id=*)
            echo "CHOIR_REALIZATION_ID=''${param#choir.realization_id=}" >> "$ENV_FILE"
            echo "CHOIR_REALIZATION_ID=''${param#choir.realization_id=}" >> "$UPDATER_ENV_FILE"
            ;;
          epoch=*)
            echo "VM_EPOCH=''${param#epoch=}" >> "$ENV_FILE"
            echo "VM_EPOCH=''${param#epoch=}" >> "$UPDATER_ENV_FILE"
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
            echo "CHOIR_PLATFORM_URL=''${param#choir.wire_publish_url=}" >> "$ENV_FILE"
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


      chmod 0640 "$ENV_FILE"
      chmod 0644 "$UPDATER_ENV_FILE"
    '';
  };

  users.groups.choir-guest-signer = {};
  users.groups.choir-verifier-signer = {};
  users.users.choir-guest-signer = {
    isSystemUser = true;
    group = "choir-guest-signer";
  };
  users.users.choir-verifier-signer = {
    isSystemUser = true;
    group = "choir-verifier-signer";
  };

  systemd.tmpfiles.rules = [
    "d /mnt/persistent/choir-updater 0700 root root -"
    "d /run/choir-updater-control 0700 root root -"
    "d /run/choir-runtime-handoff 0700 root root -"
    "d /run/choir 0700 root root -"
    "d /mnt/persistent/choir-signers 0711 root root -"
    "d /mnt/persistent/choir-signers/guest-core 0700 choir-guest-signer choir-guest-signer -"
    "d /mnt/persistent/choir-signers/verifier 0700 choir-verifier-signer choir-verifier-signer -"
    "d /run/choir-signers 0711 root root -"
    "d /run/choir-signers/guest-core 0750 choir-guest-signer choir-guest-signer -"
    "d /run/choir-signers/verifier 0750 choir-verifier-signer choir-verifier-signer -"
  ];

  # Fixed privileged restart bridge. The updater may create only the trigger;
  # it has no access to PID 1's control sockets or arbitrary unit names.
  systemd.paths.go-choir-sandbox-restart = {
    description = "Watch for a verified Choir restart request";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/restart";
  };

  systemd.services.go-choir-sandbox-restart = {
    description = "Restart only the Choir sandbox service";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-sandbox-restart" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/restart
        install -m 0400 /run/choir-runtime-handoff/restart-capability /run/choir-runtime-handoff/recovery-capability
        exec ${pkgs.systemd}/bin/systemctl restart go-choir-sandbox.service
      '';
    };
  };

  systemd.paths.go-choir-sandbox-recovery = {
    description = "Watch for a verified Choir recovery restart request";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/recover";
  };

  systemd.services.go-choir-sandbox-recovery = {
    description = "Restore the prior Choir release with reserved transient credentials";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-sandbox-recovery" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/recover
        install -m 0400 /run/choir-runtime-handoff/recovery-capability /run/choir-runtime-handoff/restart-capability
        exec ${pkgs.systemd}/bin/systemctl restart go-choir-sandbox.service
      '';
    };
  };

  systemd.paths.go-choir-sandbox-recovery-cleanup = {
    description = "Watch for recovery credential cleanup";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/cleanup";
  };

  systemd.services.go-choir-sandbox-recovery-cleanup = {
    description = "Delete the reserved credential after observed healthy startup";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-sandbox-recovery-cleanup" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/cleanup /run/choir-runtime-handoff/recovery-capability
      '';
    };
  };

  systemd.services.go-choir-guest-receipt-signer = {
    description = "Isolated guest-core receipt signer";
    wantedBy = [ "multi-user.target" ];
    after = [ "go-choir-extract-cmdline.service" ];
    requires = [ "go-choir-extract-cmdline.service" ];
    serviceConfig = {
      Type = "simple";
      User = "choir-guest-signer";
      Group = "choir-guest-signer";
      ExecStart = "${goChoirPackages.receiptSigner}/bin/choir-receipt-signer --mode guest-core --socket /run/choir-signers/guest-core/signer.sock --key /mnt/persistent/choir-signers/guest-core/key.ed25519 --state-root /mnt/persistent/choir-signers/guest-core/receipts";
      EnvironmentFile = [ "-/run/go-choir-updater.env" ];
      Restart = "on-failure";
      RestartSec = 1;
      UMask = "0077";
      NoNewPrivileges = true;
      CapabilityBoundingSet = [ ];
      AmbientCapabilities = [ ];
      PrivatePIDs = true;
      ProtectProc = "invisible";
      ProcSubset = "pid";
      PrivateTmp = true;
      PrivateDevices = true;
      ProtectHome = true;
      ProtectSystem = "strict";
      ProtectControlGroups = true;
      ReadWritePaths = [ "/mnt/persistent/choir-signers/guest-core" "/run/choir-signers/guest-core" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers/verifier" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-verifier-signer = {
    description = "Isolated verifier receipt signer";
    wantedBy = [ "multi-user.target" ];
    after = [ "go-choir-extract-cmdline.service" ];
    requires = [ "go-choir-extract-cmdline.service" ];
    serviceConfig = {
      Type = "simple";
      User = "choir-verifier-signer";
      Group = "choir-verifier-signer";
      ExecStart = "${goChoirPackages.receiptSigner}/bin/choir-receipt-signer --mode verifier-control --socket /run/choir-signers/verifier/signer.sock --key /mnt/persistent/choir-signers/verifier/key.ed25519 --state-root /mnt/persistent/choir-signers/verifier/receipts";
      EnvironmentFile = [ "-/run/go-choir-updater.env" ];
      Restart = "on-failure";
      RestartSec = 1;
      UMask = "0077";
      NoNewPrivileges = true;
      CapabilityBoundingSet = [ ];
      AmbientCapabilities = [ ];
      PrivatePIDs = true;
      ProtectProc = "invisible";
      ProcSubset = "pid";
      PrivateTmp = true;
      PrivateDevices = true;
      ProtectHome = true;
      ProtectSystem = "strict";
      ProtectControlGroups = true;
      ReadWritePaths = [ "/mnt/persistent/choir-signers/verifier" "/run/choir-signers/verifier" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers/guest-core" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-kernel-capability-probe = {
    description = "Probe mandatory guest kernel isolation capabilities";
    before = [ "go-choir-updater.service" "go-choir-sandbox.service" ];
    requiredBy = [ "go-choir-updater.service" "go-choir-sandbox.service" ];
    environment.CHOIR_KERNEL_CAPABILITY_PROBE_OUTPUT = "/run/choir/kernel-capabilities.json";
    serviceConfig = {
      Type = "oneshot";
      User = "root";
      Group = "root";
      ExecStart = "${goChoirPackages.updater}/bin/choir-updater";
      UMask = "0077";
      CapabilityBoundingSet = [ "CAP_SYS_ADMIN" "CAP_SETUID" "CAP_SETGID" ];
      AmbientCapabilities = [ "CAP_SYS_ADMIN" "CAP_SETUID" "CAP_SETGID" ];
      Delegate = true;
      PrivateTmp = true;
      PrivateDevices = true;
      ProtectHome = true;
      ProtectSystem = "strict";
      ProtectControlGroups = false;
      ReadWritePaths = [ "/run/choir" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
    };
  };

  systemd.timers.go-choir-kernel-capability-probe = {
    description = "Refresh guest kernel isolation capability evidence";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnUnitActiveSec = "5min";
      Unit = "go-choir-kernel-capability-probe.service";
    };
  };

  systemd.services.go-choir-updater = {
    description = "Choir guest release updater";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer.service" "go-choir-verifier-signer.service" ];
    before = [ "go-choir-sandbox.service" ];
    wants = [ "network-online.target" ];
    requires = [ "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer.service" "go-choir-verifier-signer.service" ];
    environment.CHOIR_UPDATER_ROOT = "/mnt/persistent/choir-updater";
    environment.CHOIR_GUEST_IMAGE_MANIFEST = guestImageManifest;
    environment.CHOIR_KERNEL_CONFIG = config.boot.kernelPackages.kernel.configfile;
    serviceConfig = {
      Type = "simple";
      User = "root";
      Group = "root";
      ExecStart = "${goChoirPackages.updater}/bin/choir-updater --root /mnt/persistent/choir-updater --socket /run/choir/updater.sock --restart-request /run/choir-updater-control/restart --recovery-restart-request /run/choir-updater-control/recover --recovery-cleanup-request /run/choir-updater-control/cleanup --restart-prepare-url http://127.0.0.1:8085/internal/self-development/restart-handoff --health-url http://127.0.0.1:8085/health --signer-socket /run/choir-signers/guest-core/signer.sock --verifier-signer-socket /run/choir-signers/verifier/signer.sock --guest-image-manifest ${guestImageManifest} --kernel-config ${config.boot.kernelPackages.kernel.configfile}";
      Restart = "on-failure";
      RestartSec = 1;
      UMask = "0077";
      NoNewPrivileges = true;
      CapabilityBoundingSet = [ ];
      AmbientCapabilities = [ ];
      PrivatePIDs = true;
      ProtectProc = "invisible";
      ProcSubset = "pid";
      EnvironmentFile = [ "-/run/go-choir-updater.env" ];
      SystemCallFilter = [ "~@debug" ];
      PrivateTmp = true;
      PrivateDevices = true;
      ProtectHome = true;
      ProtectSystem = "strict";
      ProtectControlGroups = true;
      ReadWritePaths = [ "/mnt/persistent/choir-updater" "/run/choir" "/run/choir-updater-control" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers" "/mnt/persistent/choir-credentials" "/run/choir-bootstrap" "/run/choir-runtime-handoff" "/run/go-choir-sandbox.env" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" "AF_INET" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
    };
  };

  # Sandbox runtime service.
  # Runs the Go sandbox binary which listens for runtime API requests
  # inside the VM. Provider credentials are never in the guest (VAL-VM-011).
  # LLM calls route through the host-side gateway using the extracted token.
  systemd.services.go-choir-sandbox = {
    description = "go-choir Sandbox Runtime (VM guest)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-extract-cmdline.service" "run-choir\\x2dbootstrap.mount" ];
    wants = [ "network-online.target" ];
    requires = [ "go-choir-extract-cmdline.service" "run-choir\\x2dbootstrap.mount" ];
    environment = {
      CHOIR_COMPUTER_CREDENTIAL_FILE = "/run/choir-bootstrap/computer-event-envelope";
      CHOIR_REVOCATION_CREDENTIAL_HANDOFF = "/run/choir-runtime-handoff/revocation-capability";
      CHOIR_RESTART_CREDENTIAL_HANDOFF = "/run/choir-runtime-handoff/restart-capability";
      CHOIR_PRIVACY_KEY_FILE = "/mnt/persistent/choir-credentials/privacy-key";
      CHOIR_KERNEL_CAPABILITY_PROBE = "/run/choir/kernel-capabilities.json";
      PATH = lib.mkForce (lib.makeBinPath (with pkgs; [
        bash
        coreutils
        curl
        findutils
        git
        gnugrep
        gnumake
        gnused
        go
        jq
        nodejs
        gcc
        binutils
        icu
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
      ]));
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
      # Guest-local capsule verification uses the standard Go toolchain; expose
      # the Nix ICU development closure through standard compiler variables.
      CGO_CFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_CXXFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_LDFLAGS = "-L${pkgs.icu}/lib";
      PKG_CONFIG_PATH = "${pkgs.icu.dev}/lib/pkgconfig";
      RUNTIME_SKILLS_ROOT = "${goChoirPackages.sandbox}/share/go-choir/skills";
      CHOIR_CAPSULE_BROKER_PATH = "${goChoirPackages.capsuleBroker}/bin/capsule-broker";
      CHOIR_CAPSULE_STATE_DIR = "/run/choir/capsules";
      CHOIR_CAPSULE_SOURCE_ROOT = "/mnt/persistent/files/Source/platform";
      CHOIR_CAPSULE_LOWER_ROOT = "/";
      RUNTIME_PROMOTION_SOURCE_REPO = sourceRepoRemote;
      RUNTIME_PROMOTION_WORKSPACE_ROOT = "/mnt/persistent/promotion-workspaces";
      # Guest health is part of staging acceptance. Stamp the source revision
      # into the VM runtime so refreshed active computers can prove which guest
      # image they are serving, even though they do not mount host deploy.env.
      CHOIR_DEPLOYED_COMMIT = buildCommit;
      GIT_AUTHOR_NAME = "Choir Capsule";
      GIT_AUTHOR_EMAIL = "capsule@choir.local";
      GIT_COMMITTER_NAME = "Choir Capsule";
      GIT_COMMITTER_EMAIL = "capsule@choir.local";
      # Keep Go build/module caches on the writable data volume.
      GOPATH = "/mnt/persistent/go";
      GOMODCACHE = "/mnt/persistent/go/pkg/mod";
      GOCACHE = "/mnt/persistent/go-build-cache";
      GOTOOLCHAIN = "local";
      # Obscura is the guest-local browser and extraction substrate.
      CHOIR_OBSCURA_BIN = "${goChoirPackages.obscura}/bin/obscura";
      OBSCURA_BIN = "${goChoirPackages.obscura}/bin/obscura";
      CHOIR_ZOT_PATH = "${goChoirPackages.zot}/bin/zot";
      # Explicit runtime-selected model. Provider credentials remain host-side;
      # guest LLM calls route through the gateway token above.
      RUNTIME_LLM_PROVIDER = "deepseek";
      RUNTIME_LLM_MODEL = "deepseek-v4-flash";
      RUNTIME_LLM_REASONING_EFFORT = "medium";
    };
    serviceConfig = {
      ExecStartPre = "";
      ExecStart = "${sandboxRuntimeExec}";
      Restart = "on-failure";
      RestartSec = 1;
      # Updater and capsule-adjacent child work can approach the guest memory
      # envelope. Keep the runtime alive so it can persist terminal evidence.
      OOMPolicy = "continue";
      StandardOutput = "journal+console";
      StandardError = "journal+console";
      EnvironmentFile = [ "-/run/go-choir-sandbox.env" ];
      ReadWritePaths = [ "/mnt/persistent" "/run/choir" "/run/choir-runtime-handoff" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers" "/run/choir-signers" "/run/choir-updater-control" ];
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
  environment.systemPackages = with pkgs; [
    coreutils
    curl
    findutils
    git
    go
    jq
    gnutar
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
  ];

  system.stateVersion = "25.11";
}
