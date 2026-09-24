# One event tape, recovery to any prior head

**Date:** 2026-08-16  
**Status:** design candidate. Not implemented. Not a checkpoint. Not Super start.  
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`  
**Owner correction:** merge the two event systems; the tape must reconstruct any prior computer state. No backwards compatibility. Forget unused legacy.  
**This file does not license Super, live mail, rematerialize, gate weakening, mode CAS, or `goal.complete`.**

---

## The defect

Choir already has a canonical event tape. Restore already replays that tape to a chosen head (`ReconstructThrough` / `ReconstructThroughTarget`). Checkpoint already binds that head plus a VM-local content witness.

The tape does not record the computer.

`computerevent.Reduce` updates `Head` only: sequence, canonical/desired/effective heads, pending transition, state commitments, credential epoch. `store.Finalize` writes `computer_event_index`, `computer_event_projection_heads`, and (via the same projection) `computer_effective_state`. Causal kinds (`trajectory_started`, `message_recorded`, `lifecycle_observed`, …) are admitted and then ignored.

Everything the owner actually uses is written beside the tape:

| Writer | Store | On the tape? |
|---|---|---|
| `internal/store/desktop_live.go` | `desktop_workspaces`, `desktop_sessions`, `desktop_app_instances`, `desktop_window_placements` | no |
| `internal/objectgraph/dolt_store.go` | `og_objects`, `og_edges` | no |
| `internal/store/texture.go` | `texture_*` plus OG | no |
| `Store.AppendEvent` | OG kind `choir.event` (legacy `events` table remains in DDL) | no |
| media / preferences / runs | their tables and/or OG | no |

There is a second “event stream”: product `types.EventKind` (`desktop.state.updated`, `loop.started`, `texture.document_revision.created`, …). `EmitProductEvent` persists that as `choir.event` objects and fans out on the in-process bus. Sequence numbers (`Seq`, `StreamSeq`) are allocated by scanning OG. That stream is not the CAS tape, is not what restore replays, and is not what checkpoint binds.

So today:

- Restore to head N rebuilds **heads**, not the desktop, not Texture, not runs.
- Live Dolt is a second, unaudited store.
- Checkpoint correctly 409s when those tables are nonempty (`EmptyUntilSupported`).

That is not an incomplete reducer list. It is two computers glued together.

---

## If we were starting from scratch

One computer. One append-only tape. One projector.

```
mutation API
    → pin payload artifact
    → CAS-append computerevent.Event
    → projector applies payload to VM-local tables in the same finalize
    → live bus notifies subscribers
