---
title: "DRAFT — Choir-in-Choir Computer Control"
definition_version: 2
draft: true
executable: false

start:
  captured_at: 2026-07-18T18:43:30Z
  source:
    canonical_ref: refs/heads/main@6738abdbe30b6b19a6b572e42801f6c810102304
    origin_ref: refs/remotes/origin/main@6738abdbe30b6b19a6b572e42801f6c810102304
    relation: canonical_ref_equals_origin_ref
    deploy_identity: "choir.news /health reported proxy commit 2bc1799f72ce437b35d4606a23d14e62b7239ac5 at authoring; this draft makes no deployed claim"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status --short --branch observed clean main equal origin/main at authoring"
    preservation_rule: "This non-executable draft may be created and registered without changing the active /goal entrypoint. Preserve unrelated work and all accepted construction/rollback evidence."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: goal_owned
      paths_or_digest: "this draft, the then-active self-development Definition, and the three Definition registries"
      recovery: "Historical authoring receipt only; current registry authority is owned by the convergence Definition."
  candidates:
    - id: none
      ref: none
      base: 6738abdbe30b6b19a6b572e42801f6c810102304
      scope: []
      disposition: paused
      evidence_ref: none
  observed_artifact:
    - claim: "The supported choir CLI already exposes owner-scoped computer status/start/stop, run start/status/list/cancel, trajectory reads/cancel, and delegated API-key operations through public product APIs."
      evidence_ref: cmd/choir/main.go
    - claim: "Cosuper agent loops need not live inside capsules. The dormant capsule role/capability contracts intend to scope exec and filesystem verbs to a capsule identity, but the current Executor lacks kernel isolation, broker-routed exec/file tools return not_implemented, and no production caller installs them."
      evidence_ref: "internal/capsule/roles.go; internal/capsule/executor.go:46-90; internal/agentcore/tools_capsule.go:385-516"
    - claim: "Current direct super/cosuper coding and worker-VM delegation are live transitional execution paths. Production kernel-isolated capsule execution must be completed by the predecessor; candidate-as-VM remains prohibited by H031."
      evidence_ref: "internal/agentprofile/agentprofile.go; internal/agentcore/tool_profiles.go; docs/computer-ontology.md:70-84; docs/choir-doctrine.md:H031"
  problem:
    classification: substrate
    statement: "After one Choir computer can develop itself through capsule-scoped effects and a complete trajectory/event history, no supported least-privilege path lets one computer's cosuper use the choir CLI to operate another Choir computer. Cross-computer work would still escape through owner-held keys, outer Codex, SSH, raw vmctl, or host operations."
    existing_fix_connection: "Extend the existing public choir CLI/API and delegated API-key model with target-computer-bound control grants and durable remote-operation receipts. Reuse the target computer's own super, cosupers, capsules, event authority, and lifecycle APIs; do not create candidate VMs, remote shells, or a second orchestration substrate."
  unknowns:
    - "The narrowest durable ComputerControlGrant representation that binds grantor owner, controlling computer, cosuper/trajectory, target computer, allowed CLI verbs, expiry, revocation, and spend/operation bounds without exposing the bearer secret to Trace or capsule artifacts."
    - "Whether the first product path can address another computer solely through a target-bound delegated key on choir.news or requires an explicit CLI --computer selector backed by owner-visible computer identity."
    - "The exact recursion and cycle policy when a remotely started target trajectory attempts another cross-computer delegation."

