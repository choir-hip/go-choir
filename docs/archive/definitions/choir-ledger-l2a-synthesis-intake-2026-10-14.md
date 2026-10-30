---
definition_version: 4

# Station L2a of the ledger metamission
# (docs/archive/definitions/choir-ledger-metamission-2026-10-13.md): synthesis
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
# Owner 2026-10-14, later in session, on the architecture: "we can use jev
# for semantic dedup. just ask it if two articles in context are
# fundamentally the same. but more generally we should give researchers
# more power here. researchers should be researching the corpus. jev can
# categorize captures as they come in, and mgmt then send content to
# researchers that are already working on existing topics, as well as
# research desk can spawn new researchers for new topics".
# Further owner direction the same night: "we should have a smart way to
# learn topic vs story; and topic and story boundaries ... learned
# clustering, not me setting these in advance ideally. and yes, we can
# intelligently let research threads go dormant... we should do this
# redesign then optimize resources."
# Revised after convergent round 1 (claude, gemini, deepseek send_back;
# glm accept with repairs; codex usage-limited): one architecture
# throughout, red class, a frozen baseline and an operational metric, a
# deterministic labelled fixture, a per-computer enable with tripwires.
readiness: drafted

review:
  reviewer: agentic-consensus convergent round 1 on 5d75fae9 (3 send_back, 1 accept with repairs; findings adopted in this revision); round 2 pending
  frozen_ref: none
  verdict: send_back
  evidence_ref: scratchpad panel-l2a-r1 (session diagnostics; adjudication in now.decision)

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
      platform tier, 14 sources, mostly English wires and tech feeds)
      holds 50 stories. 47 opened from one capture, 2 from two, 1 from
      four. About 11 management Notes fed one more capture each into 8
      existing stories, mostly follow-ups from the same source. No story
      synthesizes across outlets or languages. These counts were read at
      00:25 UTC from management records in each story's inbox; they are
      not per-version citation counts.
    - >-
      Management's prompt (internal/promptstore/defaults/management.yaml,
      "Living stories") says to cluster by event and feed by Note. Its only
      library view is doc ids and titles. Texture's prompt is
      internal/textureprompts/texture.yaml. Research is opened per document
      by Texture (inbox research:<doc>; recordsRoute in
      internal/agentcore/inbox_records.go allows only texture:<doc> and
      research:<doc>). Research has a corpus search
      (choir.SearchWireCorpus).
    - >-
      Semantic dedup (internal/agentcore/qdrant_dedup.go) is dead code with
      no production caller. Nothing on the poll path calls a model
      (internal/agentcore/source_schedule.go). The registry is embedded
      (internal/sources/registry.json), and a schedule needs a registry
      id.
    - Model cost is unpriced on /api/costs (station LR).
    - >-
      Dated correction, 2026-10-14 00:55 UTC, to how the first observation
      reads. It does not change the counts. Scored by
      scripts/l2a_paper_window.py on 15:00-21:00 UTC (baseline A, on the
      current intake): 33 stories opened, 33 published, and 31 of the 33
      versions cite two or more outlets (median about 6; one cites 36).
      Research widens every story from the open web. So the stories are
      not copies of one article. What is missing is synthesis across
      captures and across the library:
      - intake opens a story per capture and rarely feeds one (11 feeds);
      - research re-searches the web per story instead of building on the
        corpus;
      - stories do not update one another.

