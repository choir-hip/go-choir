# Choir RLM Architecture Cutover: Autonomous Run Progress & Current State Report

**Date**: September 5, 2026  
**Subject**: Comprehensive analysis of the ~12-hour autonomous cutover run under Grok 4.6 on mission `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`, incorporating Two Iterative Agentic Consensus Panels (8 models: Claude Opus, GPT-5.6 Sol, GPT-5.6 Terra, GPT-5.6 Luna, Gemini 3.8 Flash, Grok 4.6 High, Cursor Agent, OpenCode) and Substrate Code Fixes (`c794915e`, `2c8904e3`)  
**Current Git HEAD**: `2c8904e393b45a0b5a6c3f6838a3962d3f789e92` (`main`)  
**Staging Host**: `https://choir.news` (`x-choir-build-commit: a281f1c0df394a719fb48fdb7f61af9dffcac5d3` pending CI deploy of `2c8904e3`)  
**Staging Retained Computer**: `computer-03335285269bdba4f94377e56879f9e6` (VM `candidate-fleet-e15cb89f25d963c220319b7b`, realization epoch **886**, `actuator=rlm`)  
**Pre-A Checkpoint Restore Fence**: `99949fe2e16d...` intact, effects `propose_only`

---

## 1. Executive Summary

Over the ~12-hour autonomous execution window (2026-09-04T22:31:01Z through 2026-09-05T20:08:08Z), the coding harness executed 19 commits (+4,345/-404 total lines across packages) progressing the Recursive Language Model (RLM) Target Architecture cutover from design proposal (Rev 5) to live staging execution on Node B. Subsequent consensus-adjudicated repairs added 2 commits (`c794915e`, `2c8904e3`, +172/-38 lines) to eliminate the receipt dual-naming trap and enforce fail-closed receipt resolution.

The autonomous agent operated in strict adherence to Choir doctrine:
1. **Substrate before symptom**: When distributed deadlocks, wake omissions, and actuator split-brain conditions were encountered in staging, the agent stopped patching CoSuper prompts and instead repaired the core messaging, wake, and reduction substrates (`7cf4050b`, `bb17d0ef`, `7574d899`), subjecting each to multi-model agentic consensus panels before landing.
2. **Problem documentation first**: Every failure encountered (FIFO mailbox contamination, seccomp `setpgid` EPERM, Yaegi date/octal parsing, Landlock directory write denial) was documented in `docs/evidence/` and the Definition now-card prior to code repair.
3. **No dual paths**: Ambient JSON capsule tools (`capsule_exec`, `capsule_read_file`, `capsule_write_file`, `capsule_list_dir`) were sealed out of CoSuper's profile (`tools=6`), enforcing that all capsule work occurs via `capsule_go_eval`.

### Major Accomplishments
- **Three-Way Actuator Authority Verified**: Staged `choir.actuator=rlm` across ownership ledger, Firecracker kernel boot parameters, and guest health on realization epoch 886.
- **Exact Super Binding Implemented**: Replaced the non-deterministic global FIFO wake queue with exact control binding (`356ce2ec`), guaranteeing that Super wakes bind the targeted Texture control rather than older unrelated backlog updates.
- **Capsule Worker Process Group Isolation**: Resolved guest seccomp `EPERM` on `SysProcAttr.Setpgid` (`3724db1a`), ensuring deterministic `<500ms` process-group SIGKILL reaping of guest worker processes.
- **Landlock Regular File Creation Enabled**: Identified that Linux Landlock V5 directory policy permitted `WRITE_FILE` but omitted `MAKE_REG` (`48c5c5b1`), causing overlay copy-up of new files to fail closed with `EACCES`. Adding `AccessFSMakeReg` unblocked in-capsule artifact generation.
- **Full In-Capsule Go Orchestration Proved**: On epoch 886, `assignment-241bb9a1` ran the verbatim `package main` cell via `capsule_go_eval`, exited 0, successfully executed `choir.ReadFile("/workspace/platform/AGENTS.md")`, computed the proof payload, wrote `/workspace/platform/rlm-option-b-proof-2026-09-05.txt` inside the capsule, and produced execution receipt `capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166`.

