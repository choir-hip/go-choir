---
definition_version: 2

start:
  captured_at: "2026-09-02T21:05:00Z"
  source:
    canonical_ref: "main@a52ef06d"
    deploy_identity: "staging https://choir.news deployed_commit 42d47604; retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 831; Def 1 substrate readiness in progress; effects OFF"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-02 read-only git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns capsule broker, yaegikernel session interpreter, and CoSuper prompt tool schemas."
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
    - claim: "internal/yaegikernel is code-complete and containment-tested under choir-private-go-actor-kernel-2026-08-12.md (digest sha256:yaegi-kernel-and-capsule-broker-complete-53f80af4)."
      evidence_ref: "internal/yaegikernel/eval.go; internal/yaegikernel/containment_test.go"
    - claim: "cmd/capsule-broker/main.go is the live in-capsule actuator, executing go_eval via yaegikernel.ExecuteWorkerStdin()."
      evidence_ref: "cmd/capsule-broker/main.go:99,440"
    - claim: "eval.go currently invokes interp.New per eval call; variables are not retained across sequential cells in an activation."
      evidence_ref: "internal/yaegikernel/eval.go:105"
    - claim: "External spikes (yaegi-binding-spike and oh-my-pi-yaegi-candidate) proved persistent Yaegi orchestration and host-mediated fan-out viable."
      evidence_ref: "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
  unknowns:
    - "Exact latency overhead of session worker rebinds following process-group SIGKILL on timeout."

