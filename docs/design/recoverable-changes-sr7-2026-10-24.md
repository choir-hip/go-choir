# Recoverable changes: keep-and-skip replay, the restore pin, and the cross-version gate (SR7 design, 2026-10-24)

Status: design, docs only, no code. Revision 2 answers the tier 2 review
(bunch 222: Devin, findings 1-9; DeepSeek v4.1 flash, findings 1-21). The
review answers are at the end. Station SR7 of
`docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md`.
Problem: `docs/problems/an-older-release-cannot-read-a-newer-tape-2026-10-24.md`.
Class of this doc: green. Class of what it designs: red (tape/evidence, boot
path, restore, store schema), so it is reviewed tier 2, like the change.

Owner invariant (2026-10-24): any release can boot on any tape written by
releases inside its support window (one week, ledger 18:5x), in both
directions. An older binary keeps and skips what it does not know; it never
fails boot and never drops it. A new kind ships in two releases: the reader
first, the writer later. Store schema changes are additive, or done in two
phases. A cross-version replay gate in CI enforces all of this, and it must
not lengthen CI wall clock.

Labels:

- **[code]**: read in code on origin/main 5e11072c5; nothing run.
- **[H]**: hypothesis.
- **[run]**: observed on a CI run.

## 0. What the code does today (the problem doc's 23:5x findings, verified)

