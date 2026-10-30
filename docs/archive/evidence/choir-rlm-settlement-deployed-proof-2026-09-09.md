# Settlement Gate Deployed Proof and Telemetry — 2026-09-09

Class: green (evidence only).
Authority: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md` (finish.acceptance items 7 and 8).

## 1. Deployment and Environment Identity

- **Pushed Commits:**
  - `d133fa9a` green(docs): item-1 code-free Define receipt with Yaegi isolation matrix
  - `b8aaa89b` red(settlement-item1): typed reuse disposition with Compile/Execute split and loop/broker carry
  - `8fd4dc6f` green(docs): item-2 code-free Define receipt for RLM fallback removal
  - `a6b898f0` red(settlement-item2): delete RLM session-spawn fallback, typed worker diagnostic
  - `8370b121` green(docs): item-3 code-free Define with frozen v1 identity receipt
  - `965e26a7` red(settlement-item3): terminal identity contract, proposition digest, slot gate, and supersede tuple
  - `f320e4fe` green(docs): item-4 code-free Define receipt for narrow admission grammar
  - `18498447` red(settlement-item4): narrow assigned-CoSuper admission grammar, sequential execution, and toolloop decoupling
  - `4fdd02d4` green(docs): item-5 code-free Define receipt for resumable fate saga
  - `0921c542` red(settlement-item5): resumable fate saga with atomic final boundary after revoke acknowledgement
  - `e28d6be8` green(docs): item-6 code-free Define receipt for fallback canonical-author port closure
  - `f5cfc206` red(settlement-item6): fallback canonical-author port closure, reducer orphan observation, and obligation routing
  - `5022c6b7` red(yaegikernel): execute compiled program directly and guard result read on failure to eliminate timeout data race
  - `6b758878` red(agentcore): increase trajectoryActivationDrainTimeout to accommodate large scale trajectory drains
  - `0c71d1f9` red(settlement-gate): post-consensus completion adjudications, immutable acceptance manifest, and cutover closure
  - `0475ed84` red(settlement-gate): round-2 closure - orphan replay by report identity, producer test, sealed manifest, registry reconciliation
- **CI Status:**
  - Workflow: GitHub Actions CI run `34408184941`
  - Result: `completed success` (superset of prior green run `34401118732`)
  - All CI gates passed: Plan CI Lanes, Heresy Detector, Go Vet + Build, Docs Truth Check, Build Differential SBOM Candidate, Detect Staging Deploy Impact, all 6 `agentcore/textureowner` shards (race detector), all 6 `non-runtime` shards (race detector), isolated scale tests, Publish Rolling Flake, and Deploy to Staging (Node B).
- **Staging Verification:**
  - URL: `https://choir.news/`
  - Response: `HTTP/2 200 OK`
  - Headers:
    - `x-choir-build-commit: 0475ed84cc2bb380dc8df69ab368e443a969a207`
    - `x-choir-build-service: proxy`
    - `via: 1.1 Caddy`
    - `date: Wed, 09 Sep 2026 22:07:34 GMT`
- **Target Computer Identity (consumed read-only from mission 0):**
  - Computer: `computer-03335285269bdba4f94377e56879f9e6`
  - Realization Epoch: 890
  - Route: vmctl routing enabled
  - Effects Mode: OFF (pre-A checkpoint `99949fe2` remains untouched as self-development fence).

## 2. Settlement Gate Acceptance Scenario Verification

| Scenario | Subsystem | Observed Result | Contract Satisfied |
|---|---|---|---|
| **S1: Isolation Matrix & Compile Gate** | `yaegikernel` | Body syntax, type, and runtime failures classify unsafe-to-reuse (`ReuseUnsafeToReuse`); compile gate verifies failed compile leaves heap untouched; timeout race eliminated. | Only proven non-mutating preflight preserves heap; one typed diagnostic contract. |
| **S2: Single Execution Route** | `capsule-broker` | `fallbackGoEval` deleted from active RLM route; session worker spawn failures return typed worker diagnostics (`ExitCode: 1, DiagKind: worker`) and never divert to one-shot worker. | One execution path and one diagnostic contract on active RLM route; tools rollback path intact. |
| **S3: Terminal Identity & Replay** | `store`, `agentcore` | Proposition digest covers canonical claims (`result`, `verdict`, ordered `commands`, ordered `outputs`, sorted unique `evidence_refs`, pinned belief); `ReportID` derived deterministically. Same proposition under fresh toolCallID, reworded summary, or fresh command envelope replays original receipt with zero new effects. | Terminal settlement is one durable truth per attempt; provider metadata excluded from identity. |
| **S4: Slot Conflict Gating** | `store` | Differing proposition on an already-reserved or completed attempt strictly returns `ErrCoSuperAssignmentCommandConflict`. Attempt 1 cannot carry supersede tuple; attempt > 1 requires valid supersede tuple (`CoSuperSupersedeTuple`). | No second terminal row for one attempt; rejections never smuggle a correction. |
| **S5: Narrow Admission Grammar** | `toolregistry` | `capsule_go_eval` and `record_assignment_result` execute sequentially. Pre-dispatch statically refuses $\ge 2$ terminals, $> 1$ eval, reversed order, and forbidden companions with none run. Mailbox `choir.Complete` / `IntentComplete` never aborts an admitted JSON terminal: `[stage_complete eval, completed terminal]` executes both sequentially. `toolloop.go:622` decouples singleton lock-in for admitted shape (a) and (b). | Contradictory turns never execute; settlement is a reducer property, not a batch shape accident. |
| **S6: Resumable Fate Saga** | `agentcore`, `store` | `PendingProposal` committed to Dolt during `FreezeRequested` before physical freeze. Physical revoke acknowledgement strictly precedes terminal report commitment. `SlotTerminalReport` blocks competing proposals while pending. Interrupted runs resume through saga without orphaning. | Atomic final boundary after durable revoke acknowledgement; no terminal truth published before revoke. |
| **S7: Single Reducer Author (Orphan Port)** | `agentcore`, `store` | `researcher_checkpoint_fallback.go` barred from synthesizing terminal updates or wakes for assignment runs. `RecordCoSuperOrphanObservation` store command enables reducer alone to close unassigned terminated runs with a failed report; pending slots route to fate saga. | Terminal truth has one author; delegation closes deterministically without a second committer. |

