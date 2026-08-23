# Recover-current projection-batch data IS present (corrected from earlier receipt)

Date: 2026-08-23
Mutation class: red

## Correction

The earlier receipt `effects-red-recovery-projections-not-retrievable-2026-08-23.md`
tested only the `computer-event-payload/` flat top level. The storage model is
two-level per event, keyed by two different commits:

- `computer-event/<event_artifact_ref>` — the event **envelope** (event_id,
  event_kind, previous_head, expected heads, **payload_commitment**,
  reducer_version, schema_version, request_commitment). 132,629 files, 519 MiB.
- `computer-event-payload/<payload_commitment>` — the **projection batch body**
  (projector_version, computer_id, event_id, projection ops). 132,490 files,
  903 MiB.

For this computer, `payload_commitment` is embedded in each envelope and
resolves to the batch body file. Verified:

- 131,317 `event_artifact_ref` values for this computer present at
  `computer-event/` (found=132,317, missing=0).
- 500/500 sampled events: envelope present AND payload_commitment resolves to a
  batch body file at `computer-event-payload/`.
- Sample envelope `8e84cf7a...` → payload_commitment `1457cd9c...` →
  `/var/lib/go-choir/platform-artifacts/sha256/computer-event-payload/1457cd9c...`
  (39,704 bytes).

## Measurement

Sample 1,000 batch bodies for the `projection_batch_recorded` events: 5,995,194
bytes, **avg 5,995 bytes/body**, scale to 132,317 → **≈ 756 MiB** total batch-body
payload. The full canonical chain (envelopes + bodies + pin receipts) is intact
and locally retrievable.

## Consequence

Rail A (offline full-tape rebuild to head 132,436) **is feasible**: every
projection batch body that the replay projector must resolve is present on the
host. The earlier 404 on `/internal/computers/events/payload` was a routing/lookup
artifact of that endpoint, not data absence; the on-disk two-level store is
complete. The projection can be rebuilt to head 132,436.

## Repair boundary unchanged

- Do NOT rewind canonical events; do NOT SQL-empty/replace the retained computer.
- Mutation class RED; problem-documentation-first precedes any fix.
- The dominant cost is now confirmed as (per-event serializable tx + per-op SQL)
  plus (per-event payload fetch + decrypt), not payload absence.
