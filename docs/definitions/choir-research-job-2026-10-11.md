---
definition_version: 4

# RJ — the runtime-owned research job. Replaces research "rounds" (one short,
# model-ended activation per Texture control) with one job per Texture Cast
# that the runtime keeps running until coverage stops growing or a budget
# runs out, a lead ledger below the model, and unconfirmed leads streamed to
# Texture as ReportPackets. Successor to PD1's research loop
# (definitions/choir-pd1-persistent-research-desk-2026-10-11.md, receipt S1j).
# Root cause: docs/problems/research-desk-structural-flaw-root-cause-2026-10-11.md.
# Research-desk half of docs/search-plane-design-fast-deep-broad-2026-10-11.md
# (§5.4 R1-R6). Owner 2026-10-11: no prompt tuning until the failure is
# understood; research must be fast, iterative, deep, broad, recursive and
# comprehensive; unconfirmed findings go to Texture right away, labeled
# unconfirmed, then are confirmed or refuted; sensible inference; all
# inter-agent communication through precommitment records; big red substrate
# changes wait for owner review.
readiness: drafted

review:
  reviewer: "none yet (owner review first: red substrate change reserved by the owner)"
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T22:45:00Z
  source:
    canonical_ref: main 3e277431
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean apart from this draft
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Research cannot run without a fresh Texture control. Provider entry
      for a lifecycle research run (admitLifecycleResearchProviderEntry,
      internal/agentcore/runtime.go:3007-3216) requires an exact Texture
      control fingerprint delivered to that run (runtime.go:3100-3105,
      3179-3191). Generic open-work recovery deliberately never starts a
      lifecycle research run ("Lifecycle Research are activated only by the
      exact Texture-control fingerprint reconciler", runtime.go:2880-2884).
      Admission runs once, at activation start (executeActivation,
      runtime.go:3297), not per model call.
    - >-
      A research run ends when the model ends its turn. The tool loop
      returns on end_turn after the inbox checkpoint, the completion guard
      and the park waiter (internal/toolregistry/toolloop.go:898-1001).
      Research runs get neither hook: the completion guard is wired for
      engineering only (runtime.go:3488), and the park waiter only for
      actor_park_on_idle runs or persistent management under a Texture
      control (coagentParkWaiter, internal/agentcore/management_controller.go:2714-2722).
      A completion guard that keeps asking past maxCompletionGuardRetries
      (2) makes the run error (toolloop.go:121, 924-925). The park waiter's
      Continue needs injected update turns (toolloop.go:977-983), bounded
      by maxToolLoopParkResumes = 16 (toolloop.go:123).
    - >-
      One activation may last 60 minutes (DefaultActivationBudget,
      internal/provideriface/config.go:24). Observed research runs lasted 67
      to 126 seconds (root-cause record, 11 of 11 runs), so the activation
      budget is not what ends them.
    - >-
      The activation preamble for a research run says "Before ending this
      activation, use choir.Report to assert work_disposition=completed ...
      otherwise use choir.Report with work_disposition=open and a precise
      blocker" (buildAssignedWorkItemPromptForCarrier,
      internal/agentcore/runtime.go:2957).
    - >-
      Texture bounds research controls at four per owner request
      (textureResearchRoundsPerOwnerRequest,
      internal/textureowner/texture_turn_runtime.go:161), counted as
      research control_queued events since the latest owner input
      (researchRoundsSinceOwnerInput, :187-203). Extras are dropped (:300-307).
      Texture's Cast, Ask and Note to research all fold into an
      open_researcher control (foldResearchDispatch,
      internal/agentcore/texture_dispatch_fold.go:98-121).
    - >-
      Every cell world verb carries a leftover checkpoint instruction. The
      cell path reuses the registry tool funcs through HostEgress
      (internal/researchtools/host_egress.go:22-33, wired at
      internal/agentcore/tools_desk.go:303-326).
      shouldRequireResearchUpdateAfterTool (internal/researchtools/researchtools.go:717-737)
      looks for successful events named update_coagent, web_search,
      fetch_url and so on; on the cell path the event's tool name is the
      call name (internal/toolregistry/batch_executor.go:71, 116), which is
      desk_go_eval. Both sequence numbers are 0, so it returns true on every
      call. The search projection then adds "Submit a concise first
      update_coagent source packet ... Include 2-4 grounded claims"
      (researchtools.go:923-925). Fetch, source search, import and read
      results add the same kind of line (researchtools.go:324, 412, 500, 548,
      621, 783-788, 994-996, 1022-1024). An outage adds "Report a precise
      blocker" (:922). The web_search tool description still teaches the
      update_coagent cadence (:662). Tests pin the instruction
      (internal/researchtools/researchtools_test.go:291, 348-349, 373-374).
      Whether the text reaches the model in a cell result is unconfirmed: no
      research cell trace has been read (root-cause record, next observation 1).
    - >-
      The search projection says "one or more providers failed or were
      unavailable" whenever any provider attempt failed, even when the
      merged target was met (researchtools.go:883-895, 926-928).
    - >-
      Egress is charged per activation: the ledger key is the run id
      (egressKey, internal/researchtools/egress_budget.go:113-125), capped at
      DefaultResearchEgressMaxCalls = 64 (:46). Children charge the parent's
      key (childExec.RunID = researchBudgetKey(parent),
      internal/agentcore/research_children.go:187). Children run at most 4
      at once (:23), with a computer-wide 1.5 s model-call spacer (:36) and
      a 10-minute window (:44), and cannot fan out (the child module set has
      no Research, internal/yaegikernel/choir.go:263-265).
    - >-
      Search results reach the host tool in full: up to 40 merged results,
      each with title, url, snippet, provider and an optional published_at
      (researchtools.go:656, 688, 861-879). Nothing stores them outside the
      run's trace and the model's context.
    - >-
      Two existing stores could hold leads. The evidence store
      (agent_evidence in the computer's own Dolt, internal/store/texture.go:151-164,
      evidence_id PRIMARY KEY; CreateEvidence :2462, ListEvidenceByAgent
      :2500; verbs SaveEvidence and ListEvidence,
      internal/yaegikernel/choir.go:616, 630) is per owner and per agent,
      with free-form kind and metadata. Its object-graph variants
      (CreateEvidenceOG, ListEvidenceByAgentOG, internal/store/graph_store.go:2710, 2755)
      exist but have no callers. The shared capture store
      (choir.web_capture, internal/objectgraph/web_capture.go:13-40, written
      by sourcegraph.WriteCaptures, internal/sourcegraph/captures.go:45-120)
      is global ("readable by every computer", internal/yaegikernel/surface.go:61-62),
      keyed per polled item, immutable, and requires a body in the body store.
    - >-
      A source in a packet can already say it is unconfirmed:
      evidence.state accepts pending and evidence.confidence low, medium or
      high (internal/coagentpacket/packet.go:170-171, 236-241). Claims carry
      a stance (supports, qualifies, contradicts, background; :168).
    - >-
      Texture's consume path takes every pending producer report addressed
      to it whose seq is within the run's scheduled seq, and defaults the
      unnamed ones to delivered at commit (textureTurnPendingInbound,
      internal/textureowner/texture_turn_runtime.go:85-150). Reports that
      arrive while a Texture run is live do not start another run: the
      wake returns while the active run's authority is valid
      (reconcileAgentWakeLocked, internal/textureowner/texture_controller.go:550-556),
      so they batch into the next turn.
    - >-
      Precedent for a runtime-owned, crash-closed schedule: the Autopaper
      source schedule (internal/agentcore/source_schedule.go:19-120,
      202-235) is a chain of durable source_poll_due wakes, in
      restartWorkKinds (internal/actorruntime/handler.go:105-115), closed as
      "interrupted by a restart" on a markerless boot.
    - >-
      A markerless boot passivates live runs with reason runtime_restarted
      (passivateInterruptedActivations, internal/agentcore/runtime.go:2382-2405),
      and the actor handler refuses pre-boot work occurrences as
      interrupted_by_restart (internal/actorruntime/handler.go:93, 103,
      117-126, 169).
    - >-
      The gateway allows each computer 60 searches a minute in a bucket
      separate from inference (internal/gateway/handlers.go:870-889,
      internal/gateway/ratelimit.go:12-16). Research runs on gpt-5.6-luna at
      low effort (internal/modelpolicy/model_policy.go:340-343).
    - >-
      The research prompt (internal/promptstore/defaults/research.yaml,
      version 6) already asks for this behavior: work in rounds and keep
      going after each, send promising leads as soon as found marked
      unconfirmed and then confirm or refute them, make sensible
      inferences, stop when another round would not change the picture. The
      runtime implements none of it.

