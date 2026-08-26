---
definition_version: 2

start:
  captured_at: "2026-08-25T17:00:00Z"
  source:
    canonical_ref: "main@20701098"
    deploy_identity: "staging https://choir.news; computer-03335285269bdba4f94377e56879f9e6 active epoch 794, host held=true, guest RUNTIME_MAINTENANCE_HOLD=1"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-08-25 git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its receipts, and the three navigation registries."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  observed_artifact:
    - claim: "Staging computer 0333528... was restored to ancestor base 132,539 via manual B14 and is serving on :8085 under maintenance hold, appending past 133,319; sealed under permanent host hold."
      evidence_ref: "docs/evidence/effects-red-b14-ancestor-restore-2026-08-25.md"
    - claim: "Agentic consensus triad (12 models) proved the normal-boot hang is binary-selection cutover via choir.refresh_runtime=1 deleting /mnt/persistent/choir-updater/current and exec'ing the new store binary on unmigrated disk state."
      evidence_ref: "docs/evidence/effects-red-b14-ancestor-restore-2026-08-25.md; docs/reports/choir-system-orientation-and-station-report-2026-08-25.md"
    - claim: "Owner direction 2026-08-25 ratified the strategic sequence: clean substrate overhauls (Tracks K, F, M, Assurance) and Yaegi Actor Kernel before self-development resumption."
      evidence_ref: "docs/reports/choir-system-orientation-and-station-report-2026-08-25.md"
  unknowns:
    - "Exact provisioning time for fresh overhaul test computers on staging."

goal:
  product_outcome: "Eliminate mutable dynamic binary fallback and emergency recovery flags from the guest MicroVM closure, restore standard embedded Dolt GC for normal operations, seal computer-0333528 under permanent hold as an immutable historical evidence artifact, suspend self-development effects, and hand off execution directly to the clean Durable Substrate Overhauls (Tracks K, F, M, Assurance)."
  observed_starting_state: "nix/autoputer-vm.nix contains mutable choir-updater/current fallback and RUNTIME_DOLT_GC_DISABLED='1'; manager.go injects choir.refresh_runtime=1; computer-0333528 is live under hold; self-development candidate proof is active in registries."
  final_artifact: "Clean NixOS guest closure running strictly immutable store binary with standard embedded Dolt GC; registries aligned with Substrate Cleanup active, 0333528 sealed, self-dev suspended, and overhauls sequenced next."
  authority_boundary: "This Definition owns the NixOS guest wrapper cleanup, manager.go launch argument cleanup, registry reconciliation, and sealed artifact archiving. It does not execute self-development candidate authoring or Track K/F code implementation."
  compact_current_state: "Mission definition active; registry updates drafted; ready for Phase A code cutover."

invariants:
  - id: single-store-binary-authority
    invariant: "Guest MicroVMs execute strictly ONE binary: the compiled, immutable Nix store binary ${goChoirPackages.autoputer}/bin/autoputer. The mutable directory /mnt/persistent/choir-updater/current and all dynamic-binary fallback branches are deleted."
    evidence_ref: "docs/choir-doctrine.md C14; docs/computer-ontology.md"
  - id: standard-embedded-dolt-gc
    invariant: "RUNTIME_DOLT_GC_DISABLED = '1'; is removed from nix/autoputer-vm.nix. Normal guest MicroVMs run standard embedded Dolt GC to maintain bounded chunk stores and prevent disk fragmentation."
    evidence_ref: "internal/store/dolt_maintenance.go"
  - id: no-refresh-deletion-dance
    invariant: "The kernel parameter choir.refresh_runtime=1 and its corresponding wrapper deletion logic are deleted. VM realization replacement is the sole code-deployment actuator."
    evidence_ref: "docs/computer-ontology.md"
  - id: sealed-historical-artifact
    invariant: "Staging computer computer-03335285269bdba4f94377e56879f9e6 remains sealed under permanent host hold (held=true) and guest fence (RUNTIME_MAINTENANCE_HOLD=1) as an immutable historical evidence artifact. Linear 133k replay is never a release, test, or CI gate."
    evidence_ref: "docs/evidence/effects-red-b14-ancestor-restore-2026-08-25.md"
  - id: self-dev-suspension
    invariant: "Self-development effects definitions are formally demoted to suspended_pending_substrate until clean substrate overhauls (Tracks K, F, M, Assurance) and Yaegi Actor Kernel deploy."
    evidence_ref: "docs/reports/choir-system-orientation-and-station-report-2026-08-25.md"

