# Autonomous Computer Self-Supervision & Self-Development Master Blueprint
## Architectural Synthesis & Execution Roadmap for Arbitrary Goals with Verification
**Date:** 2026-08-20  
**Authority:** `docs/choir-doctrine.md`, `docs/agent-product-doctrine.md`, `docs/ACTIVE.md`  
**Derived From:** 3-Round Agentic Supervision Panel (Divergent → Lateral → Convergent)  
**Consensus Panelists:** Claude Opus, Gemini 3.7 Flash, Grok 4.6, Devin, Cursor, Opencode, Codex  
**Restore Fence Baseline:** Checkpoint `99949fe2` (Epoch 324)  

---

## 1. Executive Verdict & Core Doctrinal Invariants

The three-round agentic supervision panel unanimously confirms that Choir is transitioning from an externally driven coding harness to an **autonomous, self-supervising, self-developing computer**.

```text
                               Exogenous User Goal (CLI / Prompt Bar)
                                                 │
                                                 ▼
                                             Conductor
                                     (Intake & Goal Framing)
                                                 │
                                                 ▼
                                              Texture
                             (Live Supervision & Revision Surface)
                                vn-1 ──► vn ──► vn+1 ──► vn+2
                                                 │
                                        execution_request
                                          Control Packet
                                                 │
                                                 ▼
                                               Super
                                       (Host-Side Governor)
                                                 │
                                          assign_co_super
                                                 │
                                                 ▼
                                              CoSuper
                                    (Disposable Guest Capsule)
                                                 │
                                                 ▼
                                         Candidate Bundle
                                  (Patch + Receipts + Manifest)
                                                 │
                                                 ▼
                                       Qualified Consensus
                                   (Frozen Decision Policy)
                                                 │
                                                 ▼
                                        Computer Event Tape
                                  (Atomic Promotion / Restore)
```

### Core Invariants
1. **One Semantic Authority:** The single `ComputerID` plus its canonical event chain is the evolving computer. Execution capsules are inert until policy-bound acceptance. Texture never accepts, materializes, checkpoints, or routes.
2. **Revisions $
eq$ Static Workflow Steps:** Texture version numbers ($v_0 	o v_n$) reflect **dynamic living document revisions**, not hardcoded pipeline stages. Each revision represents a self-contained, human-readable update to the current system state, embedding structured citations that open into transcluded content (source diffs, test logs, candidate manifests, verifier receipts).
3. **Intake $
eq$ Governor $
eq$ Actuator:**
   - **Conductor:** Exogenous intake agent routing user/app input into Texture and trajectory work items (`AuthorityProfile: conductor > texture > super > co-super`). Has no self-development mutation authority.
   - **Texture:** Canonical living document and revision surface.
   - **Super:** Downstream host-side privileged execution governor; receives execution authority strictly from Texture `execution_request` Control packets.
   - **CoSuper:** Capability-scoped execution actuator in disposable guest capsules.
4. **Authorship is the Computer, Not Git:** Self-development means the guest computer authors, builds, tests, and verifies candidate code dynamically inside its own VM workspace/capsules via `choir run start`, rather than human developers pushing git commits from an external coding harness.
5. **Sequenced Yaegi RLM Cutover:** Current active definition (`choir-supervised-self-development-effects-2026-08-11`) proves policy-governed autonomy and restore on the repaired substrate first; successor definition (`choir-private-go-actor-kernel-2026-08-12`) implements the Yaegi Go REPL single-actuator engine as the permanent execution upgrade.

---

## 2. The 3-Round Supervision Synthesis

### Round 1: Divergent Option Space
- **Executable Computational Canvas:** Texture as an active notebook combining high-level intent, code blocks, and transcluded live evidence widgets (`[source:diff]`, `[test:receipts]`, `[consensus:quorum]`).
- **Probe-First Trajectory Synthesis:** Conductor executes zero-mutation inspection probes against live computer state before committing initial document scope.
- **Hierarchical Document Transclusion:** Large long-horizon tasks fork child Texture documents whose live summaries transclude into the root supervision document.

