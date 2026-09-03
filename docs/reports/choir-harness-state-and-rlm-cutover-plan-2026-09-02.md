# Choir Harness State and RLM Cutover Plan

*Repository audit, doctrine reconciliation, and agentic-consensus verdict ahead of autonomous engineering. 2026-09-02.*

---

## 1. Executive Synthesis & Consensus Verdict

An independent agentic consensus panel across 9 frontier models (Claude Fable 5.1, Codex/GPT-5.6-Sol, GPT-5.6-Luna, Cursor/Grok-4.6, Gemini-3.8-Flash, GLM-5.3-Flash, Cursor Agent, and Opencode) evaluated the repository state, the external architecture audit, and the path forward.

### Unanimous Verdict: Option B — Three Sequential Definitions
The panel **unanimously rejected** co-landing substrate repairs with the RLM interpreter (Option A) and rejected a monolithic milestone (Option C).

```text
Def 1: Substrate & Scheduling Readiness
  └── FIFO execution request contract; normal-boot characterization/fix
       ↓ (Must pass on non-held computer before Def 2 touches code)
Def 2: RLM Session Interpreter & Ambient Parity
  └── Persistent Yaegi session per activation; prebound modules; JSON tools dropped
       ↓ (Must yield sealed multi-eval proof on retained VM)
Def 3: Supervised Self-Development on RLM
  └── Candidate A solitaire proof; promotion; live play; falsification; restore to 99949fe2
```

### Why Option B is mandatory under Choir Doctrine:
1. **Isolates distinct failure domains**: Def 1 mutates Host Lifecycle and Dolt tables (`internal/agentcore` scheduling, `internal/vmctl`, guest image); Def 2 mutates the in-guest actor runtime (`cmd/capsule-broker` and `internal/yaegikernel`); Def 3 exercises autonomous agency.
2. **Prevents the 2026-08-26 deadlock**: On August 26 (`choir-yaegi-private-go-actor-kernel-reorientation-report-2026-08-26.md`), 11 of 12 kernel deliverables passed, but autonomous progress stalled completely because the retained microVM could not boot through the product path to provide the live sealed activation receipt. Combining vmctl boot fixes with Yaegi session persistence creates a circular dependency: you cannot verify the interpreter without a bootable VM, and you cannot isolate a VM boot regression while mutating the in-guest actor runtime.
3. **Attribution and clean rollback**: Def 1 rolls back via Git revert of substrate/scheduling commits. Def 2 rolls back via runtime flag flip (`actuator=tools`). Def 3 rolls back via event-chain restore to pre-A checkpoint `99949fe2`.

---

## 2. Grounded Architecture Mapping

Auditing the codebase against the external audit document (`choir-harness-repo-architecture-audit.md`) and Thesis v3 revealed critical facts about the real system:

### 2.1 The Live Actuator Path vs. Prototype Sub-stack
A crucial finding emerged from source analysis:
- **`cmd/capsule-broker/main.go` is the real live actuator**: It runs inside the guest capsule namespace, receives Unix RPC from `agentcore`, and executes `go_eval` by spawning itself in `--exec-go-stdin` worker mode via `yaegikernel.ExecuteWorkerStdin()`.
- **`internal/yaegikernel/broker.go` is an unused prototype**: Its five-verb flat DTO was part of an earlier sub-stack. Def 2 must wire session persistence and prebound modules into the **real live path** (`cmd/capsule-broker`), not the prototype broker.

### 2.2 Causal Event Log vs. Canonical Semantic State
- **Causal History**: Implemented in `internal/store/computer_events.go` appending to Dolt table `computer_events`. It answers *what happened*.
- **Canonical Semantic State**: Implemented in `internal/store/texture.go` and `internal/agentcore/texture.go`. Texture documents in Dolt *are* the versioned, diffable semantic state. There is no separate abstract "state plane" engine; external thesis v3's proposed "Phase B — Canonical semantic-state hardening" is rejected as speculative scaffolding.

