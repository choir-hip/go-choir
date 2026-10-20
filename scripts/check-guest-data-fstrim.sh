#!/usr/bin/env bash
# Nix evaluation check for the guest data-volume trim timer
# (docs/problems/guest-data-image-never-trims-2026-10-20.md).
#
# Evaluates the guest NixOS config only (no build) and asserts:
#   - choir-data-fstrim.timer is wanted by timers.target, runs hourly after a
#     first run shortly after boot, and fires the service;
#   - the service is a oneshot that trims /mnt/persistent only (never "-A",
#     which would also try the read-only erofs store), and exits 0 when the
#     device offers no discard (a VMM without virtio-blk discard);
#   - no `discard` mount option was put on the data volume (continuous
#     discard costs latency; the timer is the decision).
set -euo pipefail

cd "$(dirname "$0")/.."
guest='.#nixosConfigurations.go-choir-autoputer-vm.config'

fail() { echo "FAIL: $*" >&2; exit 1; }
ev() { nix eval --raw "$guest.$1"; }
evj() { nix eval --json "$guest.$1"; }

[ "$(evj 'systemd.timers.choir-data-fstrim.wantedBy')" = '["timers.target"]' ] \
  || fail "timer is not wanted by timers.target"
[ "$(ev 'systemd.timers.choir-data-fstrim.timerConfig.OnUnitActiveSec')" = "1h" ] \
  || fail "timer is not hourly"
[ "$(ev 'systemd.timers.choir-data-fstrim.timerConfig.OnBootSec')" = "10min" ] \
  || fail "timer has no first run after boot"
[ "$(ev 'systemd.timers.choir-data-fstrim.timerConfig.Unit')" = "choir-data-fstrim.service" ] \
  || fail "timer does not fire choir-data-fstrim.service"
[ "$(ev 'systemd.services.choir-data-fstrim.serviceConfig.Type')" = "oneshot" ] \
  || fail "service is not a oneshot"

script="$(ev 'systemd.services.choir-data-fstrim.script')"
case "$script" in *"fstrim -v /mnt/persistent"*) ;; *) fail "service does not trim /mnt/persistent" ;; esac
case "$script" in *"fstrim -A"*|*"fstrim -v -A"*|*"fstrim -Av"*) fail "service trims all mounts" ;; esac
case "$script" in *"discard_max_bytes"*"exit 0"*) ;; *) fail "service does not exit 0 when no discard is offered" ;; esac

opts="$(evj 'fileSystems."/mnt/persistent".options')"
case "$opts" in *discard*) fail "data volume mounted with the discard option: $opts" ;; esac

echo "ok: choir-data-fstrim timer and service evaluate as designed"
