---
definition_version: 4

# PD2 — a desk's Go session outlives its run. Station of
# definitions/choir-persistent-desks-metamission-2026-10-11.md. Owner
# 2026-10-11: "for any follow-ups, texture is going back to the research
# desk, which already has done work and has Go cells. So it's like a
# continued conversation between agents."
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686
    deploy_identity: staging choir.news b3dbc686
  worktrees:
    - path: /Users/wiz/go-choir
      status: see metamission start
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      desk_go_eval keys its yaegi worker by run ID
      (deskWorkerActivationID, internal/agentcore/tools_desk.go). Each run
      of any desk starts a fresh REPL; values from the desk's previous run
      are gone. The model is told that a timed-out cell loses earlier
      values; nothing tells it that a new run does.
    - >-
      CLAUDE.md: a crash restart never resumes work; a planned restart may
      resume once via a durable marker. Process-local state is erased by
      any restart (standing question 8).
    - >-
      Desk workers are killable subprocesses with an 8 GiB address-space
      cap (deskWorkerMemoryLimitBytes); a worker dies with its session on a
      cell timeout.

finish:
  deliver: >-
    When a desk runs again for the same agent — a follow-up to research, the
    next Texture turn, management's next wake — its cells see the values its
    earlier runs defined, until the computer restarts or the desk is idle
    past a retention window. The desk is told when its session is new.
  artifact: >-
    Desk workers keyed by agent ID (the persistent desk), not run ID, with
    an idle retention window and a memory bound; the cell result or run
    preamble says "new session" vs "continuing session (cells N..M from runs
    ...)"; restart erases sessions (no resume from disk in this station).
  acceptance:
    - action: >-
        Focused test: two runs of one agent; the second run's cell reads a
        variable the first defined; a different agent cannot; after the
        retention window the session is new and says so.
      proves: session continuity and isolation
      evidence_class: local tests
    - action: >-
        Staging: research-only request then a follow-up on the same
        document; the follow-up research run's first cell reads a value from
        the first run (trace).
      proves: continued conversation on the product path
      evidence_class: deployed proof (test computer)
  rollback: revert; workers return to per-run keying
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize work a desk repeats because its earlier session is gone, while
    preserving crash-restart closes work, memory bounds, and one writer per
    session.
  goodharting_would_be: >-
    Keeping sessions alive while the model is never told they persist, so it
    rebuilds state anyway; leaking workers until the guest runs out of
    memory.

homotopy:
  realism_axis: >-
    in-memory session per agent within one computer boot -> retention and
    memory policy -> (later, separate) checkpointed sessions that survive a
    planned restart.

boundaries:
  mutation_class: red (desk worker lifecycle)
  authority_sources: [owner statement 2026-10-11, CLAUDE.md restart rule]
  must_preserve:
    - a crash restart never resumes work (sessions are process memory; a restart erases them)
    - one active run per agent uses a session at a time (no concurrent cells on one REPL)
    - worker memory cap; idle workers are reclaimed
    - capsule (engineering assignment) workers stay per assignment
  excluded:
    - persisting sessions to disk or across restarts
    - sharing a session between agents
  protected_surfaces: [internal/agentcore tools_desk worker lifecycle]

now:
  status: working
  slice: not started (waits on PD1)
  source_ref: b3dbc686
  deploy_identity: staging b3dbc686
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: PD2-1
    claim: >-
      If a desk's session survives between its runs and the desk is told so,
      follow-up runs reuse earlier results instead of repeating searches.
    test: follow-up run traces — cells referencing earlier values vs repeated searches
    edge: missing_oracle
    delta_o: a session line in each run's first cell result naming the session's age and cell count
    scope_if_supported: desks on one computer within one boot
    status: proposed
    evidence_refs: []
  decision: {what: none, kind: none, status: none, evidence_ref: none, owner_ratification_ref: not_applicable}
  belief:
    believed_state: sessions are per run for every desk
    main_uncertainty: >-
      Where desk workers are reclaimed today when a run ends normally, and
      whether a long-lived worker interacts with the cell-terminal deadline
      and reduction state (armCellTerminalDeadline, reduction inbox).
    next_observation: read desk worker release paths and the reduction state carried per worker
  blocker_or_risk: none
  next_action: after PD1 lands, read the worker lifecycle and write the failure list before code

receipts: []
---

# PD2 — desk sessions outlive runs

A persistent desk that forgets everything between runs isn't really
persistent. This station keeps a desk's Go session alive between its runs
within one computer boot, tells the desk whether its session is new or
continuing, and reclaims idle sessions. A restart still erases everything,
which is what the crash rule requires.