```

Replay of events `1..N` into an empty workspace **is** the computer at head N. Checkpoint hashes that workspace. Restore is replay through a target head, then the same hash check.

There is no save-then-emit. There is no SQL write that is not a projection of a finalized event. Direct mutation of a behavior-bearing table is a bug.

### Two jobs, one reducer pipeline

Keep the existing CAS `Reduce` (head algebra, effect/materialize/rollback, genesis). Add a **projector** that `Finalize` already has a transaction for and currently does not use:

1. `Reduce(current, event, input) → next Head` (already exists; do not overload it with SQL).
2. `Project(tx, event, payload) → VM-local rows` (missing).

Replay (`ReconstructInto` / `ReconstructThrough`) already loops durable events through Reduce → Prepare → Finalize. Once Project lives inside Finalize, arbitrary-head restore is the same loop with a stop head. No second reconstruct path.

### What belongs on the tape

**On the tape** — anything restore-to-head-N must recreate:

- Desktop layout: app roster, focus, window geometry (the current `SaveDesktopStateForSession` snapshot).
- Object graph: puts, deletes, edges for VM-local kinds (`choir.run`, `choir.texture_revision`, `choir.agent`, …).
- Texture documents/revisions/decisions (if they remain tables, they are indexes of the same events).
- Run/trajectory/work-item/mailbox durable records.
- Media progress/recents, user preferences, content items, podcast subscriptions.
- Self-development operations/intents.
- The existing causal and effect events (promotion, materialize, restore requested, lifecycle, keys).

**Not on the tape** — runtime that is not the computer:

- Driver leases, `last_input_at`, `visibility_state`, “who is looking right now.”
- In-process bus delivery, SSE cursors, HTTP sessions.
- Guest credential envelopes (already a different capability; revocation epoch is already on the Head).
- Host/platform publication graph (`choir.publication`, routes, corpusd OG). That is another computer.

If a field is needed after reboot of *this* computer, it is on the tape. If it is only needed while a browser tab is open, it is not.

### Event shape

Do not invent a second envelope. Use `computerevent.Event`. New kinds are ordinary causal events whose **payload artifact** is the mutation. The envelope stays the CAS identity; the payload is the state delta or snapshot.

Sketch (names for review, not frozen schema):

| Kind | Payload | Projector |
|---|---|---|
| `desktop_state_recorded` | owner, desktop_id, windows, active, app instances, placements | upsert desktop projection; last-write-wins |
| `object_recorded` | object (kind, canonical_id, body, metadata, tombstone) | upsert `og_objects` |
| `object_edge_recorded` | edge | upsert `og_edges` |
| `texture_revision_recorded` | revision body + citations | OG + any texture index tables |
| existing causal kinds | already pinned | no VM-local rows (or fanout-only) |
| existing effect kinds | already bound | Head algebra only, plus any operation-row projection already implied by self-development |

Prefer **snapshots for high-churn last-write-wins state** (desktop) and **object records for identity-bearing graphs**. Replay of snapshots is fold-last; replay of objects is fold-by-id. Do not log per-pixel drags. `SaveDesktopStateForSession` is already a snapshot; it becomes one event per driver save.

Privacy: desktop and some OG bodies use the existing private payload pin path (`AppendNewPrivatePayload`). Restore decrypts with the computer’s keys the same way other private events already must.

### Tables: two classes, not three

| Class | Meaning |
|---|---|
| **event_projection** | Projector writes these. Nonempty is required to match the tape. Includes today’s head tables **and** desktop, OG, texture, runs, … |
| **retired_absent** | Not in current DDL. Must not appear. Drop writers and readers. |

`empty_until_supported` is a **staging label during implementation**, not a permanent ontology. A table is either projected or gone. While a vertical is not yet wired, it must stay empty (today’s 409). After it is wired, it moves to `event_projection`. There is no third life where nonempty SQL is “ok until later.”

### Product events

The live bus stays. The durable `choir.event` / `events` ledger goes away as an authority.

After Finalize, the runtime may publish a `types.EventRecord` to the bus for UI. Subscribers never read a second log to learn history. History is the tape (and whatever projected tables the UI already queries). `Seq` / `StreamSeq` on product records, if they survive at all, are derived from `computerevent.Event.Sequence`, not allocated by scanning OG.

`choir.event` as an object kind is unused legacy. Forget it.

### Legacy to forget (no compatibility shims)

- `desktop_state` table, `getLegacyDesktopState`, `defaultDesktopSessionID = "legacy"`.
- Dual desktop models (workspace JSON vs instances/placements as independent sources of truth). One snapshot payload, one projection.
- `events` table as a durable log; `AppendEvent` seq allocation by listing OG.
- Retired DDL already classified: `app_adoptions`, `app_change_packages`, `candidate_package_intakes`, `computer_source_lineages`, `computer_supervision_commands`.
- OwnerRecovery as promotion evidence (already refused; do not revive as a tape substitute).
- “Empty the desktop so checkpoint works.”
- A second reconstruct implementation.

`CREATE TABLE IF NOT EXISTS` stays. Removing a retired table from a live workspace is a cutover or a later DDL drop, not an in-place compatibility migration. New code must not read the old names.

### Volume and CAS

Every durable mutation is a platform CAS append. Desktop saves are user-scale, not pointer-move-scale. Object puts are already discrete. That is acceptable. If CAS latency is too high for a save, the fix is batching snapshots in the API, not a side store.

Do not compact the tape by dropping events. Compaction, if ever, is a new snapshot event plus an explicit decision that older payloads are still retrievable from artifacts. Recovery to an old head still requires those artifacts.

### Existing computer (sequence 26)

Historical events have no desktop/OG payloads. Replay of 1..26 will never grow those tables. Current nonempty rows are residue, not history.

No backwards compatibility means we do **not** pretend 1..26 recorded the desktop.

Honest options for *this* computer, after the projector exists:

1. **Import snapshot event(s)** — one (or a few) `desktop_state_recorded` / `object_recorded` events whose payload is the current residue. After they finalize, live equals replay and checkpoint can succeed. This is not emptying tables and not weakening the gate: the rows become event-derived.
2. **Workspace-replace then live from empty** — discard residue, bootstrap, then only tape writes. Lossy; already panel-contested.

This design prefers (1). It is the single-tape analogue of “make the record complete.” It is not OPTION_2 from the eligibility panel.

Heads before the import event still will not recreate that residue. That is the truth of an incomplete historical tape. Arbitrary recovery is guaranteed **from the first complete-tape event onward**, which is the best this computer can do without inventing history.

---

## What this is not

- Not a list of five desktop reducers bolted onto `EmptyUntilSupported`.
- Not “also append a product event when SQL succeeds.”
- Not a new event package beside `computerevent`.
- Not changing `Reduce` into a SQL function.
- Not putting driver leases on the tape.
- Not merging corpusd’s publication graph into the guest tape.
- Not permission to start Super or take a checkpoint before Project is live and this computer’s residue is imported or discarded.

---

## Implementation order (after panel)

1. Freeze kinds + payload schemas for desktop snapshot and object record. Tests: append → live tables; ReconstructInto empty workspace → same hashes; ReconstructThrough older head → earlier snapshot.
2. Move `SaveDesktopStateForSession` (driver path) to append+project. Delete SQL upsert as a public writer. Session presence (lease/visibility) stays off-tape.
3. Move VM-local `PutObject` / edges to append+project. Texture/runs follow the same object events; drop parallel `texture_*` writers or make them indexes of Project.
4. Replace `EmitProductEvent` persistence with bus fanout from Finalize. Stop writing `choir.event`.
5. Reclassify wired tables to `event_projection`. Leave unwired tables `empty_until_supported` until their vertical, then promote or retire.
6. Drop legacy readers (`getLegacyDesktopState`, `events` SQL path).
7. Import snapshot event(s) on the retained computer **or** owner-ratified cutover.
8. Then, and only then, eligible pre-A checkpoint. Super still waits on that fence.

ReducerVersion can stay `1` if Head algebra is unchanged. Projector completeness is a manifest class change, not a Head field.

---

## Feasibility risks (for the panel to attack)

1. **CAS on the desktop save path** — user-visible latency; failure mode if platform CAS is down (desktop must not SQL-fall-back).
2. **Private payload restore** — desktop/OG bodies must round-trip through existing private artifacts or they are a new privacy hole.
3. **Texture dual write** — `texture_*` tables vs `og_objects`; one must be the projection, the other an index or gone.
4. **Import event as fake history** — it records “state as of now,” not the true past. Accept that, or cut over.
5. **choir.event clients** — anything that lists OG events as the run log must switch to the tape or to projected run tables.
6. **Scope explosion** — wiring every OG kind in one slice vs desktop-first then objects. Desktop-first unblocks checkpoint only if `og_objects` is also empty or also imported; live probe had both nonempty. **Desktop and OG residue must move together** or eligibility stays false.
7. **Platform vs guest** — tape is platform-canonical; projection is guest Dolt. Projectors run in the guest Finalize. Platform must store payloads; guest must be able to fetch them on replay (already true for event artifacts).

---

## Standing constraints (unchanged)

- Effects remain OFF. Do not present the ModeReceipt.
- Genesis stays 409. `Armed=false`. No live mail.
- Do not weaken `EmptyUntilSupported` to admit nonempty unprojected tables.
- Do not SQL-empty desktop/OG.
- Do not rematerialize. Do not invent `choir computer create`.
- Do not use OwnerRecovery for promotion.
- Do not `goal.complete`.

---

## Sources

- Head-only reducer: `internal/computerevent/reducer.go`
- Replay loop: `internal/computerevent/appender.go` `reconstruct` / `ReconstructThrough`
- Finalize (heads only): `internal/store/computer_events.go`
- Desktop SQL: `internal/store/desktop_live.go` (`legacy` session id, `getLegacyDesktopState`)
- Product stream: `internal/agentcore/product_events.go`, `Store.AppendEvent` → `AppendEventOG`
- Eligibility: `internal/agentcore/replay_eligibility.go`
- Live 409: `docs/choir-effects-pre-a-checkpoint-eligibility-2026-08-16.md`

---

## Panel result (2026-08-16, divergent, Claude included)

Five of six panelists answered. Codex failed (`--autoputer` removed from current `codex exec`). Transcripts: `.agentic-consensus/unified-event-tape-20260816/` (gitignored).

| Agent | Lens | Usable verdict |
|---|---|---|
| Claude (Opus) | cas-latency-skeptic | **APPROVE_WITH_CONDITIONS** |
| Gemini 3.6 | legacy-forget | **APPROVE_WITH_CONDITIONS** |
| Cursor Grok 4.5 | projector-vs-reduce | **APPROVE_WITH_CONDITIONS** (medium) |
| GPT 5.6 Sol | privacy-restore | packages: REJECT (new epoch) / **APPROVE_WITH_CONDITIONS** (import boundary) / ESCALATE (key custody) |
| GPT 5.6 Terra | arbitrary-head-restore | **ESCALATE** until the recovery domain is chosen |
| Codex | single-tape-purist | failed to start |

**Direction stands: one `computerevent` tape.** Dual durable logs, gate weakening, and checkpoint-time serialization were offered as options and rejected as cheating the owner correction.

### Conditions that amend this candidate

These are now part of the design, not leftover review nits.

1. **Recovery domain — recorded `complete_from_head`.** New epoch is refused on this computer: `Reduce` rejects duplicate genesis, `choir computer create` is forbidden, rematerialize is forbidden. Keep the sequence-26 chain. Tape-recovery restore of eligible incomplete-tape checkpoints (empty `EmptyUntilSupported` tables) remains valid. Full projected computer restore (desktop/OG payloads) is admitted only at/after an explicit `complete_from_head`. Restore of a later complete-tape checkpoint to a pre-completeness sequence fails closed (`ErrIncompleteTapeRestore`). Literal time-travel through 1–26 is impossible; those payloads were never recorded. Import snapshot events are “state observed now,” not fabricated history.

2. **Payload resolver before SQL.** `Finalize(computerID, digest, receipt)` has no payload bytes. Replay has pin, not fetch. Fetch, hash-verify, and decrypt **before** the serializable Dolt transaction; pass a typed batch in. No network/decrypt inside `BeginTx`.

3. **Atomic projection batches.** `object_recorded` + `object_edge_recorded` as independent events can leave Texture/OG half-applied. Durable unit is a versioned batch (conditions, upserts, tombstones, edges) so every intermediate head is a valid computer.

4. **`desktop_sessions`.** That table is in the eligibility set and currently nonempty, and it also holds leases/`last_input_at`/`visibility_state` which this design called off-tape. Split presence out of the witnessed schema **or** project it. A nonempty unprojected sessions table is gate weakening.

5. **`choir.event` is live, not unused.** Run history, WebSocket catch-up (`StreamSeq`), acceptance, memory, spend, worker recovery. Bus fanout is not a replacement. Either typed tape events plus derived read models, or an explicitly non-computer trace authority. Do not delete the kind first.

6. **Desktop and OG residue move together.** Live 409 is both. Wiring one vertical leaves eligibility false. Import/reclassify is one fence.

7. **Projector failure is poison.** If Finalize/Project fails after platform CAS, today’s `ErrNeedsProjectionRepair` plus deterministic re-fail wedges the computer. Project must be total, with a repair/reprojection path that is not “retry the same crash.”

8. **Texture is a second Dolt workspace.** `s.db` Finalize cannot atomically commit `.texture`. Freeze OG-as-authority vs SQL-as-authority vs Texture-as-another-computer before wiring it.

9. **Projector version.** Do not keep `ReducerVersion=1` as if projection semantics were invisible. Bind projector/schema version on the mutation (or a tape-epoch manifest) so replay is not “whatever binary is running.”

10. **Notifications.** Idempotent Finalize cannot be the sole bus publish. Transactional outbox or tape-derived index; restore must not spam live clients.

### First slice (recovery domain recorded)

`complete_from_head` is wired. Payload resolver and projection-batch schema are frozen. SQL-only Project now applies atomic desktop+OG batches inside `FinalizeBatch`. `desktop_sessions` presence is in-memory, not Dolt. `EmptyUntilSupported` is unchanged. No live residue import.

Live writers are bound in autoputer: desktop driver saves and OG mutations append+project. Platform GET `/internal/computers/events/payload` is registered. Presence stays off Dolt.

Residue import is implemented (`Store.ImportResidueSnapshot`) and tested. It is not executed live. Autoputer does not auto-import.

Still unpaid:

- staging deploy of current main
- live `ImportResidueSnapshot` on the retained computer
- then reclassify wired tables to `event_projection`

Then eligible pre-A checkpoint. Super still waits.

### Unanimous forbids (still)

No Super. No ModeReceipt. No live mail. No rematerialize. No SQL-empty. No weakening `EmptyUntilSupported`. No `goal.complete`.

---

## Recovery domain decision (2026-08-16)

**Chosen: `complete_from_head`.** `new_epoch` is refused.

Reason: this is the same retained computer. A second `genesis_imported` is `ErrInvalidTransition`. Inventing a computer is forbidden. Rematerialize is forbidden. No-backwards-compatibility means do not pretend heads 1–26 recorded desktop/OG, not “throw away the paid tape.”

Contract, implemented in `internal/selfdevprotocol/tape_completeness.go`:

- Incomplete-tape checkpoints (`tape_completeness` empty) restore what the tape recorded. That is the choir-tape-recovery substrate.
- Complete-tape checkpoints must carry `tape_completeness=complete_from_head` and a SHA-256 `complete_from_head`.
- `AdmitRestoreSequence(target < completeFrom)` returns `ErrIncompleteTapeRestore` once completeness is declared.
- `new_epoch` on a checkpoint or restore is `ErrNewEpochRefused`.

This is not Super start, not an import event, and not a live checkpoint.
