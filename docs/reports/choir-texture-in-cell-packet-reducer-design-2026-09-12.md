# Texture packet and reducer design — constraints under the in-cell carrier

Status: non-gating design artifact owed by mission 3
(`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`,
`finish.non_gating_artifacts`). **No Texture runtime code lands in this
mission**, no canonical-writer behavior changes, and no Texture JSON writer is
deleted here. This document is a constraint specification, not an
implementation plan: it states what a Texture packet/reducer design must
satisfy, cites the canonical writer's invariants from their own authority, and
names the engineering carrier's proven mechanisms as the precedent to copy.

It deliberately does **not** assert Texture internals it could not verify. Where
a row requires the Texture codebase (object shapes, the mutation vocabulary, the
revision-graph identity construction), the row says so and Mission 4 owns it.

## Authority

- `docs/choir-doctrine.md:278` — `I1`: two writer classes; `AuthorAppAgent` is
  the sole *agent* writer, every revision a new monotonic self-contained
  snapshot, one revision per semantic-changing turn and none for
  wait/block/no-change, and `texture_turn_committed` counts turn outcomes rather
  than revisions.
- `docs/choir-doctrine.md:289-334` — `I2` (289), `I2a` (293), `I2b` (301), `I2c` (309), `I3` (315), `I4` (319), `I5` (323), `I6` (327), `I7` (330):
  Texture is not forced into semantic delegation; owner-triggered work is
  visible as artifact state; agent-to-agent update identity is runtime-owned and
  deterministically derived, never invented by a model; parent/child is not a
  control ontology; work items are trajectory obligations; **`I5` dual paths are
  bugs**; no new dependency on a live heresy; anything that matters for
  settlement or rewarm becomes durable obligation state.
- `docs/semantic-registry.md:14` — `SEM-02`: Texture is the sole agent writer of
  canonical document revisions; every Texture-agent semantic-changing authoring
  turn creates exactly one new monotonic, self-contained snapshot; no
  change/wait/block/control turns create revisions.
- `docs/semantic-registry.md:16` — `SEM-04`: authority is an envelope of
  obligations, evidence, scope and settlement criteria, not a persona or a
  free-form script.
- `docs/reports/choir-rlm-missions-overview-2026-09-09.md:122-130` — the eight
  canonical-writer invariants named as the Texture landing condition.
- `docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md:161` — records
  that the eight invariants were *not* in evidence for the panel, which is why
  this artifact states them explicitly and one by one.

## The eight invariants, as constraints on the packet/reducer design

Each row gives the invariant, the constraint it imposes on an in-cell Texture
packet and its one reducer, the engineering-carrier precedent that already
survived the same test, and the observable that would falsify the design.

