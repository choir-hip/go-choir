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
- No promotion, route-slot, or key-scoping work. Those are later phases of the
  `autoputer-cli` spine. VM provisioning/lifecycle is likewise out of scope,
  **except** run-level cancel/drain: cancelling a stuck run so that its VM's
  admission capacity is released is run-lifecycle work and is in scope
  (see Phase C). Reprovisioning, resizing, or replacing VMs is not.
- No changes to the `universal-wire` feed UI beyond the read path owned by
  `choir-wire-store-conformance`.

## Dependency Truth (verified 2026-07-11)

Sequencing claims in this Definition are pinned to observed repository state,
not to the state assumed when the post-mortem was written:

- `choir-wire-store-conformance-2026-07-11.md` is **defined but not
  implemented** (definition commit `a2f110e`; no code commits). Therefore any
  phase whose acceptance requires a fetchable artifact in the `corpusd`
  world-wire store is gated until that mission's completion semantics are
  observed; the Phase D gate self-resolves by executing that mission first.
  Phases A–C below have no such dependency and are executable now.
- `isTerminalRuntimeState` in `cmd/sourcecycled/main.go` **already** treats
  `blocked` as terminal (`f1ceba5`). The remaining work is not adding the case
  but deciding terminal-error vs retryable-dispatch routing (Phase B).
- `RunningCountByProfile` in `internal/runtime/runtime.go` already derives from
  `store.ListRunsByState(ctx, types.RunRunning, ...)`. The residue is: (a) the
  error path falls back to the in-memory `RunningCount()` map; (b) `RunPending`
  is not counted; (c) `processorRunOccupiesAdmission` defaults to "occupies"
  on any lookup error. Phase A targets this residue, not a wholesale rewrite.
- The run that froze `vm-universal-wire-platform` (`running_runs: 1`,
  documented in `docs/ACTIVE.md` "Remaining Error") is in state **`running`**,
  not `blocked`. None of the post-mortem's three freeze modes covers a run
  that stays `running` forever. This Definition adds it as the fourth freeze
  mode and Phase C repairs it.

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
- **Running-run lease/timeout:**
  - *Default:* a `RunRunning` run carries a lease (heartbeat or max-duration
    bound, default 60 minutes for processor runs). A run whose lease expires is
    transitioned to `RunFailed` by the runtime sweep and releases capacity.
    `choir run cancel <id>` is exposed as the operator drain path and follows
    the same capacity-release semantics.
- **Artifact predicate for the processor:**
  - *Default:* the required artifact is a published world-wire article/story
    route with ingestion lineage in the `corpusd` store. **This predicate is
    only enforceable after `choir-wire-store-conformance` is settled**; until
    then Phase D is gated (see Dependency Truth).

## Invariants

- A run record in state `RunCompleted` must have a fetchable artifact in the
  world-wire store (enforced from Phase D onward).
- `RunBlocked`, `RunFailed`, and `RunCancelled` must not freeze admission
  capacity and must not permanently consume cycle idempotency.
- A `RunRunning` run must hold a live lease; an expired lease transitions the
  run to a terminal error state that releases capacity. No run may occupy
  admission capacity indefinitely without observable progress.
- Admission capacity is derived from `RunRecord.State` (`RunPending` +
  `RunRunning` with live lease), not from an in-memory or separately maintained
  counter, including on the error path (no silent fallback to the in-memory
  map).
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
4. The four freeze modes (`blocked` with no continuation, `passivated` with
   live trajectory, `completed` with live trajectory, and `running` with an
   expired lease) are reproduced as regression tests and resolve under the
   single authority.
5. Duplicate concurrent submissions for an active run are rejected with
   `409 Conflict`, while retries for failed/cancelled runs are accepted and run.
6. The stuck run on `vm-universal-wire-platform` (documented in
   `docs/ACTIVE.md` "Remaining Error") is drained via the Phase C cancel/lease
   path, `running_runs` returns to 0, and the previously bypassed
   `Deploy to Staging (Node B)` hot-refresh verifies a new commit on the next
   runtime-package push.

## Sequencing and Gates

Execution order is Phase 0 → A → B → C → D → E. Phases A–C have no dependency
on `choir-wire-store-conformance`; Phase D is gated on it, and the gate
self-resolves by chain-executing that mission first (see Phase D and
Dependency Truth). Every phase lands through the same gate protocol:

