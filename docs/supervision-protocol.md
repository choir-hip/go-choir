# Layered Supervision Protocol (Target)

**Last updated:** 2026-08-03. **Scope:** Target — decided direction, not yet
fully implemented. The durable-work kernel this protocol builds on is **Live**
(see [current-architecture.md](current-architecture.md) and the
[convergence Definition](definitions/choir-coherent-computer-convergence-2026-07-21.md)),
but the unified event-derived supervision layer is unbuilt.

This document is the current home for Choir's supervision design intent. The
archived
[conductor-supervision-protocol](archive/design-conductor-supervision-protocol-2026-06-23.md)
and
[observer-hierarchy](archive/design-observer-hierarchy-2026-06-23.md) notes
remain in Git history as pre-kernel inputs, not current authority.

## Why layered supervision

Choir has multiple actors operating on one durable computer. They work at
different bandwidths and carry different authority:

```text
CoSupers (scoped technical work) -> Super (operational supervision)
                                      |
Researchers (sourced evidence) ------+-> Texture (user point of view)
                                           |
                                           v
                                      owner (root authority)
```

Texture is not merely a UI or passive rendering layer. Texture is an agent. It
owns the request-fulfilling document state, may message Super, Researchers,
both, or neither, and integrates their returned evidence and work into a
legible account for the owner. In that sense Texture is the human-side
supervisor: it represents the user's point of view and controls what the user
must inspect or decide.

Super is the operational supervisor. It decomposes granted intent, scopes
CoSuper work, maintains beliefs and obligations, reconciles evidence and
dissent, and proposes settlement. CoSupers are workers, not supervisors.
Researchers produce sourced evidence. The owner remains the root authority.

This layering is only real if every operationally meaningful statement shown
by Texture is grounded in the computer's causal history. Today direct embedded
Texture/lifecycle commits and the canonical computer-event appender form two
semantic tapes (H032). The target repairs that substrate first: one causal
tape, multiple deterministic projections.

## 1. Authority boundary (Target)

The per-computer event appender is the sole causal sequencer. Each actor may
authorize only its own typed events. Embedded Texture, lifecycle, CLI, Trace,
and supervision views are deterministic projections of that tape, never
independent authorities.

| Actor | Owns | May authorize | Cannot do |
|---|---|---|---|
| Owner | root intention, ratification, and promotion authority | requests, clarifications, decisions, acceptance, rollback | delegate away root accountability implicitly |
| Texture | request-fulfilling document semantics and the owner-facing account | document revisions; messages to Researcher or Super; typed summaries; requests for owner decision | invent evidence or operational success; execute worker effects; hide a blocker, dissent, irreversible gate, or owner decision |
| Researcher | sourced evidence packets | evidence claims with source references and uncertainty | edit Texture; execute operational work; settle a decision |
| Super | operational supervision state | decompositions, scoped obligations, messages, belief updates, findings, reconciliation, settlement proposals | edit Texture directly; perform CoSuper effects; promote or accept on the owner's behalf |
| CoSuper | assigned technical result | scoped findings, evidence, or inert change bundles | redefine intent; self-settle; supervise the portfolio; mutate canonical Texture state |

The invariant is single semantic authorizer per object type and one causal tape
for all resulting events. A recipient owns its response; no actor can forge
another actor's authorization.

## 2. Texture as agent and human-side supervisor (Target)

Texture fulfills the user's request through document state. For each material
request it may:

1. revise the document directly when the available record is sufficient;
2. ask Researchers for sourced evidence;
3. ask Super for operational investigation or work;
4. ask both and reconcile their returned material; or
5. pause with an explicit blocker or owner decision.

Texture decides how dense machine work becomes a human-scale interaction. A
typical trajectory compresses many worker actions into fewer claims and fewer
still contested decisions:

```text
N technical actions -> M grounded claims -> K owner decisions, where K <= M <= N
```

This compression is semantic work, not lossy decoration. A Texture-authored
narrative summary is itself a typed event with exact references to the claims,
evidence, obligations, decisions, and effects it summarizes. Rebuilding the
projection replays that event; it does not require rerunning a model.

Human bandwidth is an explicit limit. When the requested material cannot fit,
Texture exposes honest overflow and lets the owner drill into the grounded
record. It never silently drops unresolved dissent or required decisions.

Mandatory deterministic control blocks remain visible regardless of narrative
compression:

