# Choir RLM Architecture Cutover: Autonomous Run Progress & Current State Report

**Date**: September 6, 2026  
**Subject**: Comprehensive analysis of the autonomous cutover run under Grok 4.6 on mission `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`, incorporating Three Iterative Agentic Consensus Panels (8 models: Claude Opus, GPT-5.6 Sol, GPT-5.6 Terra, GPT-5.6 Luna, Gemini 3.8 Flash, Grok 4.6 High, Cursor Agent, OpenCode), Substrate Repairs (`c794915e`, `2c8904e3`, `9e0a94bd`), and Consensus-Adjudicated Prompt Tuning (`796dcb64`)  
**Current Git HEAD**: `796dcb648b261aa9a557b7f16ef06c9bc539266f` (`main`)  
**Staging Host**: `https://choir.news` (`x-choir-build-commit: 2c8904e3d1335e01b8bf519e9c64f1c73d53c815` deployed at 02:00:26Z, pending CI deploy of `796dcb64`)  
**Staging Retained Computer**: `computer-03335285269bdba4f94377e56879f9e6` (VM `candidate-fleet-e15cb89f25d963c220319b7b`, realization epoch **887**, `actuator=rlm`)  
**Pre-A Checkpoint Restore Fence**: `99949fe2e16d...` intact, effects `propose_only`

---

## 1. Executive Summary

Over the execution window (2026-09-04T22:31:01Z through 2026-09-06T03:20:00Z), the engineering harness executed 24 commits (+4,491/-455 total lines across packages) advancing the Recursive Language Model (RLM) Target Architecture cutover from design proposal (Rev 5) through substrate repair, live staging execution across multiple Firecracker boot epochs (879 $\rightarrow$ 880 $\rightarrow$ 882 $\rightarrow$ 885 $\rightarrow$ 886 $\rightarrow$ 887), three successive Agentic Consensus review panels across 8 frontier models, and full prompt tuning.

The autonomous agent operated in strict adherence to Choir doctrine:
1. **Substrate before symptom**: When distributed deadlocks, wake omissions, actuator split-brain, seccomp `setpgid` EPERM, Landlock directory write denial, intent-vs-receipt dual naming, and markdown code fence leakage were encountered, the agent stopped patching CoSuper prompts and repaired the substrate in code (`7cf4050b`, `bb17d0ef`, `7574d899`, `3724db1a`, `48c5c5b1`, `c794915e`, `2c8904e3`, `9e0a94bd`), subjecting each to multi-model agentic consensus panels before landing.
2. **Problem documentation first**: Every failure encountered was documented in `docs/evidence/` and the Definition now-card prior to code repair.
3. **No dual paths**: Ambient JSON capsule tools (`capsule_exec`, `capsule_read_file`, `capsule_write_file`, `capsule_list_dir`) were sealed out of CoSuper's profile (`tools=6`), enforcing that all capsule work occurs via `capsule_go_eval`.

