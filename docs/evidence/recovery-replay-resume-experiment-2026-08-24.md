# Recovery replay resume experiment (A2/A3) — 2026-08-24

Status: **resumable durability PROVEN; full-chain run interrupted by a separate
post-replay panic (see the sibling receipt on the nil-executor CoSuper fate
panic).**

## Setup (staging, node-b, root, disposable copy)

- Copy of the retained computer disk
  (`computer-03335285269bdba4f94377e56879f9e6`,
  `candidate-fleet-e15cb89f25d963c220319b7b/data.img`, 32 GiB ext4,
  LABEL=go-choir-data) at `/tmp/recovery-exp/data.img` (btrfs reflink, sparse),
  loop-mounted rw at `/tmp/exp-mnt`. The real data.img was never touched
  (verify: sha256/mtime unchanged; the 00:31 mtime is pre-experiment).
- Runtime: the deployed same-commit guest package
  `/var/lib/go-choir/services/autoputer/bin/autoputer` (build `63168fb3`,
  deploy receipt 2026-08-24T00:38Z).
- Credential path: fresh 4-min envelope minted via
  `POST 127.0.0.1:8086/internal/computers/credentials/issue`
  (`X-Internal-Caller: true`, computer/realization
  `candidate-fleet-e15cb89f25d963c220319b7b-epoch-709`), canonicalized by the
  same `CanonicalJSON` rules (UTF-16 key sort, struct field order for the
  typed envelope) and exchanged at startup exactly as the guest does
  (`ExchangeGuestCredential` → capability → `NewGuestHTTPClient`);
  `RUNTIME_STORE_PATH=/tmp/exp-mnt/state` (the copy's marker path; the Dolt
  workspace `state.texture/` on the copy). Env mirroring the guest boot args
  (fc-config.json) with `CHOIR_PLATFORM_URL=http://127.0.0.1:8086`.

## A2 — SIGKILL mid-replay resumes from the committed head

Run 1 (pid 3510815, 01:17:28Z):

- `curl 127.0.0.1:8099/health` → the replay gate:
  - t0: `{"committed_sequence":57622,"sequence":57763,"status":"replaying"}`
  - t0+20s: 58,622 / 59,097
  - t0+40s: 60,122 / 60,360
- `kill -9` at sequence ~60,360 / committed 60,122. Workspace on disk after
  kill: `state.texture` 1.8 GiB; `.dolt/noms` + `repo_state.json` present
  (committed head durable).
- Re-run (01:22:27Z, same store, fresh envelope):
  - health: `{"committed_sequence":62651,"sequence":63116,"status":"replaying"}`
  - The replay RESUMED at the durable committed head (~62.6k — it had
    checkpointed further between the last poll and the kill) and continued
    monotonically to 130,329; no reset to 0, no double-apply (sequence stayed
    strictly increasing across the reopen).

**Verdict: SIGKILL mid-replay → reopen resumes from the committed head without
re-applying checkpointed events. B7/B10 durable checkpointing is proven on the
real data artifact.**

## A3 — measured pace, RSS, disk

From the resumed run (run 2, then run 3 continuation had the same shape):

| sample | sequence | RSS | workspace |
|---|---|---|---|
| t1 01:27 | 70,351 | 1,194,612 KB (~1.14 GiB) | 1.3 GiB |
| t2 01:29 | 73,414 | 1,199,628 KB | 1.6 GiB |
| t3 01:31 | 76,458 | 1,264,132 KB (~1.21 GiB) | 1.9 GiB |
| t4 01:35 | 96,101 | 1,552,976 KB (~1.48 GiB) | 3.9 GiB |
| t5 01:40 | 115,298 | 1,804,996 KB (~1.72 GiB) | 5.9 GiB |
| t6 01:44 | 130,329 | 1,948,796 KB (~1.86 GiB) | 7.6 GiB |

- Pace: 27–82 ev/s; mean ~50 ev/s (vs Phase-0's 3.8 ev/s estimate — the old
  substrate mis-measure; the actual full replay window is ~25-30 min ≈ one
  30-min resume quantum, so just **1-2 boot sessions** (not ~20)).
- RSS 1.14 → 1.86 GiB over replay; peak ~1.9 GiB (fits the 4096 MiB guest,
  marginal at head with the runtime stack — watch the recovery).
- Workspace disk 1.3 → 7.6 GiB (~60 KB/event incl. payload pins); at
  132,436 extrapolates ~8-9 GiB (fits the 32 GiB data.img).

## Purity checks

- Platform tape head unchanged after both runs:
  `sequence 132,436, canonical_event_head 8df7efbba...` — NO experiment
  appends reached the real tape (reconcile receipts were empty; no lifecycle
  appends occurred).
- Real data.img untouched (experiment ran entirely on the reflink copy).

## Run 3 (re-run with the capsule executor configured) — full-chain finish

Run 2's reconstruct COMPLETED (fetch drained through the tape) and the runtime
started; it then panic-crashed in
`ReconcileCoSuperAssignmentsForTrajectory` (typed-nil executor — see the
sibling receipt). Run 3 re-ran against the same workspace with the capsule env
set (dummy broker/state/source/lower paths — the executor's capsule map is
empty, which the reconcile handles gracefully):

- Resumed from the durable committed head (~131k), completed the remaining
  tape, and the health gate flipped:
  `{"status":"ready","service":"autoputer",...,"runtime_health":"ready"}`
- Runtime started, passivated a stale lifecycle run and dispatched the
  persistent-Super rewarm (`run=fe92ea2b-86ba-4deb-90ae-d73ec985f3e6`).
- The process was killed immediately after (the harness has no gateway token;
  the Super could not execute anyway).

## Live-tape impact of run 3 (IMPORTANT — target head moved)

The post-replay runtime is DESIGNED to report and record; run 3's runtime
appended 103 events to the real platform tape:

- `key_revoked` × 2 (credential envelope consumption for the exchange — the
  same lifecycle event a real boot records) at 132,437/132,438.
- `projection_batch_recorded` × 101 (the replay's projection batches recorded
  authoritatively to the tape) 132,439..132,539.

Platform head after run 3: **sequence 132,539, canonical_event_head
`acc54c39ee05d89af13223e3b8cca195e04d7dfc8f137ce1bb27b96f657b7201`** (was
132,436 / `8df7efbba...`). No run-lifecycle events were appended (the Super
dispatch had not written any before the kill). These are authentic chained
events (CAS-accepted), not corruption; the recovery target for 0333528 must
now be the head 132,539 (B9 update in the definition — the old 132,436/
8df7efbba target is stale).

## Remaining gap

Run 3 closes the full-chain gap: head 132,539 reached → 200 health (ready) →
runtime started with the capsule executor configured. The pending A4/B9
completion checks (head+witness equality at the NEW head 132,539, route CAS
under fencing token, quiesced active state) remain for the REAL recovery
BootVM on 0333528.
