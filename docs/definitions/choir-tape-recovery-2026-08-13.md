---
definition_version: 2
definition_id: choir-tape-recovery-2026-08-13
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-13T16:30:00Z
  source:
    canonical_ref: main@f4a72d8b
    deploy_identity: "staging https://choir.news deployed db265d1e32e73ab4c51914332eaf6fb55f62a09c; retained computer computer-03335285269bdba4f94377e56879f9e6 active/ready at sequence 1"
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-13 read-only git status before drafting; owner's frontend-invariant recordings present in the worktree
    preservation_rule: preserve owner's in-flight doctrine/ontology/registry recordings; this Definition owns itself and its registry entries
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-tape-recovery-2026-08-13.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only draft commit
  candidates:
    - id: none
  observed_artifact:
    - claim: "The computer is not restorable from the tape. Checkpoints bind event heads, an opaque state commitment, and release/reconstruction/materialization/verifier digests, but no VM-local Dolt content witness and no frontend identity. ProjectionMaterializer is non-runtime (only a cmd/basecompare analysis CLI plus tests). restorePrior swaps only the release symlink; nothing rematerializes VM-local Dolt state. The frontend is one host-global Caddy SPA (frontend-current) plus per-computer API reverse-proxy; a computer's UI has no ComputerID and is not restored."
      evidence_ref: "internal/selfdevprotocol/control.go:74-94,176-205; internal/computerversion/types.go:40-48; internal/computerversion/projection_materializer.go:9-12; internal/updater/updater.go:554-592; internal/computerversion/dolt_state_extractor.go:27-35,79-202; nix/node-b.nix:23-24,194-207; internal/proxy/handlers.go:653-677; docs/memo-per-computer-frontend-2026-08-13.md:83-105"
    - claim: "The audited-construction mission (2026-07-15, settled) proved fresh same-ComputerVersion reconstruction (dispose data.img, rebuild identical code/artifacts), NOT rematerialization of accumulated VM-local state from an event head. The effects Definition's observed_artifact records the gap honestly. World Wire end-to-end article/lineage production was never receipted and is tabled until after the actor RLM rewrite."
      evidence_ref: "docs/evidence/audited-construction-terminal-receipt-2026-07-17.md; docs/evidence/g4-owner-zero-realization-reconstruction-2026-07-17.json; docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:26-27; docs/current-architecture.md:335-359"
  unknowns:
    - Whether rematerialization is product-ready without the pin-checkout contingency; pin checkout is evidence-only and never a completion route
    - Which of the three serving hops (guest-static, host-staging-keyed-by-computer, encapsulated-origin) this Definition proves; the hop is owned here, not deferred to a successor
    - Whether guest capability renewal (five-minute lifetime) reliably serves tape replay and restore, given the 2026-08-12 refusal recurrence
    - Whether the retained computer's sequence-1 head plus pinned receipts can reproduce every behavior-bearing VM-local row, or some rows exist only in the live Dolt and the checkpoint must fail closed