### The Blocker Resolution & Convergence
The terminal step `record_assignment_result` failed on epoch 886 with `executor receipt unavailable` immediately after `capsule_go_eval` exited 0.
Through two consecutive rounds of multi-model agentic consensus review across 8 frontier models, the panel converged on the root cause and verified the code repair:
1. **Root cause**: `GoEvalResult` dual-naming (`receipt_ref` vs `receipts: ["rlm:complete:1"]`) invited the model to supply intent sequence tokens into `execution_refs`, failing `OpenExecutionReceipt` closed.
2. **Repaired code (`c794915e`, `2c8904e3`)**:
   - Renamed `GoEvalResult.Receipts` $\rightarrow$ `StagedIntentIDs []string `json:"staged_intent_ids,omitempty"``, leaving `receipt_ref` as the sole model-visible execution reference.
   - Added early prefix validation in `OpenExecutionReceipt` rejecting `rlm:*` with a typed error naming internal intent tokens, and rejecting unsupported prefixes (`capsule-fate:`) before filesystem lookup.
   - Enforced that a terminal completed pass requires $\ge 1$ valid execution receipt, closing the `0==0` false-pass bypass.
   - Updated prompt overlay instructions in `rlm_co_super_runtime.yaml`.
   - Cleaned up formatting with `gofmt` and added unit tests in `internal/capsule` and `internal/agentcore`.
3. **Convergence**: 7 of 8 models approved the code repair; the sole dissenting block (GPT-5.6 Terra) holds the line that Step 6 completion requires the live Node B deployed proof, which is the immediate next action.

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

Between baseline `de93d6aa` and current HEAD `2c8904e3`, 22 commits landed on `main`:
- **Red (Runtime / Substrate Changes)**: 11 commits (+3,717 / -334 lines)
- **Green (Docs / Evidence / Manifests)**: 11 commits (+1,058 / -108 lines)

| Commit | Timestamp (UTC) | Class | Component | Summary & Substrate Impact |
| :--- | :--- | :--- | :--- | :--- |
| `d0e2e5f6` | 2026-09-04 22:31 | `green` | `docs` | Compile and register `choir-rlm-target-architecture-cutover-2026-09-04.md` superseding 2026-09-02. |
| `624e50ba` | 2026-09-04 23:24 | `red` | `rlm` | Implement core cutover: direct-argv allowlist, UDS session worker, Yaegi intent tray, Dolt reducer, and sealed prompt overlay (+2226/-153). |
| `01d4c721` | 2026-09-04 23:38 | `red` | `test` | Add end-to-end integration test for in-memory intent tray and Go evaluation. |
| `ea94da5e` | 2026-09-04 23:49 | `green` | `docs` | Record CI/deploy run status for 624e50ba. |
| `66939532` | 2026-09-05 00:01 | `green` | `docs` | Update doc authority manifest schema for cutover artifacts. |
| `19bcd957` | 2026-09-05 00:26 | `green` | `docs` | Record CI 33931633587 success and initial Node B deploy receipt. |
| `7cf4050b` | 2026-09-05 02:47 | `red` | `substrate` | **Substrate Repair 1**: Fix cursor overadvance, enforce reducer idempotency, repair actuator split-brain, and wire missing `ChannelCast` wakes (+461/-55). |
| `bb17d0ef` | 2026-09-05 03:24 | `red` | `substrate` | **Substrate Repair 2**: Add `channel_message` handler to actor runtime, cell content fingerprinting, and replay deduplication (+262/-20). |
| `7574d899` | 2026-09-05 03:46 | `red` | `substrate` | **Substrate Repair 3**: Enforce occurrence-scoped channel wakes (`channelID:seq`), destination routing keys, and re-wake on resume (+144/-32). |
| `9cd554d3` | 2026-09-05 04:35 | `green` | `docs` | Publish consensus report `choir-rlm-substrate-repairs-and-g1-producer-2026-09-05.md`. |
| `578fe411` | 2026-09-05 14:32 | `green` | `docs` | Record epoch 880 actuator refresh and Super mailbox backlog blockers. |
| `2b62db2d` | 2026-09-05 14:47 | `green` | `docs` | Record Source/platform population (2,097 files) and FIFO mailbox race. |
| `356ce2ec` | 2026-09-05 15:05 | `red` | `agentcore` | **Super Exact Bind**: Bind persistent Super resident wakes to exact update IDs (`ResolvePersistentSuperLiveOccurrence`), eliminating FIFO queue stealing (+211/-4). |
| `9ff717d0` | 2026-09-05 16:14 | `red` | `capsule` | Bind `go_eval` execution receipts to frozen final subject; add disk receipt lookup fallback in `OpenExecutionReceipt` (+196/-26). |
| `3724db1a` | 2026-09-05 17:03 | `red` | `capsule` | **Seccomp Fix**: Allow `setpgid` syscall in capsule seccomp profile, resolving worker launch `EPERM` (+26/-1). |
| `f019eeaf` | 2026-09-05 18:49 | `green` | `docs` | Record exact Super bind, tools=6 overlay, and CoSuper Go parse errors. |
| `48c5c5b1` | 2026-09-05 19:08 | `red` | `capsule` | **Landlock Fix**: Add `AccessFSMakeReg` to Landlock directory policy, enabling regular file creation inside `/workspace/platform` (+9/-1). |
| `a281f1c0` | 2026-09-05 19:08 | `green` | `docs` | Record verbatim package-main evaluation and Landlock MAKE_REG remainder. |
| `dd6a8f04` | 2026-09-05 20:08 | `green` | `docs` | Record exit-0 `capsule_go_eval`, proof file creation, and `executor receipt unavailable` blocker. |
| `6933c8c1` | 2026-09-05 20:45 | `green` | `docs` | Add cutover progress report with first consensus panel review synthesis. |
| `c794915e` | 2026-09-06 00:57 | `red` | `capsule` | **Receipt Fix 1**: Separate `StagedIntentIDs` from `receipt_ref`, fail closed in `OpenExecutionReceipt`, and require $\ge 1$ execution ref on completed pass (+137/-8). |
| `2c8904e3` | 2026-09-06 01:25 | `red` | `capsule` | **Receipt Fix 2**: Reject `capsule-fate` in `OpenExecutionReceipt`, format structs with `gofmt`, and add fate rejection test (+35/-30). |

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

