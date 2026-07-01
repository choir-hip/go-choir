did we ever document this? or is it just a totally open loop? what do you know about this idea, where it progressed to?

>## Conductor Supervision

Conductor is the foreground supervisor for appagent liveness, routing, and
protocol health. It observes structured logs, appagent events, source packets,
tool results, mailbox state, blockers, and contract violations.

Conductor does not author canonical Texture revisions or appagent artifacts.
Its control surface is message passing: it sends structured messages to Texture,
researcher, super, or other appagents when the event stream shows missing input,
blocked progress, stale work, contract violation, or required user clarification.

Conductor input is structured observation, not raw narrative scraping.
Conductor output is addressed actor messages, not direct mutation.

Examples:
- Texture has source-free claims -> conductor messages Texture: request source substrate or mark uncited.
- Researcher returns prose without sources -> conductor messages researcher: return CoagentSourcePacket.sources.
- Super receives malformed execution request -> conductor messages Texture: resend action packet.
- Appagent is idle with unresolved blocker -> conductor asks user or routes to the right actor.
- All actors settle -> conductor records all threads clear.

Invariant:
text
Conductor supervises protocol and liveness.
Texture owns canonical document state.
Appagents own their artifacts.
Conductor acts by messages.
Structured logs are its sensory input.

This also resolves the earlier thread pattern cleanly: the orchestration thread is basically a human-visible conductor. Long-term, Choir’s conductor should do the same thing internally for Texture/appagents.


Show less

12:12 AM






Worked for 2m 33s



well, lets design this now.











lets think hard about this. cognitive transforms












Current Uncertainty
The risk is naming this “conductor” and accidentally recreating the thing Choir doctrine already rejects: a semantic babysitter that routes every appagent loop and quietly becomes the real owner of artifacts.
The opportunity is also real: Choir needs a live evaluator/controller that notices stuck trajectories, protocol violations, missing sources, malformed packets, and stale work, then nudges the right actor through messages.
So the hard question is:
How do we build supervision without stealing ownership?
Selected Transforms
Controller Transform
Sensors, actuators, model, objective. This prevents “conductor” from becoming vibes.

Single-Writer / Authority Transform
Who owns mutation? This protects Texture/appagents from hidden conductor authorship.

Observer Hierarchy Transform
Who watches the watcher? This reveals why “conductor” and “meta-conductor” should not be the same object.

State Machine Transform
What are the legal states and transitions? This turns “all threads clear” into a real settlement state.

Deletion-First Transform
What old control machinery does this replace? This keeps us from adding another orchestration layer beside stale controllers.

Route-Changing Insights
First: the thing we need is not “a smarter conductor.” It is a supervision protocol.
The protocol can be implemented by an actor called conductor, trajectory-conductor, meta-conductor, or watchdog, but the name is secondary. The core object is:
structured observations -> health verdicts -> addressed messages/work items
Second: the conductor must not own the artifact graph. It owns attention and protocol pressure, not content.
So its actuator set is intentionally narrow:
send_actor_message
open_work_item
ask_user
record_clear
escalate_protocol_violation
Not allowed:
patch_texture
edit app artifact directly
invent source packets
rewrite researcher findings
execute super work
Third: there are three different supervision scopes, and conflating them causes architecture drift.
Ingress conductor:
  routes new exogenous input.

Trajectory supervisor:
  watches one trajectory for liveness/protocol health.

Meta-conductor:
  allocates attention across many trajectories/work queues.
The pasted design mostly describes the trajectory supervisor, not the current ingress conductor.
Fourth: “all threads clear” should not be a chat phrase. It should be the result of a settlement query:
no open obligations
no pending mailbox items
no active actor holding work
artifact validators pass
no blocking protocol findings
Fifth: this should delete or shrink existing control paths. If it just adds a new agent that watches the old run-tree/controller/mutation machinery, it becomes more complexity. The win is replacing hidden bespoke loops with a uniform observation/action protocol.
Changed Design
The right abstraction is:
Supervisor = agent-as-judge over a trajectory's protocol health.
It has:
Sensors:
  trace events
  appagent events
  source packets
  tool results
  mailbox state
  work items / obligations
  artifact validators
  actor liveness
  user clarification state