finish:
  deliver: "A computer's restore set — the event-derived VM-local projection plus the computer-surface frontend — can be put back to a recorded head from the tape: rematerialized, verified against recorded content hashes, and the route/serving join flips visibility only on exact match. Restore-set independence holds; the claim does not extend to platform control-plane, auth material, or non-restore-set guest files."
  artifact: "A deployed staging trajectory proving: a checkpoint that binds event head + CodeRef + ArtifactProgramRef + VM-local Dolt content witness + frontend identity; a runtime rematerializer that rebuilds the VM-local projection through the target head from the tape alone; a per-computer serving hop; and an owner-reachable acceptance-fenced restore that restages state and surface together and refuses on any mismatch."
  acceptance:
    - action: "Extend the checkpoint to bind the target applied/effective event head, CodeRef, ArtifactProgramRef, a VM-local Dolt content witness (schema/table/content-root hashes; Dolt HEAD as audit receipt), and a frontend identity derivable from the release or an explicit frontend digest. Refuse checkpoint creation while any behavior-bearing VM-local row is not event- or receipt-derivable, and while the served SPA is underivable."
      proves: "A checkpoint that cannot be rebuilt from the tape is not recorded; the witness is the verification commitment, not itself the restore address."
      evidence_class: focused fault-injection tests plus a deployed checkpoint on the retained computer
    - action: "Prove scope refusal: a user-computer restore attempt that would touch the shared platform store or cycle state is refused, and those surfaces are not part of the witness or the restore operand."
      proves: "Restore is scoped to the user computer; rewinding shared state is impossible by construction, not by convention."
      evidence_class: focused refusal tests plus a deployed refused attempt
    - action: "Make rematerialization a destructive tape-only product path. Dispose or quarantine the entire original realization, deny the restore implementation access to its data.img and workspace, and reconstruct a new realization using only the canonical event tape and checkpoint-pinned content/artifact inputs, crossing a restart or platform deploy. Verify the reconstructed VM-local Dolt witness against the checkpoint. Pin checkout is evidence-only and is never an accepted completion route."
      proves: "The tape can rebuild the projection; a pass that copies surviving local state would fail, so this is restore, not local rollback."
      evidence_class: deployed destructive rematerialization proof against the retained computer's sequence-1 head
    - action: "Pick and prove one per-computer serving hop for the computer-surface frontend (Desktop/Texture/apps/Settings/assets): split control-plane (TLS/Caddy bootstrap/auth/picker/proxy/vmctl/corpusd/NixOS) OUT from computer surface IN; freeze frontend source and artifacts through the capsule pipeline; select the computer's bytes only after vmctl route resolve. Route CAS greens only after the serving join."
      proves: "Two computers can serve two UIs; a digest without a serving join does not satisfy the invariant."
      evidence_class: deployed serving-join proof with two divergent computers
    - action: "Exercise whole-computer restore through an owner-reachable product API/CLI (not SSH or host scripts): mutate VM-local Dolt state and the served frontend, then restore to a recorded checkpoint and verify both the extractor hash and the served SPA bytes match the checkpoint binding, with guest capability renewal succeeding across the restore without a refusal loop. A later platform deploy must not move the computer back onto host frontend-current. On any mismatch, keep the prior realization and its UI; partial never greens."
      proves: "The computer the user operates — state and surface — is recoverable from the tape through the product path."
      evidence_class: deployed restore proof with state and frontend divergence, owner CLI invocation, and capability-renewal pass
  rollback: "Revert mission commits. Restore never erases the event chain; a failed restore keeps the prior realization and its UI. Do not roll back canonical events; correction for an irreversible consequence is compensation or new forward action."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, staging_build_identity, checkpoint_witness, scope_refusal, destructive_rematerialization, serving_join, owner_reachable_whole_computer_restore, capability_renewal_pass]
  not_done_when:
    - a checkpoint records a digest without a VM-local content witness or a serving join
    - rematerialization is a dry-run probe, a local test, or a pin checkout rather than a destructive tape-only product path
    - the frontend remains host-global and restore repaints restored state with today's CI SPA
    - restore swaps the release pointer without rebuilding VM-local Dolt state and the surface
    - a behavior-bearing row is silently lost rather than refusing the checkpoint
    - restore is reachable only through SSH or host scripts, not an owner product API/CLI
    - guest capability renewal fails across replay or restore, or is unproven
    - the effects Definition still lists checkpoint completeness, revert, scope refusal, or total restore as its own unfinished acceptance
    - World Wire work is started before the actor RLM rewrite

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-13 (tape-based recovery is the priority; frontend is required for whole-computer restore; World Wire tabled until after the actor RLM rewrite), docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/memo-per-computer-frontend-2026-08-13.md, docs/standing-questions.md, AGENTS.md]
  must_preserve:
    - Canonical computer event chain is the single semantic state authority; one guest appender.
    - The event chain is the restore address; VM-local Dolt is a projection and audit witness, never an alternate head. The checkpoint witness is the verification commitment, not the address.
    - Restore never erases history; returning to a prior point is a forward transaction that appends a restore intent, and the tape retains the full record.
    - Restore holds three distinct identities: restore_target_head (the historical checkpoint being realized), canonical_head (the current forward chain including the restore intent), and effective_content_witness (the reversible projection the target checkpoint commits to). Route join binds both the current canonical head and the realized target witness.
    - Restore is acceptance-fenced: stage, verify against recorded content hashes, flip visibility only on exact match; partial never greens.
    - The computer-surface frontend (Desktop, Texture, apps, Settings panes, served asset graph) is part of the computer and in the restore set; the thin platform shell (TLS, Caddy bootstrap, /auth, picker, proxy/vmctl/corpusd, NixOS) is OUT.
    - Checkpoint creation fails closed when any behavior-bearing write is not event- or receipt-derivable, or the served SPA is underivable.
    - The shared platform store and cycle state are out of restore scope; restoring them would rewind other computers.
    - Effects remain OFF until this Definition's restore proof and the effects Definition's decision-policy gates pass.
    - Problem-documentation-first: a discovered platform problem is documented before repair code.
  excluded:
    - Decision-policy / qualified consensus / effect promotion (the effects Definition owns that envelope on top of this substrate).
    - The effects Definition's restore legs — checkpoint completeness, revert build, scope refusal, total restore — are satisfied-by and superseded-by this Definition's receipts; they must not be independently green.
    - RLM/Yaegi actor authoring (successor capability mission, sequenced after the effects Definition).
    - World Wire end-to-end article/lineage production (tabled until after the actor RLM rewrite, so it inherits code-based orchestration).
    - Production environment.
  protected_surfaces: [canonical computer event chain, checkpoint/route projection, materializer + rematerializer, updater root, vmctl lifecycle + route CAS, frontend serving hop, auth/session renewal, gateway/provider calls, deployment routing]
  completion_evidence_floor: [deployed proof, independent review of checkpoint + rematerialization + serving join, focused fault-injection tests]
  conjecture_delta:
    - "The computer's recoverability is its primary property: the restore set (event-derived VM-local projection plus computer-surface frontend) derives from the tape, and restore-set independence holds."
  heresy_delta:
    discovered:
      - checkpoints bind no VM-local content witness and no frontend identity
      - rematerialization has no runtime path; restore is release-pointer-only
      - the frontend is host-global and outside the restore set
      - production never wires CHOIR_UPDATER_ROOT into Runtime, so checkpoint bind cannot see a staged guest SPA
      - rematerialize closes the guest store, so restore is not owner-reachable without restart
      - a pre-rename sandbox_id SPA on a post-rename guest loops BIOS despite healthy computer_id bootstrap
    introduced: []
    repaired_when_complete:
      - restore that repaints restored state with today's CI SPA
      - a "reconstructible" claim that cannot rebuild the VM-local projection from the tape
      - a computer identity that omits its served surface
      - rematerialize that closes the guest store so restore is unreachable without restart (Store.Reopen; capability_renewal_pass 2026-08-15)
      - authenticated computer surface served from host frontend-current (serving_join 2026-08-15)

