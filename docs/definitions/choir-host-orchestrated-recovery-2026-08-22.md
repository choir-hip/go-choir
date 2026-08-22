---
definition_version: 2
definition_id: choir-host-orchestrated-recovery-2026-08-22
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-22T21:00:00Z"
  source:
    canonical_ref: "main@f54eb735"
    deploy_identity: "staging https://choir.news proxy f54eb7351dca guest f54eb7351dca computer-03335285269bdba4f94377e56879f9e6 stopped epoch 361 `COMPUTER BOOT IS STILL PENDING`"
  worktree_inventory:
    status: clean
    evidence_ref: "git status 2026-08-22 clean after f54eb735 landing; problem documented in docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its design (docs/designs/host-orchestrated-recovery-2026-08-22.md), and the vmctl/proxy/corpusd recovery surfaces."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: none
  observed_artifact:
    - claim: "Tape restore is guest-dependent and cannot recover a down guest. Rematerialize/restore require live Runtime and X-Authenticated-Computer after an active ComputerURL; proxy 502s otherwise. BIOS wake_current_computer loops Bootstrap probe 1..3 across subnets with dial tcp 10.200.x.2:8085 connection refused."
      evidence_ref: "docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md; internal/agentcore/rematerialize.go:210; internal/agentcore/api_self_development.go:105; internal/proxy/computer_lifecycle.go:311; internal/proxy/api_key_computer_authority.go:182; vmmanager/manager.go:696; nix/autoputer-vm.nix:678"
    - claim: "Scheduling Definition choir-scheduling-and-candidate-proof-2026-08-21 remains working with effects OFF; its E2 acceptance-fenced restore to 99949fe2 is still the proof leg and must not be smuggled. This Definition provides only operational recover_current."
      evidence_ref: "docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md:finish.acceptance; docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md"
  unknowns:
    - Whether any non-privacy-key secret in /mnt/persistent is also required on fresh data.img before private replay succeeds beyond the single-key inventory assumed here.
    - Whether corpusd can atomically enforce the per-ComputerID recovery lease + fencing token without a new transaction type or if a simpler head/route CAS retry loop suffices for the first slice.

finish:
  deliver: "A host-orchestrated, corpusd-driven recover_current path puts a down computer back to its current canonical head without host ext4 surgery or new event authority — quarantining data.img as a file, preserving the privacy key via a trusted-guest attachment, booting fresh, and publishing the route only after replay equivalence and ComputerVersion/frontend verification. BIOS no longer loops; staging 0333528 is active."
  artifact: "Deployed staging trajectory proving: owner-authorized recover_current from stopped state, whole-file quarantine as evidence, trusted-attachment single-key copy, fresh boot with canonical replay to final head, ReplayCompleteness + effective ComputerVersion/frontend serving-join verification, route CAS bound to fencing token, idempotent and crash-safe journal, and honest recovery.status."
  acceptance:
    - action: "Operational recovery from stopped state via owner product path (no SSH, no X-Internal-Caller alone) — BIOS/owner cookie wake after :8085 refusal does one recover_current, quarantines data.img as a file, uses trusted-guest unit to copy only privacy-key from read-only quarantine, boots fresh, replays canonical chain to final head, verifies live/replay equivalence + effective ComputerVersion + frontend serving_join before route publish, retains quarantine as evidence, surfaces recovery.status=rematerializing then active."
      proves: "A down guest is recoverable from the tape to current head without host filesystem parsing or arbitrary rewind."
      evidence_class: deployed proof on staging 0333528 with divergence (stopped) → recovery → active
    - action: "Verify recover_current cannot rewind: requests bearing checkpoint_digest, authorization_ref, mode, or trailing checkpoint fields are rejected 400/409 before any host state change; 99949fe2 and arbitrary heads are refused; capability minted for this operation is read-events(computer_id) only and cannot append EventRestoreRequested."
      proves: "Pre-E2 structural fence holds; consensus bypass is impossible, not policy-forbidden."
      evidence_class: refusal tests + deployed refused attempts during proof
    - action: "Verify single-appender and crash safety: corpusd recovery lease allowlists only boot lifecycle appends from the token-bearing realization; head movement during staging is detected and re-verified before route CAS; kill vmctl mid-journal and resume yields same quarantine/staging identity and no cross-tenant overwrite; fencing token (ComputerID + recoveryGeneration + canonicalHead + routeGen) rides every mutation."
      proves: "Recovery is restart-safe and does not create a second semantic writer."
      evidence_class: fault-injection tests + deployed kill/restart proof
    - action: "Verify multitenant isolation: vmctl derives owner/VMID/route/paths from registry only, rejects symlinks and non-regular images, enforces containment, and ignores caller-supplied owner_id; capability binds audience+op+ComputerID+owner+VMID+routeGen+expectedHead+recoveryGen+nonce+expiry and is single-use; cross-tenant attempt is refused."
      proves: "A tenant cannot recover another computer."
      evidence_class: isolation tests + deployed cross-tenant refused attempt
  rollback: "Revert mission commits; a failed recover_current keeps quarantine as evidence and either reattaches prior accepted realization's route or leaves route safely unavailable with honest recovery.status; never rewind corpusd canonical events."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, staging_build_identity, recovery_from_stopped, rewind_refusal, lease_and_crash_safety, multitenant_isolation]
  not_done_when:
    - recover_current accepts a checkpoint digest or authorization_ref
    - recovery is reachable via X-Internal-Caller alone without owner+route verification
    - host parses or writes guest ext4 directly (outside the trusted-guest attachment)
    - a fresh disk is expected to boot without privacy-key continuity handling
    - route is published before ReplayCompleteness + effective ComputerVersion/frontend verification against final head
    - 99949fe2 is exposed as a recoverable target before the E2 Definition
    - quarantine is deleted or pruned while recovery is unfinished, destroying rollback evidence

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-22 (host-orchestrated, corpusd-driven, no data.img surgery), docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md, docs/designs/host-orchestrated-recovery-2026-08-22.md, docs/choir-doctrine.md, docs/computer-ontology.md Ledger Split, docs/standing-questions.md, AGENTS.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; host reconstruct is dry-run replay or boot replay, never a second writer.
    - Corpusd canonical events are never rewound; recover_current is a forward recovery to current head, not a restore event.
    - Checkpoint 99949fe2 and authorized_checkpoint remain untouched until the scheduling Definition reaches E2.
    - Effects remain OFF; no candidate, promotion, or World Wire work in this Definition.
  not_goals:
    - authorized_checkpoint / 99949fe2 historical restore (quorum, EventRestoreRequested, RestoreFromRequest engine) — deferred to E2 Definition
    - internal/computerrestore checkpoint engine extraction (deferred)
    - Cloud Hypervisor / parallel assignment / new initramfs / platform-computer recovery actor