## 3. Immutable Acceptance Manifest and Sealed Scenario Receipts

Shared deployment identity for every scenario row: staging computer
`computer-03335285269bdba4f94377e56879f9e6`, realization epoch 890, deployed
build `0475ed84cc2bb380dc8df69ab368e443a969a207`, CI run `34408184941`
(all green; superset of prior green run `34401118732` at `6b758878`), staging
`HTTP/2 200 OK` with `x-choir-build-commit: 0475ed84...` (Section 1). No live
assignment attempt was opened on the staging computer; per-scenario execution
evidence is the named contract test (run in CI `34408184941`) against the named
repair commit. Attempt/run/capsule bindings below are the testcase fixture
identities exercised by those verifiers, not staging attempts.

| Scenario ID | Repair Commit | Verifier (resolvable) | Contract Verified |
|---|---|---|---|
| `SCENARIO-S1-COMPILE-ISOLATION` | `b8aaa89b`, `5022c6b7` | `internal/yaegikernel`: `TestSessionCompileRejectionPreservesHeap`, `TestSessionLoopSurvivesCompileRejection`, `TestClassifyExecuteError`, `TestSessionFailureKinds` | Failed compile preserves heap/imports; typed failure classes; no timeout race |
| `SCENARIO-S2-SINGLE-ROUTE` | `a6b898f0` | `cmd/capsule-broker`: `TestGoEvalSessionFailsClosedWithoutBinary`, `TestInitSessionFailsCleanWithoutBinary` | One execution route; typed worker diagnostic on spawn failure; no one-shot diversion |
| `SCENARIO-S3-TERMINAL-IDENTITY` | `965e26a7` | `internal/store`: `TestTerminalPropositionDigestExcludesMetadata`, `TestTerminalSlotReplayAndConflict` | Proposition digest over canonical claims only; same digest replays with zero new effects |
| `SCENARIO-S4-SLOT-CONFLICT` | `965e26a7` | `internal/store`: `TestCoSuperAssignmentCommandsReplayAndDigestConflict`, `TestOpenSupersedeTuple` | Differing proposition conflicts; supersede tuple required for attempt > 1 |
| `SCENARIO-S5-ADMISSION-GRAMMAR` | `18498447`, `0c71d1f9` | `internal/toolregistry`: `TestExecuteToolBatchAssignedCoSuperAdmissionGrammar` (test 8: `[stage_complete eval, completed terminal]` executes sequentially) | Sequential execution; pre-dispatch refusal; mailbox complete never aborts admitted terminal |
| `SCENARIO-S6-FATE-SAGA` | `0921c542` | `internal/store`: `TestCoSuperPendingProposalDurabilityAndAtomicRevokeFinality` | Durable pending proposal; revoke acknowledgement precedes terminal commit |
| `SCENARIO-S7-ORPHAN-REDUCER` | `f5cfc206`, `0475ed84` | `internal/store`: `TestRecordCoSuperOrphanObservation` (close, pending-conflict, bound-run mismatch, terminal retry replay); `internal/agentcore`: `TestFallbackRecordsOrphanObservationOnAssignedTerminalRun`, `TestFallbackAbstainsOnAssignmentRun` | Fallback synthesizes nothing on assignment runs; reducer closes orphans with bound-run validation and replay |

## 4. Residual Debt & Open Residues

- **R1 - Mission-0 live failure-injection drill (blocked):**
  Stays mission-0-owned per owner direction 2026-09-09; neither implemented nor waived here.
- **R6 - Cutover remainder-holder status retained:**
  Target cutover definition retained as remainder holder; settlement acceptance closed atomically under this mission.