measures:
  - name: whole-computer restore pass
    kind: gate
    baseline: 0
    desired: "restore to a recorded checkpoint restores VM-local Dolt state AND the served SPA, verified by hash, through the owner product path, on staging before any effect turns on"
    decision_use: unlocks the effects Definition's revert proof
    cannot_prove: the effects envelope, or that restore holds for arbitrary future state
  - name: checkpoint witness coverage
    kind: gate
    baseline: no witness
    desired: every published checkpoint carries a VM-local Dolt witness and a frontend identity
    decision_use: blocks effects promotion when absent
    cannot_prove: that the witness is sufficient for a state excursion never exercised

now:
  status: complete
  slice: "Staging 4ac90583 paid all six required receipts. Independent review of checkpoint + rematerialization + serving join ACCEPT 2026-08-15. Live hashes 2026-08-15T17:10Z: unsigned 4e2d1954, retained 2c74a7b0, secondary 1e62d8b9. This docs-only land stamps complete. Owner-recovery security review remains post-mission and is not a tape-recovery gate. Effects remain OFF."
  question: none
  reconciliation:
    observed_at: 2026-08-15T17:40:00Z
    source_ref: main@ec0aa346
    deploy_identity: "staging deployed 4ac90583 at 2026-08-14T23:24:20Z; retained epoch 268; secondary epoch 12; all six required_receipts paid; serving_join independent review ACCEPT; docs-only terminal-closure land"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/memo-per-computer-frontend-2026-08-13.md, docs/standing-questions.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-15T17:40:00Z docs-only terminal-closure of serving-join evidence, independent-review receipt, Definition complete, ACTIVE, mission-graph, and doc-authority-manifest
    status: reconciled
  candidate:
    id: owner-recovery-checkpoint-publication
    state: landed
  decision:
    selected: "Owner-evidence publication path (option A): CheckpointRequest gains OwnerRecovery; on that route verifier fields must be empty, platform verifies head/receipt/witness-shape server-side, guest attests the VM-local witness, and the restore reconstruction gate remains the enforcement that the witness is true. Distinct decision provenance recorded in the checkpoint receipt kind fields. Effects sequencing intact: route-projection and effects paths reject owner-recovery checkpoints (pinned by test)."
    kind: owner
    status: settled
    source: owner
    evidence_ref: "docs/evidence/tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14.json; .agentic-consensus/tape-recovery-resolve-20260814; this conversation 2026-08-14 (owner ratified option A; security review deferred to post-mission)"
    owner_ratification_ref: "owner direction 2026-08-14: do the owner-evidence path, document it well, security review once missions complete"
    recorded_at: 2026-08-14T09:15:00Z
    consequence: "Red mutation on checkpoint authority (protected surface): full ceremony — conjecture delta, protected surfaces, admissible evidence class, rollback path, heresy delta (discovered: none new; introduced: none; repaired: publication-path sequencing circularity) recorded here. Rollback: git revert of the mission commits restores bind-only checkpoint. Security review obligation recorded for post-mission. serving_join is paid: a second existing interactive computer (a@b.com) served a divergent SPA after vmctl resolve. Independent review ACCEPT 2026-08-15."
  evidence_refs: [docs/evidence/tape-recovery-serving-join-independent-review-2026-08-15.md, docs/evidence/tape-recovery-serving-join-2026-08-15.json, docs/evidence/tape-recovery-secondary-bootstrap-incident-2026-08-15.json, docs/evidence/tape-recovery-capability-renewal-pass-2026-08-15.json, docs/evidence/tape-recovery-owner-restore-2026-08-14.json, docs/ACTIVE.md, docs/mission-graph.yaml]
  blocker_or_risk: "None for this Definition. Owner-recovery publication still carries the guest-attested witness trust split for the post-mission security review. No owner restage-frontend verb. Effects remain OFF until the effects Definition's decision-policy gates pass."
  next_action: "Invoke /goal docs/definitions/choir-supervised-self-development-effects-2026-08-11.md for the decision-policy envelope. Do not rematerialize. Do not invent choir computer create. Do not enable effects."