phases:
  - name: Phase A — Registry & Document Authority Reconciliation
    items:
      - "Update docs/ACTIVE.md to promote cleanup definition as sole active executable /goal."
      - "Update docs/mission-graph.yaml and docs/doc-authority-manifest.yaml."
      - "Update suspended status in choir-scheduling-and-candidate-proof-2026-08-21.md and sealed status in choir-0333528-stabilize-and-hold-2026-08-24.md."
  - name: Phase B — NixOS Configuration & VM Manager Cleanup
    items:
      - "In nix/autoputer-vm.nix: simplify autoputerRuntimeExec to directly exec store binary; delete choir-updater/current fallback and refresh_runtime deletion logic."
      - "In nix/autoputer-vm.nix: remove RUNTIME_DOLT_GC_DISABLED = '1';."
      - "In internal/vmmanager/manager.go: remove choir.refresh_runtime=1 argument formatting."
  - name: Phase C — Landing Loop & Staging Verification
    items:
      - "Commit code & doc changes under mutation class red / orange."
      - "Push to origin/main, monitor CI build and NixOS guest image generation."
      - "Verify staging deployment SHA and health on https://choir.news/health."
      - "Verify guest MicroVM boots strictly the immutable store binary with standard Dolt GC active."
      - "Hand off directly to docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md."

boundaries:
  mutation_class: red
  protected_surfaces:
    - "nix/autoputer-vm.nix"
    - "internal/vmmanager/manager.go"
    - "docs/ACTIVE.md"
    - "docs/mission-graph.yaml"
    - "docs/doc-authority-manifest.yaml"
  evidence_class: staging-smoke
  rollback_path: "Git revert of mission commits to origin/main."
  heresy_delta:
    discovered:
      - "Dual-binary split-brain: mutable /mnt/persistent/choir-updater/current dynamic binary overrode deployed Nix store binaries on reboot."
      - "B11 GC disablement: RUNTIME_DOLT_GC_DISABLED=1 left active for all guests after the 133k recovery."
    repaired:
      - "Single immutable store binary authority enforced in guest wrapper."
      - "Standard embedded Dolt GC restored in guest environment."
      - "Refresh deletion dance eliminated."
    introduced: []

finish:
  deliver: "Clean, single-binary NixOS guest closure with standard Dolt GC enabled, registries aligned, 0333528 sealed, and overhauls unblocked."
  acceptance:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  completion_cutover:
    - id: promote-overhauls-entrypoint
      action: "Promote docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md to active executable /goal."
      class: registry update

now:
  status: working
  slice: "Phase A complete (registries updated); ready for Phase B code cutover."
  question: "Does the NixOS guest build cleanly and boot strictly the immutable store binary on staging?"
  reconciliation:
    observed_at: "2026-08-25T17:30:00Z"
    source_ref: "main@20701098"
    deploy_identity: "staging https://choir.news; computer-0333528 sealed under hold (epoch 794)"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/computer-ontology.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean single worktree /Users/wiz/go-choir"
    status: reconciled
  candidate:
    id: clean-store-closure
    state: ready
    ref: "nix/autoputer-vm.nix; internal/vmmanager/manager.go"
    owner: main
    base: "20701098"
    digest: pending_commit
    scope: [nix/autoputer-vm.nix, internal/vmmanager/manager.go]
  decision:
    selected: "Eliminate mutable binary fallback, re-enable standard Dolt GC, seal computer-0333528, suspend self-dev effects, and sequence Durable Substrate Overhauls -> Yaegi Actor Kernel -> Self-Development."
    kind: architecture
    status: owner-ratified
    source: owner direction 2026-08-25 + agentic consensus triad 2026-08-25
    owner_ratification_ref: "owner direction 2026-08-25"
    recorded_at: "2026-08-25T17:30:00Z"
    consequence: "Active goal is cleanup; self-dev suspended; overhauls unblocked."
  evidence_refs:
    - docs/reports/choir-system-orientation-and-station-report-2026-08-25.md
    - docs/evidence/effects-red-b14-ancestor-restore-2026-08-25.md
  blocker_or_risk: "None blocking Phase B implementation; computer-0333528 safely sealed."
  next_action: "Execute Phase B code cleanup in nix/autoputer-vm.nix and internal/vmmanager/manager.go."

receipts:
  - id: define-substrate-cleanup
    boundary: define
    commit_or_artifact: "docs/definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md"
    proof_refs: [docs/reports/choir-system-orientation-and-station-report-2026-08-25.md]
    rollback_ref: "revert docs commit"
    disposition: "Substrate cleanup Definition v2 authored and ratified by owner direction + agentic consensus."
    problem_ref: "dual-binary split brain and B11 GC disablement"
    authorization_ref: "owner direction 2026-08-25"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
---