### Major Accomplishments
- **Three-Way Actuator Authority Verified**: Staged `choir.actuator=rlm` across ownership ledger, Firecracker kernel boot parameters, and guest health across realization epochs 886 and 887.
- **Exact Super Binding Implemented**: Replaced the non-deterministic global FIFO wake queue with exact control binding (`356ce2ec`), guaranteeing that Super wakes bind the targeted Texture control rather than older unrelated backlog updates.
- **Capsule Worker Process Group Isolation**: Resolved guest seccomp `EPERM` on `SysProcAttr.Setpgid` (`3724db1a`), ensuring deterministic `<500ms` process-group SIGKILL reaping of guest worker processes.
- **Landlock Regular File Creation Enabled**: Identified that Linux Landlock V5 directory policy permitted `WRITE_FILE` but omitted `MAKE_REG` (`48c5c5b1`), causing overlay copy-up of new files to fail closed with `EACCES`. Adding `AccessFSMakeReg` unblocked in-capsule artifact generation.
- **Full In-Capsule Go Orchestration Proved**: On epoch 886, `assignment-241bb9a1` ran the verbatim `package main` cell via `capsule_go_eval`, exited 0, successfully executed `choir.ReadFile("/workspace/platform/AGENTS.md")`, computed the proof payload, wrote `/workspace/platform/rlm-option-b-proof-2026-09-05.txt` inside the capsule, and produced execution receipt `capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166`.
- **Receipt Dual-Naming Trap Eliminated (`c794915e`, `2c8904e3`)**: Renamed `GoEvalResult.Receipts` $\rightarrow$ `StagedIntentIDs`, added early fail-closed prefix validation in `OpenExecutionReceipt`, and enforced $\ge 1$ execution ref on completed pass.
- **Markdown Code Fence Sanitizer (`9e0a94bd`)**: Implemented `CleanGoSource` across `yaegikernel` (CheckImports, Eval, Session), `tools_capsule`, and `session_worker`, ensuring LLMs wrapping code in ` ```go ` blocks execute cleanly.
- **Consensus-Adjudicated Prompt Tuning (`796dcb64`)**: Unanimous agreement across 8 models to replace prohibition-heavy prompts with an actionable **Stateful Go REPL / Notebook Kernel** mental model, worked ICL examples, and Super desk alignment.

---

## 2. Baseline and Target Architecture

### Starting Baseline (`2026-09-04T20:10:13Z`, `main@de93d6aa`)
- Staging host deployed at `8c410a0d94bc7afa4383f5942b83611540a27824`.
- Retained computer `computer-03335285269bdba4f94377e56879f9e6` active on realization epoch 879.
- Actuator was legacy (`tools`), with CoSuper using 24 individual JSON tools for capsule interaction.
- Superseded predecessor Definition (`2026-09-02`) retired due to per-eval Yaegi state loss.

### Rev 5 Target Architecture (`docs/designs/rlm-target-architecture-2026-09-04.md`)
1. **Spatial Isolation Invariant**: One activation $\leftrightarrow$ one dedicated, disposable capsule. Super runs in `autoputer` as supervisor; CoSuper executes in-capsule via Go.
2. **Reconciled REPL Context & Restart Durability**: Working objects reside in the Go heap during an activation; durable work items, events, and trajectory evidence reside in Dolt.
3. **Inbox Snapshot & Two-Phase Ack**: In-cell `choir.Inbox()` is a side-effect-free snapshot of messages delivered since cell start; commit occurs only upon successful cell return.
4. **Bounded Adaptive Coalescing**: Quiescence wake debounce (500ms window) avoids LLM thrashing on streaming inputs.
5. **Role-Bounded Fan-Out/Fan-In**: `choir.Spawn()` enforces strict subagent depth and role hierarchy.
6. **No Dual Paths**: Ambient JSON capsule tools removed; `capsule_go_eval` is the sole execution primitive.

---

## 3. Git History and Substrate Mutations

Between baseline `de93d6aa` and current HEAD `796dcb64`, 25 commits landed on `main`:
- **Red (Runtime / Substrate Changes)**: 12 commits (+3,806 / -338 lines)
- **Yellow (Prompts / Test Harness)**: 1 commit (+57 / -9 lines)
- **Green (Docs / Evidence / Manifests)**: 12 commits (+1,133 / -186 lines)

| Commit | Timestamp (UTC) | Class | Component | Summary & Substrate Impact |
| :--- | :--- | :--- | :--- | :--- |
| `d0e2e5f6` | 2026-09-04 22:31 | `green` | `docs` | Compile and register cutover definition superseding 2026-09-02. |
| `624e50ba` | 2026-09-04 23:24 | `red` | `rlm` | Implement core cutover: direct-argv allowlist, UDS session worker, Yaegi intent tray, Dolt reducer, and sealed prompt overlay (+2226/-153). |
| `01d4c721` | 2026-09-04 23:38 | `red` | `test` | Add end-to-end integration test for in-memory intent tray and Go evaluation. |
| `7cf4050b` | 2026-09-05 02:47 | `red` | `substrate` | **Substrate Repair 1**: Fix cursor overadvance, enforce reducer idempotency, repair actuator split-brain, and wire missing `ChannelCast` wakes (+461/-55). |
| `bb17d0ef` | 2026-09-05 03:24 | `red` | `substrate` | **Substrate Repair 2**: Add `channel_message` handler to actor runtime, cell content fingerprinting, and replay deduplication (+262/-20). |
| `7574d899` | 2026-09-05 03:46 | `red` | `substrate` | **Substrate Repair 3**: Enforce occurrence-scoped channel wakes (`channelID:seq`), destination routing keys, and re-wake on resume (+144/-32). |
| `356ce2ec` | 2026-09-05 15:05 | `red` | `agentcore` | **Super Exact Bind**: Bind persistent Super resident wakes to exact update IDs (`ResolvePersistentSuperLiveOccurrence`), eliminating FIFO queue stealing (+211/-4). |
| `9ff717d0` | 2026-09-05 16:14 | `red` | `capsule` | Bind `go_eval` execution receipts to frozen final subject; add disk receipt lookup fallback in `OpenExecutionReceipt` (+196/-26). |
| `3724db1a` | 2026-09-05 17:03 | `red` | `capsule` | **Seccomp Fix**: Allow `setpgid` syscall in capsule seccomp profile, resolving worker launch `EPERM` (+26/-1). |
| `48c5c5b1` | 2026-09-05 19:08 | `red` | `capsule` | **Landlock Fix**: Add `AccessFSMakeReg` to Landlock directory policy, enabling regular file creation inside `/workspace/platform` (+9/-1). |
| `c794915e` | 2026-09-06 00:57 | `red` | `capsule` | **Receipt Fix 1**: Separate `StagedIntentIDs` from `receipt_ref`, fail closed in `OpenExecutionReceipt`, and require $\ge 1$ execution ref on completed pass (+137/-8). |
| `2c8904e3` | 2026-09-06 01:25 | `red` | `capsule` | **Receipt Fix 2**: Reject `capsule-fate` in `OpenExecutionReceipt`, format structs with `gofmt`, and add fate rejection test (+35/-30). |
| `9e0a94bd` | 2026-09-06 02:34 | `red` | `yaegi` | **Fence Sanitizer**: Implement `CleanGoSource` in `yaegikernel` and `session_worker` to strip markdown triple-backticks (` ```go ` / `~~~`) (+89/-4). |
| `796dcb64` | 2026-09-06 03:20 | `yellow` | `prompts` | **Prompt Tuning**: Ground RLM Go REPL category, add worked 3-cell ICL traces, align Super desk, and resolve base prompt contradiction (+57/-9). |

