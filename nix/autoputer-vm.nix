# Autoputer guest NixOS config for Firecracker microVMs on Node B.
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
# Guest contains ONLY the autoputer runtime binary — no provider credentials,
# no auth signing keys, no gateway secrets (VAL-VM-011).
#
# The vmmanager package (internal/vmmanager) launches Firecracker with
# the kernel, rootfs, and store disk from the microvm runner outputs.
# It does NOT use the microvm runner scripts directly because vmmanager
# needs per-VM networking, port assignment, and lifecycle control.
# buildCommit is the current deployment commit in the guest's signed executable
# identity, not the independently versioned ComputerVersion route source.
{ config, lib, pkgs, goChoirPackages, choirGo, sourceRepoRemote ? "https://github.com/choir-hip/go-choir.git", buildCommit ? "local", ... }:

let
  documentPython = pkgs.python3.withPackages (ps: with ps; [
    beautifulsoup4
    ebooklib
    lxml
    pdfplumber
    pip
    pypdf
    python-docx
  ]);
  # claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md):
  # pinned, unmodified Claude Code from nixpkgs (unfree; the only unfree
  # package admitted below). In the store of every guest but on no PATH: only
  # a claude-mode capsule's broker runs it, and only on the allowlisted
  # computer. Upgrading it is a reviewed image change (flags are pinned to
  # this version in internal/claudedelegate).
  claudeCode = pkgs.claude-code;
  # visual capture (docs/design/visual-evidence-slice-2026-10-15.md): full
  # Chromium (nixpkgs has no headless shell) in the store of every guest but
  # on no PATH. Only a capture job in a capsule of a computer the host granted
  # gets its path (CHOIR_CAPTURE_BROWSER, set by the capsule broker); the
  # fontconfig file names DejaVu so text renders without /etc/fonts.
  captureBrowser = pkgs.chromium;
  captureFontsConf = pkgs.makeFontsConf { fontDirectories = [ pkgs.dejavu_fonts ]; };
  guestImageManifest = pkgs.writeText "choir-immutable-guest-image-manifest" ''
    contract=choir-guest-image-v1
    build_commit=${buildCommit}
    autoputer=${goChoirPackages.autoputer}
    updater=${goChoirPackages.updater}
    capsule_broker=${goChoirPackages.capsuleBroker}
    claude_code=${claudeCode}
    capture_browser=${captureBrowser}
    kernel=${config.boot.kernelPackages.kernel}
    kernel_config=${config.boot.kernelPackages.kernel.configfile}
  '';
  guestSignerStateMigration = pkgs.writeShellApplication {
    name = "choir-guest-signer-state-migrate";
    runtimeInputs = [ pkgs.coreutils pkgs.findutils ];
    text = builtins.readFile ../scripts/guest-signer-state-migrate;
  };
  guestSignerStateProjection = pkgs.writeShellApplication {
    name = "choir-guest-signer-state-project";
    runtimeInputs = [ pkgs.coreutils ];
    text = builtins.readFile ../scripts/guest-signer-state-project;
  };
  guestSignerReadiness = pkgs.writeShellApplication {
    name = "choir-guest-signer-ready";
    runtimeInputs = [ pkgs.coreutils pkgs.curl ];
    text = ''
      while true; do
        if curl --fail --silent --max-time 1 \
          --unix-socket /run/choir-signers/guest-core/signer.sock \
          http://localhost/v1/public-key >/dev/null; then
          exit 0
        fi
        sleep 0.1
      done
    '';
  };
  guestSignerFailureDiagnostics = pkgs.writeShellScript "choir-guest-signer-failure-diagnostics" ''
    set -euo pipefail
    unit=go-choir-guest-receipt-signer-state-migration.service
    echo "choir-guest-signer: migration unit failure diagnostics"
    ${pkgs.systemd}/bin/systemctl show "$unit" \
      --property=Result \
      --property=ExecMainCode \
      --property=ExecMainStatus \
      --property=ExecStart \
      --property=ExecStartPost || true
    ${pkgs.systemd}/bin/journalctl \
      --boot=0 \
      --unit="$unit" \
      --lines=20 \
      --no-pager \
      --output=short-monotonic || true
  '';




  autoputerRuntimeExec = pkgs.writeShellScript "go-choir-run-autoputer-runtime" ''
    set -euo pipefail

    if [ -f /run/go-choir-autoputer.env ]; then
      set -a
      # shellcheck disable=SC1091
      . /run/go-choir-autoputer.env
      set +a
    fi

    # Reassert the immutable baseline after mutable kernel-derived environment
    # expansion. Pristine updater roots need this exact release to produce a
    # route-bound kernel capability receipt before genesis.
    export CHOIR_BASELINE_RELEASE_ROOT="${goChoirPackages.autoputer}"

    export CHOIR_UPDATER_ROOT="/mnt/persistent/choir-updater"
    export RUNTIME_SKILLS_ROOT="${goChoirPackages.autoputer}/share/go-choir/skills"

    # S2 layering: when the updater has pinned a layered release, exec its
    # binary instead of the base-image autoputer. The release's delta store
    # paths live in the updater's private store ($CHOIR_UPDATER_ROOT/store);
    # the base store is a read-only EROFS. Run the release inside a private
    # mount namespace that overlays the private store (upper) over the base
    # /nix/store (lower) so the release's own closure paths resolve while
    # base-present paths come through the EROFS unchanged. Any layering
    # failure must fall back to the base binary — a wedged ExecStart on every
    # guest boot is a worse failure than running the baseline.
    priv_store="$CHOIR_UPDATER_ROOT/store"
    overlay_work="$CHOIR_UPDATER_ROOT/.overlay-work"
    # The exec pointer lives inside the release dir, so the updater's
    # current/ swap moves served frontend and exec atomically and recovery
    # restores the prior exec with the same pointer (S2-e rollback
    # atomicity). A plain release carries no pointer and execs the base.
    release_bin=""
    if [ -f "$CHOIR_UPDATER_ROOT/current/layering-entrypoint" ]; then
      release_bin="$(head -1 "$CHOIR_UPDATER_ROOT/current/layering-entrypoint" 2>/dev/null)"
    fi
    # Base binding: a layered release runs only on the base image it was
    # built against. Apply enforces this (updater base join); boot must too,
    # or a platform base-image deploy leaves the previous layer exec'ing on a
    # base it never resolved against, and the deploy's own runtime (in the
    # new base) never runs (docs/problems/guest-runtime-fix-never-reaches-
    # layered-computers-2026-10-09.md). A release without a recorded base
    # digest, or an unreadable manifest, keeps the prior behavior.
    if [ -n "$release_bin" ]; then
      release_base="$(${pkgs.jq}/bin/jq -r '.base_image_manifest_digest // empty' "$CHOIR_UPDATER_ROOT/current/release-manifest.json" 2>/dev/null || true)"
      booted_base="$(${pkgs.coreutils}/bin/sha256sum ${guestImageManifest} 2>/dev/null | ${pkgs.coreutils}/bin/cut -d' ' -f1)"
      if [ -n "$release_base" ] && [ -n "$booted_base" ] && [ "$release_base" != "$booted_base" ]; then
        echo "go-choir-autoputer: layered release built for base $release_base, booted base $booted_base; serving base runtime" >&2
        printf '%s base_mismatch release_base=%s booted_base=%s entrypoint=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)" "$release_base" "$booted_base" "$release_bin" >> "$CHOIR_UPDATER_ROOT/layering-diag.log" 2>/dev/null || true
        release_bin=""
      fi
    fi
    # Boot-loop guard: a layered release that fails to stay started 3 times
    # inside 60s is refused further exec attempts; the base runtime serves
    # instead with CHOIR_LAYERING_BOOTGUARD set so /health reports
    # layering_bootguard (failing the apply probe and flagging the trip),
    # and a host-readable receipt records the key, the count, and the time.
    # The guard key is the release dir basename, so recovery/restage of a
    # different release — or the same release after a manual retry — is not
    # penalized by the failed release's count.
    layering_guarded=""
    if [ -n "$release_bin" ]; then
      g_key="$(basename "$(readlink "$CHOIR_UPDATER_ROOT/current" 2>/dev/null || echo unknown)")"
      g_dir="$CHOIR_UPDATER_ROOT/bootguard"
      mkdir -p "$g_dir" 2>/dev/null || true
      g_file="$g_dir/$g_key"
      now_s="$(date +%s 2>/dev/null || echo 0)"
      g_count=0
      g_since=0
      if [ -f "$g_file" ]; then
        g_count="$(sed -n '1p' "$g_file" 2>/dev/null || echo 0)"
        g_since="$(sed -n '2p' "$g_file" 2>/dev/null || echo 0)"
      fi
      [ -z "$g_count" ] && g_count=0
      [ -z "$g_since" ] && g_since=0
      case $g_count in *[!0-9]*) g_count=0 ;; esac
      case $g_since in *[!0-9]*) g_since=0 ;; esac
      if [ "$((now_s - g_since))" -gt 60 ]; then
        g_count=0
        g_since="$now_s"
      fi
      g_count=$((g_count + 1))
      printf '%s\n%s\n' "$g_count" "$g_since" > "$g_file" 2>/dev/null || true
      if [ "$g_count" -ge 3 ]; then
        layering_guarded="$g_key"
        printf '{"release_key":"%s","starts":%s,"first_start_epoch":%s,"tripped_epoch":%s}\n' \
          "$g_key" "$g_count" "$g_since" "$now_s" > "$g_dir/$g_key.tripped" 2>/dev/null || true
        echo "go-choir-autoputer: layering bootguard tripped for release $g_key after $g_count starts; serving base runtime" >&2
      fi
    fi
    if [ -z "$layering_guarded" ] && [ -n "$release_bin" ] && [ -x "$release_bin" ] && [ -d "$priv_store" ] && [ -n "$(ls -A "$priv_store" 2>/dev/null)" ]; then
      mkdir -p "$overlay_work" || true
      # The release's own root (frontend, share, skills) is its layerdir —
      # point the baseline root at it so the computer surface serves the
      # new release even when the store overlay is not in effect. Keep the
      # base skills root unless the release actually ships its own skills.
      export CHOIR_BASELINE_RELEASE_ROOT="$(dirname "$(dirname "$release_bin")")"
      if [ -d "$CHOIR_BASELINE_RELEASE_ROOT/share/go-choir/skills" ]; then
        export RUNTIME_SKILLS_ROOT="$CHOIR_BASELINE_RELEASE_ROOT/share/go-choir/skills"
      fi
      echo "go-choir-autoputer: layering release $release_bin over private store" >&2
      # Diagnostics: capture each layering stage's outcome to a durable file.
      # The guest journal is not host-readable and disposables carry no serial
      # sink, so this file is the only record of why the overlay failed.
      diag="$CHOIR_UPDATER_ROOT/layering-diag.log"
      {
        echo "=== layering attempt $(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true) ==="
        echo "release_bin=$release_bin"
        echo "guard_key=$(basename "$(readlink "$CHOIR_UPDATER_ROOT/current" 2>/dev/null || echo unknown)")"
        ${pkgs.util-linux}/bin/unshare -m --propagation private ${pkgs.runtimeShell} -c "
          echo unshare_ok
          ${pkgs.util-linux}/bin/mount -t overlay overlay -o lowerdir=/nix/store,upperdir='$priv_store',workdir='$overlay_work' /nix/store && echo mount_ok || echo mount_fail:\$?
          [ -x '$release_bin' ] && echo exec_path_resolves || echo exec_path_missing
        " -- "$@" 2>&1 || echo unshare_or_stage_failed:\$?
      } > "$diag" 2>&1 || true
      ${pkgs.util-linux}/bin/unshare -m --propagation private ${pkgs.runtimeShell} -c "
        ${pkgs.util-linux}/bin/mount -t overlay overlay -o lowerdir=/nix/store,upperdir='$priv_store',workdir='$overlay_work' /nix/store &&
        exec '$release_bin' \"\$@\"
      " -- "$@" || {
        # The overlay failed (private mount-ns semantics, or a guest kernel
        # constraint). Fall back to exec'ing the release directly: autoputer
        # is a static binary whose frontend/skills resolve through the
        # release root env above, so same-toolchain releases run correctly
        # without the store overlay. Log the specific stage so the journal
        # carries why the overlay was skipped.
        echo "go-choir-autoputer: store overlay unavailable; exec'ing release directly" >&2
        exec "$release_bin" "$@"
      }
    fi
    if [ -n "$layering_guarded" ]; then
      export CHOIR_LAYERING_BOOTGUARD="$layering_guarded"
    fi
    exec ${goChoirPackages.autoputer}/bin/autoputer "$@"
  '';
