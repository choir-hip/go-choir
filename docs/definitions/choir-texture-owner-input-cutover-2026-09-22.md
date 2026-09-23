---
definition_version: 3
definition_id: choir-texture-owner-input-cutover-2026-09-22
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-22T23:30:00Z'
  source:
    canonical_ref: main@6d9b3e97
    deploy_identity: staging https://choir.news — proxy/vmctl OK; deployed
      commit not surfaced in /health payload (record as observed gap)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  predecessor:
    mission: choir-rlm-engineering-carrier-2026-09-11
    disposition: superseded — this mission is the new sole working entrypoint.
      The carrier's remaining scope (engineering desk on in-cell carrier,
      R7/R9 deletions, canonical run acceptance) becomes mission M2 in the
      ordered stack; M1 is the input-path cutover that unblocks it.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (ordered mission
      list, consensus 2026-09-22)
  observed_artifact:
    - claim: 'Owner input to a lifecycle-bound Texture document is a
        LifecycleOwnerInstruction side channel: /revise forwards to /tell
        (texture_agent_revision.go:117-121), which queues an owner-instruction
        object and schedules a handler-created wake (scheduleTextureWorkerWake,
        texture_controller.go:21). The roster driver (cmd/choir/roster.go)
        posts to /tell.'
      claim_scope: current
      evidence_ref: internal/textureowner/texture_agent_revision.go:117-121;
        internal/textureowner/texture_owner_instruction.go;
        internal/textureowner/texture_controller.go:21;
        internal/store/lifecycle_owner_instruction.go;
        cmd/choir/roster.go
    - claim: 'A canonical document revision already exists in the same commit:
        the OwnerCorrection path in store/lifecycle.go creates the revision AND
        the owner instruction together (lifecycle.go:3262-3383), and
        texture.go:1309 already emits a document revision event. The LOI is the
        extra occurrence/wake, not the revision itself.'
      claim_scope: current
      evidence_ref: internal/store/lifecycle.go:3262-3383;
        internal/textureowner/texture.go:1309
    - claim: 'The LOI consumer surface is ~28 files: store (queue/get/list,
        turn-commit OwnerInstructions binding, atomic consume), agentcore
        (super_controller pending-instruction reads, selfdev synthetic tell,
        TextureOwnerInstructionOccurrence), actorruntime (wake-time conversion),
        textureowner (boot scan, occurrence matching, wake scheduling, routes),
        types, graph_store (ogKindOwnerInstruction), cmd/choir (roster, tell,
        correct).'
      claim_scope: current
      evidence_ref: grep LifecycleOwnerInstruction across internal/ + cmd/
        (~28 files)

finish:
  deliver: >-
    An owner edits a Texture document and the edit is a canonical document
    revision event on the tape — the desk observes the new head. The
    tell/correct/roster/LifecycleOwnerInstruction side channel no longer
    exists. Texture owner input is fixed.
  artifact: >-
    Deployed staging state where POST /api/texture/documents/{id}/revise on a
    lifecycle-bound document commits an owner-authored revision on the
    canonical path (no /tell forward), the Texture actor's turn is caused by
    the document revision event, and the deleted ingress paths return 404 /
    "unknown command".
  acceptance:
    - action: >-
        On staging, owner edits a bound Texture document via the product path;
        observe a new revision on the tape and the desk observing the head.
      proves: owner input is a canonical document revision event
      evidence_class: deployed proof
    - action: >-
        `choir roster`, `choir texture tell`, `choir texture correct` return
        "unknown command"; POST /tell and /correct return 404.
      proves: the wrong-path ingress is gone
      evidence_class: deployed proof
    - action: >-
        No new lifecycle_owner_instruction rows are created by an owner edit;
        no scheduleTextureWorkerWake fires for owner input.
      proves: the side channel is not silently recreated
      evidence_class: deployed proof + code inspection
    - action: >-
        `go build ./...` and `go test ./internal/store ./internal/textureowner
        ./internal/agentcore ./internal/actorruntime ./cmd/choir` pass.
      proves: the deletion compiles and the surviving paths hold
      evidence_class: local test
  rollback: >-
    git revert of the mission commits + redeploy. The deleted ingress is
    additive-removal: no schema migration destroys data, so revert restores
    the prior behavior. Leftover pending LOI rows in existing stores are
    orphaned objects — harmless, consumed by nothing after deletion.
  landing:
    required: true
    environment: staging
    required_receipts:
      - pushed_commit
      - ci
      - deploy
      - environment_identity
      - deployed_acceptance

