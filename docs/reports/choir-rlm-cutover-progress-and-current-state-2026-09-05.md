# Choir RLM Architecture Cutover: Autonomous Run Progress & Current State Report

**Date**: September 5, 2026  
**Subject**: Comprehensive analysis of the ~12-hour autonomous cutover run under Grok 4.6 on mission `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`, incorporating Agentic Consensus Panel Review (7 models: Claude Opus, GPT-5.6 Sol, GPT-5.6 Terra, Gemini 3.8 Flash, Grok 4.6 High, Cursor Agent, OpenCode)  
**Current Git HEAD**: `dd6a8f040b05896fccba2091b78a9d38d0348509` (`main`)  
**Staging Host**: `https://choir.news` (`x-choir-build-commit: a281f1c0df394a719fb48fdb7f61af9dffcac5d3`)  
**Staging Retained Computer**: `computer-03335285269bdba4f94377e56879f9e6` (VM `candidate-fleet-e15cb89f25d963c220319b7b`, realization epoch **886**, `actuator=rlm`)  
**Pre-A Checkpoint Restore Fence**: `99949fe2e16d...` intact, effects `propose_only`

---

## 1. Executive Summary

Over the ~12-hour autonomous execution window (2026-09-04T22:31:01Z through 2026-09-05T20:08:08Z), the coding harness executed 19 commits (+4,345/-404 total lines across packages) progressing the Recursive Language Model (RLM) Target Architecture cutover from design proposal (Rev 5) to live staging execution on Node B.

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

### The Immediate Blocker & Consensus Adjudication
Following the successful exit-0 `capsule_go_eval`, CoSuper called `record_assignment_result` to seal the assignment. The call failed with `executor receipt unavailable` before freeze/grant could be committed. 

The multi-model agentic consensus panel (7 independent models) unanimously confirmed the root cause:
- **The failure is a schema-naming defect**: `GoEvalResult` (`types.go:119-128`) returns both `receipt_ref` (scalar string containing `capsule-go-eval:sha256:...`) and `receipts` (string slice populated with `rlm:complete:1` by `reduction.commit`). When `record_assignment_result` requests `execution_refs`, the model passes either `["rlm:complete:1"]` or both. `OpenExecutionReceipt` fails closed on `rlm:complete:1` because it is an in-memory intent sequence ID, not an `ExecutionReceipt`.
- **Silent filtering of `rlm:*` is rejected**: Filtering worker-local tokens would allow a model that passed only `["rlm:complete:1"]` to seal a terminal `pass` with zero execution evidence (`0==0` pass).
- **The required fix**: (1) Rename/separate `GoEvalResult.receipts` $\rightarrow$ `staged_intent_ids` so the model receives only one execution receipt reference; (2) fail closed with early prefix validation in `OpenExecutionReceipt`; (3) require $\ge 1$ valid execution receipt for terminal `completed`+`pass`.

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

Between baseline `de93d6aa` and HEAD `dd6a8f04`, 19 commits landed on `main`:
- **Red (Runtime / Substrate Changes)**: 9 commits (+3,545 / -296 lines)
- **Green (Docs / Evidence / Manifests)**: 10 commits (+800 / -108 lines)

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
3. **Receipt Resolution**: The tool immediately executes:
   ```go
   receipts, err := toolCtx.Executor.ResolveExecutionReceipts(ctx, executionRefs)
   if err != nil {
       return nil, err
   }
   ```
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
6. **Error Return**: An error returned from `OpenExecutionReceipt` halts `record_assignment_result` immediately, before `recordAssignedCoSuperReport` or `bindFrozenAssignmentExecutionReceipts` can be invoked.

### Causal Hypotheses & Discriminators

The static source and consensus review rule out several potential explanations:
- **Not a naming hash discrepancy**: `persistReceiptArtifact` and `OpenExecutionReceipt` use the exact same `receiptArtifactName(ref)` helper (`sha256(ref).json`).
- **Not prefix validation failure**: A receipt with an invalid prefix or mismatched SHA256 digest returns `executor receipt digest mismatch` or `executor receipt is invalid`, **not** `executor receipt unavailable`. The `unavailable` error occurs strictly when both the in-memory map lookup and the disk `os.ReadFile` fail.
- **Not an Autoputer or Firecracker restart (Hypothesis B excluded)**:
  - Staging telemetry confirms Firecracker PID 3052070 ran continuously from 19:55:16Z without reboot.
  - `record_assignment_result` first checks `requireCapsuleRole`. An autoputer restart would destroy live capability keypairs, causing earlier failure on `capsule authority unavailable`.
  - In `GoEval` (`executor.go:664-666`), `receipt.ReceiptRef` is inserted into `e.executionReceipts` *before returning*. In the same process, a lookup for that exact string is a guaranteed in-memory map hit.

