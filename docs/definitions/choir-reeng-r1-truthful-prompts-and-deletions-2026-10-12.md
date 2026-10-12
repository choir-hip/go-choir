---
definition_version: 4

# R1 — truthful prompts and deletions. First station of
# definitions/choir-reeng-metamission-2026-10-12.md; decisions
# docs/re-engineering-decisions-2026-10-12.md §1 (affordance invariant,
# intelligence in models, prefer deletion), §4 (sweep), §9 (Assign).
readiness: drafted

review:
  reviewer: none yet (agentic-consensus file review before execution)
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
      Every desk's prompt describes affordances that do not exist or omits
      ones that do (study G §1: management M1-M16, engineering E1-E10,
      Texture T1-T7, research R1-R6, cross-desk X1-X4; study D §2: 26
      Texture mismatches; study C §1: research). Prompts are hand-written;
      only the generated surface list is built from the real verbs
      (tool_profiles.go:568, surface.go:211), and the parity test
      (choir_parity_test.go:310) checks one direction.
    - >-
      Every research search result carries a runtime-written
      next_instruction naming the deleted update_coagent tool, gated by a
      check on deleted tool names so it always fires
      (internal/researchtools/researchtools.go:717-738, 787, 924, 995, 1023).
      update_coagent also survives in live wake text
      (coagent_update_packet.go:294-305, trajectory.go:114, runtime.go:1556).
    - >-
      Live wake text tells management and research to set work_item_id and
      work_disposition on choir.Report, which the API cannot express and the
      reducer rejects (runtime.go:2957-2963; tools_engineering_assignment.go:67-72).
    - >-
      choir.Assign is an in-memory stub that returns "dispatched" and
      persists nothing (broker.go:356-377). Message, Emit and Emits are
      retired stubs still in module sets and the generated surface
      (choir.go:440-445, surface.go:36-41).
    - >-
      Parallel choir.* calls from one cell's goroutines get each other's
      results (problems/parallel-choir-calls-cross-wire-results-2026-10-12.md),
      and the research prompt recommends parallelism
      (rlm_research_runtime.yaml:25).
    - >-
      Runtime caps the owner rejected: the research round cap of 4
      (texture_turn_runtime.go:155-187, 295-305, 405, and prompt text
      tool_profiles.go:252, run_system.yaml:10, 31); Texture and engineering
      tool-loop budgets (runtime.go:4105-4152); the Texture breaker
      (texture_controller.go:623-633).

