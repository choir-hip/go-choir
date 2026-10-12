---
definition_version: 4

# Re-engineering: three cuts to Autopaper live. Owner 2026-10-12, after a
# line-by-line review of the master report and six studies: agreement
# before building; then "a more radical cutover, less gradual. We don't
# have any backwards compatibility needs." Decisions:
# docs/re-engineering-decisions-2026-10-12.md (§2 conversations and cursors,
# §10 three cuts, §12 cutover decisions, §13 later owner rulings and the
# convergent review). Studies:
# docs/reports/re-engineering-studies-2026-10-12/. The six gradual station
# drafts R1-R6 (03499ef9) are superseded by the cuts; their code-cited
# findings live on in the cut files and at that commit.
readiness: reviewed

review:
  reviewer: owner (decisions 2026-10-12) and two 10-model panels (divergent, lateral; 16 answers) on the gradual drafts; a convergent panel (6 of 8 answered) on this file and the three cuts
  frozen_ref: 03499ef9 (gradual drafts); 2d2b864e (three cuts)
  verdict: send_back on the gradual shape; three cuts adopted (decisions §12); this file accepted by all six convergent reviewers, the cuts repaired (decisions §13)
  evidence_ref: docs/re-engineering-decisions-2026-10-12.md §12, §13
  rereview: claude opus and gemini-3.8-flash at deb6f760 (codex and gpt-6.1-sol hit a usage limit); accept (claude, gemini); stale register phrase removed