### The Confirmed Primary Mechanism: Schema Naming Contamination
- In `internal/capsule/types.go:119-128`, `GoEvalResult` serializes:
  ```go
  type GoEvalResult struct {
      Stdout     string        `json:"stdout"`
      Stderr     string        `json:"stderr"`
      Error      string        `json:"error,omitempty"`
      Duration   time.Duration `json:"duration,omitempty"`
      ExitCode   int           `json:"exit_code"`
      ReceiptRef string        `json:"receipt_ref,omitempty"`
      Fallback   bool          `json:"fallback,omitempty"`
      Receipts   []string      `json:"receipts,omitempty"` // populated with rlm:complete:1
  }
  ```
- The prompt instructs CoSuper: *"Report typed intermediate progress and the one terminal assignment result through record_assignment_result with ... exact execution_refs."*
- Faced with a scalar `receipt_ref` and an array field named `receipts: ["rlm:complete:1"]`, LLMs predictably pass `execution_refs: ["capsule-go-eval:sha256:7fe...", "rlm:complete:1"]` (or only `["rlm:complete:1"]`).
- `OpenExecutionReceipt("rlm:complete:1")` executes:
  1. Map lookup `e.executionReceipts["rlm:complete:1"]` $\rightarrow$ **MISS** (never stored).
  2. Disk lookup `stateDir/receipts/execution/sha256("rlm:complete:1").json` $\rightarrow$ **MISS** (does not exist).
  3. Returns `executor receipt unavailable`.
- `ResolveExecutionReceipts` fails the entire batch on the first error, aborting before capsule freeze.

---

## 6. Agentic Consensus Synthesis

A 7-model agentic consensus panel was run across diverse model families and reasoning tiers:
- **Claude Opus** (`claude -p --model opus`)
- **OpenCode** (`opencode run`)
- **Cursor Agent** (`agent --mode ask`)
- **GPT-5.6 Sol** (`omp --model openai-codex/gpt-5.6-sol --thinking medium`)
- **GPT-5.6 Terra** (`omp --model openai-codex/gpt-5.6-terra --thinking xhigh`)
- **Gemini 3.8 Flash** (`omp --model google-antigravity/gemini-3.8-flash --thinking high`)
- **Grok 4.6 High** (`omp --model cursor/cursor-grok-4.6-high --thinking high`)

### Panel Verdicts
- **Approve with Changes (5/7)**: Claude Opus, OpenCode, Cursor, Gemini 3.8 Flash, Grok 4.6 High.
- **Block (2/7)**: GPT-5.6 Sol, GPT-5.6 Terra.
  - *Basis of Block*: Both Sol and Terra blocked claiming Step 6 is "90% accepted" and blocked retrying the sealed proof on Texture-tell instructions alone without landing the structural code repair. Sol also noted that Step 5's coalescer is not yet wired to a production consumer.

### Consensus Findings
1. **Unanimous Root Cause Agreement**: All 7 models agreed that `GoEvalResult` dual-naming (`receipt_ref` vs `receipts`) directly caused the model to submit `rlm:complete:1` into `execution_refs`.
2. **Rejection of Silent Filtering**: All 7 models emphatically rejected silently filtering `rlm:*` in `record_assignment_result`. Silent filtering allows an assignment that submitted only `["rlm:complete:1"]` to collapse to `execution_refs: []`, which bypasses grant binding and lands a false "green" pass with zero execution evidence.
3. **Rejection of Wholesale StateDir Relocation**: All 7 models advised against moving `CHOIR_CAPSULE_STATE_DIR` to `/mnt/persistent/` as an immediate fix. The state directory contains disposable capsule mounts and upper layers; persisting them violates the Spatial Isolation Invariant and causes disk leaks.
4. **Step 6 Completion Recalibration**: The panel unanimously concluded that Step 6 is not "90% accepted." While the in-capsule execution substrate is ~75% complete in code and staged execution, the Definition's completion gate requires a sealed freeze/grant receipt, which is a binary gate.

---

## 7. Definition Deliverables Matrix & Step Status