### Round 2: Lateral Reframes & Inverted Assumptions
- **The Tactic Engine Frame (Lean / Theorem Prover):** Move away from conversational chat turns. The model authors a typed Go tactic; the capsule evaluates the tactic at machine speed; if verification predicates pass, the computer accepts the state transition.
- **The Computer as Agent (Autopilot Envelope):** The computer is the agent; the LLM is a bootstrap compiler and an exception handler. Model activations occur on provable ignorance or invariant breach, not routine task stepping.
- **Continuous Compilation:** Arbitrary user goals compile into executable Go cells and verification assertions executed natively in the VM.

### Round 3: Convergent Architectural Blueprint
- **Document-Driven Supervision Surface:** Completely eliminate synthetic chat-turn prepending and raw mailbox message dumping in model context. Super and CoSuper communicate progress via structured `update_coagent` source packets that Texture renders into living document revisions with clickable citations.
- **CI Test Acceleration:** Sharding `internal/store` across 4 runners and expanding runtime matrix to 6 shards cuts CI test duration from 18.9m to 8.0m, bringing total CI + Deploy under 13 minutes.

---

## 3. The Dynamic Live Supervision Protocol

When a user submits an arbitrary goal via the Prompt Bar or CLI (`choir run start --prompt "..."`), Texture maintains an active, updating supervision surface:

1. **Continuous Revision Publication:** Whenever work items are assigned, code is modified, tests are executed, or consensus votes are cast, a new document revision is committed via `ApplyTextureTurn`.
2. **Structured Citation Transclusions:**
   - `[prompt:seed]` $	o$ Transcludes the raw user goal and commitment hash.
   - `[trajectory:work]` $	o$ Transcludes open/closed work item obligations and assigned roles.
   - `[capsule:handle]` $	o$ Transcludes capability grants, capsule ID, and resource bounds.
   - `[source:diff]` $	o$ Transcludes live unified diff of modified files in the capsule overlay.
   - `[test:receipts]` $	o$ Transcludes command execution receipts, exit codes, and test logs.
   - `[bundle:manifest]` $	o$ Transcludes the immutable 5-artifact candidate bundle hashes.
   - `[consensus:quorum]` $	o$ Transcludes individual agent votes, dissents, and policy receipts.
   - `[checkpoint:witness]` $	o$ Transcludes the 40-table Dolt content root and restore fence.
3. **Observable Feedback Loop:** Any observer on `choir.news` opens the living document and watches the autonomous self-development flywheel operate in real-time with full provenance.

---

## 4. Execution Roadmap for Arbitrary Goals with Verification

```
+---------------------------------------------------------------------------------------------------+
|                                      PHASED ROADMAP                                               |
+---------------------------------------------------------------------------------------------------+
| Milestone 1: Effects Acceptance Spine (Current Active Definition)                                 |
|   • Prove policy-governed autonomy on staging computer 03335285... (Epoch 333)                    |
|   • Conductor intake -> Texture living document -> Super governor -> CoSuper capsule authoring     |
|   • Freeze candidate bundle with 5 required artifacts                                             |
|   • Qualified consensus acceptance under frozen reversible-selfdev-v1                             |
|   • Live promotion -> play verification -> falsification -> restore to pre-A checkpoint 99949fe2  |
+---------------------------------------------------------------------------------------------------+
| Milestone 2: Yaegi RLM Private Go Kernel Cutover (Successor Definition)                           |
|   • Implement choir-private-go-actor-kernel-2026-08-12                                            |
|   • Cut CoSuper and Researcher over to single-actuator Yaegi Go REPL in disposable capsules       |
|   • Eliminate LLM tool-loop iteration caps (200-iter ceiling) via model-authored Go execution     |
+---------------------------------------------------------------------------------------------------+
| Milestone 3: Arbitrary Goal Self-Hosting Flywheel                                                 |
|   • Support general-purpose in-VM self-development across arbitrary user goals                     |
|   • Continuous verification against empirical evidence lattices and living Texture supervision    |
+---------------------------------------------------------------------------------------------------+
```

---
*Blueprint committed to repository: `docs/reports/choir-autonomous-supervision-and-self-development-master-blueprint-2026-08-20.md`*  
*Rendered and delivered to iCloud Drive Choir Reports: `choir-autonomous-supervision-and-self-development-master-blueprint-2026-08-20.pdf`*
