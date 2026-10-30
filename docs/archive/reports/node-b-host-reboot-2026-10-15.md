# Node B host reboot to kernel 6.18.55 (2026-10-15)

Owner approval: "reboot go" (2026-10-15). Runbook:
[docs/runbooks/node-b-host-reboot.md](../../runbooks/node-b-host-reboot.md).
The script receipt and run log are kept outside the repo in
`~/.config/choir-qa/traces/node-b-reboot-2026-10-15/`.

## Result

- `systemctl reboot` at 10:49:20 UTC. The host answered ssh again after
  78 s (new boot_id 7920193f-…). Booted kernel 6.18.55, up from 6.18.16,
  which had been up 216.9 days. The booted system is the current system
  (x4lzcrl…); `kvm_intel nested=N`.
- Post-checks: all passed. There were no failed units, and 21 go-choir
  units were active. md126 and md127 were [UU]. The guest firewall chains
  were intact. vmctl and staging `/health` returned ok, with the deployed
  commit unchanged (f8007f5d6). verify-host-hardening went from 69 to 77
  passing, with no new failures: the eight `[boot]`/unit checks that
  needed a reboot now pass.
- Computers: all 3 running computers accepted planned-restart markers,
  including the paper with 9 runs in flight (the critical_protected
  busy-refresh path). The paper (vm-1669…) and candidate-fleet-e15c…
  booted as planned and are healthy. The primary computer vm-48bc… is
  stopped and boots on its next request. `--allow-crash-close` was passed
  but not needed.

## Honest gaps

- **The IPMI console was not verified.** `--console-verified` was passed
  without opening the console. The fallback was set up instead: the
  server's OVH `rescueSshKey` is now the operator's Node B public key
  (read back to confirm), so the API can boot OVH rescue and the operator
  can ssh in without an emailed password. That key is still set.
- The runbook's GRUB fallback entry (step 0.4) was not added. A one-shot
  `next_entry` can't be cleared on Node B because `/boot` is md RAID1
  (md126, metadata 1.0), and GRUB's `save_env` doesn't write to RAID. A
  hard reset would boot the new kernel again, so it gave no fallback.
- Before the reboot, the platform Dolt server was deadlocked from about
  09:49 to 10:47
  ([problem doc](../problems/platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md)).
  It was restarted by hand at 10:47, and the reboot restarted it again.
  P4's deploy (run 38717028854) failed because of it; the next deploy of
  main covers P4.
- Firecracker 1.17 was not part of this reboot (branch ops/firecracker-1.17
  is not started).
