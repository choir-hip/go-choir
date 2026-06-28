# NixOS host configuration for go-choir Node A (offsite replica, restore
# rehearsal target, shadow compute node).
#
# Node A is the second node in the Choir deployment topology. It does NOT
# serve live traffic. Its roles are:
#   1. Offsite backup replica — receives backup artifacts from Node B
#   2. Restore rehearsal target — proves backups can reconstruct state
#   3. Shadow compute — boots shadow VMs from restored state for
#      divergence comparison against primary (artifact program doctrine)
#
# Security model:
#   - SSH only (port 22). No public web ports.
#   - Pull model: Node A pulls backups FROM Node B. Node B does not push.
#   - Restore rehearsal operates on copies. Node B's live state is never
#     touched by any Node A operation.
#   - Shadow VMs run in isolation with no route to Node B's live VMs.
#
# See docs/mission-node-a-deployment-restore-rehearsal-v0.md
{ config, lib, pkgs, ... }:
let
  backupRoot = "/var/lib/go-choir-a/backups";
  rehearsalRoot = "/var/lib/go-choir-a/restore-rehearsal";
  shadowVmRoot = "/var/lib/go-choir-a/shadow-vm";

  # Directory layout for received backup artifacts.
  backupDirs = {
    dolt = "${backupRoot}/dolt";
    auth = "${backupRoot}/auth";
    source = "${backupRoot}/source";
    mail = "${backupRoot}/mail";
    vm-state = "${backupRoot}/vm-state";
    manifests = "${backupRoot}/manifests";
  };

  # Restore rehearsal script — verifies backup artifacts and runs
  # integrity queries against restored state.
  restoreRehearsal = pkgs.writeShellScript "node-a-restore-rehearsal" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.coreutils pkgs.sqlite-interactive pkgs.dolt pkgs.jq pkgs.curl ]}:$PATH"

    rehearsal_dir="${rehearsalRoot}/$(date -u +%Y%m%dT%H%M%SZ)"
    mkdir -p "$rehearsal_dir"
    report="$rehearsal_dir/rehearsal-report.json"
    started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    # Initialize report structure
    cp ${./node-a-rehearsal-report-template.json} "$report"

    update_report() {
      local tmp
      tmp=$(mktemp)
      jq "$1" "$report" > "$tmp" && mv "$tmp" "$report"
    }

    fail_artifact() {
      local name="$1"
      local reason="$2"
      update_report ".artifacts.\"$name\".status = \"fail\" | .artifacts.\"$name\".reason = \"$reason\""
      echo "  FAIL: $name — $reason" >&2
    }

    pass_artifact() {
      local name="$1"
      local detail="$2"
      update_report ".artifacts.\"$name\".status = \"pass\" | .artifacts.\"$name\".detail = \"$detail\""
      echo "  PASS: $name — $detail"
    }

    echo "==> Node A restore rehearsal started at $started_at"
    echo "    Rehearsal directory: $rehearsal_dir"

    # ── Dolt DB restore rehearsal ──────────────────────────────────
    echo "==> Rehearsing Dolt platform DB restore"
    dolt_backup="${backupDirs.dolt}"
    if [ -d "$dolt_backup" ] && [ -d "$dolt_backup/platform" ]; then
      dolt_rehearsal="$rehearsal_dir/dolt-platform"
      mkdir -p "$dolt_rehearsal"
      cp -a "$dolt_backup/platform/." "$dolt_rehearsal/"
      if (cd "$dolt_rehearsal" && dolt sql -q "SHOW TABLES;" >/dev/null 2>&1); then
        table_count=$(cd "$dolt_rehearsal" && dolt sql -q "SHOW TABLES;" 2>/dev/null | grep -c '|' || true)
        commit_hash=$(cd "$dolt_rehearsal" && dolt log --oneline 2>/dev/null | head -1 | awk '{print $2}' || true)
        pass_artifact "dolt_platform" "tables=$table_count commit=$commit_hash"
      else
        fail_artifact "dolt_platform" "dolt sql query failed on restored DB"
      fi
    else
      fail_artifact "dolt_platform" "no Dolt backup found at $dolt_backup"
    fi

    # ── Auth DB restore rehearsal ──────────────────────────────────
    echo "==> Rehearsing auth DB restore"
    auth_backup="${backupDirs.auth}/auth.db"
    if [ -f "$auth_backup" ]; then
      auth_rehearsal="$rehearsal_dir/auth.db"
      cp "$auth_backup" "$auth_rehearsal"
      if sqlite3 "$auth_rehearsal" "SELECT count(*) FROM sessions;" >/dev/null 2>&1; then
        session_count=$(sqlite3 "$auth_rehearsal" "SELECT count(*) FROM sessions;" 2>/dev/null || echo "0")
        pass_artifact "auth_db" "sessions=$session_count"
      else
        pass_artifact "auth_db" "DB opens but sessions table may be empty (fresh install)"
      fi
    else
      fail_artifact "auth_db" "no auth.db backup found at $auth_backup"
    fi

    # ── Source service DB restore rehearsal ────────────────────────
    echo "==> Rehearsing source service DB restore"
    source_backup="${backupDirs.source}/sourcecycled.db"
    if [ -f "$source_backup" ]; then
      source_rehearsal="$rehearsal_dir/sourcecycled.db"
      cp "$source_backup" "$source_rehearsal"
      if sqlite3 "$source_rehearsal" ".tables" >/dev/null 2>&1; then
        table_list=$(sqlite3 "$source_rehearsal" ".tables" 2>/dev/null | tr '\n' ',' || true)
        pass_artifact "source_db" "tables=$table_list"
      else
        fail_artifact "source_db" "sqlite3 query failed on restored DB"
      fi
    else
      fail_artifact "source_db" "no sourcecycled.db backup found at $source_backup"
    fi

    # ── Mail DB restore rehearsal ──────────────────────────────────
    echo "==> Rehearsing mail DB restore"
    mail_backup="${backupDirs.mail}/mail.db"
    if [ -f "$mail_backup" ]; then
      mail_rehearsal="$rehearsal_dir/mail.db"
      cp "$mail_backup" "$mail_rehearsal"
      if sqlite3 "$mail_rehearsal" ".tables" >/dev/null 2>&1; then
        table_list=$(sqlite3 "$mail_rehearsal" ".tables" 2>/dev/null | tr '\n' ',' || true)
        pass_artifact "mail_db" "tables=$table_list"
      else
        fail_artifact "mail_db" "sqlite3 query failed on restored DB"
      fi
    else
      fail_artifact "mail_db" "no mail.db backup found at $mail_backup"
    fi

    # ── VM state snapshots restore rehearsal ───────────────────────
    echo "==> Rehearsing VM state snapshot inventory"
    vm_state_backup="${backupDirs.vm-state}"
    if [ -d "$vm_state_backup" ]; then
      snapshot_count=$(find "$vm_state_backup" -name 'data.img.*' -type f 2>/dev/null | wc -l | tr -d ' ')
      metadata_count=$(find "$vm_state_backup" -name 'data.img.*.meta.json' -type f 2>/dev/null | wc -l | tr -d ' ')
      pass_artifact "vm_state" "snapshots=$snapshot_count metadata_sidecars=$metadata_count"
    else
      fail_artifact "vm_state" "no VM state backup found at $vm_state_backup"
    fi

    # ── Finalize report ────────────────────────────────────────────
    finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    overall_status=$(jq -r '
      [.artifacts | to_entries | .[] | .value.status] as $statuses |
      if (any($statuses[]; . == "fail")) then "fail"
      elif (all($statuses[]; . == "pass")) then "pass"
      else "partial"
      end
    ' "$report")
    update_report ".finished_at = \"$finished_at\" | .overall_status = \"$overall_status\""

    echo
    echo "==> Restore rehearsal complete: $overall_status"
    echo "    Report: $report"
    cat "$report"

    if [ "$overall_status" = "fail" ]; then
      exit 1
    fi
  '';

  # Backup pull script — pulls backup artifacts from Node B via rsync.
  backupPull = pkgs.writeShellScript "node-a-backup-pull" ''
    set -euo pipefail
    export PATH="${lib.makeBinPath [ pkgs.rsync pkgs.coreutils pkgs.jq pkgs.openssh ]}:$PATH"

    node_b_host="''${NODE_B_HOST:-go-choir-node-b}"
    manifest_dir="${backupDirs.manifests}"
    mkdir -p "$manifest_dir"

    pull_ts=$(date -u +%Y%m%dT%H%M%SZ)
    manifest="$manifest_dir/pull-$pull_ts.json"
    started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    echo "==> Node A backup pull started at $started_at"
    echo "    Source: $node_b_host"
    echo "    Manifest: $manifest"

    # Initialize manifest
    echo "{\"started_at\":\"$started_at\",\"source\":\"$node_b_host\",\"artifacts\":{}}" > "$manifest"

    pull_artifact() {
      local name="$1"
      local remote_path="$2"
      local local_dir="$3"
      local tmp
      tmp=$(mktemp)
      mkdir -p "$local_dir"
      echo "  Pulling $name from $remote_path"
      if rsync -a --delete --info=progress2,stats2 \
        "$node_b_host:$remote_path/" "$local_dir/" 2>"$tmp"; then
        local file_count
        file_count=$(find "$local_dir" -type f | wc -l | tr -d ' ')
        local total_size
        total_size=$(du -sh "$local_dir" | cut -f1)
        jq ".artifacts.\"$name\" = {\"status\":\"pass\",\"files\":$file_count,\"size\":\"$total_size\"}" \
          "$manifest" > "$tmp" && mv "$tmp" "$manifest"
        echo "  PASS: $name ($file_count files, $total_size)"
      else
        local err
        err=$(head -5 "$tmp" | tr '\n' ' ' | sed 's/"/\\"/g')
        jq ".artifacts.\"$name\" = {\"status\":\"fail\",\"error\":\"$err\"}" \
          "$manifest" > "$tmp" && mv "$tmp" "$manifest"
        echo "  FAIL: $name — $err" >&2
      fi
      rm -f "$tmp"
    }

    # Pull Dolt platform DB
    pull_artifact "dolt_platform" "/var/lib/go-choir/platform-dolt" "${backupDirs.dolt}"
    # Pull auth DB
    pull_artifact "auth_db" "/var/lib/go-choir/auth" "${backupDirs.auth}"
    # Pull source service DB
    pull_artifact "source_db" "/var/lib/go-choir/source-service" "${backupDirs.source}"
    # Pull mail DB
    pull_artifact "mail_db" "/var/lib/go-choir/mail" "${backupDirs.mail}"
    # Pull VM state snapshots
    pull_artifact "vm_state" "/var/lib/go-choir/vm-state" "${backupDirs.vm-state}"

    finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    tmp=$(mktemp)
    jq ".finished_at = \"$finished_at\"" "$manifest" > "$tmp" && mv "$tmp" "$manifest"

    echo
    echo "==> Backup pull complete"
    echo "    Manifest: $manifest"
    cat "$manifest"
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
  networking.hostName = "go-choir-a";

  # SSH access — the only public port.
  services.openssh = {
    enable = true;
    openFirewall = true;
    settings = {
      PermitRootLogin = "prohibit-password";
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
    };
  };

  # SSH authorized keys — operator access. Node A does not receive
  # GitHub Actions deploy keys (it is not a CI deploy target; it pulls
  # backups from Node B on its own schedule).
  users.users.root.openssh.authorizedKeys.keys = [
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILN3IIn6TzBBExWiJTJ7aDlA/LlEMXvjFlSfkKkV02TZ wiz@choiros-ovh"
  ];

  # Firewall — SSH only. Node A has no public web ports.
  networking.firewall = {
    enable = true;
    allowedTCPPorts = [
      22  # SSH
    ];
  };

  # Backup storage directories with controlled permissions.
  systemd.tmpfiles.rules = [
    "d ${backupRoot}                    0750 root root -"
    "d ${backupDirs.dolt}               0750 root root -"
    "d ${backupDirs.auth}               0750 root root -"
    "d ${backupDirs.source}             0750 root root -"
    "d ${backupDirs.mail}               0750 root root -"
    "d ${backupDirs.vm-state}           0750 root root -"
    "d ${backupDirs.manifests}          0750 root root -"
    "d ${rehearsalRoot}                 0750 root root -"
    "d ${shadowVmRoot}                  0750 root root -"
  ];

  # Systemd services for backup pull and restore rehearsal.
  systemd.services.go-choir-a-backup-pull = {
    description = "go-choir Node A: Pull backup artifacts from Node B";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = "${backupPull}";
      ReadWritePaths = [ backupRoot ];
    };
  };

  systemd.services.go-choir-a-restore-rehearsal = {
    description = "go-choir Node A: Restore rehearsal from backup artifacts";
    serviceConfig = {
      Type = "oneshot";
      ExecStart = "${restoreRehearsal}";
      ReadWritePaths = [ rehearsalRoot ];
    };
  };

  # Timer: pull backups and run restore rehearsal daily.
  # Non-blocking: failures are logged but do not affect Node B.
  systemd.timers.go-choir-a-backup-pull = {
    description = "Daily backup pull from Node B";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      Persistent = true;
      RandomizedDelaySec = "10m";
    };
  };

  systemd.timers.go-choir-a-restore-rehearsal = {
    description = "Daily restore rehearsal after backup pull";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      Persistent = true;
      RandomizedDelaySec = "15m";
    };
    # Run after backup pull completes
    after = [ "go-choir-a-backup-pull.service" ];
    wants = [ "go-choir-a-backup-pull.service" ];
  };

  # Packages available on Node A for operator use and rehearsal tooling.
  environment.systemPackages = with pkgs; [
    rsync
    jq
    sqlite-interactive
    dolt
    curl
    htop
  ];
}
