---
definition_version: 4

# PD1 — one persistent research desk per computer. Station of
# definitions/choir-persistent-desks-metamission-2026-10-11.md; supersedes
# definitions/choir-research-fanout-2026-10-10.md (its receipts S1, S1b and
# its baseline carry over). Owner 2026-10-11: "We shouldn't have multiple
# research desks for a task ... The one research desk should fan out if
# needed"; "we really shouldn't be encouraging recursive fan out ... models
# can and should do a round of search and then report back and then do
# another round of search ... because we want to get the data to the texture
# agent as soon as possible"; "desks are persistent per computer".
readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686 (pushed) + c566f330, 7bcc87eb (local)
    deploy_identity: staging choir.news b3dbc686
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean; two local commits ahead of origin
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Texture's turn opens research with agent ID research:<id derived from
      the Texture run, tool call and control index> and a fresh work item
      (textureTurnControls, internal/textureowner/texture_turn_runtime.go):
      a new research agent per opener. Budget: two openers per owner
      request (textureResearchBudgetPerOwnerRequest), extras dropped
      silently (problems/texture-research-opener-dropped-silently-2026-10-11.md).
    - >-
      Store target validation (internal/store/lifecycle_texture_target.go)
      requires a research target with LifecycleVersion > 0, an existing
      target work item, and, when the research agent has an active run, that
      run binding the target work. Persistent management instead is
      management:<owner>, ensured by EnsurePersistentManagementAgent, with a
      new work item per request.
    - >-
      7bcc87eb (local): any turn of a document's Texture may follow up on
      research its earlier turns opened.
    - >-
      Texture's run prompt (internal/textureprompts/overlays/run_system.yaml)
      tells Texture to "use parallel research desks when branches are
      distinct" and to open a new research entry for work an earlier turn
      opened.
    - >-
      Research children (choir.Research) can call choir.Research; S1b showed
      recursion overrunning the computer's model budget. 3faa001a paces
      children; c1da4e97 carves time; b3dbc686 softens the child preamble.

finish:
  deliver: >-
    When the owner asks a research question, Texture sends it to the
    computer's one research desk. The desk works in rounds — search, read,
    report what landed, next round on what is still open — reporting to
    Texture after each round, and widens with choir.Research children only
    when a question splits into separate lines. Texture's follow-up
    questions, from any turn, go to the same desk.
  artifact: >-
    research:<computer> as the only research agent on a computer; Texture
    controls open a work item on it (first time per request) or follow up on
    that work item (every later time); research children cannot spawn
    children; prompts for Texture and research say so.
  acceptance:
    - action: >-
        Focused tests: a second open_researcher in one request becomes a
        follow-up to the request's research work; two requests on two
        documents target the same research agent; a research child has no
        choir.Research; the target validator accepts a control for a second
        work item while the desk is busy with the first (queued, not
        refused).
      proves: identity and routing on the code path
      evidence_class: local tests (each fails on the prior code)
    - action: >-
        S1c on staging: "Research the news on and around AI for the last
        week." on a fresh document of demo-arxiv-2026-10-10c. Then one
        follow-up question typed by the harness into the same document.
      proves: >-
        one research agent for the request and the follow-up; reports per
        round; item count, dates and links against
        evidence/ai-news-haiku-baseline-2026-10-10/report.md; time to first
        grounded revision; no gateway rate-limit storm.
      evidence_class: deployed proof (test computer) with traces
  rollback: >-
    Revert the PD1 commits in reverse order; per-opener research agents
    return. Pacing, the time carve-out and the store follow-up rule are
    independent and may stay.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize research agents per computer (target 1) and the gap to the
    Haiku baseline in correctly dated, cited items, and the time to first
    grounded revision, while preserving research read-only authority,
    Texture sole writer, and the computer's model budget.
  goodharting_would_be: >-
    One agent ID whose runs never see earlier work; reports every minute
    that repeat themselves; counting items without checking dates and links.

homotopy:
  realism_axis: >-
    one request on one document -> follow-up on the same document -> two
    documents sharing the desk -> (PD2) the desk remembers across runs.

boundaries:
  mutation_class: red (Texture control path, lifecycle target validation)
  authority_sources:
    - owner statements 2026-10-11 (header and metamission)
    - docs/choir-doctrine.md I26
  must_preserve:
    - research read-only world and message authority
    - Texture sole writer; per-document base-revision check
    - only this document's Texture directs research work it opened
    - the per-request loop bound of problems/texture-research-loop-never-idles-2026-10-09.md, as a bound on research rounds
    - crash restart closes work
  excluded:
    - desk sessions across runs (PD2)
    - Texture root (PD3)
  protected_surfaces:
    - internal/textureowner texture_turn_runtime (controls)
    - internal/store lifecycle_texture_target (validation)