receipts:
  - id: tape-recovery-serving-join-independent-review-2026-08-15
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-serving-join-independent-review-2026-08-15.md
    proof_refs: [docs/evidence/tape-recovery-serving-join-independent-review-2026-08-15.md, .agentic-consensus/tape-recovery-serving-join-20260815/manifest.tsv, docs/evidence/tape-recovery-serving-join-2026-08-15.json]
    rollback_ref: revert the docs-only independent-review receipt
    disposition: "independent review ACCEPT — five completed panelists paid serving_join and the bundled floor; Devin REPAIR only for Git/registry land; Codex failed CLI flag; opencode and DeepSeek timed out. This docs-only terminal-closure commit is the land; Definition now.status is complete."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority); Definition completion_evidence_floor
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: ec0aa346
      ci_ref: not_applicable
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268; secondary epoch 12
      deployed_acceptance: not_applicable
    registry_conformance_ref: "registered 2026-08-15 across docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml (tape-recovery complete/settled; effects entrypoint true)"
  - id: tape-recovery-serving-join-2026-08-15
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-serving-join-2026-08-15.json
    proof_refs: [docs/evidence/tape-recovery-serving-join-2026-08-15.json, docs/evidence/tape-recovery-secondary-bootstrap-incident-2026-08-15.json, internal/autoputer/computer_surface.go, internal/proxy/computer_surface.go]
    rollback_ref: revert guest-static serving-hop 490e779b; secondary prior release retained at choir-updater/releases/secondary-divergent-20260815T030500Z
    disposition: "deployed — unsigned choir.news serves host shell index-YTmyLpSn.js sha 4e2d1954; retained computer-033352 epoch 268 serves index-BH09hKq-.js sha 2c74a7b0; secondary computer-bb0f4fa epoch 12 serves index-BgRdleu6.js sha 1e62d8b9 after computer_id restage over sandbox_id BIOS loop. Owner desktop loaded. serving_join paid. Independent review ACCEPT 2026-08-15. Docs-only terminal-closure land stamps Definition complete."
    problem_ref: tape-recovery-secondary-bootstrap-incident-2026-08-15
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority); owner authorized SSH to node-b; owner authenticated as a@b.com on the existing secondary computer
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md, docs/memo-per-computer-frontend-2026-08-13.md]
    landing:
      source_commit: 4ac90583
      ci_ref: "31848671245 success (Deploy to Staging Node B)"
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268; secondary epoch 12
      deployed_acceptance: serving_join paid with two divergent computers plus unsigned platform shell
    registry_conformance_ref: not_applicable
  - id: tape-recovery-capability-renewal-pass-2026-08-15
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-capability-renewal-pass-2026-08-15.json
    proof_refs: [docs/evidence/tape-recovery-capability-renewal-pass-2026-08-15.json, internal/store/store.go, internal/selfdev/credentials.go, internal/agentcore/rematerialize.go]
    rollback_ref: quarantine /mnt/persistent/rematerialize-quarantine-20260815T003923.233034401Z
    disposition: "deployed — choir computer restore on staging 4ac90583 epoch 268 returned store_closed=false; SPA restored to 2c74a7b0; live-only Texture doc 404; choir computer replay-completeness succeeded at 00:42:51Z and 00:45:06Z without start/restart. capability_renewal_pass paid. serving_join paid 2026-08-15."
    problem_ref: tape-recovery-rematerialize-closes-store-2026-08-14
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority); owner authorized SSH to node-b
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 4ac90583
      ci_ref: "31848671245 success (Deploy to Staging Node B)"
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583 at 2026-08-14T23:24:20Z; retained computer epoch 268
      deployed_acceptance: capability_renewal_pass paid across restore without subsequent start
    registry_conformance_ref: not_applicable
  - id: tape-recovery-checkpoint-witness-published-2026-08-14
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-checkpoint-witness-published-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-checkpoint-witness-published-2026-08-14.json, internal/selfdevprotocol/control.go, internal/platform/checkpoints.go, internal/agentcore/rematerialize.go]
    rollback_ref: revert 57e2992d
    disposition: "deployed — choir computer checkpoint published owner-recovery checkpoint 70f9ce2b on computer-03335285269bdba4f94377e56879f9e6 after RefreshVM to 57e2992d epoch 261. 40-table witness, release-bound frontend identity, empty verifier fields. Unpaid: capability_renewal_pass across restore, serving_join."
    problem_ref: tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14
    authorization_ref: owner direction 2026-08-14 (owner-evidence publication path)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 57e2992d
      ci_ref: "31842366975 success (Deploy to Staging Node B)"
      deploy_ref: 57e2992de16e4114c079912c473a0d20aee2aff7
      environment_identity: staging https://choir.news deployed 57e2992d at 2026-08-14T21:51:10Z; retained computer epoch 261
      deployed_acceptance: checkpoint_witness published
    registry_conformance_ref: not_applicable
  - id: tape-recovery-destructive-rematerialization-2026-08-14
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-destructive-rematerialization-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-destructive-rematerialization-2026-08-14.json, internal/agentcore/rematerialize.go]
    rollback_ref: quarantine /mnt/persistent/rematerialize-quarantine-20260814T221120.832064307Z
    disposition: "deployed — choir computer rematerialize-from-tape reconstructed through checkpoint 70f9ce2b, witness_matched, original_denied, frontend_restaged, pin_checkout unused. store_closed=true left the guest degraded until RefreshVM epoch 262. Named problem: rematerialize is not owner-reachable without restart. Unpaid: capability_renewal_pass, serving_join."
    problem_ref: tape-recovery-rematerialize-closes-store-2026-08-14
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 57e2992d
      ci_ref: "31842366975 success"
      deploy_ref: 57e2992de16e4114c079912c473a0d20aee2aff7
      environment_identity: staging https://choir.news deployed 57e2992d; rematerialize epoch 262
      deployed_acceptance: destructive_rematerialization paid; store_closed problem named
    registry_conformance_ref: not_applicable
  - id: tape-recovery-owner-restore-2026-08-14
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-owner-restore-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-owner-restore-2026-08-14.json, cmd/choir/main.go, internal/agentcore/rematerialize.go]
    rollback_ref: quarantine /mnt/persistent/rematerialize-quarantine-20260814T221659.842773134Z
    disposition: "deployed — choir computer restore after Texture+SPA mutation matched checkpoint SPA bytes and dropped the live-only Texture doc. Restore itself is owner CLI. Guest was 502/degraded until choir computer start epoch 264 because rematerialize closed the store. capability_renewal_pass unpaid (start is credential re-exchange, not in-process renewal). serving_join owner-blocked."
    problem_ref: tape-recovery-rematerialize-closes-store-2026-08-14
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 57e2992d
      ci_ref: "31842366975 success"
      deploy_ref: 57e2992de16e4114c079912c473a0d20aee2aff7
      environment_identity: staging https://choir.news deployed 57e2992d; restore then start epoch 264
      deployed_acceptance: owner_reachable_whole_computer_restore paid through owner CLI; post-restore reachability unpaid until store reopen
    registry_conformance_ref: not_applicable
  - id: tape-recovery-eligible-bind-no-owner-publication-2026-08-14
    boundary: define
    commit_or_artifact: docs/evidence/tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14.json, internal/agentcore/rematerialize.go, internal/platform/checkpoints.go, internal/agentcore/self_development_materializer.go]
    rollback_ref: "workspace quarantine dir /mnt/persistent/workspace-replaced-20260814T084541.510816938Z (host rename-back); revert docs commits"
    disposition: "working — eligibility flipped true after owner-ratified workspace-replace cutover; checkpoint bind eligible=true; named new problem: no owner-reachable checkpoint PUBLICATION path for an accumulated computer (verifier evidence sequencing circularity). Unpaid: published checkpoint_witness, destructive_rematerialization, serving_join, owner_reachable_whole_computer_restore, capability_renewal_pass."
    problem_ref: tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14
    authorization_ref: owner direction 2026-08-14 (review, consensus, debug, resolve); effects-Definition owner-ratified workspace-replace exception
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: c1a1a132
      ci_ref: "31776629641 success (deploy 10dfa594)"
      deploy_ref: 10dfa594a625de03c0693cecae53ee0c7ac43ea0
      environment_identity: "staging choir.news 10dfa594; computer epoch 259; replay eligible=true; bind eligible=true"
      deployed_acceptance: "working — major progress (eligibility + bind); publication gap named and documented"
    registry_conformance_ref: "verified 2026-08-14: mission-graph tape-recovery entrypoint true; ACTIVE.md updated this commit"
  - id: tape-recovery-checkpoint-bind-live-only-after-updater-wire-2026-08-14
    boundary: define
    commit_or_artifact: docs/evidence/tape-recovery-checkpoint-bind-live-only-after-updater-wire-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-checkpoint-bind-live-only-after-updater-wire-2026-08-14.json, internal/autoputer/run.go, internal/agentcore/rematerialize.go]
    rollback_ref: revert the docs-only live-only-after-wire stamp
    disposition: "blocked_incomplete — updater-root wiring verified on staging 10dfa594 epoch 258. Checkpoint bind HTTP 409 replay is ineligible (nine live-only tables), no longer SPA underivable. Unpaid: published checkpoint_witness, destructive_rematerialization, serving_join, owner_reachable_whole_computer_restore, capability_renewal_pass. Do not rematerialize. Do not stamp complete."
    problem_ref: tape-recovery-retained-computer-live-only-rows-2026-08-13
    authorization_ref: owner direction 2026-08-14 (SSH to node-b authorized to continue); owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 10dfa594
      ci_ref: "31776629641 success (product deploy 10dfa594)"
      deploy_ref: 10dfa594a625de03c0693cecae53ee0c7ac43ea0
      environment_identity: staging https://choir.news deployed 10dfa594 at 2026-08-14T06:47:48Z; retained computer epoch 258
      deployed_acceptance: blocked_incomplete (scope_refusal paid; checkpoint_witness fail-closed 409 live-only rows after updater wire; remaining four receipts unpaid)
    registry_conformance_ref: "verified 2026-08-14: docs/mission-graph.yaml tape-recovery status blocked with entrypoint true; docs/ACTIVE.md records blocked_incomplete; docs/doc-authority-manifest.yaml remains is_root [authority, entry]"
  - id: tape-recovery-checkpoint-updater-root-unwired-2026-08-14
    boundary: define
    commit_or_artifact: docs/evidence/tape-recovery-checkpoint-updater-root-unwired-2026-08-14.json
    proof_refs: [docs/evidence/tape-recovery-checkpoint-updater-root-unwired-2026-08-14.json, internal/agentcore/runtime.go, internal/agentcore/rematerialize.go, internal/autoputer/computer_surface.go]
    rollback_ref: revert the docs-only updater-root problem stamp
    disposition: "blocked_incomplete — host-staged current/frontend makes guest / HTTP 200, but checkpoint bind still HTTP 409 served SPA is underivable because production never calls WithSelfDevelopmentUpdater. Live-only rows remain. Unpaid: published checkpoint_witness, destructive_rematerialization, serving_join, owner_reachable_whole_computer_restore, capability_renewal_pass. Do not rematerialize. Do not stamp complete."
    problem_ref: tape-recovery-checkpoint-updater-root-unwired-2026-08-14
    authorization_ref: owner direction 2026-08-14 (SSH to node-b authorized to continue); owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: fdb8f413
      ci_ref: pending docs-only push
      deploy_ref: 8a172b84f7285a83d3c502ad2e9e7c2ed4b65307
      environment_identity: staging https://choir.news deployed 8a172b84 at 2026-08-13T20:45:20Z; retained computer epoch 257
      deployed_acceptance: blocked_incomplete (scope_refusal paid; checkpoint_witness fail-closed 409 after staged SPA; remaining four receipts unpaid)
    registry_conformance_ref: "verified 2026-08-14: docs/mission-graph.yaml tape-recovery status blocked with entrypoint true; docs/ACTIVE.md records blocked_incomplete; docs/doc-authority-manifest.yaml remains is_root [authority, entry]"
  - id: tape-recovery-blocked-incomplete-no-eligible-computer-2026-08-13
    boundary: define
    commit_or_artifact: docs/evidence/tape-recovery-blocked-incomplete-2026-08-13.json
    proof_refs: [docs/evidence/tape-recovery-blocked-incomplete-2026-08-13.json, docs/evidence/tape-recovery-checkpoint-bind-refusal-2026-08-13.json, cmd/choir/main.go, internal/vmctl/handlers.go]
    rollback_ref: revert the docs-only blocked_incomplete stamp
    disposition: "blocked_incomplete — owner product path cannot reach an eligible computer. choir computer create is unknown. vmctl resolve provisions only the primary desktop. Retained computer remains the only interactive VM; checkpoint bind still HTTP 409 served SPA is underivable and did not publish. Unpaid: destructive_rematerialization, serving_join, owner_reachable_whole_computer_restore, capability_renewal_pass. Do not rematerialize. Do not stamp complete."
    problem_ref: tape-recovery-retained-computer-live-only-rows-2026-08-13
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority); Definition next_action authorized this stop
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: d839ec9d
      ci_ref: "31740343847 success (product deploy 8a172b84)"
      deploy_ref: 8a172b84f7285a83d3c502ad2e9e7c2ed4b65307
      environment_identity: staging https://choir.news deployed 8a172b84 at 2026-08-13T20:45:20Z
      deployed_acceptance: blocked_incomplete (scope_refusal paid; checkpoint_witness fail-closed 409; remaining four receipts unpaid)
    registry_conformance_ref: "verified 2026-08-13: docs/mission-graph.yaml tape-recovery status blocked with entrypoint true; docs/ACTIVE.md records blocked_incomplete; docs/doc-authority-manifest.yaml remains is_root [authority, entry]"
  - id: tape-recovery-draft-consensus-2026-08-13
    boundary: define
    commit_or_artifact: .agentic-consensus/tape-recovery-20260813/
    proof_refs: [".agentic-consensus/tape-recovery-20260813/manifest.tsv (6 succeeded, 1 failed devin, 1 timed-out deepseek)"]
    rollback_ref: not_applicable (draft review)
    disposition: "approved with conditions; ten conditions adjudicated into this revision; landed 1ecbf9e6 (Definition + registry) and 0c16d8f7 (effects restore-leg carve-out)"
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/memo-per-computer-frontend-2026-08-13.md]
    landing:
      source_commit: 1ecbf9e6
      ci_ref: local doccheck pass
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "registered 2026-08-13 in 1ecbf9e6 across docs/ACTIVE.md Active Definition, docs/mission-graph.yaml entrypoint true, docs/doc-authority-manifest.yaml authority/entry; effects restore-leg carve-out in 0c16d8f7; ACTIVE Invocation and residual effects restore claims closed in the 2026-08-13 Define reconcile"
  - id: tape-recovery-define-reconcile-2026-08-13
    boundary: define
    commit_or_artifact: docs/definitions/choir-tape-recovery-2026-08-13.md
    proof_refs: [docs/ACTIVE.md, docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
    rollback_ref: revert the docs-only Define reconcile commit
    disposition: "accepted — now reconciled to 0c16d8f7; reconstruction named as ReplayCompleteness; effects duplicate now keys and leftover restore-scope claims closed; /goal continues through all five acceptance actions"
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 0c03a479
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "verified 2026-08-13: docs/ACTIVE.md Invocation points at choir-tape-recovery-2026-08-13; docs/mission-graph.yaml unique product entrypoint true; docs/doc-authority-manifest.yaml tape-recovery is_root [authority, entry] and effects is_root []; effects restore leftovers closed in the same commit"
  - id: tape-recovery-checkpoint-witness-2026-08-13
    boundary: implement
    commit_or_artifact: internal/selfdevprotocol/restore.go
    proof_refs: [internal/selfdevprotocol/restore_test.go, internal/selfdevprotocol/control.go, internal/agentcore/checkpoint_restore_bindings.go]
    rollback_ref: revert the checkpoint-witness red commit
    disposition: "accepted locally — CheckpointFromRequest binds VM-local witness + frontend identity and refuses platform/cycle; RestoreFromRequest refuses those operands; genesis/materializer populate bindings from ReplayCompleteness. Deployed checkpoint and deployed refused attempt remain required."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 6b28999e
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: pending (checkpoint_witness, scope_refusal)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-rematerialize-2026-08-13
    boundary: implement
    commit_or_artifact: internal/agentcore/rematerialize.go
    proof_refs: [internal/agentcore/rematerialize_test.go, internal/selfdevprotocol/rematerialize.go, cmd/choir/main.go, internal/proxy/computer_lifecycle.go]
    rollback_ref: revert the rematerialize red commit
    disposition: "accepted locally — RematerializeFromTape reconstructs via ReconstructInto into a sibling workspace, verifies the extractor witness, quarantines the original realization, and flips only on match. Pin checkout is refused. FrontendRestaged is false. Full-chain reconstruct equals through-target at sequence 1. Deployed destructive rematerialization remains required."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: d2558168
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: pending (destructive_rematerialization)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-deployed-checkpoint-bind-refusal-2026-08-13
    boundary: implement
    commit_or_artifact: docs/evidence/tape-recovery-checkpoint-bind-refusal-2026-08-13.json
    proof_refs: [docs/evidence/tape-recovery-checkpoint-bind-refusal-2026-08-13.json, internal/agentcore/rematerialize.go]
    rollback_ref: revert the evidence/docs stamp
    disposition: "deployed fail-closed checkpoint — POST /lifecycle/checkpoint on computer-03335285269bdba4f94377e56879f9e6 at epoch 252 returned HTTP 409 served SPA is underivable and did not publish. Scope refusal still HTTP 400 for platform and cycle. Authenticated guest surface is 503. Replay probe digest 8d96ba02 remains ineligible. Destructive rematerialization, serving-join, owner restore, and capability-renewal-across-restore remain unpaid."
    problem_ref: tape-recovery-retained-computer-live-only-rows-2026-08-13
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 8a172b84
      ci_ref: "31740343847 success"
      deploy_ref: 8a172b84f7285a83d3c502ad2e9e7c2ed4b65307
      environment_identity: staging https://choir.news deployed 8a172b84 at 2026-08-13T20:45:20Z
      deployed_acceptance: partial (checkpoint_witness fail-closed 409; scope_refusal collected; serving_join unpaid; destructive_rematerialization not attempted)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-retained-computer-live-only-rows-2026-08-13
    boundary: define
    commit_or_artifact: docs/evidence/tape-recovery-retained-computer-2026-08-13.json
    proof_refs: [docs/evidence/tape-recovery-retained-computer-2026-08-13.json]
    rollback_ref: revert the evidence/docs stamp
    disposition: "problem documented — replay probe digest 0b8354ce on retained computer computer-03335285269bdba4f94377e56879f9e6 at sequence 3 / epoch 251 is ineligible. Nine behavior-bearing texture tables are non-empty in live Dolt and empty after tape replay. Checkpoint must fail closed. Deployed restore refused platform and cycle operands (HTTP 400). Long-idle capability renewal refused; owner restart then replay succeeded. Do not add reducers until this record lands."
    problem_ref: tape-recovery-retained-computer-live-only-rows-2026-08-13
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority); problem-documentation-first
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: a2a80630
      ci_ref: pending
      deploy_ref: eb91808a8b935ee8ea356fec2932b05a0a21f776
      environment_identity: staging https://choir.news deployed eb91808a
      deployed_acceptance: partial (scope_refusal collected; checkpoint_witness fail-closed by ineligibility; destructive rematerialization not attempted)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-reconstruct-through-restore-intent-2026-08-13
    boundary: implement
    commit_or_artifact: internal/computerevent/appender.go
    proof_refs: [internal/agentcore/rematerialize.go, internal/computerevent/computerevent_test.go, internal/agentcore/rematerialize_test.go]
    rollback_ref: revert the reconstruct-through red commit
    disposition: "accepted locally and pushed — ReconstructThroughTarget halts at checkpoint AcceptedEventHead; owner restore appends EventRestoreRequested first. Not yet a staging runtime identity. Deployed restore receipts remain required on an eligible computer."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: a2a80630
      ci_ref: "31740343847 success"
      deploy_ref: 8a172b84f7285a83d3c502ad2e9e7c2ed4b65307
      environment_identity: staging https://choir.news deployed 8a172b84
      deployed_acceptance: pending (through-target reconstruct is deployed; restore receipts still require an eligible computer)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-freeze-frontend-capability-grace-2026-08-13
    boundary: implement
    commit_or_artifact: internal/capsule/executor.go
    proof_refs: [internal/capsule/source_snapshot.go, internal/capsule/release_secret_test.go, internal/platform/event_capability.go, internal/platform/event_artifacts_test.go]
    rollback_ref: revert the freeze/renewal red commit
    disposition: "accepted locally — capsule spawn and StageGrantedRelease fail closed without frontend source/artifacts; credential renewal accepts 60s expiry grace. Deployed serving-join and capability_renewal_pass remain required. Idle past TTL+grace still fails closed."
    problem_ref: replay-probe-credential-renewal-refused-2026-08-12
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: 8bbba401
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: pending (serving_join, capability_renewal_pass)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-spa-restage-owner-restore-2026-08-13
    boundary: implement
    commit_or_artifact: internal/agentcore/rematerialize.go
    proof_refs: [internal/agentcore/rematerialize_test.go, internal/updater/updater.go, cmd/choir/main.go, internal/proxy/computer_lifecycle.go, flake.nix]
    rollback_ref: revert the SPA-restage/owner-restore red commit
    disposition: "accepted locally — RematerializeFromTape restages current onto the checkpoint-pinned release after witness match (FrontendRestaged true); mismatch keeps prior realization and UI; guest autoputer baseline includes frontend/; choir computer restore posts vm_local plus computer_surface_frontend. Capability renewal and deployed restore receipts remain required."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md]
    landing:
      source_commit: cd403f98
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: pending (owner_reachable_whole_computer_restore, capability_renewal_pass)
    registry_conformance_ref: not_applicable
  - id: tape-recovery-serving-hop-guest-static-2026-08-13
    boundary: implement
    commit_or_artifact: internal/autoputer/computer_surface.go
    proof_refs: [internal/autoputer/computer_surface_test.go, internal/proxy/computer_surface.go, internal/proxy/computer_surface_test.go, nix/node-b.nix]
    rollback_ref: revert the guest-static serving-hop red commit
    disposition: "accepted locally — hop is guest-static. Guest serves staged current/frontend; proxy selects bytes after vmctl resolve; two computers serve two UIs in tests; unsigned callers get host platform shell; Caddy no longer file_servers frontend-current as computer surface. Missing SPA is 503. Deployed serving-join paid 2026-08-15: two live computers plus unsigned platform shell serve three distinct index.html hashes."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md, docs/memo-per-computer-frontend-2026-08-13.md]
    landing:
      source_commit: 490e779b
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: serving_join paid 2026-08-15
    registry_conformance_ref: not_applicable