in
{
  # The base-image identity contract in a stable guest-facing location: the
  # updater reads it via --guest-image-manifest, and mkGuestImage exports it
  # beside vmlinux/storedisk so the host builder's base-join inputs come
  # from the deployed image itself (S2-g).
  environment.etc."choir-guest-image-manifest".source = guestImageManifest;

  networking.hostName = "go-choir-autoputer";

  # Capsule overlays and the enforcing boot probe both require the module to
  # be loaded, not merely present in the immutable kernel closure.
  # virtio_balloon (=m in the guest kernel) carries free page reporting
  # (CONFIG_PAGE_REPORTING=y) when vmctl adds the balloon device; udev would
  # load it by modalias, this makes it explicit. Inert without the device.
  # virtio_mem (=m) serves vmctl's memory leases on computers launched with
  # a hotplug region; Firecracker refuses a resize until the driver binds
  # (docs/reports/memory-hotplug-controller-design-2026-10-14.md). The
  # onlining policy (memhp_default_state=online_movable) is on the command
  # line of those computers only. Inert without the device.
  boot.kernelModules = [ "overlay" "virtio_balloon" "virtio_mem" ];

  # Guest kernel hardening (SDC SEC-15; the guest set no sysctls before).
  # Each is a default-off-for-attackers setting that nothing in the guest
  # needs: kernel addresses and the kernel log are hidden from unprivileged
  # processes, unprivileged BPF and kexec are off, perf events are
  # restricted, the root user's magic SysRq is off, links and fifos in
  # sticky directories are protected, and the network stack ignores source
  # routes and redirects and checks reverse paths (the guest has one
  # interface and one route). A key the kernel lacks is logged by
  # systemd-sysctl and skipped, never a boot failure. Deliberately not set:
  # user.max_user_namespaces (the runtime makes each capsule's user
  # namespace) and kernel.io_uring_disabled (pending the capsule workload
  # inventory).
  boot.kernel.sysctl = {
    "kernel.kptr_restrict" = 2;
    "kernel.dmesg_restrict" = 1;
    "kernel.unprivileged_bpf_disabled" = 1;
    "net.core.bpf_jit_harden" = 2;
    "kernel.kexec_load_disabled" = 1;
    "kernel.sysrq" = 0;
    "kernel.perf_event_paranoid" = 2;
    "fs.protected_symlinks" = 1;
    "fs.protected_hardlinks" = 1;
    "fs.protected_fifos" = 1;
    "fs.protected_regular" = 1;
    "net.ipv4.conf.all.rp_filter" = 1;
    "net.ipv4.conf.default.rp_filter" = 1;
    "net.ipv4.conf.all.accept_source_route" = 0;
    "net.ipv4.conf.all.accept_redirects" = 0;
    "net.ipv4.conf.default.accept_redirects" = 0;
    "net.ipv4.conf.all.send_redirects" = 0;
    "net.ipv6.conf.all.accept_redirects" = 0;
    "net.ipv6.conf.all.accept_source_route" = 0;
    "net.ipv4.tcp_syncookies" = 1;
  };

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

    # Mutable autoputer state on a virtio-blk volume (/dev/vdb).
    # vmmanager creates the actual data.img per-VM at runtime from
    # the VM state directory. This declaration tells microvm.nix to
    # include virtio-blk support in the guest kernel/initrd.
    volumes = [{
      image = "data.img";
      mountPoint = "/mnt/persistent";
      size = 2048;
    }];

    # Use upstream microvm.nix API for the nix store disk.
    # erofs provides one read-only nix store image that every VM reads: the
    # host keeps a single copy of it in its page cache. Guest RAM is not
    # deduplicated: KSM is off on the host and nothing here relies on it
    # (SDC SEC-09; it was a side channel and it merged nothing).
    storeOnDisk = true;
    storeDiskType = "erofs";
    # Favor deploy-loop speed over maximum image compaction. microvm.nix's
    # default EROFS flags include fragments/dedupe on newer kernels, which
    # force the single-threaded mkfs.erofs path; keeping only fast LZ4 lets the
    # builder use the multithread-capable tool. CI lets Node B build the
    # canonical guest image so its persistent Nix store absorbs the cost
    # instead of rebuilding and copying the large image output from each
    # ephemeral runner.
    storeDiskErofsFlags = [ "-zlz4" ];

    # No virtiofs or 9p shares. With shares = [], microvm.nix
    # automatically generates an erofs disk for the nix store closure.
    # This is more efficient than virtiofs for our use case because:
    # - No virtiofsd daemon needed on the host
    # - erofs disk is a single shared file referenced by all VMs
    # - The host page cache holds the image once for all guests; KSM is off,
    #   so identical guest pages are not merged (SDC SEC-09)
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
      dns_ip=""

      for param in $(cat /proc/cmdline); do
        case "$param" in
          ip=*)
            value="''${param#ip=}"
            IFS=':' read -r client_ip _ gateway_ip _ _ _ _ <<EOF