| Claim | Verdict | Where |
|---|---|---|
| Boot replay is `Reconstruct` with no target, and any error ends boot | confirmed [code] | `internal/autoputer/run.go:764`, `:807` |
| An unknown event kind fails replay | confirmed [code] | `Event.Validate` (`internal/computerevent/event.go:157-159`), called by `Reduce` (`reducer.go:61`) from the replay apply (`appender.go:902`) |
| An unknown op in a known batch fails replay | confirmed [code] | `ProjectionBatch.Validate` (`projection_batch.go:202-204`) via `DecodeProjectionBatch` (`:226`, called at `appender.go:1136`); the `projectOp` default case (`internal/store/project.go:105-106`) |
| An unknown envelope field fails replay | confirmed; the mechanism is sharper than the problem doc said [code] | The digest is the canonical JSON of the Go struct (`event.go:119-138`). corpusd checks the raw-bytes digest (`internal/platform/event_replay.go:74`), decodes into its struct (`:77`) and serves the struct. The guest recomputes the struct digest (`reducer.go:64`), so `sameHead` fails (`appender.go:906`). `Store.Prepare` recomputes it as well and stores *struct* bytes (`internal/store/computer_events.go:37-43`). On writes, `PinEvent` requires a struct round trip (`event_artifacts.go:34-41`) and admission refuses unknown fields (`decode.go:152`) |
| An unknown op-body field is dropped from the projection but kept on the tape | confirmed [code] | typed bodies (`projection_batch.go:84-170`) are decoded with `json.Unmarshal` |
| **New:** a receipt with one extra kind field fails an older reader | [code] | `RequireKindFields` checks an exact count (`receipt.go:161-164`). It names 12 fields (`event_receipt.go:37-41`). The receipt does not sign `ReducerVersion` or the credential epoch; the page server rederives them (`event_replay.go:96-103`) |
| **New:** a batch-level field is safe for an older reader | [code] | the batch decode ignores unknown fields (`projection_batch.go:223`); the payload digest is taken over raw bytes (`payload_resolver.go:77`) |
| **New:** corpusd is the head authority and signs the next head | [code] | CAS `Reduce` (`internal/platform/computer_events.go:237`); signed fields (`:347-361`); owner-act admission exists for `effect_accepted` only (`:247-251`) |
| **New:** any guest code that can append can self-declare a restore's authority | [code] | `AuthorityRef` is a free string (`event.go:167`); the restore sets it from guest code (`internal/agentcore/rematerialize.go:415`) |
| Restore re-converges at the next boot | **confirmed in code (static), not run** | §3.1 |
| **New:** between a restore and its restart every append is refused | [code], [H] until probed | `appendLocked` requires local head = platform head (`appender.go:741-743`) |
| **New:** a restore restart resumes work; a restore that does not change the binary schedules no restart | [code] | the marker is reason-agnostic (`internal/agentcore/planned_restart.go:200-203`); the restart is gated on `ExecChanged` (`rematerialize.go:328`, `:372`) |
| **New:** a repeat restore to the same checkpoint is deduplicated | [code] | idempotency key `restore-intent-<checkpoint digest>` (`rematerialize.go:399`) |
| **New:** a new table or column makes older checkpoints unrestorable, and older readers ineligible | [code] | the airworthiness manifest denies unknown tables (`internal/agentcore/replay_eligibility.go:51-102`, `:185-189`); the witness covers every table and column except `optionalTables`, which holds only `choir_events` (`internal/computerversion/dolt_state_extractor.go:244-256`) |
| **New:** a pending transition blocks checkpoints | [code] | `internal/platform/checkpoints.go:59` |
| **New:** catch-up cannot finalize behind the head | [code] | `finalizeBatch` requires the next sequence and moves the head (`internal/store/computer_events.go:192`) |
| Store: epoch 3, a raise-only floor, no migration framework | confirmed [code] | `internal/storeschema/schema.go:54`; `floor.go:111-121`, checked at `store.go:938`; the updater refuses an older-epoch release (`internal/updater/updater.go:459-461`) |
| **New:** destructive DDL runs at Open | [code] | DROP of retired tables (`store.go:1102`, `:1113`; `texture.go:369`, `:377`); primary-key reshapes (`store.go:3597`; `texture.go:479-481`, `:2446-2448`) |
| **New:** the store-floor CI guard is not wired in | [code] | `scripts/check-store-schema-floor.sh` exists; `ci.yml` never calls it |
| CI baseline | [run] | Run 40138087758 (CI0's baseline, `docs/definitions/ci-test-cooptimization-2026-10-24.md`): push to deploy 19.9 min. Plan ended 22:20:49; prebuild started 22:21:51 (+62 s); the outlier non-runtime shard 1 ran 17.3 min, the next 9.1 min. `go-vet-build` ran 90 s here and 101 s in 40132893674. The account runs at most 20 jobs at once, and 24-25 are ready when plan ends (`docs/problems/ci-critical-path-race-shards-2026-10-22.md` finding 3) |

## 1. Failure modes (written before the design)

| ID | Failure mode | Answered in |
|---|---|---|
| F1 | A skipped event carried authority: a skipped restore, approval or owner act, possibly one that does not move the head. The owner then approves twice, or applies twice | §2.1, §2.2, §2.6 |
| F2 | A skip leaves the projection wrong, not just missing data: a later known op depends on rows that were skipped | §2.1 (structural table check), §4.3 (oracle) |
| F3 | A skip hides a corrupt event | §2.2 |
| F4 | An unknown envelope field breaks the digest | §2.3 |
| F5 | A receipt field is added, and every older reader refuses every receipt that carries it | §2.2 |
| F6 | Skipped events are never projected once the reader learns them | §2.5 |
| F7 | Replay differs across binaries: the restore rebuild projects what the target skipped | §3.4 |
| F8 | The window boundary: a far-older release quietly projects a thin state | §2.6 |
| F9 | Re-convergence: a restore lasts only until the next boot | §3 |
| F10 | Pins are applied to history written under re-convergence: to old restores, or after a rollback below S5 | §3.2, §6 S5 rollback |
| F11 | Work open at the target T is resumed, duplicating effects that already happened inside the window | §3.5 |
| F12 | Idempotency collides across the window; a repeat restore is deduplicated away | §3.5 |
| F13 | Monotone facts (key rotation and revocation) are undone | §3.3 |
| F14 | A projection base straddles a pin | §3.4 |
| F15 | A field without `omitempty` changes every historic digest; a version constant bumps inside the window | §4.4 |
| F16 | The gate lengthens CI: a held slot during the 20-slot crunch, a per-SHA cache evicting the Go caches, or a fetch of history | §4.1 |
| F17 | The gate tests one step only, or one direction only | §4.2, §4.4 |
| F18 | An older Open destroys a newer store; a new table or column breaks older checkpoints or eligibility | §5 |
| F19 | A guest self-declares a pin | §3.2 |
| F20 | A restore fails after its pin was written, so the tape and the store disagree | §3.2 |
| F21 | The pin index is missing after a corpusd downgrade, so readers refuse the tape forever | §3.2 |
| F22 | A pinned restore leaves a pending transition, so the computer can no longer checkpoint; the head names restored-away code | §3.6, D7 |
| F23 | The window held external effects or adoptions (mail sent, paper published): they are not undone, and the lineage salt turns a dedup into a duplicate | §3.6 |
| F24 | A rollback turns a reader off after its data is on a tape | §6 (rollback = writer off) |
| F25 | A guest flips a switch | §6 (switch provenance) |

## 2. Keep-and-skip

### 2.1 The reader manifest and declarations

`internal/computerevent/reader-manifest.json` is checked in and append-only.
It lists what this release can *read*:

- event kinds;
- projection op kinds, each with the tables it writes;
- envelope field names;
- typed op-body fields, per op;
- the version constants;
- store tables (§5).

Each entry records a `since` date. The static check (§4.4) requires a new
entry's date to be within one day of the run, and never lets an existing date
change. A Go test, run in the existing shards, keeps the file equal to the
registries and to the reflected body structs.

**Every new event kind or op kind declares `skip` explicitly; there is no
default.**

- `safe` needs a one-line independence reason. Its op kinds' tables must be
  new: tables no known op writes and no authority decision (approve, apply,
  publish, send, restore) reads.
- `refuse` is required for anything that carries authority, including authority
  that does not move the head, or that changes the meaning of known state.
- Review expects `safe` for data kinds. That is the owner's keep-and-skip
  policy. The reader enforces the structural part itself (§2.2), so a wrong
  promise fails closed instead of skipping silently.

Today one map is both the reader set and the writer allowlist
(`validEventKinds`, `projectionOpKinds`). S1 splits them; the writer set must
be a subset of the reader set.

Retired kinds stay in the manifest forever. History is decoded under frozen
rules (`decode.go:9-10`).

### 2.2 Per level

**Never skipped:**

- the raw digest of every processed event;
- the corpusd signature and the receipt binding;
- sequence and previous-head continuity.

Payload digests are verified for every payload the reader fetches (projected
events, `payload_resolver.go:77`). The reader does not fetch payloads of
excluded or skipped-kind events. Their pin receipts were checked by corpusd at
append (`ValidateEventPins`, `internal/platform/computer_events.go:218`), and
the signed event commits to them.

| Level | Rule | Declared by | Recorded |
|---|---|---|---|
| **Unknown event kind** | Skip iff all three hold. (a) The receipt verifies. (b) In the signed next head, every field other than sequence and canonical head equals the current head; this covers today's 7 head fields and any future field. (c) The receipt carries `compat {skippable: true, writer_base_commit, writer_deployed_at}`. If (b) fails, the event is a transition: refuse. If (c) is missing, refuse. An unknown kind never runs `Reduce`, because `Validate` rejects it (`event.go:157`). The head advances on the receipt-verified path: next head = the signed head, with sequence, canonical head and reducer version derived as the page server derives them | corpusd, in the receipt (`PinEvent` validates every kind, `event_artifacts.go:34-41`). Only events of kinds new after S4a carry the field, so no older reader sees a new failure (F5) | a skip-ledger row |
| **Unknown op in a known batch** | Apply the known ops. Skip an unknown op iff (i) the batch's `compat.skippable_ops` lists it, and (ii) every table in its declared `tables` is absent from this reader's manifest (structural independence, checked by the reader). Otherwise refuse | the guest writer: a batch-level `compat {skippable_ops: [{kind, tables}], writer_base_commit, writer_deployed_at}`, taken from its manifest. Older readers ignore the field (§0) | a skip-ledger row per (sequence, op index) |
| **Unknown envelope field** | Ignore it. After S3, the tape and the local index keep the raw bytes. A field that changes the meaning of an event is not allowed; that is a new `refuse` kind | static check: `omitempty`, listed in the base manifest, and S3 deployed for at least the window | nothing |
| **Unknown receipt kind field** | Accept extra signed fields; `RequireKindFields` becomes "required ⊆ present" (S4a) | corpusd | nothing |
| **Unknown op-body field** | Dropped from the projection and kept on the tape. An older writer's upsert loses it (residual R2). Body fields follow the two-release rule | static check | nothing |
| **Unknown version constant** | Refuse, as today (`event.go:141`, `:170`; `projection_batch.go:183-187`). A version is never bumped inside a window | static check | typed refusal |

A writer cannot stamp an op it does not itself know. The batch dry run
(`internal/store/computer_events.go:97`) validates every batch against the
writer's own reader, so a garbage op is never declared skippable (F3).

### 2.3 Raw digest (S3, the prerequisite for any envelope growth)

- **corpusd.** `EventsPage` adds `request.event_raw`, the pinned bytes it
  already reads and digests (`event_replay.go:70-76`). The field is additive:
  `DecodeHistoricDurableEvents` ignores unknown fields (`decode.go:134-141`).
- **The guest.** It verifies sha256(`event_raw`) = `event_digest` = the
  receipt's `event_digest`, decodes the known fields from the raw bytes, and
  calls `ReduceVerified(current, event, digest)`.
- **The store.** `Store.Prepare` checks and stores the raw bytes.
- **Live appends** are unchanged.

### 2.4 The skip ledger and its surface

The ledger is one new table, `computer_event_skips`. It is tape-derived and
holds rows only for:

- an event skipped as an unknown kind;
- an op skipped as unknown;
- a pin applied (one row per pin, §3.2).

On a computer with no skips and no pins it is empty.

- **Restore compatibility (F18).** It is registered as
  `ReplayEventProjectionOptional` in the airworthiness manifest and in
  `optionalTables`. Both registrations ship in a release *before* any release
  creates the table (two-release, §5). While empty it changes no witness, so
  older checkpoints stay restorable.
- **No new column on `computer_event_index`.** A column would change that
  table's schema witness for every older checkpoint (verified:
  `dolt_state_extractor.go:244-256`).