view:
  path: none
  generator: none
---

# Tape-Based Recovery

The computer's recoverability is its primary property. A computer's restore set —
the event-derived VM-local projection and the computer-surface frontend — can be
put back to a recorded head from the tape, verified against recorded content
hashes, and the route/serving join flips visibility only on exact match.

The audited-construction mission proved a computer's *code and artifacts* are
reconstructible from an immutable `ComputerVersion`. It did not prove that a
computer's *accumulated state* — the rows and served surface a person operates —
can be rebuilt from the event head. This Definition closes that gap.

Three things must become true:

1. **A checkpoint binds the whole restore set.** Event head, `CodeRef`,
   `ArtifactProgramRef`, a VM-local Dolt content witness, and the served
   frontend identity. A checkpoint that cannot be rebuilt from the tape, or that
   cannot serve the UI it claims, is refused.

2. **Rematerialization is a destructive tape-only runtime path.**
   Event-chain reconstruction already lives in `Runtime.ReplayCompleteness`
   (`internal/agentcore/replay_completeness.go`); promote that into the restore
   procedure — stage a new realization from the tape, verify the extractor
   match, flip visibility. `ProjectionMaterializer` turns an already-extracted
   `ObservationSet` into a `Realization` and is not the restore path. The
   original realization is disposed or quarantined and denied to the restore
   implementation. Pin checkout is evidence-only, never a completion route.

3. **The frontend is part of the computer.** One host-global SPA is a shared
   platform accident, not a computer's surface. The computer-surface UI is
   authored, frozen, accepted, and restored through the same pipeline as the
   state; the thin platform shell stays OUT. This Definition owns and proves the
   serving hop; it does not defer it to a successor.

Restore is a forward transaction: it appends a restore intent and holds three
distinct identities — the `restore_target_head`, the `canonical_head` that now
includes the restore intent, and the `effective_content_witness` the target
checkpoint commits to. The witness is the verification commitment; the event
head plus pinned refs is the restore address.

Sequencing: this Definition is the foundation. The effects Definition's
decision-policy envelope rides on it, and its restore legs (checkpoint
completeness, revert build, scope refusal, total restore) are satisfied-by this
Definition. The private-Go actor RLM rewrite follows the effects envelope.
World Wire is tabled until after the RLM rewrite, so its processor pipeline
inherits code-based orchestration instead of the old workflow structure. This
supersedes the frontend memo's "after or beside effects" packaging; under the
2026-08-13 priority the whole-computer restore — state and surface together — is
the foundation.