finish:
  deliver: >-
    Autopaper writes one story per real-world event, synthesized from the
    convergence and the conflicts among its sources, across outlets and
    languages. A new capture updates the story it bears on, and the new
    version says what changed. Topic and story boundaries are learned
    from recorded judgments, not preset.
  artifact: >-
    A deployed intake, enabled per computer, in which:
    - Jev labels each arriving capture (same article as X; same event as
      story S; same topic T but a new event; new). The label is recorded
      as a typed edge in the object graph. Labels never filter.
    - Management routes each capture with its label: to the standing
      researcher of topic T, or to the research desk, which spawns a
      researcher for a new topic. Every capture still gets an answer.
    - Topic researchers read the corpus (SearchWireCorpus, ReadCapture)
      and the web across languages. They report convergence, conflict and
      what each new capture changes to the stories of their topic. They
      go dormant when idle and wake on a new capture.
    - Texture writes and revises each story from those reports, and stays
      the sole writer.
    - Splits and merges by researchers or management are recorded edges.
      They become labelled examples for Jev, so boundaries improve from
      the ledger.
    - The dead semantic dedup is deleted.
  acceptance:
    - action: >-
        The fixture run. A static multilingual feed of real articles
        covers at least 8 real-world events, each in 2-3 captures across
        at least 2 languages including zh, ja or ko against en, from at
        least 2 different outlets, plus at least 6 unrelated singletons.
        It is served from this public repository through a fixture-only
        registry entry (tos_class test, never scheduled on the paper
        computer). A minted test computer with the intake enabled polls
        it through the real poller. The event labels live only in the
        proof harness, outside the repo and outside the desks' reach. A
        judge from a model family other than Jev's and the desks',
        registered before the run (model, prompt, tie rule), labels
        anything ambiguous.
      proves: >-
        Each labelled event yields exactly one story. (Amended by the
        owner 2026-10-14: "no hard rule that multiple languages be in
        every story. That's silly"; the earlier clause that each
        published version cite at least 2 languages and 2 outlets was a
        test bar only, never a production rule, and is removed.) Over all same-event capture pairs, merge recall is at least 0.8
        and false merges of different events are at most 1. Every
        capture has a disposition. Singletons open or decline. The run is
        scored by a script against the labels, and the bar is not
        lowered: a shortfall is recorded as incomplete.
      evidence_class: deployed proof
    - action: >-
        The live paper, measured in two 6-hour windows with one script: a
        baseline window before the paper is enabled (starting tonight on
        the current build) and a window after. The script counts, for
        stories opened or revised in the window:
        - citations in the current published version, by independent
          outlet. An outlet is the publisher's registered domain, and wire
          copies count once;
        - feeds into existing stories;
        - captures without a disposition;
        - a sample of revision diffs checked for substance.
        The denominator is the same-event, cross-outlet capture pairs the
        registered judge finds in each window.
      proves: >-
        Against the measured baseline, merge recall over that denominator
        rises without false merges, and feeds into existing stories rise.
        The share of versions citing two or more outlets is already 0.94
        in baseline A, so it is reported but is not a target. Publication continues. Cost per published
        version and per capture is reported, including Jev's added calls,
        and not capped.
      evidence_class: deployed proof
  rollback: >-
    Owner act: disable the intake on the computer. The paper falls back to
    the current intake on its next turn and needs no deploy. Repo
    rollback is git revert, then deploy and an idle refresh of the paper
    computer, with its effective build read back. Published versions are
    immutable and untouched.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize merge recall of same-event captures across outlets and
    languages into one story, with false merges near zero, and the share
    of a story's evidence that comes from the corpus and the library
    rather than a fresh web search. Every capture
    keeps a disposition and publication does not stall.
  goodharting_would_be: >-
    Citing a second source that adds nothing; merging unrelated events;
    counting wire copies or one outlet's language services as
    independent; declining captures to shrink the denominator; trivial
    "what changed" lines on Notes; moving the fixture bar after seeing
    the run.