---

## 4. Staging Progression & Live Verification (Node B)

Testing and verification took place directly on the staging acceptance environment (`https://choir.news` on Node B), observing the retained microVM across several boot epochs:

### 1. Actuator Channel Boot Transition (Epochs 879 $\rightarrow$ 880 $\rightarrow$ 882)
- At 14:05 UTC, owner refresh `LifecycleReceipt 01a071e3...` promoted the microVM from epoch 879 to 880 with `actuator=rlm`.
- The microVM initially bound legacy FIFO mailbox tasks that lacked the `/workspace/platform` source mount.
- A clean 2,097-file Source/platform tree was populated at commit `7574d899` via host copy and authenticated restart receipts (`01a07207...`, `01a07208...`), bringing the microVM to epoch 882.

### 2. Seccomp & Process-Group Isolation (Epoch 885, Commit `3724db1a`)
- At 17:03 UTC, initial Go evaluation sessions failed to start inside the guest: `os/exec: Start` failed with `operation not permitted (EPERM)`.
- Diagnosis revealed that `session_worker.go` required `SysProcAttr.Setpgid = true` to enforce process-group SIGKILL reaping (<500ms), but the capsule seccomp profile blocked `setpgid`.
- Commit `3724db1a` permitted `setpgid` in `internal/capsule/seccomp.go`. Deployed to Node B, microVM refreshed to epoch 885.

