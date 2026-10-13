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
      status: landed (code b0d69d4d; staging acceptance PASS, docs/evidence/m0-model-pools-acceptance-2026-10-12.json; failover-event proof carried to C0)
      depends_on: []
      note: model pools (owner 2026-10-12, "do this configuration setup first"); red; its acceptance is in the design doc
    - id: D0
      path: docs/reports/dead-code-census-2026-10-12.md
      readiness: reviewed
      status: landed (6505fa33; about 50,000 lines deleted; staging acceptance PASS, docs/evidence/d0-dead-code-acceptance-2026-10-12.json)
      depends_on: [M0]
      note: >-
        dead-code census, then removal (owner 2026-10-12: "identify all the
        dead code and remove it, after I learn about it and what it
        does/did"). The census names every dead item with what it does or
        did, why it is dead, its citers, and whether a cut already deletes
        it; the owner reads it and rules; removals land before C0 so the
        cuts start from a smaller system. Read-only until the owner rules.
    - id: C0
      path: docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md
      readiness: reviewed
      status: landed (2026-10-13; receipts docs/evidence/c0-eyes-acceptance-2026-10-13.json)
      depends_on: [D0]
    - id: C1
      path: docs/definitions/choir-reeng-cut1-authority-2026-10-12.md
      readiness: reviewed
      status: core landed (2026-10-13 at 76ab40fd; receipts docs/evidence/c1-core-mgmt-acceptance-2026-10-13.json, c1-core-crash-acceptance-2026-10-13.json); C1e items remain
      depends_on: [C0]
    - id: C1e
      path: docs/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md (scope: cut 1 scope_split, C1e items)
      readiness: reviewed (convergent round 3 accept 2026-10-13)
      status: working (next B0, the instruments)
      depends_on: [C2]
      note: engineering and self-development on records, path B deleted, L5, L7, wholesale packet and work-item deletion; runs after C2's autopaper_live and before the completion review; waits for the ledger metamission's L1a (docs/definitions/choir-ledger-l1-restart-2026-10-13.md), which changes the same boot re-drivers (one shared-runtime cutover at a time, 2026-10-13)
    - id: C2
      path: docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md
      readiness: reviewed (pre-flight panel 2026-10-13 running)
      status: autopaper live 2026-10-13 (docs/evidence/c2-autopaper-live-2026-10-13.json); residual acceptance items open (cut now card)
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
    - action: "the end-to-end story: published in public from its first revision, disconfirming evidence arrives, a later version corrects it, the earlier version's link unchanged, logged-out read; and the live window (watched for at most an hour, boundaries.pace 6) producing stories at the rhythm of ingestion (Cut 2's autopaper_live)"
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
    - >-
      owner 2026-10-12, at launch: the whole metamission runs autonomously
      overnight, through C2's autopaper_live. The run itself takes the
      "one owner act" that starts the paper computer's schedule, and the
      first public permalink needs no owner gate ("the first public link
      doesn't really matter"). This stage proves the architecture (the
      concept was validated earlier); a non-functional requirements phase
      and deferred missions come before anyone is told it is stable in
      production. Published versions still never change once public.
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
  delegation: >-
    The orchestrator (Opus) does red-surface code, integration, trace
    debugging, the four desk prompts, tripwire judgment and every push.
    Opus subagents in worktrees take independent units that need design
    judgment: C0's trace export, tracer and invariant checker in parallel
    once the orchestrator has fixed the bundle format; C2's units 2a and
    2b. C1 stays mostly with the orchestrator (one tightly coupled
    change). Sonnet subagents take mechanical deletion sweeps, test
    fallout after a shape change, probe runs, trace summaries,
    measurement write-ups and code searches; never prompts, Texture
    writes or gateway calls. Only the orchestrator pushes.
  review_loops: >-
    Owner 2026-10-12. (1) When the redesign tripwire fires (decisions
    §12.5): write the structural assessment, then run an agentic-consensus
    panel on it (skills/agentic-consensus; lateral mode first to break the
    frame, then convergent on the surviving reframes, --keep-going).
    Adjudicate the findings, record the chosen redesign in the cut's
    breakage log, and continue the cut on it. If the same layer trips
    again after two panel-guided redesigns, stop and leave the
    assessments for the owner. (2) When every station has landed:
    run a convergent panel on the landed result and its evidence
    (deployed acceptance, tracer bundles, autopaper_live). Fix the
    blocking findings through the Landing Loop and rerun the panel.
    Iterate until a round raises no new blocking finding; then the
    metamission is complete. Panel prompts, manifests and adjudications
    are archived under docs/reports/ as review receipts.
  pace: >-
    Owner 2026-10-13, after Cut 0 spent hours on a 70-minute session probe
    and three back-to-back acceptance runs: "in development, we want to
    have a rapid feedback loop. So we shouldn't be doing these long runs
    ... we should be doing things quickly. Watching the traces and
    iterating." The 60-minute run limit is a guardrail against runaway
    spend, not part of any spec, and nothing is tested past it. Rules,
    overriding the cut files where they differ:
    (1) The loop is push, deploy (about 9 minutes), one tracer run
    (target 10 minutes or less), read the trace, fix. Aim for three
    iterations an hour.
    (2) Nothing runs live to confirm what the code already says: write
    the failing test and fix it.
    (3) The tracer runs fast by default: crash drill as soon as a cell is
    in flight, no waiting for desks to settle, a 2-minute quiescence
    bound, no load retries. Read run events while it runs and stop as
    soon as the answer is visible. Stop every test computer after use.
    (4) A cut's acceptance is one tracer run at the landed SHA that shows
    the cut's claim, with the checker's verdicts; a second run only when
    the first is ambiguous. A failure that a later cut owns is a
    recorded finding, not a blocker.
    (5) Cut 1 lands commit by commit, each through deploy and a tracer
    run, ordered so the checker's verdicts move every time; not one
    acceptance at the end.
    (6) Cut 2's acceptance watches the live window for at most an hour;
    the schedule keeps running after the receipt is written.
    (7) A cut's consensus review runs in the background and does not
    hold the next cut; the completion loop in review_loops stays.
    (8) No wait longer than 15 minutes without reading the trace in
    between.
    Amended by the plan review of 2026-10-13
    (docs/reports/reeng-plan-review-2026-10-13.md, owner-requested
    lateral and convergent panels): (9) a landing is read by a
    claim-directed tracer scenario (scripts/reeng_tracer.mjs --scenario),
    the full story scenario only at cut boundaries; (10) every tracer run
    is identity-fenced (--expect-sha: staging at the landed commit, no CI
    run on main queued or deploying, the guest build unchanged during the
    run), and nothing is pushed while a run is in flight; (11) pushes are
    batched by dependency as separate commits, and every push that changes
    behavior still gets its scenario run (rule 5's letter relaxed, its
    purpose kept); (12) deterministic claims are proven by tests before the
    push, and the deployed scenario proves the product path.

now:
  status: executing
  slice: >-
    M0, D0 and C0 landed. C1 core landed 2026-10-13 at 76ab40fd: crash
    closure, records as the work state for Texture with research and with
    management, Texture turns without work items, the work-state epoch,
    late arrival, desk surfaces and prompts that teach only callable verbs;
    accepted by the management and crash scenarios
    (docs/evidence/c1-core-*-acceptance-2026-10-13.json); breakages B1-B4
    fixed. C1e (engineering on records and the wholesale deletions) runs
    after C2's live window and after the ledger metamission's L1a. C2 executing: pre-flight done, review panel
    running, quotematch building. The owner's pace ruling and the plan
    review of 2026-10-13 reshaped the loop: claim-directed tracer
    scenarios, identity fence, batched pushes; a tracer attempt now takes
    7 to 12 minutes instead of 22 to 42. The cell-error analysis found 52%
    of failed desk cells kernel-fixable; the largest class is fixed.
  source_ref: 76ab40fd
  deploy_identity: staging 76ab40fd
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
    main_uncertainty: how many fix loops L3 (records, cursors, packets and work items deleted) needs; whether Cut 2 starts tonight
    next_observation: L1's crash-scenario receipt, then L3's
  blocker_or_risk: Cut 1 is a large red change across store, agentcore, textureowner and yaegikernel, now landed commit by commit (boundaries.pace 5); test computers left running hold Node B memory and stall mints (docs/problems/new-owner-waits-while-qa-computers-hold-node-b-memory-2026-10-13.md); the owner's computer takes every cut (accepted risk, §13.1)
  next_action: C2 unit 2a per Cut 2's landing order (quotematch and Refs, block captures and Read, async search in parallel; then Texture Write; then support judgement), one claim-directed scenario per behavior push; tag reeng-cut2-last-good at 76ab40fd before 2a deploys

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
