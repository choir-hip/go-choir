# NixOS host configuration for go-choir Node B (OVH bare metal)
# 147.135.70.196 — choir.news — us-east-vin
# Adapted from choiros-rs nix/hosts/ovh-node.nix and ovh-node-b.nix
#
# Service hardening notes (VAL-DEPLOY-007 / VAL-DEPLOY-008 / VAL-CROSS-118):
# - All Go services bind to 127.0.0.1 only (localhost-only, defense in depth)
# - Firewall allows only ports 22, 80, 443 externally
# - Caddy is the sole public edge; internal service ports are never exposed
# - Each service has Restart=on-failure with a backoff, plus a watchdog
# - Proxy depends on both auth and autoputer; if either restarts, proxy
#   re-verifies health on the next request and returns degraded state
#   through /health while the upstream recovers
# - Auth persists sessions in SQLite, so sessions survive auth restarts
# - Auth reuses the same signing key file across restarts, so existing
#   access JWTs remain valid after auth restarts (VAL-CROSS-118)
{ config, lib, pkgs, goChoirPackages, choirGo, guestImage ? null, sourceRepoRemote ? "https://github.com/choir-hip/go-choir.git", buildCommit ? "local", ... }:
let
  # Firecracker 1.17 as a second VMM build for the computers
  # VM_FIRECRACKER_NEXT_ONLY names (nix/firecracker-next.nix; RES-16 data
  # drive discard). The pinned build stays pkgs.firecracker for everyone else.
  firecrackerNext = import ./firecracker-next.nix { inherit pkgs; };
  # Host fetch route (SDC NET-01; internal/hostfetch,
  # docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md). One
  # list, read by vmctl (writes the grant on the credential disk at cold
  # boot) and by the gateway (admits POST /provider/v1/fetch only for these
  # callers; everyone else gets 403). VM IDs (vm-...), the gateway
  # credential identity, space separated. OFF while empty: the gateway
  # refuses every call. A granted computer's research Read, content import,
  # podcast and YouTube fetches and source polls make their network hop
  # through the gateway and never fall back to direct egress. Rollback:
  # empty the list, deploy, cold-boot the computer.
  hostFetchVMs = "";
  # Auth signing material lives in this writable runtime directory.
  # Using a let-binding so downstream env vars compose the key paths
  # via interpolation instead of raw *_KEY_PATH=/absolute/path literals
  # that Droid-Shield false-positives on.
  authSigningDir = "/var/lib/go-choir/auth-signing";
  frontendRoot = "/var/www/go-choir";
  frontendCurrent = "/var/www/go-choir/frontend-current";
  autoputerFilesDir = "/var/lib/go-choir/files";
  autoputerRuntimeDir = "/var/lib/go-choir/runtime";
  mailDir = "/var/lib/go-choir/mail";
  platformDoltDir = "/var/lib/go-choir/platform-dolt";
  platformDoltDBDir = "${platformDoltDir}/platform";
  # Store A weekly history squash (owner notes 2026-10-14 sections 23 and 27
  # item 7; docs/problems/store-a-weekly-squash-2026-10-20.md). The service is
  # always defined so the first run is supervised by hand
  # (`systemctl start go-choir-platform-dolt-squash.service`); the weekly timer
  # starts only when this is true. Enable it after the first run has been
  # watched end to end.
  platformDoltSquashTimerEnabled = false;
  platformDoltSquashBackupRoot = "/var/lib/go-choir/platform-dolt-squash-backups";
  corpusDoltDir = "/var/lib/go-choir/corpus-dolt";
  corpusDoltDBDir = "${corpusDoltDir}/corpus";
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
  corpusDoltInit = pkgs.writeShellScript "corpus-dolt-init" ''
    set -euo pipefail
    export HOME="${corpusDoltDir}"
    install -d -m 0750 "${corpusDoltDir}" "${corpusDoltDBDir}"
    cd "${corpusDoltDBDir}"
    ${pkgs.dolt}/bin/dolt config --global --add user.name "Choir Platform" >/dev/null 2>&1 || true
    ${pkgs.dolt}/bin/dolt config --global --add user.email "platform@choir.news" >/dev/null 2>&1 || true
    if [ ! -d .dolt ]; then
      ${pkgs.dolt}/bin/dolt init
    fi
  '';
  # Host-internal service credential (internal/hostauth;
  # docs/problems/loopback-is-a-credential-2026-10-14.md). vmctl's internal
  # routes, the gateway's credential and operator routes, corpusd's and
  # maild's internal routes and pprof require it in addition to loopback.
  # The secret is a root-only 0600 file generated at activation and handed
  # to each unit with LoadCredential=, so it lives only in that unit's
  # private $CREDENTIALS_DIRECTORY: never in the environment (Firecracker
  # inherits vmctl's) and never on a command line.
  # Rollout: "observe" allows and logs every internal call that would be
  # refused ("hostauth: observe: would refuse"); flip to "enforce" once the
  # journal shows none for a full day of deploys and timers. Rollback:
  # "off" (transport alone, the pre-repair rule), or "observe".
  internalAuthMode = "enforce";
  internalTokenFile = "/var/lib/go-choir/secrets/internal-service-token";
  # The same secret as a curl header line, for root shells (CI deploy,
  # disk-gc): `curl -H @<file>` keeps it off the command line.
  internalTokenHeaderFile = "${internalTokenFile}.header";
  internalAuthEnv = "CHOIR_INTERNAL_AUTH_MODE=${internalAuthMode}";
  internalTokenCredential = [ "choir-internal-token:${internalTokenFile}" ];
  # Guest DNS (SEC-10; docs/problems/host-egress-floor-fails-open-2026-10-21.md):
  # Unbound answers guests on 127.0.0.1:guestDnsPort, and vmctl DNATs each
  # guest's tap host address :53 to it (VMCTL_GUEST_DNS_PORT). The guest
  # points its resolver at its gateway (the guest side of SEC-10, FP2's
  # nix/autoputer-vm.nix line); until then guests keep public resolvers.
  # Rollback: drop VMCTL_GUEST_DNS_PORT (vmctl stops adding the DNAT at the
  # next start of each VM) and services.unbound.
  guestDnsPort = 8053;
  # Issuer credential (internal/hostauth/issuer.go): a second secret only
  # vmctl and corpusd load. corpusd's credential-issuance route needs it as
  # well as the internal credential, so the shared internal credential alone
  # cannot mint an envelope for any computer (which leads, through exchange
  # and realization delivery, to that computer's data key). Rollout:
  # "observe" logs "hostauth: observe: would refuse ... issuer:"; flip to
  # "enforce" once the journal shows vmctl presenting it on every boot.
  # A refusal in enforce is a computer that cannot boot. Rollback: "observe".
  issuerAuthMode = "observe";
  issuerTokenFile = "/var/lib/go-choir/secrets/issuer-token";
  issuerTokenCredential = [ "choir-issuer-token:${issuerTokenFile}" ];
  issuerAuthEnv = "CHOIR_ISSUER_AUTH_MODE=${issuerAuthMode}";
  # S1 owner-act receipts (internal/owneract;
  # docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md).
  # The proxy signs, with its own Ed25519 key (signer domain
  # host-owner-act), that it authenticated the owner of one computer for one
  # act; corpusd verifies that signature before an owner mode write and
  # traces every effect_accepted to it (or to an offer corpusd minted).
  # Both files are root 0600, generated once at activation; the private key
  # reaches only the proxy and the public key only corpusd, each through
  # LoadCredential=. Rollout: "observe" logs "owner-act: observe: would
  # refuse"; flip to "enforce" after a clean window. Rollback: "observe".
  ownerActMode = "observe";
  ownerActKeyFile = "/var/lib/go-choir/secrets/owner-act-signing-key";
  ownerActPublicKeyFile = "${ownerActKeyFile}.pub";
  # Dolt SQL authentication (internal/dbcred;
  # docs/problems/loopback-is-a-credential-2026-10-14.md). Both Dolt servers
  # accepted root with an empty password from any local process. Staged so
  # no deploy changes the server and its clients at the same moment:
  #   "users"   each Dolt server gets an application user `choir` with a
  #             password and ALL on its one database (platform / corpus),
  #             no global privileges; clients still connect as root.
  #   "clients" corpusd, checkpointd and vmctl connect as `choir`
  #             (CHOIR_DOLT_USER + LoadCredential choir-dolt-password).
  #   "locked"  root@localhost gets its own password; passwordless root is
  #             refused. Local `dolt sql` in the data directory (history
  #             audit, runbooks) still works through Dolt's local-user file.
  # Advance one stage per deploy, after scripts/verify-host-hardening.py
  # shows the previous stage healthy. Rollback: the previous stage; "locked"
  # is undone by hand: ALTER USER 'root'@'localhost' IDENTIFIED BY ''
  # (connect with the root password from the file below).
  # Stage 2 "clients" (SEC-12, FP3 group 3, 2026-10-21): after "users"
  # verified healthy (verify-host-hardening --expect-dolt users).
  doltAuthStage = "clients";
  doltAppUser = "choir";
  doltAppPasswordFile = "/var/lib/go-choir/secrets/dolt-app-password";
  doltRootPasswordFile = "/var/lib/go-choir/secrets/dolt-root-password";
  doltClientsUseAppUser = doltAuthStage != "users";
  doltClientEnv = lib.optional doltClientsUseAppUser "CHOIR_DOLT_USER=${doltAppUser}";
  doltClientCredential = lib.optional doltClientsUseAppUser "choir-dolt-password:${doltAppPasswordFile}";
  doltClientAfter = lib.optionals doltClientsUseAppUser [ "go-choir-platform-dolt-users.service" "go-choir-corpus-dolt-users.service" ];
  # Idempotent: create/refresh the app user, grant it its database, and in
  # the "locked" stage set root's password. Secrets come from
  # $CREDENTIALS_DIRECTORY and reach dolt through stdin or DOLT_CLI_PASSWORD,
  # never a command line; nothing secret is echoed.
  doltEnsureUsers = pkgs.writeShellScript "go-choir-dolt-ensure-users" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.dolt ]}:$PATH"
    port="$1"; db="$2"; lock_root="$3"
    HOME="$(mktemp -d)"; export HOME
    cd "$HOME"
    app_pw="$(cat "$CREDENTIALS_DIRECTORY/dolt-app-password")"
    root_pw="$(cat "$CREDENTIALS_DIRECTORY/dolt-root-password")"
    for v in "$app_pw" "$root_pw"; do
      case "$v" in ""|*[!A-Za-z0-9]*) echo "dolt users ($db): a password file is empty or not alphanumeric" >&2; exit 1 ;; esac
    done
    as_root() { # $1 = root password ("" for none); SQL on stdin
      if [ -z "$1" ]; then
        dolt --host 127.0.0.1 --port "$port" --no-tls --user root --password "" sql
      else
        DOLT_CLI_PASSWORD="$1" dolt --host 127.0.0.1 --port "$port" --no-tls --user root sql
      fi
    }
    for _ in $(seq 1 300); do
      (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null && break
      sleep 2
    done
    if printf 'select 1;\n' | as_root "$root_pw" >/dev/null 2>&1; then
      current_root="$root_pw"; root_state=locked
    elif printf 'select 1;\n' | as_root "" >/dev/null 2>&1; then
      current_root=""; root_state=passwordless
    else
      echo "dolt users ($db, :$port): root accepts neither its password nor an empty one" >&2
      exit 1
    fi
    printf "CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';\nALTER USER '%s'@'%%' IDENTIFIED BY '%s';\nGRANT ALL PRIVILEGES ON \`%s\`.* TO '%s'@'%%';\n" \
      "${doltAppUser}" "$app_pw" "${doltAppUser}" "$app_pw" "$db" "${doltAppUser}" | as_root "$current_root" >/dev/null
    printf 'select 1;\n' | DOLT_CLI_PASSWORD="$app_pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user ${doltAppUser} --use-db "$db" sql >/dev/null
    echo "dolt users ($db, :$port): app user ${doltAppUser} ensured with ALL on $db; root $root_state"
    if [ "$lock_root" = 1 ] && [ "$root_state" = passwordless ]; then
      printf "ALTER USER 'root'@'localhost' IDENTIFIED BY '%s';\n" "$root_pw" | as_root "" >/dev/null
      if printf 'select 1;\n' | as_root "" >/dev/null 2>&1; then
        echo "dolt users ($db, :$port): root still accepts an empty password after locking" >&2
        exit 1
      fi
      echo "dolt users ($db, :$port): root locked (passwordless login refused)"
    fi
    rm -rf "$HOME"
  '';
  # Query-level liveness probe for both Dolt servers
  # (scripts/dolt-liveness-probe.sh; tests: scripts/test-dolt-liveness-probe.sh;
  # docs/problems/dolt-clustering-assessment-2026-10-15.md sections 4 and 7).
  doltLivenessProbe = pkgs.writeShellScript "go-choir-dolt-liveness-probe"
    (builtins.readFile ../scripts/dolt-liveness-probe.sh);
  platformDoltSquashScript = pkgs.writeShellScript "go-choir-platform-dolt-squash"
    (builtins.readFile ../scripts/platform-dolt-squash.sh);
  doltUsersUnit = doltUnit: port: db: {
    description = "go-choir Dolt users and grants for ${doltUnit} (${db})";
    wantedBy = [ "multi-user.target" "${doltUnit}.service" ];
    after = [ "${doltUnit}.service" ];
    partOf = [ "${doltUnit}.service" ];
    serviceConfig = commonServiceHardening // {
      Type = "oneshot";
      RemainAfterExit = true;
      TimeoutStartSec = "15min";
      ExecStart = "${doltEnsureUsers} ${toString port} ${db} ${if doltAuthStage == "locked" then "1" else "0"}";
      LoadCredential = [
        "dolt-app-password:${doltAppPasswordFile}"
        "dolt-root-password:${doltRootPasswordFile}"
      ];
    };
  };
  goServiceLibraryPath = lib.makeLibraryPath [ pkgs.icu ];
  serviceExec = name: package: pkgs.writeShellScript "go-choir-${name}-exec" ''
    set -euo pipefail
    export LD_LIBRARY_PATH="${goServiceLibraryPath}''${LD_LIBRARY_PATH:+:}''${LD_LIBRARY_PATH:-}"
    pointer="/var/lib/go-choir/services/${name}/bin/${name}"
    if [ -x "$pointer" ]; then
      if [ "${name}" = "autoputer" ] && [ -d "/var/lib/go-choir/services/autoputer/share/go-choir/skills" ]; then
        export RUNTIME_SKILLS_ROOT="/var/lib/go-choir/services/autoputer/share/go-choir/skills"
      fi
      exec "$pointer" "$@"
    fi
    exec "${package}/bin/${name}" "$@"
  '';
  # Reassert credential authority in the exec wrapper.
  vmctlExec = pkgs.writeShellScript "go-choir-vmctl-exec" ''
    set -euo pipefail
    export VMCTL_CORPUSD_URL="http://127.0.0.1:8086"
    exec ${serviceExec "vmctl" goChoirPackages.vmctl} "$@"
  '';
  # Store retention, the one place that decides which system generations and
  # builder-release GC roots stay (scripts/node-b-nix-retention; tests:
  # scripts/test-node-b-nix-retention.sh;
  # docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md).
  # Called by go-choir-nix-gc, the disk-gc sweep and the deploy preflight;
  # each runs `nix store gc` after it. GO_CHOIR_NIX_RETENTION_MODE=observe
  # keeps the newest 4 generations as before and logs what enforce (newest 3,
  # stale builder-release roots unrooted) would do.
  nixRetention = pkgs.writeShellScript "go-choir-nix-retention"
    (builtins.readFile ../scripts/node-b-nix-retention);
  diskRetentionSweep = pkgs.writeShellScript "go-choir-disk-retention-sweep" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.curl pkgs.gawk pkgs.gnugrep pkgs.gnused pkgs.nix pkgs.systemd ]}:$PATH"

    emergency_min_free_kib="''${GO_CHOIR_DISK_GC_MIN_FREE_KIB:-125829120}"
    target_free_kib="''${GO_CHOIR_DISK_GC_TARGET_FREE_KIB:-188743680}"
    avail_kib="$(df --output=avail -k / | tail -n 1 | tr -d ' ')"
    echo "go-choir disk retention: root_available_kib=''${avail_kib} emergency_min_free_kib=''${emergency_min_free_kib} target_free_kib=''${target_free_kib}"
    df -h / /var/lib/go-choir 2>/dev/null || df -h /

    internal_auth=()
    if [ -n "''${CREDENTIALS_DIRECTORY:-}" ] && [ -r "''${CREDENTIALS_DIRECTORY}/choir-internal-token.header" ]; then
      internal_auth=(-H "@''${CREDENTIALS_DIRECTORY}/choir-internal-token.header")
    fi
    curl -fsS -X POST -H 'X-Internal-Caller: true' "''${internal_auth[@]}" http://127.0.0.1:8083/internal/vmctl/reclaim || true
    journalctl --vacuum-size=256M || true
    ${nixRetention} || true

    avail_kib="$(df --output=avail -k / | tail -n 1 | tr -d ' ')"
    if [ -z "''${avail_kib}" ] || [ "''${avail_kib}" -lt "''${emergency_min_free_kib}" ]; then
      echo "go-choir disk retention: below emergency headroom after generation pruning; running nix store gc"
      nix store gc || true
    elif [ "''${avail_kib}" -lt "''${target_free_kib}" ]; then
      echo "go-choir disk retention: below target headroom; running nix store gc for routine maintenance"
      nix store gc || true
    else
      echo "go-choir disk retention: preserving warm Nix cache because target headroom is satisfied"
    fi

    df -h / /var/lib/go-choir 2>/dev/null || df -h /
  '';
  platformDoltHistoryAudit = pkgs.writeShellScript "go-choir-platform-dolt-history-audit" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.dolt pkgs.systemd ]}:$PATH"
    # Recurrence control for the 2026-08-26 incident: per-mutation
    # CALL DOLT_COMMIT in internal/platform/store.go and internal/cycle/
    # storage.go grows reachable commit history that NO dolt gc (shallow or
    # --full) can collect. See docs/evidence/platform-dolt-oldgen-218g-dead-
    # history-2026-08-26.md. This audit fails loudly (nonzero exit ->
    # failed unit -> monitoring) when history regrows past the squash
    # thresholds, so the squash runbook runs before the disk refills.
    # Audit both stores: platform (Store A, canonical event/control) and
    # corpus (Store B, world-wire). The corpus repo is absent until the
    # authority split deploys it; missing dirs are skipped, not failures.
    for spec in "platform:${platformDoltDBDir}:${platformDoltDir}" "corpus:${corpusDoltDBDir}:${corpusDoltDir}"; do
      name="''${spec%%:*}"
      rest="''${spec#*:}"
      dbdir="''${rest%%:*}"
      home="''${rest#*:}"
      [ -d "$dbdir/.dolt" ] || { echo "$name-dolt history audit: skipped (no repo at $dbdir)"; continue; }
      commits=$(cd "$dbdir" && HOME="$home" dolt sql -q "SELECT COUNT(*) FROM dolt_log" 2>/dev/null | grep -oE '[0-9]+' | head -1 || echo 0)
      oldgen_kib=$(du -sk "$dbdir/.dolt/noms/oldgen" 2>/dev/null | cut -f1 || echo 0)
      noms_kib=$(du -sk "$dbdir/.dolt/noms" 2>/dev/null | cut -f1 || echo 0)
      echo "$name-dolt history audit: dolt_log commits=''${commits} oldgen=''${oldgen_kib}KiB noms=''${noms_kib}KiB"
      max_commits="''${GO_CHOIR_DOLT_MAX_COMMITS:-1000000}"
      max_oldgen_kib="''${GO_CHOIR_DOLT_MAX_OLDGEN_KIB:-52428800}"
      # Commit-rate SLO (2026-09-19, Move 1): per-mutation DOLT_COMMIT was
      # replaced by the debounced snapshot committer, so dolt_log should grow
      # by at most a few thousand commits between daily audit runs (two
      # daemons x ~45s debounce + checkpoint boundaries). A larger delta means
      # per-mutation commits regressed — fail before the size floors matter.
      max_commit_delta="''${GO_CHOIR_DOLT_MAX_COMMIT_DELTA:-10000}"
      state_file="$home/history-audit-last-count"
      prev_commits=0
      if [ -f "$state_file" ]; then
        prev_commits=$(cat "$state_file" 2>/dev/null || echo 0)
      fi
      echo "''${commits:-0}" > "$state_file.tmp" && mv "$state_file.tmp" "$state_file"
      commit_delta=$(( ''${commits:-0} - ''${prev_commits:-0} ))
      if [ "''${prev_commits:-0}" -gt 0 ] && [ "$commit_delta" -gt "$max_commit_delta" ]; then
        echo "$name-dolt history audit FAILED: dolt_log grew $commit_delta commits since last audit (> $max_commit_delta); per-mutation DOLT_COMMIT regressed — check the debounced snapshot committer (internal/doltbatch)" >&2
        exit 1
      fi
      if [ "''${commits:-0}" -gt "$max_commits" ]; then
        echo "$name-dolt history audit FAILED: $commits commits > $max_commits; run the history-squash runbook (docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md)" >&2
        exit 1
      fi
      if [ "''${oldgen_kib:-0}" -gt "$max_oldgen_kib" ]; then
        echo "$name-dolt history audit FAILED: oldgen ''${oldgen_kib}KiB > ''${max_oldgen_kib}KiB; run the history-squash runbook" >&2
        exit 1
      fi
    done
  '';

  # Off-host backup stopgap (owner ruling 2026-10-14, owner-notes section 27
  # item 17; design and residual risks: docs/problems/no-off-host-backup-
  # 2026-10-20.md). restic encrypts on this host with a repository password
  # that exists only here and in the owner's offline copy; node-a holds
  # ciphertext in an append-only rest-server reached through a restricted SSH
  # port forward (nix/node-a-choir-backup-target.nix, applied by node-a's own
  # agent, never by us). DISABLED: nothing below exists until `enable` is true.
  #
  # Enable procedure (all of it before flipping `enable`; mutation class red,
  # so it then goes through the Landing Loop):
  #   1. node-a's agent applies nix/node-a-choir-backup-target.nix and returns
  #      its SSH host key (docs/reports/node-a-backup-handover-2026-10-20.md).
  #   2. Owner, on Node B as root, in a terminal: scripts/off-host-backup.sh
  #      keygen. It creates secretsDir (0700) and, inside it, restic-password
  #      (printed once to the terminal; copy it to the offline medium),
  #      rest-password and ssh-key; it prints the ssh public key and the
  #      htpasswd line to hand to node-a's agent. Place known-hosts (node-a's
  #      host key, verified out of band) there by hand.
  #   3. scripts/off-host-backup.sh init (creates the daily and archive repos),
  #      then confirm the offline copy opens the repo (`restic key list`).
  #   4. Temporarily set enable = true on a staging build or `systemd-run` the
  #      script by hand; one supervised `backup` run; measure wall time, RSS,
  #      bytes, free disk. Then the restore drill in the problem record.
  #   5. Flip enable = true here and deploy.
  # Rollback: enable = false. Nothing on node-a is touched by that.
  offHostBackup = {
    # On 2026-10-20 after two supervised runs and a passing restore drill
    # (docs/reports/off-host-restore-drill-2026-10-20.md); owner approved
    # node-a as the target the same day.
    enable = true;
    host = "51.81.93.94"; # node-a (docs/node-a-shared-host.md)
    sshPort = 22;
    sshUser = "choir-backup";
    remoteRestPort = 8000; # rest-server, loopback on node-a
    localPort = 18000; # the forward's end on this host, loopback only
    restUser = "nodeb";
    quotaGiB = 450; # must match the node-a module's quotaGiB
    uploadLimitKiB = 25000; # about 200 Mbit/s; 0 = unlimited
    minFreeGiB = 40; # disk guard: Dolt backup copies and image clones need room
    # Guest data.img images in the nightly run: the paper and the owner
    # computer only. A computer rebuilds from its tape by design; an image
    # is a recovery-time hedge, and the tracers' test computers need none
    # (docs/problems/no-off-host-backup-2026-10-20.md section 13 item 10).
    # Empty = every image.
    imageComputers = [
      "computer-1355b83bced1613eb048ee28d66688a8" # the paper computer
      "computer-03335285269bdba4f94377e56879f9e6" # the owner computer
    ];
    # Created by the owner's keygen (2026-10-20). The unit fails to start
    # (loudly) if any credential file below goes missing.
    secretsDir = "/var/lib/go-choir/secrets/offhost-backup";
    stateDir = "/var/lib/go-choir/offhost-backup";
  };
  offHostBackupScript = pkgs.writeShellScript "go-choir-offhost-backup"
    (builtins.readFile ../scripts/off-host-backup.sh);
  offHostBackupEnv = [
    "PATH=${lib.makeBinPath [ pkgs.coreutils pkgs.gnugrep pkgs.gnused pkgs.gawk pkgs.findutils pkgs.util-linux pkgs.jq pkgs.curl pkgs.restic pkgs.openssh pkgs.sqlite pkgs.dolt pkgs.apacheHttpd pkgs.systemd pkgs.btrfs-progs ]}"
    "OFFHOST_STATE_DIR=${offHostBackup.stateDir}"
    "OFFHOST_SECRETS_DIR=${offHostBackup.secretsDir}"
    "OFFHOST_HOST=${offHostBackup.host}"
    "OFFHOST_SSH_PORT=${toString offHostBackup.sshPort}"
    "OFFHOST_SSH_USER=${offHostBackup.sshUser}"
    "OFFHOST_REMOTE_REST_PORT=${toString offHostBackup.remoteRestPort}"
    "OFFHOST_LOCAL_PORT=${toString offHostBackup.localPort}"
    "OFFHOST_REST_USER=${offHostBackup.restUser}"
    "OFFHOST_QUOTA_GIB=${toString offHostBackup.quotaGiB}"
    "OFFHOST_UPLOAD_LIMIT_KIB=${toString offHostBackup.uploadLimitKiB}"
    "OFFHOST_MIN_FREE_GIB=${toString offHostBackup.minFreeGiB}"
    "OFFHOST_IMAGE_COMPUTERS=${lib.concatStringsSep "," offHostBackup.imageComputers}"
    # Go runtime soft limit for restic, 2 GiB under MemoryHigh: the
    # collector runs harder near it instead of letting GOGC slack grow
    # past the unit's line (docs/problems/offhost-backup-reads-terabytes-
    # and-needs-swap-2026-10-22.md, option 1).
    "GOMEMLIMIT=8GiB"
  ];
  offHostBackupCredentials = [
    "restic-password:${offHostBackup.secretsDir}/restic-password"
    "rest-password:${offHostBackup.secretsDir}/rest-password"
    "ssh-key:${offHostBackup.secretsDir}/ssh-key"
    "known-hosts:${offHostBackup.secretsDir}/known-hosts"
  ];
  offHostBackupUnit = description: sub: extra: {
    inherit description;
    # A deploy must not kill a run in flight; a run cut off leaves only
    # unreferenced packs and the next tick continues. Never resumed at boot.
    restartIfChanged = false;
    stopIfChanged = false;
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = "${offHostBackupScript} ${sub}";
      Environment = offHostBackupEnv;
      Nice = 19;
      IOSchedulingClass = "idle";
      CPUWeight = 10;
      # restic holds the tree of a 9.5M-file tape in memory: about 13 GB
      # with no GOMEMLIMIT (docs/problems/no-off-host-backup-2026-10-20.md
      # section 13 item 8), and every run filled its 4G of swap, reading
      # terabytes. No swap now (docs/problems/offhost-backup-reads-
      # terabytes-and-needs-swap-2026-10-22.md): the Go heap is held under
      # GOMEMLIMIT (offHostBackupEnv), the unit throttles at 10G and is
      # killed inside its own cgroup at 12G, loudly. OOMPolicy=continue: an
      # OOM kill of restic is left to the script, which records it in
      # status.json, instead of systemd stopping the unit around it. The
      # backup and verify units add OOMScoreAdjust (below).
      MemoryHigh = "10G";
      MemoryMax = "12G";
      MemorySwapMax = "0";
      OOMPolicy = "continue";
      TimeoutStartSec = "12h";
      # Creates ${offHostBackup.stateDir} (status.json, restic cache, staging)
      # and keeps it writable under ProtectSystem=strict.
      StateDirectory = "go-choir/offhost-backup";
      StateDirectoryMode = "0700";
      PrivateTmp = true;
      NoNewPrivileges = true;
      ProtectHome = "tmpfs";
      ProtectSystem = "strict";
      ProtectKernelModules = true;
      ProtectKernelTunables = true;
      ProtectControlGroups = true;
      ReadWritePaths = [ "-${offHostBackup.stateDir}" "-${platformDoltDir}" "-${corpusDoltDir}" ];
    } // extra;
  };

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
    # Restrict which system calls the service can make. "~@privileged"
    # removes chown, setuid, capset, chroot, bpf, acct, ...; the old third
    # entry "@privileged" re-allowed them (systemd applies the list in
    # order; docs/problems/loopback-is-a-credential-2026-10-14.md). No host
    # service or tool it runs (ip, iptables, mkfs.ext4, git, dolt,
    # ssh-keygen, firecracker without the jailer) calls one.
    SystemCallFilter = [ "@system-service" "~@privileged" "~@mount" ];
    # A filtered call fails with EPERM (logged by the caller) instead of
    # killing the process with SIGSYS, so a missed syscall degrades one
    # operation rather than crash-looping a unit.
    SystemCallErrorNumber = "EPERM";
    # Don't allow the service to change its mount namespace.
    MountFlags = "private";
    # No root service needs a capability: every file in their state is
    # root-owned and owner-writable (live check 2026-10-14), ports are
    # unprivileged, and none changes uid. Units that need one (vmctl:
    # CAP_NET_ADMIN) override this.
    CapabilityBoundingSet = "";
    # /root, /home and /run/user are an empty read-only tmpfs. "tmpfs" (not
    # "yes") so a tool that probes $HOME (git) sees ENOENT, not EACCES.
    ProtectHome = "tmpfs";
  };
  # Env files that units read only through EnvironmentFile=, kept in one
  # root-only directory that every long-running host unit, and every oneshot
  # that declares hidden paths, hides as a directory (the thirteen oneshots
  # that hide nothing are listed in the problem record below). Hiding them one
  # by one failed (docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md):
  # InaccessiblePaths mounts over a file when the unit's namespace is made,
  # so a file replaced by rename later (the deploy's rewrite of deploy.env),
  # or created after the unit started ("-" skips an absent path), is
  # visible to that unit until its next restart. A rename or a create
  # inside a hidden directory stays hidden. The activation snippet
  # go-choir-unit-env moved the files here from /var/lib/go-choir, leaves a
  # link at each old path and quarantines anything else found there. Operators
  # write (install -m 0600) and remove unit-env/<name>: a
  # `>` through an old path's link lands here too, but `rm` of an old path
  # removes only the link (the file, and a switch it holds, stays).
  unitEnvDir = "/var/lib/go-choir/unit-env";
  unitEnvNames = [ "deploy.env" "corpus-dsn.env" "corpusd-retrieval.env" "sandbox-gateway-token.env" ];
  unitEnvFile = name: "-${unitEnvDir}/${name}";
  # Written by the activation snippet at the end of the first run that
  # passes: every name was imported, linked or absent, and no stray waits. A
  # failing run never writes it. The units that read an env file from
  # unitEnvDir assert it, so a partial or failed import (an IO failure after
  # some moves, an obstruction at an old path) never starts them. It says
  # the import ran, not that any file is there: a file a unit requires has
  # its own assert (unitEnvDsn, round 2c).
  unitEnvImported = "${unitEnvDir}/.imported";
  # corpusd and checkpointd refuse to start without it: absent, they would
  # put corpus tables on Store A. Store A on purpose (the authority-split
  # runbook's rollback) is a file without CORPUSD_CORPUS_DOLT_DSN.
  unitEnvDsn = "${unitEnvDir}/corpus-dsn.env";
  # Host secrets a unit does not read stay out of its mount view
  # (InaccessiblePaths; "-" tolerates an absent file). Each unit removes
  # what it needs. EnvironmentFile= and LoadCredential= are read by systemd
  # before the namespace exists, so files only used that way stay hidden.
  # A file entry is hidden only while the inode the unit saw at its start
  # stays in place: the gateway's renames of codex-auth.json and
  # gateway-identities.json are open (the same problem record, item 3).
  hostSecretPaths = {
    jwtKey = "-${authSigningDir}/ed25519-key";
    providerEnv = "-/var/lib/go-choir/gateway-provider.env";
    codexAuth = "-/var/lib/go-choir/codex-auth.json";
    maildEnv = "-/var/lib/go-choir/maild.env";
    searxngEnv = "-/var/lib/go-choir/searxng.env";
    gatewayIdentities = "-/var/lib/go-choir/gateway-identities.json";
    secretsDir = "-/var/lib/go-choir/secrets";
    platformSigningKey = "-${platformArtifactsDir}/signing-key";
    escrowKey = "-${platformArtifactsDir}/key-escrow-key";
    mail = "-${mailDir}";
    # Env files read only through EnvironmentFile= (systemd reads them before
    # the namespace exists), hidden as one directory (unitEnvDir),
    # so a unit that can write /var/lib/go-choir can neither read them nor
    # write another unit's environment for its next start. No "-": a unit
    # whose directory is missing refuses to start (226/NAMESPACE) instead of
    # running with it unmasked once it appears. The activation makes it
    # before any unit starts (boot) or restarts (switch), and tmpfiles too.
    unitEnv = unitEnvDir;
    # PID 1's private socket and the system bus: a root unit that can reach
    # either can ask systemd to run a command outside its own sandbox. Only
    # units that run systemctl keep them (vmctl's store compaction, the Dolt
    # liveness probe); the guest units already hide both.
    systemdPrivate = "-/run/systemd/private";
    dbusSystemSocket = "-/run/dbus/system_bus_socket";
  };
  hiddenSecretsExcept = keep: lib.attrValues (removeAttrs hostSecretPaths keep);
  # SEC-05 gateway bearer binding (internal/gateway/bearer_binding.go;
  # docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md): a
  # computer's gateway bearer is accepted only from the address vmctl
  # attests for that computer. Staged like the host-internal credential:
  #   "observe"  log and count what enforce would refuse ("would_refuse" in
  #              the gateway's /health bearer_binding); refuse nothing.
  #   "enforce"  refuse a pinned bearer from any other address; bind an
  #              unpinned one to its first caller.
  #   "strict"   enforce, and refuse an unpinned bearer.
  #   "off"      the old behavior.
  # Flipped to "enforce" on 2026-10-22: would_refuse stayed 0 for the paper
  # and the test computers across deploys L to O (FP7 definition, "SEC-05
  # enforce flip"). Rollback: "observe" and redeploy. An unset or unknown
  # value enforces in code.
  gatewayBearerBindingMode = "enforce";
  # RES-11 gateway model-call scheduler (internal/gateway/model_scheduler.go;
  # docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md): "on"
  # replaces the per-computer 60/minute window for inference and chat calls
  # with a fair queue (24 in flight, 6 per computer, 256 queued, 120 s wait,
  # then 429 with Retry-After). "off" is the old behavior. Flip as its own
  # deploy after the staging proof; /health model_scheduler shows the queue.
  # Rollback: "off" and redeploy.
  gatewayModelScheduler = "off";
  # B7 gateway request body caps (internal/gateway/body_caps.go;
  # docs/problems/gateway-request-bodies-unbounded-2026-10-22.md). Every
  # route's body is capped and refused 413 unread past its cap. This mode
  # governs only the model routes (inference, OpenAI chat; limit 160 MiB):
  #   "observe"  serve a model body over 160 MiB, up to the 576 MiB ceiling,
  #              and log `body cap would_refuse` with its size and route;
  #   "enforce"  refuse it.
  # Search (1 MiB), source-api (64 KiB), Jev (1 MiB) and the operator
  # routes (64 KiB) enforce in every mode, and so do the two bounds below.
  # Observe for one deploy, then enforce if /health
  # body_caps.would_refuse.model stays 0 and max_seen stays under the
  # limit. An unset or unknown value enforces in code. In every mode the
  # bytes of all bodies held at once are bounded (limits.in_flight, 1 GiB,
  # whatever the scheduler's mode), one computer holds at most a quarter of
  # them (limits.per_caller, 256 MiB, or its one first body), a body waits
  # at most 60 s for room and then has 60 s to arrive
  # (GATEWAY_BODY_READ_TIMEOUT). Limits: GATEWAY_BODY_LIMITS, e.g.
  # "model=256MiB,in_flight=2GiB". Rollback: "observe" only while
  # gatewayModelScheduler is "off" or with in_flight unchanged; otherwise
  # raise GATEWAY_BODY_LIMITS model= in enforce.
  gatewayBodyCaps = "observe";
  # claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md): the
  # owner's setup-token token, root 0400, written by the owner only. Every
  # unit, vmctl included, keeps the secrets dir hidden; vmctl's start copies
  # this one file (mode and owner preserved, so vmctl's owner-only check
  # still sees the original's) into its private runtime dir, which systemd
  # removes when vmctl stops. A token installed or rotated after vmctl
  # started is seen at the next vmctl restart (every deploy restarts it).
  # The copy never echoes the token and never fails the unit.
  claudeDelegateTokenFile = "/var/lib/go-choir/secrets/claude-oauth-token";
  claudeDelegateTokenCopy = "/run/go-choir/claude-oauth-token";
  claudeDelegateTokenStage = pkgs.writeShellScript "go-choir-claude-delegate-token-stage" ''
    src=${claudeDelegateTokenFile}
    dst=${claudeDelegateTokenCopy}
    ${pkgs.coreutils}/bin/rm -f "$dst"
    if [ -f "$src" ] && [ ! -L "$src" ]; then
      ${pkgs.coreutils}/bin/cp -p "$src" "$dst" 2>/dev/null || ${pkgs.coreutils}/bin/rm -f "$dst"
    fi
    exit 0
  '';

  # CI deploy channel (SDC SEC-14;
  # docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md). The CI key
  # logs in as choir-deploy with one forced command, which can only run the
  # gate as root. The gate accepts three verbs (receipt, prebuild, deploy)
  # with typed values and runs the deploy program read from a commit on main.
  ciDeployKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHR2N41wH+Uw3BFTbgThe4f4PGnODEcm6nVI6aPN2ugf github-actions-deploy@go-choir";
  ciDeployGate = pkgs.writeShellScript "go-choir-ci-deploy-gate"
    (builtins.readFile ../scripts/node-b-ci-gate);
  ciDeployForcedCommand = pkgs.writeShellScript "go-choir-ci-deploy-forced-command" ''
    exec /run/wrappers/bin/sudo -n ${ciDeployGate} -- "''${SSH_ORIGINAL_COMMAND:-}"
  '';
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
    # The CI deploy key logs in only as choir-deploy (SEC-14).
  ];

  # The CI deploy user: no password, no pty, no forwarding; the forced
  # command is its only program, and the gate its only sudo rule.
  users.groups.choir-deploy = { };
  users.users.choir-deploy = {
    description = "go-choir CI deploy (forced command)";
    isSystemUser = true;
    group = "choir-deploy";
    useDefaultShell = true;
    openssh.authorizedKeys.keys = [
      ''restrict,command="${ciDeployForcedCommand}" ${ciDeployKey}''
    ];
  };
  security.sudo.extraRules = [
    {
      users = [ "choir-deploy" ];
      runAs = "root";
      commands = [
        {
          command = "${ciDeployGate}";
          options = [ "NOPASSWD" ];
        }
      ];
    }
  ];

  # Firewall — ports 22, 80, 443 ONLY. Service ports (8081-8087) NOT open externally.
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

  # The guests' resolver (guestDnsPort above). Loopback only; the tap DNAT
  # keeps the guest's source address, so access control names the guest
  # pool (vmmanager guestPoolCIDRs: 10.200.0.0/13 and 10.208.0.0/13) and
  # refuses everything else. Private and link-local answers are stripped
  # (DNS rebinding: a public name must not resolve to a host or LAN
  # address). The host's own resolv.conf is not changed.
  services.unbound = {
    enable = true;
    resolveLocalQueries = false;
    settings.server = {
      interface = [ "127.0.0.1@${toString guestDnsPort}" ];
      access-control = [
        "0.0.0.0/0 refuse"
        "::/0 refuse"
        "127.0.0.0/8 allow"
        "10.200.0.0/13 allow"
        "10.208.0.0/13 allow"
      ];
      private-address = [
        "0.0.0.0/8" "10.0.0.0/8" "100.64.0.0/10" "127.0.0.0/8" "169.254.0.0/16"
        "172.16.0.0/12" "192.168.0.0/16" "fc00::/7" "fe80::/10" "::ffff:0:0/96"
      ];
      hide-identity = true;
      hide-version = true;
    };
  };

  # Caddy reverse proxy (TLS termination → Go services + frontend)
  # Primary public/staging host is choir.news; old domains redirect here.
  services.caddy = {
    enable = true;
    virtualHosts."choir.news" = {
      extraConfig = ''
        # Compress what benefits: a story bundle is up to ~460 KB of JSON
        # and the app ~475 KB of script and CSS. The explicit list keeps
        # text/event-stream (live Texture and trajectory updates) unencoded,
        # so a compressor never buffers a stream; WebSocket upgrades pass
        # untouched (docs/problems/edge-serves-everything-uncompressed-2026-10-16.md).
        # A request that asks for a stream is never wrapped: Caddy's encoder
        # holds the headers until the first body write, so an idle stream
        # would not open (lifecycle.js waits for open before its snapshot).
        @compressible not header Accept *text/event-stream*
        encode @compressible {
          zstd
          gzip
          match {
            header Content-Type application/json*
            header Content-Type text/html*
            header Content-Type text/css*
            header Content-Type text/javascript*
            header Content-Type application/javascript*
            header Content-Type image/svg+xml*
          }
        }
        handle /auth/* {
          reverse_proxy 127.0.0.1:8081
        }
        handle /health {
          reverse_proxy 127.0.0.1:8082
        }
        handle /health/* {
          reverse_proxy 127.0.0.1:8084
        }
        handle /api/email/resend/webhook {
          reverse_proxy 127.0.0.1:8087
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
          reverse_proxy 127.0.0.1:8082
        }
        handle {
          # Computer-surface HTML/assets are selected after vmctl resolve.
          # Host frontend-current is platform-shell chrome only (unsigned).
          reverse_proxy 127.0.0.1:8082
        }
      '';
    };
    virtualHosts."choir-ip.com" = {
      extraConfig = ''
        redir https://choir.news{uri} permanent
      '';
    };
  };

  # Qdrant vector search engine (host service, localhost-only).
  # The Go qdrant client (internal/qdrant) talks to http://127.0.0.1:6333.
  # VMs on the tap network cannot reach 127.0.0.1; see VM reachability note
  # in the mission doc. If VMs need Qdrant access, bind to the tap IP or
  # 0.0.0.0 (the firewall already blocks 6333 externally).
  services.qdrant = {
    enable = true;
    settings = {
      service = {
        host = "127.0.0.1";
        http_port = 6333;
        grpc_port = 6334;
      };
      storage = {
        storage_path = "/var/lib/qdrant/storage";
        snapshots_path = "/var/lib/qdrant/snapshots";
        hnsw_index.on_disk = true;
      };
      telemetry_disabled = true;
    };
  };

  # SearXNG self-hosted meta-search engine (localhost-only, free unlimited queries).
  # Aggregates Google, Bing, DuckDuckGo, and 70+ engines. JSON API enabled for
  # the gateway's SearXNGProvider. No API key, no credits. The limiter is
  # disabled because this is an internal API endpoint, not a public-facing
  # search UI — only the gateway (127.0.0.1) can reach it.
  services.searx = {
    enable = true;
    package = pkgs.searxng;
    environmentFile = "/var/lib/go-choir/searxng.env";
    settings = {
      use_default_settings = true;
      general = {
        instance_name = "Choir Search";
        debug = false;
      };
      search = {
        safe_search = 0;
        autocomplete = "";
        formats = [ "html" "json" ];
        default_range = "";
      };
      server = {
        secret_key = "$SEARXNG_SECRET";
        bind_address = "127.0.0.1";
        port = 8888;
        limiter = false;
        image_proxy = false;
      };
      ui = {
        static_use_hash = true;
      };
      outgoing = {
        request_timeout = 10;
        max_request_timeout = 15;
      };
    };
  };

  # The host units that come from NixOS modules hide the same set as the
  # go-choir units (docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md,
  # problems 2 and 5). None runs as root and none uses D-Bus or PID 1's
  # socket: caddy proxies to loopback and reloads through its own admin
  # API, qdrant and unbound serve loopback (unbound also the guests, through
  # the tap DNAT), searx and searx-init read their settings from /run/searx;
  # name lookups go through nscd. searx and searx-init keep their own
  # searxng.env (FP0's ruling; hiding it gains nothing, the process holds
  # SEARXNG_SECRET in its environment and settings). The first deploy of
  # this restarts each of them once (a unit-file change); caddy's restart
  # drops open edge connections, which clients reopen.
  systemd.services.caddy.serviceConfig.InaccessiblePaths = hiddenSecretsExcept [ ];
  systemd.services.qdrant.serviceConfig.InaccessiblePaths = hiddenSecretsExcept [ ];
  systemd.services.unbound.serviceConfig.InaccessiblePaths = hiddenSecretsExcept [ ];
  systemd.services.searx.serviceConfig.InaccessiblePaths = hiddenSecretsExcept [ "searxngEnv" ];
  systemd.services.searx-init.serviceConfig.InaccessiblePaths = hiddenSecretsExcept [ "searxngEnv" ];

  # ── Systemd services ──────────────────────────────────────────────────
  # Host services: auth, proxy, vmctl, gateway, autoputer, maild and
  # corpusd.
  # Autoputer workloads for authenticated traffic are expected to run inside
  # Firecracker microVMs managed by vmctl. Node B disables vmctl's
  # host-process fallback so deployed routing fails closed instead of silently
  # landing on the placeholder host autoputer.
  #
  # Guest images are repo-built (VAL-VM-010):
  #   nix build .#guest-image  →  kernel (vmlinux) + rootfs (ext4) + initrd
  # The guest contains ONLY the autoputer binary — no provider credentials,
  # no auth signing keys, no gateway secrets (VAL-VM-011).
  #
  # Restart and recovery behavior (VAL-DEPLOY-008 / VAL-CROSS-118):
  # - Each service uses Restart=on-failure with a 3-second backoff.
  # - Proxy depends on auth and autoputer; auth and autoputer restart
  #   independently. After an auth restart, existing access JWTs remain
  #   valid because the signing key file persists across restarts. After
  #   a autoputer restart, the proxy /health endpoint reports "degraded"
  #   until the autoputer comes back, then returns to "ok".
  # - Auth sessions are persisted in SQLite, so session state survives
  #   auth restart. Browser users either rehydrate via refresh-token
  #   rotation or fall back safely to the guest state.
  # - WatchdogSec is intentionally NOT set because the Go server package
  #   does not send sd_notify keepalives. Adding WatchdogSec without
  #   sd_notify causes the service to be killed every 30 seconds.

  systemd.services.go-choir-auth = {
    # The deploy restarts this unit once, after the service pointer holds the
    # new binary; a switch-time restart ran the old pointer and held the site
    # down while the gateway drained (L1, deploy-without-site-outage design;
    # vmctl precedent 2f755bb3).
    restartIfChanged = false;
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
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
      InaccessiblePaths = hiddenSecretsExcept [ "jwtKey" ];
      # auth.db holds sessions and API-key records; it was 0644 in a 0755
      # directory, private only because /var/lib/go-choir is 0750.
      StateDirectoryMode = "0750";
      # Account-recovery mail goes to maild's host-internal route, which
      # needs the host-internal credential once hostauth enforces
      # (docs/problems/account-recovery-token-discarded-2026-10-14.md).
      LoadCredential = internalTokenCredential;
      Environment = [
        "AUTH_PORT=8081"
        "AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        "AUTH_RP_ID=choir.news"
        "AUTH_RP_ORIGINS=https://choir.news"
        "AUTH_JWT_PRIVATE_KEY_PATH=${authSigningDir}/ed25519-key"
        "AUTH_ACCESS_TOKEN_TTL=5m"
        "AUTH_REFRESH_TOKEN_TTL=720h"
        "AUTH_COOKIE_SECURE=true"
        # Recovery links are mailed through maild (empty disables mail).
        "AUTH_MAILD_URL=http://127.0.0.1:8087"
        # Sign-up cap with a waitlist (L4-03, internal/auth/registration_cap.go):
        # unset is no cap. FP0 sets "AUTH_REGISTRATION_CAP=<n>" after reading
        # the count of users holding a passkey (FP8 station, L4-03).
        internalAuthEnv
      ];
    };
  };

  systemd.services.go-choir-proxy = {
    # The deploy restarts this unit once, after the service pointer holds the
    # new binary; a switch-time restart ran the old pointer and held the site
    # down while the gateway drained (L1, deploy-without-site-outage design;
    # vmctl precedent 2f755bb3).
    restartIfChanged = false;
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Proxy Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-auth.service" "go-choir-corpusd.service" "go-choir-maild.service" ];
    wants = [ "network-online.target" "go-choir-corpusd.service" "go-choir-maild.service" ];
    requires = [ "go-choir-auth.service" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "proxy" goChoirPackages.proxy}";
      Restart = "on-failure";
      RestartSec = 3;
      EnvironmentFile = unitEnvFile "deploy.env";
      LoadCredential = internalTokenCredential ++ [ "choir-owner-act-key:${ownerActKeyFile}" ];
      # Proxy reads only the public key (PROXY_AUTH_PUBLIC_KEY_PATH) and the
      # auth DB (SQLite needs write access for its -wal/-shm). It never
      # writes auth-signing/ and never reads the private key.
      ReadWritePaths = [ "/var/lib/go-choir/auth" ];
      InaccessiblePaths = hiddenSecretsExcept [ ];
      Environment = [
        "SERVER_HOST=0.0.0.0"
        "PROXY_PORT=8082"
        # SEC-11: loopback callers of the object-graph route
        # (/internal/platform/objects*, edges, captures, published search,
        # story verdicts) present the host credential. Observe first: logs
        # "hostauth: observe: would refuse" per caller and allows. Flip to
        # "enforce" after a clean day (no such line for these routes).
        # Rollback: "observe".
        "PROXY_OBJECT_GRAPH_INTERNAL_AUTH=observe"
        "PROXY_AUTOPUTER_URL=http://127.0.0.1:8085"
        "PROXY_AUTH_PUBLIC_KEY_PATH=${authSigningDir}/ed25519-key.pub"
        "PROXY_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        # When vmctl is running, the proxy resolves user VM ownership
        # through vmctl instead of using the static autoputer URL
        # (VAL-VM-001, VAL-VM-002).
        "PROXY_VMCTL_URL=http://127.0.0.1:8083"
        # Bounded fail-fast timeout for the proxy -> vmctl resolve path. A hung
        # or slow vmctl resolve returns a legible 504 to the caller within 60s
        # instead of keeping the public request open for the full boot wait.
        # Cold computer boot completion is handled by vmctl's
        # VM_BOOT_READY_TIMEOUT and the retry/replay path (Phase D).
        "PROXY_VMCTL_TIMEOUT=30m"
        # Replay completeness reconstructs a disposable projection and is an
        # owner-only route with a dedicated 10m budget; ordinary interactive
        # proxy routes stay at 30s. The handler extends the response deadline
        # for this route only; it does not widen the global proxy write budget.
        "PROXY_REPLAY_COMPLETENESS_TIMEOUT=10m"
        # Residue import serializes a bounded reducer snapshot and uses its own
        # 110s budget for the same fail-closed route-boundary reason.
        "PROXY_RESIDUE_IMPORT_TIMEOUT=110s"
        "PROXY_CORPUSD_URL=http://127.0.0.1:8086"
        "PROXY_MAILD_URL=http://127.0.0.1:8087"
        # Story expansion requests (docs/reports/story-expansion-request-
        # design-2026-10-20.md, phase 1). Comma-separated user ids allowed to
        # ask a story to be expanded; empty means off (the routes are dark and
        # the page shows no button). Not a secret. To turn on, set the owner's
        # user id here and deploy. Kill switch: PROXY_EXPAND=off.
        "PROXY_EXPAND_REQUESTERS="
        # Public door (owner 2026-10-20, owner notes section 32): readers who
        # are not logged in may ask an Autopaper story to be expanded. They
        # are limited to 3 an hour and 10 a UTC day per address (counted in
        # proxy memory, reset by a restart) on top of the global 3 in flight
        # and 12 a day. Turn off with "off" or by removing the line;
        # PROXY_EXPAND=off still wins over everything.
        "PROXY_EXPAND_PUBLIC=on"
        # Unsigned / and /assets/* serve this host tree (picker/auth chrome).
        # Authenticated computer surface is selected after vmctl resolve.
        "PROXY_PLATFORM_SHELL_ROOT=${frontendCurrent}"
        internalAuthEnv
      ];
    };
  };

  systemd.services.go-choir-maild = {
    # The deploy restarts this unit once, after the service pointer holds the
    # new binary; a switch-time restart ran the old pointer and held the site
    # down while the gateway drained (L1, deploy-without-site-outage design;
    # vmctl precedent 2f755bb3).
    restartIfChanged = false;
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Mail Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "maild" goChoirPackages.maild}";
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/mail";
      # Mail state contains private message bodies and provider metadata.
      StateDirectoryMode = "0700";
      UMask = "0077";
      EnvironmentFile = "-/var/lib/go-choir/maild.env";
      LoadCredential = internalTokenCredential;
      ReadWritePaths = [ mailDir ];
      InaccessiblePaths = hiddenSecretsExcept [ "mail" ];
      Environment = [
        # Autoputer guest VMs persist Email appagent drafts via the host tap
        # address. The host firewall still keeps 8087 closed externally.
        "SERVER_HOST=0.0.0.0"
        "MAILD_PORT=8087"
        "MAILD_DB_PATH=${mailDir}/mail.db"
        "MAILD_STORAGE_ROOT=${mailDir}"
        "MAILD_PRIMARY_DOMAIN=choir.news"
        # Maild routes trace events through vmctl to user VMs. The host
        # autoputer fallback (MAILD_RUNTIME_URL) was removed in PR 5.
        "MAILD_VMCTL_URL=http://127.0.0.1:8083"
        # Autopaper daily digest: where maild reads the front list, and the
        # kill switch ("off" makes the run route do nothing). Nobody receives
        # a digest until the owner opts in from the Mail app.
        "MAILD_CORPUSD_URL=http://127.0.0.1:8086"
        "MAILD_DIGEST=on"
        # Send gate (docs/problems/maild-send-gate-defects-2026-10-14.md):
        # only the owner's session approves and sends; a guest drafts and
        # proposes. Rollback: "legacy" restores the pre-E1 rules for guest
        # sends, approval-reply authentication and trusted senders.
        "MAILD_SEND_GATE_MODE=enforce"
        # MAILD_TRUSTED_AUTHSERV_IDS (comma-separated) names the receiving
        # server whose Authentication-Results maild believes. Unset, approval
        # replies and trusted-sender aliases fail closed. Set it from the
        # authserv-id (first token) of a real inbound message's
        # Authentication-Results header, read in the Mail app, after checking
        # that the provider strips sender-supplied copies of that header.
        internalAuthEnv
      ];
    };
  };

  systemd.services.go-choir-platform-dolt = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Platform Dolt SQL Server";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStartPre = platformDoltInit;
      ExecStart = "${pkgs.dolt}/bin/dolt sql-server --host 127.0.0.1 --port 13306";
      WorkingDirectory = platformDoltDBDir;
      # Host memory budget (docs/problems/node-b-memory-overcommitted-swap-
      # 2026-10-09.md): the canonical log is throttled, never swapped and
      # never hard-killed. ~1.1 GB RSS measured 2026-10-09.
      MemoryHigh = "3G";
      MemorySwapMax = "0";
      OOMScoreAdjust = -500;
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/platform-dolt";
      StateDirectoryMode = "0750";
      ReadWritePaths = [ platformDoltDir ];
      InaccessiblePaths = hiddenSecretsExcept [ ];
      # The default 90 s stop timeout made the 2026-10-15 recovery wait for
      # SIGKILL; the liveness probe also relies on a bounded stop.
      TimeoutStopSec = "20s";
      # No core dump of a multi-GB process when the probe sends SIGABRT:
      # GOTRACEBACK=all prints every goroutine to the journal and exits 2.
      LimitCORE = 0;
      Environment = [
        "HOME=${platformDoltDir}"
        "GOTRACEBACK=all"
      ];
    };
  };

  # Store B: world-wire/corpus Dolt sql-server. Separate process from the
  # canonical event/control store (Store A, port 13306) so a corpus GC-OOM or
  # bulk scan cannot take the canonical log down — the blast-radius isolation
  # is the point of the split (docs/designs/
  # platform-dolt-storage-normalization-2026-09-18.md).
  systemd.services.go-choir-corpus-dolt = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Corpus Dolt SQL Server (Store B)";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStartPre = corpusDoltInit;
      ExecStart = "${pkgs.dolt}/bin/dolt sql-server --host 127.0.0.1 --port 13307";
      # Host memory budget: replaces the untracked 18G/20G set-property
      # drop-ins in /etc/systemd/system.control. ~4.0 GB RSS measured
      # 2026-10-09. Store B is the blast-radius-isolated store: it is the one
      # that may be killed and restarted, never the guests.
      MemoryHigh = "5G";
      MemoryMax = "7G";
      MemorySwapMax = "0";
      # WorkingDirectory is the parent dir (StateDirectory-created): systemd
      # chdirs before ExecStartPre runs, so pointing at corpusDoltDBDir would
      # fail on first boot before the init script creates it. dolt sql-server
      # serves the `corpus` repo under cwd either way.
      WorkingDirectory = corpusDoltDir;
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = "go-choir/corpus-dolt";
      StateDirectoryMode = "0750";
      ReadWritePaths = [ corpusDoltDir ];
      InaccessiblePaths = hiddenSecretsExcept [ ];
      # The default 90 s stop timeout made the 2026-10-15 recovery wait for
      # SIGKILL; the liveness probe also relies on a bounded stop.
      TimeoutStopSec = "20s";
      # No core dump of a multi-GB process when the probe sends SIGABRT:
      # GOTRACEBACK=all prints every goroutine to the journal and exits 2.
      LimitCORE = 0;
      Environment = [
        "HOME=${corpusDoltDir}"
        "GOTRACEBACK=all"
      ];
    };
  };

  # Dolt SQL users and grants (doltAuthStage above).
  systemd.services.go-choir-platform-dolt-users = doltUsersUnit "go-choir-platform-dolt" 13306 "platform";
  systemd.services.go-choir-corpus-dolt-users = doltUsersUnit "go-choir-corpus-dolt" 13307 "corpus";

  systemd.services.go-choir-corpusd = {
    # The deploy restarts this unit once, after the service pointer holds the
    # new binary; a switch-time restart ran the old pointer and held the site
    # down while the gateway drained (L1, deploy-without-site-outage design;
    # vmctl precedent 2f755bb3).
    restartIfChanged = false;
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Platform Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "go-choir-platform-dolt.service" "go-choir-corpus-dolt.service" ] ++ doltClientAfter;
    wants = [ "network-online.target" "go-choir-corpus-dolt.service" ] ++ doltClientAfter;
    requires = [ "go-choir-platform-dolt.service" ];
    # P5 host build service (docs/reports/p5-host-build-layered-apply-2026-10-15.md):
    # corpusd evaluates `.#autoputer` from the deployed checkout's object
    # store at a computer's base commit plus its frozen patch; the build
    # itself runs in the nix daemon's sandbox (remote builders included).
    # The patch may not touch the recipe or the dependency set, so the
    # evaluation is the base commit's own. Rollback: drop
    # CORPUSD_HOST_BUILD_SOURCE_DIR (Go-touching applies then fail with
    # host_builder_unconfigured and change nothing).
    path = [ config.nix.package pkgs.git ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "corpusd" goChoirPackages.corpusd}";
      Restart = "on-failure";
      RestartSec = 3;
      StateDirectory = [ "go-choir/platform-artifacts" "go-choir/host-build" ];
      StateDirectoryMode = "0750";
      ReadWritePaths = [ platformArtifactsDir "/var/lib/go-choir/host-build" ];
      InaccessiblePaths = hiddenSecretsExcept [ "platformSigningKey" "escrowKey" ];
      LoadCredential = internalTokenCredential ++ issuerTokenCredential ++ doltClientCredential ++ [ "choir-owner-act-public-key:${ownerActPublicKeyFile}" ];
      # Track K two-approval operator credentials live in a deploy-created
      # file outside git (same trust class as gateway credentials):
      # CHOIR_KEY_ESCROW_OPERATORS="name:token,..." — the unwrap gate stays
      # closed (503) while the file or entries are absent.
      EnvironmentFile = [
        "-/var/lib/go-choir/secrets/corpusd-escrow-operators.env"
        # Store B DSN flip: written by the authority-split cutover runbook
        # (docs/runbooks/platform-dolt-authority-split-2026-09-18.md). The
        # unit asserts the file (unitEnvDsn): a file without
        # CORPUSD_CORPUS_DOLT_DSN (the runbook's rollback) shares the Store A
        # pool for corpus tables; no file, no start. The "-" stays so the
        # assert, not the env loader, is what refuses.
        (unitEnvFile "corpus-dsn.env")
        # Retrieval rollback switch, flippable without a deploy
        # (docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
        # write CORPUSD_CAPTURE_SEARCH=off to
        # /var/lib/go-choir/unit-env/corpusd-retrieval.env and restart
        # corpusd; the search and URL routes refuse and every guest falls
        # back to the window of its newest captures. Absent: the routes
        # answer.
        (unitEnvFile "corpusd-retrieval.env")
      ];
      Environment = [
        "SERVER_HOST=0.0.0.0"
        "CORPUSD_PORT=8086"
        "CORPUSD_DOLT_DSN=root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
        "CORPUSD_ARTIFACTS_ROOT=${platformArtifactsDir}"
      ] ++ doltClientEnv ++ [
        "CORPUSD_VMCTL_URL=http://127.0.0.1:8083"
        "CHOIR_OWNER_ACT_MODE=${ownerActMode}"
        # The public Autopaper front list shows only the designated paper
        # computer's owner (Cut 2 2b; docs/reports/reeng-autopaper-paper-computer-2026-10-13.md),
        # so tracer test computers' stories stay off choir.news's front page.
        "CORPUSD_AUTOPAPER_OWNERS=e5962310-8767-4624-87e5-a9b93a4e1db1"
        # Opinion pieces: any signed-in user's Texture published as a
        # labelled op-ed, listed in the Opinion section and found by research
        # (docs/problems/oped-not-connected-2026-10-20.md). Kill switch, no
        # deploy: write CORPUSD_OPINION_LISTING=off to
        # /var/lib/go-choir/unit-env/corpusd-retrieval.env (an EnvironmentFile
        # overrides Environment) and restart corpusd; every opinion piece
        # leaves the front, research search and the publish route at once,
        # direct links keep working. New opinion pieces per user per 24 hours
        # are capped by CORPUSD_OPINION_DAILY_LIMIT (default 5).
        "CORPUSD_OPINION_LISTING=on"
        "CORPUSD_OPINION_DAILY_LIMIT=5"
        # Platform-artifacts reachability GC
        # (docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md).
        # Keeps everything a current computer, restore point (pin) or route
        # receipt (rollback ref) names, plus every unreferenced object younger
        # than GRACE. Each run deletes at most MAX_DELETES objects and
        # MAX_BYTES bytes, logs an intent record before every unlink to
        # platform-artifacts/artifact-gc-log/<day>.jsonl, and the report
        # (journal: "artifact gc:") lists the full eligible set per namespace.
        # og is outside ACTIVE_NAMESPACES (owner step: the pre-teardown corpus
        # bodies). Rollback: MODE=dry-run (report only) or MODE=off.
        # The on-demand /internal/platform/artifact-gc endpoint is clamped to
        # this policy.
        "PLATFORM_ARTIFACT_GC_MODE=active"
        "PLATFORM_ARTIFACT_GC_GRACE=168h"
        "PLATFORM_ARTIFACT_GC_INTERVAL=1h"
        "PLATFORM_ARTIFACT_GC_MAX_DELETES=50000"
        "PLATFORM_ARTIFACT_GC_MAX_BYTES=34359738368"
        "PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES=projection-base,platform-update,file-cas-roots,file-cas-chunks,og"
        # Tape sweep (orphan pins, destroyed computers' tapes), on demand only
        # at /internal/platform/artifact-gc/tape. dry-run admits manifests and
        # refuses apply; flip to active only after a dry-run manifest has been
        # reviewed (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md).
        # Archives default to <artifacts>/tape-sweep-archive (ReadWritePaths).
        # active since 2026-10-20, after the reviewed dry-run
        # ts-20261020T051653Z-fd085f79. Rollback: set back to dry-run.
        "PLATFORM_TAPE_SWEEP_MODE=active"
        # Tape segments (A2): projection-batch pins go to per-computer
        # append-only segments instead of one file each. Owner notes 34
        # ("2. B"): on for every computer after residual r1 (687867fd5).
        # Rollback: set back to off; new pins go legacy again and segments
        # stay readable (docs/definitions/choir-tape-segments-a2-repack-2026-10-21.md).
        "PLATFORM_TAPE_SEGMENTS=write"
        # P5 host build service. The checkout is the deploy's /opt/go-choir
        # (read only: its objects, never its worktree); the base is the
        # deployed guest image a computer must have booted.
        "CORPUSD_HOST_BUILD_SOURCE_DIR=/opt/go-choir"
        "CORPUSD_HOST_BUILD_STATE_DIR=/var/lib/go-choir/host-build"
        "CORPUSD_HOST_BUILD_BASE_MANIFEST=/var/lib/go-choir/guest/guest-image-manifest"
        "CORPUSD_HOST_BUILD_BASE_STOREDISK=/var/lib/go-choir/guest/storedisk.erofs"
        "HOME=/var/lib/go-choir/host-build"
        "XDG_CACHE_HOME=/var/lib/go-choir/host-build/cache"
        internalAuthEnv
        issuerAuthEnv
      ];
    };
  };

  # Owner-authorized maintenance key use is isolated from corpusd requests.
  # One oneshot process per host; the timer does not overlap an active replay.
  systemd.services.go-choir-checkpointd = {
    description = "Choir verified projection checkpoint maintenance";
    after = [ "go-choir-corpusd.service" "go-choir-platform-dolt.service" ] ++ doltClientAfter;
    requires = [ "go-choir-platform-dolt.service" ];
    path = [ pkgs.dolt ];
    serviceConfig = commonServiceHardening // {
      Type = "oneshot";
      ExecStart = "${serviceExec "checkpointd" goChoirPackages.checkpointd}";
      # A timer-started oneshot must not be able to hold a nixos switch: with
      # 8h, a checkpointd stuck on a deadlocked Dolt blocked the P4 deploy for
      # 30 minutes (docs/problems/platform-dolt-deadlock-hangs-checkpointd-
      # and-crashloops-vmctl-2026-10-15.md). Rebuilds take 43-56 s (maximum
      # over 65 publishes, 2026-10-01..15), so 10 minutes is about 10x slack.
      # Rollback: restore "8h".
      TimeoutStartSec = "10min";
      MemoryHigh = "4G";
      MemoryMax = "6G";
      MemorySwapMax = "0";
      # Under host memory pressure the kernel must kill the checkpoint worker
      # (a retryable job) before any guest VM. The 16G ceiling exceeded the
      # host budget (docs/problems/node-b-memory-overcommitted-swap-
      # 2026-10-09.md); compact projection bases are ~1.7 GB.
      OOMScoreAdjust = 1000;
      CPUQuota = "100%";
      Nice = 10;
      UMask = "0077";
      # The projection rebuilder opens a scratch embedded Dolt workspace,
      # which needs a writable $HOME/.dolt. ProtectHome=tmpfs leaves /root
      # read-only, so without this the unit panicked once a minute after
      # claiming a job (docs/problems/dolt-clustering-assessment-2026-10-15.md
      # section 7.1). cmd/checkpointd refuses to start without a writable
      # home. Rollback: revert this block.
      StateDirectory = "go-choir/checkpointd";
      StateDirectoryMode = "0700";
      ReadWritePaths = [ platformArtifactsDir "/var/lib/go-choir/checkpointd" ];
      InaccessiblePaths = hiddenSecretsExcept [ "platformSigningKey" "escrowKey" ];
      EnvironmentFile = [ (unitEnvFile "corpus-dsn.env") ];
      LoadCredential = doltClientCredential;
      Environment = doltClientEnv ++ [
        "HOME=/var/lib/go-choir/checkpointd"
        "CHOIR_CHECKPOINT_MAINTENANCE_AUTHORIZED=true"
        "CORPUSD_DOLT_DSN=root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
        "CORPUSD_ARTIFACTS_ROOT=${platformArtifactsDir}"
      ];
    };
  };
  systemd.timers.go-choir-checkpointd = {
    description = "Reconcile verified projection checkpoints every minute";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnBootSec = "1min";
      OnUnitInactiveSec = "1min";
      Unit = "go-choir-checkpointd.service";
    };
  };

  # Autopaper front-page importance (`corpusd front-importance`,
  # internal/frontimportance). A low-priority
  # oneshot asks Jev to score new or revised front stories and writes
  # /var/lib/go-choir/front-importance/scores.json atomically; corpusd only
  # reads that file. Failures just log; a missing file leaves the front page
  # ranked by recency. It reaches Jev through the gateway's host route
  # (/provider/v1/host/judgments, SEC-16) with the host-internal credential
  # and loads no provider environment, so the gateway is the one holder and
  # meter of the OpenRouter key; no other unit depends on it. Rollback while
  # the old path exists in code: add `EnvironmentFile =
  # "-/var/lib/go-choir/gateway-provider.env"` and
  # `FRONT_IMPORTANCE_DIRECT=1` back and redeploy.
  systemd.services.go-choir-front-importance = {
    description = "Autopaper front-page importance scores (Jev)";
    after = [ "network-online.target" "go-choir-corpusd.service" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      Type = "oneshot";
      ExecStart = "${serviceExec "corpusd" goChoirPackages.corpusd} front-importance";
      TimeoutStartSec = "5min";
      MemoryMax = "512M";
      CPUQuota = "50%";
      Nice = 15;
      IOSchedulingClass = "idle";
      OOMScoreAdjust = 1000;
      StateDirectory = "go-choir/front-importance";
      StateDirectoryMode = "0755";
      ReadWritePaths = [ "/var/lib/go-choir/front-importance" ];
      InaccessiblePaths = hiddenSecretsExcept [ ];
      LoadCredential = internalTokenCredential;
      Environment = [
        internalAuthEnv
        "FRONT_IMPORTANCE_FRONT_URL=http://127.0.0.1:8086/internal/platform/publications/front?limit=100"
        "FRONT_IMPORTANCE_SCORES_PATH=/var/lib/go-choir/front-importance/scores.json"
      ];
    };
  };
  systemd.timers.go-choir-front-importance = {
    description = "Score Autopaper front stories for importance every 10 minutes";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnBootSec = "5min";
      OnUnitInactiveSec = "10min";
      Unit = "go-choir-front-importance.service";
    };
  };

  systemd.services.go-choir-vmctl = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir VMCtl Service (Firecracker VM lifecycle)";
    wantedBy = [ "multi-user.target" ];
    # The deploy restarts vmctl once, after the service pointer holds the new
    # binary. A switch-time restart ran the old pointer and doubled every
    # deploy's restarts, each of which detaches every guest (vmctl 360 review
    # Phase 0 step 6).
    restartIfChanged = false;
    after = [ "network-online.target" "go-choir-platform-dolt.service" "go-choir-corpusd.service" ] ++ doltClientAfter;
    wants = [ "network-online.target" "go-choir-platform-dolt.service" "go-choir-corpusd.service" ] ++ doltClientAfter;
    serviceConfig = commonServiceHardening // {
      ExecStart = "${vmctlExec}";
      Restart = "on-failure";
      RuntimeDirectory = "go-choir";
      RuntimeDirectoryMode = "0700";
      RestartSec = 3;
      # Firecracker needs access to /dev/kvm for VM hardware acceleration.
      # We must allow KVM device access while keeping other hardening.
      PrivateDevices = lib.mkForce false;
      # VMM jailer slice 1 (docs/problems/firecracker-runs-as-root-without-
      # jailer-2026-10-14.md, "Design: slice 1"). vmctl's bounding set:
      #   NET_ADMIN   ip tuntap/addr/link/route and iptables (vmctl only);
      #   SYS_ADMIN MKNOD CHOWN SETUID SETGID   the jailer it runs (mount
      #               namespace, pivot_root, device nodes, uid drop);
      #   DAC_OVERRIDE FOWNER KILL   VM files, sockets, /proc entries and
      #               signals now belong to per-VM uids;
      #   SETPCAP     cut the bounding set of every VMM launch (direct VMMs
      #               keep NET_ADMIN only; jailed VMMs run as a non-root uid
      #               and hold nothing).
      # CAP_SYS_PTRACE is gone: nothing used it (it came with 7a15266d7).
      CapabilityBoundingSet = [
        "CAP_NET_ADMIN" "CAP_SYS_ADMIN" "CAP_MKNOD" "CAP_CHOWN" "CAP_SETUID" "CAP_SETGID"
        "CAP_DAC_OVERRIDE" "CAP_FOWNER" "CAP_KILL" "CAP_SETPCAP"
      ];
      # The jailer mounts and pivot_roots in its own mount namespace.
      SystemCallFilter = [ "@system-service" "@privileged" "@mount" ];
      # Per-VM cgroup v2 leaves: vmctl runs in <unit>/supervisor and each
      # jailed VMM in <unit>/vms/<vm>, all under this unit's MemoryLow and
      # MemorySwapMax. cgroup v2 forbids processes in a cgroup whose
      # controllers are delegated, hence the subgroup.
      Delegate = "yes";
      DelegateSubgroup = "supervisor";
      ProtectControlGroups = lib.mkForce false;
      # Host memory budget: Firecracker guests live in this cgroup. Reserve
      # 14G for them against reclaim by everything else (3 always-on 4 GiB
      # guests plus slack), never swap them, and keep them last in OOM order.
      # Guest count admission against this reservation is vmctl's job.
      MemoryLow = "14G";
      MemorySwapMax = "0";
      # vmctl must outlive every VM under global OOM. Firecracker children
      # set their own priority at launch (+500, lowered to -400 for
      # protected computers by vmctl's process reconcile) instead of
      # inheriting this (docs/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md).
      OOMScoreAdjust = -900;
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
        # claude delegate: "+" runs outside the sandbox, so it can read the
        # hidden secrets dir; it never fails the unit.
        "+${claudeDelegateTokenStage}"
      ];
      # VM state directory for Firecracker VM persistence and epoch tracking.
      # Persistent user data in VMs is stored here and survives stop/resume
      # cycles (VAL-CROSS-116). Provider credentials are NEVER written here
      # (VAL-VM-011).
      StateDirectory = "go-choir/vm-state";
      # data.img files were 0644 in a 0755 vm-state, private only because
      # /var/lib/go-choir is 0750.
      StateDirectoryMode = "0750";
      ReadWritePaths = [ "/var/lib/go-choir" "/var/lib/go-choir/vm-state" ];
      # vmctl reads none of the host secrets, and every Firecracker VMM runs
      # in vmctl's mount namespace: hiding them here takes the JWT key,
      # provider credentials, escrow key and operator tokens out of reach of
      # a VMM escape (F1 partial; VMMs started before the restart keep the
      # old view until their next boot).
      # Store compaction runs systemctl and systemd-run, so vmctl keeps PID 1's
      # sockets: vmctl stays root-equivalent through them (named residual;
      # the fix is a dedicated compaction unit vmctl starts by name).
      InaccessiblePaths = hiddenSecretsExcept [ "systemdPrivate" "dbusSystemSocket" ];
      # vmctl reads but never writes the deployed service binaries and the
      # guest images (the shared images attach read-only; data.img lives in
      # vm-state). Read-only here means a VMM escape cannot replace corpusd's
      # binary or the kernel every later cold boot runs. auth/ stays readable
      # for the retention sweep (VMCTL_RETENTION_AUTH_DB_PATH).
      ReadOnlyPaths = [ "/var/lib/go-choir/auth" "/var/lib/go-choir/services" "/var/lib/go-choir/guest" ];
      LoadCredential = internalTokenCredential ++ issuerTokenCredential ++ doltClientCredential;
      # Computer tiers (paid accounts, platform computers) are not host
      # configuration: they live in internal/computertiers/computer-tiers.json,
      # embedded in vmctl and changed by a reviewed commit (owner ruling
      # 2026-10-13). The old vmctl-priority.env always-on list is retired.
      Environment = [
        "VMCTL_PORT=8083"
        "VMCTL_GUEST_DNS_PORT=${toString guestDnsPort}"
        # D-ROUTE and immutable ComputerVersion inputs are tables on the same
        # corpusd world-wire SQL server. vmctl is the sole route CAS writer.
        "VMCTL_ROUTE_DSN=root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
        "VMCTL_ARTIFACTS_ROOT=${platformArtifactsDir}"
        "VMCTL_BASE_BLOB_ROOT=${platformArtifactsDir}/computer-inputs/blobs"
        "VMCTL_PROMOTION_AUTHORITY_PUBLIC_KEY=oEdoKFfUCLiOsxNX5J8bT3PQDrjqjVeuU8usTLHSYZ4="
        # Firecracker VM configuration (VAL-VM-010):
        # Guest images are built from the repo via `nix build .#guest-image`.
        # The microvm.nix approach produces:
        #   - vmlinux (kernel)
        #   - rootfs.ext4 (boot disk / guest root filesystem)
        #   - initrd (for systemd module loading)
        #   - storedisk.erofs (shared nix store)
        # pkgs.firecracker is 1.16.2 via flake.nix choirFirecrackerModule
        # (one package: this binary and VM_JAILER_BIN below move together).
        "VM_FIRECRACKER_BIN=${pkgs.firecracker}/bin/firecracker"
        # VMM jailer slice 1: off until staging proves it on one test
        # computer. Rollout (FP8 station, SEC-04): VMCTL_JAILER=on with
        # VMCTL_JAILER_ONLY=<that computer's id>, verify with
        # scripts/vmm_confinement_verify.py; then VMCTL_JAILER_ONLY=class:test
        # (every test account's computer); then drop VMCTL_JAILER_ONLY.
        # Jailed VMs share one inode per store image (RES-05). Rollback:
        # VMCTL_JAILER=off (new launches go direct; a jailed snapshot is
        # refused and the computer cold boots).
        "VMCTL_JAILER=off"
        "VM_JAILER_BIN=${pkgs.firecracker}/bin/jailer"
        "VMCTL_JAILER_CGROUP_PARENT=system.slice/go-choir-vmctl.service/vms"
        "VM_MKFS_EXT4_BIN=${pkgs.e2fsprogs}/bin/mkfs.ext4"
        "VM_KERNEL_IMAGE=/var/lib/go-choir/guest/vmlinux"
        "VM_ROOTFS_IMAGE=/var/lib/go-choir/guest/rootfs.ext4"
        "VM_INITRD_IMAGE=/var/lib/go-choir/guest/initrd"
        "VM_STORE_DISK_IMAGE=/var/lib/go-choir/guest/storedisk.erofs"
        "VM_KERNEL_PARAMS_FILE=/var/lib/go-choir/guest/kernel-params"
        "VM_STATE_DIR=/var/lib/go-choir/vm-state"
        "VM_HOST_BASE_PORT=9000"
        "VM_CPU_COUNT=2"
        "VM_MEM_MIB=4096"
        # Interactive guest shape: 4 CPU / 8 GiB. A 4 GiB trial on the
        # compacted owner store thrashed (3.67 GB RSS at the cap, 78 ms guest
        # RTT, health unreachable) and was rolled back per the owner-approved
        # plan (docs/problems/guest-store-history-bloat-and-memory-shape-
        # 2026-10-09.md). Host swap is bounded by the memory budget, not by
        # guest size. This is the only memory setting.
        "VM_INTERACTIVE_CPU_COUNT=4"
        "VM_INTERACTIVE_MEM_MIB=8192"
        "VM_HEALTH_CHECK_INTERVAL=15s"
        "VM_HEALTH_CHECK_TIMEOUT=10s"
        "VM_BOOT_READY_TIMEOUT=30m"
        # A deploy refresh answers only after the computer boots; a replaying
        # boot of the paper takes about 4 min. The server default (120s)
        # emptied the reply of a refresh that succeeded and failed the deploy
        # (problems/deploy-refresh-empty-reply-vmctl-write-timeout-2026-10-16.md).
        # The deploy's refresh curl gives up at 600s, before this.
        "SERVER_WRITE_TIMEOUT=10m30s"
        # Recovery replay: guest Dolt checkpoint commits at the 5-8 GiB workspace
        # scale exceed 120s on the 4096 MiB guest (stall-gate fired mid-commit
        # at seq 99,765 / 103,148 / 105,196 during recovery boots). B10 range
        # allows 120-1500s; the ceiling gives a legitimate commit room while
        # still failing a truly frozen replay inside the 30m boot window.
        "VM_REPLAY_STALL_TIMEOUT=1500s"
        # Free page reporting (virtio balloon, never inflated): freed guest
        # memory returns to the host while the guest runs. Applies at each
        # computer's next cold boot. Rollback: delete this line
        # (docs/reports/lr-guest-memory-return-2026-10-14.md).
        "VM_BALLOON_FREE_PAGE_REPORTING=1"
        # RES-10 balloon statistics (internal/vmmanager/balloon_stats.go):
        # deliberately NOT set. Staged per computer like the FP8 features:
        #   "VM_BALLOON_STATS_ONLY=<one test computer id>"   then class:test
        #   "VM_BALLOON_STATS_POLLING_INTERVAL_S=5"          optional, default 5
        # A named computer cold boots once (the balloon is in the snapshot
        # launch-config hash); unnamed computers keep their exact config.
        # Read: GET localhost:8083/internal/vmctl/memory/balloon (host only),
        # or scripts/res10_balloon_stats_probe.sh. Rollback: delete the line.
        # RES-06 guest memory return (internal/vmmanager/guest_reclaim.go,
        # FP8 station): DAMON_RECLAIM boot arguments for the computers this
        # selector names, so cold guest page cache is freed and free page
        # reporting returns it to the host. Staged: one disposable test
        # computer first (FP8 proof step 8 compares its host RSS with a
        # control), then class:test, then the paper. A named computer cold
        # boots once (boot arguments are in the launch-config hash).
        # The first test computer (9648031f) has not woken since: host
        # admission refuses its start. The second (71afbf14) is the test
        # computer FP0's monitor keeps awake, so its host RSS can be read
        # against its own pre-DAMON readings at similar guest usage.
        # Rollback: delete this line and redeploy.
        "VM_GUEST_RECLAIM_ONLY=computer-9648031f92e131adc1fb65ec455c5903,computer-71afbf1461dffabcce8eeebe558d03bd"
        # SEC-06 capsule egress tunnel identity
        # (internal/vmmanager/capsule_tunnel_identity.go): every computer runs
        # the check in observe (the guest image's
        # CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY); the computers named here get
        # the boot argument choir.capsule_egress_tunnel_identity=enforce and
        # run it in enforce. Owner rule 2026-10-22: one selected test computer
        # in the same deploy that ships observe. Empty: none. Never "*": the
        # global flip is the image line, behind the gate in
        # docs/problems/egress-proxy-tunnel-identity-2026-10-21.md. A named
        # computer cold boots once (boot arguments are in the launch-config
        # hash). Name computer ids only (a class: token misses the platform
        # computer's boot and the resume config). Rollback for a named
        # computer: empty the list and redeploy (it cold boots into
        # observe); the image line set to off does not override it.
        "VM_CAPSULE_TUNNEL_ENFORCE_ONLY=computer-9648031f92e131adc1fb65ec455c5903"
        # Data drive discard (RES-16; docs/problems/guest-data-image-never-
        # trims-2026-10-20.md, docs/problems/node-b-disk-fills-from-the-paper-
        # data-image-2026-10-22.md). Only the computers named here launch
        # Firecracker 1.17 (nix/firecracker-next.nix) and get virtio-blk
        # discard on the data drive; the guest's hourly choir-data-fstrim
        # timer (nix/autoputer-vm.nix) then hole-punches the freed blocks on
        # the host. Owner rule 2026-10-22: one selected test computer first
        # (computer-9648031f, proved in D3); then, owner 22:40 (resource use
        # ahead of short-term availability), every computer, the paper
        # included ("*"). Each computer cold boots once (its snapshot is from
        # the other VMM build). The 1.17
        # hotplug objection is gone: the hotplug region is the same in 1.16.x
        # (docs/problems/memory-hotplug-cannot-work-on-node-b-39-bit-cpu-
        # 2026-10-22.md). Rollback: delete all four lines and redeploy (every
        # computer cold boots again on the pinned build with no discard).
        # Never leave VM_DATA_DISCARD=1 with an empty VM_DATA_DISCARD_ONLY:
        # once the pinned build is 1.17 or newer, an empty selector means
        # every computer, every data drive gains discard, every launch-config
        # hash changes, and the whole fleet's snapshots are refused.
        "VM_FIRECRACKER_NEXT_BIN=${firecrackerNext}/bin/firecracker"
        "VM_FIRECRACKER_NEXT_ONLY=*"
        "VM_DATA_DISCARD=1"
        "VM_DATA_DISCARD_ONLY=*"
        # Memory hotplug (virtio-mem) for test computers only (it cannot plug
        # on Node B's 39-bit CPU; see the problem record above): a selected
        # computer boots with a 4 GiB unplugged region and grows under
        # admitted, TTL-bound leases (capsule jobs that declare memory_mib).
        # Takes effect at each test computer's next cold boot. Rollback:
        # delete both lines (new boots get no device; leases end at their
        # TTL) (docs/reports/memory-hotplug-controller-design-2026-10-14.md).
        "VM_MEMORY_HOTPLUG=on"
        "VM_MEMORY_HOTPLUG_ONLY=class:test"
        # S3 snapshot hibernate (docs/reports/s3-snapshot-hibernate-design-
        # 2026-10-14.md): an idle hibernate fences the guest and writes a
        # Firecracker memory snapshot; the next wake loads it once instead of
        # cold booting. Any refusal or failure falls back to the kill
        # hibernate and a cold (crash) boot. Rollback: remove this line (the
        # code default is kill, and kill mode never loads a snapshot).
        "VM_HIBERNATE_MODE=snapshot"
        "VM_SNAPSHOT_BUDGET_GIB=48"
        # Owner 2026-10-14: overnight hibernation resumes from its snapshot,
        # at least 24 h. The credential window no longer limits it (resume
        # delivers a fresh credential); the budget's oldest-first eviction
        # and the 16 GiB free-space floor still bound disk.
        "VM_SNAPSHOT_MAX_AGE=36h"
        # L3a Diff snapshots (RES-13; internal/vmmanager/snapshot_diff.go):
        # a hibernate of a cold-booted, selected guest writes only its
        # resident pages. Full and no computer selected: inert. FP0 names
        # one test computer for the proof, never the paper or the owner's
        # computer. Rollback: set the list empty again and deploy.
        "VM_SNAPSHOT_TYPE=full"
        "VM_SNAPSHOT_DIFF_ONLY="
        "VMCTL_STOP_MANAGED_ON_EXIT=false"
        # visual capture (docs/design/visual-evidence-slice-2026-10-15.md):
        # the right to launch the guest image's Chromium in a capsule capture
        # job. OFF while the list is empty (resumed 2026-10-20: no computer is
        # granted until the staging drill is scheduled; the guest image
        # carries Chromium on no PATH either way). To turn it on: ComputerIDs
        # and/or class:test (every test account's computer), space or comma
        # separated. A computer gets the grant at its next cold boot (a
        # snapshot resume keeps its old credential disk). A capture runs
        # inside the engineering capsule's existing 2 GiB / 1 CPU budget
        # (0.4-0.55 GiB per shot, measured). Rollback: empty the list and
        # deploy.
        # 2026-10-20: one test computer (visual-evidence-20261020) for the
        # --visual staging drill; owner scope "one test computer first".
        "CHOIR_VISUAL_CAPTURE_COMPUTERS=computer-7011602c92710d9cf13ee52fd2675908"
        # capsule package mirrors (station FP10,
        # docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md):
        # point the named computers' L2 capsules at go-choir-pkgmirror for
        # the listed ecosystems (npm, pypi, crates, git). OFF while either
        # list is empty. Each ecosystem must also be on in the
        # go-choir-pkgmirror unit's -ecosystems, or the capsule's tools are
        # pointed at a prefix that answers 404. MIRRORS_ONLY=1 also takes the
        # mirrored registries off those capsules' CONNECT allowlist. A
        # computer reads the grant at its next cold boot. Rollback: empty the
        # computer list and deploy.
        "CHOIR_CAPSULE_MIRROR_COMPUTERS="
        "CHOIR_CAPSULE_MIRROR_ECOSYSTEMS="
        "CHOIR_CAPSULE_MIRRORS_ONLY="
        # Host fetch route (SDC NET-01): the grant half. See hostFetchVMs.
        "CHOIR_HOST_FETCH_VMS=${hostFetchVMs}"
        # claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md;
        # Option T, owner-ratified 2026-10-14). OFF while the list is empty.
        # To turn it on for one test computer: put its ComputerID in
        # CHOIR_CLAUDE_DELEGATE_COMPUTERS, keep the expiry within the
        # experiment window, deploy, then cold-boot that computer (stop and
        # start; a snapshot resume keeps its old credential disk). The owner
        # puts the token at ${claudeDelegateTokenFile} (root, 0400).
        # Rollback: empty the list and deploy; revoke the token.
        # 2026-10-15 ~04:00 UTC: OFF again by owner request — the owner's
        # subscription is near its limit and must not be spent by Choir.
        # The token file stays installed; re-add the test computer
        # (computer-33594053a05f46e93d80e7e15ce8a0d2) to turn it back on.
        # 2026-10-20: ON again, on Sonnet, by owner request (owner notes §28;
        # docs/problems/claude-delegate-resume-sonnet-2026-10-20.md). Two-week
        # expiry, renewed only by an explicit edit.
        # The 10-14 test computer is gone (ownership 404); replacement minted 2026-10-20.
        "CHOIR_CLAUDE_DELEGATE_COMPUTERS=computer-ad1b46d4c20129e84b9e239cf4720fb9"
        "CHOIR_CLAUDE_DELEGATE_EXPIRES=2026-11-03T00:00:00Z"
        "CHOIR_CLAUDE_DELEGATE_TOKEN_FILE=${claudeDelegateTokenCopy}"
        # Keep personal computers resident while the host is under capacity.
        "VMCTL_IDLE_TIMEOUT=30m"
        "VMCTL_IDLE_SWEEP_INTERVAL=2m"
        "VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity"
        # Active reclaim uses the same ranking exposed by dry-run mode, but
        # hibernates a bounded number of lower-priority idle computers when
        # host pressure crosses threshold.
        "VMCTL_PRESSURE_RECLAIM_MODE=active"
        "VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m"
        "VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_MIB=4096"
        "VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT=15"
        "VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_MIB=32768"
        "VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_PERCENT=10"
        "VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG10=1.0"
        "VMCTL_PRESSURE_MAX_CPU_SOME_AVG10=90.0"
        "VMCTL_PRESSURE_MAX_IO_SOME_AVG10=5.0"
        "VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES=5"
        # Density quick tier (docs/problems/pressure-reclaim-wedged-a-user-
        # computer-2026-10-14.md): reclaim only on a sustained shortage
        # (memory some avg60 > 5 AND MemAvailable under the floor, 4 samples
        # 15 s apart), one computer at a time, 3 min cool-down after any
        # hibernate; avg10 and MAX_CANDIDATES above then only report. Admit
        # starts by measured memory with reservations for starts in flight.
        # Rollback: SIGNAL=instant and ADMISSION_MODE=guest-size restore the
        # single-sample reclaim and the floor + 8 GiB admission.
        "VMCTL_PRESSURE_RECLAIM_SIGNAL=sustained"
        "VMCTL_ADMISSION_MODE=measured"
        # A proof-account computer busy longer than this (e.g. stuck
        # mid-apply) loses busy protection under host pressure.
        "VMCTL_STALE_STATE_MIN_AGE=6h"
        "VMCTL_STALE_STATE_MAX_DELETES=25"
        # Codex-created staging/product-proof accounts use the example.com and
        # example.test domains. Their hibernated primary VM state is not a
        # rollback primitive, so vmctl may delete it after a day while keeping
        # real-user computers protected.
        "VMCTL_RETENTION_PRUNE_MODE=active"
        "VMCTL_RETENTION_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        "VMCTL_RETENTION_EPHEMERAL_EMAIL_DOMAINS=example.com,example.test"
        "VMCTL_RETENTION_EPHEMERAL_USER_PREFIXES=diagnostic-,sourcemaxx-proof-"
        "VMCTL_RETENTION_ORPHAN_MIN_AGE=6h"
        "VMCTL_RETENTION_EPHEMERAL_MIN_AGE=24h"
        "VMCTL_RETENTION_MAX_DELETES=100"
        "VMCTL_RETENTION_MAX_BYTES_MIB=122880"
        # Shadow retention mirrors the active policy as an observation endpoint
        # so reports can compare deletion pressure before and after sweeps.
        "VMCTL_RETENTION_SHADOW_PRUNE_MODE=dry-run"
        "VMCTL_RETENTION_SHADOW_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
        "VMCTL_RETENTION_SHADOW_EPHEMERAL_EMAIL_DOMAINS=example.com,example.test"
        "VMCTL_RETENTION_SHADOW_EPHEMERAL_USER_PREFIXES=diagnostic-,sourcemaxx-proof-"
        "VMCTL_RETENTION_SHADOW_ORPHAN_MIN_AGE=6h"
        "VMCTL_RETENTION_SHADOW_EPHEMERAL_MIN_AGE=24h"
        "VMCTL_RETENTION_SHADOW_MAX_DELETES=100"
        "VMCTL_RETENTION_SHADOW_MAX_BYTES_MIB=122880"
        # The test-class idle rule and the quarantine age-out
        # (docs/problems/vm-state-retention-skips-test-computers-and-
        # quarantines-2026-10-22.md), observed first: the shadow plan lists
        # every test-class computer and quarantined data image with bytes,
        # age and verdict; it never deletes. The active plan's switches
        # (VMCTL_RETENTION_TEST_IDLE_MODE, VMCTL_RETENTION_QUARANTINE_MODE)
        # stay unset (off) until FP0 has read the shadow report.
        "VMCTL_RETENTION_SHADOW_TEST_IDLE_MODE=dry-run"
        "VMCTL_RETENTION_SHADOW_TEST_IDLE_MIN_AGE=168h"
        "VMCTL_RETENTION_SHADOW_QUARANTINE_MODE=dry-run"
        "VMCTL_RETENTION_SHADOW_QUARANTINE_MIN_AGE=168h"
        # Gateway URL for issuing autoputer credentials to VM guests.
        # vmctl calls this endpoint to get a token before booting each VM.
        "VMCTL_GATEWAY_URL=http://127.0.0.1:8084"
        "VMCTL_CORPUSD_URL=http://127.0.0.1:8086"
        "VMCTL_PLATFORM_WIRE_ENABLED=true"
        "VMCTL_AUTOPUTER_PROXY_SOCK=/run/go-choir/vmctl.sock"
        internalAuthEnv
      ] ++ doltClientEnv ++ [
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
      LoadCredential = [ "choir-internal-token.header:${internalTokenHeaderFile}" ];
      Environment = [
        # Emergency floor remains the lower bound. The higher target runs
        # routine Nix GC before storage pressure reaches recovery territory.
        "GO_CHOIR_DISK_GC_MIN_FREE_KIB=125829120"
        "GO_CHOIR_DISK_GC_TARGET_FREE_KIB=188743680"
        "GO_CHOIR_NIX_RETENTION_MODE=observe"
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

  # Nix store retention between the daily sweeps. Every deploy adds a system
  # generation whose closure roots a new 3.26 GB guest image; at ~20 deploys a
  # day the daily sweep let 25 generations (~80 GB of images) pile up
  # (docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md). Which
  # generations and builder-release roots stay is nixRetention's (above);
  # this unit runs it and collects unrooted paths every 4 hours. Prebuild
  # and deploy out-links stay GC roots. Only Nix state is touched: no VM,
  # journal or vmctl work. Rollback: remove this unit and timer (the daily
  # sweep remains).
  systemd.services.go-choir-nix-gc = {
    description = "go-choir Nix generation and store retention";
    serviceConfig = {
      Type = "oneshot";
      Nice = 19;
      IOSchedulingClass = "idle";
      Environment = [ "GO_CHOIR_NIX_RETENTION_MODE=observe" ];
      ExecStart = pkgs.writeShellScript "go-choir-nix-gc" ''
        set -euo pipefail
        export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.gawk pkgs.gnused pkgs.nix ]}:$PATH"
        df -h /
        ${nixRetention}
        nix store gc
        df -h /
      '';
    };
  };

  systemd.timers.go-choir-nix-gc = {
    description = "go-choir Nix generation and store retention every 4 hours";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "*-*-* 00/4:30:00";
      RandomizedDelaySec = "15m";
      Persistent = true;
    };
  };

  # btrfs chunk headroom for metadata. The platform artifact store writes
  # millions of small files, which btrfs keeps inline in metadata; when every
  # chunk is allocated to data, metadata cannot grow and writes fail with
  # ENOSPC while df shows 135 GB free
  # (docs/problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).
  # Once a day, if unallocated space is under 30 GiB, relocate at most 20
  # data chunks that are at most 20% used, returning them as unallocated
  # space. Bounded and online; no file content changes.
  # Rollback: remove this unit and timer.
  systemd.services.go-choir-btrfs-headroom = {
    description = "go-choir btrfs unallocated-space headroom for metadata";
    serviceConfig = {
      Type = "oneshot";
      Nice = 19;
      IOSchedulingClass = "idle";
      ExecStart = pkgs.writeShellScript "go-choir-btrfs-headroom" ''
        set -euo pipefail
        export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.gawk pkgs.btrfs-progs ]}:$PATH"
        unallocated=$(btrfs filesystem usage -b / | awk '/Device unallocated:/ {print $3}')
        echo "btrfs unallocated: $((unallocated / 1073741824)) GiB"
        if [ "$unallocated" -ge $((30 * 1073741824)) ]; then
          exit 0
        fi
        btrfs balance start -dusage=20,limit=20 /
        btrfs filesystem usage -b / | awk '/Device unallocated:/ {print "btrfs unallocated after balance: " int($3 / 1073741824) " GiB"}'
      '';
    };
  };

  systemd.timers.go-choir-btrfs-headroom = {
    description = "Daily go-choir btrfs metadata headroom check";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      RandomizedDelaySec = "1h";
      Persistent = true;
    };
  };

  # Host disk alarm (scripts/node-b-host-disk-alarm; tests:
  # scripts/test-node-b-host-disk-alarm.sh;
  # docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md). Every
  # 15 minutes, one journal line with root free space, btrfs unallocated
  # space and metadata use: priority warning under the deploy floor (60 GiB)
  # or 20 GiB unallocated (go-choir-btrfs-headroom balances under 30), error
  # under nix min-free (24 GiB) or 10 GiB unallocated, or when a reading
  # fails. Read-only; always exits 0. See alarms:
  #   journalctl -u go-choir-host-disk-alarm -p warning
  # Rollback: remove this service and timer.
  systemd.services.go-choir-host-disk-alarm = {
    description = "go-choir host disk alarm (free space, btrfs unallocated)";
    serviceConfig = {
      Type = "oneshot";
      Nice = 19;
      IOSchedulingClass = "idle";
      TimeoutStartSec = "1min";
      Environment = [
        "PATH=${lib.makeBinPath [ pkgs.coreutils pkgs.gawk pkgs.btrfs-progs ]}"
        "HOST_DISK_WARN_FREE_GIB=60"
        "HOST_DISK_WARN_UNALLOC_GIB=20"
        "HOST_DISK_ERR_FREE_GIB=24"
        "HOST_DISK_ERR_UNALLOC_GIB=10"
      ];
      ExecStart = pkgs.writeShellScript "go-choir-host-disk-alarm"
        (builtins.readFile ../scripts/node-b-host-disk-alarm);
    };
  };

  systemd.timers.go-choir-host-disk-alarm = {
    description = "go-choir host disk alarm every 15 minutes";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "*:0/15";
      # A missed reading is not resumed; the next one is current.
      Persistent = false;
    };
  };

  # Guest store compaction (docs/problems/guest-store-compaction-scheduled-
  # 2026-10-20.md; owner notes §26). vmctl compacts, one at a time, every
  # active unheld platform computer (critical_protected, public_platform)
  # whose data image holds at least 32 GiB on the host, as a planned restart
  # with offline maintenance; a busy refusal waits for the next day. The
  # paper grows about 6 GB a day, so it is compacted every few days. An
  # account's computer is never compacted by the timer.
  systemd.services.go-choir-store-compaction = {
    description = "go-choir guest store compaction sweep";
    after = [ "go-choir-vmctl.service" ];
    requires = [ "go-choir-vmctl.service" ];
    serviceConfig = {
      Type = "oneshot";
      TimeoutStartSec = "3h";
      LoadCredential = [ "choir-internal-token.header:${internalTokenHeaderFile}" ];
      ExecStart = pkgs.writeShellScript "go-choir-store-compaction" ''
        set -euo pipefail
        ${pkgs.curl}/bin/curl -sS --fail-with-body --max-time 10800 \
          --unix-socket /run/go-choir/vmctl.sock \
          -X POST http://vmctl/internal/vmctl/compact-sweep \
          -H "@''${CREDENTIALS_DIRECTORY}/choir-internal-token.header" \
          -H 'Content-Type: application/json' \
          -H 'X-Choir-Lifecycle-Caller: go-choir-store-compaction-timer' \
          -d '{"min_file_bytes":34359738368}'
        echo
      '';
    };
  };

  systemd.timers.go-choir-store-compaction = {
    description = "Daily go-choir guest store compaction sweep";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "*-*-* 04:00:00 UTC";
      Persistent = false;
    };
  };

  # Autopaper daily digest (docs/reports/autopaper-daily-digest-design-2026-10-20.md,
  # docs/problems/autopaper-daily-digest-phase1-2026-10-20.md). A deterministic
  # host job: this oneshot makes one call to maild's host-internal run route,
  # and maild reads the front list, picks stories with a fixed function and
  # sends one fixed-template mail through its existing Resend sender to each
  # owner who opted in (the Mail app toggle; nobody is opted in by default). No
  # agent, no computer, no model call.
  # Auth: the call carries the host-internal credential header from the unit's
  # private $CREDENTIALS_DIRECTORY, never loopback alone; if the credential
  # cannot be read the unit fails instead of calling without it. curl fails on
  # an HTTP error, so a missing Resend key (maild answers 503) or a provider
  # failure (502) shows as a failed unit and a journal line.
  # Persistent = false: after a host outage the timer does not fire a stale
  # digest; a missed day is closed, not resumed (CLAUDE.md "Restarts End Work").
  # Rollback: MAILD_DIGEST=off on go-choir-maild (the run route does nothing),
  # or `systemctl disable --now go-choir-digest.timer`, or revert this block.
  systemd.services.go-choir-digest = {
    description = "Autopaper daily digest (deterministic host job; no agent, no model)";
    after = [ "go-choir-maild.service" "network-online.target" ];
    wants = [ "go-choir-maild.service" ];
    serviceConfig = {
      Type = "oneshot";
      TimeoutStartSec = "6min";
      PrivateTmp = true;
      NoNewPrivileges = true;
      LoadCredential = [ "choir-internal-token.header:${internalTokenHeaderFile}" ];
      ExecStart = pkgs.writeShellScript "go-choir-digest" ''
        set -euo pipefail
        if [ -z "''${CREDENTIALS_DIRECTORY:-}" ] || [ ! -r "''${CREDENTIALS_DIRECTORY}/choir-internal-token.header" ]; then
          echo "go-choir-digest: host-internal credential header is not readable; refusing to call maild without it" >&2
          exit 1
        fi
        ${pkgs.curl}/bin/curl -sS --fail-with-body --max-time 300 \
          -X POST \
          -H 'X-Internal-Caller: true' \
          -H "@''${CREDENTIALS_DIRECTORY}/choir-internal-token.header" \
          http://127.0.0.1:8087/internal/maild/digest/run
        echo
      '';
    };
  };

  systemd.timers.go-choir-digest = {
    description = "Daily Autopaper digest at 12:00 UTC";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "*-*-* 12:00:00 UTC";
      Persistent = false;
    };
  };

  # Store A weekly history squash, offline (docs/problems/store-a-weekly-squash-
  # 2026-10-20.md). The script stops the Dolt unit's timers and Requires=
  # dependents and the unit, takes a reflink backup of the data directory, runs
  # platform-dolt-squash (squash to one commit, bounded full GC, refuse unless
  # the root hash is identical), restores the backup on any failure, starts
  # everything again, runs a query check and keeps only the newest backup.
  # This unit deliberately has no After/Requires/Wants/PartOf on the Dolt unit:
  # it stops that unit and must not be stopped with it. ExecStopPost runs
  # `--finish`, so an OOM kill of the run cannot leave Store A down or a torn
  # squash in place. It never touches vmctl.
  # Memory: MemoryMax without MemoryHigh and no swap, as the paper's proven run
  # (MemoryHigh made it thrash on 2026-10-19); the command also sets a Go
  # memory limit of 5 GiB.
  # First run by hand: docs/problems/store-a-weekly-squash-2026-10-20.md.
  systemd.services.go-choir-platform-dolt-squash = {
    description = "go-choir Store A (platform Dolt) weekly history squash";
    path = [ pkgs.coreutils pkgs.gawk pkgs.gnugrep pkgs.gnused pkgs.systemd pkgs.dolt ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = platformDoltSquashScript;
      ExecStopPost = "${platformDoltSquashScript} --finish";
      TimeoutStartSec = "2h";
      MemoryMax = "10G";
      MemorySwapMax = "0";
      Nice = 10;
      CPUWeight = 20;
      IOWeight = 20;
      RuntimeDirectory = "go-choir-platform-dolt-squash";
      RuntimeDirectoryMode = "0700";
      # The query check reuses the liveness probe's --sql mode, which reads the
      # app user's password from the credential directory and never prints it.
      LoadCredential = [ "dolt-app-password:${doltAppPasswordFile}" ];
      Environment = [
        "HOME=${platformDoltDir}"
        "LD_LIBRARY_PATH=${goServiceLibraryPath}"
        "SQUASH_BIN=${goChoirPackages.platformDoltSquash}/bin/platform-dolt-squash"
        "DOLT_UNIT=go-choir-platform-dolt.service"
        "DATA_DIR=${platformDoltDir}"
        "DATABASE=platform"
        "BACKUP_ROOT=${platformDoltSquashBackupRoot}"
        "SQL_CHECK_CMD=${doltLivenessProbe} --sql 13306 platform"
        "SQUASH_NOTE=weekly"
      ];
    };
  };

  # DISABLED until the first supervised run: no wantedBy while
  # platformDoltSquashTimerEnabled is false. Persistent=false so a missed
  # window is skipped, never replayed at boot.
  systemd.timers.go-choir-platform-dolt-squash = {
    description = "Weekly Store A history squash";
    wantedBy = lib.optional platformDoltSquashTimerEnabled "timers.target";
    timerConfig = {
      OnCalendar = "Sun *-*-* 06:00:00 UTC";
      Persistent = false;
    };
  };

  systemd.services.go-choir-platform-dolt-history-audit = {
    description = "go-choir platform dolt commit-history growth audit";
    after = [ "go-choir-platform-dolt.service" ];
    requires = [ "go-choir-platform-dolt.service" ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = platformDoltHistoryAudit;
    };
  };

  systemd.timers.go-choir-platform-dolt-history-audit = {
    description = "Daily platform dolt commit-history growth audit";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      RandomizedDelaySec = "30m";
      Persistent = true;
    };
  };

  # Off-host backup to node-a (see offHostBackup in the let block). Every
  # attribute below is behind offHostBackup.enable. The daily job is append-only on the remote and
  # cannot delete backups; pruning is a supervised monthly act (design,
  # section 5). Persistent = false on every timer: a missed run is never
  # replayed at boot (CLAUDE.md, "Restarts End Work").
  systemd.services.go-choir-offhost-backup = lib.mkIf offHostBackup.enable (offHostBackupUnit
    "go-choir daily encrypted backup to node-a (restic, append-only)" "backup" {
      # In a host OOM the backup dies before any guest. At +1000 it ties
      # only with nix-daemon builds, checkpointd and front-importance, and
      # outscores them by size.
      OOMScoreAdjust = 1000;
      LoadCredential = offHostBackupCredentials ++ [ "dolt-root-password:${doltRootPasswordFile}" ];
    } // {
      after = [ "network-online.target" "go-choir-platform-dolt.service" "go-choir-corpus-dolt.service" ];
      wants = [ "network-online.target" ];
    });

  systemd.timers.go-choir-offhost-backup = lib.mkIf offHostBackup.enable {
    description = "Daily encrypted backup to node-a";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "*-*-* 02:30:00 UTC";
      RandomizedDelaySec = "15m";
      Persistent = false;
    };
  };

  # Weekly: re-read and re-hash a rotating tenth of the repository.
  systemd.services.go-choir-offhost-backup-verify = lib.mkIf offHostBackup.enable
    (offHostBackupUnit "go-choir weekly off-host backup verification (restic check, rotating subset)" "verify" {
      OOMScoreAdjust = 1000;
      # Fewer tree loaders in `restic check` (connections + GOMAXPROCS + 1,
      # each holding a decoded tree; the script sets two connections): its
      # 10.3 GB plus 4 GB of swap (no-off-host-backup record, section 13
      # item 18) has no swap to go to now.
      Environment = offHostBackupEnv ++ [ "GOMAXPROCS=2" ];
      LoadCredential = offHostBackupCredentials;
    });

  systemd.timers.go-choir-offhost-backup-verify = lib.mkIf offHostBackup.enable {
    description = "Weekly off-host backup verification";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "Sun *-*-* 06:00:00 UTC";
      RandomizedDelaySec = "30m";
      Persistent = false;
    };
  };

  # Every 6 h: the unit FAILS when the last good backup is older than 36 h,
  # the last run was partial, no verify passed in 10 days, or the repository
  # is over 80% of the node-a quota. Failure is visible in `systemctl
  # --failed` and the journal; paging is still a named gap.
  systemd.services.go-choir-offhost-backup-freshness = lib.mkIf offHostBackup.enable
    (offHostBackupUnit "go-choir off-host backup freshness check" "freshness" { });

  systemd.timers.go-choir-offhost-backup-freshness = lib.mkIf offHostBackup.enable {
    description = "Off-host backup freshness check every 6 hours";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnBootSec = "2h";
      OnUnitActiveSec = "6h";
      Persistent = false;
    };
  };

  # Platform (13306) and corpus (13307) Dolt liveness: SELECT 1 with a 5 s
  # deadline every 30 s. Three consecutive timeouts or refusals on a unit that
  # has been active for 2+ minutes journal /proc state, send SIGABRT (Go dumps
  # all goroutines) and let Restart=on-failure bring Dolt back; a 10 minute
  # cooldown bounds signals per unit. The app user's password arrives via
  # LoadCredential (the file the dolt-users units use) and is never printed.
  # Dry run by hand (sends nothing): systemd-run --wait -p Type=oneshot
  #   -p LoadCredential=dolt-app-password:<file> -E PROBE_DRY_RUN=1 <ExecStart>
  # Rollback: remove this service and timer; the Dolt unit changes
  # (TimeoutStopSec, LimitCORE, GOTRACEBACK) are independent.
  systemd.services.go-choir-dolt-liveness-probe = {
    description = "go-choir Dolt liveness probe (platform :13306, corpus :13307)";
    path = [ pkgs.coreutils pkgs.dolt pkgs.systemd pkgs.gnugrep pkgs.gnused pkgs.gawk pkgs.util-linux ];
    serviceConfig = commonServiceHardening // {
      Type = "oneshot";
      ExecStart = doltLivenessProbe;
      TimeoutStartSec = "2min";
      RuntimeDirectory = "go-choir-dolt-liveness";
      RuntimeDirectoryMode = "0700";
      RuntimeDirectoryPreserve = "yes";
      LoadCredential = [ "dolt-app-password:${doltAppPasswordFile}" ];
      InaccessiblePaths = hiddenSecretsExcept [ "systemdPrivate" "dbusSystemSocket" ];  # it runs systemctl kill, show and restart
      # Reading /proc/<dolt pid>/{stack,io,wchan} for the evidence record can
      # need ptrace access to another process under Yama.
      CapabilityBoundingSet = [ "CAP_SYS_PTRACE" ];
    };
  };

  systemd.timers.go-choir-dolt-liveness-probe = {
    description = "Probe platform and corpus Dolt with SELECT 1 every 30 seconds";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      # Dolt and its users unit need a moment after boot; the script also
      # skips any unit that has been active for under 2 minutes.
      OnBootSec = "2min";
      OnUnitActiveSec = "30s";
      AccuracySec = "1s";
    };
  };

  # Paper liveness: alert in the journal (priority err) when the paper computer
  # made no inference call for 30 minutes, judged from the gateway journal line
  # "inference succeeded for autoputer <vm>" (scripts/paper-liveness-probe.py;
  # tests: scripts/test_paper_liveness_probe.py;
  # docs/problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md).
  # The paper's vm id is looked up, not hardcoded: the platform computer in the
  # tracked tiers file, mapped to its vm_id by vmctl's ownerships.json. The
  # probe reads only. See alerts:  journalctl -u go-choir-paper-liveness-probe -p err
  # Rollback: remove this service and timer.
  systemd.services.go-choir-paper-liveness-probe = {
    description = "go-choir paper liveness probe (no inference call for 30 minutes)";
    path = [ pkgs.coreutils pkgs.systemd ];
    environment = {
      PAPER_TIERS_FILE = "${../internal/computertiers/computer-tiers.json}";
      OWNERSHIPS_FILE = "/var/lib/go-choir/vm-state/ownerships.json";
    };
    serviceConfig = commonServiceHardening // {
      Type = "oneshot";
      ExecStart = "${pkgs.python3}/bin/python3 -I ${../scripts/paper-liveness-probe.py}";
      TimeoutStartSec = "1min";
      InaccessiblePaths = hiddenSecretsExcept [ ];
    };
  };

  systemd.timers.go-choir-paper-liveness-probe = {
    description = "Check the paper made an inference call in the last 30 minutes, every 5 minutes";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      # The probe itself skips while the host has been up for under 30 minutes.
      OnBootSec = "10min";
      OnUnitActiveSec = "5min";
      AccuracySec = "10s";
    };
  };

  systemd.services.go-choir-gateway = {
    # The deploy restarts this unit once, after the service pointer holds the
    # new binary; a switch-time restart ran the old pointer and held the site
    # down while the gateway drained (L1, deploy-without-site-outage design;
    # vmctl precedent 2f755bb3).
    restartIfChanged = false;
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Gateway Service";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" "searx.service" ];
    wants = [ "network-online.target" "searx.service" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = "${serviceExec "gateway" goChoirPackages.gateway}";
      Restart = "on-failure";
      RestartSec = 3;
      TimeoutStopSec = "11min";
      # B7 review (bunch 88, F1; bunch 109; bunch 116; FP0 round 5): a
      # backstop under the in_flight body bound, the image decode slots and
      # the downscaled output budget, never a bound itself. No measurement
      # of the gateway's normal RSS is in the repo. The unit's worst case,
      # every term at its bound at once (internal/gateway TestBodyCapsUnit-
      # MemoryArithmeticFitsUnderGOMEMLIMIT computes it from the constants):
      #   bodies in flight, at most 1 GiB, three copies each: the JSON
      #   decoder's buffer (up to twice the body, during the decode only),
      #   or the decoded request and its upstream body (during the call);
      #   a body is never in both phases, so 3 x 1 GiB = 3.00 GiB;
      #   image decodes, at most 3 at once (provider.MaxConcurrentImageDecodes,
      #   1 per computer), each up to 50 Mpx: 400 MB decoded (16-bit PNG)
      #   + 200 MB RGBA copy + its 20 MiB input, decoded from base64 inside
      #   the slot, about 620 MB, so 3 x 620 MB = 1.73 GiB;
      #   downscaled output, at most 192 MiB of base64 held by all calls
      #   (provider.MaxImageOutputBytes; one computer holds at most
      #   max(96 MiB, its one call's 133.3 MiB)), three copies
      #   a byte: the string, the upstream body, the encoder's growth by
      #   doubling: 0.56 GiB;
      #   the runtime and everything else, about 0.5 GiB;
      # about 5.80 GiB: 0.2 GiB under GOMEMLIMIT 6GiB and 2.20 GiB under
      # MemoryMax (systemd reads 8G as 8 GiB).
      # Four decode slots left no room for the output (6.38 GiB). Without
      # the decode bound, 24 calls (scheduler on) or 60 (off) decoding at once were
      # 7 to 18 GB. MemoryMax without MemoryHigh, as the squash unit: throttling
      # would stall every tenant's calls; GOMEMLIMIT (below) makes the Go
      # collector work harder before the kill. Tighten after /health and
      # MemoryPeak (proof 10) give a measurement.
      MemoryMax = "8G";
      MemorySwapMax = "0";
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
      #   # ChatGPT search reuses the same auth file; optional tuning:
      #   CHATGPT_SEARCH_MODEL=gpt-6-luna
      #   CHATGPT_SEARCH_REASONING_EFFORT=low
      #   CHATGPT_SEARCH_CONTEXT_SIZE=low
      #   # Search Provider Keys
      #   # (SearXNG is free and self-hosted — no key needed, just SEARXNG_URL
      #   #  which is set in the Environment block below)
      #   TAVILY_API_KEY=...
      #   BRAVE_API_KEY=...
      #   EXA_API_KEY=...
      #   SERPER_API_KEY=...
      #   PARALLEL_API_KEY=...
      #   SERPAPI_API_KEY=...
      #   KEENABLE_API_KEY=...
      EnvironmentFile = "-/var/lib/go-choir/gateway-provider.env";
      LoadCredential = internalTokenCredential;
      ReadWritePaths = [ "/var/lib/go-choir" ];
      InaccessiblePaths = hiddenSecretsExcept [ "codexAuth" "gatewayIdentities" "jwtKey" ] ++ [
        "-/var/lib/go-choir/vm-state"
        "-${authSigningDir}"
        "-/var/lib/go-choir/auth"
        "-${platformDoltDir}"
        "-${corpusDoltDir}"
      ];
      Environment = [
        # Guest autoputeres call the host gateway via the tap subnet
        # (172.X.0.1:8084). Keep operator-only credential endpoints locked to
        # loopback at the handler layer, but let the process accept guest
        # traffic on tap addresses.
        "SERVER_HOST=0.0.0.0"
        "SERVER_SHUTDOWN_TIMEOUT=10m30s"
        # Inference and upstream provider calls may run for up to 10m; keep the
        # HTTP response open long enough for the handler to return a sanitized
        # result instead of dropping the guest connection with EOF.
        "SERVER_WRITE_TIMEOUT=10m30s"
        "GATEWAY_PORT=8084"
        # Host fetch route: the admit half (see hostFetchVMs).
        "CHOIR_HOST_FETCH_VMS=${hostFetchVMs}"
        "GATEWAY_IDENTITY_STORE_PATH=/var/lib/go-choir/gateway-identities.json"
        "GATEWAY_CHATGPT_MODELS=gpt-6-luna"
        "GATEWAY_CHATGPT_REASONING_EFFORT=low"
        # Which OpenCode keys serve is policy, not a credential edit (owner,
        # 2026-10-22: "ChatGPT and Opencode api key alt 0 only currently").
        # Names key variables in gateway-provider.env; unlisted keys never
        # serve. OPENCODE_API_KEY answers 401
        # (docs/problems/opencode-401-key-never-parked-2026-10-22.md).
        "GATEWAY_OPENCODE_KEYS=OPENCODE_API_KEY_ALT_0"
        # Tokens are currently issued at autoputer/VM bootstrap and not
        # proactively rotated. Use a longer TTL in staging to avoid
        # authentication lapses during normal multi-hour sessions.
        "GATEWAY_AUTOPUTER_TOKEN_TTL=720h"
        "GATEWAY_BEARER_BINDING=${gatewayBearerBindingMode}"
        "GATEWAY_MODEL_SCHEDULER=${gatewayModelScheduler}"
        "GATEWAY_BODY_CAPS=${gatewayBodyCaps}"
        # Under MemoryMax 8G (above): the collector runs harder past 6 GiB.
        "GOMEMLIMIT=6GiB"
        # SearXNG self-hosted meta-search (free, no credits). SearXNGProvider
        # is first in the provider list, so this absorbs the majority of
        # search load. Paid providers (Tavily, Brave, etc.) act as fallback.
        "SEARXNG_URL=http://127.0.0.1:8888"
        # Keenable free plan (owner, 2026-10-15): 100,000 requests a month,
        # renewing on the 5th, 10 requests/s, no payment method on file.
        # The gateway paces it: each UTC day's cap is (90% of the allowance
        # minus the period's use) / days left, and calls are held to 8/s
        # (default CHOIR_SEARCH_PROVIDER_RPS brave=1,keenable=8). These are
        # the code defaults, written out so the plan is visible here.
        # PERIOD_USED seeds the 10-05..11-05 period with the billing page's
        # count (10,776 used at 10-15 03:40Z): the daily counts began
        # 10-14, so they never saw most of it. It is ignored from 11-05 on;
        # drop it then. docs/problems/search-throughput-keenable-underused-2026-10-15.md
        "CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE=100000"
        "CHOIR_SEARCH_KEENABLE_RENEW_DAY=5"
        "CHOIR_SEARCH_KEENABLE_MONTHLY_TARGET_PCT=90"
        "CHOIR_SEARCH_KEENABLE_PERIOD_USED=10776@2026-10-15"
        internalAuthEnv
      ];
    };
  };

  # Generate SearXNG secret key on first deploy (or reuse existing).
  # The environmentFile substitutes $SEARXNG_SECRET into settings.yml via envsubst.
  system.activationScripts.go-choir-searxng-secret = ''
    if [ ! -f /var/lib/go-choir/searxng.env ]; then
      secret="$(${pkgs.openssl}/bin/openssl rand -hex 32)"
      umask 077
      echo "SEARXNG_SECRET=$secret" > /var/lib/go-choir/searxng.env
      chmod 600 /var/lib/go-choir/searxng.env
      echo "go-choir SearXNG secret key generated"
    fi
  '';

  # Host-internal service credential (internal/hostauth). Generated once;
  # rotate by deleting both files and redeploying (every unit that loads it
  # must restart to pick up the new value). Never printed.
  system.activationScripts.go-choir-internal-token = ''
    install -d -m 0700 -o root -g root /var/lib/go-choir/secrets
    if [ ! -s ${internalTokenFile} ]; then
      ( umask 077
        ${pkgs.openssl}/bin/openssl rand -hex 32 > ${internalTokenFile}.next
        mv -f ${internalTokenFile}.next ${internalTokenFile} )
      rm -f ${internalTokenHeaderFile}
      echo "go-choir internal service credential generated"
    fi
    chmod 0600 ${internalTokenFile}
    if [ ! -s ${internalTokenHeaderFile} ]; then
      ( umask 077
        printf 'X-Choir-Internal-Token: %s\n' "$(cat ${internalTokenFile})" > ${internalTokenHeaderFile}.next
        mv -f ${internalTokenHeaderFile}.next ${internalTokenHeaderFile} )
    fi
    chmod 0600 ${internalTokenHeaderFile}
    if [ ! -s ${issuerTokenFile} ]; then
      ( umask 077
        ${pkgs.openssl}/bin/openssl rand -hex 32 > ${issuerTokenFile}.next
        mv -f ${issuerTokenFile}.next ${issuerTokenFile} )
      echo "go-choir issuer credential generated"
    fi
    chmod 0600 ${issuerTokenFile}
  '';

  # S1 owner-act key (ownerActMode). Ed25519, generated once, PEM, root
  # 0600, never printed. The public key is re-derived whenever it is missing.
  # Rotate by deleting both files and redeploying; the proxy and corpusd
  # must both restart (every deploy restarts them). A key that cannot be
  # generated leaves the proxy minting no receipts, which observe allows.
  system.activationScripts.go-choir-owner-act-key = ''
    install -d -m 0700 -o root -g root /var/lib/go-choir/secrets
    if [ ! -s ${ownerActKeyFile} ]; then
      ( umask 077
        ${pkgs.openssl}/bin/openssl genpkey -algorithm ed25519 -out ${ownerActKeyFile}.next \
          && mv -f ${ownerActKeyFile}.next ${ownerActKeyFile} ) || rm -f ${ownerActKeyFile}.next
      rm -f ${ownerActPublicKeyFile}
      echo "go-choir owner-act signing key generated"
    fi
    if [ -s ${ownerActKeyFile} ] && [ ! -s ${ownerActPublicKeyFile} ]; then
      ( umask 077
        ${pkgs.openssl}/bin/openssl pkey -in ${ownerActKeyFile} -pubout -out ${ownerActPublicKeyFile}.next \
          && mv -f ${ownerActPublicKeyFile}.next ${ownerActPublicKeyFile} ) || rm -f ${ownerActPublicKeyFile}.next
    fi
    chmod 0600 ${ownerActKeyFile} ${ownerActPublicKeyFile} 2>/dev/null || true
  '';

  # Dolt passwords (doltAuthStage). Hex, generated once, 0600, never printed.
  system.activationScripts.go-choir-dolt-passwords = ''
    install -d -m 0700 -o root -g root /var/lib/go-choir/secrets
    for f in ${doltAppPasswordFile} ${doltRootPasswordFile}; do
      if [ ! -s "$f" ]; then
        ( umask 077
          ${pkgs.openssl}/bin/openssl rand -hex 32 > "$f.next"
          mv -f "$f.next" "$f" )
        echo "go-choir Dolt password generated: $f"
      fi
      chmod 0600 "$f"
    done
  '';

  # The env directory (unitEnvDir). Activation runs at boot before units
  # start and in a switch before units restart, so each unit's new
  # namespace is made with the directory present. Per name, by what sits at
  # the old path /var/lib/go-choir/<name>:
  #   - the link unit-env/<name>, or nothing: (re)made the link;
  #   - before the first import (no .imported marker), a regular file or a
  #     link to one elsewhere: imported (a regular file by rename, which keeps
  #     the inode and the old view's mask on it during the landing; a link's
  #     target by copy, the target left in place and named). The old path
  #     wins over a file already in unit-env, which becomes a stray;
  #   - after the import, a regular file or a link to one elsewhere with the
  #     same content as unit-env/<name> (a reverted deploy's scrub): relinked;
  #     with other content, or with no unit-env/<name>: never adopted (a unit
  #     that can write /var/lib/go-choir could have put it there), moved or
  #     copied into the hidden directory as <name>.stray.<epoch>.<pid> and
  #     the old path relinked, so nothing secret stays where every root unit
  #     can read it;
  #   - a dangling link or a link to a non-file: repointed.
  # A link is replaced through a temporary name and a rename, so the old name
  # never goes missing when it is replaced. The activation fails (non-zero,
  # a line on stderr and in the journal naming the path) while any stray
  # waits in unit-env, so the deploy's one switch retry fails too, until the
  # operator merges what is wanted into unit-env/<name> and removes the
  # stray. A link or file planted at the directory's own path is moved aside,
  # never followed. A rollback moves no files (the links serve the reverted
  # units); one that moves them back must also remove unit-env/.imported, so
  # the next landing imports them again
  # (docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md, round 2).
  # Between activations, a regular file put at an old path is readable by
  # every root unit until the next activation quarantines it; the verifier
  # flags it on any run. The marker unit-env/.imported is written only by a
  # first run that passed (every file moved, no obstruction, no stray); until
  # then (an IO failure part way, a FAIL) the env readers' AssertPathExists
  # keeps them down, loudly, and the next activation that passes (a boot,
  # or the deploy's switch retry) finishes the import. A required file
  # missing after it (corpus-dsn.env) has its own assert. Never remove the marker except as the rollback text says. Remove
  # this block and the links once no rollback can reach a commit from before
  # the move.
  system.activationScripts.go-choir-unit-env = ''
    (
      set -eu
      umask 077
      state=/var/lib/go-choir
      d="$state/unit-env"
      tag="$(date +%s).$$"
      fail=0
      say() {
        echo "go-choir-unit-env: $*" >&2
        ${pkgs.util-linux}/bin/logger -t go-choir-unit-env -p daemon.err -- "$*" 2>/dev/null || true
      }
      same() { [ -f "$2" ] && [ "$(cat -- "$1")" = "$(cat -- "$2")" ]; }
      put_link() {
        tmp="$state/.$1.link.$$"
        rm -f "$tmp"
        ln -s "unit-env/$1" "$tmp"
        # mv onto a link to a directory would move into the directory.
        if [ -L "$state/$1" ] && [ -d "$state/$1" ]; then rm -f "$state/$1"; fi
        mv -f "$tmp" "$state/$1"
      }
      take() { # $1 from (a regular file, or a link to one), $2 to: rename a file, copy a link's target
        if [ -L "$1" ]; then
          cat -- "$1" > "$2.part"
          mv "$2.part" "$2"
        else
          mv "$1" "$2"
        fi
      }
      if [ -L "$d" ] || { [ -e "$d" ] && [ ! -d "$d" ]; }; then
        mv "$d" "$d.invalid.$tag"
        say "$d was not a directory; moved it aside to $d.invalid.$tag"
      fi
      mkdir -p "$d"
      if [ "$(id -u)" = 0 ]; then chown root:root "$d"; fi
      chmod 0700 "$d"
      imported=0
      if [ -e "$d/.imported" ]; then imported=1; fi
      for name in ${lib.concatStringsSep " " unitEnvNames}; do
        old="$state/$name"
        new="$d/$name"
        stray="$d/$name.stray.$tag"
        what="a regular file"
        if [ -L "$old" ]; then what="a link to $(readlink "$old")"; fi
        if [ -L "$old" ] && [ "$(readlink "$old")" = "unit-env/$name" ]; then
          :
        elif [ -L "$old" ] && [ ! -f "$old" ]; then
          put_link "$name"
          echo "go-choir-unit-env: repointed $old ($what)"
        elif [ -f "$old" ]; then
          if [ "$imported" = 0 ] && [ ! -e "$new" ] && [ ! -L "$new" ]; then
            take "$old" "$new"
            put_link "$name"
            echo "go-choir-unit-env: imported $old ($what) into $new"
            if [ "$what" != "a regular file" ]; then
              say "the link's target is left in place; remove it if nothing else reads it: $what"
            fi
          elif same "$old" "$new"; then
            put_link "$name"
            echo "go-choir-unit-env: $old ($what) held what $new holds; relinked"
          elif [ "$imported" = 0 ]; then
            mv "$new" "$stray"
            take "$old" "$new"
            put_link "$name"
            say "FAIL: $new existed before the import with other content than $old ($what); kept $old's as $new, quarantined the other as $stray"
            fail=1
          else
            take "$old" "$stray"
            put_link "$name"
            say "FAIL: $old was $what after the import: never adopted (no unit reads it, every root unit could), quarantined as $stray"
            fail=1
          fi
        elif [ -e "$old" ]; then
          say "FAIL: $old is neither a file nor a link; left in place"
          fail=1
        else
          put_link "$name"
        fi
        if [ -f "$new" ] && [ ! -L "$new" ]; then
          if [ "$(id -u)" = 0 ]; then chown root:root "$new"; fi
          chmod 0600 "$new"
        fi
      done
      for q in "$d"/*.stray.*; do
        if [ ! -e "$q" ] && [ ! -L "$q" ]; then continue; fi
        if [ "$(id -u)" = 0 ]; then chown -h root:root "$q"; fi
        # chmod follows a link: a stray that is a link keeps its target's mode.
        if [ ! -L "$q" ]; then chmod 0600 "$q" 2>/dev/null || true; fi
        say "FAIL: $q waits for the operator: merge what is wanted into $d/<name>, then remove it"
        fail=1
      done
      if [ "$fail" != 0 ]; then exit 1; fi
      # Only a run that passed writes the marker (round 2c, DS8). [ -e ] is
      # false for a dangling link, so remove whatever is there and never
      # write through it (DS10).
      if [ "$imported" = 0 ]; then
        rm -f "$d/.imported"
        : > "$d/.imported"
        chmod 0600 "$d/.imported"
      fi
    )
  '';

  # The units that read an env file from unitEnvDir start only after a
  # first import has passed (round 2b, DS2; round 2c, DS8): an assertion
  # failure is loud (the start job fails), and the deploy's restart of them
  # fails too. corpusd and checkpointd also need corpus-dsn.env itself
  # (unitEnvDsn). The proxy reads only the optional deploy.env, so it does
  # not wait on a corpus DSN.
  systemd.services.go-choir-proxy.unitConfig.AssertPathExists = unitEnvImported;
  systemd.services.go-choir-corpusd.unitConfig.AssertPathExists = [ unitEnvImported unitEnvDsn ];
  systemd.services.go-choir-checkpointd.unitConfig.AssertPathExists = [ unitEnvImported unitEnvDsn ];

  # Go module proxy for engineering capsules
  # (docs/reports/capsule-go-module-proxy-design-2026-10-14.md). Capsules
  # reach it only through their egress route goproxy.choir.internal, which
  # the guest executor sends to the tap peer on 8088; vmmanager DNATs that to
  # this loopback listener. It serves only GOPROXY and sum.golang.org proxy
  # paths, fetches misses from proxy.golang.org (following its redirect to
  # storage.googleapis.com here, on the host), and caches append-only.
  # On every start it pre-warms and re-verifies the deployed go.sum in the
  # background, so a deploy leaves the platform's own module set cached.
  # ExecStart names the Nix store path (no service pointer), so the default
  # restartIfChanged runs the new binary at switch time; the pointer reason
  # behind 2db4bc02's restartIfChanged = false does not apply. Holds no
  # secrets; runs as a dynamic user.
  systemd.services.go-choir-goproxy = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a): after 10 starts in 10 minutes the unit stays failed until a deploy or an operator (systemctl reset-failed) starts it again.
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir Go module proxy for engineering capsules";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = commonServiceHardening // {
      ExecStart = lib.concatStringsSep " " [
        "${goChoirPackages.goproxy}/bin/goproxy"
        "-listen 127.0.0.1:8088"
        "-cache-dir /var/lib/go-choir-goproxy"
        "-max-bytes 8589934592"
        "-warm-go-sum ${goChoirPackages.goproxy}/share/go-choir/go.sum"
      ];
      Restart = "on-failure";
      RestartSec = 3;
      # SIGTERM drains in-flight module downloads for up to 45 s.
      TimeoutStopSec = "60s";
      DynamicUser = true;
      # A top-level state dir the dynamic user owns: a nested one under root's
      # 0750 /var/lib/go-choir is not traversable by the dynamic user
      # (docs/problems/goproxy-unit-cannot-create-cache-2026-10-14.md).
      StateDirectory = "go-choir-goproxy";
      StateDirectoryMode = "0750";
      RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
      MemoryMax = "1G";
      # Under host memory pressure the kernel kills this cache before a guest.
      OOMScoreAdjust = 500;
      # Guest-reachable through the tap; reads nothing of the host's.
      InaccessiblePaths = hiddenSecretsExcept [ ];
    };
  };

  # Package mirror for engineering capsules (station FP10,
  # docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md): npm,
  # PyPI, crates and git, following the Go module proxy above. Capsules on a
  # granted computer (CHOIR_CAPSULE_MIRROR_* in go-choir-vmctl) reach it only
  # through their egress route mirror.choir.internal, which the guest
  # executor sends to the tap peer on 8089; vmmanager DNATs that to this
  # loopback listener. GET and HEAD only; artifacts enter the cache only
  # after they match the registry's published hash; git is served over the
  # dumb protocol from bare copies this unit's git keeps. Every ecosystem is
  # OFF until named in -ecosystems (for example "-ecosystems npm,pypi,git").
  # Holds no secrets; runs as a dynamic user.
  systemd.services.go-choir-pkgmirror = {
    # No crash loops (docs/restart-subsystem-design-2026-10-13.md, L1a).
    startLimitIntervalSec = 600;
    startLimitBurst = 10;
    description = "go-choir package mirror for engineering capsules";
    wantedBy = [ "multi-user.target" ];
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    path = [ pkgs.git ];
    serviceConfig = commonServiceHardening // {
      ExecStart = lib.concatStringsSep " " [
        "${goChoirPackages.pkgmirror}/bin/pkgmirror"
        "-listen 127.0.0.1:8089"
        "-cache-dir /var/lib/go-choir-pkgmirror"
        "-public-base http://mirror.choir.internal"
        "-max-bytes 17179869184"
        "-git-max-bytes 17179869184"
        "-ecosystems ''"
      ];
      Restart = "on-failure";
      RestartSec = 3;
      # SIGTERM drains in-flight downloads for up to 45 s.
      TimeoutStopSec = "60s";
      DynamicUser = true;
      # Top-level, as for goproxy
      # (docs/problems/goproxy-unit-cannot-create-cache-2026-10-14.md).
      StateDirectory = "go-choir-pkgmirror";
      StateDirectoryMode = "0750";
      RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
      MemoryMax = "1G";
      # Under host memory pressure the kernel kills this cache before a guest.
      OOMScoreAdjust = 500;
      # Guest-reachable through the tap; reads nothing of the host's.
      InaccessiblePaths = hiddenSecretsExcept [ ];
    };
  };

  # The host system closure owns the exact immutable guest image. Point vmctl
  # at that store output atomically; never let current host binaries boot an
  # unrelated mutable directory. Preserve the one pre-managed image directory
  # as an explicit rollback ref during the first cutover.
  system.activationScripts.go-choir-guest-image = lib.optionalString (guestImage != null) ''
    set -euo pipefail
    state=/var/lib/go-choir
    target="$state/guest"
    legacy="$state/guest-pre-managed-rollback"
    conflict="$state/guest-cutover-conflict-recovery"
    next="$state/.guest-next"
    src="${guestImage}"
    for artifact in vmlinux rootfs.ext4 initrd storedisk.erofs kernel-params build.json; do
      if [ ! -f "$src/$artifact" ]; then
        echo "go-choir guest image missing $artifact in $src" >&2
        exit 1
      fi
    done
    mkdir -p "$state"
    moved_to=
    restore_moved_guest() {
      local current_target=
      if [ -z "$moved_to" ]; then
        return 0
      fi
      if [ -L "$target" ]; then
        if ! current_target=$(readlink "$target"); then
          return 1
        fi
        if [ "$current_target" != "$src" ]; then
          echo "go-choir guest image cutover refuses to replace unexpected target $target -> $current_target" >&2
          return 1
        fi
        if ! rm -f "$target"; then
          return 1
        fi
      fi
      if [ -e "$target" ] || [ -L "$target" ]; then
        echo "go-choir guest image cutover refuses to restore over existing target $target" >&2
        return 1
      fi
      if ! mv -T "$moved_to" "$target"; then
        echo "go-choir guest image cutover could not restore $moved_to to $target" >&2
        return 1
      fi
      moved_to=
    }
    saved_hup=$(trap -p HUP)
    saved_int=$(trap -p INT)
    saved_term=$(trap -p TERM)
    restore_saved_trap() {
      local signal="$1"
      local saved="$2"
      if [ -n "$saved" ]; then
        eval "$saved"
      else
        trap - "$signal"
      fi
    }
    restore_cutover_traps() {
      restore_saved_trap HUP "$saved_hup"
      restore_saved_trap INT "$saved_int"
      restore_saved_trap TERM "$saved_term"
    }
    on_cutover_signal() {
      local signal="$1"
      restore_moved_guest || true
      case "$signal" in
        HUP) exit 129 ;;
        INT) exit 130 ;;
        TERM) exit 143 ;;
      esac
    }
    move_to=
    preserved_conflict=false
    if [ -e "$target" ] && [ ! -L "$target" ]; then
      if [ -e "$legacy" ] || [ -L "$legacy" ]; then
        if [ -e "$conflict" ] || [ -L "$conflict" ]; then
          echo "go-choir guest image cutover refuses a second ambiguous guest tree; recovery already exists at $conflict" >&2
          exit 1
        fi
        move_to="$conflict"
        preserved_conflict=true
      else
        move_to="$legacy"
      fi
    fi
    rm -f "$next"
    ln -s "$src" "$next"
    trap 'on_cutover_signal HUP' HUP
    trap 'on_cutover_signal INT' INT
    trap 'on_cutover_signal TERM' TERM
    if [ -n "$move_to" ]; then
      moved_to="$move_to"
      if mv -T "$target" "$moved_to"; then
        :
      else
        status=$?
        moved_to=
        restore_cutover_traps
        rm -f "$next" || true
        exit "$status"
      fi
    fi
    if mv -Tf "$next" "$target"; then
      :
    else
      status=$?
      restore_moved_guest || status=1
      restore_cutover_traps
      rm -f "$next" || true
      exit "$status"
    fi
    moved_to=
    restore_cutover_traps
    if [ "$preserved_conflict" = true ]; then
      echo "go-choir guest image cutover preserved ambiguous active tree at $conflict" || true
    fi
    echo "go-choir guest image pointer updated to $src" || true
  '';

  # Host autoputer service deleted in PR 5 of store-consolidation mission.
  # All runtime work happens in VMs via vmctl. Served proxy routes require an
  # immutable D-ROUTE slot and fail before VM lookup when route authority is
  # unavailable; PROXY_AUTOPUTER_URL is not a production fallback.
  #
  # The autoputer binary is still built and packaged (goChoirPackages.autoputer)
  # because it runs inside Firecracker VMs (nix/autoputer-vm.nix). vmctl serves
  # it to VMs from /var/lib/go-choir/services/autoputer. Since the systemd
  # service is gone, node-b-sync-service-pointers can no longer discover the
  # package via systemctl show. This activation script installs the pointer
  # directly from the Nix closure on every NixOS switch.
  system.activationScripts.go-choir-autoputer-package-pointer = ''
    mkdir -p /var/lib/go-choir/services
    src="${goChoirPackages.autoputer}"
    if [ -x "$src/bin/autoputer" ]; then
      rm -rf /var/lib/go-choir/services/.autoputer-next
      mkdir -p /var/lib/go-choir/services/.autoputer-next
      cp -a "$src/." /var/lib/go-choir/services/.autoputer-next/
      rm -rf /var/lib/go-choir/services/autoputer-previous
      if [ -e /var/lib/go-choir/services/autoputer ]; then
        mv /var/lib/go-choir/services/autoputer /var/lib/go-choir/services/autoputer-previous
      fi
      mv /var/lib/go-choir/services/.autoputer-next /var/lib/go-choir/services/autoputer
      echo "go-choir autoputer package pointer updated from NixOS closure"
    fi
  '';

  # Workspace directory (for CI git pull deploys) and runtime paths.
  # Auth persistence and signing material must live in writable runtime
  # locations, not in the repo checkout or the Nix store.  Secrets are never
  # committed to git or embedded in the Nix store — the signing key is
  # generated on the host at first deploy.  The autoputer and proxy are
  # stateless for dev and need no writable directories.
  #
  # Firecracker guest image directory (VAL-VM-010): the repo-built guest
  # kernel and rootfs are deployed here. Provider credentials are never
  # placed in this directory or in the guest image itself (VAL-VM-011).
  systemd.tmpfiles.rules = [
    # KSM off (SEC-09; review N4): merged pages across guests are a
    # cross-tenant side channel. 2 stops ksmd and unmerges what it merged.
    # Firecracker does not madvise guest memory MERGEABLE, so nothing was
    # merged; this makes the posture explicit. Rollback: drop the line.
    "w /sys/kernel/mm/ksm/run - - - - 2"
    # /dev/kvm 0660 root:kvm, not systemd's 0666 (NET-07): direct VMMs run
    # as root and jailed VMMs get their own device node from the jailer, so
    # no other local user needs KVM. Local Nix builds with the kvm feature
    # go to node-a. Rollback: drop the line and the udev rule below.
    "z /dev/kvm 0660 root kvm -"
    "d /opt/go-choir 0755 root root -"
    "d /var/www 0755 root root -"
    "d /var/www/go-choir 0755 root root -"
    "d /var/lib/go-choir 0750 root root -"
    "d /var/lib/go-choir/services 0755 root root -"
    "d ${autoputerRuntimeDir} 0750 root root -"
    "d /var/lib/go-choir/auth 0750 root root -"
    "d /var/lib/go-choir/auth-signing 0750 root root -"
    "d ${mailDir} 0700 root root -"
    "d ${mailDir}/raw 0700 root root -"
    "d ${mailDir}/attachments 0700 root root -"
    "d ${mailDir}/attachments/quarantine 0700 root root -"
    "z ${mailDir} 0700 root root -"
    "z ${mailDir}/mail.db 0600 root root -"
    "z ${mailDir}/raw 0700 root root -"
    "z ${mailDir}/attachments 0700 root root -"
    "z ${mailDir}/attachments/quarantine 0700 root root -"
    # World-readable files that only root (or systemd, as EnvironmentFile)
    # reads; private today only because /var/lib/go-choir is 0750, which
    # blocks moving any service to its own user.
    "z /var/lib/go-choir/auth/auth.db 0600 root root -"
    "z /var/lib/go-choir/auth/auth.db-wal 0600 root root -"
    "z /var/lib/go-choir/auth/auth.db-shm 0600 root root -"
    # The env directory (unitEnvDir), before any unit starts at boot; the
    # activation snippet go-choir-unit-env also makes it before a switch
    # restarts units.
    "d ${unitEnvDir} 0700 root root -"
  ] ++ map (name: "z ${unitEnvDir}/${name} 0600 root root -") unitEnvNames ++ [
    "z ${unitEnvImported} 0600 root root -"
    "d /var/lib/go-choir/vm-state 0750 root root -"
    "d ${platformDoltDir} 0750 root root -"
    "d ${platformDoltDBDir} 0750 root root -"
    "d ${platformArtifactsDir} 0750 root root -"
    "d ${platformArtifactsDir}/computer-inputs 0750 root root -"
    "d ${platformArtifactsDir}/computer-inputs/blobs 0750 root root -"
    "d ${platformArtifactsDir}/sha256 0750 root root -"
    # SearXNG secret key file directory (writable by the searx service)
    "d /var/lib/searx 0750 searx searx -"
  ];

  # Nix settings
  nix.settings = {
    experimental-features = [ "nix-command" "flakes" ];
    # Hard-link each new store path's files as it is added instead of
    # waiting for the weekly nix.optimise below (docs/problems/node-b-nix-
    # store-and-host-disk-policy-2026-10-22.md item 1). Costs a hash of each
    # new file at build or substitution time.
    auto-optimise-store = true;
    # nix GCs inside a build when free space falls under min-free, up to
    # max-free, and that GC deletes the build's own not-yet-rooted inputs
    # (2026-09-17: min-free above the deploy floor looped a deploy). So the
    # deploy floor in scripts/node-b-deploy-disk-preflight sits above this
    # band by a deploy's footprint, a guest store compaction and the paper's
    # regrowth (scripts/test_node_b_disk_policy.py checks the inequality),
    # and the preflight also reads the live min-free from /etc/nix/nix.conf
    # and never takes a floor under it plus 10 GiB. Rollback: 80 GiB /
    # 100 GiB (85899345920 / 107374182400); the preflight's guard raises its
    # floor back to 90 GiB on its own at the next deploy.
    min-free = 25769803776;
    max-free = 42949672960;
    # CI builds run on the production host beside guests: two derivations at
    # a time, six cores each, inside the bounded nix-daemon slice below.
    max-jobs = 2;
    cores = 6;
    # Remote builders fetch their inputs from cache.nixos.org themselves
    # instead of receiving them over SSH from this host.
    builders-use-substitutes = true;
  };

  # Builds go to node-a (12 cores, 31 GiB, no guests; 0.5 ms away) so CI
  # compiles stop competing with guests for this host's memory and CPU
  # (docs/problems/ci-deploy-latency-per-service-go-builds-2026-10-09.md,
  # proposal item 3). The key is node-b-local (/root/.ssh, not in the repo);
  # node-a restricts it to `nix-daemon --stdio`. If node-a is unreachable,
  # Nix builds locally under the budget above.
  nix.distributedBuilds = true;
  nix.buildMachines = [{
    hostName = "node-a-builder";
    protocol = "ssh-ng";
    sshUser = "root";
    sshKey = "/root/.ssh/nix-builder-node-a";
    system = "x86_64-linux";
    maxJobs = 8;
    speedFactor = 10;
    supportedFeatures = [ "benchmark" "big-parallel" "kvm" "nixos-test" ];
  }];
  programs.ssh.knownHosts."node-a-builder" = {
    hostNames = [ "node-a-builder" "51.81.93.94" ];
    publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKYpptwKiBVbvuEzT2fE/NfaGDO5CfDBOveEy6UlC/XJ";
  };
  programs.ssh.extraConfig = ''
    Host node-a-builder
      HostName 51.81.93.94
      ConnectTimeout 5
      ServerAliveInterval 15
      ServerAliveCountMax 4
  '';

  # Host memory budget (docs/problems/node-b-memory-overcommitted-swap-
  # 2026-10-09.md, owner direction: swapping is service degradation). Builds
  # are throttled at 6G, killed at 7G, never swap, and die first under OOM.
  systemd.services.nix-daemon.serviceConfig = {
    MemoryHigh = "6G";
    MemoryMax = "7G";
    MemorySwapMax = "0";
    OOMScoreAdjust = lib.mkForce 1000;
  };
  boot.kernel.sysctl."vm.swappiness" = 1;

  # KVM halt polling off (docs/reports/computer-cpu-efficiency-2026-10-14.md,
  # lever C5 there, C4 in the store-scan fix mission). After each guest HLT
  # the host spun up to 200 us (the default halt_poll_ns=200000) hoping the
  # vCPU wakes again: 0.07-0.10 core per busy computer (7-9% of the paper),
  # measured from debugfs halt_poll_success_ns + halt_poll_fail_ns. KVM's own
  # guidance is to reduce polling when vCPUs overcommit host threads, which
  # this host does. Cost: microseconds of extra wake latency per halt exit;
  # the A/B on test computers (report section 7 step 4) checks cell and call
  # latency.
  # Takes effect at the next host boot (kvm-intel loads at boot). Without a
  # reboot: `echo 0 > /sys/module/kvm/parameters/halt_poll_ns` as root;
  # whether running VMs follow the module value or keep the one they were
  # created with on this kernel is unverified, so read
  # /sys/kernel/debug/kvm/*/halt_poll_*_ns before and after.
  # Rollback: delete this line (or echo 200000 into the same file).
  boot.kernelParams = [ "kvm.halt_poll_ns=0" ];

  # Nested virtualization off (docs/problems/host-kernel-216-days-unpatched-
  # 2026-10-15.md; docs/reports/boundary-strength-ladder-2026-10-15.md rung
  # 4a). No Choir guest runs a hypervisor: Firecracker is host-only and its
  # guests never get VMX. nested=Y only widened the KVM surface: the
  # Januscape shadow-MMU bug needs guest root plus nested virtualization.
  # Takes effect when kvm_intel next loads, i.e. at the next host boot
  # (verify: cat /sys/module/kvm_intel/parameters/nested -> N).
  # Rollback: delete this line and reboot.
  boot.extraModprobeConfig = ''
    options kvm_intel nested=0
  '';

  # The udev half of the /dev/kvm mode (tmpfiles above sets the live node).
  services.udev.extraRules = ''
    KERNEL=="kvm", GROUP="kvm", MODE="0660"
  '';

  # SMT (hyperthreading): left ON by owner ruling (NET-07, DEC-06: until
  # outside users share the host; owner notes, around line 330). The E-2236 is
  # Coffee Lake; L1TF, MDS and VMScape cross-tenant leaks need SMT off for
  # full protection. Cost: 6 cores / 12 threads -> 6 threads, i.e. half the
  # vCPU-schedulable threads for computers (density analysis in the problem
  # doc above). To turn it off, uncomment and reboot:
  # boot.kernelParams = [ "nosmt=force" ];

  # Weekly whole-store optimise. With auto-optimise-store on it only catches
  # what the per-path optimise missed; kept as the backstop.
  nix.optimise = {
    automatic = true;
    dates = [ "Sun 03:30" ];
    randomizedDelaySec = "45min";
    persistent = true;
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
    choirGo # Go toolchain lane (flake.nix choirGo)
    gnumake
    gnugrep
    gnused
    dolt
    htop
    jq
    nodejs
    python3
    pkg-config
    icu
    icu.dev
    goChoirPackages.maildctl
    goChoirPackages.projectionCompact
    procps
    lsof
    strace
    ripgrep
    vim
    # Off-host backup by hand (keygen, init, a supervised run, the restore
    # drill): the units carry these on their own PATH, a root shell did not
    # (docs/problems/no-off-host-backup-2026-10-20.md section 13 item 5).
    # htpasswd stays out: it ships with the Apache web server, and keygen,
    # run once, takes it from nix-shell.
    restic
    sqlite
  ];

  # Timezone
  time.timeZone = "UTC";

  system.stateVersion = "25.11";
}
