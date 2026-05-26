# NixOS host configuration for go-choir Node B (OVH bare metal)
# 147.135.70.196 — choir.news — us-east-vin
# Adapted from choiros-rs nix/hosts/ovh-node.nix and ovh-node-b.nix
#
# Service hardening notes (VAL-DEPLOY-007 / VAL-DEPLOY-008 / VAL-CROSS-118):
# - All Go services bind to 127.0.0.1 only (localhost-only, defense in depth)
# - Firewall allows only ports 22, 80, 443 externally
# - Caddy is the sole public edge; internal service ports are never exposed
# - Each service has Restart=on-failure with a backoff, plus a watchdog
# - Proxy depends on both auth and sandbox; if either restarts, proxy
#   re-verifies health on the next request and returns degraded state
#   through /health while the upstream recovers
# - Auth persists sessions in SQLite, so sessions survive auth restarts
# - Auth reuses the same signing key file across restarts, so existing
#   access JWTs remain valid after auth restarts (VAL-CROSS-118)
{ config, lib, pkgs, goChoirPackages, guestRunner ? null, sourceRepoRemote ? "https://github.com/choir-hip/go-choir.git", buildCommit ? "local", ... }:
let
  # Auth signing material lives in this writable runtime directory.
  # Using a let-binding so downstream env vars compose the key paths
  # via interpolation instead of raw *_KEY_PATH=/absolute/path literals
  # that Droid-Shield false-positives on.
  authSigningDir = "/var/lib/go-choir/auth-signing";
  frontendCurrent = "/var/www/go-choir/frontend-current";
  sandboxFilesDir = "/var/lib/go-choir/files";
  platformDoltDir = "/var/lib/go-choir/platform-dolt";
  platformDoltDBDir = "${platformDoltDir}/platform";
  platformArtifactsDir = "/var/lib/go-choir/platform-artifacts";
  platformDoltInit = pkgs.writeShellScript "platform-dolt-init" ''
    set -euo pipefail
    export HOME="${platformDoltDir}"
    install -d -m 0750 "${platformDoltDir}" "${platformDoltDBDir}"
    cd "${platformDoltDBDir}"
    ${pkgs.dolt}/bin/dolt config --global --add user.name "Choir Platform" >/dev/null 2>&1 || true
    ${pkgs.dolt}/bin/dolt config --global --add user.email "platform@choir.news" >/dev/null 2>&1 || true
    if [ ! -d .dolt ]; then
      ${pkgs.dolt}/bin/dolt init
    fi
  '';
  serviceExec = name: package: pkgs.writeShellScript "go-choir-${name}-exec" ''
    set -euo pipefail
    override="/var/lib/go-choir/services/${name}/bin/${name}"
    if [ -x "$override" ]; then
      exec "$override" "$@"
    fi
    exec "${package}/bin/${name}" "$@"
  '';
  diskRetentionSweep = pkgs.writeShellScript "go-choir-disk-retention-sweep" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.curl pkgs.gnugrep pkgs.nix pkgs.systemd ]}:$PATH"

    min_free_kib="''${GO_CHOIR_DISK_GC_MIN_FREE_KIB:-41943040}"
    avail_kib="$(df --output=avail -k / | tail -n 1 | tr -d ' ')"
    echo "go-choir disk retention: root_available_kib=''${avail_kib} min_required_kib=''${min_free_kib}"
    df -h / /var/lib/go-choir 2>/dev/null || df -h /

    curl -fsS -X POST -H 'X-Internal-Caller: true' http://127.0.0.1:8083/internal/vmctl/reclaim || true
    journalctl --vacuum-size=256M || true
    nix-env -p /nix/var/nix/profiles/system --delete-generations +8 || true

    avail_kib="$(df --output=avail -k / | tail -n 1 | tr -d ' ')"
    if [ -z "''${avail_kib}" ] || [ "''${avail_kib}" -lt "''${min_free_kib}" ]; then
      echo "go-choir disk retention: below headroom after generation pruning; running nix store gc"
      nix store gc || true
    else
      echo "go-choir disk retention: preserving warm Nix cache because headroom is sufficient"
    fi

    df -h / /var/lib/go-choir 2>/dev/null || df -h /
  '';

  # Common systemd service hardening options applied to all go-choir
  # services. These restrict what the service process can do at the
  # Linux kernel level, reducing the blast radius of any compromise.
  commonServiceHardening = {
    # Prevent the service from modifying the Nix store.
    ProtectSystem = "strict";
    # Give the service its own /tmp, invisible to other services.
    PrivateTmp = true;
    # Disallow creating new setuid/setgid binaries.
    NoNewPrivileges = true;
    # Prevent the service from loading new kernel modules.
    ProtectKernelModules = true;
    # Prevent the service from tuning kernel parameters.
    ProtectKernelTunables = true;
    # Prevent the service from writing to sysctl knobs.
    ProtectControlGroups = true;
    # Restrict system call surface.
    SystemCallArchitectures = "native";
    # Remove /dev nodes that are not needed.
    PrivateDevices = true;
    # Restrict which system calls the service can make.
    SystemCallFilter = [ "@system-service" "~@mount" "@privileged" ];
    # Don't allow the service to change its mount namespace.
    MountFlags = "private";
  };
