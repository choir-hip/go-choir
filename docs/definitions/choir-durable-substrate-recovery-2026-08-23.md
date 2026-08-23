---
definition_version: 2
definition_id: choir-durable-substrate-recovery-2026-08-23
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-23T02:00:00Z"
  source:
    canonical_ref: "main@3f9173be"
    deploy_identity: "staging https://choir.news; retained computer computer-03335285269bdba4f94377e56879f9e6 (VM candidate-fleet-e15cb89f25d963c220319b7b) epoch 361 state stopped, stopped_by recover_current; `COMPUTER BOOT IS STILL PENDING`"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-23: untracked docs/designs/choir-durable-substrate-2026-08-23.md, docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md, docs/evidence/effects-red-recovery-projections-present-2026-08-23.md"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns recovery of 0333528 and its first ProjectionBase artifact. The overarching substrate design lives in docs/designs/choir-durable-substrate-2026-08-23.md."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: commit the docs at the first Define boundary
  candidates:
    - id: none
  observed_artifact:
    - claim: "Retained computer 0333528 is stopped at epoch 361. The initial break on 2026-08-21 was a capsule memory exhaustion (2 GiB used / 3 GiB limit) and rapid assignment supersession loop, followed by boot timeouts because autoputer tries to replay 132,436 events sequentially during boot before opening HTTP port :8085."
      evidence_ref: "docs/evidence/effects-red-capsule-memory-budget-exhaustion-2026-08-21.md; docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md; docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md"
    - claim: "The canonical event chain is intact at head sequence 132,436. All 132,490 projection batch bodies are present on disk (~756 MiB in /var/lib/go-choir/platform-artifacts/sha256/computer-event-payload/)."
      evidence_ref: "docs/evidence/effects-red-recovery-projections-present-2026-08-23.md"
    - claim: "The privacy key is a 32-byte XChaCha20 DEK stored in a mode-0400 file in the quarantined disk image; offline rebuild can read this key via trusted-guest copy."
      evidence_ref: "internal/computerevent/privacy.go; docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md"
    - claim: "Predecessor definition choir-host-orchestrated-recovery-2026-08-22.md is superseded by this Definition."
      evidence_ref: "docs/definitions/choir-host-orchestrated-recovery-2026-08-22.md"

finish:
  deliver: "The retained stopped computer computer-03335285269bdba4f94377e56879f9e6 is recovered to its canonical head 132,436 via an isolated, resumable offline full-tape rebuild that publishes the first ProjectionBase as a content-addressed blob artifact bound to the canonical head — recovered with no rewind and no second semantic writer, returning the computer to a quiesced active state on staging."
  artifact: "A deployed staging trajectory showing 0333528 stopped-epoch-361 → offline rebuild (batched tx 1,000-5,000 events, bounded RSS <2 GiB, one Dolt commit at head 132,436, decrypting batch bodies with the existing guest DEK via trusted-guest copy) → first ProjectionBase blob (in /var/lib/go-choir/platform-artifacts/sha256/projection-base/ with atomic tmp+fsync+rename) bound to computer_id, sequence 132,436, canonical_head, reducer/projector/schema versions, VM-local content witness → base-verify → guest gateway hydration into /mnt/persistent/state → delta=0 boot → final head+witness match → route CAS under fencing token → computer active in quiesced state, with no-rewind refusal receipts and quarantine preserved."
  acceptance:
    - action: "Recover 0333528 to head 132,436 via the product path: offline rebuild publishes the first ProjectionBase with atomic fsync; recovery verifies base ancestor of the final head → hydrates into guest workspace via gateway → boots with delta=0 replay → verifies final head/witness → publishes route under fencing token → computer active in quiesced state. Show ReplayCompleteness equivalent and effective ComputerVersion/frontend serving-join before route CAS."
      proves: "The down computer is recovered to current canonical head without host ext4 surgery, memory exhaustion, or event rewinds."
      evidence_class: deployed proof on staging 0333528, stopped → recovered → active
    - action: "Verify no rewind and structural single-appender: requests bearing checkpoint_digest/authorization_ref/mode rejected 400/409; capability is read-events only; host recovery path never constructs a semantic CASRequest; canonical event tape has zero host-appended semantic events."
      proves: "Recovery cannot rewind history and introduces no second semantic writer."
      evidence_class: refusal tests + chain inspection
    - action: "Verify ProjectionBase is a blob artifact: resolvable under platform-artifacts/sha256/projection-base/, content-addressed, written with atomic fsync, and verified before route CAS."
      proves: "The snapshot is a durable, reusable base artifact, not a temporary hack."
      evidence_class: blob-store inspection
    - action: "Verify quarantine preservation & structural pruning disable: pruneCompletedQuarantines is disabled during active recovery; data.img quarantine is preserved throughout recovery and not pruned."
      proves: "Forensic and rollback evidence survives the recovery run."
      evidence_class: quarantine inspection

rollback: "Revert mission commits; a failed recovery preserves quarantine and leaves route safely unavailable. Canonical events are never rewound."
landing:
  required: true
  environment: staging
  required_receipts: [pushed_commit, ci, deploy, staging_build_identity, recover_0333528_to_head, no_rewind, projection_base_blob, quarantine_preserved]