- **The surface.** `/health` and the runtime observability endpoint show
  `tape_compat {skipped: [{kind, count, first_seq, last_seq, needs}], pins:
  [{restore_seq, target_seq, excluded}]}`. The host's release catalog turns
  `needs` into "needs release ≥ X".

### 2.5 Catch-up after a forward update (F6)

1. At boot, before the tail, the binary reads the ledger rows whose kind it now
   knows.
2. It applies each one on a **head-neutral** path: a transaction that runs the
   ops (all of an unknown-kind event's now-known finalize effects, or only the
   ops skipped before) and deletes the ledger row. It does not touch the head
   or the index status. `finalizeBatch` cannot do this (`computer_events.go:192`).
3. Rows are applied in sequence order. This is sound because `safe` ops write
   only tables that no older known op writes (§2.2 (ii)).
4. Any error falls back to a staged full rebuild, never to a partial store.

### 2.6 Refusal, the window boundary, never-skip

- **The refusal.** The typed boot refusal on `/health`
  (`internal/autoputer/boot_refusal.go:65-73`) gains the kind
  `tape_needs_newer_reader`, naming the kind, the sequence and the writer
  release.
- **The window boundary (F8).** A reader refuses to *project* content whose
  stamp `writer_deployed_at` is more than 7 days after the reader release's own
  `deployed_at`.
  - Both times come from the host's release record (deploy receipts),
    delivered host-set to the guest, never from a commit time (D2).
  - Excluded events are not projected, so they never trigger this refusal.
- **Never skippable:**
  - `genesis_imported`;
  - every head-moving kind (rule (b));
  - key and security kinds and any new monotone kind (`refuse`);
  - any op an authority decision reads.
  - Pinned restores are known kinds; a release that cannot honour them is
    handled by the restore preflight (§3.6).

## 3. The restore pin

### 3.1 Re-convergence is confirmed in code

Today's restore:

1. `restoreComputer` appends a causal `restore_requested` R, with
   `DecisionRef` = T (`rematerialize.go:363`, `:391-419`).
2. `RematerializeFromTape` rebuilds a staged store through T (`:165`), flips it
   in and reopens it (`:244-286`).
3. A restart is scheduled only when the binary changes (`:372`).

At the next boot:

- The marker is consumed (`run.go:536`), but replay is `Reconstruct` with no
  target (`run.go:764`). It runs from T to the platform head, applying every
  event in (T, R] with the restored binary.
- Replay must end with local = platform (`appender.go:1045`). The boot base
  planner also targets the platform head (`internal/autoputer/projection_base.go:121-125`).

So the restore is undone at boot, or the boot crashes on an unknown event in
(T, R]. This matches the 03:39 drill.

Two further points:

- **The model cannot hold a pin.** It requires the local projection to be a
  prefix replay of the whole tape (`appender.go:741-743`, `:1045`). A pin
  therefore needs the replay change below; a boot flag would only fail boot.
- **Before the restart [H].** The flipped store is at T while corpusd is at R,
  so every append is refused until a restart.

