# Mission M3.3 - VM Bootstrap Recovery Race Ledger

## 2026-06-15T10:34:48Z - Create Pre-M3 Bootstrap Recovery Paramission

Claim/scope: the owner-visible `BOOTSTRAP FAILED (502)` / reload-success
behavior is now its own pre-M3 substrate bug. It should not be hidden inside M3
proper, because M3 needs vmctl refresh/restart as trustworthy lifecycle proof.

Move: created `docs/mission-vm-bootstrap-recovery-race-m3.3-v0.md` as a narrow
Parallax paramission. The paradoc records the current evidence: owner bootstrap
requests canceled around `2026-06-15T10:17Z`, vmctl refreshed the owner VM active
seconds later, the owner VM later reported `ready` with low persistent-disk use,
and a separate stale Universal Wire/sourcecycled route at `10.200.17.2:8085`
continues to produce 502 health noise.

Expected Delta V: 0 for repair, -1 for mission ambiguity. Actual Delta V: -1
against mission ambiguity. The implementation pass now has a focused source
program instead of resuming M3 while the boot/recovery substrate is noisy.

Receipts:

- Owner VM current health after investigation: `status=ready`,
  `sandbox_id=vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`, deployed commit
  `0a5fb602151c8373086c4a2774e1236faa53831b`, persistent disk about `8.9%`
  used.
- Public health after investigation: `bootstrap.total` 52 requests, 40
  `http_200`, 3 `http_502`, 9 `resolve_error`, max duration about 15 seconds.
- Stale platform/source route found:
  `vm-universal-wire-platform`, state `booting`, route
  `http://10.200.17.2:8085`; `sourcecycled` retries against it about every 32
  seconds with 502.

Open edge: implement the durable recovery/status repair and deployed first-load
proof before M3 lifecycle settlement.
