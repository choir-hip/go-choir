---
definition_version: 3
definition_id: choir-vocab-decoders-seed-freeze-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T23:45:00Z'
  source:
    canonical_ref: main@35fdf243
    deploy_identity: staging https://choir.news build.commit=44e4169e
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-platform-update-push-restore-2026-09-26
    disposition: >-
      M9a landed+deployed 2026-09-26 (44e4169e, ci 36277989883). The
      platform-signed update push + pinned-head restore is proven on
      staging: fresh computer accepted the signed offer, applied under
      verification, minted a platform_follow checkpoint, promoted the
      route, and restored to the pinned head via tape_reconstruct.
      Five substrate strands peeled doc-first.
    evidence_ref: docs/evidence/choir-platform-update-push-restore-deployed-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R5a
  owner_directive: >-
    Plan §11.2 R5a (off spine, must precede M11): per-family frozen
    decoders + V1→V2 profile normalization + explicit
    keep-choir:co-super-assignment:v3 decision. Proof: a pre-migration
    tape folds identically. Plan §11.4 ruling: R5a gates M11; R5b (the
    rename) is deferred indefinitely.
  census_ref: >-
    Scout census 2026-09-26: frozen decode machinery today covers
    profile tokens only (computerevent/decode.go roots,
    vocabmigrate/migrate.go+fence.go, store SQL+OG row migration). All
    other stratum-B families — lifecycle command/event kinds, lifecycle
    JSON fields, OG object/edge kinds, schema strings, digest domains,
    SQL identifiers, CLI schemas — have no frozen decoder. Two live
    normalization gaps: platform/checkpoints.go:228 and
    agentcore/self_development_decision_binding.go compare decoded
    historic ActorProfile against live V2 constants, so any
    pre-cutover event carrying a V1 profile spelling fails the
    comparison (latent heresy, this station repairs it).

finish:
  outcome: >-
    Every stratum-B durable-vocabulary family has a frozen decoder table
    and a decode function routed at its real interpretation boundary, so
    pre-migration bytes keep meaning the same thing no matter what the
    live constants are later renamed to. One named normalization point
    joins decoded historic profile tokens to live constants; the two
    known gap sites route through it. The v3 identity seed is pinned
    keep-forever with every neighboring seed enumerated. A recorded
    pre-migration tape fixture folds to identical state under the
    frozen-decoder path and the pre-cutover path.
  artifact: >-
    Per-family frozen decoder registries + decode functions (lifecycle
    command/event kinds, lifecycle JSON fields, OG kinds, schema/digest
    domains, identity seeds) in the frozen-vocabulary package; a single
    NormalizeHistoricProfile normalization point consumed by the
    checkpoints witness scan and the selfdev decision binding; a
    coverage test proving every census-listed V1 spelling is in its
    family table; a tape-fold equivalence test over a recorded
    pre-migration fixture. No renames: every decoder maps today's V1
    spellings to today's semantics; nothing durable changes bytes.
  acceptance:
    - action: >-
        Enumerate the stratum-B families and route a frozen decoder at
        each family's interpretation boundary: lifecycle command/event
        kind strings decoded in store/lifecycle.go, OG object/edge kind
        strings where they drive behavior (identity formulas, kind
        dispatch, schema writes), schema strings at their parse/validate
        sites, and the identity-seed literals pinned as frozen. A
        coverage test asserts every V1 spelling the census listed exists
        in its family table (fails if a new V1 spelling appears outside
        a frozen table).
      proves: per-family frozen decoders exist and guard the vocabulary
        the way the profile tables guard profiles — a later R5b rename
        can never orphan history.
      evidence_class: local test + boundary census
    - action: >-
        A single named normalization point joins decoded historic
        ActorProfile tokens to live constants; platform/checkpoints.go
        (selfdev verifier witness scan) and
        agentcore/self_development_decision_binding.go route through it.
        A historic event carrying actor_profile "co-super" or "super"
        passes the comparisons that today require the V2 spelling —
        verified by test on both sites.
      proves: the stratum-C normalization point exists and repairs the
        latent historic/live mismatch at both known gap sites.
      evidence_class: local test (latent-bug reproduction, then fix)
    - action: >-
        Record the keep-choir:co-super-assignment:v3 decision: the seed
        literal is pinned by a constant + test at
        engineering_assignment_runtime.go:118-121 and every neighboring
        seed in the same file (request, decision, scope-digest seeds at
        ~211-213/247-249/382-404) is enumerated and frozen in the seeds
        family table. Renaming any of them would mint different IDs for
        replayed history.
      proves: identity seeds are frozen protocol forever; the keep-v3
        decision is executable, not just prose.
      evidence_class: local test + this goal file
    - action: >-
        Fold a recorded pre-migration tape fixture — durable bytes
        carrying today's V1 spellings across the families (lifecycle
        kinds, OG kinds, schema strings, V1 actor_profile) — through the
        routed frozen-decoder path and through direct decode, and prove
        identical folds (same decoded semantic values, same witness
        digest on replay). Reuse the replay_completeness /
        projectionbase rebuild harness if a recorded tape exists;
        otherwise seed the fixture in-test.
      proves: a pre-migration tape folds identically — M11's proof tape
        will be readable under frozen decoders.
      evidence_class: local test
    - action: >-
        Boundary check before implementation: confirm or refute that
        staging carries any durable V1-spelled rows whose decode could
        change (post-migration stores are already fenced V2; the frozen
        decoders are decode-side only). Deployed acceptance is scoped to
        health + identity at the landed head.
      proves: R2-boundary discipline — this station is decode-side and
        provably cannot alter stored bytes.
      evidence_class: boundary probe + deployed health/identity
  rollback: >-
    git revert + redeploy. Decoders are additive + identity-preserving;
    the normalization point only widens acceptance of V1 spellings at
    two comparison sites; no stored bytes are written or rewritten. The
    v3 seed pin is a constant + test; reverting restores the literal
    inline call.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    M11's proof tape must fold under frozen decoders — a gate whose
    receipts can't be re-read deterministically after a rename is not a
    proof. Freezing per-family tables now, while the vocabulary is still
    V1-everywhere, costs nothing in behavior and buys the property that
    every durable byte written before R5b decodes the same forever. The
    normalization point repairs a real latent bug: a pre-cutover
    selfdev verification event spelled "co-super" would fail today's
    checkpoint witness scan on replay.
  goodharting_would_be: >-
    Writing the frozen tables but never routing a decode boundary
    through them (dead-code decoders), declaring the normalization point
    by comment without changing the two gap sites, pinning the seed in a
    doc but not in code, or proving fold identity on a fixture that
    carries no V1 spellings.
  falsifiers:
    - 'Falsified: a V1-spelled durable family exists with no frozen
      decoder table — the census coverage test misses a family.'
    - 'Falsified: the normalization point exists but a gap site still
      compares raw decoded ActorProfile to a live constant — the latent
      bug survives under a new name.'
    - 'Falsified: the tape-fold test only exercises V2 spellings — the
      fixture proves nothing about pre-migration bytes.'
    - 'Falsified: the v3 seed is renamed or re-derived — replayed
      history mints new assignment IDs.'

