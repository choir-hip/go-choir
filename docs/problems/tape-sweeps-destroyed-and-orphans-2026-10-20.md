# Tape sweeps: orphan pins and destroyed computers' tapes (2026-10-20)

Date: 2026-10-20. Problem and plan record, written before the code. The
code follows it on branch `feat/tape-sweeps-2026-10-20`.

Authority: owner notes §26 (`docs/reports/owner-notes-2026-10-14.md:542`),
2026-10-20:

- "Destroyed computer tapes can be deleted."
- "Orphan receipts too can be deleted". The review's one-day grace still
  applies.
- Safe tape surgery is approved before launch.
- Deletion runs dry-run first, against a manifest, with a short local
  archive where the bytes are small.

The design source is variants 0 and 2a of
[`tape-volume-forward-fix-and-surgery-2026-10-19`](../reports/tape-volume-forward-fix-and-surgery-2026-10-19.md)
(§6.2, §6.3). Variant 2b, evicting old projection payloads of live
computers, is **not** in scope (owner: archive, not delete, until a target
exists).

## Problem

These figures are measured in the host record
([`platform-artifact-small-file-growth-2026-10-19`](platform-artifact-small-file-growth-2026-10-19.md)).

- **Nothing deletes a tape file.** The artifact GC excludes the tape
  namespaces `computer-event`, `computer-event-payload` and `pin-receipts`
  (`internal/platform/artifact_gc.go:100-106`).
- **Destroyed computers.** 224 computers in `computer_event_heads` have no
  vmctl ownership record. They hold 308,221 events, about 1.24M files and
  about 2.7 GB (13.6% of the tape). vmctl's retention prune destroys the VM
  directory and nothing else.
- **Orphan pins.** About 17,391 payload files and 20,792 receipts exceed
  what the chain needs. They are pins whose head CAS never landed.
  `AppendNewPayload` pins before the CAS (`computerevent/appender.go:375`,
  `:657`, then `:682`), and nothing collects a refused append's pins.
- **Cost.** Each file is under 2 KiB, inline in btrfs metadata. Deleting
  them, followed by a metadata balance, is the only way to return that
  metadata.

## Mutation class and protected surfaces

**Class: red.** The deletes are black on the node, and recoverable only
from the local archive the sweep writes first. Protected surfaces:

- the event evidence store (event, payload and pin-receipt files);
- head CAS admission (a tombstone refuses appends);
- replay and payload fetch (a tombstone refuses reads);
- the checkpoint authority's evidence reads.

## Design decisions

1. **Extend the artifact GC, not a new subsystem.**
   - The new code lives in `internal/platform`, beside `artifact_gc.go`.
   - It shares the GC deletion log (`artifact-gc-log/<day>.jsonl`, with the
     same intent-then-unlink record shape) and the endpoint family
     (`/internal/platform/artifact-gc/tape`).
   - It follows the same policy-clamp pattern
     (`PLATFORM_TAPE_SWEEP_MODE`).

   Two deliberate differences, each with a concrete reason:

   - **A separate run lock, not `artifactGCMu`.** That lock also
     serializes explicit projection-base pins
     (`artifact_gc.go:171-176`). A tape sweep reads about 2.26M event files,
     which takes minutes, and would block those pins for its whole run.
   - **The run is asynchronous: POST starts it, GET reads it.** The corpusd
     HTTP write timeout is 120 s (`internal/server/server.go:69`), and a
     full classification takes longer.

2. **Dry-run is the default.**
   - A dry-run writes a manifest
     (`artifact-gc-log/tape-sweep-<run>.manifest.jsonl`): one header line,
     then one item per file, with kind, computer, sequence, namespace,
     digest, path, size, mtime and reason.
   - It also writes the manifest's sha256 and a report with totals.
   - Apply names the dry-run's run id **and** its manifest sha256, and
     deletes only manifest items.
   - Apply is refused unless the configured mode is `active`.

