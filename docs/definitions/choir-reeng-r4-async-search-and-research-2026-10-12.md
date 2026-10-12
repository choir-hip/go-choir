---
definition_version: 4

# R4: async search and the research desk redesign. Station of
# definitions/choir-reeng-metamission-2026-10-12.md. Carries
# docs/re-engineering-decisions-2026-10-12.md §1, §3, §4, §5 (owner-agreed;
# wins over the studies). Studies: reports/re-engineering-studies-2026-10-12/
# A-async-search-yaegi.md and C-research-desk-redesign.md. Replaces the
# deleted research-job and search-deadline-ladder drafts (decisions §11).
readiness: drafted

review:
  reviewer: "none yet (owner review first; owner rule 2026-10-11: panels for uncertainty, not ceremony)"
  frozen_ref: none
  verdict: none
  evidence_ref: none

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
      Search is one blocking call. choir.WebSearch (internal/yaegikernel/choir.go:456)
      goes through the guest client with a 30 s hostlink timeout
      (internal/search/search.go:85) to HandleSearch, which wraps every search
      in a fixed 30 s deadline (internal/gateway/handlers.go:914). The router
      runs waves of two providers with a first-good-enough cut-off
      (internal/gateway/searchplane/router.go:64, executeWave :221) and
      returns one merged list. Per-provider completions already arrive one at
      a time on a done channel (router.go:232) but are not surfaced.
    - >-
      Search waits on the slowest provider: Keenable answers in about 0.2 s,
      Tavily and Parallel in 2-4 s, ChatGPT in about 23 s; 15-27% of searches
      hit the 30 s cap after ChatGPT joined the pool
      (docs/search-plane-design-fast-deep-broad-2026-10-11.md §1-2).
    - >-
      A desk REPL session is one persistent interpreter in a host subprocess,
      kept across cells and across runs of the same desk ("agent:"+id key,
      internal/agentcore/tools_desk.go:111; 30 min idle retention :49;
      reclaimLocked :177, release :226). Host values reach a cell only as a
      prebound object filled from the cell frame (SessionFrame,
      internal/yaegikernel/session_loop.go:25; BindCell Begin/End,
      choir.go:145, :154). The cell result map is built at
      tools_desk.go:402-416. Study A's race-detector probe showed a
      mutex-guarded store read mid-cell is clean; a raw exported map races.
    - >-
      Concurrent choir.* network calls from model goroutines cross-wire
      results (egressRoundTrip reads the next frame with no request-id match,
      internal/yaegikernel/sidecar.go:325-335;
      docs/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md).
      R1 fixes it or keeps model goroutines off network verbs.
    - >-
      The research desk's surface still lists retired and legacy verbs
      (Message, Emit, Cast, evidence and poll verbs; choir.go:255-259).
      Children get the parent's whole prompt (research_children.go:201, :213)
      but only 11 read verbs (choir.go:263-265). The overlay invites parallel
      choir.* calls (internal/runtimeprompts/overlays/rlm_research_runtime.yaml:25).
      R1's sweep covers stale text, update_coagent leftovers
      (researchtools.go:664, :787, :922-924, :995, :1023), the 4-round cap
      (internal/textureowner/texture_turn_runtime.go:161), listed here only
      so the pre-flight can confirm R1 landed them. The R1 draft does not
      name the 64-call, 32 MiB research egress budget
      (internal/researchtools/egress_budget.go:46-47); if it survives R1,
      this station deletes it (decisions §1.2), keeping per-response caps.
    - >-
      Research cannot close its own work: its report disposition is
      hard-set to open (internal/agentcore/rlm_reduce.go:2099), and only
      engineering and Texture have a terminal or passivating tool result
      (internal/agentcore/runtime.go:3475, :3518-3521). R3 adds final-flag
      Reports and store-stamped thread ids. A Cast fills none of the
      hypothesis fields the record type has (types.CommitmentPrediction,
      internal/types/commitment.go:115-122); R3 adds the Cast hypothesis fields.
    - >-
      Baselines on staging (docs/problems/research-desk-structural-flaw-root-cause-2026-10-11.md):
      demo19 (P2a deployed, a2bcf724, demo-p2a-2026-10-11, doc 07968375) and
      demo20 (b172895b, demo-p2c-2026-10-11, doc 34f4781e). 11 research runs,
      all started by a Texture control, each 67-126 s; first report about
      2 min (demo19 run acec9ce2) and about 70 s (demo20 run 03707bc1) after
      research started; 13 and 15 claims, 10 and 11 distinct sources reached
      Texture; 5 of 11 and 5 of 10 updates rejected; 1.1 searches per active
      minute; research busy about half the active time. Haiku baseline:
      about 190 searches, 26 merged items, 139 links.

