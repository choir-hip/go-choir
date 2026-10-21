---
definition_version: 4

# Station FP10 / ARCH-06 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): agents are records,
# worker slots are pooled. Authority: the metamission's FP10 entry (the P2
# items, each decided item by item) and SDC ARCH-06 in
# docs/security-density-architecture-checklist-2026-10-21.md.
# Decision of this lane: design only. Every slice edits
# internal/agentcore/tools_desk.go, which RES-17 (lane/fp10-res17-reaper,
# rebased as lane/res17-on-fp2) changes, and reads FP2's desk manifest and
# slot-bound frames (lane/b3-desk-boundary-2), neither on main yet.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main d1c60ad87
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: lanes/fp10-arch06-agent-records
      status: clean
      class: goal_candidate
      owner: FP10 (ARCH-06)
      touch: goal_owned
      recovery: branch lane/fp10-arch06-agent-records
  observed:
    - >-
      tools_desk.go:84-305 at d1c60ad87: one session (REPL and process) per
      agent:<id>; per-activation for research children; 30 minute idle
      retention; at most 12 idle computer-wide; busy sessions unbounded.
    - >-
      research_children.go: a fresh pool per child, released when the child
      returns; at most 4 children at once (researchChildConcurrency).
    - >-
      FP2 (lane/b3-desk-boundary-2): internal/deskmanifest gives each desk a
      Kind, a Taint (untrusted_input or privileged) and its own uid;
      sameDeskSessionConfig compares role, so no worker crosses desks
      (TestPoolNeverSharesAWorkerAcrossTaintClasses).
    - >-
      RES-17 (lane/fp10-res17-reaper 12c2fbb36): an idle reaper timer on the
      pool; idle worker RSS 82,752 kB -> 20,128 kB with cmd/desk-worker.
    - >-
      PD2 receipt S1d-PD2: desks did not reuse earlier REPL values across
      runs; the session is a cache, the records are the state.

finish:
  deliver: >-
    The number of desk worker processes on a computer is bounded per
    (tenant, desk class, taint class, tree), not by agent count, and a desk
    that loses its slot loses only its REPL cache, told so, never its records.
  artifact: >-
    Problem record docs/problems/desk-worker-count-follows-agents-not-classes-2026-10-21.md;
    this design; then slices S1 to S3 below as commits on a lane rebased on
    FP2 and RES-17 after they land.
  acceptance:
    - action: S1 on staging, a research run with fan-out plus Texture and management turns
      proves: per-class live, busy, idle and peak worker counts, before any cap
      evidence_class: deployed proof
    - action: S2 tests (written first) and a staging run with caps enforced on the test computer
      proves: no class exceeds its cap; one class never evicts another's session; a refused cell is retryable and named
      evidence_class: local tests, then deployed proof
  rollback: revert each slice; S1 and S2 ship behind DESK_POOL_MODE (off|observe|enforce), default observe
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize peak desk worker processes per computer for the same desk work,
    while preserving desk isolation (no worker crosses desk or taint class),
    PD2's session notice, the crash-restart rule, and every busy cell.
  goodharting_would_be: >-
    A low process count from refusing cells the desk then cannot retry; a
    pool that reuses one process across agents or desks and leaks values
    between them; a cap tuned from a quiet test computer.

homotopy:
  realism_axis: >-
    design -> observe counts on staging -> per-class caps on the test
    computer -> caps on every computer -> (ARCH-11, later) slots that leave
    the guest, where tenant becomes a real key.