finish:
  deliver: "A cosuper working for one Choir computer can use the supported choir CLI from its scoped capsule to observe and control a different Choir computer through an explicit, revocable, least-privilege grant. The target computer remains sovereign: it authenticates the grant, records the remote trajectory and effects, runs its own agents and capsules, and returns durable receipts without SSH, host access, raw vmctl, shared owner credentials, or candidate VMs."
  artifact: "A product-owned ComputerControlGrant and remote-operation protocol; target-addressed choir CLI/API operations for computer status/start/stop and run/trajectory start/status/cancel; capsule-safe credential delivery; joined controller/target trajectory and effect receipts; restart recovery, revocation, refusal, and deployed two-computer acceptance."
  acceptance:
    - action: "From computer A, start a cosuper trajectory whose bash/file capability is confined to capsule A1. Give it a short-lived grant minted by computer B's owner and bound to computer A, that cosuper/trajectory, computer B, and an explicit verb set. Run the installed choir CLI inside A1 against choir.news; never expose the bearer value in argv, Trace payloads, logs, diffs, or returned artifacts."
      proves: "The controlling agent uses the ordinary product CLI from an effect chamber with delegated authority rather than ambient owner credentials or a privileged internal client."
      evidence_class: deployed_capsule_scoped_remote_cli
    - action: "Use the grant to fetch B's status, start B if stopped, submit a harmless prompt, observe its submission, run, trajectory, and settlement receipts, cancel one bounded remote run, and stop/start B through supported CLI commands."
      proves: "Computer A can operate computer B end to end through public product paths, while B's own runtime remains the execution and state authority."
      evidence_class: deployed_two_computer_control
    - action: "Have the accepted prompt on B delegate effectful work to B's own capsule, produce a harmless verifier-known artifact/event, and return joined B-side trajectory, capsule, acceptance, and artifact refs that A can inspect through scoped CLI reads."
      proves: "Choir-in-Choir means one computer controls another computer that performs its own agent/capsule work; A does not execute mutations against B's filesystem or database."
      evidence_class: deployed_recursive_agent_capsule_control
    - action: "Restart the controller runtime, target runtime, and both computer realizations at bounded checkpoints. Recover the same grant status, remote operation handles, target trajectory identity, and terminal receipts without repeating accepted effects."
      proves: "Cross-computer control is durable and idempotent rather than an in-memory RPC chain."
      evidence_class: deployed_cross_computer_restart_recovery
    - action: "Attempt every operation with no grant, expired/revoked grant, wrong controlling computer, wrong cosuper/trajectory, wrong target, read-only scope, disallowed verb, reused terminal idempotency key, and a recursively delegated grant. Require deterministic refusal before target mutation."
      proves: "Control is target-bound, non-transitive by default, least privilege, and safe under stale or stolen context."
      evidence_class: deployed_cross_computer_authority_refusal
    - action: "Revoke the grant from B, prove no later A-side CLI operation can affect B, and fetch the complete joined audit from both computers showing intent, authorization, target receipt, result, revocation, and any refusal."
      proves: "The target computer and its owner retain final authority, and the audit survives loss of the controller capsule."
      evidence_class: deployed_grant_revocation_and_audit
  rollback: "Revoke or expire the ComputerControlGrant, cancel any still-authorized remote operations through B's public API, and retain both computers' immutable trajectory/effect receipts. Rollback never uses SSH, host mutation, raw vmctl, shared root credentials, direct database writes, or deletion of audit history."
  landing:
    required: true
    environment: staging_node_b_and_choir_news_two_disposable_computers
    required_receipts: [pushed_origin_main_commit, ci, deploy, staging_build_identity, controller_computer_identity, target_computer_identity, control_grant, capsule_scoped_cli, remote_lifecycle, remote_run_trajectory, target_capsule_effect, restart_recovery, authority_refusals, revocation, no_ssh_acceptance]
  not_done_when:
    - "Computer A merely calls a test server, raw internal endpoint, vmctl, SSH, or a host command."
    - "The cosuper receives B's owner-wide key, a grant appears in argv/Trace/artifacts, or authority can be delegated recursively without B's explicit policy."
    - "A worker or candidate VM substitutes for B's stable computer and its own runtime/capsule/event authority."
    - "Only status reads work; B never runs and settles a real target trajectory with a target-local capsule effect."
    - "Only local tests, mocked computers, CLI JSON, or dashboard prose passes."