S5 step 0 probes both points before any code (§6).

### 3.2 The pin: an owner act admitted by corpusd, written before the flip, with explicit failure

**What a pin is.** A pin is a `restore_requested` with `AuthorityRef =
platform-control:restore-pinned` (D6) **and** a row in corpusd's
`computer_restore_pins`. corpusd writes that row in the same CAS transaction
that admits the event (F19, F21).

- **Owner-act admission.** The CAS admits the pinned value only together with
  an owner-act receipt, the pattern `admitEffectAccepted` uses
  (`computer_events.go:247-251`). A guest cannot self-declare a pin.
- **A guest-written pinned value without a row is not a pin.** A pre-S5 corpusd
  admits the string without a row. Readers then treat the event as an unpinned
  restore, which is deterministic.
- **Capability gate.** The guest writes the pinned value only after corpusd
  reports the capability `restore_pins`. Otherwise it writes an unpinned
  restore, with today's semantics and the visible warning (D3).
- **The index is corpusd's record of admission.** Its rows are never written by
  a downgraded corpusd, and a re-upgraded corpusd keeps the table. Replay
  fetches the index once per reconstruct.
  - At each pinned R, the event and its index row must agree. An index row with
    no matching event refuses.
  - An unreachable index refuses, which adds no fate-sharing: replay already
    reads its pages from corpusd.

**Old restores keep their meaning (F10).** Historic `restore_requested` events
carry the old value and no row, so they are not pins.

**The order of a pinned restore (F20, H2):**

1. Preflight (§3.6). The restore is refused here, before any mutation.
2. Append the pin P on the live store. The live head equals the platform head.
   The idempotency key is minted per owner act, not per checkpoint (§3.5).
3. Build the staged store through `effective(P)`: replay through T with the
   target's manifest (§3.4), check the witness at T, then process (T, P] as
   excluded. The staged head equals P, and the ledger holds P's row.
4. Flip, reopen, rebind. Local head = platform head = P, so appends work at
   once.
5. Write the planned marker (reason `self_development_restore`) and restart
   (§3.5).

**If step 3 or 4 fails:**

- The runtime appends a cancel: `restore_requested`, `AuthorityRef =
  platform-control:restore-cancelled`, `DecisionRef` = P's digest. A cancelled
  pin is not a pin, and the live store is correct for that history.
- If the process dies before the cancel, the next boot finds pin P at or below
  its local head with no ledger row for P. The store is then known not to honour
  the tape, so boot runs a staged rebuild to `effective(head)`. The owner's
  restore completes; if the rebuild fails, the boot refusal is typed.

**Nothing to consume or clear.** A pin is a range of history. A later owner act
is a new event after P. The planned-restart marker keeps its single job:
resume versus close (§3.5).

### 3.3 Effective history: P(tape, manifest, pins)

```text
effective(x) = let Rj = the last pin (cancelled pins excluded) with seq(Rj) <= x
               if none: [1..x]
               else: effective(Tj) ++ (Rj..x]
```

This is a pure function, unit-tested. Nested restores compose, and so does a
restore to a checkpoint that lies inside an earlier window.

**Every event is processed:**

- its digest, signature and continuity are checked;
- the head advances, by `Reduce` for known kinds and by the receipt-verified
  path for unknown kinds (§2.2);
- an index row is written.

Inside an excluded window, an unknown kind uses the receipt path even when it
is head-moving. Its effects are not interpreted, so this is safe. S4a is
therefore a prerequisite for any pin whose window can contain a new kind.

**Projected** means inside `effective(head)`, or a monotone kind
(`key_rotated`, `key_revoked`; F13). The head reducer always runs, so the
credential epoch stays exact.

**An excluded `projection_batch_recorded` event** uses a head-only finalize
(head and status advance, the batch is not applied). Today `finalizeBatch`
demands the batch for that kind (`internal/store/computer_events.go:164-165`).

**Index readers use effective history.** That includes the recovery query
(`internal/store/computer_event_recovery.go:16`) and `EventByIdempotency`
(`computer_events.go:338`). S5 inventories all seven readers (`:50`, `:299`,
`:338`, `:357`, `:435`, `:471`, `:493`).

**One implementation, in `reconstruct`** (`appender.go:869`). Every replay
uses it: boot, `ReconstructThrough`, replay completeness
(`replay_completeness.go:318-320`), and the projection-base rebuilder
(`internal/projectionbase/rebuilder.go:364`).

### 3.4 Determinism, bases, forks (F7, F14)

**Rebuild with the target's manifest.** The restore rebuild uses `current ∩
target release's manifest`. The target manifest comes from the release record,
stamped by S8a, which therefore precedes S5. The witness check stays the
verifier and refuses loudly (`rematerialize.go:197`).

**Base selection.** A base at S may seed a rebuild to T only if `effective(S)
== effective(T) ∩ [1..S]`; otherwise use an earlier base or genesis. This
applies to `planRestoreTarget` (`internal/agentcore/restore_base.go:87`) and
the boot planner.

**Forks.** A forked computer has its own ComputerID and its own chain
(`docs/computer-ontology.md:409-414`).

- Built from a checkpoint's content, it inherits `effective(H)` as content.
  Its chain carries no pins, and its lineage records the parent checkpoint.
- Built by replaying the parent's tape to H, it uses the same `reconstruct`,
  so it applies the parent's pins.

Either way the fork's content equals the parent checkpoint's witness.

### 3.5 Work, restarts, idempotency (F11, F12)

**Every pinned restore restarts.** Without a restart, the process would keep
running window-era activations in memory, whether or not the binary changed.

**The restore marker closes work instead of resuming it.**

- The marker reason `self_development_restore` makes boot close every open
  obligation in the restored projection: runs, wakes, controls, directives,
  assignments, work items. The visible fate is "superseded by a restore to
  `<checkpoint>`".
