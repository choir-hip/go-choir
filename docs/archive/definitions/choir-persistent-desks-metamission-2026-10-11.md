---
definition_version: 4

# Persistent desks: each computer has exactly four persistent root desks
# (Texture, management, engineering, research) that scale horizontally
# inside themselves and hold a continuing conversation with each other.
# Owner, 2026-10-11 (overnight mission, ~01:25Z): "We shouldn't have
# multiple research desks for a task ... The one research desk should fan
# out if needed, but Texture shouldn't be managing multiple research
# desks"; "The whole point is to have persistent desks so that for any
# follow-ups, texture is going back to the research desk, which already has
# done work and has Go cells. So it's like a continued conversation between
# agents"; "we have four persistent desks and ... the desks themselves can
# scale horizontally, but they don't need to worry about other desks'
# operations on the inside"; "desks are persistent per computer. And once
# we get everything working in the same computer setting, we will enable
# one autoputer to control another. And the second autoputer will have its
# own four desks." The owner accepted the three-step plan and the Texture
# root proposal ("this all sounds great. Write all of these docs. Then get
# to work ... the overnight mission").
readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: PD1
      path: docs/definitions/choir-pd1-persistent-research-desk-2026-10-11.md
      readiness: executable
      status: working
      depends_on: []
    - id: PD2
      path: docs/definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md
      readiness: executable
      status: working
      depends_on: [PD1]
    - id: PD3
      path: docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md
      readiness: drafted
      status: pending
      depends_on: [PD2]
    - id: PD4
      path: docs/definitions/choir-pd4-computer-controls-computer-2026-10-11.md
      readiness: intent
      status: pending
      depends_on: [PD3]

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686 (pushed) + 7bcc87eb (local, store follow-up rule)
    deploy_identity: staging choir.news b3dbc686
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean apart from local commits c566f330, 7bcc87eb ahead of origin
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Desk identities in code: management:<owner> (one, persistent);
      texture:<doc> (one per document); engineering:<doc> plus one worker
      per assignment (engineering:<assignment>); research:<id derived from
      the Texture turn, tool call and control index> — a new research agent
      for every open_researcher (internal/textureowner/texture_turn_runtime.go).
    - >-
      A desk's Go session (yaegi worker) is keyed by run ID
      (deskWorkerActivationID, internal/agentcore/tools_desk.go): every run
      starts a fresh REPL, for every desk. What persists across a desk's
      turns is its agent identity and the ledger, not its variables.
    - >-
      Doctrine I26 (docs/choir-doctrine.md): "Each computer has four
      persistent root RLM desks — management, engineering, research, and
      Texture"; engineering already has root + per-assignment sub-RLMs.
    - >-
      Research-only demos S1 and S1b (definitions/choir-research-fanout-2026-10-10.md
      receipts): Texture opened two research desks per request; a later
      Texture turn could not follow up on an earlier turn's desk; recursive
      choir.Research overran the computer's 60-calls-a-minute model budget.

finish:
  deliver: >-
    On one computer, the owner's requests flow through four persistent
    desks that remember their work: Texture sends a research question to the
    one research desk, gets reports round by round, and sends follow-ups to
    the same desk, which continues from its earlier work; Texture itself is
    one root desk with a sub-RLM per document and writes a master document
    that shows the owner every document, open commitment and desk.
  artifact: >-
    PD1-PD3 landed on staging with deployed acceptance; PD4 shaped as a
    goal file for computer-controls-computer once single-computer desks
    work.
  acceptance:
    - action: each station's own acceptance, on staging
      proves: the station's claim on the product path
      evidence_class: deployed proof (test computer)
    - action: >-
        The research-only prompt and one follow-up question on the same
        document, after PD1-PD2: the follow-up reaches the same research
        agent, whose cell reads a value its earlier run defined.
      proves: continued conversation between persistent desks
      evidence_class: deployed proof (test computer) with trace
  rollback: per station; each station reverts independently in order PD3, PD2, PD1
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of desk identities and fresh contexts a single
    owner request creates (target: one per desk role per computer, reused
    across requests), and the time from request to first grounded revision,
    while preserving Texture as sole writer, research read-only, management
    admission of engineering, and crash-restart closes work.
  goodharting_would_be: >-
    One agent ID that is reused while its context still starts empty each
    turn; a "master document" that is a static index nobody reads; reports
    that arrive sooner but carry no new evidence.