### 3. Model Passthrough vs. Yaegi File Evaluation
- On epoch 885, Super `e3306a66` and `da34787a` bound exact controls, and CoSuper launched with `tools=6`.
- CoSuper initially failed evaluation:
  - `assignment-60bdda62`: Super paraphrased the task into prose; Yaegi failed parsing missing `package main`.
  - `assignment-b8fad426`: Model emitted an unquoted date `2026-09-05`, which Go parsed as arithmetic `2026 - 09 - 05`; Go rejected `09` as an invalid octal literal.
- The prompt was adjusted via Texture-tell to require Super to inject the exact `package main` program verbatim into the objective.

### 4. Landlock regular file creation (`48c5c5b1`, Epoch 886)
- In `assignment-e7107631`, CoSuper received the verbatim cell. `choir.ReadFile("/workspace/platform/AGENTS.md")` succeeded.
- However, `choir.WriteFile("/workspace/platform/rlm-option-b-proof-2026-09-05.txt")` failed with `permission denied (EACCES)`.
- Analysis of `internal/capsule/landlock.go` revealed that `directoryAccess` contained `AccessFSWriteFile | AccessFSReadFile | AccessFSReadDir | AccessFSMakeDir | AccessFSRemoveFile | AccessFSRemoveDir | AccessFSMakeSym | AccessFSTruncate | AccessFSExecute`, but **omitted `AccessFSMakeReg`**. Linux Landlock V5 requires `AccessFSMakeReg` to create new regular files.
- Commit `48c5c5b1` added `ll.AccessFSMakeReg`. Deployed via CI run 33986200851 (after resolving intermittent SQLite lock test flakes in shard 5).
- MicroVM refreshed via `LifecycleReceipt 01a07323-c4fa-718a-96cf-8d8291e3c8ec` from epoch 885 to **886**.