3. **Apply re-checks every item at delete time,** under the tape write lock
   (point 7):
   - the file still exists and is still past grace by mtime;
   - for orphans, still unreferenced;
   - for destroyed computers, still destroyed, with an unchanged head.

   "Still unreferenced" is computed as a delta. The chain is append-only and
   gapless (`internal/platform/event_replay.go:22-27`), so every new
   reference since the snapshot lives in rows above each computer's
   snapshot head. Apply reads those rows by sequence (the indexed point
   lookups replay uses) and the events they name. Every outcome is logged
   against the manifest run id.

4. **The orphan reference set** is the union of:
   - every append receipt's event digest and pin-receipt digests;
   - every event's input and output artifact refs, plus its
     `payload_commitment` and `proposed_effect_ref`, read from the event
     files;
   - every hex digest named in code closures, artifact programs and
     checkpoints.

   If any event file named by a receipt is unreadable, the payload
   namespace is not swept (fail closed). Grace is at least 24 h by file
   mtime; a lower request is clamped up. A reused pin (identical bytes
   pinned again) now refreshes the file's mtime (point 7), so an old orphan
   that becomes referenced again is never inside the delete window.

5. **The destroyed proof, per computer.** All of these must hold:

   - (a) **No ownership record.** vmctl `GET /internal/vmctl/lookup` by
     ComputerID answers 404.
   - (b) **Not in the vmctl registry.** vmctl's ownership list, every
     state including hibernated and stopped, does not contain it.
   - (c) **No route.** No row in `computer_version_route_slots`, whose slot
     id is `computer:<owner>:<computer>` (`internal/routeledger/ledger.go:94-104`).
   - (d) **Quiet.** The head row has not changed for at least the quiet
     window (default 7 days).
   - (e) **No fork-lineage child: not checkable.** No fork-lineage record
     exists anywhere in the code base (forks have not landed, per CLAUDE.md
     and `docs/computer-ontology.md:409-414`). The check therefore **fails
     closed**. A computer is deletable only when the dry-run request
     **and** the apply request both carry
     `"accept_unrecorded_fork_lineage": true`. The manifest records that
     acknowledgement in each proof. Without it, destroyed computers are
     reported and nothing of theirs is listed for deletion.

   If vmctl is not configured, or any registry or route query fails, the
   whole destroyed sweep is refused.

6. **A tombstone, not row deletion.** A new table,
   `computer_tape_tombstones`, has one row per computer. Its status moves
   archiving → archived → deleted, and it records the manifest run and
   hash, the archive path and hash, the counts and the proof.

   Once the row exists:

   - head CAS refuses the computer;
   - `EventsPage`, `FetchPayload` and the checkpoint evidence reads return
     `ErrComputerTapeDeleted` ("computer tape archived/deleted") with the
     archive path, not a chain-gap or file-not-found error.

   `computer_event_heads` and `computer_event_append_receipts` rows are
   **kept**:

   - they are small, and Dolt history keeps them anyway, so deleting them
     reclaims nothing until a platform Dolt GC;
   - they are the sequence-to-digest index that makes the archive
     verifiable and restorable;
   - the head row stops a reused ComputerID from starting a second genesis
     under the same identity;
   - deleting rows from the CAS tables would widen the red surface for no
     gain.