value:
  better_means: >-
    Minimize the number of live owner-input channels to exactly one (the
    document revision event) while preserving the ability of an owner edit to
    reach a live Texture desk.
  goodharting_would_be: >-
    Deleting roster.go and the CLI verbs while leaving /tell and
    LifecycleOwnerInstruction live — the visible wrong path is gone but the
    side channel still carries owner input. Or: replacing tell with a
    tell-shaped "revision-wake" side table that M3 would have to delete again.

homotopy:
  realism_axis: >-
    Input-path authority: from "owner edit is a side-table instruction + a
    handler-created wake" (current, low) to "owner edit is a document revision
    event; the desk derives its activation from the head" (target, high). The
    intermediate rung — revision event exists, existing Texture actor still
    consumes it via the current occurrence mechanism — is a valid projection:
    same interface, same state transitions, no fake island.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction 2026-09-22 (tell is a hallucination; roster is a sub-RLM
      call — delete, not migrate)
    - ratified ontology cutover (docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md)
    - ordered mission list consensus (docs/world-wire-mission-stack-2026-09-22.md)
    - wrong-path cluster assessment (docs/problems/root-cause-wrong-path-cluster-2026-09-22.md)
  must_preserve:
    - Owner edits to lifecycle-bound documents still reach the Texture desk.
    - The canonical document revision event (already emitted at texture.go:1309)
      is the input; no new side-table or wake object is invented.
    - The unbound /revise legacy mailbox path is either deleted in this mission
      or explicitly deferred to M3 with a named exception.
    - `run start` prompt-bar ingress is NOT touched — it is a canonical durable
      lifecycle-start path, not a wrong-path instance.
    - Mail ingestion, webhook, and draft paths are untouched.
  excluded:
    - Overlay tool deletions (M2/R7 scope).
    - actuator=tools and Super substrate retirement (M4 scope).
    - Process-local wakes, sweeps, non-event mutations beyond the owner-input
      path (M3 scope).
    - install_frontend_pointer, vmctl sweeper, sourcecycled (deferred).
    - The remaining desk crossings (M4/1e).
  protected_surfaces:
    - Texture canonical writes (the revision commit path).
    - Lifecycle event append (owner_instruction_queued is deleted; the
      document revision event is the replacement).
    - Run acceptance (no change to acceptance semantics in this mission).

now:
  status: working
  slice: M1 — owner input is a document revision; wrong-path ingress deleted
  source_ref: main@6d9b3e97
  deploy_identity: staging https://choir.news (proxy/vmctl OK)
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: m1-revision-is-the-event
    claim: >-
      The owner edit already produces a canonical document revision (the
      OwnerCorrection commit creates revision + owner instruction together);
      the LOI is only the extra occurrence/wake. If true, M1 is "stop the
      extra wake, delete the channel" — the smallest honest cutover.
    test: >-
      Read the OwnerCorrection commit path (store/lifecycle.go:3262-3383) and
      the revision event emission (texture.go:1309): does the revision exist
      independently of the instruction, and does the Texture actor's turn
      already consume the document head?
    edge: missing_oracle — CONFIRMED 2026-09-22. The revision exists
      independently (OwnerCorrection block is an add-on inside the commit),
      but no backend subscriber converts LifecycleArtifactHeadAdvanced into an
      actor wake. M1 grows the small adapter: a revision-keyed
      TextureActorOccurrence kind dispatched by the commit caller.
    delta_o: >-
      Trace one owner edit end-to-end on staging: does the desk activate from
      the revision alone when the instruction wake is removed?
    scope_if_supported: >-
      M1 deletes the LOI channel without a new wake table; the desk's
      activation derives from the document head.
    status: supported
    evidence_refs:
      - internal/store/lifecycle.go:3262-3383
      - internal/textureowner/texture.go:1309
      - internal/textureowner/texture_controller.go (occurrence machinery)
  decision:
    what: >-
      M1 is the subclass-(a) input-path cutover: replacement and deletion in
      the same mission. Not leaf-only (leaves /tell live), not the full
      ~44-instance cutover (M3).
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (ordered mission
      list, consensus 2026-09-22)
    owner_ratification_ref: owner direction 2026-09-22 (tell is a
      hallucination; roster is a sub-RLM call; start with cleanup)
  belief:
    believed_state: >-
      Design settled 2026-09-22. Owner edit commits an AuthorUser revision
      (direct edit: new content; /revise: head content carried forward with
      input_origin=user_prompt + owner_prompt metadata). The commit caller
      dispatches a new TextureActorOccurrence kind "document_revision" keyed
      to HeadRevisionID. Pending = doc.CurrentRevisionID==R &&
      R.AuthorKind==user && no texture_turn_committed event with
      ArtifactRefs[1]==R (consume marker derived from the event tape; no new
      table). ApplyTextureTurn drops the OwnerInstructions binding; the head
      CAS is the fence. Injection renders the head revision's owner_prompt.
      Self-dev rewake commits a synthetic owner revision. Unbound /revise
      mailbox DELETED (named decision: same wrong-path class; unbound docs
      get explicit 409).
    main_uncertainty: >-
      Whether any non-obvious consumer reads LifecycleResult.OwnerInstruction
      or the owner_instruction_queued event (frontend observation projections,
      CLI). Census says none load-bearing; build will confirm.
    next_observation: >-
      go build ./... plus focused tests; then staging trace of one owner edit.
  blocker_or_risk: >-
    M1 dual-path risk: if the revision event coexists with a leftover
    scheduleTextureWorkerWake, tell is recreated. Acceptance forbids the side
    table. Resolved: unbound /revise mailbox deleted in this mission (named
    exception consumed, not deferred).
  next_action: >-
    Implement the cutover: (1) /revise commits owner revision + dispatches
    document_revision occurrence; (2) delete /tell,/correct, LOI type/store,
    roster.go, CLI verbs, OwnerCorrection, OwnerInstructions binding,
    ogKindOwnerInstruction; (3) rewire boot scan, reconcile, injection,
    self-dev rewake, adapter/handler to the revision occurrence; (4) repair
    all citers and tests.