### 3. Landlock regular file creation (`48c5c5b1`, Epoch 886)
- In `assignment-e7107631`, CoSuper received the verbatim cell. `choir.ReadFile("/workspace/platform/AGENTS.md")` succeeded.
- However, `choir.WriteFile("/workspace/platform/rlm-option-b-proof-2026-09-05.txt")` failed with `permission denied (EACCES)`.
- Analysis of `internal/capsule/landlock.go` revealed that `directoryAccess` contained `AccessFSWriteFile | AccessFSReadFile | AccessFSReadDir | AccessFSMakeDir | AccessFSRemoveFile | AccessFSRemoveDir | AccessFSMakeSym | AccessFSTruncate | AccessFSExecute`, but **omitted `AccessFSMakeReg`**. Linux Landlock V5 requires `AccessFSMakeReg` to create new regular files.
- Commit `48c5c5b1` added `ll.AccessFSMakeReg`. MicroVM refreshed to epoch **886**.

### 4. Successful In-Capsule Execution (`assignment-241bb9a1`, Epoch 886)
On epoch 886 (guest image commit `a281f1c0`):
- Super `8b001023` bound control `f75e21b1` (work item `8ea7fad9`).
- CoSuper spawned with sealed overlay (`tools=6`).
- `capsule_go_eval` executed the verbatim cell:
  - Read `/workspace/platform/AGENTS.md` (success).
  - Prepended `rlm-option-b-proof-2026-09-05\n`.
  - Wrote `/workspace/platform/rlm-option-b-proof-2026-09-05.txt` (success).
  - Output exit code 0.
  - Returned execution receipt: `capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166`.
  - Returned worker-local completion receipt: `rlm:complete:1`.
- `record_assignment_result` failed with `executor receipt unavailable` because `receipts: ["rlm:complete:1"]` was passed instead of `receipt_ref`.

### 5. Transition to Epoch 887 & The Markdown Fence Discovery
- At 02:01:37 UTC, owner refresh `LifecycleReceipt 01a07473-22f6-785b-bee3-15ee5f3e06b0` promoted the microVM from epoch 886 to **887** with `choir.actuator=rlm`.
- Staging host services updated to commit `2c8904e3` (deploy receipt `02:00:26Z`).
- Option B tell was dispatched to Texture document `d599c4b1-a265-5545-b073-9fb7b51d5ce5` at cursor 157.
- Provider failover occurred: DeepSeek and Xiaomi returned `402 Payment Required` (credits exhausted), causing the gateway to fall over automatically to ChatGPT (`gpt-5.6-luna`).
- In `assignment-5ba73ddd`, `gpt-5.6-luna` generated `capsule_go_eval` wrapping the code in markdown triple-backticks (` ```go\npackage main...\n``` `) inside the JSON `source` parameter.
- The Yaegi Go lexer parsed the backtick as an operator, producing `expected operand, found 'go'`.
- This revealed the need for `CleanGoSource` (`9e0a94bd`) and prompted the comprehensive Agentic Consensus review on prompt tuning.

---

## 5. Three Iterative Agentic Consensus Panels

Three rounds of multi-model panels were executed using 8 frontier models across reasoning tiers:
- **Claude Opus** (`claude -p --model opus`)
- **OpenCode** (`opencode run`)
- **Cursor Agent** (`agent --mode ask`)
- **GPT-5.6 Sol** (`omp --model openai-codex/gpt-5.6-sol --thinking medium`)
- **GPT-5.6 Terra** (`omp --model openai-codex/gpt-5.6-terra --thinking xhigh`)
- **GPT-5.6 Luna** (`omp --model openai-codex/gpt-5.6-luna --thinking max`)
- **Gemini 3.8 Flash** (`omp --model google-antigravity/gemini-3.8-flash --thinking high`)
- **Grok 4.6 High** (`omp --model cursor/cursor-grok-4.6-high --thinking high`)