### 5. Successful In-Capsule Execution (`assignment-241bb9a1`)
On epoch 886 (guest image commit `a281f1c0`):
- Super `8b001023` bound control `f75e21b1` (work item `8ea7fad9`).
- CoSuper spawned with sealed overlay (`tools=6`).
- `capsule_go_eval` executed the verbatim cell:
  - Read `/workspace/platform/AGENTS.md` (success).
  - Prepended `rlm-option-b-proof-2026-09-05\n`.
  - Wrote `/workspace/platform/rlm-option-b-proof-2026-09-05.txt` (success).
  - Invoked `choir.Complete("completed", "pass", ...)` (success).
  - Output exit code 0.
  - Returned execution receipt: `capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166`.
  - Returned worker-local completion receipt: `rlm:complete:1`.

---

## 5. Detailed Root Cause Analysis: `executor receipt unavailable`

While `capsule_go_eval` succeeded, the terminal step `record_assignment_result` failed, leaving the assignment unsealed.

### Code Path Tracing
1. **Tool Invocation**: CoSuper invokes `record_assignment_result` with parameter `execution_refs` (`internal/agentcore/tools_capsule.go:867-929`).
2. **Whitespace Trimming**: `trimNonEmptyStrings` strips leading/trailing whitespace from each ref (`tools_capsule.go:954-960`).
3. **Receipt Resolution**: The tool immediately executes `receipts, err := toolCtx.Executor.ResolveExecutionReceipts(input.ExecutionRefs)`.
4. **Resolution Loop**: `ResolveExecutionReceipts` iterates over every supplied ref and invokes `e.OpenExecutionReceipt(ref)` (`internal/capsule/executor.go:936-943`).
5. **Lookup & Fallback**: `OpenExecutionReceipt` executes:
   ```go
   e.mu.RLock()
   stored, ok := e.executionReceipts[ref]
   e.mu.RUnlock()
   if ok {
       return stored, nil
   }
   raw, err := os.ReadFile(filepath.Join(e.stateDir, "receipts", "execution", receiptArtifactName(ref)))
   if err != nil {
       return ExecutionReceipt{}, fmt.Errorf("executor receipt unavailable")
   }
   ```
6. **Error Return**: An error returned from `OpenExecutionReceipt` halted `record_assignment_result` immediately, before `recordAssignedCoSuperReport` or `bindFrozenAssignmentExecutionReceipts` could be invoked.

### Causal Hypotheses & Discriminators

The static source and consensus reviews ruled out several potential explanations:
- **Not a naming hash discrepancy**: `persistReceiptArtifact` and `OpenExecutionReceipt` use the exact same `receiptArtifactName(ref)` helper (`sha256(ref).json`).
- **Not prefix validation failure**: A receipt with an invalid prefix or mismatched SHA256 digest returns `executor receipt digest mismatch` or `executor receipt is invalid`, **not** `executor receipt unavailable`. The `unavailable` error occurs strictly when both the in-memory map lookup and the disk `os.ReadFile` fail.
- **Not an Autoputer or Firecracker restart (Hypothesis B excluded)**:
  - Staging telemetry confirms Firecracker PID 3052070 ran continuously from 19:55:16Z without reboot.
  - `record_assignment_result` first checks `requireCapsuleRole`. An autoputer restart would destroy live capability keypairs, causing earlier failure on `capsule authority unavailable`.
  - In `GoEval` (`executor.go:664-666`), `receipt.ReceiptRef` is inserted into `e.executionReceipts` *before returning*. In the same process, a lookup for that exact string is a guaranteed in-memory map hit.

### The Confirmed Primary Mechanism: Schema Naming Contamination
- In `internal/capsule/types.go:119-128`, `GoEvalResult` serialized:
  ```go
  type GoEvalResult struct {
      ...
      ReceiptRef string   `json:"receipt_ref,omitempty"`
      Receipts   []string `json:"receipts,omitempty"` // populated with rlm:complete:1
  }
  ```