- current request and fulfillment status;
- active blockers and unmet obligations;
- material dissent or incompatible evidence;
- irreversible or externally consequential gates;
- decisions reserved for the owner; and
- exact evidence/event references for claims of work, decision, or completion.

Without an exact reference, an operational statement is presented as pending,
conjectural, or blocked — never as accomplished fact.

## 3. Super as operational supervisor (Target)

Super observes semantic state and supervises execution toward granted intent.
Its durable state includes:

```text
granted_intent
beliefs and uncertainties
scoped obligations
worker assignments
findings and dissent
evidence references
settlement conditions
proposed decisions
```

Super may dispatch CoSupers, request Researcher evidence through the defined
message path, compare independent results, and replan when evidence changes.
It returns compact, structured operational state to Texture. Dense technical
jargon may remain below this boundary; Texture is responsible for translating
what matters to the owner.

Super does not become a second document author. It cannot directly write the
canonical Texture projection, execute worker effects itself, or claim owner
acceptance. Its settlement output is a grounded proposal for Texture and the
owner to inspect.

## 4. Workers and evidence producers (Target)

CoSupers receive bounded assignments and return typed results. External coding
agents, when later activated, are CoSupers under this protocol. Their output is
evidence or an inert candidate bundle until the applicable verifier and
promotion authority accept it. They cannot broaden scope or treat a successful
tool call as mission settlement.

Researchers return sourced packets with claim-to-source coverage, uncertainty,
and material conflicts. Unsourced prose is not a Researcher result. Texture or
Super may request Researcher work, but neither may manufacture its missing
sources.

## 5. Fan-out and promotion seam (Target)

Super may open multiple CoSuper assignments in one transaction. Each assignment
and attempt binds a stable identity, the parent Super decision, intent revision,
observed working base, scope and obligations, capability/policy digest, and
idempotency commitment. CoSupers may execute concurrently; their results append
to the single tape in completion order. Serial acknowledgement does not imply
serial execution.

Retry creates another attempt in the same assignment lineage. Cancellation ends
authority but not history. A late result remains evidence and receives an
explicit disposition. Super cannot propose settlement until every required
assignment and attempt—including failures, cancellations, late output, and
dissent—is dispositioned.

Fan-out branches are evidence/result producers, not independently promotable
desired states. In a future effects-on mission, selected results flow through an
explicit integration plan and one bounded integrator CoSuper. That integrator
constructs one new content-addressed `CapsuleEffectBundle` at the then-current
canonical base. Independent verification covers both its inputs and composed
behavior. Only that exact digest may receive owner acceptance and become the
single pending desired-state transition.

```text
N concurrent branch results
  -> Super reconciliation and integration plan
  -> one current-base composed bundle
  -> independent verification
  -> owner decision / accepted desired state
  -> guest materialization receipt
  -> checkpoint receipt
  -> expected-generation route projection receipt
```

Overlapping results, composition failure, or a head advance force explicit
rebase/recomposition and re-verification. There is no implicit merge and no
direct landing of a stale or branch-local bundle. Checkpoint and route systems
are actuators/projections; they cannot authorize meaning. Materialization,
checkpoint, route, or rollback failure adds causally ordered evidence and any
required compensation instead of rewriting the tape.

The one-tape supervision mission preceding effect activation models and
property-tests this seam but uses non-effect CoSuper results. It does not invoke
capsule freeze, stage `updater/incoming`, advance a self-development operation,
or emit promotion events.

## 6. Event-derived observation and grounding (Target)

All supervision reads projections derived from the same causal tape. A typed
observation may include:

```text
observation_id
observed_at
trajectory_id
event_ref
sensor_kind: one of
  texture_revision | actor_message | source_packet | worker_result
  | obligation_state | verifier_result | effect_receipt | settlement_state
subject_id
subject_kind
payload: typed JSON
schema_version
```

Observation rows never acknowledge semantic state on their own. Reducers must
be deterministic, sequence-aware, restart durable, and able to reconstruct the
same Texture and supervision state from the retained tape.

Grounding is stricter than provenance decoration:

- a claim that work happened cites the accepted worker/effect event;
- a claim that evidence supports a conclusion cites the evidence event and its
  source references;
- a claim that a decision was made cites the authorized decision event;
- a claim of completion cites the settlement and verifier events; and
- a superseded claim remains traceable through its semantic rebase lineage.

## 7. Semantic rebase (Target)

