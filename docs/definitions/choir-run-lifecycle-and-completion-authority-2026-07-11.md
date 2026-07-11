# Choir Run Lifecycle and Completion Authority

## Subordinate Invocation Semantics

This document is the S6 run-truth specification of:

```text
/goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
```

Do not invoke it independently. The grand-suite orchestrator executes it only
after S1 Deploy restoration, S2 Wire authority cutover, S3 runtime extinction,
S4 audited-computer proof, and S5 observation/receipt proof. Its lifecycle
semantics survive; implementation must target the extracted core boundary, not
recreate `internal/runtime`.
## Why this mission exists

Twelve activation attempts, culminating in the 2026-07-11 post-mortem,
exposed that processor run completion and retry authority are fragmented.

1. **Cornerstone C4 (split lifecycle authority):** `RunRecord.State`, trajectory
   status, processor-resolution status, the sourcecycled request ledger, and the
   runtime admission counter are five separate projections with no shared
   authority. Each pairwise disagreement froze the pipeline a different way:
   - Historically (pre-`f1ceba5`), `blocked` was non-terminal in runtime and had
     no sourcecycled **redispatch / retry** path, so one provider 429 froze
     admission. (`f1ceba5` already classifies `blocked` as terminal; residue is
     retry routing — Phase B. Do **not** reopen continuation machinery.)
   - A runtime refresh passivated a live trajectory (51 open work items) and
     sourcecycled counted it in-flight forever.
   - `run state=completed` coexisted with a live unresolved trajectory, so
     sourcecycled released capacity while runtime admission still counted an
     active processor.

2. **Cornerstone C4b (at-most-once-ever dedup):** Per-cycle deduplication treats
   any prior run — including a terminal failed run that never reached tool
   iteration zero — as the cycle's one authoritative activation.

3. **Cornerstone C5 (narrative completion):** Runs and reconcilers reported
   success while mandatory output artifacts did not exist. In `aabf0e75`,
   duplicate same–Texture-actor-mailbox rewarms cancelled the only mandatory
   canonical write, and the reconciler still narrated success.

