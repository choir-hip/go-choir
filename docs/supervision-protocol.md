# Texture Is the Supervisory Control Loop

**Last updated:** 2026-08-08. **Scope:** supervisory design boundary and the
joined continuous-supervision candidate. Effects remain OFF until the exact
candidate passes deployed Linux acceptance and registry promotion.

Supervision is not a separate Choir subsystem, protocol-health layer, actor
class, causal log, or workflow engine. It is the continuous control work
performed by Texture and Super:

- the owner delegates ongoing trajectory supervision to Texture;
- Texture maintains the canonical, versioned semantic state of the work and
  uses that state to direct Researcher and Super;
- Super supervises execution, decomposition, and verification;
- Researcher gathers sourced evidence;
- CoSuper performs a capability-bounded implementation or verification
  assignment.

The owner is not expected to read or approve every Texture version. A trajectory
may run for hours, days, weeks, or months and produce many versions between
owner reads. The owner can sample the latest head, inspect history when useful,
and correct the standing state. Texture remains responsible between those
interventions for keeping the trajectory aligned with the owner's intent and
moving it toward completion.

The durable-work kernel, typed updates, trajectories, Trace, work items,
settlement queries, capsules, and canonical computer events support this loop.
They are substrate and evidence, not another supervisor.

## The Control Loop

```text
                         occasional intent or correction
Owner  ------------------------------------------------------------+
  ^                                                                 |
  | samples the current head when useful                            v
  |                                                          Texture agent
  |                                               observes updates + current head
  |                                               revises semantic control state
  |                                               judges progress and drift
  |                                               redirects, deepens, or waits
  |                                                     |             ^
  |                              addressed direction and questions    | updates
  |                                                     v             |
  +-- immutable versions <--- current Texture head     Super / Researcher
                                                            |
                                                            v
                                                     scoped CoSupers
```

This is a feedback loop, not a request followed by a final report:

1. Texture holds the current idea-level state of the task in version `Vn`.
2. Texture sends focused requests, questions, or corrections to Researcher or
   the persistent Super.
3. Those agents return evidence, partial results, contradictions, questions,
   and blockers as they arise. They do not wait for the whole trajectory to end.
4. Each material semantic change wakes Texture. Texture metabolizes the update
   into `Vn+1` and re-evaluates the direction of the still-running work.
5. Texture may then send new direction downstream. This repeats as long as the
   trajectory remains live.
6. An owner edit is an asynchronous correction to this loop, not an approval
   gate on every preceding version.

Completion is one semantic transition in this loop, not the first time Texture
hears back from a worker.

## Semantic State Versus Operational Action

The canonical Texture is the controller's semantic state. It should describe:

- the current understanding of the system or problem;
- what changed that understanding;
- the objective, constraints, and active ideas;
- the evidence and uncertainty that matter;
- competing explanations or possible futures;
- the judgment or conceptual correction still needed.

It should not become a worker transcript, command queue, action checklist,
topology dump, or status dashboard. Concrete tool choreography, shell commands,
worker assignments, retries, and capsule operations belong in addressed
messages, work items, artifacts, and Trace. Texture incorporates what those
actions teach at idea level.

A new version means the semantic control state changed. It does not mean the
owner was notified, read it, or must approve it.

## Capability Boundary

These are capability restrictions, not prose instructions. A role does not
merely promise not to cross the boundary; its production tool registry and
runtime capability checks prevent the crossing.

| Actor | Control responsibility | Mechanically unavailable |
|---|---|---|
| Texture | Canonical semantic state; trajectory direction; owner-readable current head | Host bash, writable filesystem, capsule execution, route or VM mutation |
| Researcher | Sourced evidence and questions | Bash, writable filesystem, canonical Texture writes, privileged effects |
| Super | Persistent execution decomposition, assignment direction, verification synthesis, and upward reporting | Bash, generic capsule lifecycle, capsule execution, direct host writes, canonical Texture writes |
| Assigned implementation CoSuper | One implementation assignment inside its exact capsule | Host bash/filesystem, unassigned capsules, route or VM authority, generic coagent/provider tools |
| Assigned verifier CoSuper | Independent inspection and verification inside its own exact capsule | Host bash/filesystem, unassigned capsules, implementation-candidate authority, generic coagent/provider tools |
| Unassigned or legacy CoSuper | None | Every capsule and reporting effect |