- This is the crash rule of "Restarts End Work Or Resume It" applied to a
  planned act that is not an update. Standing intents (Autopaper schedules)
  continue.
- Today boot does not read the reason (`planned_restart.go:200-203`).

**The restart does not wait for running runs**, because the restore supersedes
them. The CLAUDE.md rule still holds: never mid self-development apply and
never with a live capsule. Those cases refuse the restore in preflight instead
of deferring it (`self_development_restore_restart.go:106-115` defers today).

**Idempotency keys:**

- **Restore events.** The key is minted per owner act. A second restore to the
  same checkpoint writes a second pin (`rematerialize.go:399` deduplicates it
  today; Devin 4).
- **Other runtime-minted keys (I2c).** They include the lineage: the digest of
  the latest pin in effective history, or ZeroHead. The lineage is empty until
  the first pin, so existing keys do not change.
- **External effects.** A retried act with external effects must deduplicate on
  the external system's identity (message id, publication id), not on the tape
  key. Otherwise the lineage salt duplicates the effect (F23). The preflight
  (§3.6) is the guard until that holds.

### 3.6 Restore preflight

The restore is refused, before any mutation, when:

- the target release's manifest lacks a monotone kind present in (T, R];
- the platform head has a pending transition (`checkpoints.go:59` would block
  every later checkpoint). The owner settles the transition first;
- the computer is `critical_protected` or `public_platform`. Pins are off there
  by default until S5 is proved on test computers and R5 lands; an unpinned
  restore stays available (DeepSeek 18);
- (T, R] holds external effects (mail send, publication, Autopaper adoption)
  and the owner has not acknowledged them in the restore request. The list of
  event and op kinds that count as external effects is an S5 inventory [H];
- a self-development apply is mid-transaction, or a capsule is live.

When the target release cannot honour pins (it predates S5), the restore is
written unpinned with the warning (D3); it is not refused.

**The head still names restored-away code (R1, D7).** A pin rewinds the
projection and the release, but not the reducer's effective state.

- Default: the pinned restore also writes the existing rollback pair
  (`rollback_requested` then `rollback_applied`, both known kinds,
  `reducer.go:110-126`), targeting T's effective state commitment, whenever T's
  effective head differs from P's.
- The self-development rollback path already writes `rollback_requested`
  (`internal/agentcore/api_self_development.go:841-875`). Whether its admission
  accepts a checkpoint target is unread [H].
- S5 does not ship until this is settled.

## 4. The cross-version gate

### 4.1 Placement, judged by run wall clock

The measure is the run as a whole, not one job's end:

- push to deploy complete;
- pull request created to `check` complete;
- the last shard's end;
- the prebuild's start, which is the job that waits for a slot.

Each is the median of the first 5 Go runs after landing, against baseline
40138087758 (19.9 min; prebuild at +62 s; slowest shard 17.3 min).

**Placement: a new job, `tape-compat`, with `needs: [plan, go-vet-build]`.**
It feeds `check`. The gate's steps are not added to `go-vet-build`.

- `go-vet-build` (90-101 s) is one of the jobs whose slot frees during the
  crunch. Lengthening it could push back the prebuild, which waits for exactly
  such a slot (DeepSeek 8).
- The new job becomes ready around +100 s, after the prebuild (+12 s) is
  already queued, so it waits behind the prebuild for the next slot [H: GitHub
  queues in roughly FIFO order; the 5-run measure decides].
- It restores the `vet-build` Go cache lane read-only and builds two small
  harnesses, the candidate's and the base's (from a `git worktree` at the base
  SHA). There is no new cache entry and no per-SHA binary cache: the
  repository cache is at 10.2 GB against its 10 GB limit.
- Its budget is ≤ 6 min, with a 7-minute step timeout [H: 2-4 min warm].
  Today's slowest shard is 17.3 min, and 9.1 min once CI0 fixes the outlier.
- If the 5-run measure shows any of the four metrics later than baseline, the
  fallback placement is the same steps appended to non-runtime shard 6 or 7
  (2.1-2.8 min jobs that end after the crunch), measured again the same way.

**If CI0's shape lands** (`docs/design/ci-prove-once-2026-10-24.md` on
`lane/ci0-coopt`):

- **Prove once.** A main push that reuses a green pull-request run skips
  `go-vet-build` and, through `needs`, `tape-compat`. The gate's verdict then
  comes from the pull-request run. CI0's rule 4 job list must add `tape-compat`
  as a required success, and `check`'s reuse exception must cover it. This is a
  one-line addition for CI0; FP0 carries it.
- **Base drift.** The base read on the pull request is the deployed commit at
  that time. The base at landing can only be newer, and the reader manifest
  only grows. The exception is a deploy rollback between the two, and the push
  run recomputes the base whenever it does not reuse.
- **The outlier fix.** With shard 1 fixed, the critical path is about 9 min and
  the gate's 6-minute budget still sits inside it.

### 4.2 Which reader, which tapes, both directions

- **The base** is the deployed commit, picked by `deploy-base-select` on pull
  requests as well (D5). `plan` gains an output `reader_base`, and the commit
  is fetched with depth 1 (no history).
- **Bootstrap.** Until the base contains `cmd/tapecompat`, the gate passes with
  "base predates the harness". It fails closed once the harness commit is an
  ancestor of the base. The harness records its own `since` date in the
  manifest, so the check needs no history.

**Candidate to base (older reads newer).** The candidate writes the fixture
with `tapecompat write-fixture`, through the real append path:

- the appender with the platform CAS and `EventArtifactService`, over a temp
  platform store with a fixture key, as `internal/platform/event_replay_test.go`
  does;
- emitting the corpusd wire format: `DurableEvent` pages (with `event_raw`
  after S3) and payload blobs.

