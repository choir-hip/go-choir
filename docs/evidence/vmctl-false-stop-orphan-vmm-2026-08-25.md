# Incident: vmctl reports "stopped" but the VMM (firecracker) remained running

Date: 2026-08-25 (observed ~03:45Z, resolved ~04:00Z)
Computer: `computer-03335285269bdba4f94377e56879f9e6`
VM: `candidate-fleet-e15cb89f25d963c220319b7b` (staging Node B)
Class: **problem documentation (no code fix in this commit — problem-documentation-first)**
Status: incident closed; the orphan VMM has been terminated; computer is cleanly stopped + host-held + disk quiesced.

## The problem

After setting the host-authoritative maintenance hold on 0333528, the authorized clean-stop path returned success but did **not** stop the VM's VMM process:

- `POST /internal/vmctl/stop` (X-Internal-Caller) returned HTTP 200 `{"status":"stopped"}`.
- The VM's firecracker process (pid `1175854`, `--id candidate-fleet-e15cb89f25d963c220319b7b`) kept running (observed `Sl`, elapsed ~10 min, until explicitly SIGTERM'd).
- The guest was pre-genesis and unhealthy: autoputer `:8085` unreachable/health-failing; run admission refused; no canonical writes. So the data disk was quiesced (no admitted runs), but the VMM process was alive and had the live `data.img` open.
- The vmctl *ownership* state (`stopped`) and the *process* state (firecracker alive) diverged.

## Verified root-cause shape

`StopVMForDesktop` (and the recover/refresh actuation paths) only actuate when the ownership is `active`/`degraded`/`booting` **or** `GetVM(vmID) != nil`. After the health-fail loop, ownership is persisted `stopped` and `GetVM` returns nil for the lingering process, so the stop call is a no-op that returns success. The VMM is an **orphan** relative to vmctl's lifecycle map — not a vmctl-owned running instance. vmctl therefore reports `stopped` while a real VMM process is alive, and only a direct process/cgroup signal (SIGTERM→SIGKILL) reaps it. This is a vmctl lifecycle truthfulness defect (state vs process divergence), not the maintenance hold working as intended (the hold correctly refuses auto-lifecycle; it does not reap an orphan process vmctl no longer tracks).

## Current verifiable state (read-only, 2026-08-25 ~04:00Z)

- Firecracker pid `1175854` is **gone** (SIGTERM from the authorized maintenance stop; verified `ps`/`pgrep` empty for `e15cb89`).
- Live `data.img`: **no process holds it** (`lsof` empty); **no loop device** on it (loop4 = `data.img.pre-upgrade-...`, loop2 = `data.img.quarantine-2-e0b4a9`, both pre-existing ro/quarantine mounts).
- Ownership: `state=stopped`, `epoch=771`, `held=true`, `stopped_by=vmctl-restart`.
- Disk firecracker `epoch` file: **790** (incremented from 789 during a vmctl boot attempt; it is a boot counter, not the platform realization epoch, which is stable at 771).
- Platform canonical head (corpusd `:8086`): `sequence=133209`, `canonical_event_head=6e7424f0c3...`, `desired/effective_event_head=a3cf16d0...`.
- Pre-quiesce reflink `data.img.stable-hold-20260825` exists (taken while the VMM was alive — **forensic only**, not a crash-consistent recovery baseline).

## Belief state / forward plan (unchanged by this defect)

- The clean-stop is now effectively complete (orphan reaped); a **post-quiesce reflink** of the now-free, quiesced live `data.img` will be taken as the recovery baseline.
- Recovery remains **B14 host-side replay-only rematerialization to platform head 133,209 under the hold** (forward-only; no rewind; no image-reuse; not escalate, since the platform chain survives at 133,209).
- Next: read-only audit of events 132,540–133,209 (chain continuity, kind histogram, reducer replay), then B14 rehearse-on-copy, then B14-to-head + acceptance, then recover-receipt, then Tracks K→F→M→Assurance on a test computer (self-dev OFF), adopt under fences, driver repair, two-gate release.
- The `desired/effective_event_head` split from `canonical_event_head` (`a3cf16d0` vs `6e7424f0`) is an open question to explain before sealing the recovery target tuple (per consensus).
- The recovery definition's "complete at 132,539" and the overhauls definition's "active/ Track K can begin" now-cards are stale vs this state and must be reconciled before Track K.

## No further mutation

No vmctl lifecycle call, no image touch, no canonical write was performed as part of this receipt. The orphan was reaped via the authorized maintenance stop; all images/quarantines preserved.
