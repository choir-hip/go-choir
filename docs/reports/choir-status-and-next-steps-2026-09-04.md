# Choir Engineering Status Report — September 4, 2026

**Subject**: Architecture, Verification, and Rollout Plan for the Recursive Language Model (RLM) Session Interpreter  
**Status**: Target Architecture Rev 3 Prepared for Owner Review  
**Staging Environment**: Retained Computer `computer-03335285269bdba4f94377e56879f9e6` (Boot Epoch 879, Commit `8c410a0d`)  

---

## 1. Executive Summary

Choir is building an autonomous computer system capable of supervised self-development. A central component of this platform is the execution harness for autonomous agents operating inside isolated microVM environments, known as **capsules**.

Currently, autonomous agents interact with their capsule workspaces by making repeated, individual JSON tool calls across the network (e.g., calling `capsule_read_file`, `capsule_write_file`, and `capsule_exec` one by one). This approach introduces significant latency, causes state fragmentation between turns, and increases the risk of network deadlocks.

To resolve these limitations, we are transitioning the agent harness to a **Recursive Language Model (RLM) session interpreter**. Under this architecture, the agent interacts with its workspace through a single execution doorway: `capsule_go_eval(source)`. Within this doorway, the agent writes concise Go code that runs inside a long-lived, stateful interpreter session (powered by Yaegi). Variables, functions, and imported packages persist across execution steps, much like an interactive computational notebook. The agent manipulates its workspace using a lightweight, built-in standard library named `choir`.

This report synthesizes the work completed to date, documents the architectural bugs identified and repaired across multiple review panels, explains the unified target architecture, and outlines the step-by-step engineering roadmap for deployment.

---

## 2. Current Deployed and Verified State

The foundational session interpreter infrastructure has been deployed to the staging environment at commit `8c410a0d` and validated on the retained test microVM:

* **Dual-Mode Route Switch**: The platform supports both the legacy individual JSON tool suite and the new persistent session mode, selectable via a centralized configuration switch. The legacy tool suite remains the active default. If the session worker fails to initialize for any reason, the system automatically falls back to legacy tools and records an explicit diagnostic flag on the execution receipt.
* **Persistent In-Guest Session**: Each agent assignment receives a dedicated, long-lived interpreter process. A bidirectional startup handshake verifies that the interpreter process is fully initialized and operational before accepting agent commands. If an interpreter process ever crashes, it is immediately discarded and replaced with a fresh process.
* **Built-in In-Capsule Standard Library (`choir`)**: The interpreter exposes core workspace operations to agent code, including `choir.ReadFile`, `choir.WriteFile`, `choir.ListDir`, `choir.Exec`, and `choir.Context`. Three early prototype methods (`Assign`, `Outcome`, and an un-routed `Message`) have been audited and are slated for clean removal and replacement (detailed in Section 4).
* **Process Group Isolation and Rapid Reaping**: Runaway, infinite-looping, or timed-out programs are isolated in dedicated process groups. When an execution deadline expires, the harness issues a `SIGKILL` to the entire process group and verifies that all child processes are completely reaped within 500 milliseconds. This behavior is covered by automated regression tests.
* **Multi-Layer Role Security**: Read-only research agents are strictly prevented from writing files or executing commands. This restriction is enforced independently at both the host boundary and inside the in-guest runtime, verified by end-to-end integration tests.
* **Sealed Tool Surface**: When session mode is active, the agent's visible tool list is reduced from ten discrete tools to a minimal set: the primary execution doorway (`capsule_go_eval`) plus necessary verification and transaction tools. Automated tests assert the exact tool surface in both operational modes.

<div class="page-break"></div>

---

## 3. Audits and Resolved Defects

Across three independent multi-model consensus review rounds (encompassing fifteen panel evaluations), several subtle failure modes in early prototype code were identified, reproduced, and repaired before reaching production:

* **Worker Initialization Failure (Unpopulated Actor Identity)**: In early prototypes, the interpreter worker process was launched with an empty actor identity data structure. Because the worker's own initialization routine required a valid identity, the process immediately exited on startup with exit code 2. However, the host supervisor incorrectly interpreted process creation as operational readiness. We introduced a mandatory bidirectional startup handshake alongside end-to-end boot tests to guarantee operational readiness before dispatching work.
* **Privilege Separation Leak in Prototype Session Worker**: Early draft code did not adequately scope permissions inside the interpreter process, which would have allowed a read-only research agent to execute arbitrary commands if session mode had been enabled. We implemented role-scoped capability checks at the interpreter boundary and deployed them *before* fixing the startup handshake, ensuring an invalid configuration could never inadvertently open an escalated execution path.
* **Unregistered Session Control RPCs**: Commands to explicitly start or terminate interpreter sessions were initially rejected across all agent roles due to an overly restrictive API allowlist, and fallback behavior depended on a hardcoded flag. Session lifecycle verbs are now properly registered, role-authorized, and covered by automated tests.
* **Absence of a Dynamic Kernel Boot Parameter Channel**: Staging analysis revealed that there was no supported mechanism to pass the session mode switch dynamically from the owner's refresh command down to the guest microVM bootloader. We mapped out the necessary control plane path (Machine Setting $\rightarrow$ `choir.actuator` kernel boot parameter $\rightarrow$ Guest Init) and placed this item first in the deployment sequence.
* **System Prompt Inconsistency**: The agent's system prompt instructions continued to describe the legacy tool suite as the exclusive interface, without mentioning `capsule_go_eval` or the `choir` package. A mode-aware prompt generation mechanism was specified to ensure the agent receives instructions matching its active tool surface.

