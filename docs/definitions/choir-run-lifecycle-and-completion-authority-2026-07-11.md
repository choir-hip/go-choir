# Choir Run Lifecycle and Completion Authority

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md
```

Read this document as executable semantic authority for unifying run lifecycle
state across the runtime, dispatcher, and trajectory projections, and for
enforcing artifact-verified completion on the processor path. It is **member 3**
of `docs/definitions/choir-run-truth-suite-2026-07-11.md` (Correctness) and the
concrete Phase 3 of the `choir-autoputer-cli-operability` spine (run truth /
G3 / G4).

**Prerequisites (suite order — do not chain-execute them from here):**

1. `docs/definitions/choir-run-deploy-unblock-2026-07-11.md` complete
2. `docs/definitions/choir-wire-store-conformance-2026-07-11.md` complete

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

After wire-store conformance dissolves C1/C2/C6, C4 and C5 remain the blocking
residue on the processor path. The fourth freeze mode (`running` forever) is
owned by suite member 1 (deploy unblock); this mission inherits that repair.

## Source Authority Order

1. This Definition.
2. `docs/definitions/choir-run-truth-suite-2026-07-11.md` (suite foliation).
3. `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md`
   (Phase 3 run truth, G3/G4, four-item external operator test, Introspection
   Contract).
4. `docs/definitions/choir-wire-store-conformance-2026-07-11.md`
   (prerequisite: world-wire store, sourcecycled durable ledger, no boot-time
   migration).
5. `docs/definitions/choir-run-deploy-unblock-2026-07-11.md`
   (prerequisite: progress deadline + `choir run cancel` + Deploy restored).
6. `docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`
   (C4/C5 evidence; original three freeze modes — this suite adds the fourth
   via deploy-unblock).
7. `docs/standing-questions.md` (Q5, Q6, Q9).
8. `AGENTS.md`, `docs/choir-doctrine.md`, `docs/runtime-invariants.md`.
9. Observed source: `internal/types/task.go`, `internal/types/trajectory.go`,
   `internal/runtime/runtime.go`, `internal/runtime/api.go`,
   `cmd/sourcecycled/main.go`, `cmd/choir`.

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
- **Progress deadline and `choir run cancel`** already exist from suite member 1
  (or must be completed there before this `/goal`). Do not re-implement unless
  staging shows regression.

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
- **No naming / rename sweeps** — owned by
  `choir-vocabulary-cutover-2026-07-11.md` (suite member 4). New code here must
  still be born in successor vocabulary (no new lease identifiers; speak *run*,
  not loop-as-product).
- **No continuation, parent/child, or result-channel deletion** — og-dolt B/C/E.
  Do not touch `run_continuations` / `/api/continuations` under this Definition.

## Dependency Truth (verified 2026-07-11; refresh at Phase 0)

- Wire-store conformance was **defined but not implemented** at authoring
  (`a2f110e`). Suite member 2 must complete before Phase D here. **Do not**
  chain-execute wire-store from this Definition — follow the suite index.
- `isTerminalRuntimeState` already treats `blocked` as terminal (`f1ceba5`).
  Phase B decides terminal-error vs retryable-dispatch routing.
- `RunningCountByProfile` already derives from
  `store.ListRunsByState(..., RunRunning, ...)`. Residue: (a) error path falls
  back to in-memory `RunningCount()`; (b) `RunPending` not counted; (c)
  `processorRunOccupiesAdmission` defaults to "occupies" on lookup error.
- Fourth freeze mode (`running` forever) is repaired by suite member 1.

## Open Decisions (defaults govern; deviations recorded in ledger)

- **Single authority:** `RunRecord.State` in `internal/runtime` is sole writer;
  other projections are read-only or derived. Re-derive the mission if the
  owner later selects a different authority.
- **Retry policy:** sourcecycled retries on the next poll, up to **3** attempts
  per cycle. No synchronous immediate retry.
- **Blocked-run timeout:** `RunBlocked` auto-cancels after **10 minutes** so
  cycle idempotency releases for retry (Phase B sweep). Admission must already
  treat all `blocked` as non-occupying (Phase A).
- **Progress deadline:** **60 minutes** — owned by suite member 1; inherit.
- **Artifact predicate:** published world-wire article/story with ingestion
  lineage in the `corpusd` store — only after wire-store conformance.

## Invariants

- `RunCompleted` has a fetchable artifact in the world-wire store (from Phase D).
- `RunBlocked`, `RunFailed`, and `RunCancelled` must not freeze admission and
  must not permanently consume cycle idempotency.
- `RunRunning` stays within its progress deadline (member 1).
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
   - `running` with expired progress deadline (member 1; re-verify here)
5. Duplicate concurrent submissions → `409 Conflict`; retries after
   failed/cancelled → accepted.
6. Member 1's Deploy restoration still holds (or is re-proven if regressing).

Naming residue is **not** a completion criterion (suite member 4).

## Sequencing and Gates

Order: Phase 0 → A → B → C (verify member 1) → D → E.

Every phase:

1. **Consensus** before mutation (`/tmp/choir-run-lifecycle-<phase>-consensus`).
2. **Local proof** (phase tests + `go build ./...` + `go vet ./...`).
3. **Landing loop** per `AGENTS.md`.
4. **QA** on staging; record IDs in the evidence ledger.
5. **Halt-on-red;** repair within phase (documentation-first) or roll back to
   the **rollback ref** = `git rev-parse HEAD` recorded at phase start before
   the first behavior commit. Do not leave staging red.
6. Do not start the next phase until acceptance is recorded.

Docs-only commits use the docs-only landing path.

## Execution Phases

### Phase 0 — Consensus on the plan (green/yellow)

- Confirm suite members 1–2 are complete on staging; if not, stop and point
  ACTIVE at the blocking member (do not chain-execute).
- Consensus on this Definition's phases and Open Decision defaults.
- Record ledger entry. No code mutation.

### Phase A — Authority alignment and derived admission (red)

- **Objective:** admission is a derived view of `RunRecord.State` with no
  in-memory fallback.
- **Changes:**
  - Remove silent fallback from `RunningCountByProfile` to `RunningCount()`;
    surface store errors. Count `RunPending` with `RunRunning`.
  - `processorRunOccupiesAdmission` returns `false` for `failed`, `cancelled`,
    and **all** `blocked` runs (blocked is terminal and must not occupy);
    lookup errors surface explicitly instead of defaulting to "occupies".
- **Local proof:** `go test ./internal/runtime -run 'Admission|RunningCount|Idempotency'`.
- **QA:** a deliberately blocked run is visible as `blocked` (API or
  `choir run status` if already sufficient), does not occupy admission, and a
  second independent request is admitted.
- **Rollback path note:** feature-flag retention of the legacy counter is
  allowed only as emergency rollback after A lands; the forward path removes
  the silent fallback.

### Phase B — Retryable ingestion idempotency (red)

- **Objective:** dedup distinguishes `succeeded already` from `failed before
  starting`; terminal errors release cycle idempotency.
- **Changes:**
  - `cmd/sourcecycled/main.go`: route terminal `blocked`/`failed`/`cancelled`
    to the retryable path with budget **3** per cycle. Implement the **10
    minute** blocked auto-cancel sweep for cycle release / retry eligibility.
  - `internal/runtime/api.go`: active fingerprint → `409`; resubmission after
    terminal error → new run.
- **Local proof:** `go test ./cmd/sourcecycled ./internal/runtime -run 'Terminal|Retry|Dedup|Conflict|BlockedTimeout'`.
- **QA:** simulated 429 → terminal state, capacity release, successful retry on
  next poll; duplicate concurrent submission → `409`.

### Phase C — Verify deploy-unblock still holds (yellow/red if regression)

- **Objective:** confirm member 1's deadline/cancel/Deploy proof still holds.
- **Changes:** none if green; if regressing, repair under member 1's authority
  (or a minimal patch here) before continuing.
- **QA:** `running_runs: 0` on the platform VM (or equivalent health) and Deploy
  not blocked by a stuck run.

### Phase D — Artifact-verified completion (red)

- **Gate:** wire-store conformance completion semantics already observed
  (suite member 2). If not, **stop** and run that `/goal` from ACTIVE — do not
  chain-execute from this document.
- **Changes:** processor `RunCompleted` requires the artifact predicate; clean
  exit without artifact → `RunFailed`.
- **Local proof:** `go test ./internal/runtime -run 'Artifact|Completion'`.
- **QA:** publish → `completed` with fetchable receipt; suppressed publish →
  `failed`.

### Phase E — Unified `choir run status` and lifecycle proof (red)

- Extend `choir run status <id>` to print unified state (artifact receipt after
  D).
- Land four freeze-mode regression tests if not already present.
- Consensus on full mission diff (code-review frame).
- Deployed acceptance: four-item external operator test from autoputer-cli.
- Update this Definition's state, `docs/ACTIVE.md`, evidence ledger.
- Suite then advances to member 4 (vocabulary) — not part of this completion.

## Follow-on Missions

- Suite member 4: `choir-vocabulary-cutover-2026-07-11.md`
- Autoputer-cli later phases (deploy receipts, promotion, keys)
- Autopaper editorial / reconciler artifact verification
- og-dolt B/C/E deletion families (continuation, parent/child, H025, …)

## Supersession Record

- Depends on suite members 1–2 and `choir-autoputer-cli-operability`.
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
- **Rollback path:** revert to pre-phase SHA; emergency feature-flag for legacy
  admission counter only after A has removed the silent fallback.
- **Heresy delta:**
  - `discovered`: split projections; at-most-once-ever dedup; narrative
    completion without artifact; `running` forever (member 1).
  - `repaired` (this mission): one `RunRecord.State` authority; retry
    semantics; artifact-gated completion; unified `choir run status`.
  - `repaired` (member 1): progress deadline + cancel.
  - Naming repairs are member 4, not here.