It must cover every kind and op in the candidate's writer set (a unit test
fails on a missing generator). It adds one sub-fixture per new `refuse` kind,
and a pinned restore once S5 lands.

**Base to candidate (newer reads older), DeepSeek 21.** The base harness writes
its own fixture, and the candidate replays it.

### 4.3 What it asserts

1. **It boots.** Each reader reaches the other's fixture head; local head =
   platform head.
2. **It skips and preserves only what it may.** Every unknown is in the ledger
   and is `safe` with new tables. The processed count equals the event count,
   and the index digests equal the raw digests.
3. **It refuses what it must.** Every `refuse` sub-fixture ends in the typed
   refusal.
4. **The projection oracle (F2).** Per-table digests, base against candidate,
   are equal on the tables both readers know.
5. **The store round trip (§5).**
6. **The witness.** The base harness's witness of its own fixture is unchanged
   when the candidate's Open has touched the store. That is the
   old-checkpoint-restorable check (F18).

### 4.4 The static check (seconds, first step of `tape-compat`)

`scripts/check-tape-compat.sh --base $reader_base` reads the base manifest with
`git show` at a depth-1 fetch, and fails closed when it cannot read it. It
fails when:

- **the two-release rule** is broken: the candidate's writer set (event kinds,
  op kinds with tables, envelope fields, body fields) is not a subset of the
  base's reader set;
- **the generic skip reader is too young** (DeepSeek 5): a new op kind is
  written before S2 has been deployed for 7 days, a new event kind before S4a
  has been deployed for 7 days and before S4b, or a new envelope field before S3
  has been deployed for 7 days. Dates come from the manifest's `since`
  entries;
- **a `refuse` kind's reader is too young:** the candidate writes one whose
  base reader entry is less than 7 days old;
- **the manifest is bad:** it lost an entry, an entry has no `skip`, a `since`
  date changed, or a new `since` date is not within one day of the run;
- **a version constant is bumped** (F15, Devin 7);
- **an envelope field lacks `omitempty`** (F15);
- **a store table is new** but the base's airworthiness manifest or
  `optionalTables` does not already name it (§5).

The override is a `Tape-Compat-Override: <reason>` trailer. It is report-only
for one week, then enforcing.

## 5. Store schema: additive, two-phase, restore-compatible

**The static check, `scripts/check-store-additive.sh`** (in `tape-compat`),
scans the lines a diff adds under `internal/store/**`. It fails on:

- `DROP COLUMN`, `MODIFY`, `CHANGE COLUMN` or `RENAME`;
- primary-key reshapes;
- `NOT NULL` without a default;
- a `DROP TABLE` of a table not listed with a date at least 7 days old in the
  base's `retiringTables`;
- a new column on a table the witness observes (every table except the optional
  ones);
- a new table not already registered as optional (below).

**New tables are two-phase as well** (Devin 1, DeepSeek 7):

1. Release N registers the table name as `ReplayEventProjectionOptional` in the
   airworthiness manifest (`replay_eligibility.go`, bumping
   `replayEligibilityManifestVersion` at `:19`) and in `optionalTables`.
2. Release N+1 creates and writes it.

A table that holds no rows on a computer stays out of every witness, so older
checkpoints stay restorable, and older readers' eligibility holds.

**New columns are not added to observed tables.** Data that needs a column
gets a new optional table.

**The dynamic round trip** (gate step 5):

1. The candidate harness opens a fresh store and replays.
2. The base harness opens the same store. The schema dump must show no drop and
   no reshape.
3. The candidate reopens it.

This decides residual R7, the primary-key reshapes at Open.

**Wire the existing floor guard.** `scripts/check-store-schema-floor.sh` runs
in the same job.

**The epoch's meaning.** `storeschema.Version` moves only for the second phase
of a two-phase change, after the window. The updater's refusal of an older
epoch (`updater.go:459-461`) stays as the fence for anything outside the
window.

## 6. Slices

**What holds for every slice:**

- **Switches are off by default and host-set** (F25). The host writes them into
  the guest's boot environment; the guest reads them once at boot and cannot
  set them. The probe endpoint's test-class gate is checked on the host side.
- **Order of turning on:** land with both switches off; turn the reader on, on
  test computers, then fleet-wide; turn the writer on only after that.
- **Rollback is: writer off, reader on** (DeepSeek 14). Once data is on any
  tape, turning its reader off would break boot again.
- **Proofs** run on a staging test computer: test class, never b@c, never
  `critical_protected` or `public_platform`. The canary is the op
  `tape_compat_probe_recorded` (`safe`, a new table `tape_compat_probes`): its
  reader ships in one release and its writer later, behind a host-gated
  test-class endpoint.

