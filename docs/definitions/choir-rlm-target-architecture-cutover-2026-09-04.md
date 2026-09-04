---
definition_version: 2
definition_id: choir-rlm-target-architecture-cutover-2026-09-04
execution_mode: mission_orchestrator

start:
  captured_at: "2026-09-04T16:20:00Z"
  source:
    canonical_ref: "main@de93d6aa"
    deploy_identity: "staging https://choir.news deployed_commit 8c410a0d94bc7afa4383f5942b83611540a27824; retained computer computer-03335285269bdba4f94377e56879f9e6 active realization_epoch 879; pre-A checkpoint 99949fe2 published restore fence intact; effects OFF"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-04 read-only git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns capsule broker, yaegikernel, actor runtime and reducers, and agent tool profiles."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "RLM Target Architecture Rev 5 (docs/designs/rlm-target-architecture-2026-09-04.md) is agreed and formally reviewed across an 8-model agentic consensus panel (Codex, Cursor, Sol, Terra, Luna, Gemini, Grok, Opencode) with 5 unanimous adjudications."
      evidence_ref: "docs/designs/rlm-target-architecture-2026-09-04.md; docs/reports/choir-status-and-next-steps-2026-09-04.md; .agentic-consensus/agentic-consensus-20260904-155821/manifest.tsv"
    - claim: "Codebase contains dual command execution implementations (internal/yaegikernel/broker.go direct-argv vs cmd/capsule-broker/main.go shell wrapper) requiring canonical unification."
      evidence_ref: "internal/yaegikernel/broker.go:448-503; cmd/capsule-broker/main.go:449-500"
    - claim: "In-cell Yaegi interpreter has ReadFile/WriteFile/Exec/Context but lacks Inbox(), Spawn(), and two-phase durable cursor acknowledgment."
      evidence_ref: "internal/yaegikernel/choir.go:1-203"
    - claim: "Autoputer actor runtime delivers via resident Go channels (mailbox chan Update) but lacks in-capsule tray framing and batch reduction."
      evidence_ref: "internal/actor/actor.go:88-120"
    - claim: "Pre-A checkpoint 99949fe2 remains published, verified, and intact on computer-03335285269bdba4f94377e56879f9e6."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md"
  unknowns:
    - "Exact latency overhead of Unix domain socket frame roundtrips vs in-memory channel delivery during rapid cell evaluations."

