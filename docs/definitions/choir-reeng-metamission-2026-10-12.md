---
definition_version: 4

# Re-engineering: honest desks, work as records, async search, Texture, one
# document model. Owner 2026-10-12, after a line-by-line review of the
# master report: agreement before building ("extremely high stakes decision
# making"), then an autonomous run that makes Autopaper live. Decisions:
# docs/re-engineering-decisions-2026-10-12.md. Studies:
# docs/reports/re-engineering-studies-2026-10-12/.
readiness: drafted

review:
  reviewer: owner (decisions agreed 2026-10-12; "Yes, sounds good. Proceed.")
  frozen_ref: fbb58c23
  verdict: accept (design direction); station files await file review
  evidence_ref: docs/re-engineering-decisions-2026-10-12.md

metamission:
  stations:
    - id: R1
      path: docs/definitions/choir-reeng-r1-truthful-prompts-and-deletions-2026-10-12.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: R2
      path: docs/definitions/choir-reeng-r2-restart-rule-2026-10-12.md
      readiness: drafted
      status: pending
      depends_on: [R1]
    - id: R3
      path: docs/definitions/choir-reeng-r3-records-as-work-2026-10-12.md
      readiness: drafted
      status: pending
      depends_on: [R1, R2]
    - id: R4
      path: docs/definitions/choir-reeng-r4-async-search-and-research-2026-10-12.md
      readiness: drafted
      status: pending (slice a, async search, may start after R1; slice b needs R3)
      depends_on: [R1, R3]
    - id: R5
      path: docs/definitions/choir-reeng-r5-texture-2026-10-12.md
      readiness: drafted
      status: pending
      depends_on: [R1, R3]
    - id: R6
      path: docs/definitions/choir-reeng-r6-one-document-model-2026-10-12.md
      readiness: drafted
      status: pending (quote verification and the capture side may start after R1; transclusion needs R5)
      depends_on: [R1]
    - id: R7
      path: none (management translates with Jev pass-through; path B, the runtime Jev judge and the self-development state machine dissolved; study G rows 7-10)
      readiness: intent
      status: pending
      depends_on: [R3]

start:
  captured_at: 2026-10-12T18:10:00Z
  source:
    canonical_ref: main fbb58c23
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Six read-only studies found one failure everywhere: prompts that
      describe a system that no longer exists, and runtime rules patching
      the gap. Nearly every fix is a deletion.
    - >-
      Two live problems contradict standing rules: parallel choir.* calls
      cross-wire, and a crash restart leaves work open and re-casts
      engineering (docs/problems/ 2026-10-12).
    - >-
      The protocol's P2d2 is held on the rule that one Texture run binds one
      work item; Autopaper W2 waits on it.

finish:
  deliver: >-
    Desks that are told the truth, own their work, and report as their
    understanding changes; work and its status read from records; search
    that never waits on the slowest provider; a Texture that writes only
    what changed; sources and stories in one document model. Then
    Autopaper W2 and W3 run on that base and Autopaper goes live.
  artifact: R1-R6 complete on staging with deployed acceptance; R7 authored
  acceptance:
    - action: each station's acceptance on staging (choir.news)
      proves: the station's claim on the product path
      evidence_class: deployed proof
    - action: one AI news demo after each station, same script, compared with R1's baseline
      proves: the stack moves the product, not only the code
      evidence_class: deployed proof
  rollback: per station, in reverse order
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize grounded, current Textures per unit of inference and owner
    attention, while preserving Texture as sole writer, the restart rule,
    authority boundaries, and one authority per fact.
  goodharting_would_be: >-
    Deleting code without the demo moving; new runtime machinery that
    decides what a model should decide; measuring cells or reports instead
    of what reaches a reader.

homotopy:
  realism_axis: >-
    honest prompts and deletions -> restart rule -> records as work ->
    async search and research -> Texture -> one document model ->
    Autopaper live

boundaries:
  mutation_class: red (stations R2-R6 touch run lifecycle, the evidence ledger, gateway calls, Texture canonical writes, publication)
  authority_sources:
    - owner 2026-10-12 decisions (docs/re-engineering-decisions-2026-10-12.md)
  must_preserve:
    - Texture is the only writer of a Texture
    - research has read-only authority toward the world
    - a crash restart never resumes work; a planned restart may, once
    - no budgets, round caps, runtime research job or runtime scoring are added
  excluded:
    - the model comparison before R5's re-engineering lands
    - circuit breakers before a pathological loop is observed
    - signing keys per computer, desk or sub-RLM (deferred)
  protected_surfaces: [see each station]

now:
  status: drafted
  slice: >-
    Decisions agreed and recorded (fbb58c23). Station files R1-R6 authored
    as drafts; R7 is intent. The protocol metamission's P2d2, P3 and P4 are
    carried by R3 and R4 here; Autopaper W2 depends on R3 and R4.
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: RE-1
    claim: >-
      The desks underperform mainly because of false prompts, runtime caps
      and the work-item substrate, not model capability; fixing those in
      order makes the AI news demo produce grounded Textures faster and
      lets Autopaper go live without a model change.
    test: the same AI news demo after each station against R1's baseline
    edge: frame_lock
    delta_o: the demo script and its metrics are fixed in R1 and exported with the guest trace
    scope_if_supported: AI news and Autopaper on staging test computers
    status: proposed
    evidence_refs: [docs/reports/re-engineering-studies-2026-10-12/README.md]
  decision: {what: "order of work R1 -> R6 with R4a and R6's early slices in parallel", kind: sequencing, status: settled (owner 2026-10-12), evidence_ref: docs/re-engineering-decisions-2026-10-12.md §10}
  belief:
    believed_state: the studies' code claims hold at fbb58c23
    main_uncertainty: whether R3's fold can replace work items without a long dual-running period
    next_observation: R1's file review, then R1's grep inventory
  blocker_or_risk: none for R1; R2-R6 are red and need full ceremony
  next_action: agentic-consensus file review of R1, then execute R1

receipts: []
---

# Re-engineering metamission

The studies of 2026-10-12 found that the desks are mostly being told
untrue things and held back by runtime rules, and that work items carry
six unrelated jobs and a cluster of closure bugs. This metamission fixes
that in order, with each station measured by the same AI news demo, and
then hands a sound base to Autopaper's living Textures and reader.

1. **R1, truthful prompts and deletions** (orange).
2. **R2, the restart rule in both directions** (red).
3. **R3, records as work**, with P2d2 as its Texture-inbox stage (red).
4. **R4, async search and the research desk** (orange and red).
5. **R5, Texture re-engineered**, then the model comparison (red).
6. **R6, one document model** for sources and stories (red).
7. **R7, routing** (intent): management translates, Jev passes through or
   management reasons, path B and self-development scaffolding dissolve.