now:
  status: working
  slice: >-
    Chartered 2026-09-26 against census findings. First cut: seed the
    family tables from the census list, write the normalization point,
    then the fold fixture.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: frozen-decoders-fold-identical
    claim: >-
      Routing every stratum-B durable-vocabulary family through frozen
      per-family decoders — while every live spelling is still V1 —
      produces zero observable behavior change and makes every
      pre-migration byte decode identically forever; the explicit V1→V2
      normalization point repairs the two known historic/live compare
      sites rather than masking them.
    test: >-
      Coverage test: every census-listed V1 spelling is in a family
      table. Gap-site test: V1-spelled historic events pass both
      comparisons through the normalization point. Fold test:
      pre-migration fixture bytes decode to identical semantic values
      and identical replay witness under routed and direct paths.
      Deployed: health + identity at landed head.
    delta_o: >-
      With R5a landed the proof-tape precondition for M11 holds — its
      receipts stay readable under frozen decoders regardless of any
      later R5b rename. The spine then has exactly one station left:
      the self-development gate.
    scope_if_supported: last precondition before M11 discharged
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §4 (strata), §11.2 (R5a), §11.4 (R5 split ruling)
      - docs/evidence/choir-rlm-v2-mapping-2026-09-10.md §1 §3 (frozen maps)
  belief:
    believed_state: >-
      Frozen vocabulary machinery exists for profile tokens only; all
      other durable families read raw. Two production comparisons
      against live profile constants silently require V2 spellings on
      historic bytes. The v3 seed and its neighbors are inline string
      literals — pinned nowhere. R5b is deferred indefinitely, so R5a's
      job is read-side freeze only: tables + routed decode + one
      normalization point + fold proof.
    next_observation: >-
      The fold fixture: whether a checked-in recorded tape exists that
      carries V1 spellings (projectionbase/rlm-replay goldens are
      operation-level), or whether the fixture must be seeded in-test
      across the families.
  blocker_or_risk: >-
    Routing granularity risk: routing every raw string read through a
    decoder would be churn without benefit; the honest boundary is where
    a persisted token drives interpretation (kind dispatch, schema
    validation, digest mint, identity derivation, profile comparison).
    Storage-opaque reads stay raw and are named in the freeze manifest.
    Fixture risk: if no recorded V1 tape exists, the seeded fixture must
    still carry real V1 spellings — production writes them today.
  next_action: >-
    (1) Seeded fixture probe + family-table authoring in the
    frozen-vocabulary package; (2) NormalizeHistoricProfile + route the
    two gap sites; (3) seed pin const + test; (4) fold-equivalence +
    coverage tests; (5) landing loop.

receipts:
  - "charter: R5a = per-family frozen decoders + V1→V2 normalization +
    keep-v3 seed decision + pre-migration fold proof (plan §11.2; R5
    split ruling §11.4 — R5a gates M11, R5b deferred indefinitely).
    Census (scout, 2026-09-26): profile tokens already frozen
    (computerevent/decode.go, vocabmigrate, store SQL+OG migration,
    check-decode-roots.sh); lifecycle kinds, OG kinds, schema/digest
    strings, JSON fields, SQL identifiers, CLI schemas, seeds have no
    frozen decoder. Latent heresy discovered: platform/checkpoints.go
    :228 and agentcore/self_development_decision_binding.go compare
    decoded historic ActorProfile to live V2 constants with no
    normalization — pre-cutover V1-spelled verification/decision events
    would fail on replay. Boundaries: no renames, no durable writes;
    decode-side + compare-site changes only; rollback is revert."
---

# R5a — Vocabulary Decoders + Seed Freeze (last station before M11)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11.2
(R5a, off spine, must precede M11): per-family frozen decoders, the V1→V2 normalization
point, and the keep-`choir:co-super-assignment:v3` decision. R5b (the rename itself) is
deferred indefinitely per §11.4 — this station freezes the *read* side so a pre-migration
tape folds identically whether or not a rename ever ships.

## What this station is NOT

- No durable bytes are rewritten. SQL row migration (`MigrateAndFenceServingVocabulary`)
  is already live and unchanged.
- No renames. Every frozen table maps today's V1 spellings to today's semantics.
- No second normalization points — one named function, all compare sites route through it.