- The prompt instructed CoSuper: *"Report typed intermediate progress and the one terminal assignment result through record_assignment_result with ... exact execution_refs."*
- Faced with a scalar `receipt_ref` and an array field named `receipts: ["rlm:complete:1"]`, LLMs predictably passed `execution_refs: ["capsule-go-eval:sha256:7fe...", "rlm:complete:1"]` (or only `["rlm:complete:1"]`).
- `OpenExecutionReceipt("rlm:complete:1")` executed:
  1. Map lookup `e.executionReceipts["rlm:complete:1"]` $\rightarrow$ **MISS** (never stored).
  2. Disk lookup `stateDir/receipts/execution/sha256("rlm:complete:1").json` $\rightarrow$ **MISS** (does not exist).
  3. Returned `executor receipt unavailable`.
- `ResolveExecutionReceipts` failed the entire batch on the first error, aborting before capsule freeze.

---

## 6. Two-Round Agentic Consensus Panel Review

The findings and code fixes were evaluated across two successive panel reviews utilizing 8 frontier models across reasoning tiers:
- **Claude Opus** (`claude -p --model opus`)
- **OpenCode** (`opencode run`)
- **Cursor Agent** (`agent --mode ask`)
- **GPT-5.6 Sol** (`omp --model openai-codex/gpt-5.6-sol --thinking medium`)
- **GPT-5.6 Terra** (`omp --model openai-codex/gpt-5.6-terra --thinking xhigh`)
- **GPT-5.6 Luna** (`omp --model openai-codex/gpt-5.6-luna --thinking max`)
- **Gemini 3.8 Flash** (`omp --model google-antigravity/gemini-3.8-flash --thinking high`)
- **Grok 4.6 High** (`omp --model cursor/cursor-grok-4.6-high --thinking high`)

*(Manifests: `.agentic-consensus/consensus-review-20260905/manifest.tsv` and `.agentic-consensus/consensus-review-20260905-fixes/manifest.tsv`)*

### Round 1: Problem Diagnosis & Architectural Adjudication (Commit `6933c8c1`)
- **Verdicts**: 5 Approve with Changes (Claude Opus, OpenCode, Cursor, Gemini 3.8 Flash, Grok 4.6 High), 2 Block (GPT-5.6 Sol, GPT-5.6 Terra), 1 Approve with Changes (GPT-5.6 Luna).
- **Core Adjudications**:
  1. *Unanimous Root Cause Agreement*: All models confirmed `GoEvalResult` dual-naming directly invited the model to submit `rlm:complete:1` into `execution_refs`.
  2. *Rejection of Silent Filtering*: The panel firmly rejected silently dropping `rlm:*` tokens. Doing so permits an assignment with intent-only tokens to collapse to `execution_refs: []`, committing an unevidenced pass (`0==0` pass).
  3. *Rejection of Wholesale StateDir Relocation*: Moving `CHOIR_CAPSULE_STATE_DIR` to `/mnt/persistent` violates spatial isolation and clutters persistent disks with disposable overlay directories.

### Round 2: Review of Code Fixes (`c794915e`, `2c8904e3`)
- **Verdicts**: **7 Approve with Changes / Approve**, **1 Block**.
  - **Gemini 3.8 Flash**: `APPROVE` (122s) — confirmed root cause eliminated, fail-closed prefix checks verified, `0==0` pass closed.
  - **Cursor Agent**: `APPROVE` (321s) — verified schema trap removed, fail-closed handling preserves all-or-nothing evidence.
  - **Grok 4.6 High**: `APPROVE-WITH-CHANGES` (321s) — confirmed local correctness, verified tests pass, ready to deploy.
  - **OpenCode**: `APPROVE-WITH-CHANGES` (432s) — independently traced call sites, confirmed `0==0` defense-in-depth, verified tests.
  - **Claude Opus**: `APPROVE-WITH-CHANGES` (486s) — verified prefix allowlist matches reload allowlist, confirmed stub parity.
  - **GPT-5.6 Sol**: `APPROVE-WITH-CHANGES` (532s) — **shifted from Block to Approve**; verified structural fix, closed false-pass hole.
  - **GPT-5.6 Luna**: `APPROVE-WITH-CHANGES` (690s) — verified fail-closed resolution, confirmed schema separation.
  - **GPT-5.6 Terra**: `BLOCK` (662s) — praised schema separation and fail-closed validation, but held the line that Step 6 completion is a binary gate requiring the live Node B deployed proof.

