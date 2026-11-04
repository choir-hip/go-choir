# Texture: a turn that only writes leaves the owner's request open with no next actor, silently

Date: 2026-11-04. Class: green (this doc). Owner ruling, 2026-11-04: "eradicate
the waiting except for true resource contention. and even then, that should be
semantically exposed to eng, mgmt, and texture."

## Evidence

Staging, claude arm, computer-a875023a6379c41f6ddb438f02e76a02.

- 01:45:08Z an owner revision asks for a new engineering round. The Texture
  desk's work item "Revise the document for the owner's new request." is open.
- The Texture desk (model deepseek-v4.1-flash) runs two cells. Cell 1:
  `ReadDoc` and `choir.Help("Cast")`. Cell 2: one `choir.Write` replacing
  blocks, and no `choir.Cast`, `Wait` or `Done`.
- One intent, receipt `rlm:texture_turn:0`; no refusal, no error. The turn
  commits a revision with the caller work disposition `open`.
- The loop passivates (reason `durable_transition_success`, empty response).
- The owner_revision work item stays open. No Cast went out, so research,
  management and engineering owe nothing; no record, report or operation will
  wake the desk. No event says so. 24 minutes pass silently until an operator
  nudge.

## Cause

Two rules that are each right, composed without a check between them:

- 658ba4282 made Wait after Write a no-op, and the Texture prompt says "a cell
  that Writes needs no Wait": a Write closes its own turn. That is right when
  the revision answers the owner, or when work it waits on is already in
  flight.
- A Write without Done commits with the caller work item left open
  (`commitTextureTurn`, `WorkDisposition: open`). Nothing at commit asks
  whether anything will ever act on that open item. When the owner asked for
  work only a Cast starts, a Write-only turn is a silent stall: the item is
  open, and there is no next actor.

The activation-end idle path has the same hole: a Texture activation whose
cells commit no turn gets the runtime's Idle turn (`answerIdleTextureTrigger`),
which consumes the trigger and leaves the work item open, again with no next
actor and no visible state.

A Write-only turn is legitimate today when something is already working for
the request: an open research or management round Texture cast (the
conversation head is open with Texture as caster), a live engineering
assignment on the trajectory, or a live self-development operation (the
runtime runs it; the document reports on it). Stories and publications keep
their Texture item open by design (new captures wake them), and the Texture
root's standing item is clamped open.

## Next actor, named

A desk turn that leaves an owner request open must name its next actor:

- a Cast, Ask or Note in the cell (a round opens and someone owes a report);
- `choir.Wait(why)`, naming what it waits on;
- `choir.Done(why)` when the revision answers the request;
- or something already working: an open round Texture cast, a live
  engineering assignment, a live self-development operation.

## Fix chosen

(a) Guard at turn commit. A Texture turn on a document trajectory (not a story,
publication or the Texture root) that leaves an open Texture work item, with no
Done, Wait or dispatch in the cell and nothing already working, is refused,
nothing committed, with the plain message: "the owner's request is still open
and nothing is working on it: Cast to start the work, Wait(why) if it waits on
something named, or Done(why) if this revision answers it". The model fixes it
in the same activation. Wait after a Write now records its why with the
revision (it was a no-op), so Write plus Wait is a valid fix.

(b) Visible backstop. When a Texture activation ends with no turn and the
runtime's Idle turn would leave the same unattended request, the Idle turn
commits and the work item is refused with the reason "no one is working on
this request: ...". The owner sees it on the document (the document response
carries `agent_request_unattended`, and the editor shows it), management sees
a system note in its inbox, and the owner's next revise reopens work. The
re-wake-once variant was not built: the guard's refusal is already the
re-prompt inside the activation.

## Residuals

- A Wait(why) that names something that does not exist is visible but not
  checked. The typed wait record (branch waits-semantic) is the place to bind
  a Wait to its holder.
- Stories and publications are exempt; their next actor is the capture and
  source schedule, not checked here.
