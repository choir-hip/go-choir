# Fresh realization of an existing computer has no privacy key; host waits 30 minutes blind

Date: 2026-10-08. Mutation class of any fix: red (privacy custody, VM
realization). Found by the recovery admission staging acceptance run.

## Evidence

Disposable `computer-f177676f…` (user `6c13cf31…`), driver log
[`evidence/recovery-admission-staging-demo-2026-10-08.log`](../evidence/recovery-admission-staging-demo-2026-10-08.log):

- 22:50:16Z vmctl ownership removed (chain retained) → concurrent resolves
  refused `projection_base_missing` (503, Retry-After 60, one deduplicated
  repair job); after a vmctl restart (22:50:33Z) still refused, no boot.
- 22:51:00Z repair job `succeeded` (W=2). Admission reopened on the changed
  watermark and vmctl started the realization `vm-3588cb08…`.
- Guest console: `projection recovery resume … (local=2 W=2 H=2 tail=0)`
  then `configure guest-owned private artifact cipher: privacy keyring: load
  guest key: open /mnt/persistent/choir-credentials/privacy-key: no such file
  or directory`; the runtime restarts in a loop.
- 23:21:03Z vmctl: `guest did not become healthy … within 30m0s`, VM killed,
  ownership `failed`.

## Problem

1. **Key gap (design).** A computer's guest-owned privacy key lives on its
   realization's persistent volume. The designed recovery path
   (`cold-recover`, `internal/vmctl/trusted_guest_copier.go`) copies that one
   file from the quarantined *previous* data image. A fresh realization with
   no previous image — lost or removed volume — has no path: custodian escrow
   is authorized for the isolated replay worker only, and human key reveal
   requires two approvals. The computer cannot come up, even though its tape
   and a verified projection base exist.
2. **Admission admits an unsatisfiable install (O8).** Recovery admission
   reopened and started the realization without checking that the guest's
   key requirement is satisfiable.
3. **Blind wait (O9/O20).** Guest fatal-startup errors outside the recovery
   planner are not reported as typed boot refusals, so the host waits the
   full 30-minute health timeout — the blind wait the recovery policy
   removed for over-cap tails.

4. **Retry of a refused fresh start loses its freshness (found 2026-10-08
   while writing the slice-1 tests).** A fresh realization refused before
   boot keeps its ownership record (state `failed`). The next resolve
   starts it through the *retained* path, which calls admission with
   `emptyStore=false`; once the durable recovery condition clears (e.g. the
   repair job publishes a base), no fresh-realization check runs and the VM
   boots with an empty data image. This is exactly the staging sequence
   above: refused → repair succeeded → "admission reopened" → keyless boot.
   The unit test `TestRecoveryAdmissionFreshInstallRefusesOvercapBeforeBoot`
   asserted this boot as correct.

## Decision (owner, 2026-10-08) — formerly open

Whether a fresh realization of an existing computer may receive its privacy
key from custodian escrow automatically (and under what audit), or whether
this state is intentionally operator-only (cold-recover / two-approval
reveal). Until decided, items 2 and 3 are the safe fix: refuse the start
with a typed `privacy_key_unavailable` condition before boot, and report any
fatal guest startup error as a typed refusal promptly.

**Decided.** Owner ratified operational invariant O21 ("a realization holds
no unique state", [register](../operational-invariants-register-2026-10-08.md),
[inventory](../state-homes-inventory-2026-10-08.md)) and its consequence for
this key: the escrowed key is delivered to the same computer's new
realization automatically — bound to ComputerID and realization epoch,
consume-once, recorded in key-escrow transparency before unwrap, delivered
on the root-only credential disk like the credential envelope. No new party
gains access (the custodian already holds and may unwrap this key for
replay). Human reveal keeps two-approval. The debugfs copier is deleted
once delivery is proven. Escrow upload must become guaranteed (not lazy,
best-effort) before a computer's key is its only copy.

Sequencing: items 2 and 3 land first as safe fixes; escrow delivery lands
as its own red slice proved on a lose-the-disk disposable.