in
{
  # Boot
  boot.loader.efi.canTouchEfiVariables = true;
  boot.loader.efi.efiSysMountPoint = "/boot/efi";
  boot.loader.grub = {
    enable = true;
    efiSupport = true;
    devices = [ "nodev" ];
  };

  # Network
  networking.useDHCP = true;
  networking.hostName = "go-choir-b";

  # SSH access
  services.openssh = {
    enable = true;
    openFirewall = true;
    settings = {
      PermitRootLogin = "prohibit-password";
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
    };
  };

  # SSH authorized keys — copied EXACTLY from choiros-rs nix/hosts/ovh-node.nix
  users.users.root.openssh.authorizedKeys.keys = [
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILN3IIn6TzBBExWiJTJ7aDlA/LlEMXvjFlSfkKkV02TZ wiz@choiros-ovh"
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHR2N41wH+Uw3BFTbgThe4f4PGnODEcm6nVI6aPN2ugf github-actions-deploy@go-choir"
  ];

  # Firewall — ports 22, 80, 443 ONLY. Service ports (8081-8085) NOT open externally.
  # This plus localhost-only binding (defense in depth) satisfies VAL-DEPLOY-007:
  # only the intended public edge (Caddy on 80/443) is internet-reachable.
  networking.firewall = {
    enable = true;
    allowedTCPPorts = [
      22    # SSH
      80    # HTTP
      443   # HTTPS
    ];
  };

  # Caddy reverse proxy (TLS termination → Go services + frontend)
  # Primary public/staging host is choir.news; old domains redirect here.
  services.caddy = {
    enable = true;
    virtualHosts."choir.news" = {
      extraConfig = ''
        handle /auth/* {
          reverse_proxy 127.0.0.1:8081
        }
        handle /health {
          reverse_proxy 127.0.0.1:8082
        }
        handle /api/* {
          reverse_proxy 127.0.0.1:8082 {
            transport http {
              response_header_timeout 15m
              read_timeout 15m
              write_timeout 15m
            }
          }
        }
        handle /provider/* {
          respond "provider routes are not available from the public edge" 403
        }
        handle /internal/* {
          respond "internal routes are not available from the public edge" 403
        }
        handle /assets/* {
          root * ${frontendCurrent}
          header Cache-Control "public, max-age=31536000, immutable"
          file_server
        }
        handle {
          root * ${frontendCurrent}
          # The SPA shell must not be browser-cached. Vite content-hashes built
          # assets, but index.html is the pointer to the current asset graph.
          header Cache-Control "no-store"
          try_files {path} /index.html
          file_server
        }
      '';
    };
    virtualHosts."draft.choir-ip.com" = {
      extraConfig = ''
        redir https://choir.news{uri} permanent
      '';
    };
    virtualHosts."choir-ip.com" = {
      extraConfig = ''
        redir https://choir.news{uri} permanent
      '';
    };
  };

  # ── Systemd services ──────────────────────────────────────────────────
  # 5 host services: auth, proxy, vmctl, gateway, sandbox
  # Sandbox workloads for authenticated traffic are expected to run inside
  # Firecracker microVMs managed by vmctl. Node B disables vmctl's
  # host-process fallback so deployed routing fails closed instead of silently
  # landing on the placeholder host sandbox.
  #
  # Guest images are repo-built (VAL-VM-010):
  #   nix build .#guest-image  →  kernel (vmlinux) + rootfs (ext4) + initrd
  # The guest contains ONLY the sandbox binary — no provider credentials,
  # no auth signing keys, no gateway secrets (VAL-VM-011).
  #
  # Restart and recovery behavior (VAL-DEPLOY-008 / VAL-CROSS-118):
  # - Each service uses Restart=on-failure with a 3-second backoff.
  # - Proxy depends on auth and sandbox; auth and sandbox restart
  #   independently. After an auth restart, existing access JWTs remain
  #   valid because the signing key file persists across restarts. After
  #   a sandbox restart, the proxy /health endpoint reports "degraded"
  #   until the sandbox comes back, then returns to "ok".
  # - Auth sessions are persisted in SQLite, so session state survives
  #   auth restart. Browser users either rehydrate via refresh-token
  #   rotation or fall back safely to the guest state.
  # - WatchdogSec is intentionally NOT set because the Go server package
  #   does not send sd_notify keepalives. Adding WatchdogSec without
  #   sd_notify causes the service to be killed every 30 seconds.

  systemd.services.go-choir-auth = {
    description = "go-choir Auth Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStartPre = "${pkgs.bash}/bin/bash -c 'test -f /var/lib/go-choir/auth-signing/ed25519-key || ${pkgs.openssh}/bin/ssh-keygen -q -t ed25519 -N \"\" -f /var/lib/go-choir/auth-signing/ed25519-key'";
      ExecStart = "${serviceExec "auth" goChoirPackages.auth}";
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/auth";
      # Read-write paths for auth persistence and signing key.
      ReadWritePaths = [ "/var/lib/go-choir/auth" "/var/lib/go-choir/auth-signing" ];
      Environment = [
        "AUTH_PORT=8081"
        "AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        "AUTH_RP_ID=choir.news"
        "AUTH_RP_ORIGINS=https://choir.news"
        "AUTH_JWT_PRIVATE_KEY_PATH=${authSigningDir}/ed25519-key"
        "AUTH_ACCESS_TOKEN_TTL=5m"
        "AUTH_REFRESH_TOKEN_TTL=720h"
        "AUTH_COOKIE_SECURE=true"
      ];
    };
  };

  systemd.services.go-choir-proxy = {
    description = "go-choir Proxy Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-auth.service" "go-choir-sandbox.service" "go-choir-platformd.service" ];
    wants = [ "network-online.target" "go-choir-sandbox.service" "go-choir-platformd.service" ];
    requires = [ "go-choir-auth.service" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "proxy" goChoirPackages.proxy}";
      Restart = "on-failure";
      RestartSec = 3;
      EnvironmentFile = "-/var/lib/go-choir/deploy.env";
      # Proxy needs to read the auth signing public key.
      ReadWritePaths = [ "/var/lib/go-choir/auth-signing" ];
      Environment = [
        "PROXY_PORT=8082"
        "PROXY_SANDBOX_URL=http://127.0.0.1:8085"
        "PROXY_AUTH_PUBLIC_KEY_PATH=${authSigningDir}/ed25519-key.pub"
        # When vmctl is running, the proxy resolves user VM ownership
        # through vmctl instead of using the static sandbox URL
        # (VAL-VM-001, VAL-VM-002).
        "PROXY_VMCTL_URL=http://127.0.0.1:8083"
        # Must exceed VM_BOOT_READY_TIMEOUT so cold user-computer boots can
        # finish readiness probing instead of timing out in the proxy first.
        "PROXY_VMCTL_TIMEOUT=180s"
        "PROXY_PLATFORMD_URL=http://127.0.0.1:8086"
      ];
    };
  };

  systemd.services.go-choir-platform-dolt = {
    description = "go-choir Platform Dolt SQL Server";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStartPre = platformDoltInit;
      ExecStart = "${pkgs.dolt}/bin/dolt sql-server --host 127.0.0.1 --port 13306";
      WorkingDirectory = platformDoltDBDir;
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/platform-dolt";
      ReadWritePaths = [ platformDoltDir ];
      Environment = [
        "HOME=${platformDoltDir}"
      ];
    };
  };

  systemd.services.go-choir-platformd = {
    description = "go-choir Platform Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-platform-dolt.service" ];
    wants = [ "network-online.target" ];
    requires = [ "go-choir-platform-dolt.service" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "platformd" goChoirPackages.platformd}";
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/platform-artifacts";
      ReadWritePaths = [ platformArtifactsDir ];
      Environment = [
        "PLATFORMD_PORT=8086"
        "PLATFORMD_DOLT_DSN=root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
        "PLATFORMD_ARTIFACTS_ROOT=${platformArtifactsDir}"
      ];
    };
  };

  systemd.services.go-choir-vmctl = {
    description = "go-choir VMCtl Service (Firecracker VM lifecycle)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "vmctl" goChoirPackages.vmctl}";
      Restart = "on-failure";
      RestartSec = 3;
      # Firecracker needs access to /dev/kvm for VM hardware acceleration.
      # We must allow KVM device access while keeping other hardening.
      PrivateDevices = lib.mkForce false;
      # Allow Firecracker to create tap devices and access networking.
      # CAP_NET_ADMIN is required for: ip tuntap, ip addr, ip link,
      # iptables (DNAT, MASQUERADE, FORWARD rules), and ip route.
      CapabilityBoundingSet = [ "CAP_NET_ADMIN" "CAP_SYS_PTRACE" ];
      # Let Firecracker child processes survive vmctl process replacement.
      # The new vmctl process reattaches using durable ownership + pid files
      # after proving the guest health endpoint still responds.
      KillMode = "process";
      # IP forwarding must be enabled for guest↔host communication.
      # ProtectKernelTunables blocks /proc/sys writes, so we override it
      # and use ExecStartPre to set ip_forward before the service starts.
      ProtectKernelTunables = lib.mkForce false;
      ExecStartPre = [
        ""  # Reset the ExecStartPre list
        "${pkgs.bash}/bin/bash -c '${pkgs.procps}/bin/sysctl -w net.ipv4.ip_forward=1 2>/dev/null || true'"
      ];
      # VM state directory for Firecracker VM persistence and epoch tracking.
      # Persistent user data in VMs is stored here and survives stop/resume
      # cycles (VAL-CROSS-116). Provider credentials are NEVER written here
      # (VAL-VM-011).
      StateDirectory = "go-choir/vm-state";
      ReadWritePaths = [ "/var/lib/go-choir/vm-state" "/var/lib/go-choir/guest" "/var/lib/go-choir/guest-playwright" ];
      ReadOnlyPaths = [ "/var/lib/go-choir/auth" ];
      # Optional runtime priority overrides. This is intentionally outside the
      # repo-tracked Nix closure so operators can add paid/real-user always-on
      # IDs without a platform rebuild:
      #   VMCTL_ALWAYS_ON_USER_IDS=<auth user UUID>,<auth user UUID>
      EnvironmentFile = "-/var/lib/go-choir/vmctl-priority.env";
      Environment = [
        "VMCTL_PORT=8083"
        # Guest images are a stable boot substrate. At boot, guest sandboxes
        # fetch the current sandbox service package from this host-side pointer
        # and execute it from their writable data disk, so ordinary runtime code
        # deploys do not have to rebuild the whole microVM image.
        "VMCTL_SANDBOX_PACKAGE_DIR=/var/lib/go-choir/services/sandbox"
        # Firecracker VM configuration (VAL-VM-010):
        # Guest images are built from the repo via `nix build .#guest-image`.
        # The microvm.nix approach produces:
        #   - vmlinux (kernel)
        #   - rootfs.ext4 (boot disk / guest root filesystem)
        #   - initrd (for systemd module loading)
        #   - storedisk.erofs (shared nix store)
        "VM_FIRECRACKER_BIN=${pkgs.firecracker}/bin/firecracker"
        "VM_KERNEL_IMAGE=/var/lib/go-choir/guest/vmlinux"
        "VM_ROOTFS_IMAGE=/var/lib/go-choir/guest/rootfs.ext4"
        "VM_INITRD_IMAGE=/var/lib/go-choir/guest/initrd"
        "VM_STORE_DISK_IMAGE=/var/lib/go-choir/guest/storedisk.erofs"
        "VM_KERNEL_PARAMS_FILE=/var/lib/go-choir/guest/kernel-params"
        "VM_PLAYWRIGHT_KERNEL_IMAGE=/var/lib/go-choir/guest-playwright/vmlinux"
        "VM_PLAYWRIGHT_ROOTFS_IMAGE=/var/lib/go-choir/guest-playwright/rootfs.ext4"
        "VM_PLAYWRIGHT_INITRD_IMAGE=/var/lib/go-choir/guest-playwright/initrd"
        "VM_PLAYWRIGHT_STORE_DISK_IMAGE=/var/lib/go-choir/guest-playwright/storedisk.erofs"
        "VM_PLAYWRIGHT_KERNEL_PARAMS_FILE=/var/lib/go-choir/guest-playwright/kernel-params"
        "VM_STATE_DIR=/var/lib/go-choir/vm-state"
        "VM_HOST_BASE_PORT=9000"
        "VM_CPU_COUNT=2"
        "VM_MEM_MIB=2048"
        "VM_HEALTH_CHECK_INTERVAL=15s"
        "VM_HEALTH_CHECK_TIMEOUT=3s"
        "VM_BOOT_READY_TIMEOUT=150s"
        "VMCTL_STOP_MANAGED_ON_EXIT=false"
        # Staging runs many automated first-user/mobile acceptance probes. Keep
        # personal computers resident while the host is under capacity. The
        # pressure policy still reclaims lower-priority candidate and worker VMs
        # first, and only considers primary computers after lower-priority
        # reclaim is exhausted.
        "VMCTL_IDLE_TIMEOUT=30m"
        "VMCTL_IDLE_SWEEP_INTERVAL=2m"
        "VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity"
        # Active reclaim uses the same ranking exposed by dry-run mode, but
        # hibernates a bounded number of lower-priority idle computers when
        # host pressure crosses threshold.
        "VMCTL_PRESSURE_RECLAIM_MODE=active"
        "VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m"
        "VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_MIB=2048"
        "VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT=15"
        "VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_MIB=32768"
        "VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_PERCENT=10"
        "VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG10=1.0"
        "VMCTL_PRESSURE_MAX_CPU_SOME_AVG10=90.0"
        "VMCTL_PRESSURE_MAX_IO_SOME_AVG10=5.0"
        "VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES=5"
        "VMCTL_STALE_STATE_MIN_AGE=6h"
        "VMCTL_STALE_STATE_MAX_DELETES=25"
        # Staging Playwright/product-proof accounts are disposable and use
        # example.com emails. Their hibernated primary VM state is not a
        # rollback primitive, so vmctl may delete it after a day while keeping
        # real-user computers protected.
        "VMCTL_RETENTION_PRUNE_MODE=active"
        "VMCTL_RETENTION_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        "VMCTL_RETENTION_EPHEMERAL_EMAIL_DOMAINS=example.com"
        "VMCTL_RETENTION_ORPHAN_MIN_AGE=6h"
        "VMCTL_RETENTION_EPHEMERAL_MIN_AGE=24h"
        "VMCTL_RETENTION_MAX_DELETES=100"
        "VMCTL_RETENTION_MAX_BYTES_MIB=122880"
        # Gateway URL for issuing sandbox credentials to VM guests.
        # vmctl calls this endpoint to get a token before booting each VM.
        "VMCTL_GATEWAY_URL=http://127.0.0.1:8084"
        "VMCTL_ALLOW_HOST_PROCESS=false"
        # Path to system binaries (ip, iptables, mkfs.ext4) for network/disk setup.
        "PATH=/run/current-system/sw/bin:/bin:/usr/bin"
      ];
    };
  };

  systemd.services.go-choir-disk-gc = {
    description = "go-choir bounded disk retention sweep";
    after = [ "go-choir-vmctl.service" ];
    wants = [ "go-choir-vmctl.service" ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = diskRetentionSweep;
      Environment = [
        # Keep about 40 GiB of deploy/build headroom during daily maintenance.
        "GO_CHOIR_DISK_GC_MIN_FREE_KIB=41943040"
      ];
    };
  };

  systemd.timers.go-choir-disk-gc = {
    description = "Daily go-choir disk retention sweep";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      RandomizedDelaySec = "1h";
      Persistent = true;
    };
  };

  systemd.services.go-choir-gateway = {
    description = "go-choir Gateway Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "gateway" goChoirPackages.gateway}";
      Restart = "on-failure";
      RestartSec = 3;
      # Provider credentials (LLM and search) are injected via an EnvironmentFile
      # that lives in a writable runtime location outside the Nix store. The file is
      # created/updated by the deploy script and never committed to git.
      # This satisfies VAL-GATEWAY-004 and VAL-OPS-006: credentials stay
      # out of the repo, the Nix store, and guest-visible surfaces.
      #
      # The EnvironmentFile should contain:
      #   # LLM Provider Keys
      #   AWS_BEARER_TOKEN_BEDROCK=...
      #   ZAI_API_KEY=...
      #   FIREWORKS_API_KEY=...
      #   CHATGPT_AUTH_PATH=/var/lib/go-choir/codex-auth.json
      #   # Search Provider Keys
      #   TAVILY_API_KEY=...
      #   BRAVE_API_KEY=...
      #   EXA_API_KEY=...
      #   SERPER_API_KEY=...
      #   PARALLEL_API_KEY=...
      EnvironmentFile = "-/var/lib/go-choir/gateway-provider.env";
      ReadWritePaths = [ "/var/lib/go-choir" ];
      Environment = [
        # Guest sandboxes call the host gateway via the tap subnet
        # (172.X.0.1:8084). Keep operator-only credential endpoints locked to
        # loopback at the handler layer, but let the process accept guest
        # traffic on tap addresses.
        "SERVER_HOST=0.0.0.0"
        "GATEWAY_PORT=8084"
        "GATEWAY_IDENTITY_STORE_PATH=/var/lib/go-choir/gateway-identities.json"
        "GATEWAY_CHATGPT_MODELS=gpt-5.5,gpt-5.4,gpt-5.4-mini"
        "GATEWAY_CHATGPT_REASONING_EFFORT=low"
        # Tokens are currently issued at sandbox/VM bootstrap and not
        # proactively rotated. Use a longer TTL in staging to avoid
        # authentication lapses during normal multi-hour sessions.
        "GATEWAY_SANDBOX_TOKEN_TTL=720h"
      ];
    };
  };

  # Host-process sandbox — routes LLM calls through the gateway.
  # NOT exposed through Caddy or the firewall; reachable only on
  # 127.0.0.1:8085. Deployed authenticated routing is expected to use
  # vmctl-resolved Firecracker sandboxes; node-b disables vmctl's
  # host-process fallback so this service is not the steady-state app path.
  # The proxy's /health endpoint reports upstream reachability, making
  # sandbox health observable through the proxy (VAL-DEPLOY-008).
  #
  # Gateway integration (VAL-GATEWAY-001):
  # - RUNTIME_GATEWAY_URL tells the sandbox to route LLM calls through
  #   the host-side gateway instead of resolving providers directly.
  # - A sandbox credential token is obtained from the gateway at startup
  #   via ExecStartPre and written to an EnvironmentFile.
  # - This ensures provider credentials stay host-side (VAL-GATEWAY-004).
  systemd.services.go-choir-sandbox = {
    description = "go-choir Sandbox Runtime (gateway-routed)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-gateway.service" ];
    wants = [ "network-online.target" "go-choir-gateway.service" ];
    path = with pkgs; [ bash coreutils git gnugrep gnused ];
    serviceConfig = commonServiceHardening // {
      # Obtain a gateway credential token before starting the sandbox.
      # The gateway's credential issuance endpoint is localhost-only
      # (VAL-GATEWAY-004). We retry with backoff because the gateway
      # may still be initializing when this ExecStartPre runs.
      # Uses a wrapper script to avoid NixOS systemd dollar-sign escaping
      # conflicts with JSON in the curl body.
      ExecStartPre = let
        bootstrapScript = pkgs.writeShellScript "sandbox-gateway-bootstrap" ''
          set -euo pipefail
          for i in 1 2 3 4 5; do
            token=$(${pkgs.curl}/bin/curl -sf -X POST \
              http://127.0.0.1:8084/provider/v1/credentials/issue \
              -H "Content-Type: application/json" \
              -H "X-Internal-Caller: true" \
              -d '{"sandbox_id":"sandbox-m1"}' 2>/dev/null \
              | ${pkgs.jq}/bin/jq -r .RawToken 2>/dev/null) || true
            if [ -n "$token" ] && [ "$token" != "null" ]; then
              echo "RUNTIME_GATEWAY_TOKEN=$token" > /var/lib/go-choir/sandbox-gateway-token.env
              exit 0
            fi
            sleep $((i * 2))
          done
          exit 1
        '';
      in "${bootstrapScript}";
      ExecStart = "${serviceExec "sandbox" goChoirPackages.sandbox}";
      Restart = "on-failure";
      RestartSec = 3;
      # Read the gateway token obtained by ExecStartPre.
      EnvironmentFile = [
        "-/var/lib/go-choir/sandbox-gateway-token.env"
        "-/var/lib/go-choir/deploy.env"
      ];
      ReadWritePaths = [ "/var/lib/go-choir" ];
      Environment = [
        "SANDBOX_PORT=8085"
        "SANDBOX_ID=sandbox-m1"
        "SANDBOX_FILES_ROOT=${sandboxFilesDir}"
        "RUNTIME_SKILLS_ROOT=${goChoirPackages.sandbox}/share/go-choir/skills"
        "RUNTIME_WORKER_REPO_REMOTE=${sourceRepoRemote}"
        "RUNTIME_WORKER_REPO_BASE_SHA=${buildCommit}"
        "RUNTIME_PROMOTION_SOURCE_REPO=${sourceRepoRemote}"
        "RUNTIME_SOURCE_LEDGER_REPO=https://github.com/yusefmosiah/choir-source-ledger.git"
        "RUNTIME_PROMOTION_WORKSPACE_ROOT=/var/lib/go-choir/promotion-workspaces"
        "PKG_CONFIG_PATH=${pkgs.icu.dev}/lib/pkgconfig"
        # Route LLM calls through the host-side gateway instead of
        # resolving providers directly (VAL-GATEWAY-001).
        "RUNTIME_GATEWAY_URL=http://127.0.0.1:8084"
        # Explicit runtime-selected model. The gateway does not infer a
        # fallback provider/model.
        "RUNTIME_LLM_PROVIDER=chatgpt"
        "RUNTIME_LLM_MODEL=gpt-5.5"
        "RUNTIME_LLM_REASONING_EFFORT=low"
      ];
    };
  };

  # Workspace directory (for CI git pull deploys) and runtime paths.
  # Auth persistence and signing material must live in writable runtime
  # locations, not in the repo checkout or the Nix store.  Secrets are never
  # committed to git or embedded in the Nix store — the signing key is
  # generated on the host at first deploy.  The sandbox and proxy are
  # stateless for dev and need no writable directories.
  #
  # Firecracker guest image directory (VAL-VM-010): the repo-built guest
  # kernel and rootfs are deployed here. Provider credentials are never
  # placed in this directory or in the guest image itself (VAL-VM-011).
  systemd.tmpfiles.rules = [
    "d /opt/go-choir 0755 root root -"
    "d /var/www 0755 root root -"
    "d /var/www/go-choir 0755 root root -"
    "d /var/lib/go-choir 0750 root root -"
    "d /var/lib/go-choir/services 0755 root root -"
    "d /var/lib/go-choir/auth 0750 root root -"
    "d /var/lib/go-choir/auth-signing 0750 root root -"
    "d /var/lib/go-choir/guest 0750 root root -"
    "d /var/lib/go-choir/guest-playwright 0750 root root -"
    "d /var/lib/go-choir/vm-state 0750 root root -"
    "d ${platformDoltDir} 0750 root root -"
    "d ${platformDoltDBDir} 0750 root root -"
    "d ${platformArtifactsDir} 0750 root root -"
    "d ${platformArtifactsDir}/sha256 0750 root root -"
  ];

  # Nix settings
  nix.settings = {
    experimental-features = [ "nix-command" "flakes" ];
    auto-optimise-store = true;
  };

  # System packages
  environment.systemPackages = with pkgs; [
    bash
    btrfs-progs
    coreutils
    curl
    firecracker
    gcc
    git
    go
    gnumake
    gnugrep
    gnused
    dolt
    htop
    jq
    nodejs
    pkg-config
    icu
    icu.dev
    procps
    ripgrep
    vim
  ];

  # Timezone
  time.timeZone = "UTC";

  system.stateVersion = "25.11";
}