now:
  status: working
  slice: >-
    prompt hill-climb stopped by the owner (2026-10-11 evening: understand
    before tuning); root cause found (receipt S1j): research has no loop.
    The next change is structural (a runtime-owned research job), red, and
    waits for the owner's review
  source_ref: ba32e0cb (root-cause document pushed)
  deploy_identity: staging b172895b
  candidate:
    id: PD1
    state: landing
    ref: main
    base: 78c9173e
    digest: 4ca9b094, 74227bcf
    scope: [internal/textureowner, internal/store, internal/agentcore, internal/yaegikernel, internal/promptstore, internal/textureprompts]
  conjecture:
    id: PD1-1
    claim: >-
      If the research desk is one persistent agent that reports per round
      and receives follow-ups, the research-only document reaches the Haiku
      baseline's in-window item count with correct dates and links within
      20 minutes, revising every few minutes.
    test: S1c traces and the item comparison
    edge: missing_oracle
    delta_o: scripted item comparison plus 10 sampled link checks per side
    scope_if_supported: research-only requests on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Landing scope is one research desk per document (research:<doc>), not
      yet per computer. The store's target rule ties research work to the
      Texture of the document that opened it; a computer-wide desk serving
      several documents' Textures needs a routing owner, which is PD3's
      Texture root. Per-computer research lifts with PD3. Conservative
      option, recorded under No Blocking Asks.
    kind: scope
    status: settled (agent, reversible)
    evidence_ref: 2ac20789 commit message; internal/store/lifecycle_texture_target.go
    owner_ratification_ref: not_applicable (narrows, does not contradict, the owner's per-computer direction; lift is scheduled)
  belief:
    believed_state: >-
      One research desk per document serves the request and the follow-up
      in rounds (S1c-S1e). Coverage is now lost after research, not in it:
      a refused report hid by a bare call, a full replace erasing sources,
      and Texture hedging every item (S1e).
    main_uncertainty: >-
      Whether, with refusals visible and sources kept, the brief reaches
      the baseline's item count with a citation per item, or whether the
      remaining gap is Texture's own selection.
    next_observation: S1f — items, citations per item, unconfirmed leads followed up
  blocker_or_risk: >-
    the fix is a red substrate change (a runtime-owned research job with a
    lead ledger and streaming unconfirmed leads), which the owner reserved
    for review; no further prompt tuning until then
  next_action: >-
    owner reviews docs/archive/problems/research-desk-structural-flaw-root-cause-2026-10-11.md
    and docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md; then design
    the research job as its own station

receipts:
  - id: S1j
    at: 2026-10-11T21:45Z
    kind: root-cause analysis from staging traces and code (no prompt change), docs/archive/problems/research-desk-structural-flaw-root-cause-2026-10-11.md
    result: >-
      The owner stopped hill-climbing until the failure is understood. Two
      deployed AI news demos (test computers demo-p2a-2026-10-11 and
      demo-p2c-2026-10-11) and the code show: every research round is one
      short run (67 to 126 seconds, 2 to 10 searches) that ends within
      seconds of its first report, and only a Texture turn starts the next,
      capped at four per request, so research was idle about half the time;
      prompts v3 to v6 said keep going and runs still ended. A leftover
      instruction keyed on deleted tool names
      (shouldRequireResearchUpdateAfterTool, internal/researchtools)
      tells research on every cell search to submit a 2-4 claim packet
      first (confirmed in code). Reports carry only claims backed by an
      imported page, so unconfirmed leads have no path (one demo turned
      over 1,000 result cards into 10 sources). Search returned full result
      sets; one search in ten ties the guest's 30-second deadline
      (docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md). This
      explains S1c to S1i's plateau better than any prompt difference.
  - id: S1c
    at: 2026-10-11T02:57Z
    deploy: staging 16d7c21a, test computer demo-arxiv-2026-10-10c
    kind: deployed proof (traces in session scratchpad s1c-1..s1c-final)
    result: >-
      Identity and routing hold: one research agent
      (research:630ce665-...) served the request, the harness follow-up
      (02:11:30) and four rounds (6939c9b3, 00080325, ff5ffe73, 5732f8f7),
      each 1-3 minutes. Earlier demos had two research agents per request.
      Each round closed its work as completed, and Texture opened the next.
      Coverage misses the conjecture: the final revision (02:37) has about
      1,000 words and 9 source links against the baseline's 26 items, with
      two overlapping. Causes from the traces: five of seven search
      providers were cooling down (24h benches from S1b;
      problems/search-rate-limit-benches-provider-for-a-day-2026-10-11.md).
      Texture spent 87 cells, 34 of them failed, mostly probing ApplyTexture
      and guessing block ids it could not see
      (problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md).
      First grounded revision came at 02:12:54, 6 minutes after the prompt
      and 4.4 minutes after research reported.
    conjecture: PD1-1 refuted on coverage; identity and follow-up claims supported
  - id: S1d
    at: 2026-10-11T04:06Z
    deploy: staging c713d63a (+ Keenable key on Node B), test computer demo-arxiv-2026-10-10c
    kind: deployed proof (traces in session scratchpad s1d-1, s1d-final)
    result: >-
      Same prompt and follow-up, with the S1c fixes deployed (outline in the
      cell, frame boundary, ApplyTexture help, one-minute rate-limit bench,
      Keenable, PD2). One research agent (research:596395c7-...) served
      eight rounds across the request and the follow-up. First grounded
      revision came at 3m51s (S1c: 6m), then a revision every 1-3 minutes;
      13 revisions. Texture: 12 turns, 66 cells, 7 errors, one block-id or
      probe error (S1c: 87 cells, 34 errors, 11 not-found). Keenable active
      and succeeding beside Parallel and Tavily; zero gateway 429s. The
      final document has about 1,700 words and 20 dated in-window mentions,
      with a section listing contested and unverified reports. That is still
      short of the baseline's 26 items: the Choir brief withholds items it
      could not trace to a primary or wire source, which the baseline
      asserts (for example GPT-6 in ChatGPT).
    conjecture: PD1-1 still short on coverage within 20 minutes; latency and cadence supported
  - id: S1e
    at: 2026-10-11T09:40Z
    deploy: staging 78c9173e (research prompt v3, ChatGPT search, shared ChatGPT auth), test computer demo-arxiv-2026-10-10c
    kind: deployed proof (traces in session scratchpad demo13/t2, demo13/final)
    result: >-
      Same prompt and follow-up. One research agent served six rounds
      (40 cells, 12 errors); Texture took seven turns (50 cells, 6 errors);
      first grounded revision at 4m12s; final revision at 09:20. ChatGPT
      search returned 17-18 results per call beside Keenable, Parallel and
      Tavily, and ChatGPT inference had no errors. Unconfirmed labels
      appear, and a round-one lead (the Manus raise) was confirmed in round
      five by CNBC and Reuters. The output is still thin: about 1,130 words,
      nine items, three citations. The traces show three causes, none in
      the prompt's intent. (1) Round one's ReportPacket was refused, and
      the bare call hid the error ("=> "), so the round's funding and
      compute leads never reached Texture
      (problems/desk-verb-error-vanishes-as-final-expression-2026-10-11.md).
      (2) A full-replace apply erased the document's sources, and later
      revisions re-cited only three
      (problems/texture-full-replace-erases-citations-2026-10-11.md).
      (3) Texture hedged every item, even ones two outlets reported, and
      round one told Texture to hold leads back. Fixed in 4ca9b094 (kernel
      shows refusals; research prompt v4; Texture states confirmed claims
      plainly) and 74227bcf (a full replace keeps the sources).
    conjecture: PD1-1 still short on coverage; follow-up confirmation of leads supported once
  - id: S1f
    at: 2026-10-11T10:47Z
    deploy: staging c1d21d5e (refusals shown in cells, research prompt v4, a full replace keeps sources), test computer demo-arxiv-2026-10-10c
    kind: deployed proof (traces in session scratchpad demo14/final)
    result: >-
      Same prompt and follow-up, but the harness sent the follow-up after
      four quiet minutes, before research's first round reported, so this
      run is not like-for-like with S1e. Three research rounds (23 cells, 5
      errors; three verb refusals now visible in the cell, two web search
      timeouts and one 404 import); Texture 43 cells, 9 errors. The final
      brief has 618 words, 7 cited sources, all in the body (S1e: 3 in
      about 1,100 words). It states confirmed items plainly, attributes
      single-source items once, and holds two leads open without
      boilerplate. It is short because search failed: guest searches timed
      out at 30 s while ChatGPT search (median 23 s) held every wave it
      joined
      (problems/search-wave-waits-for-its-slowest-provider-2026-10-11.md,
      fixed in 9c1c96c7).
    conjecture: PD1-1 still short on coverage; citation and voice fixes supported
  - id: S1g
    at: 2026-10-11T12:15Z
    deploy: staging 19b2f964 (gateway 9c1c96c7 wave fix), test computer demo-arxiv-2026-10-10c restarted onto it
    kind: deployed proof (traces in session scratchpad demo15/trace)
    result: >-
      Same prompt and follow-up; the follow-up waited for eight quiet
      minutes and went after research's first grounded revision (11:40,
      six minutes in). Six research runs (38 cells; 24 web searches, 7
      imports, 2 child fan-outs); no search wave timed out. The final brief
      has 790 words and 7 cited sources (S1f 618 and 7; the baseline 5,765
      words). GPT-6 moved from unconfirmed to confirmed once research found
      OpenAI's announcement through search after a direct import got 403.
      Claude Haiku 5.5 stayed unconfirmed although the baseline confirms
      it. Policy coverage is one item (baseline seven). Two of seven search
      providers are out of quota (Serper until 2026-10-15, SerpAPI until
      2026-10-12), Exa fails to parse responses and SearXNG returns empty,
      so waves ran on Tavily, Parallel and Keenable. One research child hit
      its 90-second budget; one cell failed to compile a long literal.
      Texture's caveats ("noted once, pending verification", "still being
      confirmed") recur on unconfirmed items.
    conjecture: PD1-1 still short on coverage; the wave fix removed timeouts but not the gap
  - id: S1h
    at: 2026-10-11T14:54Z
    deploy: staging ebe02b17 on the reader computer (research prompt v5, Texture caveat line)
    kind: deployed proof (traces in session scratchpad demo16/trace; final revision demo16/ainews/revisions/12)
    result: >-
      Same prompt and follow-up. Eight research runs (13:59 to 14:30), about
      53 web searches in cell code (S1g 24) and 3 fetches. The final brief
      has 839 words and 10 cited sources (S1g 790 and 7; baseline 5,765).
      Claude Haiku 5.5 is now confirmed from Anthropic's own page; the stock
      caveats are gone; claims are stated in their sources' terms ("TechCrunch
      reported", "the announcement says"). GPT-6, confirmed in S1g, is
      absent. Policy is one item, and the follow-up's policy, chips and
      funding threads end in a "still open" list. The trace shows why:
      Texture's questions to research after round one were confirmation
      rounds ("confirm the Oct 4 task-force lead", "dated primary records
      for the remaining gaps"), and research spent them re-confirming the
      same few items (the Haiku 5.5 claim reported six times, the task-force
      lead four). The run_system line "ask research to confirm it" sends
      every lead back for confirmation while research rounds per request
      are bounded, so confirmation crowds out coverage.
    conjecture: PD1-1 still short on coverage; the binding constraint moved from search capacity to how Texture spends research rounds
  - id: S1i
    at: 2026-10-11T16:03Z
    deploy: staging 25c8b5b0 (research prompt v6; Texture coverage-before-confirmation line); reader computer restarted onto it at 15:12:41Z
    kind: deployed proof (traces in session scratchpad demo17/trace; final revision demo17/ainews/revisions/09)
    result: >-
      Same prompt and follow-up. Texture's second question to research
      asked it to "open the four branches the document does not cover
      yet"; six research rounds ran about 59 web searches. The brief (834
      words) now covers five branches with about sixteen dated items:
      model releases (Mistral Large 4, Claude Haiku 5.5 confirmed), policy
      (a New York City Council hearing, H.R. 10701, a reported copyright
      suit, an EU AI Office check), chips and power, three funding rounds
      and three arXiv papers. Coverage roughly doubled against S1h. But
      only 4 sources are cited (S1h 10): most items name their outlet
      ("CNBC reported") without a citation. Hypothesis, not yet checked in
      the trace: research reported search-result items without importing
      the pages, so Texture had no source to cite. GPT-6, the Genesis
      Mission and ThinkingBox, found in S1g and S1h, are absent, so runs
      still sample the week differently. The closing paragraph explains
      search gaps to the reader.
    conjecture: >-
      PD1-1 coverage improved by changing how Texture spends rounds; the
      next gap is citation (importing what is reported). Prompt
      hill-climbing pauses here: the owner's precommitment-records protocol
      (docs/precommitment-records.md) is the measured
      replacement for reading these traces by hand
---

# PD1 — persistent research desk

Texture should talk to one research desk per computer, the way it already
talks to one management desk. The desk works in rounds and reports after
each round, so Texture can publish early and often. Children are the desk's
way to go wide on a question that splits. They do the work themselves and
cannot spawn children of their own. Follow-ups from any Texture turn go back
to the same desk.