boundaries:
  mutation_class: green (this design); red (S1 to S3, the desk worker lifecycle)
  authority_sources:
    - metamission FP10 entry; SDC ARCH-06, ARCH-05
    - definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md (must_preserve carried)
    - CLAUDE.md restart rule; doctrine I26 desk ontology (unchanged)
  must_preserve:
    - A worker never serves two desks, two taint classes, or two agents' sessions at once (PD2 excluded sharing a session between agents).
    - A busy session is never reclaimed or cancelled to make room.
    - A crash restart erases every slot and session; nothing resumes (no disk persistence of REPL state).
    - Each worker keeps its own cgroup memory budget (FP2); no multi-agent process, so one runaway cell kills one agent's slot.
    - Capsule (engineering assignment) workers stay a separate pool.
    - The desk ontology (system desks, app desks) is not changed.
  excluded:
    - Edits to tools_desk.go, research_children.go or yaegikernel before FP2 and RES-17 land.
    - In-process interpreter reset across agents (needs ARCH-07's per-slot token rebind; a reused slot is a fresh process until then).
    - Replaying cells to rebuild a session (side effects; restart rule).
    - Workers outside the guest (ARCH-11).
  protected_surfaces: [internal/agentcore tools_desk worker lifecycle, research_children, guest desk confinement]

now:
  status: blocked_incomplete
  slice: design written; S1 waits for FP2 and RES-17 to land on main
  source_ref: main d1c60ad87
  deploy_identity: none (docs only)
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: ARCH06-1
    claim: >-
      Peak desk workers on a busy computer are set by agent count and
      research fan-out; per-class caps with a bounded wait cut the peak
      without failing desk work, because the REPL is a cache the desks
      barely reuse (S1d-PD2).
    test: S1's per-class peak counts on staging, then S2's refusal and wait counts under caps
    edge: missing_oracle
    delta_o: a desk_pool line per acquire with class, live, busy, idle, peak and would_wait/would_evict
    scope_if_supported: desks on one computer within one boot
    status: proposed
    evidence_refs: [docs/problems/desk-worker-count-follows-agents-not-classes-2026-10-21.md]
  decision: {what: design only in this lane, kind: scope, status: decided, evidence_ref: this file header, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      Process count follows agents; the idle bound is global; children
      spawn per child. FP2's manifest supplies desk class and taint.
    main_uncertainty: the real peak and class mix on a busy computer (S1 measures it)
    next_observation: S1 desk_pool lines from a staging fan-out run
  blocker_or_risk: FP2 and RES-17 not on main; both edit the pool
  next_action: after both land, rebase this lane and build S1 (tests first)
---

# FP10 / ARCH-06: agents are records, worker slots are pooled

## The split

Today a desk session is one object: an agent's REPL values living in one
worker process. ARCH-06 splits it in two.

- **Agent (record).** The agent's state is what the tape already holds: run
  records, cell transcripts, `choir.Records`. A session's REPL values are a
  cache on top of that.
- **Slot (process).** A confined worker process of one desk class, run as
  that desk's uid (FP2), with its own memory budget. A slot serves one
  agent's session at a time.

The pool bounds slots per class. When a class is full, an idle session of
the same class is evicted (its slot closed, its next cell told "new
session"), never a busy one and never another class's.

## Pool class key

`(tenant, desk class, taint class, tree)`:

- **tenant**: the computer ID. One per guest today, so it is constant; it
  is in the key so slots that leave the guest (ARCH-11) cannot cross
  tenants.
- **desk class**: the manifest desk name (`deskmanifest.Lookup(role)`).
  Since each desk has its own uid, a slot cannot serve another desk anyway;
  the key makes the bound per desk.
- **taint class**: the manifest `Taint`. Redundant with desk class today,
  kept so an app desk that declares untrusted input never shares a cap
  with a privileged desk of the same kind.
- **tree**: the root run of a `choir.Research` tree for research children,
  empty for root desks. Children of one tree share their tree's slots; two
  trees never do, so one request's fan-out cannot starve another's.

An unknown role (not in the manifest) gets no class and keeps today's
behavior, logged. Red code fails closed: in enforce mode an unknown role
is refused, not pooled.

## Slices

**S1, observe (red, ships on, changes nothing).** On every acquire and
done, compute the class key and record per-class live, busy, idle and peak
counts; with a per-class cap configured (`DESK_POOL_CAPS`, e.g.
`research_child=4,research=2`), also log `would_wait` and `would_evict`.
Out: a `desk_pool` field on the existing desk session trace event and on
the runtime stats endpoint. Ways it could fail, for the tests written
first: the key is computed from the spawning run instead of the cell's
(PD2 failure 3 again); counts drift when a dead worker is replaced or
`release` runs outside `done`; the reaper (RES-17) closes a session without
updating counts; observe mode changes eviction order.

**S2, per-class caps (red, `DESK_POOL_MODE=enforce` on the test computer
first).** Idle LRU becomes per class (one class's burst never evicts
another's session). A cell whose class is at its busy cap waits, bounded by
its own context, for a slot of its class, then gets a retryable refusal
naming the class and the runs holding it (the same shape as today's "busy
with run X's cell" error). A busy session is never cancelled. Nothing
waits across a restart (the wait is process memory). Caps come from S1's
numbers, not from a guess.

**S3, tree reuse for research children (red).** Children of one tree draw
from one tree pool capped at `researchChildConcurrency`. A finished
child's slot is closed and the next child gets a fresh process: no
interpreter reuse across agents until ARCH-07's per-slot token can be
rebound on handoff and a reset is proved empty. The gain is a bound, and
later (with ARCH-07) warm spares.

**Later: richer eviction notice.** When a session is evicted, keep a
one-line summary (cells, runs, names defined) so the "new session" notice
can say what was lost and point at the records that hold it. In memory
only; a restart erases it.

## Standing questions, short

- Decision provenance: SDC ARCH-06; PD2 must_preserve carried over.
- Single state authority: the tape; the pool is a cache and never a source.
- Fate-sharing: one agent per process; a cell's OOM kills its own slot.
- Restart durability: the pool is process memory; a crash erases it and
  nothing resumes; no boot-time step.
- Artifact-verified success: S1's per-class peaks from staging, not tests.
- No-SSH operability: counts on the stats endpoint and trace events.

## Red ceremony (for S1 to S3)

- Conjecture delta: ARCH06-1 proposed (front matter).
- Protected surfaces: the desk worker lifecycle and guest desk confinement.
- Admissible evidence: local tests with a fake clock and fake workers, then
  staging desk_pool lines; deployed proof run by FP0.
- Rollback: `DESK_POOL_MODE=off`, or revert the slice.
- Heresy delta: discovered 2 (busy workers unbounded; global idle LRU
  crosses desk classes), introduced 0, repaired 0.
