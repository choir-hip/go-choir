---
definition_version: 3
definition_id: choir-ontology-kernel-2026-09-24
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-24T15:45:00Z'
  source:
    canonical_ref: main@66981cef
    deploy_identity: staging https://choir.news — capture build.commit at execution
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: choir-sub-rlm-document-channel-2026-09-22 (M2)
    disposition: >-
      satisfied — M1 landed the document channel; M2 proved the carrier
      (sub-RLM cast on the document channel). R1 vocabulary cutover landed
      2026-09-24 (66981cef). The contract this kernel must carry is frozen.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (M3)
  observed_artifact:
    - claim: 'The wrong-path cluster is enumerated and classified: 51
        derivable instances, 3 deferred boundary exceptions (sourcecycled
        ticker, vmctl sweeper, frontend-current pointer), 1 keeper class.
        Every in-scope instance has a named tape-derived replacement.'
      claim_scope: current
      evidence_ref: docs/problems/root-cause-wrong-path-cluster-2026-09-22.md
    - claim: 'No kernel substrate exists yet: no dispatcher, no pending
        projection, no not_before due-index, no state-head. The kernel must
        be built, then the 51 instances migrated onto it.'
      claim_scope: current
      evidence_ref: this charter's classification (KClusterScout 2026-09-24)
    - claim: 'The ratified design: delivered = in state head, fenced atomic
        commit, serial-per-actor, cast-only sub-RLMs, migration inside a write
        fence; pending = tape events minus actor state head; scheduled work
        uses not_before + dispatcher due-index; fate continuation is an
        executor actor.'
      claim_scope: current
      evidence_ref: docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md

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
    - ratified ontology cutover (docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md)
    - ordered mission list (docs/world-wire-mission-stack-2026-09-22.md, M3)
    - wrong-path cluster assessment (docs/problems/root-cause-wrong-path-cluster-2026-09-22.md)
    - desk-RLM rectification plan (docs/desk-rlm-rectification-plan-2026-09-23.md, K)
  must_preserve:
    - Event-backed reducer commands (QueueLifecycle*, SetEngineeringCapsuleDisposition,
      CancelEngineeringAssignment) are keepers — only their scan/timer callers die.
    - The dispatcher's pending projection is the one scan that may remain —
      as an internal projection, never an independent trigger.
    - Migration runs inside a write fence; no mixed-authority interval.
    - install_frontend_pointer, vmctl sweeper, sourcecycled are excluded
      (deferred/platform-shell, not this mission).
  excluded:
    - Owner-input path (M1 — already landed).
    - Desk crossings (R3 — migration targets, run after this kernel).
    - actuator=tools and Management substrate deletion (R3 — blocked on crossings).
    - Records mechanism (R4+).
    - Updating system (M9).
  protected_surfaces:
    - Texture canonical writes.
    - Lifecycle event append and projection.
    - Run acceptance.
    - The write fence around migration.
    - vmctl / VM lifecycle (deferred boundary — do not touch).

