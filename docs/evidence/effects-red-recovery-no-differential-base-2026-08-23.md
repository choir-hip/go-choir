# Recover-current has no healthy differential base; full-tape boot rebuild is the substrate flaw

Date: 2026-08-23
Mutation class: red

## Summary

The retained computer `computer-03335285269bdba4f94377e56879f9e6` (VM
`candidate-fleet-e15cb89f25d963c220319b7b`, epoch 361, state `stopped`,
stopped_by `recover_current`) is unrecoverable by the current `recover_current`
design, and there is **no healthy recent projection base on the host to diff
against**. Any differential-recovery strategy must therefore manufacture a base
(in one non-scalable offline build) rather than find one, and the platform must
then publish verified projection bases so recovery is O(delta) and multiuser-safe.

## Probe results (2026-08-23, read-only host inspection)

### 1. The only quarantined image is a blank-seed partial, not a healthy base

`data.img.quarantine-2-e0b4a9d4206a50fb` (2026-08-23 13:27:59) was mounted
read-only (raw ext4, no partition table, `losetup -fP`). Contents:

- `state` — a 0-byte file (not a Dolt store).
- `state.texture/texture/.dolt` — a bare repo shell only (`repo_state.json`,
  `config.json`, `noms/` with empty `manifest`, `LOCK`, `journal.idx`) — no
  committed rows.
- `choir-credentials`, `choir-signers`, `choir-updater` — secret/release
  scaffolding.
- **No `/mnt/persistent/state`** (the guest `RUNTIME_STORE_PATH` per
  `nix/autoputer-vm.nix:727`) and **no `/mnt/persistent/files`**. No
  `computer_event_projection_heads` table, no embedded Dolt projection.

Conclusion: this quarantine was produced by the broken blank-seed path itself
(only the privacy key is copied; `internal/vmctl/trusted_guest_copier.go`). It
is not a pre-failure realized computer. There is no projection-bearing
`data.img` on the host for this computer.

### 2. The canonical chain is intact and is the source of truth

From corpusd Dolt (`computer_event_append_receipts`, computer_id =
`computer-03335285269bdba4f94377e56879f9e6`):

| event_kind                | n      | minseq | maxseq |
|---------------------------|--------|--------|--------|
| projection_batch_recorded | 132317 | 29     | 132436 |
| key_revoked               | 107    | 2      | 83847  |
| trajectory_started        | 9      | 33     | 27955  |
| restore_requested         | 2      | 16     | 21     |
| genesis_imported          | 1      | 1      | 1      |

Head sequence = 132,436. Chain is fully present (no rewind needed).

### 3. Host quarantine retention prunes to maxRetained=3

`internal/vmctl/cold_recover.go:543` calls
`storage.QuarantineDataImage(root, vmid, generation, operationID, 3)`. Across
the ~4-5 failed recovery attempts, only `quarantine-2` (blank partial) and the
current in-progress `data.img` remain. Any healthy pre-failure image has been
pruned or was never retained.

### 4. The cost driver is per-event serializable transactions (confirmed)

`internal/store/computer_events.go` `finalizeBatch`: for each replayed event it
opens `s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})`
(`computer_events.go:96`), does a `FOR UPDATE` head load/CAS, updates the
projection head, finalizes the index row, and loops every batch op via
`projectBatchForReplay` → `projectOp` (`project.go:45`). One tx per event ×
132,436 events, plus per-op SQL. `53aca583` deferred the Dolt checkpoint to
`CommitReplay`, but did **not** batch the per-event transactions or the per-op
applies.

- `EventReplayPageSize = 1024` (`computerevent/event.go:21`).
- Guest `bootstrapCtx = 30*time.Minute` (`autoputer/run.go:155`).
- `VM_BOOT_READY_TIMEOUT` default 30m (`vmmanager/config.go`).

### 5. No platform-side projection snapshot publish exists

`DoltHeadSnapshot` (`computerversion/dolt_head_snapshot.go`) and
`ObjectGraphSnapshot` (`computerversion/object_graph_snapshot.go`) are
**fixture-only, non-production** observation types. `VMLocalContentWitness`
(`selfdevprotocol/restore.go:27`) hashes + Dolt audit head — an audit receipt,
never restorable projection bytes. `99949fe2` is a head witness at sequence
3403, not stored bytes. The implemented rematerializer opens a fresh store and
replays the tape (`agentcore/rematerialize.go`). There is **no** existing
mechanism to publish/consume a content-addressed projection base.

### 6. `rematerialize-from-tape` is guest-dependent; it cannot help a down guest

`POST /api/computers/{id}/lifecycle/rematerialize-from-tape` forwards to the
live Runtime (`cmd/choir/main.go:1425`; `proxy/computer_lifecycle.go`). A down
guest cannot receive it. The only host-orchestrated path is
`/internal/vmctl/computers/{id}/cold-recover` (`recover_current`).

## Consequences

1. **No differential base exists.** Rail A (unblock this computer) must be a
   **non-scalable, resumable offline full-tape build** that manufactures the
   projection to head 132,436 (page-chunked transactions + one final Dolt
   commit), then publishes it as the first base. This is `<8h`-class work.
2. **Recovery must become differential + scaled.** Rail B is the durable
   substrate: publish typed `ProjectionBase` artifacts at event-head watermarks
   (bound to computer_id, sequence H, canonical head, reducer/projector/schema
   versions, VM-local content witness, content-addressed encrypted bytes).
   Recovery = verify H is an ancestor → hydrate base → replay only H+1…current
   (already incremental in `appender.go:583`) → verify final head/witness. This
   makes recovery O(delta) and independent of computer age × user count.
3. **The 30-minute boot budget must never gate an unbounded full replay.** Full
   replay is an audit/fallback job, not the boot-critical path.

## Repair boundary

- Do NOT rewind canonical corpusd events; do NOT SQL-empty/replace the retained
  computer. Chain (head 132,436) is authoritative.
- Do NOT raise `maxToolLoopIterations`; do NOT cancel in-flight Super; do NOT
  HTTP Super-start. Effects remain OFF.
- Mutation class RED; full ceremony on any repair.
- Problem-documentation-first: this receipt precedes any code fix.
