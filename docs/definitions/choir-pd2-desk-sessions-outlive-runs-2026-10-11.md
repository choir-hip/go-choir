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
  slice: staging proof in S1d (follow-up research run's first cell continues the session)
  source_ref: fcb424b9 (in c713d63a)
  deploy_identity: staging c713d63a
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
    believed_state: >-
      Desk sessions are keyed by agent for runs with a run record, retained
      30 minutes idle, at most 12 idle; children keep per-activation
      sessions; Egress reads the cell's run. Local tests and the full
      agentcore package pass; deployed in c713d63a.
    main_uncertainty: >-
      Where desk workers are reclaimed today when a run ends normally, and
      whether a long-lived worker interacts with the cell-terminal deadline
      and reduction state (armCellTerminalDeadline, reduction inbox).
    next_observation: read desk worker release paths and the reduction state carried per worker
  blocker_or_risk: none
  next_action: read S1d's follow-up research run; record the session line and whether it reused earlier variables

receipts:
  - id: S1d-PD2
    at: 2026-10-11T04:06Z
    deploy: staging c713d63a
    kind: deployed proof (trace s1d-final)
    result: >-
      Eight research runs of one desk: the first cell of the first run said
      "new session"; the other seven said "continuing session" with correct
      cell, run and age counts (for example a232777e continued efd81261's 4
      cells after 3m31s). A failed cell that killed the worker showed up as a
      fresh session count in the next run, as designed. The desk did not
      yet reuse earlier variables: each run re-read choir.Updates() and
      searched anew.
    conjecture: PD2 mechanism supported on the product path; PD2-1 (reuse instead of repeat) not yet observed
---

# PD2 — desk sessions outlive runs

A persistent desk that forgets everything between runs isn't really
persistent. This station keeps a desk's Go session alive between its runs
within one computer boot, tells the desk whether its session is new or
continuing, and reclaims idle sessions. A restart still erases everything,
which is what the crash rule requires.

## Failure list (written before code, 2026-10-11)

Read of `internal/agentcore/tools_desk.go`, `research_children.go`,
`internal/yaegikernel/desk_session_worker.go`, `session_loop.go`,
`internal/researchtools/egress_budget.go` at 701b0273.

Discovered while reading (heresy, today's code): **desk workers leak.** A
worker is released only when it dies (`tools_desk.go` after a failed cell) or
when a research child returns. A run that ends normally leaves its worker
subprocess (a re-exec of the autoputer binary) idle until the computer
restarts. Every Texture turn and every research run adds one. Keying by agent
reduces the count; the idle sweep below removes the leak for every key.

Ways PD2 could fail, and what the change must do about each:

1. **Two runs of one agent at once share a REPL.** Lifecycle serializes an
   agent's runs, but a stale run can overlap a new one. If the agent's session
   is busy with another run, the new run gets its own session (keyed by run)
   rather than waiting on the other run's cell or interleaving with it.
2. **Research children join their parent's session.** A child carries its
   parent's agent ID but no run record. Children keep their own per-run
   worker pool; the agent key applies only to runs with a run record. A child
   that took the parent's session would block on the parent's in-flight cell
   forever.
3. **Later runs act with the first run's lineage.** The worker's Egress
   closure captures the spawning call's execution context; `choir.Research`
   children would inherit the first run's run ID and deadline. Egress must
   read the execution context from the cell's own context, as Emit already
   does.
4. **The worker leaks or grows without bound.** Sessions idle past a
   retention window are closed; the pool keeps at most a fixed number of idle
   sessions and closes the least recently used beyond it. A busy session is
   never reclaimed.
5. **The model doesn't know its session persists, so it redoes the work.** The
   first cell of each run reports `session`: new, or continuing with how many
   cells and runs it already holds.
6. **The model trusts a session that is gone.** After a restart, a dead
   worker, a timed-out cell, or reclamation, the next cell's session line
   says "new", so a variable that has vanished doesn't look like a bug.
7. **A crash restart resumes work through a session.** Sessions are process
   memory and nothing writes them to disk, so a restart erases them. No
   resume path is added.
8. **The config differs between runs.** If a later run's role, root, or
   computer differs from the session's, the old session is closed and a new
   one spawned (session line: new).
9. **The egress budget spans runs.** Not a failure: the budget ledger is on
   the host, keyed by the run ID read from each cell's context, so each run
   keeps its own budget even when the session continues.
10. **The capsule path changes.** Engineering capsule workers
    (`capsule_go_eval`) are a different pool and are not touched.
11. **`choir.Context()` reports the first run's activation ID.** It is
    informational; record it as a known residual rather than re-spawning.