7. **Archive before delete, plus a write lock.**
   - Before any file of a computer is unlinked, its tape files go into
     `<archive_dir>/<computer>-<run>.tar.zst` with an embedded
     `MANIFEST.jsonl` (path, sha256, size).
   - A sidecar `<archive>.json` records the archive's sha256, the
     manifest's sha256 and the counts.
   - The archive is re-read and every entry is re-hashed against its
     content-addressed name before the tombstone moves to `archived`.
   - Orphans are archived the same way, one archive per apply run.
   - The archive directory defaults to
     `<artifacts_root>/tape-sweep-archive`, because corpusd's systemd unit
     may write only there (`ReadWritePaths`). Keep it about 7 days; pruning
     is an explicit request field, never automatic.
   - **Pin and unlink are serialized by the existing CAS reuse lock**
     (`store.casReuseMu`; the plan said a dedicated `tapeSweepMu`, and the
     code reuses the lock file-cas and og already share):
     - `writeImmutable` takes it for read, and touches mtime when the file
       already exists;
     - apply re-checks and unlinks each item under it for write
       (`unlinkIfStillUnreachable`, shared with the artifact GC).
   - **Append and tombstone are serialized by a tombstone gate.** Head CAS
     holds `tombstones.gate` for read from its tombstone check through its
     commit; apply holds it for write while it writes the tombstone and
     re-reads the head, so no append lands after that check.

8. **Bounded deletion.**
   - Batches default to 256 items, with a pause between batches (default
     250 ms).
   - A run is capped by max deletes and max bytes, so a capped run leaves
     the rest for the next apply of the same manifest (idempotent:
     already-gone items log `gone`).
   - The unlink thread does not get idle IO priority. The Go runtime offers
     no portable ioprio, and the existing GC does not set it either. The
     pause and the caps are the bound instead.
   - Log lines, prefixed "tape sweep:", carry counts and bytes per phase.

## Failure modes, written before the code (each pinned by a test)

1. **A referenced file deleted.**
   - An orphan sweep deletes an event, receipt or payload that a live chain
     names, including a payload shared between a destroyed and a live
     computer, and one named only by a code closure.
   - Test: these are kept, with reasons.
2. **A partial reference set taken for complete.**
   - An unreadable event file, a missing route table or a failed vmctl
     list.
   - Test: the payload namespace, or the destroyed sweep, is refused; no
     items are listed.
3. **In-flight pins taken for orphans.**
   - Test: a file younger than grace is kept, and a request for grace under
     24 h is clamped to 24 h.
4. **An orphan re-referenced between dry-run and apply.**
   - A new append names it, or a re-pin refreshes it.
   - Test: apply keeps it (`kept_referenced` / `kept_refreshed`).
5. **A computer that is not actually destroyed.**
   - It has an ownership record, is in the registry, has a route, has
     appended recently, or appends between dry-run and apply.
   - Test: it is never listed, or apply skips it and removes its own
     tombstone.
6. **The fork-lineage check is absent.**
   - Test: without the acknowledgement, no destroyed computer's file is
     listed or deleted.
7. **Apply without a matching manifest.**
   - Wrong run id, a tampered manifest (hash mismatch), or mode not
     `active`.
   - Test: refused; nothing deleted.
8. **Delete before a verified archive.**
   - Test: a failed archive write leaves every file in place and removes
     the tombstone. The archive entries re-hash to their names.
9. **Readers see a chain gap instead of an explicit error.**
   - Test: `EventsPage`, `FetchPayload` and CAS on a tombstoned computer
     return `ErrComputerTapeDeleted`.
10. **Runaway deletion.**
    - Test: max deletes and max bytes stop a run, and a re-apply finishes
      it idempotently.
11. **A crash mid-apply.**
    - Every unlink is preceded by a durable (fsynced) intent record, and a
      re-apply resumes.
    - Test: this is the existing GC log mechanism, re-used.

## Rollback

- **Code:** revert the code commits. The tombstone table stays, harmless,
  and without the code nothing reads it. A reverted build must not run
  while a tombstone is in `archiving`/`archived` state for a computer
  whose files are still present. That is safe either way: files present,
  no tombstone check.
- **Data:** before an apply, nothing is deleted. After an apply, restore the
  files from the archive:
  `zstd -dc <archive>.tar.zst | tar -x -C /var/lib/go-choir/platform-artifacts --exclude=MANIFEST.jsonl`
  restores the files at their paths. Then delete the tombstone row, or call
  the untombstone request, which is allowed only while nothing has been
  deleted.
