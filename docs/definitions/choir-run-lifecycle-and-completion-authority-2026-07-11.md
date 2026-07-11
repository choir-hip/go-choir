# Choir Run Lifecycle and Completion Authority

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md
```

Read this document as executable semantic authority for unifying run lifecycle
state across the runtime, dispatcher, and trajectory projections, and for
enforcing artifact-verified completion on the processor path. It is the
concrete Phase 3 child of the `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`
spine (run truth / G3 / G4).

## Why this mission exists

Twelve activation attempts, culminating in the 2026-07-11 post-mortem,
exposed that processor run completion and retry authority are fragmented.

1. **Cornerstone C4 (split lifecycle authority):** `RunRecord.State`, trajectory
   status, processor-resolution status, the sourcecycled request ledger, and the
   runtime admission counter are five separate projections with no shared
   authority. Each pairwise disagreement froze the pipeline in a different way
   during the 12-hour run:
   - `blocked` (non-terminal in runtime) had no sourcecycled continuation path,
     so one provider 429 froze all admission indefinitely.
   - A runtime refresh passivated a live trajectory (51 open work items) and
     sourcecycled counted it in-flight forever.
   - `run state=completed` coexisted with a live unresolved trajectory, so
     sourcecycled released capacity while runtime admission still counted an
     active processor and 429'd every new submission.

2. **Cornerstone C4b (at-most-once-ever dedup):** Per-cycle deduplication treats
   any prior run — including a terminal failed run that never reached tool
   iteration zero — as the cycle's one authoritative activation. A news pipeline
   whose unit of work is unretryable on transient failures cannot be reliable.

3. **Cornerstone C5 (narrative completion):** Runs and reconcilers reported
   success to the harness while their mandatory output artifacts did not exist.
   In `aabf0e75`, duplicate same-channel Texture rewarms cancelled the only
   mandatory canonical write, and the reconciler still narrated success and
   exited.

After `docs/definitions/choir-wire-store-conformance-2026-07-11.md` dissolves
C1/C2/C6, C4 and C5 remain the blocking residue on the processor path.

## Source Authority Order

1. This Definition.
2. `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`
   (Phase 3 run truth, G3/G4, the four-item external operator test, and the
   Introspection Contract).
3. `docs/definitions/choir-wire-store-conformance-2026-07-11.md`
   (prerequisite: world-wire store, sourcecycled durable ledger, no boot-time
   migration, reconciler work deferred).
4. `docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`
   (C4/C5 evidence and the three freeze modes).
5. `docs/standing-questions.md` (Q5 single authority, Q6 artifact proves success,
   Q9 no-SSH operability).
6. `AGENTS.md`, `docs/choir-doctrine.md`, `docs/runtime-invariants.md`.
7. Observed source:
   - `internal/types/task.go` (`RunRecord`, `RunState`)
   - `internal/types/trajectory.go` (`TrajectoryRecord`, `WorkItemRecord`)
   - `internal/runtime/runtime.go` (run execution and lifecycle management)
   - `internal/runtime/api.go` (run submission, fingerprinting, concurrency)
   - `cmd/sourcecycled/main.go` (dispatch ledger and `isTerminalRuntimeState`)
   - `cmd/choir` (CLI verbs)

## Settled Inputs (do not re-litigate)

- **D-WIRE and the two-store taxonomy** from
  `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` and
  `docs/computer-ontology.md` are inherited via `choir-wire-store-conformance`.
  The wire state is on the `corpusd` world-wire store.
- **Sourcecycled trigger monopoly** is inherited from `choir-wire-store-conformance`.
  Sourcecycled remains the only source-cycle trigger and the typed ingestion
  handoff remains the activation identity.
- **The five projections are split and the `autoputer-cli` spine requires one
  lifecycle authority and artifact-verified completion.** This is the problem
  statement from the post-mortem and from G3/G4.
- **A run that fails before producing its required artifact is not a successful
  activation.** This is the operative meaning of C5 / Q6.
- **The processor is the one-agent publish path in scope for this mission.**
  Reconciler/editorial artifact verification is explicitly out of scope until
  that one-agent path is stable.

## Mission Purpose

1. **Choose one lifecycle authority** (default: `RunRecord.State` in
   `internal/runtime`) and make the trajectory, sourcecycled ledger,
   processor-resolution, and admission counter read-only projections of it.
2. **Implement retryable ingestion idempotency:** a failed, cancelled, or blocked
   run must release capacity and allow a new run for the same cycle; a
   concurrently active run must prevent duplicate submission.
3. **Implement artifact-verified completion for the processor path:** terminal
   `RunCompleted` requires a fetchable processor artifact (a published world-wire
   article with intact ingestion lineage) in the `corpusd` world-wire store.
4. **Expose `choir run status <id>`** as the CLI-visible, substrate-neutral truth
   for the run state, trajectory summary, and artifact receipt.

## Mission Non-Purpose

- No reconciler/editorial-review work. The reconciler is post-publication by
  definition and is gated behind a stable one-agent processor publish path.
- No changes to the LLM provider interfaces or model-client logic.
- No PC-5 / Base exact-byte kernel or audited-computer candidate-materialization
  work. PC-5 is owned by `docs/definitions/choir-product-completion-2026-07-10.md`
  and `docs/computer-ontology.md`.
- No promotion, route-slot, VM lifecycle, or key-scoping work. Those are later
  phases of the `autoputer-cli` spine.
- No changes to the `universal-wire` feed UI beyond the read path owned by
  `choir-wire-store-conformance`.

## Open Decisions (owner input required; default if silent)

- **Single authority selection:**
  - *Default:* `RunRecord.State` in `internal/runtime` is the sole writer and
    source of truth; all other projections are read-only or derived views.
  - If the owner selects a different authority (e.g., the sourcecycled ledger on
    the `corpusd` server), this mission must be re-derived from that choice.
- **Retry policy:**
  - *Default:* sourcecycled retries on the next polling cycle or next source
    poll, up to a bounded retry budget (e.g., 3 attempts per cycle). No
    synchronous immediate retry.
- **Blocked-run timeout:**
  - *Default:* `RunBlocked` is auto-cancelled after a configurable timeout (e.g.,
    10 minutes) so it releases capacity and allows a retry.
- **Artifact predicate for the processor:**
  - *Default:* the required artifact is a published world-wire article/story
    route with ingestion lineage in the `corpusd` store.

## Invariants

- A run record in state `RunCompleted` must have a fetchable artifact in the
  world-wire store.
- `RunBlocked`, `RunFailed`, and `RunCancelled` must not freeze admission
  capacity and must not permanently consume cycle idempotency.
- Admission capacity is derived from `RunRecord.State` (`RunPending` +
  `RunRunning`), not from an in-memory or separately maintained counter.
- Two runs with the same cycle/ingestion fingerprint cannot be concurrently
  active.
- `choir run status <id>` returns the same state as the single authority.

## Completion Semantics

The mission is `complete` when all of the following are observed on staging:

1. A simulated provider 429 or execution error during a processor run
   transitions the run to a terminal error state (`RunFailed` or
   `RunCancelled`), releases admission capacity, and allows the next sourcecycled
   poll to submit a retry request.
2. A processor run that does not produce a world-wire article terminates in a
   failed state, even if the agent process exits without crash or OOM.
3. `choir run status <id>` prints the unified state (run state, trajectory
   summary, work items, and artifact receipt) and matches the single authority.
4. The three freeze modes from the 2026-07-11 post-mortem
   (`blocked` with no continuation, `passivated` with live trajectory, and
   `completed` with live trajectory) are reproduced as regression tests and
   resolve under the single authority.
5. Duplicate concurrent submissions for an active run are rejected with
   `409 Conflict`, while retries for failed/cancelled runs are accepted and run.

## First Phase: Authority Alignment and Projection Cleanup

- **Objective:** Inventory the five projections, choose the single authority, and
  make the admission counter a derived view of `RunRecord.State`.
- **Changes:**
  - Update `isTerminalRuntimeState` in `cmd/sourcecycled/main.go` to handle
    `blocked` deterministically and route it to a terminal error state or a
    retryable dispatch state.
  - Replace the in-memory admission counter with a derived query against
    `RunRecord.State` in `internal/runtime/runtime.go` and
    `internal/runtime/api.go`.
  - Ensure `processorRunOccupiesAdmission` is a predicate over the derived view,
    returning `false` for `failed`, `cancelled`, and timed-out `blocked` runs.
- **Acceptance:** A deliberately blocked run is visible as `blocked` in
  `choir run status`, and a second independent request can be admitted.

## Follow-on Missions

- **Audited computer / candidate materialization** — `autoputer-cli` Phase 1;
  owned by `docs/definitions/choir-product-completion-2026-07-10.md` PC-5 and
  `docs/computer-ontology.md`.
- **Deploy/verify receipt trust** — `autoputer-cli` Phase 2 / G6.
- **Promotion truth gate / route-slot CAS** — `autoputer-cli` Phase 4 / G5.
- **Key-scoping containment** — `autoputer-cli` Phase 5 / G7.
- **Autopaper editorial / reconciler artifact-verified completion** —
  post-publication, after the one-agent processor path is stable.
- **Decision-provenance hygiene** — `settled_by` field in
  `docs/doc-authority-manifest.yaml` for orchestrator-settled vs owner-ratified
  nodes (named follow-on in `choir-wire-store-conformance`).

## Supersession Record

- Depends on:
  `docs/definitions/choir-wire-store-conformance-2026-07-11.md` and
  `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`.
- Investigation basis:
  `docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`.
- Does not supersede `docs/definitions/choir-product-completion-2026-07-10.md`
  PC-5 or `docs/computer-ontology.md`.

## Red-Class Ceremony

- **Mutation class:** Green/yellow for this document; the code changes this
  Definition authorizes touch red surfaces (run acceptance, canonical writes,
  sourcecycled ledger, CLI command surface) and must be executed with full
  red-class ceremony.
- **Protected surfaces:** [run acceptance, canonical writes in the world-wire
  store, sourcecycled dispatch ledger, trajectory/processor state, `choir run`
  CLI surface, external-agent observable set].
- **Admissible evidence class:**
  - Regression tests for the three freeze modes.
  - Integration/e2e traces on staging that show `choir run status` truth and
    artifact existence in the world-wire store.
  - `409 Conflict` for duplicate active submissions and successful retry for
    failed/cancelled submissions.
  - No SSH, journalctl, or raw SQL in the acceptance path.
- **Rollback path:** Revert to the pre-authority runtime/sourcecycled admission
  logic; keep the legacy in-memory admission counter under a feature flag until
  the derived `RunRecord.State` counter is staging-proven.
- **Heresy delta:**
  - `discovered`:
    - five run-state projections with no shared authority;
    - at-most-once-ever dedup burning cycles on transient failures;
    - agent completion narrated without a required artifact.
  - `repaired`:
    - one `RunRecord.State` authority with read-only projections;
    - retry semantics that distinguish `succeeded already` from
      `failed before starting`;
    - terminal completion requires a fetchable artifact in the world-wire store.