now:
  status: working
  slice: substrate landed + boot/doom fixes deployed; deletion pass pending owner sweep ruling
  source_ref: main@66981cef
  deploy_identity: staging https://choir.news
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
      The remaining wrong-path instances can be deleted behind one
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
      Classification complete (2026-09-24): 51 derivable, 3 boundary
      exceptions (sourcecycled, vmctl, frontend-current — all deferred
      external control planes, not in-scope), 1 keeper class. The claim
      holds for the in-scope Choir runtime.
    scope_if_supported: >-
      One continuation authority; restart-safe derivable delivery; the
      substrate the records mechanism and harnessless self-dev need.
    status: supported
    evidence_refs:
      - docs/problems/root-cause-wrong-path-cluster-2026-09-22.md
      - KClusterScout classification 2026-09-24
  decision:
    what: >-
      K is the ontology kernel: derivable continuations, fenced atomic
      commit, serial-per-actor, dispatcher as sole consumer. Desk crossings
      are R3, not this mission — the kernel lands first so the crossings have
      a target.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: ratified ontology cutover 2026-09-22
  belief:
    believed_state: >-
      Kernel substrate landed and deployed. Owner ruling 2026-09-25: boot
      causes no work. Deletion pass in progress — the five restart-resumption
      boot phases (rewarm_lifecycle_activations, rewarm_persistent_management,
      sweep_passivated_spawned_work, sweep_open_work_item_actors,
      sweep_pending_update_actors) and their boot-only helpers are deleted;
      boot_recovery.go removed wholesale; reconcile_terminal_run_outcomes
      kept as state-repair (delivers committed outcomes, mints no wake beyond
      committed obligation). runtime.go progress-deadline context.AfterFunc
      deleted (durable activation_budget_deadline owns terminalization).
    main_uncertainty: >-
      Resolved — the (e) classifier ran: UpdateRun routes lifecycle-bound
      records through persistLifecycleRunWithEvent → ReplaceLifecycleActivation
      (event-backed); UpdateWorkItem*/UpdateTrajectory* reject lifecycle
      objects with ErrLifecycleAuthorityRequired. No non-event mutation of
      lifecycle-owned state exists. Residual edges: wire debounce durable
      conversion (in flight), wire_publication legacy records (canonical
      obligations on v0 non-lifecycle objects → migration, not deletion),
      runtime.go:1942 trajectory-identity stamp (missing command).
    next_observation: >-
      The (b)/(d) acceptance grep is near-clean: the only surviving
      AfterFunc is textureWakeAfter for the wire debounce, being converted
      to a durable not_before continuation by the in-flight change.
  blocker_or_risk: >-
    Resumption sweeps + progress AfterFunc + wire debounce process-local
    timer all deleted; wire debounce converted to durable
    wire_reconciler_publish_deadline (SQLite wire_publish_debounce_entries/
    state, atomic consume, ReplayEmptyUntilSupported). Stranding is
    intentional per owner ruling (boot starts no work): zombie RunRecords,
    lifecycle work version>1, delivered-control persistent Management,
    spawned-work mint — recorded in
    docs/problems/kernel-cutover-wake-gap-analysis-2026-09-24.md.
    Pipeline note: a docs-only head push with a cancelled earlier code push
    skips deploy-impact (non_docs is push-diff-scoped); deploy was forced
    via workflow_dispatch force_staging_deploy.
 next_action: >-
   Deletion pass landed and deployed (head 4daecd60, CI run 36110172609
   green). Held-VM resurrection regression closed: deploy startup called
   EnsureUniversalWirePlatformComputer -> ensureUniversalWirePlatformOwnership,
   which drove stopped/booting/failed ownership to startExistingVM with no
   IsHeld gate (internal/vmctl/platform_computer.go:104). Deployed as
   91c8fd9f (CI 36156273423); Node B now logs `refused: held` and the
   platform computer is durably stopped/held, zero firecracker, zero
   pre-genesis spam. Remaining: restart-resume deployed proof (kill a live
   guest autoputer mid-task, restart, observe tape-derived delivery),
   in-scope recount to zero, consensus gate. Residual surfaced: a vmctl
   restart can orphan a Firecracker proc for restart-loaded stopped
   ownership (not manager-tracked) — parked as a follow-up, not blocking.