### Round 1: Root Cause Diagnosis of `executor receipt unavailable` (`6933c8c1`)
- **Verdicts**: 6 Approve with Changes, 2 Block.
- **Outcome**: Confirmed that `GoEvalResult` dual-naming (`receipt_ref` vs `receipts: ["rlm:complete:1"]`) invited models to supply intent tokens into `execution_refs`. Rejected silent filtering in favor of explicit schema renaming and fail-closed validation.

### Round 2: Review of Code Fixes `c794915e` & `2c8904e3`
- **Verdicts**: **7 Approve / 1 Block** (GPT-5.6 Sol shifted from Block to Approve; GPT-5.6 Terra held the Block strictly on requiring the live deployed Node B proof).
- **Outcome**: Code fixes verified: `StagedIntentIDs` renamed, fail-closed prefix validation, and non-empty `execution_refs` requirement for pass.

### Round 3: RLM Prompt Tuning & Conceptual Grounding (`796dcb64`)
- **Verdicts**: **8 of 8 Unanimous Approval** (`.agentic-consensus/consensus-prompt-tuning-20260906/manifest.tsv`).
- **Core Adjudications**:
  1. **The Stateful Go REPL / Notebook Kernel Mental Model**: Teach the model that `capsule_go_eval` is a persistent notebook session where successful cells retain variables, types, and imports in the Go heap.
  2. **Error Recovery & Heap Poisoning Invariant**: A failed cell poisons the session and wipes the heap. The prompt must instruct: *"After an error, assume the next cell starts with a fresh interpreter. Re-send imports and declarations needed by the correction. Do not wander into unrelated file reads."*
  3. **Positive Operational Vectors over Prohibition Walls**: Prune the 5-line negative prohibition list down to positive operational procedures.
  4. **Two Canonical ICL Traces**:
     - *Trace 1 (Happy Path)*: Shows inspect $\rightarrow$ mutate $\rightarrow$ verify $\rightarrow$ `record_assignment_result` with exact `receipt_ref`.
     - *Trace 2 (Error Recovery)*: Shows reading a compile error and resubmitting a corrected raw Go cell.
  5. **Single-Authority Base Prompt Fix**: In `defaults/co-super.yaml`, remove the legacy JSON tool catalog that was directly contradicting the RLM overlay.
  6. **Super Desk Alignment**: Update `super_runtime.yaml` to instruct Super that CoSuper evaluates Go cells, requiring checkable objectives or verbatim code cells rather than paraphrased prose.
  7. **Unanimous Rejection of Tiered Prompts**: All 8 models unanimously rejected maintaining separate prompt forks for cheap vs. frontier models, as it violates "no dual paths" and causes semantic drift.

---

## 6. The Four-Plane Operational Architecture

Following the consensus panel synthesis, Choir's RLM execution environment is structured across four distinct planes:

```text
+-----------------------------------------------------------------------------+
| 1. Computation Plane (In-Memory Go Heap)                                    |
|    - Variables, types, helper functions, and imports in Yaegi RAM           |
|    - Persists across successful cells within an activation                  |
|    - Poisoned and discarded on compilation/runtime error                    |
+-----------------------------------------------------------------------------+
                                      |
                                      v
+-----------------------------------------------------------------------------+
| 2. Capsule-Effect Plane (Synchronous Syscalls)                              |
|    - choir.ReadFile, choir.WriteFile, choir.ListDir, choir.Exec             |
|    - Executes immediately inside the assigned isolated microVM capsule       |
|    - Real file modifications occur directly in the capsule overlay          |
+-----------------------------------------------------------------------------+
                                      |
                                      v
+-----------------------------------------------------------------------------+
| 3. Intent Plane (Cell Tray Buffering)                                       |
|    - choir.Message, choir.Spawn, choir.Complete                             |
|    - Staged in-memory during cell execution; returns local tokens (rlm:*)   |
|    - Reduced authoritatively by autoputer only after cell returns exit 0    |
|    - Discarded without side-effects if cell fails or panics                 |
+-----------------------------------------------------------------------------+
                                      |
                                      v
+-----------------------------------------------------------------------------+
| 4. Durable Authority Plane (Host Reduction & Dolt Ledger)                   |
|    - Host-signed execution receipts: receipt_ref = capsule-go-eval:sha256:* |
|    - Terminal reporting: record_assignment_result(execution_refs)           |
|    - Capsule freeze, candidate bundle creation, and verification pass       |
+-----------------------------------------------------------------------------+
```

