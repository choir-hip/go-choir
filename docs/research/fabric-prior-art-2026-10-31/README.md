# Resource fabric prior art (2026-10-31)

Research corpus for the resource fabric. It is input to a later
first-principles design, not a decision. Owner, 2026-10-31: design the fabric
"from first principles" as a small, general mechanism with organization
needs as configuration on top; "we should really be exhaustive on all the
distributed systems prior art before we take on the design from a space of
making committal decisions."

The questions it informs are Q1 to Q10 in [brief.md](brief.md). The design
it will inform is [resource fabric](../../design/resource-fabric-2026-10-31.md)
(proposal 1 accepted with site access control; proposals 2 to 10 open).

## Surveys

Seven Sonnet surveys, then a second Sonnet pass per survey (run on the agents
VM) that opened primary sources and retagged each claim: `[V]` verified with a
URL, `[CORRECTED]`, or `[UNVERIFIED]`. Each file opens with its verification
log. None recommends a final choice.

1. [Consensus, leases, fencing, replication](surveys/01-consensus-leases-replication.md)
2. [Partitions, offline work, branching](surveys/02-partitions-offline-branching.md)
3. [Capabilities and authorization](surveys/03-capabilities-authorization.md)
4. [Placement, scheduling, reachability](surveys/04-placement-scheduling-reachability.md)
5. [Actors, durable execution, external effects](surveys/05-actors-durable-execution-effects.md)
6. [Distributed and personal operating systems](surveys/06-distributed-personal-os.md)
7. [Verification, configuration spaces, self-tuning](surveys/07-verification-config-self-tuning.md)

## Panels

The same brief went to the agentic-consensus panel twice:
[divergent](panel-divergent/) (candidate kernel architectures, eleven lenses)
and [lateral](panel-lateral/) (hidden assumptions, reframes, outside
analogies). Seven of eleven panelists answered each: Codex and both GPT
anchors were out of usage until 2026-11-05, and MiniMax M3.1 flash was
unavailable. Raw outputs, unsynthesized.

## Threads the first pass surfaced (not decisions)

- A write lease is a liveness device, not a safety device: laptop sleep
  breaks lease clock assumptions, and the writer epoch at the append CAS is
  the fencing token. The open cost of a lease with no expiry is the stale
  lock.
- If every policy knob affects only liveness or loss bounds and none touches
  a safety invariant, safety can be model-checked once for all
  configurations (Flexible Paxos, seL4 capDL, Cedar are the precedents).
- Irreversible effects are the output-commit problem. After an equivocation
  the same precommit could fire from two branches, so the effect actuator
  must deduplicate by intent and occurrence id. "Precommitment" also names
  a scored prediction in docs/precommitment-records.md; the two uses need
  separate words.
- Confidential computing does not protect employee A's work from employee
  B's own workstation (physical attacks are out of scope by vendor
  statement), so cross-person placement is a policy question.
- Closest whole-system precedents: HTCondor (owner policy on desktop
  machines), Tailscale (home as coordination, relays carry ciphertext,
  Tailnet Lock), MDM work profiles (offboarding), Kubernetes spec and status
  (control without writing another's state).