finish:
  deliver: "The automatic computer executes autonomous multi-desk orchestration (Management, Engineering, Research) exclusively through model-written Go in persistent Yaegi sessions inside dedicated capsules, with direct-argv command execution, non-blocking in-cell intent staging, Dolt log persistence, Go-channel mailbox delivery, cell-start Inbox snapshots with two-phase cursor commitment, bounded adaptive coalesced wakes, and legacy JSON tools removed from the model-facing schema."
  artifact: "Deployed and verified RLM Target Architecture across cmd/capsule-broker, internal/yaegikernel, internal/actor, and internal/agentcore, with live sealed proof on staging computer computer-03335285269bdba4f94377e56879f9e6."
  acceptance:
    - action: "Implement mechanical rollback flag (actuator=tools vs actuator=rlm) wired from machine boot settings through choir.actuator."
      proves: "Mechanical rollback to legacy JSON tools is guaranteed before any prompt schemas change."
      evidence_class: local_test
    - action: "Unify command execution in cmd/capsule-broker using direct-argv semantics with strict environment allowlist (PATH, HOME, TMPDIR, LANG=C.UTF-8) and process-group SIGKILL reaping <500ms."
      proves: "Shell injection vulnerabilities, process leakage, and credential exposure are eliminated from in-capsule execution."
      evidence_class: local_test
    - action: "Implement multiplexed Unix domain socket frame protocol between Yaegi session worker and capsule broker."
      proves: "Clean, non-blocking transport replaces raw stdin/stdout piping."
      evidence_class: local_test
    - action: "Implement in-cell intent tray (choir.Message, choir.Spawn, choir.Complete) returning in microseconds without blocking."
      proves: "In-cell execution never deadlocks on peer agents or external network calls."
      evidence_class: local_test
    - action: "Implement autoputer post-cell reduction: persist intent envelopes to Dolt, deliver updates to recipient Go channels (mailbox chan Update), and commit durable unread cursors only upon successful cell completion."
      proves: "Restart durability and crash recovery conform to doctrine ('The database remembers; Go delivers')."
      evidence_class: local_test
    - action: "Implement choir.Inbox() as a cell-start snapshot injected by autoputer, side-effect-free inside the cell."
      proves: "Preserves 'all context in the REPL' without blocking mid-cell network RPCs."
      evidence_class: local_test
    - action: "Implement bounded adaptive coalescing (debounce window, all-complete, timeout deadline, or error tombstones) for parent wakes."
      proves: "Eliminates distributed barrier deadlocks and token waste during fan-in."
      evidence_class: local_test
    - action: "Implement role-bounded fan-out in choir.Spawn and scoped fan-in enforcing worker reporting strictly to assigned return target."
      proves: "Prevents unauthorized role escalation and cross-desk message leakage."
      evidence_class: local_test
    - action: "Remove ambient JSON tool definitions from CoSuper prompt schema under RLM mode."
      proves: "CoSuper operates exclusively through model-written Go."
      evidence_class: local_test
    - action: "Execute live sealed CoSuper proof on staging computer computer-03335285269bdba4f94377e56879f9e6 with effects OFF."
      proves: "End-to-end RLM target architecture functions reliably on the physical staging microVM."
      evidence_class: deployed_proof
  rollback: "Runtime flag flip (actuator=tools) and Git revert; pre-A checkpoint 99949fe2 remains the immutable fence."
  landing:
    required: true
    environment: staging
    required_receipts: [source_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "cmd/capsule-broker still uses sh -c string execution for in-cell commands."
    - "Capsule broker inherits ambient daemon environment without strict allowlist."
    - "choir.Inbox() makes a live, blocking network call mid-cell."
    - "Unread mailbox cursor advances on a failed, timed-out, or poisoned cell."
    - "CoSuper prompt schema exposes ambient JSON tools when actuator=rlm is active."
    - "Parent wake uses an unbounded join barrier that can deadlock on child hangs."
    - "Linux namespaces are shared across agent activations."
    - "Candidate A self-development is attempted in this Definition (owned by Def 3)."

boundaries:
  mutation_class: red
  authority_sources:
    - "Choir Doctrine (Orchestration as code, Single authority, Restart durability)"
    - "RLM Target Architecture Rev 5 (docs/designs/rlm-target-architecture-2026-09-04.md)"
    - "Agentic Consensus Review 2026-09-04 (8-model panel unanimous adjudications)"
    - "Owner directive 2026-09-04 (compile target architecture into superseding mission)"
  must_preserve:
    - "Pre-A checkpoint 99949fe2 untouched until restore verification in Def 3"
    - "Spatial Isolation Invariant: One activation ↔ one dedicated, disposable capsule"
    - "Super runs in autoputer as supervisor, never inside a capsule"
    - "Context in the REPL for active turn; restart durability in Dolt log"
    - "Effects remain OFF until candidate A promotion gates in Def 3"
  excluded:
    - "Candidate A solitaire authoring, promotion, and falsification (owned by Def 3)"
    - "World Wire, prompt caching, or naming cutover"
  protected_surfaces:
    - "cmd/capsule-broker/main.go"
    - "internal/yaegikernel/broker.go"
    - "internal/yaegikernel/eval.go"
    - "internal/yaegikernel/choir.go"
    - "internal/actor/actor.go"
    - "internal/agentcore/tool_profiles.go"
    - "internal/agentcore/tools_capsule.go"
  completion_evidence_floor:
    - deployed_proof

measures:
  - name: direct_argv_runner_gate
    kind: gate
    baseline: "false (cmd/capsule-broker uses sh -c)"
    desired: "true"
    decision_use: "certifies elimination of shell injection and process leakage"
    cannot_prove: "cannot prove logic correctness of executed binaries"
  - name: inbox_two_phase_ack_gate
    kind: gate
    baseline: "false (no inbox or cursor tracking in Yaegi)"
    desired: "true"
    decision_use: "certifies restart durability of agent mailboxes"
    cannot_prove: "cannot prove agent comprehension of messages"
  - name: adaptive_coalescing_gate
    kind: gate
    baseline: "false (unbounded join or immediate wake)"
    desired: "true"
    decision_use: "certifies absence of barrier deadlocks during fan-in"
    cannot_prove: "cannot prove sub-task solution quality"
  - name: json_tools_in_cosuper_prompt
    kind: gate
    baseline: "10 capsule tools in CoSuper profile"
    desired: "0 under RLM profile"
    decision_use: "certifies cutover to pure Go orchestration"
    cannot_prove: "cannot prove model reasoning capability"

now:
  status: working
  slice: "landing: CI green on Go lanes; staging deploy-watch; G1 owner decisions pending"
  question: none
  reconciliation:
    observed_at: "2026-09-05T00:00:00Z"
    source_ref: "main@01d4c72e (cutover 624e50ba + e2e fix 01d4c72e)"
    deploy_identity: "staging https://choir.news still x-choir-build-commit 8c410a0d94bc7afa4383f5942b83611540a27824; computer-03335285269bdba4f94377e56879f9e6 active realization_epoch 879; effects OFF"
    authority_identities:
      - "docs/designs/rlm-target-architecture-2026-09-04.md"
      - "docs/reports/choir-status-and-next-steps-2026-09-04.md"
      - ".agentic-consensus/agentic-consensus-20260904-155821/manifest.tsv"
      - "docs/evidence/rlm-live-proof-staging-gaps-2026-09-04.md (G1 owner-gate)"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "git status --short; clean"
    status: reconciled
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Supersede choir-rlm-session-interpreter-cutover-2026-09-02 with choir-rlm-target-architecture-cutover-2026-09-04, incorporating full 8-model consensus adjudications and 6-step migration sequence."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/designs/rlm-target-architecture-2026-09-04.md"
    owner_ratification_ref: "Owner directive 2026-09-04 ('compile this into a mission... to supercede')"
    recorded_at: "2026-09-04T16:20:00Z"
    consequence: "Proceed to Step 1 implementation under this Definition."
  evidence_refs:
    - "CI 33930249230 on 01d4c72e: Go Vet+Build success; all race shards success (incl. capsule-broker direct-argv gates + session e2e with tray contract); scale success; Heresy Detector success"
    - "CI 33930249230 failures isolated: Docs Truth Check (missing report artifacts; also red on docs-only 33925843489, pre-existing plumbing) + aggregate gate cascade"
    - "CI actorruntime SQLite recovery failures on first pass passed on re-run without code change (load flake in shard 5)"
    - "internal/capsule/actuator.go (Step 1 guest contract: choir.actuator boot param wins, env fallback, fail-closed tools)"
    - "cmd/capsule-broker direct-argv exec with allowlist + 500ms group reap (Step 3)"
    - "internal/yaegikernel/transport.go framed UDS + socketpair worker migration (Step 2)"
    - "internal/yaegikernel/intent.go tray/Inbox/hooks + internal/agentcore/rlm_reduce.go Dolt persist, run-memory cursor, two-phase ack (Step 4)"
    - "internal/actor/coalesce.go bounded wakes + spawnRoleAllowed + scoped fan-in (Step 5)"
    - "internal/runtimeprompts/overlays/rlm_co_super_runtime.yaml + sealed registry (Step 6)"
  blocker_or_risk: "Sealed RLM staging proof needs two owner-gated items: (1) Node B deploy of 01d4c72e (still serving 8c410a0d); (2) G1 producer channel (nix choir.actuator branch, VMConfig.Actuator, owner refresh transport) per docs/evidence/rlm-live-proof-staging-gaps-2026-09-04.md repair step 2-3, which requires owner decisions on G2 scope and persistence scope before code."
  next_action: "Watch Node B for 01d4c72e; run tools-mode control acceptance on deploy; escalate G1 owner decisions (G2 scope, persistence scope, proof artifact fate, abort authority) to unblock the sealed RLM proof."
receipts:
  - id: rlm-target-architecture-consensus-and-definition-2026-09-04
    boundary: define
    commit_or_artifact: "main@de93d6aa"
    proof_refs:
      - "docs/designs/rlm-target-architecture-2026-09-04.md"
      - "docs/reports/choir-status-and-next-steps-2026-09-04.md"
      - ".agentic-consensus/agentic-consensus-20260904-155821/manifest.tsv"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted as Definition governing complete RLM target architecture cutover; supersedes choir-rlm-session-interpreter-cutover-2026-09-02"
    problem_ref: not_applicable
    authorization_ref: "Owner directive 2026-09-04; Agentic consensus 8-model panel unanimous adjudications"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
---

# Choir RLM Target Architecture Cutover

This Definition establishes the complete, production-grade RLM Target Architecture
as formally specified in `docs/designs/rlm-target-architecture-2026-09-04.md` (Rev 5)
and reviewed across the 8-model agentic consensus panel.

It supersedes `docs/definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md`,
subsuming session worker persistence into the comprehensive architecture encompassing
direct-argv command execution, in-cell intent staging, post-cell reduction,
`choir.Inbox()` cell-start snapshots with two-phase cursor commitment, bounded adaptive
coalescing, role-bounded fan-out/fan-in, and pure Go orchestration.

## Phased Implementation Sequence

1. **Step 1: Control Plane Boot Channel**
   Wire machine boot setting through `choir.actuator` into guest microVM parameters.
   Guarantee mechanical fallback to `actuator=tools`.

2. **Step 2: Multiplexed Transport Pipe**
   Establish the dedicated Unix domain socket and frame protocol between worker and broker.

3. **Step 3: Canonical Command Runner & Allowlist**
   Port direct-argv execution semantics natively into `cmd/capsule-broker` with an
   authoritative environment allowlist (`PATH`, `HOME`, `TMPDIR`, `LANG=C.UTF-8`) and
   guaranteed process-group SIGKILL reaping <500ms.

4. **Step 4: Intent Tray, Reduction Engine, and `Inbox()` Snapshot**
   Deploy in-cell in-memory intent tray in Yaegi and the reduction engine in `autoputer`
   (Dolt log persistence, Go-channel mailbox delivery, two-phase cursor commit,
   and cell-start snapshot injection).

5. **Step 5: Bounded Adaptive Coalescing & Role-Bounded Fan-Out/Fan-In**
   Implement debounced quiescence wakes (500ms window, error tombstones) and enforce
   role-bounded `choir.Spawn()` with durable return targets.

6. **Step 6: Tool Surface Cutover & Live Staging Verification**
   Remove ambient JSON tools from CoSuper prompt schema; run the live sealed proof
   on staging `computer-03335285269bdba4f94377e56879f9e6`.
