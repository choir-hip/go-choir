# Texture Is the Supervisory Control Loop

**Last updated:** 2026-08-05. **Scope:** current design boundary and current
implementation gap, corrected by the owner after the Texture tape cleanup.

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
| Super | Execution decomposition, capsule lifecycle, verification, operational synthesis | Bash, capsule command execution, direct host writes, canonical Texture writes |
| CoSuper implementation slot | One assigned implementation inside one capsule | Host bash/filesystem, unassigned capsules, route or VM authority |
| CoSuper verifier slot | Independent capsule inspection and verification | Capsule execution or writes, host bash/filesystem, implementation authority |

The current CoSuper execution boundary is layered: only the CoSuper registry
contains `capsule_exec`; mutation tools require the CoSuper role and
`implementation` slot; the executor resolves an opaque capability by agent run
and handle; and the command runs through the assigned capsule broker rather than
on the host. Super may create, inspect, and destroy capsules but cannot execute
inside them.

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
- The persistent Super consumes typed `execution_request` packets and can
  supervise Researcher and capability-scoped CoSuper work.
- Researcher, Super, and CoSuper can return typed packets upward.

## What Is Not Yet Connected

The full bidirectional control loop is not currently accepted live behavior.
The missing path is more specific than “Super is disabled”:

1. Texture's current production registry does not contain `update_coagent`,
   although its prompt tells it to send follow-up messages to active
   researchers. Texture can spawn a Researcher but cannot currently steer that
   Researcher through the promised typed follow-up path.
2. Texture's current delegate policy names only Researcher. The former
   `request_super_execution` tool is absent, and Texture has no current
   capability to send a typed execution request to the persistent Super.
3. Self-development effects remain OFF. No current executable Definition
   authorizes a deployed Super-to-CoSuper capsule task, checkpoint, updater, or
   route effect.
4. Consequently, source contains strong pieces of the upward observation path,
   resident Texture loop, Super controller, and capsule boundary, but no
   deployed proof that Texture repeatedly observes, revises, and redirects a
   live execution trajectory.

This is a control-loop connectivity gap. It is not justification for a generic
supervision transaction grammar, findings database, observer hierarchy,
parallel causal tape, or public raw supervision API.

## Path Forward

The successor mission should connect the smallest capability-safe loop over the
existing substrate:

1. Give Texture a typed, target-constrained messaging capability for its
   Researcher coagents and the one persistent Super. Do not restore a host tool
   or a prompt-only permission.
2. Carry Texture-to-Super work as an idempotent, durable
   `execution_request` bound to the Texture trajectory and its open obligation.
3. Require Super and Researcher to return useful intermediate updates, not only
   terminal results.
4. Let each material update produce a new canonical semantic revision while the
   trajectory remains live, then let Texture send revised direction back down.
5. Preserve the current split: Super owns capsule lifecycle but cannot execute;
   implementation CoSuper executes only inside its assigned capsule; verifier
   CoSuper remains read-and-verify only.
6. Settle only through the existing trajectory/work-item reducer after
   obligations and pending updates are resolved.

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