| Step | Scope | Status | Evidence & Verification |
| :--- | :--- | :--- | :--- |
| **Step 1: Control Plane Actuator Channel** | MicroVM boot parameter `choir.actuator=rlm` wired from `vmctl` through Firecracker. | **COMPLETE** | Staged and verified on Node B. Three-way readback holds on epoch 886 (`ownerships.json`, Firecracker cmdline, guest health). |
| **Step 2: Multiplexed Transport Pipe** | Dedicated Unix domain socket and frame protocol between worker and broker. | **COMPLETE** | Implemented in `internal/yaegikernel/transport.go` (`624e50ba`). Deployed and running inside microVM. |
| **Step 3: Canonical Command Runner** | Direct-argv allowlist and process-group SIGKILL reaping (<500ms). | **COMPLETE** | Implemented in `cmd/capsule-broker/` (`624e50ba`). Seccomp `setpgid` fixed in `3724db1a`. |
| **Step 4: Intent Tray, Reducer, Inbox** | In-memory tray buffering in Yaegi; Dolt reducer + Go channel mailbox delivery + two-phase ack. | **COMPLETE** | Implemented in `624e50ba`. Substrate wake and deduplication bugs repaired in `7cf4050b`, `bb17d0ef`, `7574d899`. `rlm:complete:1` proves live reduction commit. |
| **Step 5: Bounded Coalescing & Role Bounds** | Quiescence debounce (500ms) and role-bounded `choir.Spawn()`. | **IMPLEMENTED (Focused Tests)** | Implemented in `internal/actor/coalesce.go` and `internal/agentcore/tool_profiles.go`. Not yet exercised live on staging (Option B ran single-agent with no spawns). |
| **Step 6: Tool Surface Cutover & Live Staging Proof** | Remove ambient JSON tools from CoSuper; execute end-to-end self-development task. | **IN PROGRESS (~75%)** | Sealed overlay (`tools=6`), exact Super bind, verbatim cell eval, and in-capsule proof file write are proved exit 0. Blocked on `record_assignment_result` receipt resolution. |

---

## 8. Actionable Remediation Plan

To close the final acceptance gate cleanly and achieve full Step 6 completion:

### 1. Fix the Schema Producer (`internal/capsule/types.go` & `tools_capsule.go`)
- In `GoEvalResult`, rename or separate the intent sequence slice:
  ```go
  type GoEvalResult struct {
      ...
      ReceiptRef     string   `json:"receipt_ref,omitempty"`     // Raw ExecutionReceipt reference
      StagedIntentIDs []string `json:"staged_intent_ids,omitempty"` // Internal reduction intent sequence IDs
  }
  ```
- This eliminates the name collision that invites models to pass `receipts` into `execution_refs`.

### 2. Implement Early Prefix Validation & Fail Closed (`internal/capsule/executor.go` & `tools_capsule.go`)
- In `OpenExecutionReceipt(ref)`: Check for valid prefixes (`capsule-exec:sha256:`, `capsule-go-eval:sha256:`, `capsule-fate:sha256:`) *before* taking the read lock or attempting `os.ReadFile`.
- If an invalid prefix is passed (e.g. `rlm:*`), return an explicit error:
  `fmt.Errorf("receipt reference %q is an intent token, not an execution receipt (expected capsule-go-eval:sha256:*)", ref)`
- In `tools_capsule.go:record_assignment_result`: Require $\ge 1$ valid execution receipt for terminal `pass`, closing the `0==0` empty-receipt hole.

### 3. Update Prompt Framing (`internal/runtimeprompts/overlays/rlm_co_super_runtime.yaml`)
- Update line 10 to explicitly specify the reference format:
  > *"Report typed intermediate progress and the one terminal assignment result through record_assignment_result with a concise summary, exact evidence_refs, and the exact capsule-go-eval:sha256:... receipt_ref in execution_refs (never cell-local rlm:* intent IDs)."*

### 4. Execute Fresh Live Staging Proof
- Deploy the schema and prefix validation fixes to Node B.
- Issue a single targeted Texture-tell to document `d599c4b1`.
- Verify the full execution arc: `capsule_go_eval` exit 0 $\rightarrow$ `record_assignment_result` resolves `receipt_ref` $\rightarrow$ capsule freezes $\rightarrow$ granted execution receipt committed $\rightarrow$ assignment fate reaches `pass` $\rightarrow$ computer restore fence `99949fe2` remains intact.