activation_requirements:
  executable_only_after:
    - "The completed convergence Definition proves one generic durable-work lifecycle with restart reconstruction, typed updates, reducer-owned settlement/cancellation, and equivalent public UI/headless observation. A later effect mission must separately prove capsule-scoped authority before this draft can execute effectful remote control."
    - "The public CLI/API exposes the target-local lifecycle, run, trajectory, refusal, and receipt semantics required here without raw vmctl or internal routes."
    - "The ComputerControlGrant authority, secret-delivery boundary, recursion policy, first two-computer target, rollback, and evidence floor are owner-ratified."
    - "All three registries promote one reconciled successor as the sole executable /goal. This draft is never invoked directly."

boundaries:
  mutation_class: red
  protected_surfaces: [cross_computer_authority, API_keys, credentials, computer_identity, agent_trajectories, capsule_capabilities, product_API, choir_CLI, vmctl_lifecycle_projection, run_acceptance, deployment_routing]
  conjecture_delta:
    discovered: "A self-developing computer is not yet Choir-in-Choir; another computer needs an explicit product grant and target-owned execution path."
    introduced: "A cosuper can safely control another Choir computer through capsule-scoped choir CLI commands when authority is target-bound, non-transitive, revocable, secret-safe, and fully joined to both computers' event histories."
    repaired_target: "Replace outer-operator, shared-key, SSH, raw-vmctl, and candidate-VM control with computer-to-computer product authority."
  heresy_delta:
    discovered: ["historical Choir-in-Choir paths relied on worker VMs, AppChangePackage transfer, and outer Codex stitching", "the current CLI key model is owner-scoped rather than an explicit computer-to-computer grant"]
    introduced: []
    repaired: []
  authority_sources:
    - "Owner direction recorded 2026-07-18: draft Choir-in-Choir first so the active self-development cutover aligns with cosupers controlling other Choir VMs through choir CLI"
    - AGENTS.md
    - docs/standing-questions.md
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
    - docs/definitions/choir-cli-self-development-2026-07-16.md
  must_preserve:
    - "Computer B owns its agents, capsules, event history, accepted state, lifecycle decisions, and refusal semantics; A supplies authorized intent only."
    - "Super has no bash. A cosuper may run outside a capsule, but every bash/filesystem effect is capability-scoped inside an identified capsule."
    - "Researchers remain VM-local and may use only their typed safely concurrent Dolt finding write, not shell or cross-computer mutation."
    - "Host repair, Node B mutation, raw vmctl, SSH, and candidate VMs are outside guest self-development and Choir-in-Choir."
    - "Grants are target-bound, least-privilege, time-bounded, revocable, non-transitive by default, and safe to inspect without revealing bearer material."
  excluded:
    - "Host/NixOS/kernel repair, deployment control, provider-secret administration, or one guest modifying the host."
    - "Fleet-wide autonomous control, arbitrary cross-cloud federation, recursive grant chains, shared owner credentials, or remote shells."
    - "Executing this draft before predecessor completion, reconciliation, owner ratification, and registry promotion."
  completion_evidence_floor: [deployed_capsule_scoped_remote_cli, deployed_two_computer_control, deployed_recursive_agent_capsule_control, deployed_cross_computer_restart_recovery, deployed_cross_computer_authority_refusal, deployed_grant_revocation_and_audit]

measures:
  - name: remote_operation_latency
    kind: telemetry
    baseline: unknown
    desired: "record status and command acknowledgement distributions before setting an SLO"
    decision_use: "Choose polling, streaming, and operation-handle behavior after the authority path is correct."
    cannot_prove: "Authorization correctness, durability, or useful target execution."
  - name: cross_computer_join_completeness
    kind: gate
    baseline: "no product-owned two-computer join"
    desired: "every accepted/refused remote command joins controller trajectory, grant, target operation, target trajectory/effect, and result receipt"
    decision_use: "Refuse completion or promotion when any causal or authority edge is missing."
    cannot_prove: "The remote work was valuable or the target implementation is correct without product-path verification."

