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
    terminal fate that is recorded and visible; a restart re-derives open
    obligations from durable state without re-arming dead ones. An owner or
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
  source_ref: da28c9f0
  deploy_identity: 85befb1c
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
    Produce the obligation inventory table (read-only, green) and the
    reconciled O1 problem-doc status; then run agentic-consensus review of
    this file plus the inventory to move readiness to reviewed/executable.

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
