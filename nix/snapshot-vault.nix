# Snapshot vault for Node B (SDC NET-06, review F12).
#
# NOT IMPORTED anywhere yet. Off by default even when imported. FP0 wires it:
#
#   imports = [ ./snapshot-vault.nix ];            # in nix/node-b.nix
#   services.choirSnapshotVault.enable = true;
#   services.choirSnapshotVault.only = [ "<the test computer's id>" ];
#
# A hibernate snapshot is the guest's RAM. This module gives vmctl a volume
# for it whose key is read from /dev/urandom at each boot and never written
# down (dm-crypt plain mode: no header, no key slot): a sparse file, a loop
# device, a plain mapping, ext4, mounted 0700 at one path. vmctl then writes
# the snapshots of the selected computers there and nowhere else, and refuses
# (the computer cold boots) whenever the path does not probe as such a mount
# (internal/vmmanager/snapshot_vault.go). A host reboot makes a new, empty
# vault: every hibernated computer cold boots; no durable data is lost.
#
# The unit never fails the host: a vault that cannot be made leaves vmctl
# running with snapshots refused. vmctl wants the vault and starts after it; it
# does not require it. A switch does not restart or stop the vault (that would
# unmount a volume VMMs still map); a change to this unit applies at the next
# boot. See docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md for
# the proof steps and the rollback (remove the enable, redeploy, reboot).
#
# The jailer is not supported with the vault: a jailed VMM captures inside its
# jail on the main file system, so vmctl refuses a vault computer's snapshot
# under it (snapshot_vault_jailed_unsupported).
{ config, lib, pkgs, ... }:
let
  cfg = config.services.choirSnapshotVault;
  inherit (lib) mkEnableOption mkIf mkOption types;
  script = ../scripts/snapshot-vault.sh;
  bash = "${pkgs.bash}/bin/bash";
  unit = "go-choir-snapshot-vault.service";
in
{
  options.services.choirSnapshotVault = {
    enable = mkEnableOption "the snapshot vault: an ephemeral-key dm-crypt volume for hibernate snapshots";
    mountPoint = mkOption {
      type = types.str;
      default = "/var/lib/go-choir/snapshot-vault";
      description = "Where the vault is mounted; vmctl's VM_SNAPSHOT_VAULT_DIR. Must be empty before the mount.";
    };
    backingFile = mkOption {
      type = types.str;
      default = "/var/lib/go-choir/snapshot-vault.img";
      description = "The sparse backing file, recreated at every boot. Outside vm-state and outside every backup path.";
    };
    sizeGiB = mkOption {
      type = types.ints.positive;
      default = 56;
      description = "Size of the sparse file. At least the snapshot budget plus 4 GiB (the vault keeps a 2 GiB floor free).";
    };
    budgetGiB = mkOption {
      type = types.ints.positive;
      default = 48;
      description = "VM_SNAPSHOT_BUDGET_GIB as set for vmctl; used only to check sizeGiB.";
    };
    only = mkOption {
      type = types.listOf types.str;
      default = [ ];
      example = [ "computer-0123456789abcdef" ];
      description = "VM ids or computer ids whose snapshots go to the vault (VM_SNAPSHOT_VAULT_ONLY). Empty: every computer.";
    };
    acceptAnyCrypt = mkOption {
      type = types.bool;
      default = false;
      description = "Let vmctl accept a LUKS mapping as the vault (a key slot is then on disk). Weaker; off.";
    };
    wireVmctl = mkOption {
      type = types.bool;
      default = true;
      description = "Add VM_SNAPSHOT_VAULT_* to go-choir-vmctl and start vmctl after the vault.";
    };
  };

  config = mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.sizeGiB >= cfg.budgetGiB + 4;
        message = "services.choirSnapshotVault.sizeGiB (${toString cfg.sizeGiB}) must be at least the snapshot budget (${toString cfg.budgetGiB}) plus 4 GiB: the snapshot vault keeps a 2 GiB floor free.";
      }
    ];

    systemd.services.go-choir-snapshot-vault = {
      description = "go-choir snapshot vault (ephemeral-key dm-crypt volume for guest memory snapshots)";
      wantedBy = [ "multi-user.target" ];
      before = [ "go-choir-vmctl.service" ];
      after = [ "local-fs.target" ];
      unitConfig.RequiresMountsFor = builtins.dirOf cfg.backingFile;
      path = [ pkgs.cryptsetup pkgs.util-linux pkgs.e2fsprogs pkgs.coreutils pkgs.gnugrep pkgs.findutils ];
      environment = {
        VAULT_IMG = cfg.backingFile;
        VAULT_MOUNT = cfg.mountPoint;
        VAULT_NAME = "choir-snapshot-vault";
        VAULT_SIZE_GIB = toString cfg.sizeGiB;
      };
      # A switch must not unmount a vault that VMMs map.
      restartIfChanged = false;
      stopIfChanged = false;
      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
        ExecStart = "${bash} ${script} up";
        # At shutdown VMMs may still map the vault, and the stop would fail the
        # unit for nothing: the power-off ends the key anyway.
        ExecStop = "-${bash} ${script} down";
        TimeoutStartSec = "10min";
        TimeoutStopSec = "2min";
      };
    };

    systemd.services.go-choir-vmctl = mkIf cfg.wireVmctl {
      after = [ unit ];
      wants = [ unit ];
      serviceConfig.Environment =
        [ "VM_SNAPSHOT_VAULT_DIR=${cfg.mountPoint}" ]
        ++ lib.optional (cfg.only != [ ]) "VM_SNAPSHOT_VAULT_ONLY=${lib.concatStringsSep "," cfg.only}"
        ++ lib.optional cfg.acceptAnyCrypt "VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT=on";
    };
  };
}
