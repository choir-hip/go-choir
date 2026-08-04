# Texture/Lifecycle Dual-Tape Authority

**Date:** 2026-08-03  
**Status:** observed; no runtime repair authorized by this record  
**Classification:** substrate — semantic event authority and projection  
**Mutation class of the exposing change:** none (read-only source reconciliation)  
**Mutation class of the planned repair:** red

## Problem

Choir's current source does not yet make Texture a projection of the canonical
per-computer audit log.

The trusted `ComputerEventAppender` is the sole sequencer for the canonical
`ComputerID` event chain, but it is called mainly by self-development capsule,
verification, decision, materialization, checkpoint, and rollback paths.
Texture and the generic durable-work kernel instead commit document revisions,
trajectory state, work items, update dispositions, command receipts, and a
separate stream of `LifecycleEvent` objects directly into the embedded object
graph.

Those embedded transactions are durable and reducer-owned, but they are not
derived from the canonical computer event chain. Choir therefore has two
overlapping causal tapes:

1. `ComputerEventAppender` + externally pinned events + corpusd head CAS; and
2. embedded-Dolt lifecycle events + directly committed Texture/lifecycle state.

The two paths currently describe different subsets of one computer's meaning.
That violates the vision's one-tape claim and prevents Texture from being the
faithful supervisory projection on which self-development can rest.

## Evidence

- `internal/computerevent/appender.go` implements pin, prepare, canonical head
  CAS, finalize, recovery, and reconstruction for the per-computer event chain.
- `internal/store/computer_events.go` stores only the embedded event index/head
  projection during `Prepare`/`Finalize`; it does not reduce Texture or durable
  lifecycle state from the event payload.
- `internal/store/lifecycle.go` constructs and commits Texture documents,
  revisions, trajectories, work, updates, `LifecycleEvent` objects, and command
  receipts directly in embedded Dolt.
- Production calls to `ComputerEventAppender.AppendNew*` are concentrated in
  `internal/agentcore` self-development/capsule paths; `internal/textureowner`
  calls `StartLifecycle` and `CommitLifecycleArtifactHead` directly.
- The completed durable-computer convergence Definition proved restart-safe
  lifecycle reducers with effects OFF. It explicitly did not prove
  self-development or one-tape Texture reconstruction.

Observed source identity: `794b99c9bf1526ee74a72fec8ba31e0c21df6d16`.

## Root-Cause Belief

The prior self-development mission built the protected computer-event spine
before the generic artifact/trajectory/work substrate converged. The convergence
mission then repaired durable work inside embedded Dolt while deliberately
leaving effects OFF. The two accepted pieces were never joined afterward.

This is a substrate problem, not a Texture UI symptom. Fixing individual Texture
flows without changing authority would make a more usable projection of the
wrong causal model.

## Existing Replacement Opportunity

The replacement already exists: the canonical `ComputerEventAppender`, its
content-addressed event artifacts, corpusd CAS, crash recovery, and reconstruction
path. The preferred repair is to route typed Texture/supervision transactions
through that spine and make the embedded Texture/lifecycle objects deterministic
reducer projections. It is not to add a third log or patch the independent
`LifecycleEvent` stream.

## Bounded Repair Contract

The proposed successor Definition is
[`choir-texture-tape-supervision-2026-08-03.md`](../definitions/choir-texture-tape-supervision-2026-08-03.md).
It must:

- preserve Texture's semantic authorship and single-writer authority while
  moving causal ordering/durability to the canonical computer tape;
- preserve the layered authority contract: Texture is the agentic owner-facing
  document author and human-side supervisor, Super is the operational
  supervisor, CoSupers are scoped workers, and Researchers provide sourced
  evidence;
- bind Texture's claims of work, evidence, decision, and completion to exact
  event references while keeping its main surface legible at human bandwidth;
- import existing state with an explicit, content-addressed migration receipt;
- reduce Texture, intent, work, evidence, decisions, and settlement from typed
  computer events;
- prove deterministic reconstruction and one material mid-trajectory intent
  revision through the public CLI and Texture product path;
- delete or derive the competing lifecycle-event authority; and
- keep self-development effects OFF.

No code mutation is authorized until that Definition is owner-ratified and all
three mission registries promote it.

## Belief State

- **Supported:** the two durable substrates exist and are not currently one
  reducer chain.
- **Supported:** the computer-event spine is the existing replacement to wire
  in, not a new store to invent.
- **Supported:** general Texture polish is downstream of the authority repair;
  only projection UX needed to prove supervision belongs in this mission.
- **Pending:** the exact closed transaction schema, migration boundary, and
  compatibility-floor rollback release require a frozen candidate and review.

## Remaining Error Field

This record does not establish that every Texture caller bypasses the canonical
tape, that all current event kinds are sufficient, or that the existing
computer-event reducer can atomically materialize the full supervision graph.
The Definition's first execution slice must inventory every production caller
and formalize the refinement from event payload to embedded projection before
implementation.

## Rollback

Not applicable to this evidence-only record. The eventual behavior change must
use an additive event schema, a frozen compatibility-floor release that can
read/reconstruct every emitted event, write disablement before rollback, and
nondestructive projection rebuild. Events are never deleted or rewritten.

## Heresy Delta

- `discovered`: H032, dual semantic tape for Texture and lifecycle state.
- `introduced`: none by this evidence record.
- `repaired`: none; discovery and Definition authoring are not repair.