homotopy:
  realism_axis: >-
    one research desk per computer on one document -> follow-ups that reach
    the same desk -> desk sessions that survive across its runs -> Texture
    root with per-document sub-RLMs and a master document -> a second
    computer with its own four desks, controlled by the first.

boundaries:
  mutation_class: red (Texture control path, desk lifecycle, provider calls)
  authority_sources:
    - owner statements quoted in the header (2026-10-11)
    - docs/choir-doctrine.md I26
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It"
  must_preserve:
    - Texture is the sole writer of document text; per-document base-revision checks
    - research has read-only world and message authority
    - management alone admits engineering
    - a crash restart never resumes work; planned-restart marker only
    - the computer's model budget (gateway per-computer limit)
  excluded:
    - cross-computer control before PD4
    - changing management's admission rules
  protected_surfaces:
    - Texture control and commit path (textureowner, store lifecycle target validation)
    - desk worker lifecycle (agentcore tools_desk)
    - gateway/provider calls

now:
  status: working
  slice: PD1 — persistent research desk per computer, rounds not recursion
  source_ref: c713d63a
  deploy_identity: staging c713d63a
  candidate:
    id: PD1
    state: landing
    ref: main
    base: b3dbc686
    digest: none
    scope: [internal/textureowner, internal/store, internal/agentcore, internal/yaegikernel, internal/promptstore, internal/textureprompts]
  conjecture:
    id: PDM-1
    claim: >-
      If each computer has one persistent desk per role that keeps its
      context across requests, Texture's research turns stop multiplying
      desks and the research-only document reaches baseline coverage sooner
      than with per-request desks.
    test: >-
      S1c on staging: one research agent across the request and its
      follow-ups; item count, dates and links against the Haiku baseline;
      time to first grounded revision.
    edge: missing_oracle
    delta_o: >-
      Trace check that every research control of the trajectory targets the
      same agent ID; scripted item comparison (scratchpad compare_items.py).
    scope_if_supported: research-only requests on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Research desk keyed by computer, like the doctrine's root desks;
      Texture root with per-document sub-RLMs (PD3) after desk sessions
      outlive runs (PD2).
    kind: architecture
    status: settled
    evidence_ref: owner statements 2026-10-11 (header)
    owner_ratification_ref: owner 2026-10-11 ~01:25Z "this all sounds great ... the overnight mission"
  belief:
    believed_state: >-
      PD1 pushed (16d7c21a): one research desk per document, follow-ups from
      any Texture turn, busy desk queues controls, no child recursion,
      prompts for rounds. Per-computer research lifts with PD3 (see PD1
      decision). Sessions are still per run (PD2).
    main_uncertainty: whether queued follow-ups reach the busy desk on the product path
    next_observation: S1c trace
  blocker_or_risk: none
  next_action: S1d on staging (c713d63a) proves PD2 and checks the S1c fixes; then PD3 trajectory-spanning agent design

receipts: []
---

# Persistent desks (overnight mission, 2026-10-11)

Choir's doctrine says each computer has four persistent root desks. In code,
only management is one desk per computer. Texture is one per document,
research is a new desk for every request Texture makes, and no desk keeps its
Go session from one run to the next. The research-only demos showed the cost:
Texture managed several research desks and could not send a follow-up to the
desk that had already done the work.

This metamission makes the four desks real, in order:

1. **PD1, persistent research desk.** One research desk per computer.
   Texture's requests and follow-ups go to it. It works in rounds and
   reports after each one. Children (`choir.Research`) are its horizontal
   scaling and cannot spawn children of their own.
2. **PD2, desk sessions outlive runs.** A desk's Go session survives
   between its runs, so a follow-up continues the earlier conversation. A
   crash restart still ends all work.
3. **PD3, Texture root desk.** One Texture desk per computer, with a
   sub-RLM per document that is that document's sole writer. The root
   writes a master document: the owner's supervision surface over every
   document, open commitment and desk.
4. **PD4, computer controls computer (intent).** Once one computer's desks
   work, one computer can drive another, which has its own four desks.