Private model:
  trajectory health state
  open findings
  actor responsibility map
  recent action fingerprints

Actuators:
  addressed actor message
  durable work item
  user question
  protocol violation receipt
  all-clear / settlement note

Objective:
  keep trajectories progressing without violating ownership boundaries.
State Machine
For each trajectory:
observing
  -> healthy
  -> watch
  -> nudge_required
  -> blocked
  -> violation
  -> settled
Meanings:
healthy:
  work is progressing; no action.

watch:
  possible issue but too early to intervene.

nudge_required:
  actor has enough context and obligation but has not acted.

blocked:
  current actor lacks required input/capability.

violation:
  protocol contract broken, e.g. researcher prose without sources.

settled:
  settlement rule passes; record all-clear.
Actions are idempotent and fingerprinted:
finding_fingerprint =
  trajectory_id + invariant + actor + subject + evidence_hash
So conductor does not spam Texture every few seconds with the same complaint.
Key Invariants
1. Conductor cannot mutate canonical artifacts.
2. Conductor cannot synthesize semantic content as if it came from an appagent.
3. Conductor output is always addressed and auditable.
4. Conductor input is structured observation, not prose scraping.
5. Every conductor action names the invariant, evidence, target actor, and expected response shape.
6. Settlement is queryable durable state, not model confidence.
Examples Reframed
Texture source-free claim:
Observation:
  artifact_validator.texture_source_coverage failed

Finding:
  unsupported_factual_claims

Action:
  message Texture:
    Current head has factual claims without native source refs.
    Either attach native source_refs from available source_entities or revise
    the prose to mark the claims as unsupported.
Researcher prose without sources:
Observation:
  coagent update lacks packet.sources

Finding:
  malformed_researcher_packet

Action:
  message researcher:
    Resend as coagent_source_packet.v1 with sources[] and selectors[].
Super malformed execution:
Observation:
  super mailbox received non-execution_request

Finding:
  wrong_actor_or_packet_kind

Action:
  message sender or Texture:
    Super requires execution_request. Resend to Texture as evidence_update
    or issue a typed execution_request.
All clear:
Observation:
  no open obligations
  no pending mailbox
  all actors passivated or idle
  artifact validators pass

Action:
  record_clear:
    all_threads_clear
Important Design Choice
I would not call the new per-trajectory actor simply conductor.
Use:
ingress_conductor
trajectory_supervisor
meta_conductor
Or, if we want fewer nouns:
conductor = ingress
supervisor = per-trajectory health
metaconductor = portfolio
This avoids fighting existing doctrine that says conductor is not the supervisor of every semantic loop.
Implementation Shape
Phase 1: read-only supervision.
Add observation records.
Add artifact validators.
Add trajectory health verdicts.
No messages yet.
Phase 2: protocol nudge for one safe case.
Malformed researcher packet -> message researcher.
No artifact mutation.
Phase 3: Texture health validators.
source-free claims
bunched citations
markdown leakage
source entities without transcludable content
Phase 4: all-clear settlement.
durable all_threads_clear record
visible in Trace / work queue
Phase 5: meta-conductor.
work queue priority
stale work
blocked trajectories
owner attention requests
Next High-Information Action
Write a short design doc with this split:
docs/design-conductor-supervision-protocol-2026-06-23.md
Core sections:
1. Problem
2. Non-goals
3. Authority boundaries
4. Observation schema
5. Finding/verdict schema
6. Action schema
7. State machine
8. First validators
9. Acceptance tests
10. Deletion targets
The first concrete proof should not be a broad autonomous conductor. It should be one narrow validator:
Given a Texture trajectory with researcher prose lacking packet.sources,
the trajectory supervisor records a protocol violation and sends exactly one
idempotent message to researcher requesting coagent_source_packet.v1 sources.
That proves the shape without letting conductor become the artifact owner.