1. **Consensus gate (before mutation):** run the agentic-consensus runner
   (`skill://agentic-consensus/agentic-consensus-runner.sh`) on the phase's
   planned diff with the planning/adversarial prompt frames, out-dir
   `/tmp/choir-run-lifecycle-<phase>-consensus`. Adjudicate findings; a severe
   blocking finding re-plans the phase and re-runs consensus. Preserve prompt
   and outputs; record an evidence-ledger entry.
2. **Local proof:** targeted `go test` packages for the phase, then
   `go build ./...` and `go vet ./...`.
3. **Landing loop (per `AGENTS.md`):** commit → push `origin main` → monitor
   the Actions run for that SHA → monitor the staging deploy → verify staging
   commit identity via `/health`.
4. **QA verification (deployed acceptance):** run the phase's acceptance
   probe against staging and record run/CI IDs, staging identity, and probe
   output in the evidence ledger. A phase is not complete — and the next phase
   must not start — until its acceptance is recorded.
5. **Halt conditions:** a failed consensus adjudication, a red CI run, a
   staging identity mismatch, or acceptance-probe failure stops the sequence.
   Diagnose and repair within the phase (documentation-first per `AGENTS.md`)
   or roll back via the phase's rollback ref; do not advance past a red gate.

Docs-only commits (this Definition, `docs/ACTIVE.md` updates) follow the
docs-only landing path and must not force the full deploy workflow.

## Execution Phases

### Phase 0 — Consensus on the whole plan (green/yellow)

- Run the agentic-consensus runner on this Definition's full phase plan,
  invariants, and Open Decision defaults (planning + adversarial frames).
- Adjudicate. If a severe finding changes the plan or a default decision,
  update this Definition, note the change in the Supersession Record, and
  re-run consensus on the changed plan.
- Record the consensus evidence-ledger entry. No code mutation in this phase.

### Phase A — Authority alignment and derived admission (red)

- **Objective:** make admission capacity a derived view of `RunRecord.State`
  with no in-memory fallback, per the five-projection inventory.
- **Changes:**
  - `internal/runtime/runtime.go` / `internal/runtime/api.go`: remove the
    silent fallback from `RunningCountByProfile` to the in-memory
    `RunningCount()` map; surface store errors instead. Count `RunPending`
    alongside `RunRunning`.
  - Make `processorRunOccupiesAdmission` a predicate over the derived view
    returning `false` for `failed`, `cancelled`, and timed-out `blocked` runs;
    remove the "occupies on any lookup error" default in favor of an explicit
    error surface.
- **Local proof:** `go test ./internal/runtime -run 'Admission|RunningCount|Idempotency'`.
- **QA acceptance:** on staging, a deliberately blocked run is visible as
  `blocked` in `choir run status`, does not occupy admission, and a second
  independent request is admitted.

### Phase B — Retryable ingestion idempotency (red)

- **Objective:** dedup distinguishes `succeeded already` from `failed before
  starting`; terminal-error runs release cycle idempotency.
- **Changes:**
  - `cmd/sourcecycled/main.go`: route terminal `blocked`/`failed`/`cancelled`
    dispatch states to the retryable path with the bounded retry budget
    (default 3 attempts per cycle; Open Decisions). `isTerminalRuntimeState`
    already classifies `blocked` as terminal (`f1ceba5`); this phase decides
    what terminal means for redispatch.
  - `internal/runtime/api.go`: duplicate submission for a concurrently active
    fingerprint returns `409 Conflict`; resubmission after a terminal error is
    admitted as a new run.
- **Local proof:** `go test ./cmd/sourcecycled ./internal/runtime -run 'Terminal|Retry|Dedup|Conflict'`.
- **QA acceptance:** on staging, a simulated provider 429 produces a terminal
  error state, capacity release, and a successful sourcecycled retry on the
  next poll; a duplicate concurrent submission returns `409`.

### Phase C — Running-run lease and operator drain (red)

- **Objective:** no run occupies capacity forever; the current stuck run is
  drained.
- **Changes:**
  - `internal/runtime/runtime.go`: add the run lease (Open Decisions default:
    60-minute processor bound) and a sweep that transitions expired-lease
    `RunRunning` runs to `RunFailed` with capacity release.
  - `cmd/choir`: add `choir run cancel <id>` calling the same transition.
- **Local proof:** `go test ./internal/runtime ./cmd/choir -run 'Lease|Cancel|Sweep'`.
- **QA acceptance:** drain the stuck run on `vm-universal-wire-platform` via
  `choir run cancel` (or the sweep); observe `running_runs: 0`, then push a
  runtime-package change and observe `Deploy to Staging (Node B)` hot-refresh
  verify the new commit — the first green Deploy since the `d8fe4336` CI
  bypass. Record run IDs and the Deploy run URL.

