# Trajectory Supervision Protocol (Target)

**Last updated:** 2026-08-01. **Scope:** Target — decided direction, not yet
fully implemented. The durable-work kernel this protocol builds on is **Live**
(see [current-architecture.md](current-architecture.md) and the
[convergence Definition](definitions/choir-coherent-computer-convergence-2026-07-21.md)),
but the supervision layer itself is unbuilt.

This document is the current home for Choir's supervision design intent. The
archived
[conductor-supervision-protocol](archive/design-conductor-supervision-protocol-2026-06-23.md)
and
[observer-hierarchy](archive/design-observer-hierarchy-2026-06-23.md) notes
remain in Git history as the pre-kernel source of this design.

## Why a supervision protocol

Choir has multiple actors operating on a shared durable computer — Texture,
researchers, super, CoSuper executors, and the owner. The durable-work kernel
(single-writer reducers, trajectories, typed updates, settlement) removes the
"who owns what" ambiguity. What it does not yet provide is a *protocol-health
immune system*: a typed, durable observer that watches the object graph
without owning canonical artifacts, and turns stalled or contract-violating
state into addressed findings instead of user-noticed failures.

The symptoms this targets are the pre-kernel liveness failures recorded in the
archived design: texture revisions stalling without explanation, researchers
emitting prose without source packets, and "all threads clear" being an
informal phrase rather than a durable settlement record.

## 1. Authority boundary (Target)

The supervision layer never writes canonical artifacts. Its only outputs are
supervision objects: findings, addressed messages, work obligations, and
settlement records. This matches the kernel's single-writer invariant — the
trajectory/update/work reducers and `CommitTextureHeadAuthority` are the only
canonical writers.

| Actor | Owns | Can mutate | Cannot do |
|---|---|---|---|
| Owner | everything | anything (via UI/commands) | n/a |
| Texture | canonical document state | revisions, source refs, publish | invent source packets; execute external actions |
| Appagents | their own artifacts | their own output objects | mutate canonical Texture state directly |
| Super | execution plans | executes tools, returns results | decide what should be done |
| Trajectory supervisor | health state, findings, addressed messages | supervision objects, messages | edit artifacts |
| Meta-conductor (Target) | portfolio attention | priority work items, owner-attention requests | edit artifacts |

The invariant is single-writer per object type. The supervisor may write
messages, but the actor that receives a message owns the response.

## 2. Observation (Target)

Supervision observes **semantic state updates**, never action streams or
prose. Each meaningful change to standing state advances the supervised
trajectory's texture to a new version; the reducers maintain snapshot
projections (artifact head, obligations, update dispositions, reducer
sequence). A sensor reads those projections and emits a typed, append-only
observation:

```text
observation_id
observed_at
trajectory_id
sensor_kind: one of
  trace_event | appagent_event | source_packet | tool_result
  | mailbox_state | work_item | artifact_validator | actor_liveness
subject_id
subject_kind
payload: typed JSON
schema_version
```

Observation rows are projections and never acknowledge semantic state; the
reducer-owned graph remains the only truth. Because every layer observes the
same projections, adding a layer costs a query, not a copy of the world — this
is what keeps a supervision hierarchy from turning into an unbounded tower.

## 3. Findings and verdicts (Target)

A finding is a durable record of a protocol-health verdict, fingerprinted to
prevent spam. Its resolution is the kernel's update-disposition vocabulary
(`pending → incorporated | rejected | cancelled | late`).

```text
finding_id
finding_fingerprint = trajectory_id + invariant + actor + subject + evidence_hash
observed_at
state: one of open | resolved | escalated
severity: watch | nudge_required | blocked | violation
trajectory_id
invariant: string, e.g. "researcher_packet_has_sources"
actor: the actor responsible for responding
subject_id: the object that violates the invariant
evidence_hash: hash of the evidence payload
expected_response_shape
resolution_at
resolved_by
```

"All threads clear" is a settlement query over no open findings, no pending
mailbox obligations, and no non-terminal updates — already defined
deterministically by the kernel's settlement rule, not by a supervisor claim.

## 4. Actions (Target)

Actions are the supervisor's only output: addressed, auditable, idempotent.

| Action | Target | Effect |
|---|---|---|
| `send_actor_message` | actor mailbox | structured message with invariant, evidence, expected response shape |
| `open_work_item` | work queue | durable obligation to resolve a finding |
| `ask_user` | owner notification | clarification request when the supervisor cannot decide |
| `record_protocol_violation` | trajectory log | non-blocking audit of a broken contract |
| `record_clear` | trajectory log | settlement record that all invariants pass |

Not allowed: `patch_texture`, `edit_artifact`, `invent_source_packet`,
`rewrite_findings`, `execute_super_work`.

## 5. State machine (Target)

```text
observing
  -> healthy        (work progressing, no findings)
  -> watch          (possible issue, too early to act)
  -> nudge_required (actor has obligation but has not acted)
  -> blocked        (actor lacks required input/capability)
  -> violation      (protocol contract broken)
  -> settled        (settlement query passes)
```

Transitions are triggered by observation processing, not model confidence. A
`nudge_required` finding produces exactly one `send_actor_message` per
fingerprint; repeated violation reopens the finding or emits
`record_protocol_violation`.

## 6. Hierarchy termination (Target)

The hierarchy terminates at the **owner as root observer** — the owner of the
computer and the source of intention. Within the system, the self-learning
layer is a periodic, event-driven reflection mode (schedule, failure trigger,
or owner request), never a continuous meta-meta-supervision tower. It proposes
policy changes as mutation transactions subject to the same verification and
promotion protocol as any other change; the owner decides.

## 7. First validators (Target)

- **`researcher_packet_has_sources`**: a researcher update lacks
  `packet.sources` → finding `malformed_researcher_packet` (violation) →
  one addressed message with the expected response shape.
- **`texture_source_coverage`**: a Texture revision contains factual claims
  without native `source_ref` nodes while available source entities are
  non-empty → finding `unsupported_factual_claims` (nudge) → one message to
  Texture.

## 8. Build order (Target)

Phase 1: read-only observation and verdicts, no actions. Phase 2: one safe
action — `send_actor_message` for `researcher_packet_has_sources`. Phase 3:
Texture validators. Phase 4: durable `record_clear` settlement object. Phase 5:
meta-conductor / portfolio attention.

The first proof is Phase 2: exactly one idempotent message when a researcher
forgets sources. The deletion targets are bespoke control paths — ad hoc retry
loops, "wait for the user to notice" stalls, and informal settlement
language —
the goal is to delete bespoke control, not add another layer.