| Slice | Change | Proof | Rollback | Class, tier |
|---|---|---|---|---|
| **S1** | The manifest, the split registries, the static check (report-only) | the selftest with throwaway repos, as in `selftest-store-schema-floor`; the manifest equality test | revert | yellow, plus the registry split (no behaviour change); tier 1 |
| **S6** | The `tape-compat` job, harness and both directions, fail-open until the base has the harness | the 4 wall-clock metrics over 5 runs (§4.1); a deliberate pull request that writes an op with no base reader, which must fail | revert | yellow; tier 1 |
| **S7** | The additive store check, the round trip, the floor guard wired in, the optional-table registrations | selftests; the round trip on today's base | revert | yellow; tier 1 |
| **S3** | corpusd `event_raw` (always sent); guest raw digest behind `CHOIR_TAPE_RAW_DIGEST` | a CI fixture with a test-only envelope field; the test computer boots and passes replay completeness with the switch on | reader stays on once fleet-on; corpusd field additive | red; tier 2 |
| **S2** | The ledger (optional table, two-phase per §5), batch `compat` stamp (`CHOIR_TAPE_STAMP`), op skip with the structural table check (`CHOIR_TAPE_SKIP`), `/health`, head-neutral catch-up | on the test computer: probe ops written, then the updater rolls back to the reader-less release (or a restore to its checkpoint); `/health` shows skipped and boot succeeds; forward update empties the ledger and the probe rows appear | stamp off; skip reader stays on | red; tier 2 |
| **S8a** | The builder stamps the manifest digest and the manifest itself into `release.Record` and the updater manifest | the release record of one build on the test computer | revert (readers treat a missing manifest as "cannot rebuild with target manifest": the restore refuses) | orange; tier 1 |
| **S5** | Step 0, documented first: the probe (checkpoint C at T; three preference writes; restore to C; read; one write, where H2 predicts a refusal; owner restart; read; restore to C again, Devin 4; record the results in the problem doc). Then: the corpusd pin index, owner-act admission and the `restore_pins` capability; the pin-before-flip order with cancel; `effective()` in `reconstruct`; the head-only finalize; the index-reader filter; the restart that always closes work; the idempotency changes; the preflight; D7 | step 0 repeated with `CHOIR_RESTORE_PIN` on: head at P, the preference stays at T across two restarts, an immediate write succeeds, a probe event in the window shows as excluded, a failed flip leaves a cancel, a second restore writes a second pin | writer off (new restores unpinned); the S5 reader stays on. A code revert below S5 while any pin exists is refused by the static check unless the corpusd pin index count is zero (F10) | red; tier 2 |
| **S8b** | Apply and restore preflight against the computer's own tape: "this release would skip N events of kind K", refusing `refuse` kinds it lacks. This is SR8's "proved reversible" input, and it covers self-development candidates on old bases (selfdev-79dc4bf4) | an apply of a candidate built on a pre-probe base: the approval surface shows the skip line | revert | red (run acceptance); tier 2 |
| **S4a** | `Validate` split, receipt-verified head for unknown kinds, `RequireKindFields` relaxed | CI fixture | reader stays on | red; tier 2 |
| **S4b** | The corpusd receipt `compat` stamp for new kinds | ships with the first new event kind, 7 days or more after S4a is fleet-on | stamp off | red; tier 2 |
| **S9** | The doctrine entry, after S2, S5 and S6 are proved | review | revert | green, reviewed like the change |

**Order:** S1, then S6 and S7 together, then S3, S2, S8a, S5 (its step 0 runs
now), S8b, S4a and S4b, and finally S9.

## 7. Open decisions and residuals

### Open decisions (defaults, No Blocking Asks)

- **D1. How unknown ops are skipped.** Default: one op at a time, only ops the
  writer listed, plus the reader's structural table check.
- **D2. The window clock.** Default: deployment time from the host's release
  record, delivered host-set. Changed from revision 1 (DeepSeek 16). CI's
  static check uses the manifest's `since` dates as a proxy.
- **D3. Restore to a release that predates S5, or with no `restore_pins`
  capability.** Default: an unpinned restore, today's semantics, with "this
  restore lasts until the next restart" in the answer and in Texture. A pin is
  never written for a binary that would ignore it.
- **D4. Pin lookahead.** Default: the corpusd index, made authoritative as the
  record of admission, plus the capability gate.
- **D5. The gate base for pull requests.** Default: the deployed commit, from
  public `/health`; after bootstrap the gate fails closed when it cannot read
  it. Changed from revision 1 (DeepSeek 17).
- **D6. The pin encoding.** Default: an `AuthorityRef` value with owner-act
  admission. A new kind would make every reader older than S4a refuse the
  event itself.
- **D7 (new). Head effective state after a pin.** Default: the rollback pair
  (§3.6).

### Residuals

- **R1. The head names restored-away code.** It is settled by D7 before S5.
- **R2. An older writer's upsert drops newer body fields.** The two-release
  rule narrows the risk; it does not close it.
- **R3. The gate tests only the latest deployed reader**, plus the static
  generic-reader age rule.
- **R4. A changed projector for a known op** still breaks the witness on
  restore. The refusal is loud.
- **R5. Showing the excluded window** in Texture and the Updates app. It is a
  prerequisite for pins on protected computers.
- **R6. Older corpusd on a newer tape.** Reads are covered by `event_raw`; the
  tape-segment decode was not read.
- **R7. Primary-key reshapes at Open** are unprobed. The S7 round trip decides
  them.
- **R8. The inventory of external-effect and adoption kinds** for the
  preflight (§3.6) [H].

## Review answers (bunch 222)

Every answer below was checked against the code named in it.

**Devin (APPROVE with required additions)**

| ID | Finding | Answer |
|---|---|---|
| D-1 | New tables break airworthiness on older targets | **Adopted.** Verified that unknown tables are denied (`replay_eligibility.go:185-189`) and that the witness covers all tables except `optionalTables` (`dolt_state_extractor.go:244-256`). Fix: two-phase table registration (§5), the ledger as an optional table with no new column (§2.4), and a gate witness assertion (§4.3 item 6) |
| D-2 | The pin index has no downgrade path | **Adopted.** A pin needs corpusd's index row, written at admission; the guest checks the `restore_pins` capability first; without a row the event is an unpinned restore (§3.2) |
| D-3 | A pinned restore with an unchanged binary schedules no restart | **Adopted.** Verified the restart is gated on `ExecChanged` (`rematerialize.go:328`, `:372`). Every pinned restore restarts and closes work. Busy runs do not defer it; mid-apply and live capsules refuse it in preflight. `critical_protected` computers get no pins by default (§3.5, §3.6) |
| D-4 | A repeat restore is deduplicated | **Adopted.** Verified (`rematerialize.go:399`). Restore keys are minted per owner act; the case is added to the step 0 probe |
| D-5 | Rule (b) omits `ReducerVersion` | **Adopted.** Rule (b) now reads "every field other than sequence and canonical head" |
| D-6 | Skipped-kind events have no writer stamp | **Adopted.** The receipt `compat` carries `writer_base_commit` and `writer_deployed_at` |
| D-7 | §4.4 omits version bumps | **Adopted** (§4.4) |
| D-8 | `decode.go:331` does not exist | **Adopted.** Verified that the file has 157 lines; fixed to `decode.go:152` |
| D-9 | Catch-up of a skipped kind; §3.6 is inconsistent with D3 | **Adopted.** Catch-up runs the full finalize effects head-neutrally (§2.5); a target older than S5 gets an unpinned restore, not a refusal (§3.6) |

