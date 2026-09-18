# Design: platform-dolt storage normalization — 2026-09-18

Class: green (problem documentation + design; no runtime change). Owner asked
for a durable fix for the platform-dolt storage leak. This is the design surface
for triage; implementation is a separate orange/red mission.

## Invariant (owner-stated)

The system must keep a **secure, auditable, immutable event log**, plus
**snapshots from which an autoputer's current state can be recovered after
corruption**. Everything else is negotiable.

## Diagnosis (measured 2026-09-18)

The platform store (`/var/lib/go-choir/platform-dolt/platform`, a `dolt
sql-server`) grew to ~224 GB before a manual `dolt dump` + reimport today
compacted it to 9.2 GB (`dump-20260918`, 20 GB SQL). Byte share of the dump:

| Table | Size | Nature |
| --- | --- | --- |
| `og_objects` | 12.6 GB | object graph — mutable, high-churn, bulk |
| `items` | 2.3 GB | corpus (GDELT/RSS `body`, `raw_json`, `reader_snapshot`) |
| `og_edges` | 1.8 GB | object graph |
| `ingestion_events` | 0.9 GB | corpus |
| `fetches` | 0.9 GB | corpus |
| `computer_event_append_receipts` | 0.39 GB | **canonical event log** |
| `computer_lifecycle_receipts` | 0.03 GB | canonical |

Two compounding causes:

1. **No GC on the platform store.** `DOLT_GC()` is wired only for the guest's
   *embedded* store (`internal/store/dolt_maintenance.go`, `MaybeRunDoltGC` /
   `StartPeriodicDoltGC`). The platform `dolt sql-server` has no scheduled GC, so
   the noms journal accumulates every written chunk forever. The 224 GB was
   ~215 GB of uncollected garbage over ~9 GB live — the same failure as the
   guest's documented 9.8 GB-journal-for-0.5 GB-live receipt (2026-09-03).
2. **Bulk mutable data shares the canonical store.** The auditable event log
   (~0.4 GB, must be immutable + retained) is co-located with ~19 GB of
   high-churn corpus/object-graph data that does not need Dolt versioning. Dolt
   versions every revision of `og_*`/`items`/`fetches`, so the churn multiplies
   the journal and forces GC/retention policy to be sized for bulk data, not the
   log.

## Durable fix — two moves

### Move 1: scheduled platform GC (stops the leak now)

Run `CALL DOLT_GC()` against the platform sql-server on a schedule, with the
same milestone/disposition discipline the guest already uses.

- Mechanism: a systemd timer (or a `vmctl`/corpusd maintenance hook) that runs
  `dolt sql -q "CALL DOLT_GC()"` (or `dolt gc` against the data dir while the
  server holds it — prefer the SQL call so it coordinates with the live server).
- Trigger: journal-size milestone + a periodic floor (e.g. GC when
  `noms/vvvv…` journal > N GiB, or every T hours whichever first). Reuse the
  `doltGCPlan` thresholds concept; write a `.choir-dolt-gc-disposition.json`
  beside the store so skips are observable (per the 2026-09-03 receipt).
- Safety: `DOLT_GC()` on a live server needs the GC to see the working set;
  confirm the platform server version supports online GC, else run it during a
  brief read-only window. Keep the last-K commits reachable so the auditable
  history is not truncated below the recovery watermark.

This alone bounds the journal; it does not shrink live data.

### Move 2: separate bulk data from the canonical store (shrinks live data + makes the log queryable)

Split the platform store into two physical stores with different retention:

- **Canonical event store** (immutable, versioned, retained): the
  `computer_event_*`, `computer_lifecycle_*`, `computer_checkpoint*`,
  `computer_replay_watermarks`, `computer_version_*`, `computer_key_*`,
  `computer_route_*`, `consent_records`, `control_key_history`, receipt and
  rollback-ref tables. This is the auditable log + recovery metadata. Keep it in
  Dolt (versioning is the audit feature) with scheduled GC bounded to the
  recovery watermark.
- **Bulk/corpus store** (mutable, non-versioned or shallow-versioned):
  `og_objects`, `og_edges`, `items`, `fetches`, `ingestion_events`,
  `cycle_events`, `cycles`, `processor_requests`, `provenance_*`,
  `publication_*`, `platform_texture_revisions`, `artifact_*`. Move to a plain
  SQLite/Postgres store (or a Dolt store with aggressive GC + no long-term
  history requirement). These are derived/rebuildable or high-churn; they do not
  need immutable history.

Normalization within the bulk store (the "use less disk + easier to query"
ask): the `items`/`og_objects` payload columns (`body`, `raw_json`,
`reader_snapshot`, object blobs) should be **content-addressed** — store the
blob once in a `blobs(digest, bytes)` table and reference it by digest, exactly
as `computerevent` already does for event payloads (`PayloadCommitment` → CAS
pin). Dedup removes the repeated GDELT/RSS bodies and object copies; the digest
keeps integrity. Queries then hit small index rows, not fat payload rows.

## Recovery invariant preserved

- The canonical store keeps the full event log + `computer_checkpoints` +
  `computer_replay_watermarks` → an autoputer can be rebuilt by replaying events
  from the last checkpoint (the existing restore path).
- Moving `og_*`/corpus out does not break recovery: those are derived
  projections/corpus, not the event source of truth. If a consumer needs them
  for replay, regenerate from the canonical log or keep a non-versioned mirror.
- GC must never collect chunks reachable from the recovery watermark — bound GC
  to `computer_replay_watermarks` so the log is never truncated below the last
  recoverable head.

## Migration path

1. Land Move 1 (scheduled GC) first — stops the leak immediately, low risk.
2. Add the bulk store alongside; dual-write or backfill `og_*`/`items`/`fetches`
   from the dump; cut readers over; then drop the bulk tables from the canonical
   store and `DOLT_GC()` to reclaim.
3. Content-address the payload columns in the bulk store as part of the move.

## Open questions for triage

- Does any consumer read `og_*`/`items` *through* the canonical store's Dolt
  history (time-travel queries)? If yes, they need a versioned mirror or a
  snapshot table before the split.
- Platform `dolt sql-server` version — confirm online `DOLT_GC()` support, or
  plan a maintenance window.
- Retention policy for the canonical log: keep-all (audit) vs. watermark-bounded
  GC. Owner decision.

## References

- Guest GC precedent: `internal/store/dolt_maintenance.go` (`MaybeRunDoltGC`,
  `StartPeriodicDoltGC`, `.choir-dolt-gc-disposition.json`).
- Journal-growth receipt: `dolt_maintenance.go` comment, 9.8 GB journal / 0.5 GB
  live, 2026-09-03.
- Event payload CAS precedent: `internal/computerevent/appender.go`
  (`PayloadCommitment`, `PinNonPrivatePayload`).
- Prior disk-pressure receipt: `docs/evidence/node-b-deploy-disk-preflight-floor-2026-08-26.md`.
- Storage retention mission: `docs/archive/mission-node-b-storage-retention-v0.md`.