<div class="page-break"></div>

---

## 4. Target Architecture: Unified Broker and Post-Cell Reduction

The multi-model review panels converged on a clean, unified target architecture (documented in `docs/designs/rlm-target-architecture-2026-09-04.md`, Revision 3). This design resolves architectural debt that accumulated during early prototyping.

### The Core Architectural Principles

1. **Local Operations Are Synchronous Syscalls**: Operations that affect only the local capsule workspace (reading files, writing files, executing programs) are executed synchronously inside the guest microVM via direct pipes to the capsule broker. They never require network roundtrips to the host supervisor, completely eliminating mid-cell distributed deadlocks.
2. **External Effects Are Staged Intents**: Any action that affects an external actor or requires durable persistence (sending messages to other agents, reporting task completion) must not attempt direct network calls mid-cell. Instead, the cell stages these actions as structured *intents* into a local, in-memory tray.
3. **The Host Reducer Has Exclusive Authority**: When an execution cell completes, its buffered intent tray is returned to the host supervisor alongside the execution result. The host reducer validates permissions, applies authoritative envelope metadata, writes messages to the durable Dolt database, binds real execution receipts, and triggers supervisor notifications. No in-cell function can claim delivery or completion before the host has committed the transaction.

![Target Architecture Topology](diagrams/diagram-architecture-topology.svg)

### In-Capsule Library Surface Cleanup

During early prototyping, three functions were added to the `choir` library without complete backend integrations:
* `choir.Assign`: Intended for sub-agent delegation, but lacked any supporting host infrastructure. **Action: Deleted**.
* `choir.Outcome`: Implemented merely as a synthetic message sent to the calling agent's own address, without connecting to the host verification or task fate pipelines. **Action: Removed**.
* `choir.Message`: Originally buffered payloads in volatile worker memory while prematurely returning status "delivered". **Action: Redesigned** as an asynchronous intent that is persisted and routed by the host reducer post-cell.

<div class="page-break"></div>

---

## 5. Resolving the Command Execution Divergence

A key architectural finding from the audit is that the codebase currently contains **two competing command execution implementations** that evolved independently in different packages. Because they were developed separately, they exhibit sharply contrasting security and operational characteristics.

![Command Execution Divergence](diagrams/diagram-command-execution.svg)

### Technical Comparison of the Two Runners

1. **The Inner Runner (`internal/yaegikernel/broker.go`)**:
   * **Execution Mechanism**: Invokes programs directly via `exec.Command(binary, args...)`. It bypasses the shell entirely, eliminating wildcard expansion, pipe parsing, and shell injection vulnerabilities.
   * **Environment Sanitization**: Enforces a strict environment allowlist (`PATH`, `HOME`, `TMPDIR`). Any caller-provided environment variables are filtered to strip sensitive credentials, including `CHOIR_*`, `*KEY*`, and `*TOKEN*`.
   * **Process Management**: Configures dedicated process groups (`Setpgid: true`). On timeout or cancellation, a `SIGKILL` is sent to the process group, cleanly terminating all child processes.

2. **The Outer Runner (`cmd/capsule-broker/main.go`)**:
   * **Execution Mechanism**: Executes commands through a shell string wrapper (`sh -c '<command>'`). While flexible, this parses shell syntax, pipes, and wildcards, and introduces shell dependency risks.
   * **Environment Sanitization**: Inherits the capsule broker daemon's entire process environment (`os.Environ()`), inadvertently exposing ambient daemon tokens and host credentials to executed commands.
   * **Process Management**: Shares the capsule broker's process group (`Setpgid: false`). When a command times out, the system can only terminate the top-level shell process, frequently leaving orphaned child processes running in the background.

### The Unified Resolution

The consensus decision is clear: **the inner runner's direct-argv execution semantics must become the single canonical standard across the entire platform**.

We will implement these direct-argv, sanitized-environment, process-group-isolated semantics natively within the primary in-guest daemon (`cmd/capsule-broker`). The legacy shell-based runner will be frozen strictly for emergency rollback of legacy JSON tools, and will be completely unreachable whenever RLM session mode is active.

<div class="page-break"></div>

---

## 6. Intent Staging and Post-Cell Reduction Lifecycle

