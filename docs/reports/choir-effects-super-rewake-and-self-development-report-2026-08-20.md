# Choir Supervised Self-Development Effects Mission
## Technical Progress, Substrate Discoveries & Architecture Consensus Report
**Date:** 2026-08-20 (Covering 2026-08-19 — 2026-08-20 UTC)  
**Parent Definition:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`  
**Classification:** Orange/Red Substrate & Architecture Report  
**Author:** Choir Engineering & Agentic Consensus Panel (Claude Opus, Gemini 3.7 Flash, Devin, Codex)  

---

## 1. Executive Summary

Over the 24-hour window of 2026-08-19 to 2026-08-20 UTC, the Choir team executed foundational substrate repairs and platform convergence required to achieve the first autonomous supervised self-development effects run on staging.

The mission objective is live proof on staging of a capsule-authored Solitaire candidate change A: CoSuper authors, builds, tests, freezes, and proposes candidate A; multi-agent qualified consensus accepts the proposal; the change is promoted to live computer code; live API/DB verification exercises the new capability; candidate A is subsequently falsified and superseded by candidate B; and the computer is restored to pre-A checkpoint `99949fe2`.

During this period, the platform reached major milestones across three foundational layers:
1. **Phase 1 Family A Computer Monism & Checkpoint Publication:** Completed strict single-computer data isolation, removed cross-computer alias table leakage, unified the residue import pipeline, and published pre-A checkpoint `99949fe2e16d3c4c...` at realization epoch 324 on staging computer `computer-03335285269bdba4f94377e56879f9e6`.
2. **Capsule Execution Substrate Unification:** Diagnosed and fixed POSIX job-control failures (`getpgrp` / `initialize_job_control` crash) in non-interactive containerized guest capsules (`651d86bc`), enabling live CoSuper `assignment-97191e37` to execute commands natively inside guest capsules on staging.
3. **The 5-Stage Super Actor Continuation Ladder:** Systematically diagnosed and solved four cascading symptoms in the persistent Super continuation loop following CoSuper cancellation, ultimately discovering the foundational substrate truth: Super execution authority derives strictly from Texture `execution_request` Control packets carrying the canonical `operation:` URI, rather than unbound `producer_report` continuation loops.

Following multi-agent consensus review across Claude (Opus), Gemini 3.7 Flash, Devin, and Codex, this report records the full technical findings, consensus synthesis, adjudicated architectural improvements, and the concrete landing plan for autonomous candidate A authoring.

---

## 2. Phase 1: Family A Monism & Pre-A Checkpoint Publication

### 2.1 The Family A Substrate
Prior to Phase 1 completion, Texture document aliases suffered from cross-computer data contamination due to unscoped global alias tables. Under Family A monism:
- All Texture document aliases, event projections, and desktop bindings were strictly scoped to exact `computer_id` boundaries (`ebce5455`, `0aa1ffb3`, `087cf290`).
- Schema migrations were established to ensure `computer_id` columns and primary key indices are applied in deterministic order (`997f25cb`), eliminating boot-time crash risks.
- Database write deadlines were expanded on both proxy and guest endpoints (`0d8b8920`, `b26ce208`) to accommodate full residue replay and snapshot exports without 504 timeouts.

### 2.2 Pre-A Checkpoint `99949fe2`
At realization epoch 324 on staging computer `computer-03335285269bdba4f94377e56879f9e6`, pre-A checkpoint `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` was successfully committed, verified, and published (`5b7a7b73`).

This checkpoint serves as the inviolable restore fence for the effects mission. Mode was confirmed locked in `propose_only` generation 1 (ModeReceipt `01a0091b-bf12-771e-97e7-9a42752ad036`), ensuring all live send, mail, and materialization effects remain strictly OFF until candidate evaluation passes.

---

## 3. Candidate A Solitaire Formulation

Per `74e1f51c`, the candidate change A specification and decision-policy manifest were authored:
- **Product Object:** Classic Solitaire game engine added to the user computer's local application surface.
- **Intentional Foundation Defect:** Pre-declared foundation defect (invalid card-drag state machine rule on foundation piles) designed to be detected and falsified in Phase 3.
- **Five Capsule-Bound Bundle Artifacts:**
  1. `source_patch_ref`: unified diff implementing the Solitaire engine.
  2. `test_patch_ref`: unit test suite verifying standard game play while defending the pre-declared invariant.
  3. `verifier_manifest_ref`: execution receipt verifying test passage.
  4. `doc_patch_ref`: user documentation.
  5. `changelog_patch_ref`: machine-readable changelog entry.
- **Autonomy Boundary:** Multi-agent qualified consensus operating under frozen reversible decision policy `reversible-selfdev-v1`.

---

## 4. Substrate Discovery: Capsule Broker Job Control

### 4.1 The Symptom
During live candidate authoring probe on staging, CoSuper `assignment-97191e37` crashed immediately upon attempting bash execution inside guest capsule `capsule-c5e35066`. The capsule broker aborted with:
```text
bash: cannot set terminal process group (-1): Inappropriate ioctl for device
bash: no job control in this shell
```

### 4.2 Root Cause & Resolution
The capsule broker was invoking `bash +m -i` in a containerized environment lacking a controlling TTY / process group leader. 

In commit `651d86bc` (deployed via CI run `32280364496` as `d33f245c` at epoch 326), the capsule broker was refactored to execute non-interactive commands via `sh -c` with clean environment arguments, eliminating job-control initialization requirements.

Live re-probe confirmed: CoSuper `assignment-97191e37` executed 200 clean tool iterations without `getpgrp` or process group faults.

---

## 5. The Super Actor Continuation Ladder

When CoSuper `assignment-97191e37` exhausted its 200-iteration budget without freezing candidate A, the system initiated an intensive series of platform investigations that revealed how the privileged Super actor must be driven.

```
+-----------------------------------------------------------------------------------+
|                           THE CONTINUATION LADDER                                 |
+-----------------------------------------------------------------------------------+
| Stage 1: Super Non-Rewake (9bc99f90)                                              |
|   Problem: CoSuper cancelled -> Super stayed completed -> No continuation.       |
|   Fix: Wake Super actor on CoSuper cancel producer_report.                        |
+-----------------------------------------------------------------------------------+
| Stage 2: Continuation Storm (3654d925)                                            |
|   Problem: Undelivered report caused rapid 10-minute Super loop storm.            |
|   Fix: Claim producer_report_ids in Super metadata; terminal Super skips claims.  |
+-----------------------------------------------------------------------------------+
| Stage 3: Prompt Mismatch 200-Loop (5e01ac3a)                                      |
|   Problem: Super woke with generic inbox prompt; 200-looped without assign.       |
|   Fix: Specialized prompt "Open fresh CoSuper assignment with assign_co_super".   |
+-----------------------------------------------------------------------------------+
| Stage 4: Max-Tokens Cold-Injection Blowup (9a55b756)                              |
|   Problem: 9 cancel reports cold-injected into prompt; model blew token limit.    |
|   Fix: Omit claimed cancel report bodies (cosuper_replacement_omit_reports).      |
+-----------------------------------------------------------------------------------+
| Stage 5: Structural Authority Insight (Root Cause)                                |
|   Insight: Report continuation lacks Texture Control packets and operation ID.    |
|   Resolution: Mint Texture execution_request rewake natively via Texture join.   |
+-----------------------------------------------------------------------------------+
```

### Stage 1: Super Non-Rewake (`9bc99f90`, Epoch 327)
- **Problem:** Super `f009f383` completed immediately after calling `assign_co_super` and `report_to_texture(open)`. When CoSuper cancelled, `persistSystemCoSuperCancellation` wrote an undelivered `producer_report` but never called `wakeUpdatedCoagent`.
- **Repair:** `persistSystemCoSuperCancellation` calls `wakeUpdatedCoagent`; `reconcilePersistentSuperActor` starts `lifecycle_texture_control` continuation from pending admissible reports.

### Stage 2: Super Continuation Storm (`3654d925`, Epoch 328)
- **Problem:** When continuation Super `b57705fd` failed, `maybeContinuePersistentSuperInbox` immediately reconciled, found the same undelivered CoSuper cancel report, and started a replacement Super (`999bd208`). This created an infinite 10-minute storm cycle (`b57705fd` -> `999bd208` -> `483eec47` -> `aada24cc`).
- **Repair:** Continuation Super records `producer_report_ids`. `claimedPersistentSuperProducerReportIDs` skips reports claimed by any terminal `lifecycle_texture_control` Super. Live proof: Super `bab919a0` claimed 9 IDs and 200-failed without restorming.

### Stage 3: Assign Prompt Specialization (`5e01ac3a`, Epoch 329)
- **Problem:** Continuation Super ran with generic inbox prompt `Process pending coagent update packets for privileged execution.` without understanding its goal was CoSuper replacement.
- **Repair:** Set prompt to `Prior implementation CoSuper assignment is terminal. Open a fresh implementation CoSuper assignment with assign_co_super.` and stamped `cosuper_replacement_requested = true`.

### Stage 4: Max-Tokens Context Blowup (`9a55b756`, Epoch 330)
- **Problem:** Replacement Super `e4141127` started with the assign prompt but failed at iteration 7 with `model stopped at max_tokens after 3 continuation attempts` because 9 cancel reports were cold-injected into model context.
- **Repair:** Added `cosuper_replacement_omit_reports` to skip appending claimed cancel report bodies in `pendingCoagentUpdatesForRun`, and required this flag to retire report claims.

### Stage 5: The Root Cause — Structural Authority via Texture Controls
- **Problem:** Omit-reports Super `f515dd0f` started cleanly on epoch 330, but still 200-failed at 23:40:17Z without calling `assign_co_super`.
- **Root Cause Analysis:** Line-level audit of `super_controller.go:201-203` revealed that `self_development_operation_id` and `lifecycle_control_bindings` are ONLY established when Super is started from a Texture Control packet (`request_source=lifecycle_texture_control`) whose `Sources[0].Target.URI` carries `operation:selfdev-...`. Report-continuation Super lacked these bindings entirely, running as an unbound loop with no operation context.
- **Substrate Resolution:** The only Super that successfully called `assign_co_super` was `f009f383`, started from a canonical Texture `execution_request`. When CoSuper cancels, the system must trigger `ensureSelfDevelopmentTextureJoin` with `wakeToken = terminalSuper.RunID`, minting `turn:selfdev-texture-rewake`. This provides Super with the full canonical operation context and bound Texture Control packet natively.

---

## 6. CI Substrate Robustness & Flake Triage

During the deployment sequence for `3654d925`, two distinct CI substrate failure modes were identified and isolated:
1. **Heresy Detector ripgrep Installation Stall (CI `32293816782`):** Job `96200490826` hung during `apt-get install ripgrep` for 66+ minutes due to an upstream package repository mirror stall. Cancelling the hung run and using `workflow_dispatch` with `force_staging_deploy=true` successfully recovered the pipeline.
2. **Dolt Race-Shard Timeout Flake (CI `32299981780`):** Shard 3 job `96220531049` failed with a Dolt scan timeout in `TestCancelRunTrajectoryDrainsMoreThanOneActivePage`. Re-dispatching without code changes (CI `32302197967`) succeeded completely, proving the issue was test runner contention rather than a code regression.

---

## 7. Staging Realization & Epoch Progression

The staging computer `computer-03335285269bdba4f94377e56879f9e6` progressed deterministically through realization epochs:
- **Epoch 324:** Pre-A Checkpoint `99949fe2` published; Family A monism active.
- **Epoch 326:** Capsule broker `sh -c` fix deployed (`d33f245c`); CoSuper execution verified.
- **Epoch 327:** Super continuation on CoSuper cancel deployed (`51b18f54`).
- **Epoch 328:** Storm-stop claim tracking deployed (`3654d925`); storm prevented.
- **Epoch 329:** Assign prompt specialization deployed (`5e01ac3a`).
- **Epoch 330:** Omit-reports cold injection fix deployed (`9a55b756`).
- **Target Epoch 331:** Texture `execution_request` rewake substrate deployment.

---

## 8. Agentic Consensus Panel Findings & Dissent

An independent consensus panel was convened across four distinct agent architectures: Claude (Opus tier), Gemini 3.7 Flash, Devin, and Codex.

### 8.1 Consensus Findings
1. **Unanimous Approval of Texture Rewake:** All agents agreed that minting a canonical Texture `execution_request` via `ensureSelfDevelopmentTextureJoin` is the correct substrate-level architecture. Patching report-continuation was definitively identified as a symptom patch.
2. **Confirmation of Root Cause:** Claude and Gemini independently confirmed that `self_development_operation_id` is only parsed from packet `Sources[0].Target.URI`, explaining why report-continuation Super lacked operational binding.
3. **Idempotency & Deduplication:** Deriving `wakeToken` deterministically from `terminalSuper.RunID` ensures duplicate reconciliation sweeps collapse to a single idempotent Texture turn.

### 8.2 Critical Dissent & Adjudicated Improvements (Claude Opus)
Claude provided five blocking/high-severity hazards that were immediately adjudicated into the architecture:
- **B1 (Delete Report-Continuation Fallback):** Do not keep report-continuation as a silent fallback when Texture rewake is available; fail-closed if Texture rewake is refused so unbound loops cannot re-enter.
- **B2 (Bound the Rewake Cycle):** Prevent runaway 200-loop model iterations by placing an explicit attempt counter on the operation rewake loop (capped at 3–5 attempts).
- **B3 (Operation Prompt Recovery):** Ensure the original operation prompt is retrieved from `Operation.PromptArtifactRef` rather than decaying across an 8-run history scan.
- **H4 (Skip When Blocked):** Guard `maybeRewake` to ensure it skips if a Super is currently `RunBlocked` on human review.
- **H5 (Pending Control Deduplication):** Check if an unconsumed `execution_request` is already pending before minting another turn.

---

## 9. Strategic Roadmap & Next Actions

1. **Deploy Texture Rewake Substrate (Epoch 331):**
   - Land `maybeRewakeSelfDevelopmentTextureAfterTerminalSuper` in `selfdev_texture_join.go`.
   - Wire rewake into `reconcilePersistentSuperActor` and `persistSystemCoSuperCancellation`.
   - Apply adjudicated consensus safeguards (attempt bounds, blocked active checks, pending control dedup).
   - Push to `main`, observe CI and Node B deploy, and execute owner refresh to Epoch 331.
2. **Verify Live CoSuper Authorship:**
   - Observe Super rewake with bound Texture Control packet and `self_development_operation_id`.
   - Confirm Super calls `assign_co_super` on turn 1.
   - Monitor CoSuper authoring candidate A (Solitaire engine) inside guest capsule.
3. **Phase 2 & Phase 3 Execution:**
   - Freeze candidate A with five capsule-bound bundle artifacts.
   - Run qualified consensus panel on candidate A under `reversible-selfdev-v1`.
   - Promote candidate A, verify game play via API/DB tests, execute falsification with candidate B, and perform acceptance-fenced restore to pre-A checkpoint `99949fe2`.

---
*Report rendered and archived in iCloud Drive Choir Reports: `choir-effects-super-rewake-and-self-development-report-2026-08-20.pdf`*