| # | Invariant | Constraint on the design | Precedent in the engineering carrier | Falsifier |
|---|---|---|---|---|
| 1 | Single-writer discipline | The reducer must be the only path that stages a canonical mutation; an in-cell function may stage an intent but must never hold the write. `SEM-02`/`I1` allow exactly two writer classes, so no packet may carry an `author_kind` a model can choose | The assigned-CoSuper registry is an exact closed set with `capsule_go_eval` as the sole JSON envelope (`tool_profiles_authority_test.go`, `TestRLMAssignedCoSuperOverlayIsSealedGo`); every other affordance is a typed cell function staging intents | A second code path can commit a revision, or a packet field lets a model name the writer class |
| 2 | Stale-base comparison | Every mutation intent must carry the base identity it was authored against, and the reducer must compare it before any effect. A stale base must conflict **before** the effect, not be merged | The frozen mapping table's pre-declared canonical identity fields per operation (`docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md` §2), applied as a pre-effect conflict on a reused identity with changed canonical input | A mutation applies on a base the reducer did not compare, or a stale base is silently rebased |
| 3 | Atomic revision-graph-identity commits | The revision, its graph identity and its source-graph write set must commit in one transaction; a partially visible revision is a corrupt truth surface | The freeze path commits an operation/bundle with the classifier digest, groups and event head as one object; `tools_capsule.go` freeze short-circuit returns only `{handle,bundle_digest,operation_id,state}` | A committed revision exists without its graph identity, or with a graph identity that does not name its revision |
| 4 | Retry-preserved pending mutations | A staged mutation that has not taken effect must survive retry with its identity intact; a retry re-submits the same semantic identity and the reducer must be idempotent | Replay goldens prove operation classes return the original receipt with zero additional effects (`internal/agentcore/testdata/rlm_replay/`), plus the effect census per row | A retry mints a second mutation, or a pending mutation is lost when the turn is retried |
| 5 | Versioned compare-and-swap | Head replacement must be an explicit CAS on a version the caller names, never a read-then-write. `I1`'s `AuthorUser` path is exactly this and must stay the only other CAS | `CancelCoSuperAssignment`/`RecordCoSuperAssignmentReport` carry `ExpectedLifecycleVersion`, and a reused identity with changed input conflicts before effect | A canonical head can be replaced without naming the version it replaces |
| 6 | Fresh-but-not-replay wakes | A wake must be distinguishable from a replay: the reducer must act on a genuinely new obligation and must not re-enter on a replayed delivery, and it must not require a model to invent an identity (`I2c`) | Deterministic identity derivation from the authenticated parent run and tool call (`deterministicAssignmentIdentity`), which changed semantic arguments do not alter | A duplicate delivery causes a second wake, or a genuine wake is suppressed as a replay |
| 7 | Per-document locking | Concurrency must be scoped to the document being mutated; two documents must not serialize against each other, and one document must not interleave two mutations | The assignment fate saga's compare-and-swap retries and its explicit revoke/cancel ordering under a single lifecycle version | Two concurrent mutations interleave inside one document, or unrelated documents serialize |
| 8 | Atomic researcher opening | When a researcher opens against a document, the opening must be one atomic act that is either fully visible or absent; it must not be observed half-open, and it must not bypass the writer class of #1 | The work-item/control admission pair (`work_opened` → `control_queued` → `control_delivered`) is durable and ordered, and the assignment cannot execute before its binding is durable | A half-open researcher state is observable, or an opening can write document text |

## Packet shape (what the design must carry, by class)

The frozen nine-operation mapping table already fixes the discipline the
Texture packets must reuse. Its two receipt classes that bear directly on
Texture are:

- **`lifecycle_fate`** — canonical fields must come from the persisted fate
  record, never from the tool envelope or the Go return value. For Texture this
  means the revision/disposition identity is read from the durable record the
  reducer wrote, not from what the cell returned.
- **`read_only_inspection`** — a synchronous typed observation under the
  read-only exemption. Texture's read-only affordances (history, source graph,
  patch preview) may remain synchronous typed functions; nothing forces them
  through a two-turn protocol.

The remaining classes (`transient_mutation`, `transient_observation`,
`selfdev_freeze`, `selfdev_verify`, `lifecycle_update`) carry the same
requirement: the packet names semantic inputs, and the reducer derives identity,
version and digest. A packet field that lets a model supply an identity the
runtime can derive is a defect, not a convenience (`I2c`).

The exact Texture object names, field set and mutation vocabulary are **not
asserted here**. Mission 4 must derive them from the Texture codebase and from
the canonical writer's own definitions, then re-check this table row by row.

## What this design forbids

- A Texture packet that names its own writer class or a non-canonical source as
  the writer (`I1`, `SEM-02`).
- A second revision path beside the reducer, even temporarily, unless it is
  frozen, gated and on a named deletion clock (`I5`).
- Replay proof by raw byte equality, or a declared-field set narrowed by hand.
  The engineering carrier's replay review already rejected both as vacuous:
  canonical equality is over declared fields, exclusions are pre-declared, and a
  rejection receipt does not earn a deletion.
- Any design that depends on parent/child for control, liveness or settlement
  (`I3`), or that keeps settlement-relevant facts only in narrative text
  (`I7`).
- Widening the Texture agent's authority "while we are in here". This artifact
  changes no behavior; it is a constraint list.
- Reading a reference to this document as authorization to change the canonical
  writer. Mission 4's landing is gated on the eight invariants being satisfied
  under the carrier, and it is a separate charter.

## Non-goals

Texture runtime code, canonical-writer mutation, deletion of any Texture JSON
writer, and the Texture packet/reducer implementation. This document exists so
that the Texture landing inherits a precedent with receipts instead of
re-deriving the discipline under pressure.