### Phase D — Artifact-verified completion (red; gated, self-resolving)

- **Gate:** `choir-wire-store-conformance-2026-07-11.md` completion semantics
  observed on staging (world-wire store on `corpusd` serving the wire read
  path). The gate is **self-resolving**: if not yet true when Phase D is
  reached, the executing agent invokes and completes
  `docs/definitions/choir-wire-store-conformance-2026-07-11.md` under that
  Definition's own authority, gates, and red-class ceremony — it is the
  declared Phase 0 of the autoputer sequence and the confirmed next
  executable focus in `docs/ACTIVE.md` — then resumes Phase D here. Phases
  A–C remain landed value regardless of the chained mission's outcome.
- **Changes:** terminal `RunCompleted` for the processor path requires the
  artifact predicate (published world-wire article with ingestion lineage in
  the `corpusd` store); a run that exits cleanly without the artifact
  terminates `RunFailed`.
- **Local proof:** `go test ./internal/runtime -run 'Artifact|Completion'`.
- **QA acceptance:** on staging, a processor run that publishes is
  `completed` with a fetchable artifact receipt; a processor run whose
  publication is suppressed terminates `failed`.

### Phase E — Unified `choir run status` truth and final proof (red)

- **Changes:** extend `choir run status <id>` (`cmd/choir/main.go`) to print
  the unified state: run state from the single authority, trajectory summary,
  work items, and artifact receipt (receipt only after Phase D).
- Add the four freeze-mode regression tests if not already landed in A–C.
- **Final consensus gate:** run the agentic-consensus runner on the full
  mission diff (code-review frame).
- **Landing:** full landing loop, then the four-item external operator test
  from the `autoputer-cli` spine as the deployed acceptance proof.
- Update this Definition's state, `docs/ACTIVE.md` (including removing the
  "Remaining Error" section once Phase C's acceptance holds), and the
  evidence ledger.

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
- Amended 2026-07-11 (post-`d8fe4336`): added Dependency Truth, the fourth
  freeze mode (`running` with expired lease), the running-run lease/timeout
  Open Decision, per-phase Sequencing and Gates (consensus → push to main →
  QA acceptance), the phased execution plan A–E with the Phase D gate on
  `choir-wire-store-conformance`, and the autonomous execution contract. The
  original "First Phase" section is superseded by Phases A and B.
- Amended again 2026-07-11 (owner directive): the mission is fully autonomous.
  The Phase D gate self-resolves by chain-executing
  `choir-wire-store-conformance`; consensus-vs-default conflicts are
  self-adjudicated with a recorded deviation; unrepairable halts end the
  attempt with a documented failure report instead of an owner pause.

## Red-Class Ceremony

- **Mutation class:** Green/yellow for this document; the code changes this
  Definition authorizes touch red surfaces (run acceptance, canonical writes,
  sourcecycled ledger, CLI command surface) and must be executed with full
  red-class ceremony — concretely, the per-phase gate protocol in
  "Sequencing and Gates" (consensus gate → local proof → landing loop → QA
  acceptance → halt-on-red).
- **Autonomous execution contract (owner-ratified 2026-07-11):** the entire
  mission is executable autonomously with no human turn. The consensus gates
  are the in-loop review mechanism. The former escalation conditions are
  converted to document-and-proceed rules:
  - (a) An adjudicated consensus finding that contradicts an Open Decision
    default is **self-adjudicated**: the executing agent picks the better-
    evidenced option, records the deviation and rationale in the Supersession
    Record and evidence ledger, and proceeds. A deviation is not a halt.
  - (b) A halt condition that cannot be repaired within the phase ends the
    mission attempt with a **documented failure report** (documentation-first
    per `AGENTS.md`): pushed SHA, failing gate, diagnosis, rollback refs, and
    next safe probes recorded in `docs/ACTIVE.md` and the evidence ledger.
    Failure is an accepted outcome; the record is the learning artifact. Do
    not leave staging red — roll back to the phase's rollback ref first.
  - (c) The Phase D gate self-resolves by chain-executing
    `choir-wire-store-conformance` (see Phase D) instead of pausing.
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
    - agent completion narrated without a required artifact;
    - a `running` run with no lease freezing admission and the staging deploy
      indefinitely (the fourth freeze mode; `docs/ACTIVE.md` Remaining Error).
  - `repaired`:
    - one `RunRecord.State` authority with read-only projections;
    - retry semantics that distinguish `succeeded already` from
      `failed before starting`;
    - a run lease and operator cancel so no run occupies capacity forever;
    - terminal completion requires a fetchable artifact in the world-wire store.
