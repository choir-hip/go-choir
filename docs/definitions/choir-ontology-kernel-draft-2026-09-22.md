---
definition_version: 3
definition_id: choir-ontology-kernel-draft-2026-09-22
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-22T23:45:00Z'
  source:
    canonical_ref: main@a4cef7cf
    deploy_identity: unknown — capture at execution time
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-sub-rlm-document-channel-draft-2026-09-22
    disposition: recommended predecessor — the carrier proof (M2) freezes the
      contract this kernel must carry. Not a hard dependency: the kernel can
      be built against the ratified design directly, but landing it after M2
      reduces the risk of building a substrate for an unverified target.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (M3)
  observed_artifact:
    - claim: 'The wrong-path cluster is ~44 instances across five sub-classes:
        out-of-band input (deleted by M1), process-local continuations/wakes,
        dual-path duplicates, sweep/enumerate-state recovery, and non-event
        mutations. The ratified ontology cutover subsumes (b)-(e).'
      claim_scope: current
      evidence_ref: docs/problems/root-cause-wrong-path-cluster-2026-09-22.md
    - claim: 'The ratified design: delivered = in state head, fenced atomic
        commit, serial-per-actor, cast-only sub-RLMs, migration inside a write
        fence; pending = tape events minus actor state head; scheduled work
        uses not_before + dispatcher due-index; fate continuation is an
        executor actor.'
      claim_scope: current
      evidence_ref: docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md