- **Policy:** set `PLATFORM_TAPE_SWEEP_MODE=dry-run` (apply refused) or
  `off`.

## Heresy delta

- discovered:
  - no fork-lineage record exists, so "no fork child" cannot be checked
    (handled by fail-closed plus an explicit acknowledgement);
  - `writeImmutable`'s reuse path never refreshed mtime, so any future tape
    retention would have raced re-pins;
  - an operator can reach `/internal/platform/artifact-gc*` only from a
    root shell on Node B (curl with the internal-token header): no-SSH
    operability is still missing, as for the existing GC.
- introduced: none intended. The tombstone check adds one in-memory map
  lookup to CAS, replay and payload fetch.
- repaired: the tape namespaces get a deletion path for destroyed computers
  and orphan pins.

## Implementation notes (code commit)

- Files: `internal/platform/tape_sweep.go` (policy, request, job,
  dry-run), `tape_sweep_apply.go` (apply, archive, prune),
  `tape_tombstone.go`, and the route in `handlers.go`.
- Endpoint: `POST /internal/platform/artifact-gc/tape` starts a job and
  answers 202 with its `job_id`; `GET ...?job=<id>` (latest when empty)
  reads its phase and report. Actions: `dry-run` (default), `apply`,
  `untombstone` (only while nothing was deleted), `prune-archives`
  (`older_than`, at least 168h; explicit only).
- Node B starts in `PLATFORM_TAPE_SWEEP_MODE=dry-run`. Apply needs a nix
  change to `active` after the dry-run manifest is reviewed.
- Reference sets are sorted 8-byte digest prefixes. A collision keeps a
  file and never deletes one. The extra records scanned for 64-hex digests
  are code closures, artifact programs, checkpoints (checkpoint and
  receipt JSON) and route projection certificates. A failure on any of
  them refuses both sweeps.
- An unfinished destroyed apply (a tombstone not in `deleted`) refuses the
  orphan payload sweep until it finishes, because that computer's payload
  references may already be gone.
- There are no Prometheus metrics in corpusd. The job status (GET), the
  report JSON beside the manifest, and the `tape sweep:` log lines carry
  the counts and bytes.

## Acceptance (to run after merge; not run here)

1. Deploy.
2. Dry-run on Node B. Expected:
   - about 224 destroyed computers reported, deletable only with the
     acknowledgement, about 1.24M files, about 2.7 GB;
   - about 38k orphans: payloads and receipts, plus a few event files.
3. Review the manifest.
4. Apply with the run id and manifest hash.
5. Confirm, from the report and the GC log:
   - the tape file count drops by the manifest totals;
   - archives exist with verified sidecars;
   - replay of the two live computers still pages;
   - a tombstoned computer's replay returns `computer tape archived/deleted`.
6. Then run a metadata balance (`btrfs balance -musage=...`).

## Dry-run receipt (2026-10-20 05:16–05:51 UTC, on 4128cb16f)

Run `ts-20261020T051653Z-fd085f79`, `accept_unrecorded_fork_lineage=true`,
manifest sha256 `ebac0efd51ba761d5322a2dcbf9468908f0b6266e5b53f7b474270d8ed903384`
(`/var/lib/go-choir/platform-artifacts/artifact-gc-log/tape-sweep-ts-20261020T051653Z-fd085f79.manifest.jsonl`).
Two earlier dry runs (02:16, 03:45) were killed by corpusd restarts during
deploys; this one ran on a quiet host and took 34 minutes.

- Walked 228 computers, 2,300,908 chain events, through the tape-segment
  reader landed in 4128cb16f (stage A1), with no read error.
- **Destroyed:** 162 computers listed and deletable, 763,610 files,
  2,301,811,887 bytes; 0 blocked by fork lineage. Fewer than the 224 /
  1.24M / 2.7 GB estimate above: the estimate counted tape directories
  without ownership, the sweep lists only computers that pass every check.