execution_outline:
  - id: A-grant-and-addressing
    outcome: "Define target computer identity, ComputerControlGrant, secret-safe capsule delivery, non-transitive authority, idempotency, and durable refusal/audit semantics."
  - id: B-cli-and-target-operations
    outcome: "Extend public CLI/API only as needed for target-addressed status/start/stop and run/trajectory start/status/cancel with durable operation handles."
  - id: C-recursive-product-proof
    outcome: "Computer A's cosuper controls disposable computer B; B runs its own super/cosuper/capsule trajectory and returns joined receipts."
  - id: D-land-and-close
    outcome: "Push, deploy, verify exact staging identity, replay two-computer acceptance, revoke the grant, and close registries only after independent review."

orchestration:
  order: "A then B then C then D. Read-only mapping and independent verification may run in parallel; grant mutation, target effects, deployment, and registry authority remain serialized."
  review: "Use deterministic authority/refusal/restart checks first, then the agentic-consensus skill over frozen G1 grant, G2 pre-remote-effect, and G3 deployed-closure packets."
  minority_rule: "One reproducible authority leak, secret exposure, cross-target effect, missing event join, or restart duplication overrides unsupported panel passes."

now:
  status: blocked_incomplete
  slice: "non-executable successor draft"
  question: "What exact target-bound grant and computer-addressing contract gives one computer useful control without owner-wide or transitive authority, and what separately authorized effect boundary executes it?"
  reconciliation:
    observed_at: 2026-07-24T06:04:09Z
    source_ref: refs/remotes/origin/main@4ffcae3ab24fba8bc24ce1767e4e638667a50367
    deploy_identity: "The predecessor generic durable-work kernel passed staging acceptance at commit 4ffcae3a; this successor remains unselected and non-executable."
    authority_identities: [owner_decision:2026-07-18-choir-in-choir-draft, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
    policy_resolution_ref: "Blocked hypothesis only; no implementation policy selected"
    worktree_inventory_ref: "Completed convergence Definition terminal receipt"
    status: reconciled
  candidate:
    id: choir-in-choir-computer-control-draft-2026-07-18
    state: paused
    ref: docs/definitions/choir-in-choir-computer-control-draft-2026-07-18.md
    owner: integration-authority
    base: 6738abdbe30b6b19a6b572e42801f6c810102304
    digest: none
    scope: [draft_definition, registries]
  decision:
    selected: "Draft only: future cosupers control other Choir computers through target-bound grants and the public choir CLI; target computers execute locally through their own agents and capsules."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "Owner statement in this 2026-07-18 conversation"
    owner_ratification_ref: "Owner statement in this 2026-07-18 conversation"
    recorded_at: 2026-07-18T18:43:30Z
    consequence: "Historical 2026-07-18 decision: register this as a non-executable successor and align the then-active self-development Definition toward the capsule/event/CLI substrate it required. The 2026-07-21 supersession keeps this topology as a hypothesis and authorizes no implementation."
  evidence_refs: [cmd/choir/main.go, internal/capsule/roles.go, internal/agentcore/tools_capsule.go, docs/computer-ontology.md, docs/choir-doctrine.md]
  blocker_or_risk: "The generic durable-work predecessor is accepted, but self-development effects remain OFF; the target-bound grant/addressing contract, secret-delivery boundary, recursion policy, capsule authority, and first two-computer acceptance path remain unresolved and unratified."
  next_action: "Keep this draft blocked. Reconcile it against the accepted durable-work, CLI, capsule, trajectory, and authority evidence only after separate owner ratification; then promote through all three registries before execution."

successor:
  status: none_selected
  candidate_goal: none

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-in-choir-computer-control-draft-2026-07-18.md --serve 127.0.0.1:8789 --watch"
  authority: "This Markdown/YAML draft is non-executable authority for future alignment only; its dashboard is a read-only projection."
---

# DRAFT — Choir-in-Choir Computer Control

Future Choir-in-Choir begins only after Choir has one accepted durable-work kernel and a separately authorized effect boundary. A cosuper on computer A may then use the supported `choir` CLI from a scoped capsule to send authorized intent to computer B. Computer B remains sovereign: it authenticates the grant, records the trajectory, runs its own agents and capsules, and returns durable receipts. Neither computer receives host authority, raw vmctl, SSH, a candidate VM, or the other's owner-wide credentials.
