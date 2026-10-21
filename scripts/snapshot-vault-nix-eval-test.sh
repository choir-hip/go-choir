#!/usr/bin/env bash
# nix/snapshot-vault.nix under the real NixOS module system: nixpkgs from
# flake.lock (offline; skipped when nix or the locked nixpkgs is absent), a
# minimal configuration, evaluation only (nothing is built). It does not
# evaluate nix/node-b.nix, which needs the repository's packages; the module
# is deliberately not imported there until FP0 wires it.
#
# Failure modes (FP9 goal file, "Host module and script"):
#  N1 The module changes anything while disabled: never. No unit, and vmctl's
#     environment is untouched.
#  N2 Enabled, the vault unit is not a oneshot that stays up, or is restarted
#     by a switch (which would unmount a vault VMMs still map): it must set
#     restartIfChanged and stopIfChanged to false.
#  N3 vmctl requires the vault (a vault failure would then stop vmctl and every
#     computer): it must only want it and start after it.
#  N4 vmctl's own environment is replaced instead of extended.
#  N5 The unit's stop fails the unit at shutdown, when VMMs still map the
#     vault: the stop is prefixed "-".
#  N6 A vault too small for the snapshot budget evaluates without complaint.
#  N7 The unit's tools are not on its PATH.
#
# Run: bash scripts/snapshot-vault-nix-eval-test.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"
cd "$REPO" || exit 1

if ! command -v nix >/dev/null 2>&1; then echo "skip: nix is not installed"; exit 0; fi
if ! command -v python3 >/dev/null 2>&1; then echo "skip: python3 is not installed"; exit 0; fi
PKGS="$(nix eval --offline --impure --raw --expr 'let l = builtins.fromJSON (builtins.readFile ./flake.lock); in (builtins.fetchTree l.nodes.nixpkgs.locked).outPath' 2>/dev/null)" || {
  echo "skip: the locked nixpkgs is not in the store (offline)"; exit 0; }

eval_module() { # extra module text for services.choirSnapshotVault and the vmctl stub
  nix eval --offline --impure --json --expr '
    let
      eval = import "'"$PKGS"'/nixos/lib/eval-config.nix" {
        system = "x86_64-linux";
        modules = [
          '"$REPO"'/nix/snapshot-vault.nix
          ({ ... }: {
            '"$1"'
            systemd.services.go-choir-vmctl.serviceConfig.Environment = [ "VMCTL_PORT=8083" ];
            fileSystems."/" = { device = "x"; fsType = "ext4"; };
            boot.loader.grub.enable = false;
            system.stateVersion = "25.05";
          })
        ];
      };
      c = eval.config;
      vault = c.systemd.services.go-choir-snapshot-vault or null;
    in {
      hasVault = vault != null;
      vmctlEnv = c.systemd.services.go-choir-vmctl.serviceConfig.Environment;
      vmctlAfter = c.systemd.services.go-choir-vmctl.after or [];
      vmctlWants = c.systemd.services.go-choir-vmctl.wants or [];
      vmctlRequires = c.systemd.services.go-choir-vmctl.requires or [];
      failedAssertions = map (a: a.message) (builtins.filter (a: !a.assertion) c.assertions);
      vault = if vault == null then null else {
        type = vault.serviceConfig.Type;
        remain = vault.serviceConfig.RemainAfterExit;
        start = vault.serviceConfig.ExecStart;
        stop = vault.serviceConfig.ExecStop;
        restartIfChanged = vault.restartIfChanged;
        stopIfChanged = vault.stopIfChanged;
        env = vault.environment;
        before = vault.before;
        path = map (p: p.pname or p.name) vault.path;
      };
    }' 2>&1
}

fails=0
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi; }
py() { python3 -I -c "import json,sys; d=json.load(sys.stdin); $1" ; }

OFF="$(eval_module '')" || { echo "FAIL the disabled module did not evaluate:"; echo "$OFF" | tail -5; exit 1; }
check "N1 disabled: no vault unit" "$(printf '%s' "$OFF" | py 'sys.exit(0 if d["hasVault"] is False else 1)'; echo $?)"
check "N1 disabled: vmctl's environment is untouched" "$(printf '%s' "$OFF" | py 'sys.exit(0 if d["vmctlEnv"]==["VMCTL_PORT=8083"] and not d["vmctlWants"] else 1)'; echo $?)"