invariants:
  - id: single-appender
    invariant: "One guest ComputerEventAppender appends semantic events via corpusd HeadCAS; host recovery never appends semantic events."
    evidence_ref: "internal/computerevent/appender.go:66; docs/computer-ontology.md Ledger Split"
  - id: no-host-ext4
    invariant: "Host never parses or writes guest ext4; the only copy from quarantine is the trusted-guest unit's single privacy-key read."
    evidence_ref: "docs/designs/host-orchestrated-recovery-2026-08-22.md Secrets continuity"
  - id: structural-no-rewind
    invariant: "recover_current request has no checkpoint/authorization/mode fields; they are rejected before any state change."
    evidence_ref: "docs/designs/host-orchestrated-recovery-2026-08-22.md vmctl cold path"

conjectures:
  - id: file-quarantine-suffices
    conjecture: "Whole-file data.img quarantine + single-key attachment suffices for guest-down recovery; no broader persist/host inventory is needed before private replay succeeds."
    falsifier: "Fresh boot after key copy still fails ReplayCompleteness with a missing updater/signer/persist dependency that is not a privacy-key."
  - id: lease-allowlist-suffices
    conjecture: "A fencing token that allowlists only the new realization's boot lifecycle appends is sufficient to reconcile captured head vs final head without deadlocking :8085."
    falsifier: "Boot lifecycle appends are not distinguishable from other writers at corpusd, or a concurrent owner write races and is incorrectly allowed."

phases:
  - name: Recovery Orchestration
    items:
      - "Implement vmctl POST /internal/vmctl/computers/{id}/cold-recover (recover_current only) with per-ComputerID fencing token, corpusd recovery lease allowlisting boot appends, durable journal (fenced→stopped→key_copied→staging→verified→swapped→booted→route_published→done), whole-file quarantine, trusted-guest single-key attachment boundary, sparse staging via vmmanager, and route CAS bound to token. The concrete trusted recovery unit remains a blocking sub-item."
      - "Implement proxy/BIOS integration: owner-authorized fallback for inactive computer (recover_current only, no checkpoint passthrough) and Desktop.svelte one-shot cold-recover after :8085 refusal with recovery.status."
      - "Tests: rewind-refusal, multitenant isolation, lease/head-movement/re-verify, crash-resume, rollback-on-verification-failure."

now:
  status: working
  slice: "Phase 1 implementation landed locally; deployed staging proof remains outstanding."
  question: "Will the product-wired trusted-guest attachment plus final-head verifier make recover_current boot-fresh without host ext4 surgery?"
  reconciliation:
    observed_at: "2026-08-22T21:00:00Z"
    source_ref: "working tree after local recovery implementation; staging remains stopped epoch 361"
    deploy_identity: "staging proxy f54eb735 guest f54eb735 computer 0333528 stopped"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, docs/standing-questions.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "working tree owns vmctl/proxy/vmmanager/platform/frontend recovery changes; unrelated WIP preserved"
    status: implementation_in_progress
  candidate:
    id: none
    state: none
  decision:
    selected: "Host-orchestrated recover_current now; authorized_checkpoint (99949fe2) deferred to E2 Definition. No ext4 host parse; privacy-key via trusted-guest attachment."
    kind: architecture
    status: settled
    source: owner direction 2026-08-22 + agentic consensus (Claude included, approve with repairs)
    evidence_ref: ".agentic-consensus/host-orchestrated-recovery-plan-20260822/codex.out; .agentic-consensus/host-orchestrated-recovery-plan-20260822/omp-gpt56-sol.out"
    recorded_at: "2026-08-22T21:00:00Z"
    consequence: "First slice ships only recover_current, structurally fenced from historical rewind; unblocks staging without enabling E2 early."
  evidence_refs:
    - "docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md"
    - "docs/designs/host-orchestrated-recovery-2026-08-22.md"
    - ".agentic-consensus/restore-when-guest-down-20260822/manifest.tsv"
    - ".agentic-consensus/host-orchestrated-recovery-plan-20260822/manifest.tsv"
    - "docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md"
  blocker_or_risk: "Local unit coverage passes for strict protocol, quarantine/staging, proxy owner isolation, lease fencing, and frontend build. New red evidence shows no concrete trusted-guest privacy-key attachment authority exists yet (`docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md`); vmctl production wiring still needs that authority plus corpusd head-reader and post-boot replay/ComputerVersion/frontend verifier. No recovery endpoint is claimed deployed until those authorities are configured."
  next_action: "Design and implement the reviewed trusted recovery-unit contract, wire it in cmd/vmctl, add fault/isolation coverage, then commit/push, pass CI, deploy, and run the owner product-path recovery proof on 0333528."
---