After S2 dissolves C1/C2/C6 and S3 removes the runtime god package, C4 and C5
remain the processor-path run-truth subgoal. S1 owns the fourth freeze mode
(`running` forever); S6 re-verifies it.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-suite-2026-07-11.md`.
2. This subordinate Definition within S6 scope.
3. `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`
   (operator test and Introspection Contract).
4. `docs/definitions/choir-wire-store-conformance-2026-07-11.md`.
5. `docs/definitions/choir-run-deploy-unblock-2026-07-11.md`.
6. `docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`
   as failure evidence.
7. `docs/standing-questions.md`, `AGENTS.md`, `docs/choir-doctrine.md`,
   `docs/runtime-invariants.md`.
8. Observed post-S3 source: extracted lifecycle/core packages,
   `internal/types`, `cmd/sourcecycled`, and `cmd/choir`.

## Settled Inputs (do not re-litigate)

- **D-WIRE and the two-store taxonomy** are inherited via
  `choir-wire-store-conformance`. After that mission completes, wire product
  state is served from the `corpusd` world-wire store. **Until then, do not
  claim the cutover is already true** (Dependency Truth).
- **Sourcecycled trigger monopoly** is inherited from wire-store conformance.
- **Five projections are split;** the autoputer-cli spine requires one
  lifecycle authority and artifact-verified completion (post-mortem G3/G4).
- **A run that fails before producing its required artifact is not successful**
  (C5 / Q6).
- **Processor path only** for this mission; reconciler/editorial verification
  waits until the one-agent path is stable.
- **Progress deadline and operator cancel** are inherited from grand S1.
  Re-verify them here; do not build a second mechanism.

## Mission Purpose

1. **Choose one lifecycle authority** (default: `RunRecord.State`) and make
   trajectory, sourcecycled ledger, processor-resolution, and admission
   read-only or derived projections of it.
2. **Retryable ingestion idempotency:** failed, cancelled, or blocked runs
   release capacity and allow a new run for the same cycle; a concurrently
   active run prevents duplicate submission.
3. **Artifact-verified completion** for the processor path: `RunCompleted`
   requires a fetchable world-wire article with ingestion lineage in the
   `corpusd` store (enforceable only after wire-store conformance).
4. **`choir run status <id>`** as CLI-visible, substrate-neutral truth for run
   state, trajectory summary, and artifact receipt.

## Mission Non-Purpose

- No reconciler/editorial-review work.
- No LLM provider interface changes.
- No PC-5 / audited-computer materialization (product-completion / ontology).
- No promotion, route-slot, or key-scoping (`autoputer-cli` later phases).
- No VM reprovisioning.
- **No naming / rename sweeps** — grand S9 owns vocabulary cutover. New code
  here is still born in successor vocabulary.
- **No continuation, parent/child, or result-channel deletion** — grand S3
  must already have completed those cutovers.

## Dependency Truth (verified 2026-07-11; refresh at Phase 0)

- Grand S2 Wire conformance and S3 runtime extinction must be complete before
  this subgoal begins. Return to the suite orchestrator if they are not.
- `isTerminalRuntimeState` already treats `blocked` as terminal (`f1ceba5`).
  Phase B decides terminal-error vs retryable-dispatch routing.
- `RunningCountByProfile` already derives from
  `store.ListRunsByState(..., RunRunning, ...)`. Residue: (a) error path falls
  back to in-memory `RunningCount()`; (b) `RunPending` not counted; (c)
  `processorRunOccupiesAdmission` defaults to "occupies" on lookup error.
- Fourth freeze mode (`running` forever) is inherited from grand S1.

## Open Decisions (defaults govern; deviations recorded in ledger)

- **Single authority:** `RunRecord.State` semantics have one writer in the
  extracted lifecycle/core package; other projections are read-only or
  derived. S3 must leave this boundary explicit before S6 begins.
- **Retry policy:** sourcecycled retries on the next poll, up to **3** attempts
  per cycle. No synchronous immediate retry.
- **Blocked-run timeout:** `RunBlocked` auto-cancels after **10 minutes** so
  cycle idempotency releases for retry (Phase B sweep). Admission must already
  treat all `blocked` as non-occupying (Phase A).
- **Progress deadline:** inherited from grand S1; exact mechanism is re-verified.
- **Artifact predicate:** published world-wire article/story with ingestion
  lineage in the `corpusd` store — only after wire-store conformance.

## Invariants

- `RunCompleted` has a fetchable artifact in the world-wire store (from Phase D).
- `RunBlocked`, `RunFailed`, and `RunCancelled` must not freeze admission and
  must not permanently consume cycle idempotency.
- Active work stays within the grand S1 progress bound.
- Admission is derived from `RunRecord.State` (`RunPending` + in-deadline
  `RunRunning`); no silent fallback to the in-memory map on errors.
- Two runs with the same cycle/ingestion fingerprint cannot be concurrently
  active.
- `choir run status <id>` matches the single authority.

## Completion Semantics

Complete when all are observed on staging:

1. Simulated provider 429 / execution error → terminal `RunFailed` or
   `RunCancelled`, admission released, next sourcecycled poll may retry.
2. Processor run without a world-wire article → `failed`, even if the process
   exits cleanly.
3. `choir run status <id>` prints unified state (run, trajectory summary, work
   items, artifact receipt) matching the single authority.
4. The four freeze modes are regression-tested and resolve under the single
   authority:
   - `blocked` with no redispatch/retry path (historical; routing fixed in B)
   - `passivated` with live trajectory
   - `completed` with live trajectory
   - `running` with expired progress bound (grand S1; re-verify here)
5. Duplicate concurrent submissions → `409 Conflict`; retries after
   failed/cancelled → accepted.
6. Grand S1 Deploy restoration still holds.

Vocabulary residue is not an S6 completion criterion; grand S9 owns cutover.

## Sequencing and Gates

Order: Phase 0 → A → B → C (verify grand S1) → D → E.

Every phase:

1. **Consensus checkpoint** through the grand-suite orchestration protocol.
2. **Focused local proof** on affected packages plus build/vet as required.
3. **Landing loop** per `AGENTS.md`.
4. **QA** on staging; record IDs in the evidence ledger.
5. **Halt-on-red;** repair within phase (documentation-first) or roll back to
   the **rollback ref** = `git rev-parse HEAD` recorded at phase start before
   the first behavior commit. Do not leave staging red.
6. Do not start the next phase until acceptance is recorded.

Docs-only commits use the docs-only landing path.

## Execution Phases

### Phase 0 — Reconciliation and checkpoint review (green/yellow)

- Confirm grand S1–S5 are complete in the suite checkpoint.
- Reconcile this semantic contract against the extracted S3 boundary.
- Run agentic consensus through the grand-suite checkpoint protocol.
- Record the adjudicated ledger entry. No code mutation.

### Phase A — Authority alignment and derived admission (red)

- **Objective:** admission is a derived view of `RunRecord.State` with no
  in-memory fallback.
- **Changes:**
  - Remove silent fallback from `RunningCountByProfile` to `RunningCount()`;
    surface store errors. Count `RunPending` with `RunRunning`.
  - `processorRunOccupiesAdmission` returns `false` for `failed`, `cancelled`,
    and **all** `blocked` runs (blocked is terminal and must not occupy);
    lookup errors surface explicitly instead of defaulting to "occupies".
- **Local proof:** run the focused extracted-lifecycle tests matching
  `Admission|RunningCount|Idempotency`.
- **QA:** a deliberately blocked run is visible as `blocked` (API or
  `choir run status` if already sufficient), does not occupy admission, and a
  second independent request is admitted.
- **Rollback path:** revert the entire Phase A landing to its recorded
  pre-phase SHA or fail closed. No feature flag, compatibility counter, or
  selectable legacy admission path may survive the cutover.

### Phase B — Retryable ingestion idempotency (red)

- **Objective:** dedup distinguishes `succeeded already` from `failed before
  starting`; terminal errors release cycle idempotency.
- **Changes:**
  - `cmd/sourcecycled/main.go`: route terminal `blocked`/`failed`/`cancelled`
    to the retryable path with budget **3** per cycle. Implement the **10
    minute** blocked auto-cancel sweep for cycle release / retry eligibility.
  - Extracted lifecycle/API owner: active fingerprint → `409`; resubmission
    after terminal error → new run.
- **Local proof:** run focused `cmd/sourcecycled` and extracted-lifecycle tests
  matching `Terminal|Retry|Dedup|Conflict|BlockedTimeout`.
- **QA:** simulated 429 → terminal state, capacity release, successful retry on
  next poll; duplicate concurrent submission → `409`.

### Phase C — Verify deploy-unblock still holds (yellow/red if regression)

- **Objective:** confirm grand S1 deadline/cancel/Deploy proof still holds.
- **Changes:** none if green; if regressing, return the finding to the
  grand-suite orchestrator for documentation-first repair.
- **QA:** `running_runs: 0` on the platform VM (or equivalent health) and Deploy
  not blocked by a stuck run.

### Phase D — Artifact-verified completion (red)

- **Gate:** grand S2–S5 completion is recorded. If not, return control to the
  grand-suite orchestrator; do not invoke another `/goal`.
- **Changes:** processor `RunCompleted` requires the artifact predicate; clean
  exit without artifact → `RunFailed`.
- **Local proof:** run focused extracted-lifecycle tests matching
  `Artifact|Completion`.
- **QA:** publish → `completed` with fetchable receipt; suppressed publish →
  `failed`.

### Phase E — Unified `choir run status` and lifecycle proof (red)

- Extend `choir run status <id>` to print unified state (artifact receipt after
  D).
- Land four freeze-mode regression tests if not already present.
- Run grand-suite agentic consensus on the full S6 diff and evidence.
- Deployed acceptance: the run-truth slice of the external operator test.
- Update the grand suite's evidence and resumption state.
- Return control to the grand-suite orchestrator for S7.

## Follow-on Missions

- Grand S9 owns vocabulary cutover and successor handoff.
- Grand S7–S8 own promotion, contained keys, and Choir-in-Choir.
- Autopaper editorial/reconciler verification remains a successor mission.
- Overlapping og-dolt deletion families are absorbed by grand S3.

## Supersession Record

- Subordinate to `choir-autoputer-completion-suite-2026-07-11.md` S6.
- Investigation basis: autopaper activation attempt report.
- Amended 2026-07-11: Dependency Truth, fourth freeze mode, gates, phases A–E,
  autonomous contract.
- Amended 2026-07-11: owner directed full autonomy (self-adjudicate deviations;
  documented failure exits).
- Amended 2026-07-11: Phase F naming added — **superseded the same day** by
  suite foliation; naming moved to `choir-vocabulary-cutover-2026-07-11.md`.
- Amended 2026-07-11 (suite rewrite): Correctness-only mission; no chain-exec;
  pinned Open Decision defaults; freeze-mode language drops "continuation";
  Phase A frees all `blocked` from admission; blocked timeout lives in Phase B;
  Phase C verifies member 1; three→four freeze modes in admissible evidence;
  "phase rollback ref" = SHA at phase start.

## Red-Class Ceremony

- **Mutation class:** green/yellow for this document; authorized code is red.
- **Autonomous execution contract:** no human turn required. Consensus is
  in-loop review. Defaults govern; contradicting consensus findings are
  self-adjudicated with deviation + rationale in the Supersession Record and
  evidence ledger. Unrepairable halt → documented failure report (SHA, gate,
  diagnosis, rollback ref, next probes) after rolling staging back to the
  phase rollback ref. Failure is an accepted outcome.
- **Protected surfaces:** run acceptance, world-wire canonical writes,
  sourcecycled ledger, trajectory/processor state, `choir run` CLI.
- **Admissible evidence:**
  - Regression tests for the **four** freeze modes.
  - Staging traces for `choir run status` truth and artifact existence.
  - `409` for duplicate active submissions; successful retry after terminal
    failure.
  - No SSH, journalctl, or raw SQL in acceptance.
- **Rollback path:** revert to the pre-phase SHA. Do not retain an old admission
  counter behind a compatibility flag.
- **Heresy delta:**
  - `discovered`: split projections; at-most-once-ever dedup; narrative
    completion without artifact; `running` forever.
  - `repaired` (S6): one `RunRecord.State` authority; retry semantics;
    artifact-gated completion; unified `choir run status`.
  - `repaired` (grand S1): progress deadline + cancel.
  - Naming repairs are grand S9.