**DeepSeek v4.1 flash (revise before implementation)**

| ID | Finding | Answer |
|---|---|---|
| S-1 | The pin is guest-writable | **Adopted.** Verified `AuthorityRef` is set by the guest (`rematerialize.go:415`) and that only `effect_accepted` has owner-act admission (`computer_events.go:247-251`). The pinned value is admitted only with an owner-act receipt (§3.2) |
| S-2 | A failed restore leaves a live pin | **Adopted.** The pin is written before the flip with explicit failure handling: a cancel event, or self-healing at boot via the missing ledger row (§3.2) |
| S-3 | S5 rollback is unsafe once a pin exists | **Adopted.** Rollback turns the writer off and keeps the reader on; a code revert below S5 is refused while the pin count is above zero (§6) |
| S-4 | `safe` as default is fail-open | **Adopted in part.** There is now no default, and the reader checks structural independence by tables (§2.1, §2.2). **Rejected:** "default `refuse`". The owner's invariant is keep-and-skip; with `refuse` as default, every new data kind would become a boot refusal after a restore, which is the SR7 receipt itself. The structural check removes the fail-open path the finding describes. `key_rotated` is a known kind, not an unknown one; new head-neutral authority must be declared `refuse` |
| S-5 | The age rule covers only `refuse` kinds | **Adopted in part.** The generic skip readers (S2, S4a, S3) must have been deployed for 7 days before any new op, kind or field is written (§4.4). **Rejected:** a 7-day age for every kind's *specific* reader. In-window releases without it skip the kind generically, so the age would only slow development |
| S-6 | S5 depends on S8 | **Adopted.** S8 is split; S8a (the manifest stamp) precedes S5 |
| S-7 | Witness breakage from the ledger or a column | **Adopted.** Same as D-1 (verified) |
| S-8 | The CI no-lengthening proof is wrong under slot saturation; `git log -S` on a shallow checkout | **Adopted.** Placement is judged by run wall clock, last shard end and prebuild start; the gate moves to a separate job queued after the prebuild, with a measured fallback; `since` dates live in the manifest; the fetch is depth 1 (§4.1, §4.4). **Rejected:** a permanently report-only gate; it is report-only only during the bootstrap week |
| S-9 | Catch-up cannot finalize behind the head | **Adopted.** Verified (`computer_events.go:192`). Catch-up uses a head-neutral path (§2.5) |
| S-10 | Excluded unknown kinds cannot run `Reduce` | **Adopted.** Verified (`reducer.go:61`, `event.go:157`). They use the receipt path, and S4a is a prerequisite (§3.3) |
| S-11 | Pending transition at R; R1 becomes permanent | **Adopted.** Verified (`checkpoints.go:59`). The preflight refuses on a pending transition; D7 (the rollback pair) settles R1 before S5 |
| S-12 | The local index claim was false | **Adopted.** Raw bytes in the index are an S3 change, not today's behaviour (§2.2, §2.3) |
| S-13 | `decode.go:331` | **Adopted** (same as D-8) |
| S-14 | "Switch off" rollback is unsafe after first use | **Adopted** (§6) |
| S-15 | The payload-digest claim was too strong | **Adopted.** Narrowed to fetched payloads; excluded payloads rely on corpusd's pin validation at append (`computer_events.go:218`) (§2.2) |
| S-16 | Commit time for a deployment window | **Adopted.** D2 now uses deployment time (§2.6) |
| S-17 | The PR fallback tests the wrong reader | **Adopted.** D5 now uses the deployed commit on pull requests and fails closed after bootstrap |
| S-18 | The paper, adoption and external effects are not in the preflight | **Adopted.** No pins on `critical_protected`/`public_platform` by default; external effects in the window need owner acknowledgement; external dedup on the external identity (§3.5, §3.6); inventory R8 |
| S-19 | Forks are unaddressed | **Adopted.** Answered, with Devin's ontology reading (`computer-ontology.md:409-414`): a fork has its own chain, its content is `effective(H)`, and replay uses the same `reconstruct` (§3.4) |
| S-20 | Switch provenance | **Adopted.** Host-set, read once at boot, not settable by the guest; the probe gate is on the host side (§6) |
| S-21 | The gate is one-directional | **Adopted.** The base writes a fixture and the candidate replays it (§4.2) |

## Class, heresy delta

- **Class:** green (this doc). Each slice is classed in §6.
- **Heresy delta, discovered:**
  - restore re-converges at boot [code];
  - appends are refused between a restore and its restart [code, H];
  - a restore restart resumes work;
  - a restore without a binary change never restarts;
  - a repeat restore is deduplicated;
  - a guest can set restore authority;
  - the receipt exact-count rule blocks receipt extension;
  - new tables or columns break older checkpoints;
  - the store-floor guard is not in CI;
  - destructive DDL at Open.
- **Heresy delta, introduced:** none.
- **Heresy delta, repaired:** none (design only).
- **Conjecture delta:** "restore rewinds" held only until the next boot. Under
  this design it becomes a property of the tape, as effective history admitted
  by an owner act, not a property of whichever store is on disk.