The joined CoSuper boundary is assignment-owned rather than a static role
registry. Generic Super-to-CoSuper spawn and model-callable capsule lifecycle
are absent. Trusted assignment runtime snapshots one complete canonical source
tree, durably opens and binds the assignment, creates the isolated capsule, and
installs an exact registry from an empty closed set: capsule-local
read/list/write/exec plus authenticated assignment reporting. Both
implementation and verification slots may execute tests and write local support
inside their own capsule; only the reducer can derive candidate or verification
authority. Every tool call revalidates the live trajectory, open work, exact
run, assignment, capability, and capsule. Terminal reports and cancellation
follow durable intent, executor effect/inspection, and structured durable
acknowledgement; delayed authenticated receipts are evidence only.

Prompt text explains these boundaries. It does not create them.

## What Current Code Already Supports

- A Texture document has one durable `texture:<doc_id>` actor and one canonical
  writer among agents.
- Addressed `update_coagent` packets can wake Texture immediately, join a
  resident activation, or survive passivation and restart.
- An update-triggered Texture turn is required to write a canonical revision;
  packet dispositions distinguish incorporated, rejected, and still-pending
  updates.
- One long-running Texture actor can write many canonical revisions. A write is
  not necessarily run completion.
- Owner revisions delivered while the actor is resident enter the same durable
  mailbox rather than replacing or bypassing the loop.
- The exact persistent Super consumes durable ordered `execution_request`
  controls without becoming lifecycle-scoped itself. Researcher and Super
  delivery is bound to the exact run and consumed only from authenticated
  durable runtime memory.
- Texture can send target-constrained lifecycle direction to its Researchers
  and the persistent Super. Both return typed canonical packets upward; reporting
  settles only the authenticated consumed delivery intersection.
- One atomic Texture turn can incorporate several inbound packets while
  projecting exactly one public version. Direct owner edits rebase the complete
  pending old-head occurrence set in one CAS transition.
- Lifecycle cancellation is intent-first and fate-shared with assignment
  capsules; actual delayed reports remain authenticated evidence without
  candidate, Pass, packet, wake, projection, or reopen authority.

## Acceptance State

The smallest capability-safe loop is connected in the joined source candidate,
but it is not yet accepted live behavior. Effects remain OFF. Promotion still
requires the exact immutable SHA to pass CI, deploy to Linux staging, and produce
product-path evidence for repeated bidirectional cycles, passivation/restart,
owner correction, cancellation while provider work is in flight, late evidence
retention, and real namespace/cgroup/overlay cleanup. The canonical receipt
registry must then promote exactly one candidate.

This is an evidence and promotion gap, not permission to add a supervision
transaction grammar, findings database, observer hierarchy, parallel causal
tape, public raw supervision API, callback authority, mailbox poller, or generic
capsule route.

## Path To Promotion

1. Freeze one immutable candidate and pass joined Store/runtime/race suites and
   independent lifecycle and capsule/security review.
2. Push that SHA, require CI, and verify Linux staging reports the exact build.
3. Exercise several Texture→Researcher/Super→Texture cycles, an owner correction,
   passivation/restart with the same exact Researcher run, and lossless 101+
   ordered delivery.
4. Exercise an assigned implementation and independent verification capsule
   through the product route, including durable intent/effect/ack receipts,
   in-flight cancellation, restart orphan cleanup, and delayed evidence-only
   reporting.
5. Confirm one semantic revision projects one public version, close every open
   registry entry, and only then promote effects for the reviewed route.

The acceptance proof must exercise multiple feedback cycles, not merely one
round trip:

- the owner starts work and then remains absent while several versions accrue;
- a partial or contradictory Researcher update causes a revision and a focused
  follow-up;
- an intermediate Super result or blocker causes another revision and changed
  direction before execution completes;
- the actors passivate, restart, and continue without losing current state or
  pending updates;
- the owner later opens the current head and sees an accurate idea-level state,
  while prior versions remain inspectable;
- an owner correction changes subsequent Texture state and downstream
  direction;
- any execution remains capsule-scoped and independently verified.

Wall-clock months need not be simulated. The proof must establish the durable
feedback mechanics that allow the same trajectory to continue for that long.

## Success Criterion

Supervision works when, without watching the action stream or reading every
version:

- the owner can inspect the latest Texture and understand the current state of
  the work;
- Texture has continuously incorporated material learning into immutable
  semantic versions;
- Researcher and Super have received adaptive direction from Texture rather
  than executing a frozen initial plan;
- work survives absence, passivation, and restart;
- no role can perform effects outside its mechanically enforced capability;
- completion follows resolved obligations and evidence, not a worker's final
  prose.

That is the agent-product outcome. More supervisory infrastructure is not proof
of more supervision.
