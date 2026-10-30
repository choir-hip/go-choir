# Agent-tree leaves can only run inside the computer's own guest (2026-10-22)

Station: [FP10, ARCH-11 and ARCH-12](../definitions/choir-launch-fp10-arch-11-12-2026-10-21.md).
Checklist items: ARCH-11, ARCH-12 in
[security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md).
Problem first; no fix here. Mutation class of this record: green.

## The problem

Every leaf of an agent tree runs in the one guest that also holds the
computer's identity material, its tape, its store and its privileged desks.
There is no place to run a leaf outside that guest, so:

1. **Untrusted content shares the guest with privileged authority.** A
   research child reads arbitrary web pages in the same guest that holds
   the computer's signer keys, gateway bearer and Texture's canonical
   writer. The only layers between them are Yaegi and the desk-cell
   sandbox (REC-01, SEC-01); the microVM, which the owner calls "the real
   security boundary" (owner notes §36), is not between them.
2. **Fan-out is bounded by one guest.** Research children run as
   goroutine activations inside the guest
   (`internal/agentcore/research_children.go:25`, `researchChildConcurrency
   = 4`). The owner's target is "a hierarchical tree of agents with 100 or
   1,000 leaf agents ... across processes, whether they be capsules or
   across VMs, and then eventually across hosts" (owner notes §36). A guest
   is 2 to 8 GiB (SDC RES-21) and one store mutex (ARCH-13); a wide tree
   cannot leave it.
3. **Heavy compute has nowhere to go but the computer.** Renders,
   conversions and builds that a leaf asks for run in a guest-local capsule
   (ARCH-09) and take the computer's memory and CPU, while the host may have
   spare capacity outside it.

## Evidence

- Code: research children are in-guest activations
  (`internal/agentcore/research_children.go:25-48`); capsules are
  guest-local namespaces (`docs/computer-ontology.md:334`, CLAUDE.md
  Safety: "Use disposable guest-local capsules inside the stable computer").
- The only machine that runs outside a guest today is another computer,
  and a sibling computer is a **forked computer** with its own ComputerID,
  event chain and identity material (`docs/computer-ontology.md`, Naming
  Rules), which waits on S9 re-keying. Nothing between a capsule and a fork
  exists: a machine with no identity, no state and no authority.
- The 2026-10-11 fan-out record shows what a wide tree inside one guest
  does to the computer's shared budget: 239 children, 4 results
  ([research-fanout-overruns-computer-model-budget-2026-10-11.md](../archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md)).
  That record is about the model budget, not isolation; it is cited only for
  the shape of the load.
- "Worker VM", the name the old design used, is retired
  (`docs/computer-ontology.md`, Naming Rules: "Do not use ... worker VM").
  There is no current ontology name for the machine ARCH-11 asks for.

## Belief state

- Believed: the missing piece is an authority-less, stateless microVM that
  a computer leases for one tree and that returns proposed acts the
  computer's own desk validates and commits. It differs from a capsule
  (outside the guest) and from a forked computer (no identity, no event
  chain, no route).
- Not believed: that this is launch work. It is P2 and depends on ARCH-05
  (taint classes), ARCH-07 (authenticated frames), ARCH-08 (budgets),
  SEC-05 (gateway bearer bound to its computer), NET-01 (host fetch) and
  FP8's jailer and image work.

## Remaining error field

- No ontology name (owner question).
- Whether snapshot templates may be shared across tenants is DEC-05; the
  default (no cross-tenant shared pages) stands.
- Whether one machine may serve several trees of the same tenant and taint
  class, or each tree gets a fresh clone.
- How a leaf reaches models and the web without holding a credential.