Long-horizon work changes the state beneath earlier conclusions. Choir therefore
rebases meaning rather than replaying prose blindly.

When new evidence, work, or owner intent invalidates an earlier claim, the
authorizing actor emits a typed supersession or reconciliation event that names
the prior event, new evidence, surviving obligations, and resulting status.
Reducers preserve both history and current meaning. Texture then updates the
human-facing document without erasing why the earlier account changed.

Rebase never rewrites the tape, changes an event's original meaning in place,
or silently resolves dissent. It adds a causally ordered interpretation whose
authority and evidence are inspectable.

## 8. Findings, messages, and settlement (Target)

A finding is a durable operational-supervision record, fingerprinted to prevent
spam:

```text
finding_id
finding_fingerprint = trajectory_id + invariant + actor + subject + evidence_hash
observed_at
state: open | resolved | escalated
severity: watch | nudge_required | blocked | violation
trajectory_id
invariant
responsible_actor
subject_id
evidence_hash
expected_response_shape
resolution_event_ref
```

Super's actions are addressed, auditable, and idempotent:

| Action | Target | Effect |
|---|---|---|
| `send_actor_message` | Researcher, CoSuper, or Texture mailbox | structured request with intent, evidence, and expected response shape |
| `open_obligation` | supervised trajectory | durable scoped work or evidence obligation |
| `record_finding` | supervision projection | grounded protocol-health verdict |
| `propose_decision` | Texture | reconciled options and reserved authority |
| `propose_settlement` | Texture | evidence-backed claim that the deterministic settlement query passes |

Texture may independently message Researcher or Super and may ask the owner
when authority or intent is missing. “All threads clear” is never an actor's
unsupported assertion. It is a projection over no open findings, no pending
mailbox or work obligations, no unresolved material dissent, and satisfied
verifier contracts.

## 9. Hierarchy termination (Target)

The hierarchy terminates at the owner, the source of intention and root
observer. Texture is the owner's agentic, human-bandwidth surface. Super is the
single operational-supervision role beneath it. Portfolio reflection and
self-development are modes of Super's supervision state, not new
“meta-conductor” actors in an infinite tower.

Policy or code changes proposed by self-development remain candidate mutations
subject to verification, promotion, rollback, and owner-reserved decisions.
The hierarchy does not weaken those boundaries.

## 10. First validators (Target)

- **`researcher_packet_has_sources`**: a Researcher result lacks native source
  references -> Super or Texture records a malformed-packet finding -> exactly
  one addressed request for the expected response shape.
- **`texture_claim_is_grounded`**: a Texture claim of evidence, work, decision,
  or completion lacks the required exact event reference -> the claim remains
  pending/conjectural/blocked and the mandatory control block identifies the
  gap.
- **`texture_control_blocks_complete`**: projection hides a material blocker,
  dissent, irreversible gate, or owner decision -> verifier rejects the
  projection.
- **`projection_rebuild_equivalent`**: replay of the retained tape does not
  reproduce the accepted Texture and supervision digests -> acceptance fails.
- **`fanout_disposition_complete`**: a required assignment or attempt lacks one
  current disposition, or a late result supports settlement without explicit
  incorporation -> settlement refuses.
- **`promotion_candidate_is_composed`**: a future acceptance names a branch
  result/bundle rather than one current-base composed digest with independent
  composition verification -> acceptance refuses.

## 11. Build order (Target)

1. Repair H032 by routing Texture and lifecycle semantic writes through the
   canonical computer-event appender and deriving embedded state by reducer.
2. Prove restart reconstruction and projection-digest equivalence, with legacy
   direct writers removed or made unreachable.
3. Implement Super's event-derived observation, findings, scoped obligations,
   idempotent messaging, and concurrent assignment/attempt lineage without
   worker effects.
4. Implement Texture's grounded narrative summaries, mandatory control blocks,
   and honest overflow.
5. Prove semantic rebase across new evidence, dissent, replanning, and owner
   decision while reconstructing the same human-facing state; property-test the
   future single-composed-candidate promotion seam without invoking actuators.
6. In a successor mission, activate external coding-agent CoSupers behind the
   existing candidate, verifier, promotion, and rollback boundaries.

The first mission intentionally keeps generic durable-work effects off. Its
proof is a real request flowing through Texture, Researcher, Super, and a
scoped CoSuper result; a deterministic rebuild; and a human inspection showing
that the owner sees the few grounded decisions that matter rather than the
full technical transcript.