not_done_when:
  - recover_current accepts a checkpoint digest/authorization_ref, or any historical rewind is reachable
  - host parses or writes guest ext4 directly (trusted-guest read of privacy key via debugfs is the sole documented exception)
  - route is published before ReplayCompleteness + effective ComputerVersion/frontend verification
  - 99949fe2 is exposed as a recoverable target before the scheduling Definition reaches E2
  - a ProjectionBase is published as a tape event rather than a blob artifact
  - quarantine is deleted or pruned while recovery is unfinished
  - computer 0333528 is SQL-emptied, replaced, or its canonical chain rewound
  - the computer boots into an active supersession loop rather than a quiesced state

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-23, docs/designs/choir-durable-substrate-2026-08-23.md, docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md, docs/evidence/effects-red-recovery-projections-present-2026-08-23.md, docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; host reconstruct is an offline read-only replay that writes a blob artifact, never an event to the canonical tape.
    - Corpusd canonical events are never rewound.
    - Checkpoint 99949fe2 remains untouched until the scheduling Definition reaches E2.
    - Effects remain OFF.
    - Quarantine is never deleted or pruned during active recovery; maxRetained=3 pruning is explicitly bypassed during recovery.
  not_goals:
    - Substrate overhauls (Track K keys, Track F file commit protocol, Track M mail, Assurance/Scale) are owned by choir-durable-substrate-overhauls-2026-08-23.md.
    - Autonomy / candidate proof is owned by choir-scheduling-and-candidate-proof-2026-08-21.md.
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, checkpoint/route projection, platform-artifacts blob store, privacy-key file, corpusd HeadCAS, recover_current, vmctl lifecycle]
  completion_evidence_floor: [deployed proof, no-rewind refusal, independent review of the frozen ProjectionBase artifact]

measures:
  - name: recover_0333528_to_head
    kind: gate
    baseline: "stopped epoch 361"
    desired: "0333528 active at head 132,436 via ProjectionBase + route CAS in quiesced state"
    decision_use: gates the recovery finish claim
    cannot_prove: O(delta) recovery on future writes (that is proven in Track F overhaul)
  - name: projection_base_is_blob
    kind: gate
    baseline: "no consumable projection base artifact exists"
    desired: "first ProjectionBase is a content-addressed blob in platform-artifacts/sha256/projection-base/ bound to canonical head"
    decision_use: gates structural single-appender claim
    cannot_prove: file-CAS durability

phases:
  - name: Define & Supersede
    items:
      - "Write Root Cause Clustering assessment and code-free Define receipt."
      - "Mark choir-host-orchestrated-recovery-2026-08-22.md superseded in registries."
      - "Define ProjectionBase blob schema under /var/lib/go-choir/platform-artifacts/sha256/projection-base/ with atomic tmp+fsync+rename protocol."
      - "Explicitly disable/bypass maxRetained=3 quarantine pruning during recovery runs."
  - name: Implement Offline Rebuilder
    items:
      - "Build isolated offline replay tool: stream envelopes from computer-event/ and batch bodies from computer-event-payload/, decrypt using guest DEK (via trusted-guest copy), apply projection ops in isolated scratch workspace with batched transactions (1,000-5,000 events/tx, RSS <2 GiB), commit once at head 132,436."
      - "Publish first ProjectionBase blob artifact with atomic fsync, bound to canonical head 132,436."
  - name: Staging Recovery & Acceptance
    items:
      - "Execute recovery of 0333528 on staging: guest bootstrap pulls ProjectionBase from host gateway into /mnt/persistent/state, boots with delta=0 replay, verifies final head + witness, CAS route to active under fencing token."
      - "Verify computer boots into a quiesced state (no immediate CoSuper cancellation/supersession churn)."
      - "Verify no-rewind refusals, structural single-appender, and quarantine retention."

now:
  status: working
  slice: "Define & Supersede: write Define receipt, mark 2026-08-22 predecessor superseded, and begin offline rebuilder implementation."
  question: none
  reconciliation:
    observed_at: "2026-08-23T02:00:00Z"
    source_ref: "main@3f9173be"
    deploy_identity: "staging proxy f54eb735 guest f54eb735 computer 0333528 stopped epoch 361"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "working tree owns recovery definition and design"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Recover computer-0333528... to head 132,436 via offline ProjectionBase rebuild as an isolated goal; overhauls proceed under a separate sequenced goal."
    kind: architecture
    status: settled
    source: owner direction 2026-08-23
    evidence_ref: docs/designs/choir-durable-substrate-2026-08-23.md
    owner_ratification_ref: "owner confirmed 2026-08-23"
    recorded_at: "2026-08-23T02:00:00Z"
    consequence: "Execution unblocks 0333528 first before starting the broader substrate overhauls."
  evidence_refs:
    - "docs/designs/choir-durable-substrate-2026-08-23.md"
    - "docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md"
    - "docs/evidence/effects-red-recovery-projections-present-2026-08-23.md"
  blocker_or_risk: "Offline rebuild batching and memory management during 132k event replay on staging."
  next_action: "Write code-free Define receipt and begin offline replay tool implementation."

receipts:
  - id: define-recovery-boundary
    boundary: define
    commit_or_artifact: "(pending)"
    proof_refs: [docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md, docs/evidence/effects-red-recovery-projections-present-2026-08-23.md]
    rollback_ref: revert docs commit
    disposition: "Recovery scope isolated and ratified by owner"
    problem_ref: recovery-no-differential-base-2026-08-23
    authorization_ref: "owner ratification 2026-08-23"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