finish:
  deliver: >-
    Choir's continuations are derivable: pending work is a tape projection,
    the dispatcher is the one consumer, a restart resumes a pending delivery
    from the tape, and the process-local wakes/sweeps/non-event mutations of
    the wrong-path cluster are deleted.
  artifact: >-
    Deployed staging state where the derivable-continuation kernel is live:
    delivered = in state head; fenced atomic commit; serial-per-actor
    dispatch; cast-only sub-RLMs; not_before due-index; pending-row migration
    inside a write fence; the wrong-path sub-classes (b) process-local
    continuations, (c) dual paths, (d) sweep recovery, (e) non-event mutations
    deleted. Recount of the cluster's remaining instances is zero for the
    in-scope classes.
  acceptance:
    - action: >-
        On staging, kill the runtime process mid-task; restart; observe the
        pending delivery resume from the tape with no sweep, watchdog, or
        process-local timer.
      proves: continuations are derivable, not process-local
      evidence_class: deployed proof
    - action: >-
        `grep -rn "time.AfterFunc\|context.AfterFunc\|go reconcile\|ListByState\|
        ListOpen\|AgentsWithBacklog\|Sweep(" internal/agentcore internal/actor
        internal/actorruntime internal/textureowner` returns no progress-causing
        call sites (only the dispatcher's internal projection).
      proves: the wrong-path sub-classes (b)/(d) are deleted
      evidence_class: code inspection
    - action: >-
        `grep -rn "UpdateRun\|UpdateWorkItem\|UpdateTrajectory\|
        PutBatchConditional" internal/agentcore internal/textureowner
        internal/actorruntime` returns no lifecycle-state mutation outside an
        event-backed reducer.
      proves: sub-class (e) non-event mutations are deleted
      evidence_class: code inspection
    - action: >-
        The migration of pending rows runs inside a write fence; no
        mixed-authority interval is observable.
      proves: migration is fenced
      evidence_class: deployed proof + code inspection
    - action: >-
        Agentic consensus panel reviews the landed candidate (frozen diff
        identity + deployed evidence) before `goal.complete`; a SEND BACK
        verdict blocks completion until the named gap is closed.
      proves: independent review gates acceptance, not just self-report
      evidence_class: consensus review
  rollback: >-
    git revert + redeploy. The write-fence migration is the risk: if it
    strands rows, restore from the pre-cutover checkpoint. The migration must
    be reversible or the rollback is a restore, not a revert.
  landing:
    required: true
    environment: staging
    required_receipts:
      - pushed_commit
      - ci
      - deploy
      - environment_identity
      - deployed_acceptance
      - consensus_review

value:
  better_means: >-
    Minimize the number of progress-causing mechanisms to exactly one (the
    dispatcher's tape-derived pending projection) while preserving every
    product behavior the wrong-path instances currently deliver.
  goodharting_would_be: >-
    Deleting the visible wrong-path files while leaving a second wake/sweep/
    mutation path that still causes progress — the cluster shrinks on paper
    but the substrate still has two ways to make something happen. Or: a
    "derivable" continuation that is actually a renamed process-local timer.

homotopy:
  realism_axis: >-
    Continuation authority: from "progress caused by scans, timers, detached
    goroutines, and side-table writes" (current) to "progress caused only by
    the dispatcher folding the tape" (target). Intermediate rungs must be
    write-fenced: a half-migrated system with two continuation authorities is
    worse than either endpoint.

boundaries:
  mutation_class: red
  authority_sources:
    - ratified ontology cutover (docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md)
    - ordered mission list (docs/world-wire-mission-stack-2026-09-22.md, M3)
    - wrong-path cluster assessment (docs/problems/root-cause-wrong-path-cluster-2026-09-22.md)
  must_preserve:
    - Event-backed reducer commands (QueueLifecycle*, SetCoSuperCapsuleDisposition,
      CancelCoSuperAssignment) are keepers — only their scan/timer callers die.
    - The dispatcher's pending projection is the one scan that may remain —
      as an internal projection, never an independent trigger.
    - Migration runs inside a write fence; no mixed-authority interval.
    - install_frontend_pointer, vmctl sweeper, sourcecycled are excluded
      (deferred/platform-shell, not this mission).
  excluded:
    - Owner-input path (M1 — already landed).
    - Desk crossings (M4 — migration targets, run after this kernel).
    - actuator=tools and Super substrate deletion (M4 — blocked on crossings).
    - Records mechanism (M5+).
    - Updating system (M9).
  protected_surfaces:
    - Texture canonical writes.
    - Lifecycle event append and projection.
    - Run acceptance.
    - The write fence around migration.
    - vmctl / VM lifecycle (deferred boundary — do not touch).

now:
  status: blocked_incomplete
  slice: draft — awaiting M1 and M2
  source_ref: main@a4cef7cf
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: m3-one-continuation-authority
    claim: >-
      The ~30 remaining wrong-path instances can be deleted behind one
      derivable-continuation kernel without losing any product behavior —
      every wake, sweep, and non-event mutation has a tape-derived
      replacement.
    test: >-
      For each cluster instance, name the tape event or fold that replaces
      it; if any instance has no derivable replacement, the claim weakens.
    edge: frame_lock — a wrong-path instance may carry a behavior the
      event-driven vocabulary cannot yet express (e.g. a host-control-plane
      action that legitimately lives outside the tape). The vmctl sweeper is
      the named candidate.
    delta_o: >-
      Classify each of the ~30 instances as "derivable" or "boundary
      exception" before deletion; the exceptions become explicit, not silent.
    scope_if_supported: >-
      One continuation authority; restart-safe derivable delivery; the
      substrate the records mechanism and harnessless self-dev need.
    status: proposed
    evidence_refs:
      - docs/problems/root-cause-wrong-path-cluster-2026-09-22.md
  decision:
    what: >-
      M3 is the ontology kernel: derivable continuations, fenced atomic
      commit, serial-per-actor, dispatcher as sole consumer. Desk crossings
      are M4, not this mission — the kernel lands first so the crossings have
      a target.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: ratified ontology cutover 2026-09-22
  belief:
    believed_state: >-
      The cluster is enumerated and the design is ratified. The risk is
      scope: ~30 instances is the largest single deletion in the stack.
    main_uncertainty: >-
      Whether the kernel can land as one write-fenced migration or must split
      into per-sub-class slices. Splitting risks a mixed-authority interval;
      one mission risks a stall.
    next_observation: >-
      M2's carrier proof: does the cast's continuation survive on the
      pre-cutover runtime, or does it expose a kernel gap M3 must close first?
  blocker_or_risk: >-
    Largest deletion in the stack; if it stalls, M4/M5/M7/M9 queue behind it.
    The hedge is M2 already compounding on the live process. Mega-mission
    risk: consider internal acceptance slices (kernel → per-class deletion)
    inside one throughline, not separate missions.
  next_action: >-
    Wait for M1/M2; then reconcile start state, classify each cluster
    instance as derivable or boundary-exception, and charter.

receipts: []
---

## What this mission is

M3 of the ordered mission list — the ontology kernel. The ratified
event-driven design lands: delivered = in state head, fenced atomic commit,
serial-per-actor, cast-only sub-RLMs, `not_before` due-index, migration inside
a write fence. The wrong-path cluster's remaining sub-classes (b)–(e) are
deleted.

## What this mission is not

- Not the desk crossings — those are M4, migration targets that need this
  kernel to exist first.
- Not the owner-input path — M1 already landed it.
- Not a rename — a scan that survives must be the dispatcher's internal
  projection, not an independent trigger.

## The deletion rule

Delete every `List*ByState` / `List*ByChannel` / `ListOpen*` / `ForEach*ByState`
scan used to *cause* progress, every `time.AfterFunc` / `context.AfterFunc` /
detached `go` continuation, every mailbox/owner-instruction/channel wake, and
every direct `Update*` that mutates lifecycle-owned state. Retain only: event
append, reducer projection, and the dispatcher's tape-derived pending/due-index.

## Draft status

Draft successor — blocked on M1/M2, not executable. Becomes the working
entrypoint only on promotion.
