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
  slice: S1c — deployed proof of one research desk across a request and a follow-up
  source_ref: 16d7c21a (pushed)
  deploy_identity: staging deploying 16d7c21a
  candidate:
    id: PD1
    state: landing
    ref: main
    base: b3dbc686
    digest: 7bcc87eb, 2ac20789, 8b350515, 16d7c21a (+ ce27ae0c harness, local)
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
      Texture turns merge openers into one desk per document; later openers
      become follow-ups on the request's open desk work; rounds per owner
      request are bounded at 4 with a visible drop note; the store queues a
      control for a busy desk instead of refusing it; children cannot fan out;
      prompts say one desk, rounds, report after each.
    main_uncertainty: >-
      Whether the queued follow-up is delivered to the desk when its current
      run ends (wakeUpdatedCoagent path) on the product path, and whether the
      model actually reports per round.
    next_observation: S1c trace — every research control targets research:<doc>; follow-up run exists
  blocker_or_risk: none
  next_action: deploy the fixes S1c found (outline in cell, frame boundary, rate-limit bench, Keenable, PD2) and rerun as S1d

receipts:
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
---

# PD1 — persistent research desk

Texture should talk to one research desk per computer, the way it already
talks to one management desk. The desk works in rounds and reports after
each round, so Texture can publish early and often. Children are the desk's
way to go wide on a question that splits. They do the work themselves and
cannot spawn children of their own. Follow-ups from any Texture turn go back
to the same desk.