metamission:
  stations:
    - id: M0
      path: docs/model-pools-design-2026-10-12.md
      readiness: reviewed
      status: pending
      depends_on: []
      note: model pools (owner 2026-10-12, "do this configuration setup first"); red; its acceptance is in the design doc
    - id: C0
      path: docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md
      readiness: reviewed
      status: pending
      depends_on: [M0]
    - id: C1
      path: docs/definitions/choir-reeng-cut1-authority-2026-10-12.md
      readiness: reviewed
      status: pending
      depends_on: [C0]
    - id: C2
      path: docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md
      readiness: reviewed
      status: pending
      note: ends with Autopaper live, then the model comparison
      depends_on: [C1]

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (this session's doc edits in progress)
      correction_2026-10-12: those edits landed at 2d2b864e; the tree was clean at review
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
      Work items (six unrelated jobs, a closure-bug cluster) and delivery
      packets are two mutable layers beside the append-only records.
    - >-
      Live problems contradict standing rules: parallel choir.* calls
      cross-wire; a crash restart leaves work open and re-casts
      engineering; a busy desk session silently gets a second REPL; boot
      cancels trajectories through wire-publication recovery.
    - >-
      All 16 panel answers judged the gradual drafts to be protecting old
      state the owner does not need.

finish:
  deliver: >-
    Autopaper live on choir.news: captures reach management, stories are
    living Textures revised as evidence arrives, each published version
    has its own permanent link, and logged-out readers can read them. It
    runs on desks that are told the truth, own their work, talk in
    conversations whose unit of progress is the hypothesis, read their
    inboxes by cursor, and search without waiting on the slowest provider.
  artifact: Cuts 0, 1 and 2 landed on staging with deployed acceptance; the model comparison run after Cut 2
  acceptance:
    - action: each cut's acceptance on staging (choir.news), read through Cut 0's tracer, trace export and invariant checker
      proves: the cut's claim on the product path
      evidence_class: deployed proof
    - action: "the end-to-end story: published in public from its first revision, disconfirming evidence arrives, a later version corrects it, the earlier version's link unchanged, logged-out read; and the six-hour live window producing stories at the rhythm of ingestion (Cut 2's autopaper_live)"
      proves: Autopaper live
      evidence_class: deployed proof
  rollback: forward-fix after Cut 1, with a tagged last-good commit per cut; no state is snapshotted, since none is valuable before Autopaper is live (decisions §12.3, §13.1)
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize grounded, current Textures readable at permanent links per
    unit of inference and owner attention, while preserving Texture as
    sole writer, the restart rule, authority boundaries, and one
    authority per fact.
  goodharting_would_be: >-
    Deleting code without the story moving; runtime machinery that decides
    what a model should decide; counting cells, reports or sources instead
    of what reaches a reader; a tracer that passes on a happy path while
    rounds silently stay open.

homotopy:
  realism_axis: >-
    eyes (trace export, tracer, invariants on today's system) -> authority
    (records, conversations, cursors, crash closure, routing) on fresh test
    computers -> evidence to publication on fresh test computers ->
    Autopaper live on a paper computer (the owner's computer takes each cut
    at its next boot throughout)

boundaries:
  mutation_class: red (Cuts 1 and 2 touch run lifecycle, the evidence ledger, gateway calls, Texture canonical writes, publication)
  authority_sources:
    - owner 2026-10-12 decisions (docs/re-engineering-decisions-2026-10-12.md), including the delegation in §12
  must_preserve:
    - Texture is the only writer of a Texture
    - research has read-only authority toward the world
    - a crash restart never resumes work; a planned restart may, once; Autopaper has no restart policy yet and is restarted by hand
    - from the first public Autopaper permalink on, published versions never change
    - no budgets, round caps, runtime research job, runtime scoring or reminders are added
  excluded:
    - shadow folds, dual-writes, interim closers, standing anchors, one-release keeps, feature flags
    - deep recursion (sub-RLM swarms come later; one fan-out level now)
    - circuit breakers before a pathological loop is observed
    - signing keys per computer, desk or sub-RLM (deferred)
  protected_surfaces: [see each cut]

now:
  status: drafted
  slice: >-
    Decisions recorded (§2, §12, §13). Cut 0, Cut 1 and Cut 2 authored
    and repaired after the convergent review; the owner then ruled that
    nothing is preserved before Autopaper is live, that article versions
    are Texture versions, that crashes are restarted by hand, and that
    production follows the ingestion rhythm. The protocol metamission's
    P2d2 and P3 and Autopaper W2, W3a and W3 are carried by Cuts 1 and 2.
  source_ref: 2d2b864e
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: RE-2
    claim: >-
      The desks underperform mainly because of false prompts, runtime caps
      and two mutable layers beside the records, not model capability. A
      radical cutover on fresh computers, watched by an end-to-end tracer,
      reaches Autopaper live faster than staged migration, and the
      failures it surfaces are fixable inside a cut or name a layer to
      redesign.
    test: Cut 0's tracer and invariant checker on every cut; the redesign tripwire (decisions §12.5)
    edge: frame_lock
    delta_o: the trace export and invariant checker make substrate failures visible without SSH
    scope_if_supported: AI news and Autopaper on staging computers
    status: proposed
    evidence_refs: [docs/reports/re-engineering-studies-2026-10-12/README.md, docs/re-engineering-decisions-2026-10-12.md §12]
  decision: {what: "three cuts, cutover deploy counts as a crash, forward-fix, one breakage log per cut, redesign tripwire", kind: sequencing, status: settled (owner 2026-10-12, delegated), evidence_ref: docs/re-engineering-decisions-2026-10-12.md §12}
  belief:
    believed_state: the studies' and panels' code claims hold at 03499ef9
    main_uncertainty: how long push-to-healthy-test-computer takes (bounds the pace of every cut) and how large Cut 1's change set is
    next_observation: a short re-review of the repaired cuts, then Cut 0's first tracer run
  blocker_or_risk: Cut 1 is one large red change across store, agentcore, textureowner and yaegikernel; the owner's computer takes every cut (accepted risk, §13.1)
  next_action: re-review the repaired Cut 0 and Cut 1 with two reviewers, then execute Cut 0

receipts:
  - id: panels-2026-10-12
    kind: review
    ref: .agentic-consensus/reeng-divergent-2026-10-12, .agentic-consensus/reeng-lateral-2026-10-12 (session diagnostics); synthesis in decisions §12
  - id: convergent-2026-10-12
    kind: review
    ref: .agentic-consensus/reeng-cuts-convergent-2026-10-12 (session diagnostics, frozen at 2d2b864e); synthesis in decisions §13
---

# Re-engineering metamission

The studies of 2026-10-12 found desks told untrue things, held back by
runtime rules, and running on two mutable layers (work items and delivery
packets) beside the append-only records. The owner chose a radical
cutover with no backwards compatibility, and the panels agreed that the
gradual drafts were protecting state nobody needs. Three cuts:

0. **Cut 0, eyes.** See breakage fast: trace export without SSH, an
   end-to-end tracer with a crash drill, an invariant checker, fresh test
   computers; fix the two defects every measurement stands on (parallel
   calls cross-wiring, a second REPL per desk); measure the baselines.
1. **Cut 1, authority.** Records become the only work state:
   conversations single-threaded from the sender, hypotheses with stable
   ids as the unit of progress, rounds, cursors instead of packets, crash
   closure as records. Engineering only through management. Prompts
   written once.
2. **Cut 2, evidence to publication.** Async search, the research desk,
   Texture's write verb, captures in the block model with quote checks,
   per-version permanent links, living stories and the reader: Autopaper
   live. Then the model comparison.
