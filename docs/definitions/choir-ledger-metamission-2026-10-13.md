---
definition_version: 4

# The ledger metamission: from Autopaper live to a dated, resolved
# precommitment ledger over the world's sources and our own computers.
# Owner 2026-10-13, in session: precommitment records are the throughline
# ("the scientific core ... what turns it from a product to a platform to a
# network with defensibility"); cost and performance first; verticals one by
# one, Taiwan and East Asia first; "we never get into a crash loop"; "the
# roadmap is going to become a meta mission ... while we're still working on
# this, we can keep adding stuff and thinking about it". Shaped by the
# roadmap draft and its lateral and convergent rounds
# (docs/reports/roadmap-after-autopaper-live-DRAFT-2026-10-13.md,
# docs/reports/roadmap-lateral-review-synthesis-2026-10-13.md), the sourcing
# research (docs/reports/multilingual-osint-sourcing-research-2026-10-13.md,
# docs/reports/taiwan-sinosphere-discourse-sources-research-2026-10-13.md)
# and the restart design (docs/restart-subsystem-design-2026-10-13.md).
readiness: reviewed

# Review history: the roadmap this file encodes passed a lateral round (8 of
# 8) and a convergent round (5 ready with changes, 1 not ready) on
# 2026-10-13, adjudicated at b800c995. A convergent round on this file at
# 8a1477c3 (claude, opencode, gemini, deepseek, muse, glm) returned 5
# send_back and 1 accept with repairs; every blocking finding is adopted
# below (warm filter moved to L1b, an L1 goal file, the C1e gate written
# into L0, schema enums, a dated start correction, a one-move now card, L5's
# dependencies). Raw outputs:
# .agentic-consensus/ledger-metamission-convergent-2026-10-13/ (session
# diagnostics). Round 2 on the repaired file at d46d3d08 (claude, opencode,
# gemini, deepseek, glm; muse timed out; codex and gpt-6.1-sol hit an OpenAI
# usage limit) accepted it unanimously. Raw outputs:
# .agentic-consensus/ledger-metamission-convergent-r2-2026-10-13/.
review:
  reviewer: agentic-consensus convergent round 2 on d46d3d08 (5 accept of 5 answering)
  frozen_ref: git blob a560388a80f0dc5018ec3e0d84b0deb1604eb6dd (this file at d46d3d08)
  verdict: accept
  evidence_ref: .agentic-consensus/ledger-metamission-convergent-r2-2026-10-13/
  open_to_additions: >-
    Owner 2026-10-13: this spine stays open while L0 runs. New directions
    enter as `intent` stations or as station scope notes, with the owner's
    words and date; each is reviewed before it becomes executable.