$value
EOF
            ;;
          # SDC SEC-10: the host names this computer's resolver (a host
          # service on the tap peer) with choir.dns=<ipv4>. Only a plain
          # IPv4 address is accepted; anything else is ignored and the guest
          # keeps its fallback resolvers, as before the host serves DNS.
          choir.dns=*)
            candidate="''${param#choir.dns=}"
            if printf '%s' "$candidate" | grep -Eq '^([0-9]{1,3}[.]){3}[0-9]{1,3}$'; then
              dns_ip="$candidate"
            else
              echo "go-choir-configure-network: ignoring malformed choir.dns" >&2
            fi
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
      if [ -n "$dns_ip" ]; then
        echo "DNS=$dns_ip" >> /run/systemd/network/10-vm-runtime.network
        # SEC-10 step 3: with the host resolver named, resolved's compiled-in
        # public fallback goes. Left in place, a fallback becomes the Global
        # scope's current server on an early lookup and stays there beside
        # the link's DNS= (docs/problems/guest-dns-host-resolver-unused-
        # 2026-10-22.md, "Observed on T"). Only this branch writes it: a guest
        # without choir.dns has no link DNS= and resolves through the fallback.
        mkdir -p /run/systemd/resolved.conf.d
        cat > /run/systemd/resolved.conf.d/10-choir-no-fallback.conf <<EOF
