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
  slice: design reconciliation — how a persistent research agent takes new work while busy
  source_ref: 7bcc87eb (local)
  deploy_identity: staging b3dbc686
  candidate:
    id: none
    state: none
    ref: main
    base: b3dbc686
    digest: none
    scope: []
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
    what: research desk keyed by computer ID (doctrine I26; owner "desks are persistent per computer")
    kind: architecture
    status: settled
    evidence_ref: owner statement 2026-10-11
    owner_ratification_ref: owner 2026-10-11 ~01:25Z
  belief:
    believed_state: >-
      Identity, the second-opener drop, recursion and the prompts are the
      known changes; follow-up authority across turns is done locally.
    main_uncertainty: >-
      How lifecycle delivery starts a research run for a work item while the
      same agent has an active run bound to another work item.
    next_observation: read the lifecycle control delivery / dispatch path for research targets
  blocker_or_risk: none
  next_action: read the research control delivery and dispatch path; then write the tests listed in acceptance

receipts: []
---

# PD1 — persistent research desk

Texture should talk to one research desk per computer, the way it already
talks to one management desk. The desk works in rounds and reports after
each round, so Texture can publish early and often. Children are the desk's
way to go wide on a question that splits. They do the work themselves and
cannot spawn children of their own. Follow-ups from any Texture turn go back
to the same desk.
