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
    introduced: []
    repaired_when_complete:
      - restore that repaints restored state with today's CI SPA
      - a "reconstructible" claim that cannot rebuild the VM-local projection from the tape
      - a computer identity that omits its served surface

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
  status: working
  slice: "Serving hop is guest-static: guest serves CHOIR_UPDATER_ROOT/current/frontend; proxy reverse-proxies authenticated / and /assets/* after vmctl resolve; Caddy no longer file_servers host frontend-current as computer surface; unsigned callers get host platform-shell chrome (OUT of restore). Missing guest SPA is 503. FrontendRestaged remains false. Guest releases still omit frontend/ files, so authenticated surface is fail-closed until a release includes them."
  question: "Can owner-reachable restore restage VM-local state and SPA bytes together, with capability renewal, on the retained computer? Guest-static is picked."
  reconciliation:
    observed_at: 2026-08-13T18:50:00Z
    source_ref: main@e60321940c5a60986c598682da6ff4b6d1b62350
    deploy_identity: "staging deployed db265d1e32e73ab4c51914332eaf6fb55f62a09c; retained computer computer-03335285269bdba4f94377e56879f9e6 active/ready at sequence 1; guest-static serving hop is local product-path code and is not yet a staging runtime identity"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/memo-per-computer-frontend-2026-08-13.md, docs/standing-questions.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-13T18:50:00Z git status --short dirty with guest-static serving hop on e6032194
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Execute /goal through all five acceptance actions until complete, blocked_incomplete, or superseded. Reconstruction product path is Runtime.ReplayCompleteness (event-chain reconstruct into a disposable workspace, then a destructive restage), not ProjectionMaterializer. Pin checkout is evidence-only and cannot complete this Definition. Serving hop remains the slice-3 pick among the three named topologies; checkpoint witness and scope refusal do not require it. Capability renewal is a gate on the restore proof, not a reason to delay checkpoint work. Local tests are slice evidence; complete requires the deployed staging receipts."
    kind: operational
    status: settled
    source: orchestrator
    evidence_ref: "docs/definitions/choir-tape-recovery-2026-08-13.md finish.acceptance; internal/agentcore/replay_completeness.go; .agentic-consensus/tape-recovery-20260813/"
    owner_ratification_ref: "owner direction 2026-08-13 (tape-based recovery is the priority); panel approve-with-conditions adjudicated into this Definition"
    recorded_at: 2026-08-13T17:14:50Z
    consequence: "Do not stop after the first slice. Continue until the finish artifact exists or a named blocker (fail-closed live-only rows, capability-renewal substrate, unpicked hop at slice 3) forces blocked_incomplete or a problem-documentation-first Define."
  evidence_refs: [docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml, .agentic-consensus/tape-recovery-20260813/, internal/agentcore/replay_completeness.go]
  blocker_or_risk: "Guest releases currently omit frontend/ files, so checkpoint SPA binding and authenticated guest-static serving fail closed until a release includes them. ReconstructThrough historical heads is not implemented; current rematerialize replays the full chain and FrontendRestaged is false. Guest capability renewal recurrence remains a restore-proof gate. Deployed checkpoint, scope-refusal, destructive rematerialization, serving-join, and owner-reachable restore receipts are still owed. Fail-closed checkpoint may discover live-only rows on the retained sequence-1 computer."
  next_action: "Land the guest-static red commit through origin/main and CI. Then make rematerialize restage SPA bytes (FrontendRestaged) and exercise owner-reachable whole-computer restore. Staging still owes deployed checkpoint, scope-refusal, destructive rematerialization, and serving-join receipts."

receipts:
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
  - id: tape-recovery-serving-hop-guest-static-2026-08-13
    boundary: implement
    commit_or_artifact: internal/autoputer/computer_surface.go
    proof_refs: [internal/autoputer/computer_surface_test.go, internal/proxy/computer_surface.go, internal/proxy/computer_surface_test.go, nix/node-b.nix]
    rollback_ref: revert the guest-static serving-hop red commit
    disposition: "accepted locally — hop is guest-static. Guest serves staged current/frontend; proxy selects bytes after vmctl resolve; two computers serve two UIs in tests; unsigned callers get host platform shell; Caddy no longer file_servers frontend-current as computer surface. Missing SPA is 503. Deployed serving-join with two divergent computers remains required. Guest releases still omit frontend/ files."
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-13 (tape-based recovery is the priority)
    candidate_or_evidence_refs: [docs/definitions/choir-tape-recovery-2026-08-13.md, docs/memo-per-computer-frontend-2026-08-13.md]
    landing:
      source_commit: pending
      ci_ref: pending
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: pending (serving_join)
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