finish:
  deliver: >-
    Every desk is told the truth about what it can do. Dead names, stubs
    and caps the owner rejected are gone, and a test keeps prompts and the
    real surface from drifting apart again.
  artifact: >-
    One series of commits:
    (1) prompt truth pass for management, engineering and research, and the
    cross-desk text (study G row 1; study C §1's false research claims: "keep
    going", "import to make citable", "use parallelism"), describing the
    true completion path (ReportPacket work_disposition) until R3 replaces
    it; Texture gets only deletions of dead and retired items, since R5
    rewrites its prompt;
    (2) delete choir.Assign, and Message, Emit and Emits from every module
    set and the reducer's IntentMessage arm (study G §3, row 2);
    (3) prune module sets to intent (engineering loses Cast and Ask,
    research loses Cast) with a reverse parity test: every export is taught
    or explicitly listed, every taught verb and addressing rule exists
    (row 3);
    (4) delete the next_instruction injection, the legacy management tool
    registrations and every update_coagent name in prompts, tools, code
    and tests, plus dead prompt builders (buildPersistentManagementUpdatePrompt,
    buildCoagentBacklogPrompt, buildAssignedWorkItemPrompt,
    buildTextureMediaSourceResearchObjective) (rows 1, 4; X2-X3; T6);
    (5) delete the research round cap and its prompt text, the Texture and
    engineering tool-loop budgets and the Texture breaker (row 5; decisions
    §1.2, §1.4). The 60-minute run limit stays as a labelled prototyping
    safety constraint;
    (6) fix the cross-wire: egressRoundTrip correlates responses by
    request id (or, minimum, holds one lock across write and read), with
    the study's probe promoted to a regular test;
    (7) research-facing text says "M of N search providers active", never
    "degraded" for a single provider failure.
  acceptance:
    - reverse parity test passes in CI and fails on a planted stale verb (shown once, then removed)
    - "grep gate: no update_coagent, choir.Assign, choir.Message, choir.Emit, work_item_id-on-Report, open_researcher or open_persistent_super in prompts, overlays, tool descriptions or live wake text"
    - concurrency test (study A probe) returns each caller its own result
    - "staging: one AI news demo on a fresh test computer runs end to end with no stale-name errors in the guest trace; its metrics (cells and model-seconds per Texture turn, research reports, sources reaching Texture, time to first revision) are recorded as the baseline R4 and R5 compare against"
  rollback: git revert per commit; no persisted state changes (Assign persisted nothing)
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between what each desk is told it can do and what it
    can do, and the runtime rules the owner rejected, while preserving
    every authority boundary and the restart rule.
  goodharting_would_be: >-
    Deleting words until the parity test passes while the desks lose
    instructions they need; or annotating stale text ("deprecated") instead
    of deleting it.

homotopy:
  realism_axis: >-
    parity and grep tests -> focused unit tests per deletion -> staging
    demo on a fresh test computer with the guest trace read

boundaries:
  mutation_class: orange (prompts, module sets, guest runtime caps, cell transport); rollback git revert
  authority_sources:
    - owner 2026-10-12 (re-engineering decisions; "update co-agent is exactly the kind of thing that we really need to sweep")
  must_preserve:
    - Texture is the only writer of a Texture
    - engineering never addresses Texture (enforced structurally by the module-set prune)
    - a crash restart never resumes work
    - the 60-minute run limit (prototyping safety constraint, labelled as such)
    - SSRF and address guards in sourcefetch
  excluded:
    - the research egress budget ledger (gateway-adjacent, red; moves to R4)
    - Texture's prompt rewrite and write interface (R5)
    - management relay and pass-through, path B, the Jev judge, the self-development state machine (R7)
    - the restart recast (R2)
  protected_surfaces: [internal/runtimeprompts, internal/promptstore/defaults, internal/yaegikernel, internal/researchtools, internal/agentcore (prompt builders, tool-loop budgets), internal/textureowner (round cap only)]
  read_only_dependencies: [internal/store, Texture canonical write path]

now:
  status: drafted
  slice: authored for file review; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: R1-1
    claim: >-
      Most of research's early stopping and Texture's extra cells are caused
      by prompts that describe a different system and by runtime caps, so
      truthful prompts and the deletions alone measurably change the demo
      (more research reports reaching Texture, fewer failed cells), before
      any redesign.
    test: the staging demo after R1 against the P2a-deployed and P2c-deployed receipts
    edge: missing_oracle
    delta_o: guest cell traces are not in Node B's journal; the demo must export the guest trace
    scope_if_supported: AI news demo on one test computer
    status: proposed
    evidence_refs: []
  decision: {what: "R1 fixes Texture only by deletion; its prompt rewrite is R5", kind: sequencing, status: proposed, evidence_ref: docs/re-engineering-decisions-2026-10-12.md §10}
  belief:
    believed_state: the mismatches and caps listed in studies C, D and G exist as cited at fbb58c23
    main_uncertainty: whether any live path still depends on Message, Emit or Assign receipts (tests will show)
    next_observation: file review, then the grep inventory as the first construct
  blocker_or_risk: none known; touches many prompts at once, so the parity test lands first
  next_action: agentic-consensus file review of this station

receipts: []
---

# R1 — truthful prompts and deletions

Every study found the same failure: prompts that describe a system that
no longer exists, and runtime rules that patch the gap. This station tells
each desk the truth and deletes what the owner rejected, before any
redesign, so every later station is measured against honest desks.

Ways it can fail, written before the code:

- a prompt loses an instruction a desk needed, hidden as a "stale text"
  deletion (each deletion names the reality it now matches);
- deleting Message, Emit or Assign breaks a path that still reads their
  receipts (run the full test shards; grep receipt kinds);
- removing the round cap lets Texture and research ping-pong (accepted:
  circuit breakers when observed, not before; watch the demo);
- removing tool-loop budgets lets a broken cell loop until the 60-minute
  limit (the limit stays; the demo trace shows it);
- the transport fix changes cell latency or breaks the broker's handle
  protocol (the probe test and the demo cover both);
- the parity test is written to pass rather than to bind (it must fail on
  a planted stale verb before it is trusted).