[Resolve]
FallbackDNS=
EOF
        # resolved is up by now (upstream it is DefaultDependencies=no and
        # Before=sysinit.target; this unit is After=sysinit.target by default,
        # so ordering it Before=resolved would be a cycle). A restart makes it
        # read the drop-in and forget a fallback it picked. Unconditional, so a
        # resolved still activating (a restart of its own) gets a restart
        # queued behind it rather than being skipped. --no-block: never wait
        # on a job from boot.
        systemctl restart --no-block systemd-resolved.service
      fi
    '';
  };

  # Extract per-VM bootstrap settings into an env file before the autoputer
  # service starts. Runtime parameters come from kernel cmdline, while the
  # gateway token is read from the persistent data volume vmmanager owns.
  systemd.services.go-choir-extract-cmdline = {
    description = "Extract go-choir secrets from kernel cmdline";
    wantedBy = [ "multi-user.target" ];
    before = [ "go-choir-updater.service" "go-choir-autoputer.service" ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
    };
    path = [ pkgs.coreutils pkgs.jq ];
    script = ''
      set -euo pipefail
      ENV_FILE="/run/go-choir-autoputer.env"
      : > "$ENV_FILE"
      UPDATER_ENV_FILE="/run/go-choir-updater.env"
      : > "$UPDATER_ENV_FILE"
      GUEST_DEPLOY_RECEIPT="/run/go-choir-guest-deploy-receipt.json"
      activated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
      jq -n \
        --arg commit "${buildCommit}" \
        --arg activated_at "$activated_at" \
        '{schema_version: 1, target_commit: $commit, activated_at: $activated_at, artifacts: {autoputer: {commit: $commit, status: "active"}}}' \
        > "$GUEST_DEPLOY_RECEIPT"
      chmod 0444 "$GUEST_DEPLOY_RECEIPT"

      # Parse kernel cmdline parameters from vmmanager.
      for param in $(cat /proc/cmdline); do
        case "$param" in
          guest_port=*)
            echo "AUTOPUTER_PORT=''${param#guest_port=}" >> "$ENV_FILE"
            ;;
          vm_id=*)
            echo "AUTOPUTER_ID=''${param#vm_id=}" >> "$ENV_FILE"
            echo "AUTOPUTER_ID=''${param#vm_id=}" >> "$UPDATER_ENV_FILE"
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
          choir.platform_url=*)
            echo "CHOIR_PLATFORM_URL=''${param#choir.platform_url=}" >> "$ENV_FILE"
            ;;
          choir.proxy_url=*)
            echo "CHOIR_PROXY_URL=''${param#choir.proxy_url=}" >> "$ENV_FILE"
            ;;
          choir.owner_id=*)
            echo "CHOIR_OWNER_ID=''${param#choir.owner_id=}" >> "$ENV_FILE"
            ;;
          choir.desktop_id=*)
            echo "CHOIR_DESKTOP_ID=''${param#choir.desktop_id=}" >> "$ENV_FILE"
            ;;
          choir.runtime_maintenance_hold=*)
            echo "RUNTIME_MAINTENANCE_HOLD=''${param#choir.runtime_maintenance_hold=}" >> "$ENV_FILE"
            ;;
          choir.runtime_recovery_replay_only=*)
            echo "RUNTIME_RECOVERY_REPLAY_ONLY=''${param#choir.runtime_recovery_replay_only=}" >> "$ENV_FILE"
            ;;
          # SEC-06: the host selects this computer for enforce
          # (vmctl's VM_CAPSULE_TUNNEL_ENFORCE_ONLY). The environment file
          # overrides the unit's observe; only the exact value is copied.
          choir.capsule_egress_tunnel_identity=enforce)
            echo "CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY=enforce" >> "$ENV_FILE"
            ;;
          # The gateway token never rides kernel argv; it arrives on the
          # root-only credential disk (S1 security-floor remainder).
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

  # Desk cell workers (management, Texture, research) run model-authored Go
  # as one unprivileged user per desk, never as the runtime's root: no
  # capabilities, no_new_privs, Landlock, a cell seccomp filter and an empty
  # network namespace (internal/yaegikernel/desk_cell_confinement*.go;
  # docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md).
  # No home, no shell, no supplementary groups. The uid and gid of each are the
  # desk manifest's (internal/deskmanifest, Sandbox.UID): the spawner uses the
  # number and never looks a user up, so these entries exist for operators
  # (ps, ls -l) and a test pins them to the manifest.
  users.groups.choir-cell-management = { gid = 61001; };
  users.groups.choir-cell-texture = { gid = 61002; };
  users.groups.choir-cell-research = { gid = 61003; };
  users.users.choir-cell-management = {
    isSystemUser = true;
    uid = 61001;
    group = "choir-cell-management";
  };
  users.users.choir-cell-texture = {
    isSystemUser = true;
    uid = 61002;
    group = "choir-cell-texture";
  };
  users.users.choir-cell-research = {
    isSystemUser = true;
    uid = 61003;
    group = "choir-cell-research";
  };
  # Document tools (pdftotext, pdfinfo, pandoc) parse untrusted bytes as this
  # user, never as root, in an empty network namespace under Landlock and
  # seccomp (SDC SEC-15, internal/doctool). The spawner uses the number; a
  # test pins it.
  users.groups.choir-doctool = { gid = 61010; };
  users.users.choir-doctool = {
    isSystemUser = true;
    uid = 61010;
    group = "choir-doctool";
  };

  systemd.tmpfiles.rules = [
    "d /mnt/persistent/choir-updater 0700 root root -"
    "d /mnt/persistent/choir-credentials 0700 root root -"
    "d /run/choir-updater-control 0700 root root -"
    "d /run/choir-runtime-handoff 0700 root root -"
    "d /run/choir 0700 root root -"
    "d /mnt/persistent/capsule-artifacts 0700 root root -"
    "d /mnt/persistent/choir-signers 0711 root root -"
    "d /mnt/persistent/choir-signers/guest-core 0700 choir-guest-signer choir-guest-signer -"
    "d /mnt/persistent/choir-signers/verifier 0700 choir-verifier-signer choir-verifier-signer -"
    "d /run/choir-signers 0711 root root -"
    "d /run/choir-signers/guest-core 0750 choir-guest-signer choir-guest-signer -"
    "d /run/choir-signer-status 0711 root root -"
    "d /run/choir-signer-status/guest-core 0750 choir-guest-signer choir-guest-signer -"
    "d /run/choir-verifier 0750 choir-verifier-signer choir-verifier-signer -"
  ];

  # Fixed privileged restart bridge. The updater may create only the trigger;
  # it has no access to PID 1's control sockets or arbitrary unit names.
  systemd.paths.go-choir-autoputer-restart = {
    description = "Watch for a verified Choir restart request";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/restart";
  };

  systemd.services.go-choir-autoputer-restart = {
    description = "Restart only the Choir autoputer service";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-autoputer-restart" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/restart
        install -m 0400 /run/choir-runtime-handoff/restart-capability /run/choir-runtime-handoff/recovery-capability
        exec ${pkgs.systemd}/bin/systemctl restart go-choir-autoputer.service
      '';
    };
  };

  systemd.paths.go-choir-autoputer-recovery = {
    description = "Watch for a verified Choir recovery restart request";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/recover";
  };

  systemd.services.go-choir-autoputer-recovery = {
    description = "Restore the prior Choir release with reserved transient credentials";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-autoputer-recovery" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/recover
        install -m 0400 /run/choir-runtime-handoff/recovery-capability /run/choir-runtime-handoff/restart-capability
        exec ${pkgs.systemd}/bin/systemctl restart go-choir-autoputer.service
      '';
    };
  };

  systemd.paths.go-choir-autoputer-recovery-cleanup = {
    description = "Watch for recovery credential cleanup";
    wantedBy = [ "multi-user.target" ];
    pathConfig.PathExists = "/run/choir-updater-control/cleanup";
  };

  systemd.services.go-choir-autoputer-recovery-cleanup = {
    description = "Delete the reserved credential after observed healthy startup";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = pkgs.writeShellScript "go-choir-autoputer-recovery-cleanup" ''
        set -euo pipefail
        rm -f /run/choir-updater-control/cleanup /run/choir-runtime-handoff/recovery-capability
      '';
    };
  };

  systemd.services.go-choir-guest-receipt-signer-state-migration = {
    description = "Normalize retained guest-core signer state ownership";
    before = [ "go-choir-guest-receipt-signer.service" ];
    onFailure = [ "go-choir-guest-receipt-signer-failure-diagnostics.service" ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
      User = "root";
      Group = "root";
      ExecStart = "${guestSignerStateMigration}/bin/choir-guest-signer-state-migrate /mnt/persistent/choir-signers/guest-core choir-guest-signer choir-guest-signer";
      StandardOutput = "journal+console";
      StandardError = "journal+console";
      ExecStartPost = "${guestSignerStateProjection}/bin/choir-guest-signer-state-project /mnt/persistent/choir-signers/guest-core/key.ed25519 /run/choir 64";
      UMask = "0077";
      NoNewPrivileges = true;
      ProtectSystem = "strict";
      ProtectHome = true;
      PrivateTmp = true;
      PrivateDevices = true;
      ReadWritePaths = [ "/mnt/persistent/choir-signers/guest-core" "/run/choir" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers/verifier" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-signers" "/run/choir-verifier" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-guest-receipt-signer-failure-diagnostics = {
    description = "Expose bounded guest-core signer migration failure evidence";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = guestSignerFailureDiagnostics;
      StandardOutput = "journal+console";
      StandardError = "journal+console";
      NoNewPrivileges = true;
      CapabilityBoundingSet = [ ];
      ProtectSystem = "strict";
      ProtectHome = true;
      PrivateTmp = true;
      PrivateDevices = true;
      InaccessiblePaths = [ "/mnt/persistent/choir-signers" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-signers" "/run/choir-verifier" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-guest-receipt-signer = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 5 starts in 10 minutes the unit stays failed until the computer is restarted from outside.
    startLimitIntervalSec = 600;
    startLimitBurst = 5;
    description = "Isolated guest-core receipt signer";
    wantedBy = [ "multi-user.target" ];
    after = [ "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer-state-migration.service" ];
    requires = [ "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer-state-migration.service" ];
    serviceConfig = {
      Type = "simple";
      User = "choir-guest-signer";
      Group = "choir-guest-signer";
      ExecStart = "${goChoirPackages.receiptSigner}/bin/choir-receipt-signer --mode guest-core --socket /run/choir-signers/guest-core/signer.sock --startup-status /run/choir-signer-status/guest-core/startup-stage --key /mnt/persistent/choir-signers/guest-core/key.ed25519 --state-root /mnt/persistent/choir-signers/guest-core/receipts";
      ExecStartPost = "${guestSignerReadiness}/bin/choir-guest-signer-ready";
      EnvironmentFile = [ "-/run/go-choir-updater.env" ];
      Restart = "on-failure";
      RestartMode = "direct";
      RestartSec = 1;
      TimeoutStartSec = 0;
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
      ReadWritePaths = [ "/mnt/persistent/choir-signers/guest-core" "/run/choir-signers/guest-core" "/run/choir-signer-status/guest-core" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers/verifier" "/run/choir-verifier" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-verifier-signer = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 5 starts in 10 minutes the unit stays failed until the computer is restarted from outside.
    startLimitIntervalSec = 600;
    startLimitBurst = 5;
    description = "Isolated verifier receipt signer";
    wantedBy = [ "multi-user.target" ];
    after = [ "go-choir-extract-cmdline.service" ];
    requires = [ "go-choir-extract-cmdline.service" ];
    serviceConfig = {
      Type = "simple";
      User = "choir-verifier-signer";
      Group = "choir-verifier-signer";
      ExecStart = "${goChoirPackages.receiptSigner}/bin/choir-receipt-signer --mode verifier-control --socket /run/choir-verifier/authority.sock --key /mnt/persistent/choir-signers/verifier/key.ed25519 --state-root /mnt/persistent/choir-signers/verifier/receipts";
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
      ReadWritePaths = [ "/mnt/persistent/choir-signers/verifier" "/run/choir-verifier" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers/guest-core" "/run/choir-signers" "/mnt/persistent/choir-updater" "/mnt/persistent/choir-credentials" "/run/choir-updater-control" "/run/choir-runtime-handoff" "/run/choir-bootstrap" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
      SystemCallFilter = [ "~@debug" ];
    };
  };

  systemd.services.go-choir-kernel-capability-probe = {
    description = "Probe mandatory guest kernel isolation capabilities";
    before = [ "go-choir-updater.service" "go-choir-autoputer.service" ];
    requiredBy = [ "go-choir-updater.service" "go-choir-autoputer.service" ];
    environment.CHOIR_KERNEL_CAPABILITY_PROBE_OUTPUT = "/run/choir/kernel-capabilities.json";
    serviceConfig = {
      Type = "oneshot";
      User = "root";
      Group = "root";
      ExecStart = "${goChoirPackages.updater}/bin/choir-updater";
      UMask = "0077";
      CapabilityBoundingSet = [ "CAP_SYS_ADMIN" "CAP_SETUID" "CAP_SETGID" "CAP_SETFCAP" ];
      AmbientCapabilities = [ "CAP_SYS_ADMIN" "CAP_SETUID" "CAP_SETGID" "CAP_SETFCAP" ];
      Delegate = true;
      PrivateTmp = true;
      PrivateDevices = true;
      ProtectHome = true;
      ProtectSystem = "strict";
      ProtectControlGroups = false;
      ReadWritePaths = [ "/run/choir" ];
      RestrictAddressFamilies = [ "AF_UNIX" ];
      LockPersonality = true;
      StandardOutput = "journal+console";
      StandardError = "journal+console";
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

  # Periodic trim of the data volume (docs/problems/guest-data-image-never-
  # trims-2026-10-20.md). data.img is a sparse file on the host's btrfs: the
  # host keeps every block the guest ever wrote until the guest trims it, and
  # Dolt rewrites about 5 GB an hour. A timer, not the `discard` mount option:
  # continuous discard adds latency to every Dolt file delete. Hourly, because
  # a daily trim lets the image grow tens of GB between runs.
  #
  # Inert until the VMM offers virtio-blk discard (Firecracker >= 1.17.0 with
  # VM_DATA_DISCARD on, vmmanager/data_discard.go): without the feature the
  # device reports discard_max_bytes 0 and the service logs it and exits 0, so
  # no failed unit appears on a guest whose VMM cannot discard.
  systemd.services.choir-data-fstrim = {
    description = "Trim the guest data volume so the host can reclaim freed blocks";
    unitConfig.RequiresMountsFor = "/mnt/persistent";
    path = [ pkgs.util-linux pkgs.coreutils ];
    serviceConfig = {
      Type = "oneshot";
      Nice = 19;
      IOSchedulingClass = "idle";
      TimeoutStartSec = "30min";
    };
    script = ''
      set -eu
      src="$(findmnt -n -o SOURCE --target /mnt/persistent)"
      dev="$(basename "$(readlink -f "$src")")"
      max="/sys/class/block/$dev/queue/discard_max_bytes"
      if [ ! -r "$max" ] || [ "$(cat "$max")" = 0 ]; then
        echo "choir-data-fstrim: $dev offers no discard; nothing to trim"
        exit 0
      fi
      fstrim -v /mnt/persistent
    '';
  };

  systemd.timers.choir-data-fstrim = {
    description = "Hourly trim of the guest data volume";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      # First run after boot lets a freshly refreshed computer shrink what
      # earlier churn left; then hourly from the last run.
      OnBootSec = "10min";
      OnUnitActiveSec = "1h";
      RandomizedDelaySec = "5min";
      Unit = "choir-data-fstrim.service";
    };
  };

  systemd.services.go-choir-updater = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 5 starts in 10 minutes the unit stays failed until the computer is restarted from outside.
    startLimitIntervalSec = 600;
    startLimitBurst = 5;
    description = "Choir guest release updater";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer.service" ];
    before = [ "go-choir-autoputer.service" ];
    wants = [ "network-online.target" ];
    requires = [ "go-choir-extract-cmdline.service" "go-choir-guest-receipt-signer.service" ];
    environment.CHOIR_UPDATER_ROOT = "/mnt/persistent/choir-updater";
    environment.CHOIR_GUEST_IMAGE_MANIFEST = guestImageManifest;
    environment.CHOIR_KERNEL_CONFIG = config.boot.kernelPackages.kernel.configfile;
    environment.CHOIR_STORE_SCHEMA_PATH = "/mnt/persistent/state.texture/store-schema.json";
    serviceConfig = {
      Type = "simple";
      User = "root";
      Group = "root";
      ExecStartPre = "${pkgs.coreutils}/bin/install -m 0600 /dev/null /run/choir/updater-unit-entered";
      ExecStart = "${goChoirPackages.updater}/bin/choir-updater --root /mnt/persistent/choir-updater --socket /run/choir/updater.sock --restart-request /run/choir-updater-control/restart --recovery-restart-request /run/choir-updater-control/recover --recovery-cleanup-request /run/choir-updater-control/cleanup --restart-prepare-url http://127.0.0.1:8085/internal/self-development/restart-handoff --health-url http://127.0.0.1:8085/health --signer-socket /run/choir-signers/guest-core/signer.sock --guest-image-manifest ${guestImageManifest} --kernel-config ${config.boot.kernelPackages.kernel.configfile}";
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
      InaccessiblePaths = [ "/mnt/persistent/choir-signers" "/run/choir-verifier" "/mnt/persistent/choir-credentials" "/run/choir-bootstrap" "/run/choir-runtime-handoff" "/run/go-choir-autoputer.env" "/run/systemd/private" "/run/dbus/system_bus_socket" ];
      RestrictAddressFamilies = [ "AF_UNIX" "AF_INET" ];
      LockPersonality = true;
      RestrictSUIDSGID = true;
    };
  };

  # Autoputer runtime service.
  # Runs the Go autoputer binary which listens for runtime API requests
  # inside the VM. Provider credentials are never in the guest (VAL-VM-011).
  # LLM calls route through the host-side gateway using the extracted token.
  systemd.services.go-choir-autoputer = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 5 starts in 10 minutes the unit stays failed until the computer is restarted from outside.
    startLimitIntervalSec = 600;
    startLimitBurst = 5;
    description = "go-choir Autoputer Runtime (VM guest)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-extract-cmdline.service" "run-choir\\x2dbootstrap.mount" "go-choir-guest-receipt-signer.service" "go-choir-verifier-signer.service" "go-choir-updater.service" ];
    wants = [ "network-online.target" "go-choir-updater.service" ];
    requires = [ "go-choir-extract-cmdline.service" "run-choir\\x2dbootstrap.mount" "go-choir-guest-receipt-signer.service" "go-choir-verifier-signer.service" ];
    bindsTo = [ "go-choir-guest-receipt-signer.service" ];
    environment = {
      CHOIR_COMPUTER_CREDENTIAL_FILE = "/run/choir-bootstrap/computer-event-envelope";
      CHOIR_REVOCATION_CREDENTIAL_HANDOFF = "/run/choir-runtime-handoff/revocation-capability";
      CHOIR_RESTART_CREDENTIAL_HANDOFF = "/run/choir-runtime-handoff/restart-capability";
      CHOIR_VERIFIER_AUTHORITY_SOCKET = "/run/choir-verifier/authority.sock";
      CHOIR_PRIVACY_KEY_FILE = "/mnt/persistent/choir-credentials/privacy-key";
      CHOIR_GUEST_IMAGE_MANIFEST = guestImageManifest;
      CHOIR_DEPLOY_RECEIPT_PATH = "/run/go-choir-guest-deploy-receipt.json";
      CHOIR_KERNEL_CONFIG = config.boot.kernelPackages.kernel.configfile;
      CHOIR_GUEST_SIGNER_SOCKET = "/run/choir-signers/guest-core/signer.sock";
      # Desk cell workers run confined or not at all: their desk's user,
      # mandatory Landlock V5, seccomp, private network namespace.
      CHOIR_DESK_CELL_CONFINEMENT = "required";
      # The desks cgroup (SDC RES-01, RES-02): each desk worker runs in its own
      # child with a hard memory.max from its desk's manifest row, under an
      # aggregate the daemon sets at start (internal/autoputer/desk_cgroup.go).
      # Required mode refuses to start a worker without it.
      CHOIR_DESK_CGROUP_ROOT = "/sys/fs/cgroup/choir-desks";
      # Capsule memory per doctrine (SDC RES-04, internal/capsule/capsule_memory.go):
      # observe keeps memory.max = the capsule's size and admission against it,
      # and records each capsule's memory.peak and memory.events with what
      # doctrine (memory.high = request, memory.max = 2x) would have done.
      # "doctrine" applies it, after the receipts show builds fit.
      CHOIR_CAPSULE_MEMORY_POLICY = "observe";
      # Egress tunnel identity (SDC SEC-06, internal/capsule/egress_tls.go):
      # observe judges each CONNECT tunnel's first TLS record (a ClientHello
      # whose only server_name is the CONNECT host), writes the verdict on the
      # capsule's egress ledger and relays it. "enforce" refuses, after the
      # gate in docs/problems/egress-proxy-tunnel-identity-2026-10-21.md. A
      # computer the host selects (vmctl's VM_CAPSULE_TUNNEL_ENFORCE_ONLY) is
      # set to enforce by the boot script's environment file, which overrides
      # this line.
      CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY = "observe";
      # The document-tool sandbox (SDC SEC-15, internal/doctool): observe runs
      # each tool as before (that is the result) and then a sandboxed shadow of
      # it (uid 61010, empty network namespace, Landlock, seccomp, in its own
      # cgroup under the document tools' root) whose outcome is only counted.
      # "required" runs sandboxed in its own cgroup or refuses.
      CHOIR_DOCUMENT_TOOL_SANDBOX = "observe";
      # The document tools' own cgroup root (never the desks'): required runs,
      # observe's shadows and the isolation self-test each run in a child of it.
      CHOIR_DOCTOOL_CGROUP_ROOT = "/sys/fs/cgroup/choir-doctools";
      # The daemon's capability floor (SDC SEC-15, internal/capfloor): it keeps
      # 16 named capabilities and drops the rest from its bounding set.
      # observe logs what it would drop and the isolation self-test reports
      # the set; "enforce" drops it (one re-exec at start) after a test
      # computer has run a capsule and every desk under enforce.
      CHOIR_RUNTIME_CAPABILITY_FLOOR = "observe";
      PATH = lib.mkForce (lib.makeBinPath (with pkgs; [
        busybox
        bash
        coreutils
        curl
        findutils
        git
        gnugrep
        gnumake
        gnused
        choirGo # Go toolchain lane (flake.nix choirGo)
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
      ]));
      # VM health checks and host forwarding reach the guest via its tap IP,
      # so the autoputer must listen on all guest interfaces, not loopback only.
      SERVER_HOST = "0.0.0.0";
      # Default port; overridden by guest_port= in kernel cmdline.
      AUTOPUTER_PORT = "8085";
      AUTOPUTER_ID = "autoputer-guest";
      # Persistent state directory on the virtio-blk data volume.
      RUNTIME_STORE_PATH = "/mnt/persistent/state";
      # Files app data must use the same persistent data volume. Without this
      # the autoputer falls back to its process-local default, which can disappear
      # across guest reboot/recovery even when runtime DB state survives.
      AUTOPUTER_FILES_ROOT = "/mnt/persistent/files";
      # Guest-local capsule verification uses the standard Go toolchain; expose
      # the Nix ICU development closure through standard compiler variables.
      CGO_CFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_CXXFLAGS = "-I${pkgs.icu.dev}/include";
      CGO_LDFLAGS = "-L${pkgs.icu}/lib";
      PKG_CONFIG_PATH = "${pkgs.icu.dev}/lib/pkgconfig";
      # Engineering capsules build cmd/autoputer, which needs cgo (gozstd,
      # go-icu-regex). The capsule executor appends these bin dirs to every
      # capsule workload's PATH and hands it the four cgo variables above;
      # the store is read-only through the capsule's lower root
      # (docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option B).
      CHOIR_CAPSULE_BUILD_PATH = lib.makeBinPath (with pkgs; [ gcc binutils pkg-config ]);
      # The frontend's locked dependencies, carried to capsule workloads like
      # the toolchain above, so a frontend build whose package files match
      # needs no package registry (docs/design/delegate-one-round-build-2026-10-21.md).
      CHOIR_FRONTEND_DEPS = "${goChoirPackages.frontendDeps}";
      RUNTIME_SKILLS_ROOT = "${goChoirPackages.autoputer}/share/go-choir/skills";
      CHOIR_BASELINE_RELEASE_ROOT = "${goChoirPackages.autoputer}";
      # Autoputer identity token: read through the file on the root-only
      # credential disk, never inherited via env by children.
      RUNTIME_GATEWAY_TOKEN_FILE = "/run/choir-bootstrap/gateway-token";
      CHOIR_CAPSULE_BROKER_PATH = "${goChoirPackages.capsuleBroker}/bin/capsule-broker";
      # claude delegate: the binary a claude-mode capsule's broker starts.
      # The token is not here; it is on the credential disk of the one
      # allowlisted computer only.
      CHOIR_CLAUDE_CODE_BIN = "${claudeCode}/bin/claude";
      CHOIR_CLAUDE_DELEGATE_CREDENTIAL_DIR = "/run/choir-bootstrap";
      # visual capture: the Chromium and fontconfig a granted computer's
      # capture jobs use. The grant itself is on the credential disk
      # (visual-capture.json), written by vmctl for granted computers only.
      CHOIR_CAPTURE_BROWSER_BIN = "${captureBrowser}/bin/chromium";
      CHOIR_CAPTURE_FONTCONFIG_FILE = "${captureFontsConf}";
      CHOIR_VISUAL_CAPTURE_CREDENTIAL_DIR = "/run/choir-bootstrap";
      # Capsule overlay uppers and /tmp live on the data volume: package
      # installs do not fit in guest RAM. The executor scopes it per boot.
      CHOIR_CAPSULE_STATE_DIR = "/mnt/persistent/capsule-state";
      CHOIR_CAPSULE_ARTIFACT_DIR = "/mnt/persistent/capsule-artifacts";
      CHOIR_CAPSULE_SOURCE_ROOT = "/mnt/persistent/files/Source/platform";
      # U1b: where the guest fetches the booted base's objects from when its
      # source repository (seeded once, at creation) lacks them. It does
      # nothing unless the source.projection setting (or
      # CHOIR_SOURCE_PROJECTION) is on. Empty, or CHOIR_SOURCE_BASE_FETCH=off in
      # /run/go-choir-autoputer.env, turns the fetch off.
      CHOIR_SOURCE_BASE_REMOTE = sourceRepoRemote;
      CHOIR_CAPSULE_LOWER_ROOT = "/";
      GIT_AUTHOR_NAME = "Choir Capsule";
      GIT_AUTHOR_EMAIL = "capsule@choir.local";
      GIT_COMMITTER_NAME = "Choir Capsule";
      GIT_COMMITTER_EMAIL = "capsule@choir.local";
      # Keep Go build/module caches on the writable data volume.
      GOPATH = "/mnt/persistent/go";
      GOMODCACHE = "/mnt/persistent/go/pkg/mod";
      GOCACHE = "/mnt/persistent/go-build-cache";
      GOTOOLCHAIN = "local";
      # Explicit runtime-selected model. Provider credentials remain host-side;
      # guest LLM calls route through the gateway token above.
      RUNTIME_LLM_PROVIDER = "chatgpt";
      RUNTIME_LLM_MODEL = "gpt-6-luna";
      RUNTIME_LLM_REASONING_EFFORT = "low";
    };
    serviceConfig = {
      ExecStartPre = "";
      ExecStart = "${autoputerRuntimeExec}";
      Restart = "on-failure";
      RestartSec = 1;
      # Updater and capsule-adjacent child work can approach the guest memory
      # envelope. Keep the runtime alive so it can persist terminal evidence.
      OOMPolicy = "continue";
      # RES-01: the daemon's pages are protected from reclaim by the desk
      # workers and capsules around it. 512M is a first value (the paper's
      # daemon measured Sys 1.46 GB, HeapInuse 1.05 GB); L1-02's measurement
      # sets the real one. A reservation only binds when used.
      MemoryMin = "512M";
      StandardOutput = "journal+console";
      StandardError = "journal+console";
      EnvironmentFile = [ "-/run/go-choir-autoputer.env" ];
      ReadWritePaths = [ "/mnt/persistent" "/run/choir" "/run/choir-runtime-handoff" "/run/choir-verifier" ];
      InaccessiblePaths = [ "/mnt/persistent/choir-signers" "/run/choir-updater-control" ];
    };
  };

  # Allow autoputer port through firewall
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
    choirGo # Go toolchain lane (flake.nix choirGo)
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
    # L2 ecosystem egress (engineering-network-grants-2026-10-10.md §4.0b):
    # implementation capsules install Python packages with pip or uv.
    uv
    # claude delegate: the cell-side client (it holds no credential; in a
    # capsule without the delegate it reports "unavailable").
    goChoirPackages.choirDelegate
    # visual capture: the capsule job's tool (it holds no browser path; without
    # the grant it exits 3, "not granted").
    goChoirPackages.choirCapture
  ];

  # claude delegate: admit exactly one unfree package.
  nixpkgs.config.allowUnfreePredicate = pkg: builtins.elem (lib.getName pkg) [ "claude-code" ];

  system.stateVersion = "25.11";
}
