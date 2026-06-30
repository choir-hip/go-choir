# Design: Conductor Supervision Protocol

## 1. Problem

Choir has multiple actors operating on a shared object graph — Texture, researcher, super, appagents, and the user. The orchestration model is still changing because there is no explicit, durable supervision layer. The symptoms are:

- Texture revisions stall without explanation.
- Researchers emit prose without source packets, and nobody notices until the user reads the output.
- Super receives malformed execution requests and the user has to debug it manually.
- There is no queryable settlement state. "All threads clear" is a chat phrase, not a durable record.
- Conductor is asked to do routing, supervision, and portfolio attention at the same time, which drags it back toward the rejected "semantic babysitter" role.

The problem is not a missing conductor. It is a missing supervision protocol, typed and durable, that watches the object graph without owning canonical artifacts.

## 2. Non-goals

- The supervisor does not author Texture revisions, appagent artifacts, or source packets.
- The supervisor does not mutate the object graph directly. Its only writes are supervision objects (findings, messages, work items, settlement records).
- The supervisor does not scrape prose. It reads structured observations.
- The supervisor is not a catch-all retry loop. It is a protocol-health immune system.
- The supervisor does not replace user judgment for semantic decisions. It enforces mechanical invariants.

## 3. Authority boundaries

| Actor | Owns | Can mutate | Cannot do |
|---|---|---|---|
| User | everything | anything (via UI/commands) | n/a |
| Texture | canonical document state | revisions, source refs, publish | invent source packets; execute external actions |
| Appagents | their own artifacts | their own output objects | mutate canonical Texture state directly |
| Super | execution plans | executes tools, returns results | decide what should be done |
| Ingress conductor | routing table | ingress work items | supervise trajectories |
| Trajectory supervisor | health state, findings, addressed messages | supervision objects, messages | edit artifacts |
| Meta-conductor | portfolio attention | priority work items, user attention requests | edit artifacts |

The invariant is single-writer per object type. The trajectory supervisor may write messages, but the actor that receives the message owns the response.

## 4. Observation schema

An observation is a typed record produced by a sensor. Observations are append-only and named.

```text
observation_id
observed_at
trajectory_id
sensor_kind: one of
  trace_event
  appagent_event
  source_packet
  tool_result
  mailbox_state
  work_item
  artifact_validator
  actor_liveness
  user_clarification
subject_id
subject_kind
payload: typed JSON
schema_version
```

Examples:
- `artifact_validator` with `subject_kind=texture_revision`, payload `{validator: "texture_source_coverage", passed: false, claim_count: 3, cited_count: 0}`.
- `source_packet` with payload `{packet_kind: "coagent_source_packet.v1", has_sources: false, from: "researcher"}`.
- `mailbox_state` with payload `{pending_count: 4, oldest_age_ms: 120000, owner: "texture:doc-123"}`.

## 5. Finding / verdict schema

A finding is a durable record of a protocol-health verdict. It is fingerprinted to prevent spam.

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
expected_response_shape: e.g. "coagent_source_packet.v1 with sources[]"
resolution_at
resolved_by
```

Findings are objects in the graph. They can be cited, queried, and settled.

## 6. Action schema

Actions are the supervisor's only output. Each action is addressed, auditable, and idempotent.

| Action | Target | Effect | Example |
|---|---|---|---|
| `send_actor_message` | Actor mailbox | Actor receives a structured message with invariant, evidence, and expected response | Message researcher to resend with `coagent_source_packet.v1` sources |
| `open_work_item` | Work queue | Durable obligation to resolve a finding | "Texture doc-123 needs source coverage" |
| `ask_user` | User notification | Request clarification when the supervisor cannot decide | "Researcher cannot reach source X; approve marking claim unsupported?" |
| `record_protocol_violation` | Trajectory log | Non-blocking audit record of a broken contract | Researcher returned prose without sources three times |
| `record_clear` | Trajectory log | Settlement record that all invariants pass | `all_threads_clear` |

Not allowed:
- `patch_texture`
- `edit_artifact`
- `invent_source_packet`
- `rewrite_findings`
- `execute_super_work`

## 7. State machine

Per trajectory:

```text
observing
  -> healthy        (work progressing, no findings)
  -> watch          (possible issue, too early to act)
  -> nudge_required (actor has obligation but has not acted)
  -> blocked        (actor lacks required input/capability)
  -> violation      (protocol contract broken)
  -> settled         (settlement query passes)
```

Transitions are triggered by observation processing, not by model confidence. A `nudge_required` finding produces exactly one `send_actor_message` action per fingerprint. If the actor responds, the finding is resolved. If the actor repeats the violation, the old finding is reopened or a new `record_protocol_violation` is emitted.

## 8. First validators

The first validator proves the shape without making the supervisor autonomous.

### Validator: `researcher_packet_has_sources`

Observation: a `coagent_source_packet.v1` or coagent update from a researcher lacks `packet.sources` or `sources[]`.

Finding: `malformed_researcher_packet`, severity `violation`.

Action: `send_actor_message` to the researcher with the expected response shape.

Fingerprint: `trajectory_id + "researcher_packet_has_sources" + "researcher" + evidence_hash`.

### Validator: `texture_source_coverage`

Observation: a Texture revision contains factual claims without native `source_ref` nodes while `available_source_entities` are non-empty.

Finding: `unsupported_factual_claims`, severity `nudge_required`.

Action: `send_actor_message` to Texture asking it to attach source refs or mark claims unsupported.

## 9. Acceptance tests

1. Given a researcher update with prose but no `packet.sources`, the trajectory supervisor records one `malformed_researcher_packet` finding and sends exactly one addressed message to the researcher.
2. Given the same condition repeated with the same evidence, no new message is sent (same fingerprint).
3. Given a proper `coagent_source_packet.v1` response, the finding is resolved.
4. Given a Texture revision with source-free claims, the supervisor sends one message to Texture.
5. The supervisor never edits a Texture document, source packet, or app artifact.
6. Settlement query returns true only when there are no open findings, no pending mailbox items, and no active actor holding work.

## 10. Deletion targets

This supervision protocol should replace or shrink the following existing patterns:
- Ad hoc retry logic in appagent loops.
- Implicit "wait for the user to notice" stalls in Texture trajectories.
- Any conductor code that currently edits artifacts or synthesizes source packets.
- Informal "all threads clear" chat messages as settlement evidence.

The goal is to delete bespoke control paths, not add another one.

## 11. Implementation phases

Phase 1: read-only observation and verdicts. No actions.
Phase 2: one safe action — `send_actor_message` for `researcher_packet_has_sources`.
Phase 3: Texture validators — source coverage, markdown leakage, bunched citations, empty source entities.
Phase 4: durable `record_clear` settlement object.
Phase 5: meta-conductor / portfolio attention.

The first proof is Phase 2: exactly one idempotent message when a researcher forgets sources.