### 2.3 Settled Policy Bounds
- **Claude / Prompt Caching**: Deferred for much later per owner decision. The audit confirmed `provider.go:495` already sets `CacheControl: {"type": "ephemeral"}` on Bedrock system prompts, but cache-read token telemetry is not yet parsed. This remains low priority and will not gate the cutover.
- **Outbound Email**: Verified live via Resend (`internal/maild/resend.go`). It is draft-first with manual owner approval. Full autonomous sending is deferred until after World Wire.
- **World Wire**: Out of scope until self-development is stable on the RLM.
- **Telemetry**: Tool-call failure telemetry (H-OOD) is collect-only; no reactive fail-over policy will be built during the cutover.
- **Naming**: "Agents are desks, not people" (management, engineering, research). Naming cutover is its own dedicated AST-grep mission after candidate A.

---

## 3. The 3-Definition Execution Sequence

### Definition 1 — Substrate & Scheduling Readiness
**Goal**: Deliver a staging computer that boots normally through the product path and executes exactly one Texture request at a time without ping-pong cancellation.

*Scope & Actions*:
1. **FIFO Scheduling Contract**:
   - Competing pending Texture `execution_request`s must execute in order of computer-scoped arrival ordinal (`internal/store/lifecycle.go:1306-1379`).
   - Stale duplicate requests whose operations have terminal attempts must settle as late evidence without spawning new assignments.
   - Settle the ping-pong cancellation loop where Super cycles repeatedly superseded predecessors.
2. **vmctl Normal-Boot Characterization & Fix**:
   - Characterize the 3/3 normal-boot hang vs 4/4 hold-param boot.
   - Prove normal-boot start of a non-held computer reaches `/health` 200 without `RUNTIME_MAINTENANCE_HOLD`.
   - Reconcile cold-recover blank-image / credential.img failure paths if required for recovery.
3. **Acceptance Gate**:
   - Two competing Texture requests queued on staging $\rightarrow$ exactly one live assignment across $\ge 3$ Super cycles.
   - Deadline-bounded boot probe: $\ge 5$ consecutive normal boots without hold parameters reaching guest health inside existing bounds.

### Definition 2 — RLM Session Interpreter & Ambient Tool Parity
**Goal**: Deliver a deployed, sealed, candidate-A-ready RLM surface where CoSuper acts exclusively through model-written Go in a persistent session interpreter.

*Scope & Actions*:
1. **Session Interpreter in `cmd/capsule-broker` & `yaegikernel`**:
   - Evolve `eval.go` from fresh-per-eval `interp.New` to a persistent `Session` worker per activation.
   - Support multiple sequential cells within one activation: cell 1 defines `x := 41`, cell 2 evaluates `x + 1` $\rightarrow$ returns `42`.
2. **Prebound Module Surface**:
   - Host exports a prebound `choir` package via `interp.Exports`:
     - `choir.ReadFile(path)`
     - `choir.WriteFile(path, data)`
     - `choir.ListDir(path)`
     - `choir.Exec(cmd, args)`
     - `choir.Assign(spec)`
     - `choir.Message(target, payload)`
     - `context` DTO (read-only mission state, computer ID, epoch, budget)
     - `outcome.{Complete, Park, Refuse, Fail}`
   - Prebound prelude pre-imports allowlisted packages so model code never hits Yaegi "redeclared in this block" errors.
3. **Process-Group Containment & Poison Handling**:
   - Any panic, timeout, or output overflow SIGKILLs the session process group (`Setpgid`).
   - Session marks poisoned; next cell gets a fresh process worker and rebinds from the last clean snapshot (`InterpreterEpoch++`).
