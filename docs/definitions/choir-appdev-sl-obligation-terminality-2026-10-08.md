---
definition_version: 4

# SL — Obligation terminality (operational invariant O1). Station of the
# supervised app-development metamission. Owner direction 2026-10-08: "good
# invariants. yes, proceed with mission next" — in reply to the proposal to
# give O1 its own mission. Absorbs SA slice 1 (Management storm convergence),
# which is an O1 instance (sa1-wake-outbox-rearm-storm).
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-08T23:45:00Z
  source:
    canonical_ref: main da28c9f0 (operational invariants register draft)
    deploy_identity: >-
      staging choir.news deployed 85befb1c (corpusd 85befb1c; checkpointd
      67729e07+; vmctl 0f7c58ba)
  worktrees:
    - path: /Users/wiz/go-choir skills/agentic-consensus/*
      status: dirty
      class: other_agent_wip
      owner: unknown (pre-existing)
      touch: forbidden
      recovery: leave in place
    - path: /Users/wiz/go-choir docs/deck/*
      status: dirty (untracked)
      class: user_wip
      owner: owner or other agent (appeared 2026-10-08)
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Operational invariants register (docs/operational-invariants-register-2026-10-08.md)
      O1: ~25 of 41 liveness problem docs are instances of one missing
      invariant — every durable obligation has one live driver and reaches a
      recorded terminal fate within bounded attempts. Enforcer: fragmented.
    - >-
      Durable obligation kinds observed in internal/store: lifecycle packets
      (worker-update objects: producer updates, controls, directives), the
      actor wake outbox (ActorWakeOutbox / ListUnprojectedActorWakes),
      work_items, run_continuations, coagent_mailboxes, inbox_deliveries,
      texture_controller_checkpoints. Drivers and expiry are per-kind
      (lifecycle_expire.go ExpireStaleLifecyclePacket, d94ce9ef paced drain
      and quarantine, ca8c8c18 delivered-page arm, …) across ~15k lines of
      lifecycle*.go.
    - >-
      Problem-doc status is stale: 36 of 85 "open" docs are cited by fix
      commits (docs/evidence/problem-doc-open-xref-2026-10-08.tsv). The true
      open set for O1 is unknown until reconciled.
    - >-
      SA slice 1 candidate d94ce9ef (wake-outbox storm convergence) pushed
      2026-10-06; its deployed acceptance (owner-guest restart with migration
      minted ~=0, storm absent) is still owed per the SA now card.

finish:
  deliver: >-
    No durable obligation on a Choir computer can loop, strand or vanish.
    Every obligation kind has exactly one driver, an attempt budget, and a
    terminal fate that is recorded and visible; a crash restart never
    resumes work (boot closes every open obligation as "interrupted by a
    restart"); a planned update restart, marked durably and consumed once
    at boot, resumes it (owner rule 2026-10-09). An owner or
    agent can ask the product API "what is owed and why is it not moving"
    and get an exact answer without SSH.
  artifact: >-
    (1) An obligation-kind registry in code: each durable obligation kind
    declares its store, its single driver, its terminal fates, its attempt
    budget and its visible dead-letter form, and the store refuses to create
    an obligation of an unregistered kind. (2) The existing per-kind drivers
    (expiry, drain, reconcile) connected to that registry — not a new poller
    or store (standing questions 4/5; August inventory "no new store or
    mailbox"). (3) A product-API obligation-fate surface per computer:
    counts and samples of non-terminal obligations by kind, attempts, last
    driver action, and any obligation without a live driver. (4) An alarm
    when an obligation exceeds its budget without a terminal fate.
  acceptance:
    - action: >-
        Fault-injection matrix on a fresh disposable (staging): for each
        obligation kind, (a) drop the initial dispatch, (b) kill the bound
        run mid-obligation, (c) inject a poison item that fails validation,
        (d) restart the guest mid-drain. Read the obligation-fate surface.
      proves: >-
        every injected obligation reaches a recorded terminal fate (consumed,
        discharged, refused or visible dead-letter) within its budget; no
        listing is poisoned by one bad item; zero obligations without a live
        driver; no re-arm after restart.
      evidence_class: deployed proof (disposable)
    - action: >-
        Owner computer: guest restart with its historical backlog, then 24h
        of normal use; read the obligation-fate surface and the wake counts.
      proves: >-
        the Management storm class is absent on the real backlog (absorbs SA
        slice 1 acceptance: migration minted ~=0, storm absent) and the
        un-driven count stays 0.
      evidence_class: deployed proof (owner computer)
    - action: >-
        Reconcile every O1-cluster problem doc: back-write status with the
        enforcing registry entry or the remaining gap.
      proves: O1 evidence set is closed or explicitly residual (register P1).
      evidence_class: document receipt
  rollback: >-
    Revert the station commits; the registry is additive and the per-kind
    drivers keep their prior behavior when the registry is absent. Never
    delete or rewrite durable obligations or tape events as rollback.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize (a) the number of non-terminal durable obligations without a
    live driver and (b) the maximum attempts any obligation consumes before a
    terminal fate, across all kinds, while preserving tape authority,
    desk authority boundaries (doctrine I1-I4) and idempotent delivery.
  goodharting_would_be: >-
    Driving the count to zero by expiring, filtering or deleting real work;
    a reconciler that "terminates" obligations by marking them failed with no
    visible dead-letter or owner-readable reason; another per-path patch that
    passes its own test while a sibling kind keeps looping; counting only the
    kinds the surface happens to list.

homotopy:
  realism_axis: >-
    One obligation kind with injected faults on a disposable -> all
    registered kinds under the full fault matrix on a disposable -> the
    owner computer's real historical backlog and live traffic.

boundaries:
  mutation_class: red
  authority_sources:
    - owner statement 2026-10-08 ("proceed with mission next")
    - docs/choir-doctrine.md (I1-I4, Live Heresies durable-obligation residue)
    - docs/operational-invariants-register-2026-10-08.md (O1, O2, O3, O4)
    - supervised app-development metamission spine
  must_preserve:
    - the canonical tape is never rewritten; obligations are never silently deleted
    - doctrine I1-I4 (Texture sole canonical writer; parent/child is not control)
    - no new store, mailbox or poller (extend existing lifecycle substrate)
    - idempotent delivery and existing receipts
    - continuations derive from durable state (O2), never process-local timers
  excluded:
    - SA memory/engine/store-split slices (continue in SA after this station)
    - semantic changes to desk prompts or verbs (O19 is separate)
    - Texture canonical revision semantics
  protected_surfaces:
    - lifecycle packet delivery/consumption (internal/store/lifecycle*.go)
    - actor wake outbox and its boot migration
    - work item and run continuation state
    - Texture canonical writes (read-only for this station)

now:
  status: working
  slice: >-
    define — obligation inventory: enumerate every durable obligation kind
    (store, creators, drivers, terminal states, attempt accounting, expiry,
    restart behavior); map each O1-cluster problem doc to its kind and
    mechanism; reconcile those docs' real status against commits.
  source_ref: 3d2247da
  deploy_identity: a72e2d32
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: C1-bridge
    claim: >-
      If every durable obligation kind has one registered driver, an attempt
      budget and a recorded visible terminal fate, the liveness problem class
      stops recurring: no new O1-instance problem docs after landing, and the
      owner computer runs unattended without obligation storms or strands.
    test: >-
      Fault matrix on a disposable plus 72h owner-computer observation; count
      new liveness problem docs filed against registered kinds.
    edge: missing_oracle
    delta_o: >-
      The obligation-fate surface itself — today no observer can list
      un-driven obligations across kinds, so strands are found only as
      downstream symptoms.
    scope_if_supported: registered obligation kinds on staging computers
    status: proposed
    evidence_refs: [docs/evidence/problem-doc-triage-2026-10-08.jsonl]
  decision:
    what: >-
      Registry-plus-existing-drivers over a new unified reconciler (prefer
      connecting and deleting over adding; CLAUDE.md convergence rules).
    kind: architecture
    status: proposal
    evidence_ref: docs/operational-invariants-register-2026-10-08.md
    owner_ratification_ref: pending (decided after the define-slice inventory)
  belief:
    believed_state: >-
      The ~25 O1 docs reduce to a small number of mechanisms — no per-kind
      attempt accounting, drivers keyed on process-local or run-local state,
      and terminal fates that are implicit rather than recorded.
    main_uncertainty: >-
      Whether the kinds can share one registry contract, or whether actor
      wakes and lifecycle packets need different fate semantics.
    next_observation: >-
      The define-slice inventory table: kind x {store, driver, terminal fates,
      attempt accounting, restart behavior} with problem docs mapped onto it.
  blocker_or_risk: >-
    Red surface; regressions here strand real work. Mitigated by
    disposable-first fault matrix and additive registry.
  next_action: >-
    Owner decisions 2026-10-09: no panel; the outbox is the single driver,
    event-driven (signal on write, timer only for the earliest due retry,
    no fixed poll); crash restart closes open work, planned update restart
    (durable marker consumed once at boot) resumes it, so the boot-time
    resumers are gated on the marker rather than deleted. Deploys restart
    only idle computers (separate CI change, problem doc
    deploy-restarts-busy-computers-2026-10-09). Next: SL problem doc, then
    marker, boot close-out, retry counts and the "what is owed" surface.
receipts: []
---

# SL — Obligation terminality

This station enforces operational invariant **O1** from the
[operational invariants register](../operational-invariants-register-2026-10-08.md),
with O2 (continuations derive from durable state), O3 (reject at commit, never
dead-letter silently) and O4 (one active run per agent) as direct
dependencies. It is the largest unowned invariant behind the liveness cluster
of problem docs.

The first slice is a read-only inventory, because the decision between
"registry plus existing drivers" and any larger restructuring must rest on
the real set of obligation kinds and mechanisms, not on problem-doc titles.

## Slice 1 — crash vs planned restart (2026-10-09, autonomous run)

Owner rule: a crash restart never resumes work; a planned update restart
may (AGENTS.md "Restarts End Work (Crash) Or Resume It").

**Today.** Boot passivates running runs (`passivateInterruptedActivations`)
and the actor boundary consumes pre-boot work-starting occurrences as
`interrupted_by_restart` — **for Texture only**
(`actorruntime/handler.go:69`, 4b9bf31d). Management, engineering and
research occurrences recorded before a crash still run after it (the wake
outbox re-delivers them). And a planned restart (a platform update or
self-development apply, which self-restarts the guest) interrupts
Texture's pending work exactly like a crash, which would break Gate 2's
"engineering reports back to Texture" step.

**Change.**
1. Planned-restart marker: `<RUNTIME_STORE_PATH>.planned-restart`
   (`{reason, target, requested_at}`), written and fsynced by the guest
   immediately before an intentional restart (the three
   `selfdevUpdater.Apply` call sites), removed if the apply returns an
   error. The runtime consumes it once at startup, before actors run; a
   marker older than 15 minutes is treated as absent (a stale marker must
   never turn a later crash into a "planned" boot).
2. The actor boundary interrupts pre-boot work-starting occurrences for
   **every desk** after a crash boot, and for none after a planned boot.
   Work-starting kinds: initial_dispatch, coagent_result,
   channel_message, owner_revision, lifecycle_work_assigned,
   delegated_assignment_spawn_deadline,
   fresh_mint_management_resume_deadline. Cancels and fail-closed
   deadlines (activation budget, cell terminal, engineering fate and
   progress, reactivated-management) and selfdev_materialization_retry
   still run: they close work or finish the planned step.
3. Boot logs the restart kind (`planned: <reason>` or `crash`).

**Evidence.** Focused tests at the actor boundary (crash boot interrupts a
pre-boot management occurrence; planned boot runs it; a stale marker is
ignored); staging: a disposable's guest restart logs `crash` and its
pre-boot management work is consumed as interrupted.
**Rollback.** Revert; the Texture-only rule returns.

## Slice 2 — event-driven outbox with a retry budget (2026-10-09)

**Today.** The projector sweeps the wake outbox every 500 ms whether or
not anything changed (`agentcore/runtime.go` startProjector): two store
queries a second while idle, on a guest store with one connection. A
dispatch that keeps failing is logged and retried every tick forever,
with no count and no visible fate (inventory Finding 2).

**Change.**
1. The store signals after a committed batch that wrote outbox rows (the
   two lifecycle commit points and the boot migration), never before
   commit, so a sweep cannot run ahead of the write it was told about.
2. The projector sweeps once at start, then waits for a signal, the
   earliest retry due, or a 60 s audit timer. A sweep that hit its
   per-pass budget runs again at once. The audit sweep logs "missed
   signal" if it finds a ready wake no signal announced, so a missed
   signal is a visible bug, not a silent strand.
3. A failing dispatch retries with backoff (1, 2, 4, 8, 16 s). After 5
   failures the wake is marked projected with fate `dispatch_exhausted`
   and its last error, durably, so the "what is owed" surface can list it.

**Failure modes to pin first.** Signal before commit (sweep misses the
row); a missed signal leaves a wake stranded (audit catches and logs);
budget overflow waits for the next signal (immediate re-sweep); a
persistent dispatch error loops forever (exhaustion); a transient error
exhausts too fast (backoff); exhaustion silently drops the wake (durable
fate and last error).
**Rollback.** Revert; the 500 ms poll returns.

## Slice 3 — the "what is owed" surface (2026-10-09)

**Today.** No observer can list what a computer owes without SSH and
store queries; strands are found only as downstream symptoms (inventory).

**Change.** `GET /api/runtime/obligations` (owner-authenticated, proxied
like every `/api/*` route) returns: how this boot began (planned with
reason, or crash/stop); wakes pending (by kind, oldest, how many are
backing off) and exhausted (count plus up to 20 samples with attempts and
last error); runs pending/running/passivated with the oldest of each;
open assigned work items with the oldest. Bounded reads only (indexed
lists, capped at 1000), so it is safe on the one-connection guest store.
Later slices add per-kind "no live driver" detection.

**Failure modes pinned first.** An exhausted wake is invisible; a pending,
backing-off wake is not counted; the surface answers without an owner.