homotopy:
  realism_axis: >-
    a pre-labelled pair probe of Jev (20-40 cross-lingual pairs) -> the
    labelled multilingual fixture on a minted test computer -> the live
    paper on its 14 sources -> the live paper with non-English feeds (the
    owner's act) -> the long tail (after the architecture work)

boundaries:
  mutation_class: red
  red_ceremony:
    conjecture_delta: >-
      L2a-1 replaces the visibility-only claim with the owner's
      topic-researcher architecture (see now.conjecture).
    protected_surfaces: >-
      the poll path (a new model call on arrival), management and research
      records routes (topic addressing and spawn), Texture-facing records,
      provider calls through the existing judge route
    admissible_evidence: deployed proof on a minted test computer and on the live paper; unit tests shape the code only
    rollback: see finish.rollback (per-computer disable first)
    heresy_delta: {discovered: [one capture per story on the live paper, a dead semantic dedup still in the tree], introduced: [], repaired: []}
  authority_sources:
    - owner 2026-10-14 in session (synthesis principle; "intake redesign, very important"; Jev categorizes, topic researchers on the corpus; learned boundaries; dormancy)
    - docs/reports/owner-roadmap-rulings-2026-10-14.md
    - docs/archive/definitions/choir-ledger-metamission-2026-10-13.md (L2 note, overnight plan)
  must_preserve:
    - Texture is the sole writer of canonical revisions
    - every captures record gets an answer (a story, a feed or a decline)
    - Jev's labels are labels, never filters; no runtime step hides a capture from the desks
    - the arrival call is bounded and recorded, and its cost is reported
    - the intake changes behavior only on a computer where it is enabled; the paper is enabled only after the fixture passes
    - the paper's schedules survive the update (an idle refresh, never a crash)
    - one shared-runtime cutover at a time; no deploy while a tracer runs
    - spend is tracked, never capped
  tripwires:
    - >-
      On the paper after enabling: if no version publishes for 90 minutes
      while captures arrive, or more than half the captures in an hour are
      declined, disable the intake (the owner act), record the problem,
      and continue on the fixture.
  excluded:
    - the per-window digest and the 10x burst bound (L2b)
    - adding sources to the paper computer (owner act; ruling 2 defers source scaling)
    - hypothesis settlement (L3), though a story's boundary may use its Cast hypotheses
  protected_surfaces: [poll path, management and research records routes, Texture-facing records, the paper computer's schedules]
  admissible_evidence: deployed proof on staging (a minted test computer and the live paper); unit tests shape the code only

now:
  status: working
  slice: >-
    S2, prove. S1a is deployed (66fe0917, CI 38499660090, deployed
    02:25:30 UTC 10-14): A5 (lossless per-poll limit, pending arrivals,
    script and language on arrivals), the intake.labels setting, Jev
    labels on arrival, the label view, choir.Stories, choir.Feed and
    choir.Decline, the synthesis contract, the fixture registry entry,
    search_wire_corpus fixed and the dead Qdrant dedup deleted. Intake is
    off by default. The fixture run (acceptance action 1) started at
    02:27:33 UTC on minted test computer
    computer-00b7886133ccd87d0cecc7079b9d78ba (guest build 66fe0917),
    intake.labels on, one fixture schedule, 10-minute interval, 90-minute
    wait, scored by scripts/l2a_fixture_score.py against labels kept
    outside the repo. Deviation from the design note, recorded here:
    max_items is 9, not 27 or more, so the 27 fixture items span three
    polls. That is stricter: it also tests lossless deferral and labels
    across polls. Early read (02:41 UTC): 2 polls, 0 errors, 18 new
    arrivals, 0 pending, 12 story documents opened.
  source_ref: 4814f21f
  deploy_identity: staging 66fe0917
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: L2a-1
    claim: >-
      Synthesis comes from standing topic researchers that research the
      corpus, not from one research run per story. Jev labels each capture
      on arrival as a recorded edge, never a filter. Management routes
      the capture to the topic's researcher, or the research desk spawns
      one. The researcher reports convergence, conflict and what changed,
      and Texture writes from it. With that, multilingual coverage of one
      event becomes one synthesized story, and boundaries learned from
      recorded splits and merges hold up without preset categories.
    test: >-
      first the Jev pair probe (does Jev tell same-event from
      same-topic-different-event across zh, ja and ko against en?), then
      the fixture's merge recall and false merges against the labels, then
      the live windows
    edge: missing_oracle
    delta_o: the labelled fixture and a judge registered before the run, from a different model family
    scope_if_supported: Autopaper's intake on every source class
    status: testing
    evidence_refs: [docs/reports/owner-roadmap-rulings-2026-10-14.md, docs/archive/evidence/l2a-jev-pair-probe-2026-10-14.md]
  decision:
    what: >-
      L2a first in the deploy lane, then C1e B1, then LR lever 6. Round-1
      findings adopted: one architecture, red class, a frozen baseline and
      an operational metric, a deterministic labelled fixture, a
      per-computer enable with tripwires, an explicit paper refresh, and
      adding feeds to the paper left to the owner.
    kind: operational
    status: proposal
    evidence_ref: docs/reports/owner-roadmap-rulings-2026-10-14.md
    owner_ratification_ref: "owner 2026-10-14: intake redesign, very important; redesign then optimize resources, alongside B1"
  belief:
    believed_state: >-
      The paper copies, one capture per story. Feeding by Note exists but
      is starved. Nothing on the poll path calls a model, and semantic
      dedup is dead code. The registry holds vertical-zero feeds in other
      languages, but a deterministic labelled fixture needs a fixture-only
      registry entry and a static feed. Topic addressing and spawning do
      not exist: research is per document.
    main_uncertainty: >-
      Whether a cheap shortlist finds the right candidate stories, and
      whether Jev holds on whole captures, zh/ko and ambiguous follow-ups.
      The pair probe (01:00 UTC) answered the first-order question: 26 of
      26 cross-lingual pairs correct across en, ja, pt, es, ar, fr and de,
      0.2 s median, about $0.00003 a pair
      (docs/archive/evidence/l2a-jev-pair-probe-2026-10-14.md).
    next_observation: the S0 design note; the second baseline window (23:53-05:53 UTC) scored before the paper is enabled
  blocker_or_risk: >-
    Topic addressing and spawning are new runtime surface. Researcher
    dormancy must cost no live process. Per-poll truncation must be
    lossless: items left over are taken on the next poll and recorded as
    skipped, never dropped; check this in S0.
  next_action: >-
    intake.labels is ON on the live paper since 2026-10-14T06:50:58Z
    (paper on build 9ace75e5, refreshed by the deploy and read back; its
    schedules were restored at about 06:49 after a deploy refresh closed
    them, docs/archive/problems/planned-refresh-closes-paper-schedules-2026-10-14.md).
    Next: the after-window 06:51-12:51 UTC, scored with
    scripts/l2a_window_pairs.py and compared with baseline A (15:00-21:00
    UTC 10-13) under the tripwire; watch that a deploy refresh does not
    close the schedules again until that fix lands.

receipts: []
---

# L2a: synthesis intake

The owner's principle and architecture are the specification
(docs/reports/owner-roadmap-rulings-2026-10-14.md, plus this file's
header). This file tracks the first slice.

- **S0, design.**
  - Start the live paper's baseline window and its scoring script.
  - Run the Jev pair probe.
  - Write the design note:
    - arrival labels as typed edges;
    - learned topic and story boundaries: Jev as the online merge judge
      over a cheap shortlist of shared entities, translated titles and a
      time window; a story bounded by the hypotheses its captures bear
      on, a topic by a standing question; time as part of identity;
      splits and merges as recorded examples; a sampled independent
      audit;
    - topic addressing and spawning;
    - dormancy;
    - Texture's synthesis contract;
    - the per-computer enable.
  - Then a panel on the note and this file.
- **S1, build.** Every point of the design note, with the dead semantic
  dedup deleted. Push, CI and deploy, with the intake off by default.
- **S2, prove.**
  - Build the fixture: real articles found by search, served from the
    repo, with labels in the harness.
  - Run it on a minted test computer.
  - Then enable the intake on the paper, after an idle refresh with its
    effective build read back, and measure the after-window against the
    baseline under the tripwire.
- **S3, close.** A consensus review on the landed candidate and its
  deployed evidence; then the deploy lane passes to C1e B1.

## Owner rulings, 2026-10-14 (about 10:40 UTC)

- **O3 (Jev settles high-confidence feeds): yes.** "let [Jev] settle high
  confidence feeds." This amends L2a's earlier "labels are advice": a Jev
  label at high confidence that names an existing story becomes a system
  Feed without a model call; management (or its tasks) still decides
  everything else, and every system Feed is a recorded act that can be
  overridden. Deterministic intake is the destination named in the
  delegation design (R2-22).
- **Snapshot hibernation resumes overnight:** at least 24 h (set to 36 h,
  `VM_SNAPSHOT_MAX_AGE`).