ON="$(eval_module 'services.choirSnapshotVault.enable = true; services.choirSnapshotVault.only = [ "computer-a" "vm-b" ];')" || { echo "FAIL the enabled module did not evaluate:"; echo "$ON" | tail -8; exit 1; }
check "N2 a oneshot that stays up" "$(printf '%s' "$ON" | py 'v=d["vault"]; sys.exit(0 if v["type"]=="oneshot" and v["remain"] is True else 1)'; echo $?)"
check "N2 a switch neither restarts nor stops it" "$(printf '%s' "$ON" | py 'v=d["vault"]; sys.exit(0 if v["restartIfChanged"] is False and v["stopIfChanged"] is False else 1)'; echo $?)"
check "N2 up on start, down on stop" "$(printf '%s' "$ON" | py 'v=d["vault"]; sys.exit(0 if v["start"].endswith("snapshot-vault.sh up") and v["stop"].endswith("snapshot-vault.sh down") else 1)'; echo $?)"
check "N5 the stop is ignored on failure" "$(printf '%s' "$ON" | py 'sys.exit(0 if d["vault"]["stop"].startswith("-") else 1)'; echo $?)"
check "N3 vmctl wants the vault and starts after it" "$(printf '%s' "$ON" | py 'sys.exit(0 if "go-choir-snapshot-vault.service" in d["vmctlAfter"] and "go-choir-snapshot-vault.service" in d["vmctlWants"] else 1)'; echo $?)"
check "N3 vmctl does not require the vault" "$(printf '%s' "$ON" | py 'sys.exit(0 if "go-choir-snapshot-vault.service" not in d["vmctlRequires"] else 1)'; echo $?)"
check "N3 the vault starts before vmctl" "$(printf '%s' "$ON" | py 'sys.exit(0 if "go-choir-vmctl.service" in d["vault"]["before"] else 1)'; echo $?)"
check "N4 vmctl's environment is extended" "$(printf '%s' "$ON" | py 'e=d["vmctlEnv"]; sys.exit(0 if "VMCTL_PORT=8083" in e and "VM_SNAPSHOT_VAULT_DIR=/var/lib/go-choir/snapshot-vault" in e and "VM_SNAPSHOT_VAULT_ONLY=computer-a,vm-b" in e else 1)'; echo $?)"
check "N4 the any-crypt flag is not set unless asked" "$(printf '%s' "$ON" | py 'sys.exit(0 if not any("ACCEPT_ANY_CRYPT" in x for x in d["vmctlEnv"]) else 1)'; echo $?)"
check "N7 the unit has its tools" "$(printf '%s' "$ON" | py 'p=d["vault"]["path"]; sys.exit(0 if all(any(t in x for x in p) for t in ["cryptsetup","util-linux","e2fsprogs","coreutils","gnugrep","findutils"]) else 1)'; echo $?)"
check "the unit passes the script its paths" "$(printf '%s' "$ON" | py 'e=d["vault"]["env"]; sys.exit(0 if e["VAULT_MOUNT"]=="/var/lib/go-choir/snapshot-vault" and e["VAULT_IMG"]=="/var/lib/go-choir/snapshot-vault.img" and e["VAULT_SIZE_GIB"]=="56" else 1)'; echo $?)"
check "defaults satisfy the size assertion" "$(printf '%s' "$ON" | py 'sys.exit(0 if not d["failedAssertions"] else 1)'; echo $?)"

SMALL="$(eval_module 'services.choirSnapshotVault.enable = true; services.choirSnapshotVault.sizeGiB = 50;')" || { echo "FAIL the small-vault module did not evaluate"; exit 1; }
check "N6 a vault smaller than budget plus 4 GiB fails an assertion" "$(printf '%s' "$SMALL" | py 'sys.exit(0 if any("snapshot vault" in m for m in d["failedAssertions"]) else 1)'; echo $?)"

UNWIRED="$(eval_module 'services.choirSnapshotVault.enable = true; services.choirSnapshotVault.wireVmctl = false;')" || { echo "FAIL"; exit 1; }
check "wireVmctl=false leaves vmctl alone" "$(printf '%s' "$UNWIRED" | py 'sys.exit(0 if d["vmctlEnv"]==["VMCTL_PORT=8083"] and not d["vmctlWants"] else 1)'; echo $?)"

ANY="$(eval_module 'services.choirSnapshotVault.enable = true; services.choirSnapshotVault.acceptAnyCrypt = true;')" || { echo "FAIL"; exit 1; }
check "acceptAnyCrypt sets the flag" "$(printf '%s' "$ANY" | py 'sys.exit(0 if "VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT=on" in d["vmctlEnv"] else 1)'; echo $?)"

if [ "$fails" = 0 ]; then echo "all passed"; else echo "$fails failed"; exit 1; fi
