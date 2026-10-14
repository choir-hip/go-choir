# The Paper's CPU Is Unindexed Store Scans, Not Agent Work

Date: 2026-10-14. Found by a 60-second guest CPU profile of the paper
computer's main `autoputer` process at 16:46 UTC
(`docs/evidence/computer-cpu-guest-profile-2026-10-14T1646Z.txt`, taken
by `scripts/cpu_efficiency_probe.py --profile`), with a host sample over
the same window. Status: documented; the fixes are levers C1 to C4 in
[computer CPU efficiency](../reports/computer-cpu-efficiency-2026-10-14.md).
Fix commits cite this record.

## Evidence

- The paper's Firecracker process used 1.29 cores in the window: vCPU
  guest mode 1.19, KVM halt polling 0.07.
- The main daemon used **1.17 cores**, 98% of guest time. All other guest
  processes, including the 12 Yaegi workers, used 0.03.
- 34 model calls in the window (31 a minute over the 20 minutes to
  16:50): **2.27 CPU-seconds per model call**.
- **73%** of the daemon's CPU is under `database/sql.(*Rows).Next`, inside
  the embedded Dolt engine. **40%** is under `encoding/json.Unmarshal`.
- **52%**: `store.listInboxRecordsBy` → `ogListAllByMetadata` →
  `objectgraph.(*DoltStore).ListObjectsByMetadataPage`. It filters
  `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?` over every
  `commitment_record` (`internal/objectgraph/dolt_store.go:765-770`).
  - **44%** of it is `recheckRecordsInbox` → `closeRoundsOfEndedRun` →
    `OpenRounds` → `ConversationState`, at the end of every desk run.
    That is 2 scans for the inbox's conversations, then one scan per
    conversation (`internal/store/inbox.go:850-902`).
  - **8%** is `loadCellRecords` → `ListInboxRecords` per cell.
- **11%**: `actingCommitmentPackForDesk` → `ListCommitmentClaimRecords` →
  `ListObjectsByOwnerAndBody`. It runs per cell, as a `JSON_EXTRACT` on
  the body of every commitment record of the owner
  (`internal/store/commitment.go:122-150`,
  `internal/objectgraph/dolt_store.go:845-880`).
- Model request build and reply parse, transcript reads and canonical JSON
  are each under 0.5% or about 1%.

## Reading

- The cost is set by the number of stored commitment records, not by the
  work done. Captures add about 270 an hour at 28 sources.
- Hypothesis, unmeasured: the paper's CPU per model call rises over the
  computer's life.
- An indexed replacement half exists. `metadataIndexColumns` (generated
  columns for `run_id`, `trajectory_id`, `channel_id`,
  `dolt_store.go:150-200`) is used by `ListObjectsByMetadata`
  (`:686-699`).
  - It is not used by the paged variant that `ogListAllByMetadata` calls
    (`internal/store/graph_store.go:138-165`).
  - It has no column for `inbox`, `sender_inbox`, `conversation_id` or
    the body's `directive`.

Same substrate as:
- the Texture list latency after idle (`internal/server/debug_pprof.go`
  header);
- the heap under `AppendNewPayload`
  (`docs/problems/autopaper-intake-defects-2026-10-14.md`).

All three are the object graph used as a JSON document store with
scan-and-filter reads under one engine mutex. The next fixes are
substrate-level: indexed access paths, not per-caller patches.

## Options

1. Wire the generated columns into the paged read and add the missing
   fields (C1). Then fold rounds only for the conversations an ended run
   touched (C2). Then claims by index (C3) and the cell inbox as a range
   read (C4). Orange (queries, guest schema at boot); S each.
2. A typed projection table for commitment records (inbox, conversation,
   seq, directive), maintained on the tape's projection path. Larger (R),
   and it subsumes 1.