### Summary of Panel Recommendations Implemented
1. **Schema Renaming**: `GoEvalResult.Receipts` $\rightarrow$ `StagedIntentIDs` (`types.go:127`).
2. **Early Prefix Validation**: `OpenExecutionReceipt` rejects `rlm:*` and unsupported prefixes before disk I/O (`executor.go:810-815`).
3. **Execution Receipt Scope**: Removed `capsule-fate:` from `OpenExecutionReceipt` (fate receipts use `OpenCapsuleFateReceipt`).
4. **False-Pass Protection**: `record_assignment_result` unconditionally requires $\ge 1$ execution receipt on completed pass (`tools_capsule.go:899-901`).
5. **Code Formatting**: Applied `gofmt` to all modified files to ensure zero struct alignment regressions.

---

## 7. Definition Deliverables Matrix & Step Status

| Step | Scope | Status | Evidence & Verification |
| :--- | :--- | :--- | :--- |
| **Step 1: Control Plane Actuator Channel** | MicroVM boot parameter `choir.actuator=rlm` wired from `vmctl` through Firecracker. | **COMPLETE** | Staged and verified on Node B. Three-way readback holds on epoch 886 (`ownerships.json`, Firecracker cmdline, guest health). |
| **Step 2: Multiplexed Transport Pipe** | Dedicated Unix domain socket and frame protocol between worker and broker. | **COMPLETE** | Implemented in `internal/yaegikernel/transport.go` (`624e50ba`). Deployed and running inside microVM. |
| **Step 3: Canonical Command Runner** | Direct-argv allowlist and process-group SIGKILL reaping (<500ms). | **COMPLETE** | Implemented in `cmd/capsule-broker/` (`624e50ba`). Seccomp `setpgid` fixed in `3724db1a`. |
| **Step 4: Intent Tray, Reducer, Inbox** | In-memory tray buffering in Yaegi; Dolt reducer + Go channel mailbox delivery + two-phase ack. | **COMPLETE** | Implemented in `624e50ba`. Substrate wake and deduplication bugs repaired in `7cf4050b`, `bb17d0ef`, `7574d899`. `rlm:complete:1` proves live reduction commit. |
| **Step 5: Bounded Coalescing & Role Bounds** | Quiescence debounce (500ms) and role-bounded `choir.Spawn()`. | **IMPLEMENTED (Focused Tests)** | Implemented in `internal/actor/coalesce.go` and `internal/agentcore/tool_profiles.go`. Unit-tested; production live multi-agent fan-out not exercised by single-agent Option B arc. |
| **Step 6: Tool Surface Cutover & Live Staging Proof** | Remove ambient JSON tools from CoSuper; execute end-to-end self-development task. | **IN PROGRESS (~80%)** | Sealed overlay (`tools=6`), exact Super bind, verbatim cell eval, and in-capsule proof file write are proved exit 0. Substrate receipt resolution fixes landed (`c794915e`, `2c8904e3`); awaiting CI deploy and fresh live proof on Node B. |

---

## 8. Final Landing Loop & Immediate Next Actions

With code fixes landed and approved by 7/8 consensus panelists, the path to full Step 6 completion is unambiguous:

1. **Monitor GitHub Actions CI for commit `2c8904e3`**:
   - Verify that test suites and `Deploy to Staging (Node B)` pass.
2. **Verify Staging Deployed Identity**:
   - Confirm `https://choir.news/health` reports `x-choir-build-commit: 2c8904e3...`.
   - Execute an authenticated microVM refresh on Node B with `actuator=rlm` to boot the new guest image containing the updated `capsule-broker` and `autoputer` runtime.
3. **Execute Fresh Sealed Option B Proof on Node B**:
   - Issue one targeted Texture-tell to document `d599c4b1`.
   - Observe Super exact bind $\rightarrow$ CoSuper verbatim cell execution via `capsule_go_eval` $\rightarrow$ `record_assignment_result` with exact `receipt_ref` $\rightarrow$ capsule freeze $\rightarrow$ granted execution receipt committed $\rightarrow$ terminal assignment status `pass`.
   - Verify restore fence `99949fe2` remains untouched and effects remain `propose_only`.
