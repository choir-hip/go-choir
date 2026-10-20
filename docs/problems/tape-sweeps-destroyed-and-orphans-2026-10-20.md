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
   - **Pin and unlink are serialized by a dedicated lock:**
     - `writeImmutable` takes `tapeSweepMu` for read on tape namespaces,
       and touches mtime when the file already exists;
     - apply takes it for write per batch.

     This is the same reuse-wins rule the CAS reuse lock gives file-cas.

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
  `zstd -dc <archive>.tar.zst | tar -x -C /var/lib/go-choir/platform-artifacts`
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