finish:
  deliver: >-
    When Texture casts a research question, research works on it as one
    job until new findings stop arriving, or until its time, search or
    model budget runs out, without Texture having to restart it. Every
    search result lands in a lead ledger the model can read. Within about
    a minute of the first searches, Texture receives the strongest
    unconfirmed leads as a ReportPacket with each source marked pending.
    Later packets confirm or refute those same sources by id. Texture can
    steer a running job with Note or Cast, and stop it with CancelAct. A
    crash ends the job, and nothing resumes it.
  artifact: >-
    Slice RJ0, observation (no code). Capture one research cell trace
    through the guest's internal run-events route on a staging test
    computer before RJ1 deploys. Record whether the cell result carries
    next_instruction, whether it carries the doubled
    durable_output/model_output envelope, and what the model's next cell
    does. This settles cause 3 of the root-cause record and is the
    "before" for RJ1.

    Slice RJ1, the runtime stops telling research to stop (orange).
    Delete shouldRequireResearchUpdateAfterTool,
    addResearchUpdateCheckpointRequirement, the requireFindingsCheckpoint
    parameter and every next_instruction line in
    internal/researchtools/researchtools.go (the five checkpoint call sites,
    the three projection branches and the outage line), plus the
    update_coagent cadence sentence in the web_search description. Rewrite
    the tests that pin them so they assert that no next_instruction is
    present. Show gateway_status only when a search missed its merged
    target or was an outage. Keep search_outage, code and provider_health
    as data. Prompt files do not change. This is a deletion and a
    discriminator, not the fix. Its acceptance claims no coverage gain.

    Slice RJ2, the lead ledger (red: evidence write path). After a
    successful web_search, when the run is a research run, the host tool
    writes each result as one evidence row of kind research_lead. The
    host tool is the same func the cell, the tool path and children all
    reach. Row fields: evidence_id = hash(document, canonical URL), so a
    second sighting is a primary-key repeat; agent research:<doc>;
    source_uri the canonical URL; title; content the snippet (400
    characters at most). Metadata holds published_at when the provider
    gave one, the queries and providers that returned it, the best rank,
    the run id, first_seen_at, and sightings. A repeat sighting appends a
    research_lead_seen row (id = hash(lead id, run id, query)) rather than
    updating the lead row; it never fails the search. A write error is
    logged and counted, and the search result is returned unchanged. A new
    read-only verb, choir.Leads(filter), on research only, returns typed
    rows: id, url, title, published_at, snippet, queries, providers,
    sightings, state, flushed. Lead state is derived, never stored on the
    lead row: pending until research's own ReportPacket cites the lead's
    id or URL; then confirmed (state available and a claim with stance
    supports), refuted (stance contradicts), or likely or unlikely (state
    pending with confidence medium or low). One store query lists
    research_lead and research_lead_seen rows for an agent since a time,
    using the existing (owner_id, agent_id, created_at) index.

    Slice RJ3, the job (red: run lifecycle). A lifecycle research run
    opened by a Texture control becomes a job. A continuation hook is added
    beside the completion guard and the park waiter in the tool loop. When
    the model ends its turn and the job's stop rule has not fired, the hook
    appends a job card as a user turn and the same run continues. The
    model's turn ends a pass, not the job, so provider admission,
    the control fingerprint and the PD2 session are unchanged. The hook's
    bound is the job's pass cap, not maxCompletionGuardRetries. When the
    job stops, the run ends normally. The job card is state only: pass
    number, budget left, stop-rule counters, new leads since the last pass
    (counts and the top ten ids), Texture's notes injected since the last
    pass, and the leads still pending. It carries no exhortation, and a
    test pins its fields. The stop rule, checked at every end of turn, in
    order:
    (a) the job's work item is no longer open (research Reported
    completed, or the work closed some other way);
    (b) the opening Cast was cancelled or retracted (CancelAct) or the
    trajectory is cancelled;
    (c) budget: 20 minutes of wall time, 300 egress calls, 80 model calls
    (WithToolLoopBudget), 16 passes;
    (d) saturation: after at least 3 passes, the last 2 passes added fewer
    than 3 new reportable leads;
    (e) two passes in a row ran no cell.
    A reportable lead is one seen by at least two distinct queries, or
    ranked in the top three of some query, or cited by the model in a
    ReportPacket. For research job runs, the egress ledger takes a per-key
    limit (the job's 300) in place of the ledger-wide 64. Children keep
    charging the root's key. The activation preamble line at
    runtime.go:2957 becomes, for job runs, a factual statement: the
    runtime continues this job until its stop rule fires, a Report with
    work_disposition=completed ends it, and findings go to Texture by
    ReportPacket. Its demand for a status Report before ending is removed.
    research.yaml does not change. When the job stops, the runtime stages
    one final ReportPacket (see RJ4) that names the stop reason, counts
    leads by state, and lists the pending leads with their best reading.
    A Texture Cast, Ask or Note to research while the job's run is live
    reaches that run at its next inbox checkpoint, as any bound control
    does. It counts against Texture's cap, starts no second run, and
    resets the saturation counter. After the job stops, a Texture Cast
    within the cap opens a new job on a new run, as today.

    Slice RJ4, unconfirmed leads stream to Texture (red: packets to
    Texture). At the end of each research cell in a job, the host checks
    two conditions: reportable, unflushed leads exist, and either this is
    the job's first flush or at least 60 seconds have passed since the
    job's last packet to Texture (the model's or the runtime's). If both
    hold, the host stages one ReportPacket intent into that cell's durable
    tray before the tray commits, marked staged_by runtime_lead_flush. It
    commits through the same report act path as a model-authored
    ReportPacket (one record, one producer_report packet to the
    document's Texture), and a replay reads it from the tray. Packet
    shape: kind evidence_update; summary "N unconfirmed leads from
    research searches, not yet read"; one source per lead, with source_id
    = the lead id, kind web_source, target {uri, title}, excerpt
    "unconfirmed; published <date or unknown>; seen in <k> searches:
    <snippet>", evidence.state pending, evidence.confidence low; no claims;
    work_disposition open. At most 10 leads per flush, at most 2 per
    domain, ranked by sightings and then by recency, and at most 80
    runtime-flushed leads per job. The model's own ReportPacket remains
    the way to send relevance-filtered leads, inferences and resolutions.
    A resolution cites the lead's id as its source_id, so Texture can
    match it to the unconfirmed item it already wrote. Texture's consume
    path, prompt and wake do not change: packets that arrive during a
    Texture turn batch into its next turn.

    Slice RJ5, conditional: parallel search within a cell (orange).
    Build it only if RJ3 and RJ4 evidence shows serial search is the
    binding constraint (fewer than 6 searches per active minute).
    choir.SearchMany(queries) runs up to 4 single searches at once from
    the host over the existing gateway endpoint. Each is charged, ledgered
    and rate-limited as today, so it needs no gateway change. The
    gateway's batch query and async lane stay in the search-plane station.
  acceptance:
    - RJ0 receipt, a saved research cell trace answering the three questions in RJ0
    - RJ1 unit tests, no researchtools projection (search, fetch, source search, import, read, selector, outage) contains next_instruction; gateway_status appears only below target or on outage
    - RJ1 deployed, one research-only AI news request on a fresh test computer. Measured, with no gain claimed, are the claims per sourced packet, seconds from first search to first report, and run length; the cell trace shows no next_instruction
    - RJ2 store and tool tests, a search writes one row per distinct canonical URL; the same URL from a second query writes a seen row and no second lead; a child's search writes under the root run; a failing evidence write still returns the search result; choir.Leads derives confirmed, refuted, likely and pending from research's ReportPackets
    - RJ2 deployed, lead rows match the gateway's logged result counts for the demo's searches (within dedup), and median search latency at the guest rises by at most 100 ms
    - RJ3 loop tests, a job run continues past end of turn with the job card; stops on each rule (a) to (e) with that reason recorded; the 17th pass never starts; the hook never errors the run the way the completion guard's retry cap does; a Texture Note mid-job is injected into the live run and starts no second run; a CancelAct of the opening Cast stops the job at its next end of turn
    - RJ3 crash test, a markerless boot during a job leaves its run passivated runtime_restarted and never re-dispatched; nothing in this station runs at boot
    - RJ3 prompt test, the job card holds only the pinned state fields; research.yaml is byte-identical
    - RJ4 tests, the flush is staged into the tray and replays byte-identically; at most 10 leads per flush and 2 per domain; no flush before 60 s except the first; the final packet names the stop reason; Texture's consume path takes a runtime-flushed packet exactly as a model-authored one
    - RJ6 deployed acceptance against docs/evidence/ai-news-haiku-baseline-2026-10-10/report.md (26 merged items, 43 links, window 3-10 October). Run "Research the news on and around AI for the last week." plus the standard follow-up on a fresh test computer, with the search deadline ladder landed if it has. A scripted audit joins the run, the ledger, the records and the revisions, and reports each target below
    - RJ6 target, one research run per opening Cast, and no research run ends with its work open unless its stop reason is (b), (c) or (d)
    - RJ6 target, the first unconfirmed lead appears in a revision within 3 minutes of the prompt (S1d's first grounded revision took 3m51s)
    - RJ6 target, at least 6 searches per active research minute (demo19 1.1)
    - RJ6 target, at least 20 distinct dated items in the 3-10 October window in the final document, each with a link, plus 10 sampled links per side checked by hand for the date and claim they support
    - RJ6 target, every unconfirmed item in the document is labeled so; at least half of the flushed leads end the job confirmed, refuted or likely/unlikely with a reason
    - RJ6 target, zero gateway search 429s, and the stop reason is recorded for each job
    - RJ6 counts, Texture turns per job, model calls per job, and status-only packets per job are recorded as counts, not targets
  rollback: >-
    git revert of the station's slices in reverse order (RJ5, RJ4, RJ3,
    RJ2, RJ1). No drain is needed. Runtime-flushed packets already
    delivered are ordinary producer reports. Lead rows are inert evidence
    rows of their own kinds that nothing reads after RJ2 is reverted
    (named residual: retention). Reverting RJ3 returns research to
    model-ended runs restarted by Texture under the four-control cap.
    Reverting RJ1 restores the checkpoint instruction.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between the research-only AI news document and the
    Haiku baseline in correctly dated, linked, in-window items, and the
    time to the first visible (unconfirmed) lead, with Texture issuing no
    restarts, while preserving research's read-only authority, Texture as
    sole writer, records as the only channel between desks, the computer's
    model and search budgets, and crash closes work.
  goodharting_would_be: >-
    Pouring search cards into the document as "unconfirmed" items, so the
    count rises while most never resolve and the brief becomes a link dump.
    Counting distinct URLs as items when several are the same story.
    Hitting "saturation" by repeating near-identical queries. Raising the
    budget until coverage rises and calling that the fix. Calling a job
    "complete" when it actually stopped on budget.
  realism_axis: >-
    unit tests with a fake search client -> RJ1 alone on one staging
    request -> ledger only (RJ2) -> one job on one research-only request
    (RJ3) -> the request plus the follow-up with streamed leads (RJ4, RJ6)
    -> two documents running jobs at once on one computer (shared spacer,
    gateway bucket and model account) -> Autopaper W2 consuming confirmed
    leads (later, not this station)
  mutation_class: >-
    red (run lifecycle and tool-loop continuation for research, evidence
    write path, packets to Texture, egress budget). RJ1 and RJ5 alone are
    orange.
  authority_sources:
    - owner direction 2026-10-11 (header) and the root-cause record's recommendation
    - docs/precommitment-protocol-design-2026-10-11.md (all inter-agent traffic as records)
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)"
    - docs/search-plane-design-fast-deep-broad-2026-10-11.md §5.4
  must_preserve:
    - research's read-only world authority; the ledger is research's own working state on this computer, never a world write
    - Texture is the sole writer; it hears from research only through ReportPacket records, never by reading the ledger
    - provider admission for lifecycle research unchanged (one exact Texture control per run)
    - crash restart closes the job; no boot path resumes or flushes anything
    - Texture's consume-at-commit, wake, breaker and delivery budget unchanged
    - research.yaml and Texture's prompts unchanged (no prompt tuning)
    - the never-idle bound - at most four research Casts per owner request, each job bounded, so a request spends at most four job budgets
    - the computer-wide child model-call spacer and the gateway's per-computer search bucket
  excluded:
    - gateway changes (deadline ladder, batch queries, async lane, latency-aware waves) - search-plane stations
    - research children at depth 2; recursion here means query-frontier expansion inside one job (owner 2026-10-11 against recursive agent fan-out)
    - mandatory predictions and the count resolver (P3, P4); the job only records what the count resolver would read
    - research model or effort changes (root-cause next observation 3, after RJ4)
    - writing leads to the shared capture store (Autopaper bridge later)
    - gating status-only research reports in the reducer (measured in RJ6 first)
    - a durable flush timer independent of cell boundaries (added only if RJ6 shows gaps over 120 s)
  protected_surfaces:
    - internal/toolregistry/toolloop.go (continuation hook)
    - internal/agentcore/runtime.go (job wiring for research runs; activation preamble at :2957)
    - internal/agentcore/research_children.go (budget key)
    - internal/researchtools/researchtools.go, egress_budget.go
    - internal/store/texture.go (evidence query)
    - internal/yaegikernel/choir.go, surface.go (choir.Leads; choir.SearchMany in RJ5)
    - internal/agentcore/rlm_reduce.go (the runtime-staged ReportPacket in the research tray)
  read_only_dependencies:
    - internal/textureowner/texture_turn_runtime.go (consume path, cap)
    - internal/textureowner/texture_controller.go (wake batching)
    - internal/coagentpacket/packet.go (evidence state and stance)
    - internal/promptstore/defaults/research.yaml
    - docs/definitions/choir-search-deadline-ladder-2026-10-11.md (lands independently; RJ6 measures cleaner after it)

now:
  status: drafted
  slice: authored from the root-cause record and the code for the owner's review; nothing built
  source_ref: 3e277431
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: RJ-1
    claim: >-
      If one runtime-owned job per Texture Cast keeps research running
      until saturation or budget, writes every search result to a lead
      ledger, and streams the strongest unconfirmed leads to Texture at
      cell boundaries, then the research-only AI news document reaches at
      least 20 dated, linked items in the window within 20 minutes, with
      the first lead visible within 3 minutes. No prompt changes and no
      gateway changes are needed for this.
    test: RJ6 scripted audit plus 10 sampled link checks per side
    edge: >-
      missing_oracle. No scripted item comparison exists yet, and model
      effort (gpt-5.6-luna at low) may cap what a longer run produces.
      That second edge is frame_lock if the audit cannot tell a starved
      model from a starved loop.
    delta_o: >-
      the RJ6 audit script (ledger rows, records, revisions and the
      baseline's items joined by URL and date), and RJ0's cell trace as
      the before-picture
    scope_if_supported: research-only requests on one computer, one document at a time
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      The job is the research run itself, kept alive by a runtime
      continuation hook, not a new chain of runs re-activated by the
      runtime as the root-cause sketch proposed. A multi-run job would need
      provider admission without a Texture control: either a forged control
      or a second admission authority in admitLifecycleResearchProviderEntry
      (runtime.go:3007-3216), the most guarded path for research. In one
      run, admission, the control fingerprint, the session and the crash
      path stay as they are. The price: the job is bounded by the
      60-minute activation budget and by one run's context (memory hooks
      compact, runtime.go:3470), and steering arrives only at inbox
      checkpoints. Lead ledger is the evidence store, not shared captures.
      The reasons:
      (1) leads are this owner's private research on this computer; the
      capture store is global, readable by every computer, and writing an
      owner's query results into it is a world write research does not
      hold;
      (2) captures need a body, and a 400-character snippet would put thin
      bodies into Autopaper's corpus;
      (3) captures are keyed per polled feed item and immutable, while a
      lead gathers sightings and a derived state per document;
      (4) every search would add about 40 cross-host writes through corpusd.
      The evidence store is local, already a research verb surface, and
      its primary key gives dedup.
      Leads are flushed by the runtime as well as by the model. This is
      the root-cause record's open question 3, answered yes. The model's
      voluntary report cadence is the failure being removed, so the
      runtime guarantees time to first lead and cadence, while the model
      still sends relevance-filtered leads, inferences and resolutions.
      Texture's four-control cap keeps its value but changes meaning: it
      now bounds Texture's openings and steering Casts per owner request,
      not research's progress. Passes are bounded by the job budget, and a
      request by four job budgets. RJ1 lands alone first. The root-cause
      record preferred landing it with the loop change; landing it alone
      measures cause 3 by itself and claims no fix.
    kind: architecture
    status: proposed (owner review)
    evidence_ref: this file's start.observed
    alternatives_rejected: >-
      multi-run job with a job-record admission authority (larger red
      surface, kept as the successor if one run's context or the 60-minute
      budget binds); shared captures as the ledger (above); the unwired
      object-graph evidence path (CreateEvidenceOG) instead of the SQL
      table (a storage migration this station does not need; named
      residual); a durable flush timer (deferred, see excluded)
  belief:
    believed_state: >-
      Search delivers. Research's unit of work is a model-ended activation
      that only a Texture control can start, and a leftover instruction
      tells it to report after every search. Leads have no path except the
      model's packets.
    main_uncertainty: >-
      Whether, once the loop is the runtime's, a low-effort model fills a
      20-minute job with useful queries and resolutions, or the job
      saturates early on repeats. Also whether runtime-flushed leads help
      Texture or bury it.
    next_observation: RJ0, one research cell trace on staging
  blocker_or_risk: >-
    Red substrate change, which the owner reserved for review. A longer
    research run spends more of the ChatGPT account that already hits
    usage limits (problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md).
    The 80-model-call budget bounds this, and RJ6 records it.
  next_action: >-
    owner review of this file (budgets, runtime-flushed leads, the evidence
    store as ledger, the cap's new meaning, RJ1 alone first); then RJ0,
    which is read-only and may run before review

receipts: []
---

# RJ — the runtime-owned research job

**Summary.** Research has no loop. Each "round" is one research run that
the model ends after its first report. Only a new Texture control can start
the next round, at most four per request, and the runtime requires that
control: provider admission for lifecycle research checks for an exact
Texture control delivered to the run (runtime.go:3007-3216). The smallest
change that removes this keeps the research run alive itself. A
continuation hook in the tool loop gives the model a job card instead of
letting the run end, and stops on a rule the runtime checks: work closed,
Cast cancelled, budget spent, coverage saturated, or idle passes. Below
the model, every search result lands in a deduplicated lead ledger in the
evidence store. At cell boundaries the runtime sends Texture the strongest
unconfirmed leads as an ordinary ReportPacket with pending sources. The
model confirms or refutes them later by citing the same source ids. The
leftover "report after every search" instruction is deleted first, alone,
so its effect is measured. Nothing in Texture's consume path, provider
admission, the gateway or the prompts changes.

**How sure I am.** High on the mechanism of today's failure: it is in the
code (admission, the tool loop's end-of-turn path, the checkpoint guard)
and in eleven runs on staging. Medium that one long run is enough. Context
growth and model effort could bind before the budget does, which is why
the multi-run design is kept as the named successor. Low on the stop-rule
constants (3 passes, 3 reportable leads, 60 s, 10 per flush). They are
predictions for RJ6 to measure, not tuned values.

**What I recommend.** Approve the shape and the budgets, or change them,
then run RJ0 (read-only) and land RJ1 alone. RJ2 to RJ4 follow in order,
each with its own staging evidence, and RJ6 is the comparison with the
baseline.

## Why this order proves something at each step

- **RJ0** shows whether the checkpoint text reaches the model at all.
- **RJ1** removes it alone. If runs get much longer, cause 3 was a larger
  share of the early stop than the record believed. If they do not, cause 1
  (no loop) is confirmed as the binding cause.
- **RJ2** changes no behavior. It proves the ledger sees what the gateway
  returned, at an acceptable latency cost.
- **RJ3** proves research keeps working without Texture, and records why
  each job stopped.
- **RJ4** proves the time to the first visible lead, and that leads
  resolve.
- **RJ5** is built only if search seriality shows up as the bottleneck
  once the loop exists.

## Ways it can fail, written before the code

- The hook keeps a run alive with nothing to do, so empty passes burn model
  calls. Rule (e) stops two cell-less passes in a row, saturation stops dry
  ones, and the 80-call budget stops the rest.
- The hook inherits the completion guard's retry cap and errors the run on
  pass three. The hook is a separate option with its own bound, and a test
  runs it past three passes.
- Context grows across passes until turns slow down or overflow. The memory
  hooks compact; RJ6 records input tokens per turn; the model-call budget
  ends the job.
- The job ignores the owner's change of mind. Each end of turn checks the
  opening Cast and the trajectory, and a CancelAct stops the job at its
  next pass (tested).
- A Texture follow-up during a live job starts a second research run on the
  same document. The control binds to the live run and is injected at its
  inbox checkpoint (tested). This is a hypothesis about today's binding
  path (management_controller.go:3325-3341) to confirm in RJ3's tests.
- A crash mid-job resumes it. The job adds no boot path. The run is
  passivated runtime_restarted and never re-dispatched at a markerless boot
  (tested). Unverified today: whether the research work item itself shows a
  visible "interrupted by a restart" fate. RJ3's crash test reads it, and a
  gap is recorded as a problem before any fix.
- Ledger writes slow every search, or a write error fails the search.
  Writes are batched per search, a failure is logged and never returned,
  and the added latency is measured (100 ms ceiling).
- A replay writes duplicate leads. Ids are deterministic and the primary
  key refuses repeats.
- Children's searches escape the ledger. Children run under the root's run
  id (research_children.go:187), and the host tool writes for them
  (tested).
- The runtime flush buries Texture in noise. It sends 10 per flush, 2 per
  domain, 80 per job, and only reportable leads. RJ6 records how many
  flushed leads Texture used, and the cap is a constant to change, not a
  prompt.
- Flushes wake Texture so often that its turns pile up. Packets that
  arrive during a Texture turn batch into the next turn (texture_controller.go:550-556).
  RJ6 counts Texture turns per job, and the breaker and delivery budget are
  unchanged.
- A replay re-derives the flush from a later ledger and commits different
  bytes. The flush is staged into the durable tray before commit and is
  never derived at commit (tested).
- An unconfirmed lead reads as fact in the document. The source is marked
  pending with low confidence and the excerpt starts "unconfirmed". RJ6
  checks the labels in the final revision.
- Leads never resolve, and the brief fills with open items. The final
  packet lists pending leads with the model's best reading, and RJ6
  requires half resolved. Below that, the next move is a shift (who
  confirms), not a larger flush.
- Saturation never fires, because each new query returns new URLs.
  Novelty counts only reportable leads (corroborated, top-ranked or cited),
  not raw URLs, and the budget backstops it. Stop reasons are recorded.
- Saturation fires too early, on a thin first pass. It needs at least 3
  passes.
- A larger egress budget turns into runaway cost. It is per job, and at
  most four jobs per owner request. The gateway's 60 searches a minute per
  computer still applies.
- The job spends the ChatGPT account that management also needs. The
  model-call budget bounds it, and RJ6 records calls per job.
- Deleting next_instruction removes something a test really guarded. Read
  each pinning test first. The outage case keeps its data fields.
- Status-only reports keep waking Texture. The job no longer asks for them
  (the preamble changes), and RJ6 counts them. A reducer gate is a later
  slice if they persist.
- The job card turns into prompt tuning by another name. A test pins it to
  state fields, and research.yaml stays byte-identical.
- The job hides that it stopped on budget rather than coverage. The final
  packet and the run metadata name the stop reason.
- Two documents' jobs on one computer starve each other. They share the
  child spacer, the gateway bucket and the model account. This is the next
  rung of the realism axis, measured there.
- The ledger grows without bound, at about 1,000 rows per job. Retention is
  a named residual.

## Open questions for the owner

1. **Budgets.** Should one job get 20 minutes, 300 network calls, 80 model
   calls and 16 passes, with at most four jobs per owner request? The
   baseline used about 250 to 300 calls for one weekly-news request.
2. **Runtime-sent leads.** May the runtime send unconfirmed leads to
   Texture without a model step (marked pending, 10 at a time), alongside
   the model's own reports?
3. **Ledger store.** Should the lead ledger be the per-computer evidence
   store, as proposed, rather than the shared capture store? Confirmed
   leads can be bridged to captures later for Autopaper.
4. **One run or many.** Is one long run per job acceptable as the first
   version, with the multi-run job (a new admission authority) as the
   successor if context or the 60-minute budget binds?
5. **RJ1 alone.** Should the checkpoint deletion land alone first as a
   measurement, or with RJ3 as the root-cause record suggested?
6. **The cap.** Texture's four research Casts per request keep their value
   but now bound openings and steering, not research's progress. Is that
   the bound you want?
