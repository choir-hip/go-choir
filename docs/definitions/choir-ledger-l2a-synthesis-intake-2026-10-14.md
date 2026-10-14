---
definition_version: 4

# Station L2a of the ledger metamission
# (docs/definitions/choir-ledger-metamission-2026-10-13.md): synthesis
# intake. Owner 2026-10-14, in session: "intake redesign, very important.
# until then we are just copying. after that we are synthesizing." And:
# "we should be compounding. Not turning captures into copied articles,
# but building an object graph, and synthesizing articles from the
# convergence and conflicts between sources. Sense making and tension
# centering. ... A disaster would be 3 separate articles from choir about
# 3 articles published about the same story in 3 different languages."
# Rulings and the product principle:
# docs/reports/owner-roadmap-rulings-2026-10-14.md. This station is the
# first slice of L2 (bounded attention), pulled ahead of C1e B1-B5 by that
# ruling. L2's digest-per-window and burst bounds remain for L2b.
readiness: drafted

review:
  reviewer: none yet (agentic-consensus convergent round on this file is the first action)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-14T00:35:00Z
  source:
    canonical_ref: main 6c6bc1aa (docs commits after it)
    deploy_identity: staging 6c6bc1aa (CI 38488951037, deployed 00:24:27 UTC)
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled` at the repo root)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      The live paper computer (computer-1355b83bced1613eb048ee28d66688a8,
      platform tier, 14 sources mostly English wires and tech feeds) holds
      50 stories. 47 opened from one capture, 2 from two, 1 from four.
      About 11 management Notes fed one more capture each into 8 existing
      stories, mostly follow-ups from the same source. No story synthesizes
      across sources or languages (counts read 2026-10-14 00:25 UTC through
      the owner's API).
    - >-
      Management's prompt (internal/promptstore/defaults/management.yaml,
      "Living stories") already says to cluster captures by event, never
      open a story twice for one event, and feed existing stories by Note.
      Its only view of the library is choir.ProductAPI GET
      /api/texture/documents (doc ids and titles). It has no view of what
      each story says, which sources and languages it has, or when it last
      changed.
    - >-
      The runtime has a semantic dedup (internal/agentcore/qdrant_dedup.go)
      that drops near-duplicate items before projection. It is listed for
      retirement in the ledger metamission (L0's parallel items). Whether
      it is active on staging, and whether it drops cross-language coverage
      of one event, is unknown.
    - >-
      Model cost is unpriced on /api/costs; research input is dominated by
      re-sent cell output (station LR, R5).

finish:
  deliver: >-
    Autopaper writes one story per real-world event, synthesized from the
    convergence and the conflicts among its sources, across languages; a
    new capture updates the story it bears on, and the new version says
    what changed.
  artifact: >-
    A deployed intake in which:
    - management reads a story library (each story's headline, current
      lede, sources and languages, last change) with every captures record;
    - management's policy routes each capture to open, feed or decline,
      with one story per event across languages;
    - Texture's story contract writes from convergence and conflict and
      states each revision's change;
    - no runtime step drops a capture a desk has not seen.
    The run receipts are a fixture run and the live paper measurement.
  acceptance:
    - action: >-
        A fixture run on a minted test computer through the real poller.
        The fixture is a set of at least 8 real-world events, each covered
        by 2-3 captures in at least 2 languages, plus unrelated singletons.
      proves: >-
        Each multilingual event yields one story, not one per capture or
        per language, and that story's published version cites captures in
        at least 2 languages. Every capture has a disposition. Singletons
        still open or decline.
      evidence_class: deployed proof
    - action: >-
        The live paper computer, measured over at least 6 hours after
        deploy, with the same count method as the start receipt.
      proves: >-
        Against the 47/50 baseline, the share of stories whose current
        version cites two or more independent sources rises, feeds into
        existing stories rise, and publication continues. Measured cost
        per published version is reported, not capped.
      evidence_class: deployed proof
  rollback: git revert of the slice commits (prompt defaults and the library view); the paper's existing stories are untouched
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize the share of published story versions that synthesize two or
    more independent sources (and two or more languages where coverage
    exists), and the share of new captures that update an existing story,
    while every capture still gets a disposition and publication does not
    stall.
  goodharting_would_be: >-
    Citing a second source that adds nothing; merging unrelated events to
    raise the multi-source share; stories that list sources without
    reconciling them; declining captures to avoid opening stories;
    counting same-outlet follow-ups as independent sources.

homotopy:
  realism_axis: >-
    unit tests of the library view -> a fixture of multilingual events on a
    minted test computer -> the live paper on its 14 sources -> the live
    paper with a few non-English general-news feeds -> the long tail
    (after the architecture work, ruling 2)

boundaries:
  mutation_class: orange (management and Texture prompt defaults, a new read view for management; no change to Texture canonical writes, run acceptance or provider routing)
  authority_sources:
    - owner 2026-10-14 in session (synthesis principle; "intake redesign, very important")
    - docs/reports/owner-roadmap-rulings-2026-10-14.md
    - docs/definitions/choir-ledger-metamission-2026-10-13.md (L2 note)
  must_preserve:
    - Texture is the sole writer of canonical revisions; management never writes a story
    - every captures record gets an answer (a story, a feed or a decline)
    - the runtime never filters a capture out of a desk's sight
    - publication on the live paper does not stall; the paper computer stays up
    - one shared-runtime cutover at a time (C1e B1 waits for this slice to land)
    - spend is tracked, never capped
  excluded:
    - the per-window digest, triage model and 10x burst bound (L2b)
    - adding long-tail or Telegram/RSSHub sources (after the architecture work)
    - hypothesis settlement (L3)
  protected_surfaces: [management desk prompt and records surface, Texture desk prompt, the paper computer's schedules]
  admissible_evidence: deployed proof on staging (a minted test computer and the live paper); unit tests shape the library view only

now:
  status: working
  slice: S0, reconcile and design
  source_ref: 6c6bc1aa
  deploy_identity: staging 6c6bc1aa
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: L2a-1
    claim: >-
      The copying comes from what management and Texture can see and are
      told, not from missing mechanism. Feeding by Note and casting into an
      existing texture inbox both exist. Given a library view (what each
      story says, its sources and languages, its last change) and a
      synthesis contract, the desks route multilingual coverage of one
      event into one story and write it from convergence and conflict,
      with no new record kind.
    test: the fixture run's stories-per-event and languages-per-story, then the live paper's multi-source share against 47/50
    edge: frame_lock
    delta_o: a labelled fixture with known event identity across languages, so merging is scored against truth rather than judged by the same desk
    scope_if_supported: Autopaper's intake on every source class
    status: proposed
    evidence_refs: [docs/reports/owner-roadmap-rulings-2026-10-14.md]
  decision: {what: "L2a ahead of C1e B1's build; B1's design rounds continue as docs", kind: operational, status: proposal, evidence_ref: docs/reports/owner-roadmap-rulings-2026-10-14.md, owner_ratification_ref: "owner 2026-10-14: intake redesign, very important"}
  belief:
    believed_state: >-
      The paper copies, one capture per story; the mechanisms to feed and
      to synthesize exist but are unseen and unrequired. Two S0
      observations, 2026-10-14 00:45 UTC: semantic dedup is dead code
      (dedupSourceItemsWithTimeout and dedupSourceItemsSemantically have no
      production caller), so no runtime step drops coverage today, and S1
      deletes qdrant_dedup.go. The registry already holds vertical-zero
      feeds in other languages: general news from DW (de, ar, ru), France 24
      (fr, es, ar), RFI, El País, El Mundo, Euronews, Folha and Agência
      Brasil; tech from heise, Golem, ITmedia, Nikkei xTECH, 36kr, Numerama
      and Xataka. So the fixture needs no hosted feed. A minted test computer
      polls English and non-English world and tech feeds over the same
      window through the real poller, and an independent judge, not a desk,
      labels event identity across the captures afterwards.
    main_uncertainty: whether same-window real feeds give at least 8 events covered in 2+ languages; if not, widen the window or add feeds before judging
    next_observation: the panel's verdict on this file; then the design note
  blocker_or_risk: cross-lingual matching by the model is unproven for zh/ja/ko against en; the fixture must include them before a grouping model is chosen (L2's 200-pair evaluation stays in L2b)
  next_action: adjudicate the convergent panel on this file (running); then the S0 design note

receipts: []
---

# L2a: synthesis intake

The owner's principle is the specification
(docs/reports/owner-roadmap-rulings-2026-10-14.md, "The product
principle"). This file tracks the first slice.

Slices, in order:
- **S0, reconcile and design.** Observe semantic dedup and the fixture
  path. Write the design note (the library view, the routing policy and
  the synthesis contract) and run a panel on it.
- **S1, build.** The library view on management's records surface and
  the policy and contract in the prompt defaults. Retire semantic dedup if
  it drops coverage a desk has not seen. Then push, CI and deploy.
- **S2, prove.** The fixture run on a minted test computer, then 6 hours
  of the live paper. Optionally add a few non-English general-news feeds
  to the paper, chosen for licence (commercial use) and overlap with the
  existing world-news feeds.
- **S3, close.** A consensus review on the landed candidate with its
  deployed evidence; then hand off to C1e B1.