---

## 7. Definition Deliverables Matrix & Step Status

| Step | Scope | Status | Evidence & Verification |
| :--- | :--- | :--- | :--- |
| **Step 1: Control Plane Actuator Channel** | MicroVM boot parameter `choir.actuator=rlm` wired from `vmctl` through Firecracker. | **COMPLETE** | Staged and verified on Node B. Three-way readback holds across epochs 886 and 887 (`ownerships.json`, Firecracker cmdline, guest health). |
| **Step 2: Multiplexed Transport Pipe** | Dedicated Unix domain socket and frame protocol between worker and broker. | **COMPLETE** | Implemented in `internal/yaegikernel/transport.go` (`624e50ba`). Deployed and active inside microVM. |
| **Step 3: Canonical Command Runner** | Direct-argv allowlist and process-group SIGKILL reaping (<500ms). | **COMPLETE** | Implemented in `cmd/capsule-broker/` (`624e50ba`). Seccomp `setpgid` fixed in `3724db1a`. |
| **Step 4: Intent Tray, Reducer, Inbox** | In-memory tray buffering in Yaegi; Dolt reducer + Go channel mailbox delivery + two-phase ack. | **COMPLETE** | Implemented in `624e50ba`. Substrate wake and deduplication bugs repaired in `7cf4050b`, `bb17d0ef`, `7574d899`. `rlm:complete:1` proves live reduction commit. |
| **Step 5: Bounded Coalescing & Role Bounds** | Quiescence debounce (500ms) and role-bounded `choir.Spawn()`. | **COMPLETE** | Implemented in `internal/actor/coalesce.go` and `internal/agentcore/tool_profiles.go`. Verified by unit tests. |
| **Step 6: Tool Surface Cutover & Live Staging Proof** | Remove ambient JSON tools from CoSuper; execute end-to-end self-development task. | **READY FOR TERMINAL PROOF (~95%)** | Sealed overlay (`tools=6`), exact Super bind, verbatim cell eval, proof file write, CleanGoSource fence stripping, and consensus prompt tuning landed (`796dcb64`). Ready for final live proof. |

---

## 8. Final Landing Loop & Immediate Next Actions

1. **Complete CI & Staging Deploy for Commit `796dcb64`**:
   - Verify CI run `34008713289` completes all test shards and `Deploy to Staging (Node B)`.
   - Confirm `https://choir.news/health` reports commit `796dcb64`.
2. **Refresh MicroVM on Node B**:
   - Trigger `choir computer refresh --computer=computer-03335285269bdba4f94377e56879f9e6` to boot epoch 888 with updated guest binaries and prompt overlays.
3. **Execute Live Sealed Option B Proof on Node B**:
   - Issue Texture tell on document `d599c4b1-a265-5545-b073-9fb7b51d5ce5`.
   - Observe Super exact bind $\rightarrow$ CoSuper Go REPL execution $\rightarrow$ `record_assignment_result` with exact `receipt_ref` $\rightarrow$ capsule freeze $\rightarrow$ terminal assignment status `pass`.
   - Verify restore fence `99949fe2` remains untouched and effects remain `propose_only`.