- **Manifest review.** Every proof has `ownership_lookup=not_found`, not in
  the registry list, no route, quiet; the newest head is 2026-10-13 04:58
  UTC. None of the 162 ids appears in the vmctl list (151 computers), and
  the paper's computer is not among them. 11 proofs carry
  `shared_payloads_kept`.
- **Orphans refused.** "append receipts (2304539) do not match the
  head-bounded chains (2300908)": 3,631 more receipts than chain events.
  The check fails closed as designed (failure mode 4). Hypothesis, not
  confirmed: receipts appended by live computers after the walk took
  their heads. A second hypothesis is receipts above heads that a
  rewind left behind. The orphan sweep stays unapplied until the count
  is explained.

Next: `PLATFORM_TAPE_SWEEP_MODE=active` (owner §26: "Destroyed computer
tapes can be deleted"), then apply this run id and manifest hash. Apply
re-proves each computer, tombstones it, archives its files to
`tape-sweep-archive/` and verifies the archive before any unlink.

## Apply receipt (2026-10-20 06:00–06:15 UTC, on 5a3cffbe6)

Job `ts-20261020T060033Z-eb7c5980`, applying run
`ts-20261020T051653Z-fd085f79` with manifest sha256 `ebac0efd…d903384`.

- 162 computers re-proved, tombstoned, archived and deleted; 0 skipped,
  0 errors.
- 763,462 files and 2,301,580,825 bytes deleted. 148 files kept as
  referenced by live chains (the `shared_payloads_kept` proofs).
- 162 archives, `tape-sweep-archive/<computer>-<run>.tar.zst`, 314 MB in
  all, each with a sidecar. Net reclaim about 2 GB of data. btrfs
  metadata used fell from 15.59 GiB to 14.70 GiB (83.6% to 78.8% of
  18.64 GiB).
- After apply, the paper stayed ready on 4128cb16f and published at
  06:12 and 06:13 UTC.
- Not verified here: replay of a tombstoned computer returning the
  explicit archived error. The replay route needs the computer's own
  event capability, which an operator does not hold; unit tests cover
  it (failure mode 9).
- Not run: the metadata balance (acceptance step 6). Metadata is under
  80% now; a balance is heavy I/O on the live host and can wait for a
  quiet window.
- The orphan sweep is still unapplied (receipt-count mismatch above).

## Problem: the orphan check can never pass on a live host (2026-10-20 06:35 UTC)

Trace, not hypothesis: `tapeSweepDryRun` (internal/platform/tape_sweep.go)
snapshots every computer's head at the start, walks each chain up to that
head while counting `receiptRows`, and only after the walk runs
`SELECT COUNT(*) FROM computer_event_append_receipts` over the whole
table. The walk took 34 minutes on staging. Every receipt appended by a
live computer in that window is in the total and not in `receiptRows`.
The 3,631 surplus in run `ts-20261020T051653Z-fd085f79` is about what the
paper and the other live computers append in 34 minutes at the measured
~268,000 events a day (about 6,300 per 34 minutes at the daily mean).
So the orphan sweep refuses on every run while any computer is working,
which on this platform is always. The earlier "rewind leftovers"
hypothesis is not needed to explain the count, but the check exists to
catch exactly those, so the fix must keep catching them.

Fix shape (red, deletion logic):

- Bound the count by the same snapshot: per computer in `ids`,
  `COUNT(*) WHERE computer_id = ? AND sequence <= head.Sequence` must equal
  that computer's walked rows.
- Receipts above the snapshot head are classified, not counted: a receipt
  at or below the computer's *current* head is a live append (its files are
  inside the 24 h grace and apply re-checks references in its delta
  phase); a receipt above the current head is a leftover past the head and
  still refuses the orphan sweep.
- Receipts for a computer not in `ids` (no head row) still refuse.
- Tests first: a fixture that appends during the walk passes; a receipt
  above the current head refuses; a receipt for a computer with no head
  refuses.