receipts: []
---

## What this mission is

M1 of the ordered mission list. The cleanup the owner asked for — but cleanup
that *fixes* Texture, not cleanup that deletes files and leaves the input path
broken. Owner input becomes a document revision event; the
tell/correct/roster/LifecycleOwnerInstruction side channel is deleted in the
same move.

## What this mission is not

- Not leaf-only: deleting `roster.go` alone leaves `/tell` as the live heresy.
- Not the full ontology cutover: wakes, sweeps, dual paths, Super, and the
  remaining ~30 wrong-path instances are M3.
- Not a migration: `tell` is a hallucination (a revision is just a diff);
  roster isn't a concept (it's a sub-RLM call). Nothing is preserved.

## The deletion surface

~28 files carry `LifecycleOwnerInstruction` or the tell/roster path. The
load-bearing ones:

- `cmd/choir/roster.go` + `roster_test.go`; `choir roster`, `texture tell`,
  `texture correct` in `cmd/choir/main.go`
- `internal/textureowner/texture_owner_instruction.go` (`/tell`, `/correct`)
- `internal/textureowner/texture_agent_revision.go` (`/revise`→`/tell` forward,
  unbound mailbox)
- `internal/textureowner/texture_controller.go` (boot scan, occurrence
  matching, `scheduleTextureWorkerWake`)
- `internal/textureowner/texture.go` (`scheduleTextureWorkerWake` call)
- `internal/textureowner/handler.go` (`wakeOwnerInstruction` wiring)
- `internal/textureowner/router.go` (`/tell`, `/correct` routes)
- `internal/store/lifecycle_owner_instruction.go` + test
- `internal/store/lifecycle.go` (`OwnerCorrection` → LOI creation)
- `internal/store/texture_turn.go` (`OwnerInstructions` binding, atomic consume)
- `internal/store/graph_store.go` (`ogKindOwnerInstruction`)
- `internal/types/owner_instruction.go`
- `internal/agentcore/super_controller.go` (pending-instruction reads)
- `internal/agentcore/selfdev_texture_join.go` (synthetic tell)
- `internal/agentcore/texture_lifecycle_api.go` (`TextureOwnerInstructionOccurrence`)
- `internal/actorruntime/adapter.go` (wake-time conversion)

## The replacement

The owner edit is the document revision. `store/lifecycle.go` already creates
the revision in the `OwnerCorrection` commit; `texture.go:1309` already emits
the revision event. The work is: make `/revise` commit the owner-authored
revision directly (no `/tell` forward), make the Texture actor's activation
derive from the document head, and delete the instruction channel.

## Supersession

This mission supersedes `choir-rlm-engineering-carrier-2026-09-11` as the sole
working entrypoint. The carrier's remaining scope (engineering desk on in-cell
carrier, R7/R9, canonical run acceptance) is M2 — it depends on this mission's
document channel existing.
