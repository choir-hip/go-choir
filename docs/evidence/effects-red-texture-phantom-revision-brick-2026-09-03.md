# Problem Receipt: Phantom Texture Revision Bricks Boot Replay (2026-09-03)

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); any repair is red
- Status: root cause of the retained-computer outage
  (`effects-red-computer-unresolvable-after-refresh-2026-09-03.md`); documented
  before the fix commit
- Computer: `computer-03335285269bdba4f94377e56879f9e6`

## The Problem (two coupled defects)

**Defect 1 (Texture lifecycle): a mutation created with a revision that never
materializes.** At 2026-09-03T16:14:56Z the live guest created Texture agent
mutation `0744fc0c-eaa6-5e22-acd5-dcb9cb68fb93 / aa4fc186-ee42-4c25-823a-61bb506a0568`
(tape seq 138555) carrying `revision_id: f1511357-...`. No such revision exists
in `texture_revisions` — the revision was pre-allocated at creation and the
spawning turn never wrote it (phantom revision).

**Defect 2 (Texture lifecycle): sleep-after-non-revision-turn is unfinalizable
for a mutation that carries a revision.** At 16:16:21Z a non-revision Texture
turn triggered `SleepAgentMutationAfterTextureTurn`, which appended seq 138612
(`require_revision: false`, `expected_states: [pending]`, snapshot preserves
`f1511357`). Live `FinalizeBatch` failed the revision-presence guard
(`internal/store/project.go`: `*RequireRevision(false) != hasRevision(true)`)
— correctly fail-closed — so the batch stayed `prepared`, never `finalized`.

**Defect 3 (substrate): one CAS'd-but-unfinalizable event bricks every future
boot.** The platform CAS accepted seq 138612 (canonical head is now
`0dcb48e4...`, seq 138612). Every boot replay re-fetches it from the canonical
tape and dies in `finalizeProjection` with `Texture mutation revision presence
changed`. The guest supervisor crash-loops (new runtime PID every ~5s);
vmctl's 5-minute ready wait expires, kills the VM, marks it failed. All
product paths return "failed to resolve user autoputer". There is no forward
path: local repair is futile (replay re-fetches the canonical poison), and the
tape cannot be rewritten without breaking digest chains and receipts.

## Evidence

- Platform head: seq 138612, canonical `0dcb48e4...` (corpusd, Node B).
- Batch payload op 0: `texture_agent_mutation_recorded`, doc `0744fc0c...`,
  state `sleeping`, `revision_id: f1511357-...`, `require_revision: false`.
- Creation batch seq 138555: same doc/loop, `pending`, `revision_id:
  f1511357-...` (phantom — absent from `texture_revisions`).
- Guest journal: `computer event projection mismatch: Texture mutation
  revision presence changed` at `replay finalize sequence 138612`, one crash
  per ~5s, 18:22–18:29Z; vmctl `guest replay stalled ... (no sequence advance
  for 5m0s, seq=0)` then kill + mark-failed.
- Method: guest `data.img` mounted read-only (`noload`) on Node B, 9.8 GB
  texture Dolt DB copied to `/var/tmp/texture-copy` for offline SELECTs;
  payloads fetched from corpusd CAS with internal-caller owner attestation.
  No guest state mutated during diagnosis.

## Repair Direction (fix commit follows separately)

Replay-only tolerance in `projectTextureAgentMutation`: when rebuilding from
the canonical tape (`allowReplayTextureBootstrap`), accept a
revision-presence mismatch iff the op preserves the projected revision
byte-for-byte (introduces and removes nothing). Live finalization stays
strict. Precedent: `FinalizeReplayBatch` legacy-row compatibility. The Texture
lifecycle defects (phantom revision at creation; unfinalizable sleep path for
revision-carrying mutations) remain open — this removes the brick, not the
inconsistency.

## Residuals

- The projected row keeps a phantom revision reference after replay; a future
  live sleep of the same mutation still fails closed (by design) until the
  lifecycle defects are fixed.
- vmctl's 5-minute ready wait vs multi-minute replays on 11 GiB stores is a
  separate sharp edge (this outage did not need more time — replay dies in
  seconds on the guard — but a healthy 11 GiB replay may yet exceed it).