receipts:
  - id: k-deletion-sweeps-timers-2026-09-25
    slice: restart-resumption sweep + progress-timer deletion
    status: landed (local; pending deploy proof)
    ref: this worktree (pending commit)
    body: >-
      Deleted the five restart-resumption boot phases in
      internal/agentcore/runtime.go:657-663 (rewarm_lifecycle_activations,
      rewarm_persistent_management, sweep_passivated_spawned_work,
      sweep_open_work_item_actors, sweep_pending_update_actors) plus their
      boot-only helper chains — internal/agentcore/boot_recovery.go removed
      wholesale; lifecycleActivationBindingsEligible and
      passivateInterruptedLifecycleActivation deleted from runtime.go;
      ResumeInterruptedPersistentManagementControlRun,
      resumeInterruptedPersistentManagementControlRunLocked,
      rewarmReactivatedManagementResumeWatchdogs, and errResumeWatchdogScanCap
      deleted from management_controller.go; reconcileTerminalRunOutcomes
      converted to no-return (dead woken map removed). Deleted the
      runtime.go:282 progress-deadline context.AfterFunc — the durable
      activation_budget_deadline (HandleActivationBudgetDeadline, idempotent)
      owns run terminalization. engineering_assignment_boot_test.go eligibility
      precondition removed (deleted helper). go build ./... and go vet
      ./internal/agentcore clean.
    digest: boot causes no work; committed obligations reach actors via the
      actor-wake outbox + dispatcher projection, not a boot wake.
---

## What this mission is

K of the rectification sequence — the ontology kernel. The ratified
event-driven design lands: delivered = in state head, fenced atomic commit,
serial-per-actor, cast-only sub-RLMs, `not_before` due-index, migration inside
a write fence. The wrong-path cluster's remaining sub-classes (b)–(e) are
deleted.

## What this mission is not

- Not the desk crossings — those are R3, migration targets that need this
  kernel to exist first.
- Not the owner-input path — M1 already landed it.
- Not a rename — a scan that survives must be the dispatcher's internal
  projection, not an independent trigger.
- Not delete-only — the kernel substrate does not exist; it must be built
  before the wrong-path instances can migrate onto it.

## The deletion rule

Delete every `List*ByState` / `List*ByChannel` / `ListOpen*` / `ForEach*ByState`
scan used to *cause* progress, every `time.AfterFunc` / `context.AfterFunc` /
detached `go` continuation, every mailbox/owner-instruction/channel wake, and
every direct `Update*` that mutates lifecycle-owned state. Retain only: event
append, reducer projection, and the dispatcher's tape-derived pending/due-index.

## Internal acceptance slices

The mission is too large for one commit. Land it as internal slices, each
independently verifiable, all inside the write fence:

1. **Kernel substrate** — dispatcher, pending projection (tape events minus
   actor state head), `not_before` due-index, fenced atomic commit. Land as a
   parallel authority behind a flag; prove the pending projection reproduces
   the current sweep's pending set on a replayed tape.
2. **Sub-class (b): process-local continuations** — `time.AfterFunc`,
   `context.AfterFunc`, detached `go` continuations, in-memory coalescer
   timers → durable events with `not_before` + due-index.
3. **Sub-class (d): sweep/enumerate-state recovery** — boot/recovery scans →
   dispatcher pending projection + recovery occurrences.
4. **Sub-class (e): non-event mutations** — direct `UpdateRun`/`UpdateWorkItem`/
   `UpdateTrajectory`/`PutBatchConditional` → event-backed reducer commands.
5. **Sub-class (c): dual paths** — legacy JSON capsule ops, DispatchWorkerUpdate,
   report_to_texture → single canonical path (R3 completes the desk crossings).
6. **Cutover + recount** — remove the flag, delete the wrong-path callers,
   recount the cluster to zero for in-scope classes.

## Boundary exceptions (deferred, not deleted)

- `cmd/sourcecycled` pre-RLM ticker — no Choir-tape vocabulary for source
  cadence; post-RLM re-engineering.
- `vmctl/ownership.go` host sweeper — no per-computer tape vocabulary for host
  resource ownership; deferred pending an authority decision.
- `.github/workflows/ci.yml` `frontend-current` pointer — host-global platform
  shell, not a computer tape; replace with immutable artifacts later.

## Status

Chartered 2026-09-24 on the completed classification. Executable with
`/goal docs/definitions/choir-ontology-kernel-2026-09-24.md`.
