# Overnight plan, 2026-10-15 (~02:40 → ~10:40 UTC)

Owner is asleep for about 8 hours. This is the lead's queue, in order, with
gates. Standing rules hold: problem documentation first; test computers
are minted and stopped; stop measurements at PSI some avg10 > 1.0; never
touch a running guest's block device; no host reboot without the owner.

## Live at start

Build e83735f2a, then 4bf25d82a, then 133de45fa:
- **Security:** hostauth (observe), the auth-boundary fixes, the maild
  send gate, and the self-arming fix.
- **Self-development:** capsule builds, capsule jobs and the capsule RPC
  re-dial; the Freeze contract.
- **Claude delegate:** slice 1, with the token installed and test computer
  computer-33594053… allowlisted.
- **Models:** the OpenCode key pool and vision by default.
- **Paper:** the Dolt GC fix.
- **Host kernel:** 6.18.55 installed, not booted.

The paper (vm-1669020a…) was recovered with an offline GC (34 → 9.9 GB,
head digest, count and history unchanged). It is still on e83735f2a,
because it is always busy; /tmp/paper-refresh-loop.sh on Node B
(systemd paper-gc-refresh2) retries deploy-refresh every minute.

## Queue

1. **Deploy** the restore head-race fix (1eaafea39, 59530c2f2) and the
   Claude delegate seccomp fix (6f6b60348, fea6d7c5b) after tracer
   selfdev-jobs-<0223> finishes.
2. **Claude delegate probe** on computer-33594053… after a cold boot
   (scripts/claude_delegate_probe.py). The delegate edits; engineering runs
   the tests. Bash in the delegate stays off (the bwrap and token decision is
   deferred).
3. **Go-drill tracer rerun** (--change go). Target: restore passes, which
   completes Station 0.
4. **Always-on planned refresh** (feat/always-on-planned-refresh). Land
   and deploy it, so the paper takes the GC fix. Confirm dolt_gc in its
   /health and a flat disk.
   - **Safety:** if the paper's disk is above 75% before this lands, rerun
     docs/runbooks/offline-guest-gc.md (proven tonight).
5. **Station A prerequisites,** on branches, deployed where safe:
   - S1 owner-act receipts (principal identity report);
   - P4 standing consensus mode with a decision poster outside the guest;
   - P5 host build service and layered apply, so Go changes run;
   - then the Station A drill.
6. **Cleanup:** sweep the surface audit's top-10 dead routes, and add the
   installed-but-not-booted detector (patch pipeline step 2).
7. **If capacity allows:** memory hotplug for class:test plus its probe;
   the Playwright minimal slice; the vision probe.
8. **About 08:00 UTC:** a system briefing for the owner in iCloud.

## Held for the owner

- The Node B host reboot into 6.18.55, with nested=0. The OVH rescue path
  is ready (ops-local/ovh_node_b.py), but the go-choir host configuration
  has never cold-booted.
- hostauth enforce waits for a clean day of observe logs.

## Expanded at ~03:00 (owner: "not ambitious enough; more concurrent and sequential")

Parallel branches; the lead merges and deploys in batches about hourly,
with a tracer run between batches:

| Stream | Branch |
|---|---|
| P5 host build + layered apply (Go changes run) | feat/p5-host-build-layered-apply |
| S1 owner-act receipts (observe) | feat/s1-owner-act-receipts (built) |
| P4 standing consensus + decision poster + signed ballots | feat/p4-standing-consensus |
| P6 identity, P7 release record, P8 source adoption (Station B) | feat/p6-p8-release-adopt |
| Bound guest Dolt history (paper ~6 GB/day) | fix/bound-guest-dolt-history |
| Desk-cell confinement landing + jailer step 2 | land/desk-cell-confinement, ops/jailer-step2 |
| Patch lanes: not-booted detector, Firecracker 1.17, Go 1.26.9, pdfjs-dist, weekly bump, SECURITY.md, Actions SHA pins | ops/patch-lanes |
| Dead-route sweep + CLI lean set | cleanup/surface-sweep |
| Account recovery + Store B retrieval rebases | land/account-recovery, land/store-b-retrieval |
| Visual evidence slice (Chromium screenshots cited at Freeze) | feat/visual-evidence-slice |
| S7-0 app packages first slice | feat/s7-0-app-packages |
| Memory hotplug for class:test, wired to capsule jobs | land/memory-hotplug |

The kernel reboot is moved to the morning with the owner.