To guarantee that agent code cannot cause distributed deadlocks or forge delivery confirmations, all inter-agent communication and task settlement follow a strict three-phase lifecycle.

![Intent Lifecycle and Reduction](diagrams/diagram-intent-lifecycle.svg)

### Phase 1: In-Cell Execution (Guest MicroVM)
Agent code executes within the Yaegi interpreter worker. File operations and command executions take place immediately as local guest syscalls. When the agent calls `choir.Message(to, body)` or `choir.Complete(...)`, the worker does not attempt external network communication. Instead, it assigns a local correlation ID, validates that cell quotas are respected, and appends the structured intent to an in-memory staging tray.

### Phase 2: Frame Packaging and Transport
When the Go cell finishes executing (or when its execution timeout expires), the worker packages the execution status, captured standard output and error, an execution manifest (listing commands run, exit codes, and output hashes), and the staged intent tray into a single structured response frame. This frame is returned atomically across the Unix domain socket to the host supervisor.

### Phase 3: Host Reduction and Settlement
The host reducer receives the completed frame and processes the intent tray:
1. **Validation & Enveloping**: The host verifies that the sender holds valid authorization, injects authoritative timestamps and sequence ordinals, and wraps each message in a fixed schema envelope (`schema_version = v1`, `Kind = evidence_update`).
2. **Durable Mailbox Persistence**: The host commits the enveloped messages to the Dolt database. Durable, globally unique message IDs are minted at this step.
3. **Supervisor Notification & Assignment Fate**: For messages, the host triggers a dedicated, coalesced wake signal to the recipient's supervisor. If the cell submitted a `Complete` intent, the host binds the recorded command execution receipts, updates the assignment status, and revokes active access tokens.

<div class="page-break"></div>

---

## 7. Key Protocol Decisions

The multi-model review panels reached formal consensus on several critical operational parameters:

* **Typed Completion Contract**: Task completion is invoked via `choir.Complete(result, verdict, summary, evidence_refs)` where `result` must be one of `{completed, failed, blocked}`. A cell cannot supply execution receipts directly; the host reducer binds authoritative command receipts from the capsule broker upon settlement. At most one `Complete` intent is permitted per assignment.
* **Host-Controlled Message Envelopes**: Agent cells specify only the recipient address (`to`) and the payload (`body`). All routing headers—including sender identity, originating role, delivery direction, timestamp, and message kind—are authoritatively generated by the host. Messages can only be addressed to the assigned parent supervisor.
* **Dedicated Mailbox Wake Channel**: To avoid interference with legacy lifecycle suppression logic, mailbox deliveries trigger a dedicated, coalesced wake signal. Replayed messages and messages arriving after assignment cancellation do not trigger wakes.
* **Operational Quotas and Bounded Buffers**: To prevent infinite loops or memory exhaustion:
  * Maximum 16 intents per execution cell.
  * Maximum 16 KiB per message body.
  * Maximum 256 KiB aggregate tray payload per cell.
  * Strict per-activation quotas enforced by the host reducer.

---

## 8. Phased Implementation Sequence

To ensure safety and maintain full rollback capability at every stage, implementation will proceed in six strictly ordered steps. No step will begin until the preceding step has passed automated verification:

| Step | Component | Description | Rollback Strategy |
|:---:|:---|:---|:---|
| **1** | **Control Plane Boot Channel** | Wire the machine setting through `choir.actuator` into the guest microVM boot parameters. | Revert commit; guest defaults to legacy tools. |
| **2** | **Multiplexed Transport Pipe** | Establish the dedicated Unix domain socket and frame protocol between the worker and broker. | Disable transport flag; falls back to stdin pipe. |
| **3** | **Canonical Command Runner** | Implement direct-argv, sanitized-environment command execution in `cmd/capsule-broker`. | Frozen legacy shell runner remains available for tools mode. |
| **4** | **Intent Tray & Host Reducer** | Deploy in-memory tray buffering in Yaegi and the host reduction engine in `internal/agentcore`. | Disable RLM mode; messages route through legacy tools. |
| **5** | **Tool Surface & Prompt Update** | Update agent tool profiles to expose only `capsule_go_eval`; enable mode-aware system prompts. | Revert profile configuration to expose legacy tool list. |
| **6** | **Live Staging Verification** | Run an end-to-end self-development task on the retained staging microVM; verify all receipts. | Reset microVM to pre-test checkpoint `99949fe2`. |

---

## 9. Conclusion and Next Actions

The architectural design for the RLM session interpreter is fully specified, vetted by fifteen panel reviews, and grounded in the realities of the existing codebase. By unifying command execution semantics, separating guest-local syscalls from host-mediated intents, and enforcing authoritative host reduction, this design eliminates the distributed deadlocks and privilege ambiguities present in earlier prototypes.

**Immediate Next Action**: Following owner approval of this report and the companion specification (`docs/designs/rlm-target-architecture-2026-09-04.md`), implementation will commence with Step 1 (the control plane boot parameter channel).
