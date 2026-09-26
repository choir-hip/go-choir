---
definition_version: 3
definition_id: choir-texture-ledger-consumer-2026-09-25
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-25T22:00:00Z'
  source:
    canonical_ref: main@77f22ced
    deploy_identity: staging https://choir.news build.commit=537fce04
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-carrier-substrate-integrity-2026-09-25
    disposition: satisfied — R2x landed 2026-09-25 (deadline wake armed at
      open+bind; both selection-path sweeps deleted; commit self-heals via
      recoverPartialActCommit). Deployed-cancel proof deferred — see its
      blocker_or_risk. The ledger-consumer mission may start.
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3a)
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R3a

finish:
  outcome: >-
    Desk-act resolution in the Texture owner reads commitment records, not
    worker_updates_* packets: ResolveTextureActorOccurrence /
    producerOccurrence* / evidenceSourceEntitiesFromWorkerUpdates / the
    ListAllPendingLifecycleUpdates desk-evidence branch resolve desk acts
    from choir.commitment_record, dual-read against the packet path for
    parity.
  artifact: >-
    A commitment-record-backed evidence resolver in textureowner that a
    reducer-minted Report/Resolve record flows through into Texture's
    evidence input, plus the dual-read comparison that flags divergence
    instead of silently preferring one source.
  acceptance:
    - action: >-
        A reducer-minted commitment record (Report/Resolve) appears in
        Texture's evidence input via the ledger resolver; the same packet
        carried by worker_updates_* produces the identical source entity
        set under dual-read.
      proves: desk acts resolve from the ledger; dual-read proves parity.
      evidence_class: local test + code inspection
    - action: >-
        Dual-read disagreement is surfaced as a logged/typed divergence,
        never silently swallowed in favor of either source.
      proves: the cutover is evidence-preserving, not lossy.
      evidence_class: local test
    - action: >-
        On staging: a desk act that lands a commitment record shows up in a
        bound Texture doc's evidence/source input without a worker_updates
        packet having been consumed first.
      proves: the ledger path is live on the deployed computer.
      evidence_class: deployed proof
  rollback: git revert + redeploy; dual-read keeps the packet path live, so
    revert restores the packet-only resolution.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Make the commitment record the single evidence substrate for desk acts,
    so texture evidence and the supervision ledger read the same durable
    truth instead of two parallel feeds.
  goodharting_would_be: >-
    A consumer that reads commitment records only when packets are absent
    (two authorities), or a dual-read that logs nothing and silently prefers
    one side — both look migrated and leave the split-brain.

homotopy:
  realism_axis: >-
    Ledger primacy: low = worker_updates_* is the evidence input and records
    are a parallel write (current); high = records are the read substrate and
    packets are the deprecated twin. This mission moves reads to the ledger
    under dual-read before any packet-path deletion (deletion is R3d's).

boundaries:
  mutation_class: red
  authority_sources: [owner, doctrine, desk-rlm-rectification-plan-2026-09-23 §11]
  must_preserve:
    - worker_updates_* packet path stays live under dual-read until R3d deletes it
    - D4 processor/reconciler resolution is unaffected
    - texture evidence selector/projection contract (shape of textureSourceEntity)
    - replay idempotency — a re-derived source entity is content-keyed
  excluded:
    - desk-cell carrier for non-engineering desks (R3b)
    - management cast path (R3c) / texture authoring (R3d)
    - deleting the worker_updates_* write path (R3d owns that deletion)
    - scores, materiality, context packs (R4)
  protected_surfaces:
    - canonical commitment record (choir.commitment_record) read path
    - texture evidence input + source-entity contract
    - worker_updates_* consumer surface under dual-read

now:
  status: working
  slice: >-
    chartered. First: map every evidence/occurrence resolution surface that
    reads worker_updates_*/CoagentSourcePacket into textureSourceEntity, and
    the commitment-record fields that carry the same act — then wire the
    ledger resolver behind dual-read.
  source_ref: main@77f22ced
  deploy_identity: staging https://choir.news build.commit=537fce04
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: ledger-subsumes-packets
    claim: >-
      Every desk act that currently reaches Texture's evidence input as a
      worker_updates_* packet is also minted as a commitment record, so the
      evidence resolver can read the ledger and produce the identical source
      entity set — the packet becomes a redundant twin.
    test: >-
      The dual-read parity acceptance above plus a fixture sweep asserting
      every packet-shaped act has a commitment record counterpart.
    edge: missing_oracle — a packet-only act with no commitment record
      (e.g. a legacy non-desk update) would diverge the dual-read; the
      surface map must enumerate any packet the ledger cannot express.
    delta_o: >-
      A census of the packet→record mapping: which CoagentSourcePacket kinds
      have a commitment_record, which do not, and the smallest record
      fields needed to reconstruct each source entity.
    scope_if_supported: all desk evidence resolution in textureowner
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3a)
  decision:
    what: >-
      Keep dual-read (not a flag-day): the ledger resolver runs alongside
      worker_updates_* and diffs before any deletion, so R3a lands as an
      additive consumer, not a cutover.
    kind: operational
    status: settled
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md §11
    owner_ratification_ref: owner 2026-09-25 — ratified the spine; R3a is
      the next station under the meta-goal
  belief:
    believed_state: >-
      Commitment records are minted at cell commit (R2x); the packet path
      is the redundant twin.
    census_findings: >-
      Two surfaces. (1) Evidence materialization: packet.Sources ->
      textureSourceEntity; the record preserves Sources only in
      packet-bodied Report hypotheses, AND TargetAgentID/desk scoping lives
      on the dropped envelope — so even a Report-only ledger read cannot
      faithfully scope to the desk. (2) Occurrence/authority: needs typed
      lifecycle identity the record lacks. Both gaps point the same way:
      the record needs a typed source+target binding to be a faithful
      evidence substrate.
    main_uncertainty: >-
      Whether to extend the commitment record with typed source/evidence
      fields (touches the R5a vocabulary freeze) or keep dual-read parity
      limited to packet-bodied Reports and leave the authority path on
      worker_updates_* until R5a/R3d.
    next_observation: >-
      Consensus ruling on record-schema extension vs. parity-limited
      dual-read, then wire the evidence-seam resolver.
  blocker_or_risk: >-
    Substrate fork, deferred per No Blocking Asks: run agentic-consensus on
    (a) extend commitment_record with typed source/evidence fields now vs.
    (b) dual-read parity over packet-bodied Reports only, authority path
    stays on packets until R5a/R3d. Do not silently narrow scope.
  next_action: >-
    Run agentic-consensus on the record-schema fork; wire
    evidenceSourceEntitiesAndRejectionsFromPendingUpdates to dual-read.

receipts: []
---

# R3a — Texture Ledger Consumer (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11:
`ResolveTextureActorOccurrence` + `producerOccurrence*` +
`evidenceSourceEntitiesFromWorkerUpdates` + the `ListAllPendingLifecycleUpdates`
desk-evidence branch resolve desk acts from `choir.commitment_record`;
dual-read against `worker_updates_*`. Processor/reconciler (D4) unaffected.
Packet-path deletion is R3d's, not this station's.