finish:
  deliver: >-
    The research desk gets to the bottom of what a Texture Cast is unsure
    about. It starts searches that return at once and reads each provider's
    results as they arrive. It reads the sources that matter and tells
    Texture what it learns as soon as its understanding changes, each claim
    labelled with its epistemic status and the hypothesis it bears on. It
    ends its own work with a final Report that says why it stopped. No
    search waits on the slowest provider, and none outlives the session
    that started it.
  artifact: >-
    Slice (a), async search. Gateway (red): a streaming mode on
    /provider/v1/search (NDJSON, opted in per request) that starts every
    eligible provider at once and sends one line per provider as it
    completes, then a final line with "M of N providers answered" and the
    providers cooling down; bounded by the provider timeout plus a margin,
    not a fixed 30 s. Health recording unchanged; the blocking call stays
    for existing callers. Guest (orange): search.Client.Stream with no
    per-attempt body bound; a host-side search table keyed by desk-session
    key, its streams cancelled in release and reclaimLocked; SessionFrame
    carries arrivals since the session's cursor; Begin merges them into a
    persistent, lock-guarded worker store that End does not clear; verbs
    choir.SearchStart(q) (returns a stub such as "s3 started: 4 providers"
    at once), choir.Searches.Get/List (copies only, never a live map or
    slice), and choir.SearchWait(id, ms) (blocks up to ms, merges arrivals,
    returns counts). The cell result gets one count line per search with
    new arrivals ("s3: 3/4 answered, +2 new"); that line is the only push.
    Slice (b), research desk. Module set: SearchStart/Searches/SearchWait,
    Read (fetch into a capture; replaces FetchURL, ImportURL,
    ImportDocument and the selector reads), Report (one way to tell
    Texture; carries R3's final flag and a stop reason), Research
    (sub-RLM), Ask, Reply, Note, plus observation verbs (Help, Context,
    Updates). Nothing retired is listed. A core prompt of about 1.5k
    characters (study C §4.3, revised to these decisions): report as
    soon as understanding changes, and a cell may report and search again;
    claims labelled confirmed, reported, inferred, speculative or refuted,
    with calibrated speculation welcome and no claim quotas; effort follows
    uncertainty times stakes in the Cast; stop at diminishing returns
    relative to the request with a final Report; "M of N search providers
    active" wording; the general-hypothesis seed list (decisions §3.1)
    offered as a space to sample from, not rules. Sub-researchers get
    their own short prompt and their own surface (read verbs, async
    search), not the parent's. The research run ends when its final
    Report commits; if the model ends its turn without one, the run ends
    and the thread stays open with no reminder, and a Texture follow-up
    starts a new activation on the open thread.
  acceptance:
    - "gateway: one stream request on staging returns provider lines in latency order (Keenable first, ChatGPT last or absent) and a final line with M of N; the blocking route answers as before"
    - "research-only demo on a fresh staging test computer (AI news, one Texture Cast carrying hypotheses, uncertainties and the omission hypothesis), run twice: per-provider arrivals visible as rising count lines in the cell trace read through the product API"
    - "sources reaching Texture: distinct cited sources in Texture's final revision above both demo baselines (10, 11); claims and rejected updates reported against 13/15 and 5-of-11/5-of-10; a sample of 10 cited claims checked against their sources by someone other than research"
    - "time to first research Report, from research run start, no worse than the baseline (about 70 s to 2 min) with status-labelled claims in it"
    - "research closes its own thread: a final Report with a stop reason that names the Cast's hypotheses; no continuation-only Texture controls in the document events"
    - "no search outlives its caller: every search started in the run reaches N/N or is cancelled with its session (cancelled count in the release notice); no stream request at the gateway outlives provider timeout plus margin"
    - "no result attributed to the wrong query: each arrival in the store carries its search id and query, and R1's request-id correlation test still passes"
  rollback: >-
    git revert per slice. The gateway stream is additive: reverting it
    leaves the blocking route. The guest verbs are additive; reverting the
    research surface and prompt restores the R1 surface. The search table
    and store are process memory, so nothing durable needs draining.
  landing:
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    minimize the gap between what research learns and what reaches Texture
    as sourced, status-labelled claims bearing on the Cast's hypotheses, and
    the time research spends waiting on the slowest provider or on Texture,
    while a crash still closes work, Texture stays the sole writer, and the
    runtime holds no budget, cap or research job
  goodharting_would_be: >-
    many shallow Reports (snippets relabelled "reported", one claim each);
    source counts inflated by search-result URLs nobody read; every claim
    hedged "speculative" so none can be wrong; an early final Report with a
    boilerplate stop reason; count lines that change while the model never
    reads the store; a "closed" thread Texture then has to re-open for what
    was asked
  realism_axis: >-
    gateway stream probed directly on staging -> one research cell starting
    searches and reading arrivals on a staging test computer -> research-only
    demo with one hypothesis-carrying Cast -> the same with sub-researchers
    -> two repeated demos compared with demo19/demo20 and read by the owner
  mutation_class: >-
    red for the gateway streaming mode (gateway/provider calls) and for the
    research run ending on its final Report (run lifecycle); orange for the
    guest client, host table, worker store, verbs and module set; green or
    yellow for the prompts
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §1, §3, §4, §5 (owner-agreed 2026-10-12)
    - CLAUDE.md Restarts End Work, Mutation Classes, Landing Loop
  must_preserve:
    - a crash closes work; nothing resumes without a planned-restart marker (the store and table are process memory; no boot-time resumer)
    - Texture is the sole writer of document revisions; research reaches it only through records
    - research has read-only world authority; one level of fan-out (PD1)
    - the blocking search route and its callers
    - no budgets, round caps, reminders or runtime research job (decisions §1.2, §4)
    - nothing built on the 60-minute activation limit (decisions §1.5)
  excluded:
    - async sub-RLMs and child run events (study C stage 3; a named residual)
    - runtime scoring and catch-all resolution (decisions §9; P4)
    - Texture prompt redesign (R5) beyond the Cast help text this station needs
    - the capture block model and citation checking (R6)
    - push of arrivals into a running cell or waking a desk on arrival (needs the transport rewrite)
  protected_surfaces:
    - internal/gateway/handlers.go HandleSearch, internal/gateway/searchplane/router.go, internal/gateway/search_plane.go
    - internal/agentcore/runtime.go research tool-loop options (run end)
    - internal/yaegikernel session frame, ChoirScope binding and broker actions
  read_only_dependencies: [internal/textureowner, internal/actorruntime, internal/coagentpacket]
  admissible_evidence: >-
    staging only (choir.news, fresh test computer): run records, cell
    traces and document events through the product API, Texture revisions,
    a third-party claim check; local race tests shape the store but prove
    nothing about staging
  heresy_delta:
    discovered: [cross-wired parallel choir.* calls (already documented), children leave no run events (study C §2)]
    introduced: [none intended; risk is a research thread that never closes, accepted by owner decision]
    repaired: [search waits on the slowest provider, research cannot end its own work, child prompt lists verbs it cannot call]
  conjecture_delta: >-
    from "research needs a runtime-owned job with budgets" (root-cause
    record, rejected) to "a truthful RLM with async search closes its own
    work at diminishing returns" (R4-1)
  standing_questions: >-
    provenance: owner decisions 2026-10-12. Conformance: no new store; Read
    writes to the capture store choir.ReadCapture already uses. Deletion
    citers: every verb removed from the research surface is grepped across
    prompts, code, tests and docs in the same commit. Single authority: the
    host table owns in-flight streams; the worker store is the model's
    copy; on disagreement the host wins and the next frame corrects it.
    Artifact: the demo's Texture revision and records, not narration.
    Fate-sharing: a search shares fate with the host process and the
    gateway, by intent. Restart: both die with the process; the run is
    closed at boot; nothing to resume. No-SSH: acceptance reads cell
    traces through the product API; gateway journal lines only corroborate.

now:
  status: drafted
  slice: written for the owner's review with the metamission; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: R4-1
    claim: >-
      If no research search waits on the slowest provider and research
      reports as its understanding changes with epistemic status, then
      more sourced claims reach Texture than in demo19/20 without Texture
      driving rounds, and research closes its own thread at a point the
      owner judges reasonable for the request.
    test: two research-only staging demos against the demo19/20 baselines, then the owner reads the Texture
    edge: missing_oracle
    delta_o: >-
      cell traces readable through the product API (the root-cause record
      could not read them); without them, "the model never read the store"
      and "the store was empty" look the same
    scope_if_supported: AI news research on one document with one hypothesis-carrying Cast
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Open, with the conservative default taken: (1) no wake on arrival
      after end_turn (study A's in-memory park waiter) — arrivals wait in
      the store for the next activation; revisit only if traces show lost
      arrivals; (2) research and child surfaces drop the blocking
      WebSearch once SearchStart lands, so one search verb is described;
      (3) Reports tag hypotheses with R3's fields if R3 lands them, else as
      a claim-text convention on the existing packet claim
      (internal/types/evidence.go:75-81), with no new schema here; (4) the
      Cast hypothesis help text for Texture lands here unless R3 carried it.
    kind: design
    status: proposed
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §4-5; study A §4b; study C §4.5, §5.4
  belief:
    believed_state: the router already completes per provider and the guest link already streams; the store pattern is probed clean; research's lifecycle depends on R3
    main_uncertainty: whether the model reads arrivals and reports without being pushed, or ends its turn with searches pending
    next_observation: pre-flight on the then-current main, confirming which R1 deletions and R3 record fields landed
  blocker_or_risk: depends on R1 (both slices) and R3 (slice b's thread closing); gateway change is red
  next_action: owner reviews this file with the metamission; after R1 lands, a direct staging probe of the gateway stream design against today's router

receipts: []
---

# R4: async search and the research desk

Research today is a short activation that Texture must restart, waits on
the slowest search provider, and cannot close its own work. The owner
decided (2026-10-12) that intelligence lives in the models: research is a
simple RLM that searches, reads, reports and reduces uncertainty until more
work stops paying for itself, and search never makes it wait. This station
builds the two pieces that decision needs: search that returns at once and
fills REPL state per provider, and a research desk whose prompt and surface
describe exactly that.

R1 has already removed the stale text, the `update_coagent` leftovers, the
caps and budgets, and has fixed or fenced the cross-wire bug; R3 gives
Reports a final flag and threads an id. This station does not repeat them.
Its pre-flight confirms they landed.

Ways it can fail, written before the code:

- arrivals cross the socketpair while a cell is waiting and the store
  races (only worker code writes, under its lock, in Begin or inside
  SearchWait; never a model goroutine; race-tested);
- a model goroutine calls a network verb and gets another query's results
  (R1 correlates by request id; concurrency stays on the host anyway, and
  the prompt teaches no goroutine pattern);
- a search keeps running after its session ends, or the gateway stream
  hangs (streams bound to the session lease, cancelled in release and
  reclaim; gateway deadline is provider timeout plus margin);
- the 30 s guest client cuts a long stream (the stream client has no
  per-attempt body bound; refused-dial retry kept);
- the count line floods the context or turns into a payload push (one line
  per search with new arrivals, counts only);
- the model ends its turn with searches pending and the arrivals are never
  read (the store persists for the session; the next activation's first
  count line shows them; a trace shows whether this happens before any
  wake is added);
- Texture churns, one turn per research Report (consume-at-commit batching
  folds pending Reports; measure Texture turns per Report in the demo);
- research reports many thin Reports, hedges everything, or closes early
  to look decisive (the goodharting list; the owner reads the Texture and
  a third party checks a sample of claims);
- research never closes (no breaker by owner decision; Cancel remains the
  stop; recorded as a residual if seen);
- sub-researchers still see the parent's verbs (their own prompt and
  surface, checked by a test that renders both);
- the one-hour broker handle (choir.go:119) expires under a long-lived
  session and every verb is refused (unconfirmed hypothesis from the
  cross-wire record; checked in the demo, documented before any fix);
- "M of N" counts cooling providers as failed and research blames search
  (N is the eligible set at stream start; cooling providers listed
  separately, never called degraded).