finish:
  deliver: "CoSuper executes exclusively through model-written Go inside a persistent session interpreter with prebound host modules, achieving complete ambient tool parity with legacy JSON tools removed from the model-facing schema."
  artifact: "Deployed RLM execution surface in cmd/capsule-broker and internal/yaegikernel: persistent Session worker per activation retaining variables across sequential cells, prebound choir host modules, sidecar process-group SIGKILL containment reaped under 500ms, and live sealed proof on retained staging computer computer-03335285269bdba4f94377e56879f9e6."
  acceptance:
    - action: "Implement runtime route flag (e.g. actuator=tools vs actuator=rlm) and test fallback."
      proves: "Mechanical rollback path is verified before any ambient tools are removed from prompt schema."
      evidence_class: local_test
    - action: "Run multi-eval integration test asserting variable persistence across sequential cells (cell 1 defines variable, cell 2 computes on it without re-import)."
      proves: "Session interpreter persists state across eval calls within an activation without state loss or redeclaration errors."
      evidence_class: local_test
    - action: "Execute P0 sidecar containment test: trigger infinite loop / blocked channel and assert process group is SIGKILLed and reaped within 500ms."
      proves: "Runaway interpreter loops cannot outlive the activation deadline or leak native host processes."
      evidence_class: local_test
    - action: "Execute ambient tool parity fixture corpus (five sealed cells matching observable contract of capsule_exec, capsule_read_file, capsule_write_file, capsule_list_dir, and assign)."
      proves: "Prebound choir modules provide identical path jailing, refusal receipts, and execution bounds as JSON tools."
      evidence_class: local_test
    - action: "Execute live sealed CoSuper proof on computer-03335285269bdba4f94377e56879f9e6 with JSON tool definitions removed from the model prompt schema."
      proves: "CoSuper successfully executes a multi-step read->compute->write->assign arc exclusively via model-written Go on staging."
      evidence_class: deployed_proof
  rollback: "Runtime flag flip (actuator=tools) and Git revert; pre-A checkpoint 99949fe2 remains the immutable fence."
  landing:
    required: true
    environment: staging
    required_receipts: [source_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "interp.New is called on every eval within an activation."
    - "Model-written code encounters 'redeclared in this block' errors when referencing standard libraries."
    - "CoSuper prompt schema still exposes ambient JSON tool definitions under the RLM profile."
    - "Rollback flag actuator=tools is missing or unverified."
    - "Candidate A self-development is attempted in this Definition (owned by Def 3)."

boundaries:
  mutation_class: orange
  authority_sources:
    - "Choir Doctrine (RLM orchestration-as-code)"
    - "Owner direction 2026-09-02 (Option B sequence, cutover first, no parallel coding)"
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
  must_preserve:
    - "Fail-closed import allowlist and banned package policy (no unsafe, os, syscall, net in guest interpreter)"
    - "Process-group SIGKILL boundary on timeout / runaway loops"
    - "Single effect gate: all mutations route through broker"
    - "Pre-A checkpoint 99949fe2 untouched"
  excluded:
    - "Substrate scheduling and vmctl repairs (owned by Def 1)"
    - "Candidate A solitaire authoring and promotion (owned by Def 3)"
    - "World Wire, prompt caching, or naming cutover"
  protected_surfaces:
    - "cmd/capsule-broker/main.go"
    - "internal/yaegikernel/eval.go"
    - "internal/yaegikernel/sidecar.go"
    - "internal/agentcore/tool_profiles.go"
    - "internal/agentcore/tools_capsule.go"
  completion_evidence_floor:
    - deployed_proof

measures:
  - name: session_persistence_pass
    kind: gate
    baseline: "false (eval.go creates fresh interp.New per eval)"
    desired: "true"
    decision_use: "certifies session interpreter target"
    cannot_prove: "cannot prove candidate A domain correctness"
  - name: json_tools_in_prompt
    kind: gate
    baseline: "10 capsule tools in CoSuper profile"
    desired: "0 under RLM profile"
    decision_use: "certifies cutover to pure Go orchestration"
    cannot_prove: "cannot prove model reasoning quality"

now:
  status: working
  slice: "RLM Session Interpreter Cutover active execution"
  question: none
  reconciliation:
    observed_at: "2026-09-04T04:00:00Z"
    source_ref: "main@7495ffa1"
    deploy_identity: "staging https://choir.news proxy deployed_commit fa0fd202; guest binary fa0fd202 epoch 876 on computer-03335285269bdba4f94377e56879f9e6 servable; pre-A fence 99949fe2 untouched; effects OFF"
    authority_identities:
      - "docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md"
      - "docs/evidence/effects-red-substrate-scheduling-review-2026-09-03.md"
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "git status --short; review repairs committed"
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
    selected: "Begin RLM cutover execution: session persistence and prebound choir modules in cmd/capsule-broker and internal/yaegikernel; code work may proceed immediately."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md"
    owner_ratification_ref: "Definition 1 terminal receipt 2026-09-03; review receipt effects-red-substrate-scheduling-review-2026-09-03"
    recorded_at: "2026-09-03T15:00:00Z"
    consequence: "Def 2 code work proceeds; staging is healthy on the final commit so no refresh gates the live sealed proof."
  evidence_refs:
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    - "docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md"
    - "docs/evidence/effects-red-substrate-scheduling-review-2026-09-03.md"
    - "docs/evidence/effects-repair-verification-2026-09-03.md"
    - "cmd/capsule-broker/main.go"
    - "internal/yaegikernel/eval.go"
  blocker_or_risk: "Definition 1 resume-hang residual closed (dispatch watchdog) and the 2026-09-03 outage chain repaired and verified; code work (session worker, prebound modules, containment tests) is unblocked and live proof is ungated."
  next_action: "Implement the runtime route flag (actuator=tools vs actuator=rlm) and persistent Session worker per the acceptance criteria."

receipts:
  - id: rlm-cutover-baseline-2026-09-02
    boundary: define
    commit_or_artifact: "main@a52ef06d"
    proof_refs:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-private-go-actor-kernel-2026-08-12.md"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted as Definition 2 governing RLM session interpreter and ambient tool parity; successor to completed kernel definition 53f80af4; paused pending Def 1"
    problem_ref: not_applicable
    authorization_ref: "owner direction 2026-09-02; agentic consensus unanimous Option B"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
---

# Definition 2: RLM Session Interpreter & Ambient Tool Parity

This Definition executes Phase 2 of the 3-Definition autonomous engineering sequence.

## Core Deliverables
1. **Mechanical Rollback Flag**: Implement `actuator=tools` vs `actuator=rlm` route flag so that rollback is an observable runtime switch before prompt schemas change.
2. **Persistent Session Interpreter**: Evolve `cmd/capsule-broker` and `internal/yaegikernel` to run a persistent `Session` worker per activation with prebound imports.
3. **Prebound Module Surface**: Provide the `choir` package exports (`ReadFile`, `WriteFile`, `ListDir`, `Exec`, `Assign`, `Message`, `context`, `outcome`) wrapping live capsule-broker operations.
4. **P0 Containment & Parity**: Verify sidecar process group is SIGKILLed within 500ms on cancel, and verify ambient tool parity via golden transcripts.
5. **Live Sealed Proof**: Prove the end-to-end RLM execution loop on computer-03335285269bdba4f94377e56879f9e6 with effects OFF.

Candidate A self-development authoring is strictly deferred to Definition 3.
