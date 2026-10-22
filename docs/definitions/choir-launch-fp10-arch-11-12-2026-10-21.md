---
definition_version: 4

# Station FP10 (items ARCH-11 and ARCH-12) of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md). Authority: the
# owner's direction of 2026-10-21 (owner notes §36: agent trees of 100 or
# 1,000 leaves "across processes ... across VMs, and then eventually across
# hosts"; §38: the first pass is the security and density checklist), relayed
# by the orchestrator (FP0) as this lane with the instruction "Design only".
# Both items are P2. This station writes the design and stops; nothing here
# changes runtime behavior.
readiness: drafted

review:
  reviewer: none yet; a design, so FP0 decides whether a panel reads it in a bunch
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:00:00Z
  source:
    canonical_ref: lane/fp10-arch11-executors from main d1c60ad87
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp10-arch11-executors
      status: clean
      class: goal_candidate
      owner: FP10
      touch: goal_owned
      recovery: branch lane/fp10-arch11-executors
    - path: the lane/fp8-image-jailer worktree
      status: unknown
      class: other_agent_wip
      owner: FP8 (vmctl, vmmanager, the VM image)
      touch: forbidden
      recovery: leave in place; this design names the vmctl and vmmanager seams it would need instead of editing them

finish:
  deliver: >-
    A reviewed-ready design for ARCH-11 (authority-less, stateless microVMs
    that a computer leases for one agent tree, which run untrusted-content
    leaves and heavy compute and return proposed acts) and ARCH-12 (the
    same machines placed on other hosts), with a proposed ontology name and
    the owner questions it needs; plus the problem record.
  artifact: >-
    docs/problems/agent-tree-leaves-share-the-computer-guest-2026-10-22.md
    and this file (green, first commit);
    docs/design/outboard-capsules-arch-11-12-2026-10-22.md (green).
  acceptance:
    - action: FP0 (or a bunch panel) reads the design against the ontology, CLAUDE.md Safety, DEC-05 and the standing questions
      proves: the design names its prerequisites, keeps the fork and capsule rules, and leaves DEC-05's default standing
      evidence_class: review receipt
  rollback: git revert of the docs commits; nothing runtime changes
  landing:
    required: false
    environment: none (docs only; the docs truth checker runs on push)
    required_receipts: [pushed_commit]

value:
  better_means: >-
    The smallest number of new concepts and authority paths that lets a
    tree's untrusted and heavy leaves leave the computer's guest, so the
    microVM sits between untrusted content and the computer's authority, and
    fan-out is bounded by host capacity rather than one guest.
  goodharting_would_be: >-
    A design that renames "worker VM" without saying how it differs from a
    capsule and a forked computer; a leaf machine that quietly holds a
    gateway token, a store or a route; a snapshot template shared across
    tenants by default; or a placement plan for other hosts that ignores
    that the frame protocol is unauthenticated today.

homotopy:
  realism_axis: >-
    Design (this station) -> an in-guest seam with the current goroutine
    leaves behind it, behavior unchanged -> one outboard machine kind cold
    booted on one host for one taint class on test computers, observe mode
    -> per-tenant snapshot templates -> placement on a second host.

boundaries:
  mutation_class: green (this station); the slices it designs are orange to red
  authority_sources:
    - owner notes §36 and §38 (2026-10-21), via FP0
    - SDC ARCH-11, ARCH-12, DEC-05
    - docs/computer-ontology.md Naming Rules; CLAUDE.md Safety (forks)
    - S9 forks and fleets; SP4 multi-host placement; PD4
  must_preserve:
    - '"Worker VM", "candidate VM", "background computer" and "candidate computer" stay retired; the new machine gets a proposed name, ratified by the owner.'
    - A forked computer keeps its definition (own ComputerID, event chain, fresh identity, results only by publication and adoption); the new machine is not a fork and never becomes one.
    - DEC-05's default stands: no page sharing (snapshot templates, pmem DAX) across tenants.
    - No boot-time resumer without a planned-restart marker; a crash closes every lease.
    - FP8's files (vmctl, vmmanager, the VM image) are not edited by this lane.
  excluded:
    - Any code. The first code slice is designed here and left for a later pass.
    - S9 fork construction and re-keying; SP4's host failover.
  protected_surfaces: [none touched; the design names vmctl, gateway calls, guest boundary and egress as the surfaces its slices would touch]

now:
  status: checkpoint_incomplete
  slice: >-
    Design written; the owner questions are open. No runtime change.
  source_ref: lane/fp10-arch11-executors
  deploy_identity: not applicable (docs only)
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: [docs/definitions, docs/problems, docs/design]
  conjecture:
    id: authority-less-leased-leaf-machines
    claim: >-
      A microVM with no ComputerID, no identity material, no store and no
      route, leased by one computer for one tree and one taint class,
      reached only through a host relay with a per-lease token, and whose
      only output is proposed acts that the leasing desk validates and
      commits, puts the microVM boundary between untrusted content and the
      computer's authority without adding a new writer or a new identity.
    test: >-
      Slice O2's deployed probes: from inside a leased machine, attempt to
      reach the lessor's guest, another tenant, host-private services and
      the gateway without the lease token, and to write to the lessor's
      tape; every attempt is refused; a real research leaf's findings arrive
      as a proposed act and are committed by the research desk.
  blocker_or_risk: >-
    Prerequisites not landed: ARCH-05 taint classes, ARCH-07 authenticated
    frames, ARCH-08 budgets, SEC-05 bound gateway bearer, NET-01 host fetch,
    and FP8's jailer and image work in vmctl and vmmanager.
  next_action: >-
    Owner answers OQ-1 (the name) and OQ-2 to OQ-4; then slice O1 (the
    in-guest LeafRunner seam, orange, behavior unchanged) can be built
    without touching FP8's files.
  heresy_delta:
    discovered: >-
      No ontology name exists between a guest-local capsule and a forked
      computer; the checklist item still leans on the retired idea of a
      worker VM.
    introduced: none
    repaired: none
---

# FP10: ARCH-11 and ARCH-12, design only

The design is [docs/design/outboard-capsules-arch-11-12-2026-10-22.md](../design/outboard-capsules-arch-11-12-2026-10-22.md);
the problem is [agent-tree-leaves-share-the-computer-guest-2026-10-22.md](../problems/agent-tree-leaves-share-the-computer-guest-2026-10-22.md).
