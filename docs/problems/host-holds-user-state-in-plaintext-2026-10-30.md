# The host holds every user computer's state changes and snapshots in plaintext (2026-10-30)

Status 2026-10-30: projection bases are deleted (the host keeps none). The remaining host-side surfaces are the tape (pinned, compare-and-swapped, finalized after the receipt verifies), sealed tape segments in platform artifacts, the file CAS, and the PostgreSQL control and graph databases. Read the Dolt-era surfaces named below as historical. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Class: green (problem record). Surfaces: tape (red), projection bases,
key custody. Found by FP0 while scoping the restore design.

## Evidence [V]

1. **Projection batches are pinned to the host unencrypted.** Every write a
   guest store makes is recorded as a `projection_batch_recorded` event whose
   payload is the batch of row operations (`internal/store/projection_tape.go:124-150`).
   The event and payload carry `PrivacyClass: "owner"`, and the append goes
   through `AppendNewPayload` (`internal/computerevent/appender.go:423-466`),
   which pins through `PinNonPrivatePayload`: the payload is content-addressed
   and stored as given, with no envelope. Only payloads flagged `Private`
   take the encrypting path (`PreparePrivatePayload`, `appender.go:592-602`).
   The host side checks the envelope only for `privacyClass == "private"`
   (`internal/platform/event_artifacts.go:53-63`). With
   `PLATFORM_TAPE_SEGMENTS=write` (`nix/node-b.nix:1321`) these payloads land
   in the host's `tape-seg` segments. The replay reader maps `owner` to
   `public` when resolving them (`appender.go:1116-1118`).
   Consequence: the host holds, in plaintext, the content of every guest
   store write: documents, Texture revisions, desk state, and whatever else
   lives in the guest store.
2. **Projection bases are plaintext copies of guest stores** (61.6 GB, no AEAD
   in non-test `internal/projectionbase` files; recorded in
   `docs/design/object-storage-2026-10-30.md` §1).
3. **The event records themselves** are pinned with the label `private` but as
   canonical JSON, unencrypted (`event_artifacts.go:46-47`); they carry
   metadata (kinds, digests, actors, times), not row content.
4. **A host-to-guest key path does exist**:
   `HandleRealizationKeyDelivery` (`internal/platform/key_delivery.go:45+`)
   wraps an escrowed computer key to a realization's recipient public key.
   The object-storage design (revision 2 onward, after Devin round-2 note 1)
   says no host-to-guest key channel exists; that statement is wrong and must
   be corrected there.

## Why it matters

- Anyone with read access to Node B's disks or backups can read every user's
  documents and history. The privacy DEK protects only payloads flagged
  private, which projection batches are not.
- Restore, PSA and compliance all depend on the host copy of the tape; a
  restore design built on plaintext segments would inherit the gap.
- The off-host restic copy carries the same plaintext.

## Belief state

- [V] the code path above. [I] that the bulk of host `tape-seg` bytes are
  these plaintext batches (the window-sizing measurement found
  `projection_batch_recorded` payloads dominate host segments at 2.64 GB/day
  for the paper).
- [I] the design intent was "owner" = readable by the owner's computers, and
  host plaintext was an accepted interim; no decision record found yet.

## Remaining error and next safe probes

- Read one non-paper computer's segment on Node B (metadata only, no content
  printed) to confirm the bytes are JSON, not envelopes.
- Find whether any doc or definition accepted host plaintext for `owner`
  payloads, and when.
- The fix belongs in the restore and audit design (host copies sealed under
  the computer or owner key; the host stores ciphertext it cannot read
  without an escrow unwrap that is itself an audited event), and in the
  cutover, which re-seals existing segments and bases.