4. **Ambient Tool Parity & Deprecation**:
   - Parity verification via golden transcript fixture corpus (five sealed cells on a disposable capsule matching the observable contract of today's JSON tools: same path jail, same refusal receipts, same output bounds).
   - Drop CoSuper's JSON tool definitions (`capsule_exec`, `capsule_read_file`, etc.) from the model-facing prompt schema under the RLM profile.
5. **Acceptance Gate**:
   - Multi-eval integration test passing: compute $\rightarrow$ observe $\rightarrow$ think $\rightarrow$ compute on retained result.
   - Retained-VM live sealed proof on staging: CoSuper writes Go cells that compute, read via broker, retain state, and freeze an artifact, with zero JSON tools in prompt.

### Definition 3 — Supervised Self-Development on RLM
**Goal**: Execute autonomous in-VM self-development under effect-specific multiagent consensus, authored entirely through the new RLM surface.

*Scope & Actions*:
1. Resume candidate A solitaire implementation inside the guest capsule.
2. CoSuper authors, builds, tests, freezes 5 bundle refs, and proposes candidate A using model-written Go.
3. Qualified consensus panel approves exact subject under `reversible-selfdev-v1`.
4. Promote to live realization; verify live play API.
5. Falsify candidate A with candidate B.
6. Acceptance-fenced restore to pre-A checkpoint `99949fe2`.
7. **Acceptance Gate**:
   - Candidate A successfully promoted, verified, falsified, and restored to `99949fe2`.
   - Every authoring step executed via the session interpreter without JSON tool invocation.

---

## 4. Top Risks & Earliest Mechanical Checks

| Priority | Failure Mode | Impact | Earliest Mechanical Check |
|---|---|---|---|
| **P0** | **vmctl boot-death recurrence** | Staging computer hangs or loops during live proof, deadlocking the mission. | Automated pre-flight probe in Def 1: 5 consecutive normal boots with hard health timeout before code edits begin. |
| **P0** | **Interpreter runaway / goroutine leak** | A model writes a tight CPU loop or leaked goroutine that ignores cooperative cancellation. | Sidecar `Setpgid` SIGKILL boundary test: verify process group is killed and reaped within 500ms of context cancellation. |
| **P1** | **Import redeclaration friction** | Model writes `import "fmt"` in subsequent cells, failing with Yaegi redeclaration errors. | Prebound prelude test: session worker pre-imports stdlib allowlist; AST cell-splitter automatically strips duplicate user imports. |
| **P1** | **Scheduling regression** | Competing Texture requests re-trigger supersession loops, killing CoSuper mid-authoring. | FIFO invariant test in Def 1: queue 3 requests, assert arrival-ordinal strict execution. |
| **P2** | **Session memory bloat** | Persistent session worker retains large byte buffers across turns, exceeding capsule cgroup. | Session memory cap and `maxEvalOutputBytes` enforcement with explicit OOM rejection receipt. |

---

## 5. Review Adjudication & Execution Readiness

A second independent consensus review across 10 frontier models (Claude Fable 5.1, Codex/GPT-5.6-Sol, GPT-5.6-Luna, Cursor/Grok-4.6, Gemini-3.8-Flash, GLM-5.3-Flash, Cursor Agent, Opencode, Nemotron-3-Ultra) evaluated the authored 3-Definition package.

### Review Verdict & Incorporated Repairs:
The panel confirmed Option B as the correct architecture and identified necessary authority and schema repairs, all of which have been executed:
1. **Single State Authority**: Explicitly reconciled `computer-03335285269bdba4f94377e56879f9e6` (epoch 831+) and pre-A restore fence `99949fe2` across all three Definitions.
2. **Definition 1 (Substrate & Scheduling)**: Completed the full I26 contract (multi-request arrival ordering, restart mid-run selection preservation, stale duplicate settlement, 5x normal-boot probe without maintenance hold), added Conjectures and Heresy deltas, and demoted cold recovery out of the critical path.
3. **Definition 2 (RLM Cutover)**: Set `now.status: blocked_incomplete` to enforce strict serialization, defined mechanical rollback via runtime route flag (`actuator=tools`), added P0 500ms sidecar process-group SIGKILL containment gate, and included `internal/agentcore/tool_profiles.go` in protected surfaces.
4. **Definition 3 (Candidate A Self-Dev)**: Set `now.status: blocked_incomplete`, restored all five named bundle refs, inherited the full predecessor acceptance suite (`reversible-selfdev-v1` consensus with gpt-5.6-terra independent reviewer, promotion, live play API, falsification, and restore), and added red ceremony deltas.
5. **Registry Hygiene (Standing Question 9)**: Updated `docs/ACTIVE.md`, `docs/mission-graph.yaml`, and `docs/doc-authority-manifest.yaml`. Predecessor `choir-scheduling-and-candidate-proof-2026-08-21.md` is superseded. Definition 1 is registered as the sole active executable entrypoint; Definitions 2 and 3 are registered as blocked/queued.

### Execution Status:
**Definition 1 (`docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md`) is approved and ready for immediate autonomous execution.**
Next action: execute 5x normal-boot probe on `computer-03335285269bdba4f94377e56879f9e6`, then run the competing 2-request FIFO selection test on staging.