metamission:
  stations:
    - id: L0
      path: docs/definitions/choir-reeng-metamission-2026-10-12.md
      readiness: reviewed
      status: working
      depends_on: []
      note: >-
        Close the re-engineering metamission, time-boxed to about a week:
        E3 rerun on cff54bf5 or later as the cursor invariant (no record
        marked read unless a Records() call returned it; mid-run arrivals
        wake the next turn), the A1 unread-source badge, C1e, the final
        convergent review. One shared-runtime cutover at a time: C1e waits
        for L1a, which touches the same boot re-drivers. In parallel and not
        gating: an off-host copy of the ledger if C1e allows (L3's entry
        criterion), the frozen labelled set for L2 (at least
        40 material events, multilingual duplicates, wire copies,
        corrections, contradictions), the Taiwan question set and source
        shortlist with permission checks for L4, L3's smallest version
        (resolves_by and criterion on Cast hypotheses, to start the
        clock), app designs for L5, registry repairs (NHK, Korea Herald,
        36kr, WHO, Agriculture Dive), retiring qdrant_dedup.go.
    - id: L1
      path: docs/definitions/choir-ledger-l1-restart-2026-10-13.md
      readiness: reviewed
      status: working
      depends_on: []
      note: >-
        No crash loops: at most (1 + allowance) spawns times each unit's
        start-limit burst per external event, then a visible hold. The
        specification is the restart design v3
        (docs/restart-subsystem-design-2026-10-13.md, reviewed; adjudicated
        in its §9). L1a (the self-sustaining loops, small, first): start
        limits on every restarting unit and deploys that release units
        latched by their start limit; crash boots run no re-driver and
        settle transitions instead. L1a touches the shared runtime, so it
        lands before C1e starts, not beside it. L1b (host only, may run
        beside C1e): grants at launchFirecracker keyed by spawn sequence,
        the existing hold with a recovery-ledger class, a wedged or failed
        computer gets the one automatic retry and then holds, explicit
        owner acts only, no auto-refresh on a held page, release by a
        different build. L1c (after C1e): host-asserted planned boots, crash
        closure as a barrier, then visibility (the ledger tail in vmctl list
        and on the owner's page, an alert on any hold). Schedules option B
        is the design default and waits for the owner. Acceptance: the
        design's fault-injection matrix on a minted test computer.
    - id: L2
      path: docs/definitions/choir-ledger-l2-bounded-attention-2026-10-13.md
      readiness: intent
      status: pending
      depends_on: [L0]
      note: >-
        Bounded attention. Captures are observations, not duties. The
        runtime does only lossless reductions (exact dedupe, language as
        metadata, grouping proposed as an observation a desk can split or
        merge); management gets one digest record per poll window; a cheap
        triage call per digest applies management's written policy
        (standing questions, exploration share) and records a disposition
        for every capture. Acceptance on a minted test computer through the
        real poller with a fixture feed: the labelled set and a 10x burst;
        the tier-1 burst replayed alongside; every capture has a
        disposition; duplicates add no paid calls (paid calls per digest
        against per unique event); replay completion time; at least 95% of
        labelled material events reach a desk; recall by language and source
        class; the audit sample reported.
    - id: L2a
      path: docs/definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        Synthesis intake, pulled ahead of C1e B1-B5 by the owner 2026-10-14
        ("intake redesign, very important. until then we are just copying").
        One story per real-world event across languages, written from the
        convergence and conflicts of its sources; a new capture updates the
        story it bears on. First slice of L2; L2b keeps the digest, triage
        and burst bounds.
    - id: LR
      path: docs/definitions/choir-ledger-lr-resource-2026-10-13.md
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        Resource efficiency and model cost (owner 2026-10-13/14): RAM,
        storage, CPU and tokens per task; agents per computer; spend
        tracked, never capped. Levers 1-2 landed; lever 6 (cell output out
        of the transcript) next. Owner 2026-10-14: SA and S3 are
        accelerated into this cost track; SP stays deferred for building
        and is investigated for the owner
        (docs/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md).
        Order: host sampler and pricing; balloon / free-page-reporting
        probe; desk-worker memory budget, GOMEMLIMIT and Dolt cache bounds;
        automatic store compaction; engine lock; S3 snapshots; store split.
        Residual for 10-14/15: Autopaper across several computers on one
        host.
    - id: LU
      path: docs/definitions/choir-ledger-lu-guest-updates-2026-10-14.md
      readiness: reviewed
      status: working
      depends_on: []
      note: >-
        Guest updates (owner 2026-10-14: "I want to be pushing safe updates
        out to guest computers ... this is going to relate to the
        self-development system"; "Yes, this all makes sense as a new
        station"). Pull, not push: a deploy changes the target, vmctl's
        sweep (the only update driver) offers each pending computer its
        update at every pass, the guest restarts at a quiet moment or after
        a bounded drain that defers new work and is never forced. Slices
        (round 2): LU1a host-only retry and LU2 persisted epoch and
        migrations first (LU2 before C1e's next epoch bump); then LU1b the
        drain; LU3a rings test -> paper -> owner with halt and refusal;
        LU3b probation and automatic rollback to the retained image (needs
        L1c and L1b); LU5 one apply path shared with self-development; LU4
        binary-only swap only if measured downtime warrants it.
        Specification: docs/reports/lu-guest-updates-design-2026-10-14.md;
        panels adjudicated in
        docs/reports/lu-guest-updates-adjudication-2026-10-14.md. Round 4
        was judged per slice: LU1a, LU2, LU3a, LU3b and LU4 reviewed (LU1a
        and LU2 may start); LU1b and LU5 drafted, with LU1b's final call
        escalated for a structural decision after four rounds. Only LU3b
        depends on L1 (L1c, L1b).
        Cause: the paper is never idle and updated only by an owner act
        (docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md).
    - id: L3
      path: docs/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md
      readiness: reviewed
      status: pending
      depends_on: [L0]
      note: >-
        Hypotheses settled and scored. Entry criterion: a recoverable
        off-host copy of the ledger (it gates L3-2; the clock-only L3-1 may
        start before it). Cast hypotheses carry a kind (assertion or
        forecast) and a criterion; forecasts also carry resolves_by (7
        days or less); settlement (outcome and evidence
        refs, by the designated settler) is separate from scoring (a record
        with scorer identity, from Jev or a model; two scorers can
        disagree); a score uses the committed version and its original
        probability, never a later edit; an ordering witness shows the
        commitment preceding the earliest capture its settlement cites
        (the paper's event-chain sequence for the commitment and the
        CaptureArrival, plus a first-creation witness on the Store B
        path; never InboxSeq or timestamps); per-source
        analytics views with denominators (lead time, independent
        corroboration by owner and lineage, corrections). Acceptance: ten or
        more short-horizon hypotheses committed before their resolving
        evidence, at least half settled with evidence refs, Brier per desk
        visible as an analytics view.
    - id: L4
      path: docs/definitions/choir-ledger-l4-taiwan-2026-10-13.md
      readiness: intent
      status: pending
      depends_on: [L2, L3]
      note: >-
        Taiwan and East Asia as standing questions: 10-20 questions chosen
        with scheduled resolutions (TWSE monthly revenue, CEC, MND figures,
        CISA KEV); about 30 zh-TW and 36 ja/ko/zh feeds (registry only); a
        JSON-diff connector (TWSE, then KEV); public-discourse sources
        that work today (JOIN open data, the Legislative Yuan API, MAC
        polls), aggregate-only with n of at least 20; a 200-pair
        cross-lingual evaluation before choosing a grouping model;
        corpus-first retrieval for settlement evidence. Gate: a seeded
        zh-TW/en/ja fixture proves question match, grouping and one
        structured settlement end to end. A 14-day live soak is observed,
        not gating. Telegram out pending the owner; CNA and UDN out if
        Autopaper counts as commercial.
    - id: LP
      path: docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        App packages, pulled in from the supervised app-development
        metamission. Owner 2026-10-20 (owner notes section 28): "yes, we
        should do the missions in order through app packages as recommended".
        This moves LP ahead of the 2026-10-14 order (efficiency, Autopaper
        scaling, then LP) and relaxes the old edge to L2a, LR and LU: S6 needs
        none of them (LU depends on S6's divergence signal, not the reverse;
        LU is needed from S7-1, when package bytes reach other computers).
        Order: S6 (the commit gate, the file above), then S7-0 (every existing
        first-party app becomes a package; definition:
        docs/definitions/choir-appdev-s7-app-packages-2026-10-01.md; a
        successor stub is in the S6 file), with a frontend self-development
        drill on one rewrite candidate in parallel; rewrites of the weaker
        apps (not mail, settings, autopaper or texture) follow, one package
        at a time. Station definition of S6:
        docs/definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md.
        Autopaper work and the data model continue alongside.
    - id: LA
      path: unauthored (docs/reports/roadmap-sequencing-appdev-security-2026-10-14.md, phase 2)
      readiness: intent
      status: pending
      depends_on: []
      note: >-
        Auth and account (owner 2026-10-14: "we need to make a better auth
        system ... I don't even think we even have passkey recovery").
        Recovery today creates a token and never sends it, and there is no
        recovery screen. Slices (owner 2026-10-14): passkeys plus email
        magic-link sign-in through maild, recovery through the same path,
        several passkeys and visible sessions, the account as the login
        for app windows. No Google/Apple/OAuth providers for now; an
        OpenAI sign-in for users' own credits is a later idea.
    - id: SP
      path: docs/definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md
      readiness: intent
      status: deferred
      depends_on: [LP]
      note: >-
        Second host, off-host copies, log/metric sink, zero-downtime
        deploys, failover. Owner 2026-10-14: needed before agentic red
        teaming; the platform keys are copied to the owner's iMac
        (docs/evidence/platform-key-backup-2026-10-14.md).
    - id: LT
      path: unauthored (docs/reports/roadmap-sequencing-appdev-security-2026-10-14.md, phase 5)
      readiness: intent
      status: pending
      depends_on: [LP, SP]
      note: >-
        Owner 2026-10-14: S8-S11 (publication, fleets, templates, fleet
        security pushes) come before red teaming, possibly before SP.
        Agentic red teaming (owner 2026-10-14), after supervised
        self-development (Gate 2) and SP failover, against disposable
        isolated forks (S9 re-keying first): capsule boundary, egress, auth,
        gateway, publication and adoption.
    - id: LF
      path: unauthored (docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md)
      readiness: intent
      status: pending
      depends_on: [L2a]
      note: >-
        Front-page ranking and personalization (owner 2026-10-14). Rank by
        importance (coverage breadth across outlets and languages from
        L2a, velocity, standing-question relevance, a recorded editorial
        judgment), with attention a weak signal; learn by scored
        predictions on the ledger; personalize on the reader's own
        computer (Follow, Less like this, More on this; no thumbs).
        Investigate and design first; do not change the live front page
        until the owner reviews.
    - id: LC
      path: unauthored (docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md)
      readiness: intent
      status: pending
      depends_on: [LA]
      note: >-
        Contributions (owner 2026-10-14): readers' own sources as
        captures, op-eds citing pinned story versions in an Opinion
        section, suggested edits to published stories (Texture stays the
        sole writer), "make your own version" and "include this story"
        in plain words. Needs accounts (LA). Design first.
    - id: L5
      path: docs/definitions/choir-ledger-l5-read-models-2026-10-13.md
      readiness: intent
      status: pending
      depends_on: [L2, L3, L4]
      note: >-
        Ledger read models. Reader: one pinned story version opens with PDF
        and media evidence and every chip resolves to a capture block, for
        a logged-out reader. Autoradio prototype: an audio artifact names
        the exact version it narrates, and a later correction produces a new
        edition citing the change. Self-development authoring optional.
  later_metamissions: >-
    Further verticals (Global South and India, cybersecurity), the apps
    rebuild beyond the reader, the security hardening pass and agentic red
    teaming (guest-to-corpusd authority containment before any public API),
    self-development Gate 2 (S4-S6) as precommitted operations, a daily
    public hash of the ledger (deferred from L3), SA density
    and SP production infrastructure re-measured after C1e, A/B image slots,
    the news API, paid tiers.

start:
  captured_at: 2026-10-13T19:15:00Z
  source:
    canonical_ref: main 8a0da6b9
    deploy_identity: staging choir.news cff54bf5
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled` at the repo root)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
    - path: stash@{0..2}
      status: unknown
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Autopaper live since 2026-10-13 on one paper computer, 14 sources.
      Published versions immutable (30 of 30), every committed revision
      published, 64 of 68 unsupported verdicts repaired by a later revision.
    - >-
      46 of 118 arrivals went unanswered; the trace showed a cursor bug
      (records marked read that no cell saw), fixed at cff54bf5; the rerun
      is open. The per-capture duty does not scale on its own terms (about
      1,000 records an hour at 211 sources on one serial desk).
    - >-
      Cast hypotheses already carry stable H-ids with open, settled and
      dropped states (internal/store/inbox.go); nothing settles or scores
      them.
    - >-
      Several boot and re-drive paths are unbounded (resolve, warm sweeps,
      guest systemd units without StartLimit, self-minted planned markers);
      the restart design names them.
  start_correction:
    dated: 2026-10-13T20:45:00Z
    preserves: canonical_ref main 8a0da6b9, deploy cff54bf5, "the rerun is open"
    correction: >-
      The capture instant postdates its ref. At 19:15Z main already held
      e928e3e1 (E3 rerun passed, 18:57Z) and 04a7b0c2 (A1, 19:00Z); this
      file landed at 8a1477c3 (19:01Z). L1a-1 (dc184db2) and L1a-2
      (c08ead07) were then committed and pushed with fdc70329; they belong
      to L1 and are recorded as its receipts.
    evidence: git log 8a0da6b9..fdc70329; the convergent round on 8a1477c3
    why: the start receipt is what promotion freezes; it must name one reality

finish:
  deliver: >-
    A running, dated ledger: Autopaper on choir.news whose management
    commits to hypotheses before the evidence that settles them, settles
    and scores them as separate records, attends to every capture through a
    recorded disposition at measured cost, covers Taiwan and East Asia as a
    standing-question vertical, renders the ledger to readers (with an
    Autoradio prototype), and runs on computers that cannot crash-loop.
  artifact: stations L0-L5 landed on staging with deployed acceptance; the ledger's off-host copy; the Taiwan vertical's question set and settlements
  acceptance:
    - action: each station's acceptance on staging (choir.news), read through the tracer, trace export and invariant checker
      proves: the station's claim on the product path
      evidence_class: deployed proof
    - action: the final convergent panel on the landed result and its evidence, iterated until a round raises no new blocking finding
      proves: the metamission holds as a whole
      evidence_class: review receipt over deployed proof
  rollback: per station; tagged last-good commit before each red station; the ledger is append-only and its off-host copy is the recovery path for history
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize informative settled hypotheses (resolution-weighted Brier
    gain against a base rate, with traceable evidence) and captures given a
    recorded disposition, per dollar and per unit of owner attention, while keeping Texture the sole writer, crash restarts
    terminal and bounded, and every reduction of the world visible.
  goodharting_would_be: >-
    Hypotheses written after the evidence or so vague they cannot fail;
    "filtered" meaning "missed"; counting sources, captures or stories
    instead of settled commitments; corroboration counts that measure wire
    syndication; near-certain or scheduled-fact hypotheses padded for count;
    the committing desk settling its own hypotheses; a blanket "ignore"
    disposition; Brier at n of about 10 read as calibration; a
    no-crash-loop claim that holds only because nothing restarts
    automatically, or holds that never release.

homotopy:
  realism_axis: >-
    fixture feeds on minted test computers -> the live paper computer on
    the current 14 sources -> the Taiwan vertical's sources and scheduled
    settlements -> the owner's computer and further verticals

boundaries:
  # L1 touches vmctl, boot and run acceptance; L2 and L3 touch the evidence
  # ledger, gateway calls and Texture-facing records. Red ceremony (heresy
  # and conjecture deltas, admissible evidence) lives in each station file.
  mutation_class: red
  authority_sources:
    - owner 2026-10-13 in session (goals 1-11 in the roadmap draft; "we never get into a crash loop"; "the roadmap is going to become a meta mission")
    - docs/choir-doctrine.md, CLAUDE.md
    - the re-engineering decisions (docs/re-engineering-decisions-2026-10-12.md) for everything L0 inherits
  must_preserve:
    - Texture is the only writer of a Texture
    - research has read-only authority toward the world
    - a crash restart never resumes work; a planned restart may, once (target invariant, repaired by L1c — only when the host attests it; until then the guest self-marks and L1a-2 bounds its replays)
    - published versions never change
    - sources never precommit; a source's record is a view over settled claims with denominators, never one reputation number
    - the runtime never thresholds a capture out of a model's sight; every reduction is recorded and reversible
    - no runtime reasoning budgets, round caps or runtime scoring
  amendments:
    - id: cost-2026-10-13
      status: proposed (owner decision 1 below)
      text: >-
        Owner amendment 2026-10-13 ("cost and performance first") to the
        2026-10-12 "no budgets" boundary: spend is measured, not budgeted,
        by the runtime. Every station's acceptance reports measured cost
        from trace events (the llmcost aggregator) as $/capture, $/story and $/settled hypothesis.
        Any ceiling is an owner kill switch in provider or host policy,
        never a value the runtime reads to decide. No new recurring spend
        (paid APIs, licences, hosts) without owner sign-off.
  excluded:
    - Telegram ingestion and scraping of walled platforms (PTT, Dcard, LIHKG, Mobile01) until the owner rules
    - individual-level tracking of people in public-discourse sources
    - a public API before guest-to-corpusd authority is contained
    - management-owned reopen of work after a crash (owner-only until crash attribution is trustworthy)
  protected_surfaces: [vmctl boot actuator and ownership, guest boot and systemd units, run acceptance, the evidence ledger, gateway/provider calls, publication]
  pace: inherits the re-engineering metamission's pace rules (claim-directed tracer scenarios, identity fence, batched pushes, nothing pushed while a run is in flight)
  review_loops: inherits the re-engineering metamission's review loops; each `intent` station is authored, then reviewed by a convergent panel, before it becomes executable

# Decided; none is open (checked 2026-10-20). The five items listed here until
# then were answered on 2026-10-14 (docs/reports/owner-roadmap-rulings-2026-10-14.md,
# see decision.owner_ratification_ref below): spend tracked and never capped (no
# kill switch), Autopaper commercial (CNA and UDN stay out), Taiwan first behind
# vertical zero, Autoradio after the architecture work, schedules option B. The
# 2026-10-20 decisions digest was answered the same day: owner notes
# docs/reports/owner-notes-2026-10-14.md section 27. Still the owner's, as
# actions rather than decisions: the Telegram MTProto account setup (Telegram's
# terms are read when that station opens) and the account-recovery launch
# blocker (section 27 item 13).
owner_decisions_open: []

now:
  # This spine is reviewed; owner ratification (decision below) and the open
  # owner decisions gate `executable`. L0 executes
  # under its own reviewed file; L1a executes under the L1 station file over
  # the reviewed restart design. The spine's card points at the live station.
  status: working
  slice: >-
    2026-10-20 pointer: the active station is LP S6, the commit gate
    (docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md), by owner
    direction (owner notes section 28); the Autopaper stations below continue
    alongside. The 2026-10-16 card follows.
    2026-10-16 23:40 UTC. Handoff: docs/reports/handoff-2026-10-15.md §0
    (overnight and 2026-10-16 tables). Landed and proven on staging
    2026-10-16:
    - Event layer V1 and V2 on the paper; V3a/V3b code behind flags (off).
      V1 fails the false-merge bar twice (7.0%, 9.5%); the remaining
      merges are story-level, so the event definition is the owner's.
    - The publish gate (two outlets), Jev pinned, retention never
      destroys a warmness-protected computer, Texture ReadCapture, and a
      Done after a refused Write is refused.
    - Storage and boot robustness: the btrfs headroom guard, end writes
      retried, capture-label identity formulas, prose leaves never
      upcast, vmctl refresh timeouts, the narrowed vocabulary fence, the
      yaegi loop rewrite, and workers volunteering for the OOM killer.
    - Desk cells: the timeout counts only the cell.
    - Owner requests from the evening: embeds retired; every desk thread
      on gpt-6-luna (deepseek fails over only); article open from
      1.1-1.3 s to 0.16-0.26 s (check overlay by ID, nav-only poll);
      edge compression (bundle 467 to 55 KB) with streams never wrapped.
    - A planned restart resumes a parked Texture turn through the Texture
      controller (e3ffabbca; not yet exercised live).
    Open for the owner:
    - the event definition, which gates V3;
    - the evidence-store growth (about 1.5M small files a day);
    - the pool split (moot while luna-only);
    - the witness key, the off-host backup target, and the yaegi
      upstream report.
    All five were answered on 2026-10-20 (owner notes §27: event definition,
    item 8; evidence-store growth, §26 and items 1 and 17; witness key,
    item 9; off-host backup, item 17; yaegi report, item 5; the pool split
    stays moot). Read §27, not this list.
  source_ref: 85a9f5742
  deploy_identity: staging 85a9f5742 (deployed 23:27:53 UTC)
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: LG-1
    claim: >-
      The defensible asset is dated, unbackfillable history. If management
      commits short-horizon hypotheses before their evidence and a separate
      scorer settles them against captures, $/settled hypothesis stays flat
      or falls as sources grow along the realism axis, because bounded
      attention keeps triage cost per capture flat.
    test: L3's acceptance (commitment precedes settling evidence); $/settled hypothesis against source count at each realism-axis step; L2's cost lines across a 10x burst
    edge: missing_oracle
    delta_o: scheduled structured settlements (TWSE, KEV) as an oracle independent of the desks
    scope_if_supported: Autopaper on staging, one vertical
    status: proposed
    evidence_refs: [docs/reports/roadmap-lateral-review-synthesis-2026-10-13.md]
  decision: {what: "stations L0-L5 as listed; L1a before C1e, L1b beside it, L1c after", kind: operational, status: proposal, evidence_ref: docs/reports/roadmap-lateral-review-synthesis-2026-10-13.md, owner_ratification_ref: "owner answered decisions 1-12 on 2026-10-14 (docs/reports/owner-roadmap-rulings-2026-10-14.md): architecture and resource efficiency before source scaling; spend tracked, never capped; schedules survive crashes (option B); general and tech news vertical zero, Taiwan first; synthesis across sources and languages, never one article per capture"}
  belief:
    believed_state: >-
      Autopaper is live on the platform-tier paper computer, which the
      always-on sweep keeps warm. Crash boots re-drive nothing (L1a). The
      boot ledger (L1b) does not exist yet; L1b-0 bounds the sweep at one
      automatic warm per failure stop per 24 h. Hypotheses exist as records
      but never settle. Model cost is unpriced, and research input is
      dominated by re-sent cell output (LR, R5).
    main_uncertainty: whether Jev labels plus an uncapped library make one story per event across languages (L2a fixture), and whether the three builds merge cleanly onto the shared runtime
    next_observation: the A5 build, then S1a labels on top of it; the fixture run on a minted test computer
  blocker_or_risk: the owner's computer still runs 761d94c5 and defers every guest-image deploy; one operator refresh was already spent (docs/evidence/owner-computer-refresh-2026-10-13.json) and the deferral recurs until planned restarts are host-attested (L1c); every durable home shares one host until L3's off-host copy
  next_action: >-
    2026-10-17:
    (a) a records wake ends a run stored running with no live loop
    (failed-terminal-run-write step 2), then land it;
    (b) watch the next planned boot that parks a Texture run (e3ffabbca
    live proof);
    (c) after the soak (about 15:40 UTC): Store B indexes for the
    route and check lookups, the Dolt squash, the module bump, and
    re-landing compaction;
    (d) an RSS budget per worker (the bare-Wait guard is not needed: 3
    bare Waits in 1,316 cells since 20:00, none stalled).
    Earlier, 2026-10-15 evening:
    (a) the checkpointd replay exit on one computer (a commitment record
    with no verified identity formula);
    (b) engineering residuals (the seccomp xattr, no management
    owner-escalation target, a broker i/o timeout still seen; the seven
    unhandled verbs are fixed in bb83f3a7e, and checkpointd scratch
    pruning is in d3f248843);
    (c) re-measure the pool split over hours;
    (d) after the soak (2026-10-17), the Dolt squash, the module bump and
    re-landing compaction;
    (e) source ingestion scale-up with the owner, 2026-10-16. L0 parallel
    items done today: the registry repairs (aa8ea3f2e) and the last
    qdrant_dedup remnant (1beb2a7ab). Store B retrieval is rebased (local
    ba12a1e0c) and lands after the soak.
    The earlier plan follows. Overnight 2026-10-14 (owner: "we should do this redesign then optimize
    resources. thats our overnight work, alongside b1"). Rule: design,
    panels, measurement and code in separate worktrees run concurrently;
    deploys to the shared guest runtime go one at a time, each verified on
    staging before the next, and never while a tracer run is in flight.
    Lane 1 (deploys, in order): L2a, then B1, then LR lever 6. Each deploy
    is a staging outage until the drain problem is fixed, so deploys are
    batched and never set off by docs-side files. Lane 2
    (concurrent): B1 design round 4 and the B1 build in its own worktree;
    the L2a design note and panel; the lever-6 design note and panel; an
    LR baseline (RAM and tokens per research thread, threads per
    computer). The owner's computer refresh happens in an idle window
    between deploys. Mission order after that:
    (1) L2a synthesis intake to deployed proof;
    (2) C1e B1 build (its design rounds continue as docs meanwhile); (3) LR
    lever 6, cell output out of the transcript; (4) L1: schedules survive
    crashes (ruling 6), the fault-injection harness, L1b; (5) C1e B2-B5
    and the re-engineering final round (closes L0); (6) L2b digest, triage
    and burst; (7) LR memory levers and the deferred SA, S3 and SP0; (8)
    sources: long tail, Telegram over MTProto, RSSHub; L3; L4 Taiwan; (9)
    L5 reader and an Autoradio demo. Standing: the owner's computer gets
    one operator refresh when idle.

receipts: []
---

# The ledger metamission

The re-engineering metamission put Autopaper live. This one turns it into
the asset the owner named: a ledger that commits before the evidence,
settles after it, and shows its work, at a cost that lets coverage grow.

- **L0** closes the re-engineering metamission (E3, A1, C1e, final review).
- **L1** makes crash loops impossible: a bounded number of automatic boots
  and re-drives per external event, then a visible hold.
- **L2** bounds attention: captures are observations with a recorded
  disposition, not duties a desk must answer one by one.
- **LU** makes platform updates reach every guest computer safely: pull
  with a bounded drain, planned boots across epoch changes, health and
  automatic rollback, staged rings, one apply path with self-development.
- **L3** settles and scores hypotheses as separate records, with an
  off-host copy of the ledger.
- **L4** is the first vertical, Taiwan and East Asia, as standing questions
  with scheduled settlements.
- **L5** renders the ledger: the evidence reader and an Autoradio prototype.

The spine stays open: new owner directions enter as `intent` stations.
