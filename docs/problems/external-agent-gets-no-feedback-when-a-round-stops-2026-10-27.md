# An external agent gets no feedback when a round it started stops (2026-10-27)

Status: open; the solution needs design. Mutation class: unknown until it
is designed. Documented before any fix.

## Evidence

On b@c, FP0 (Claude Code, an agent outside the computer) started the
Texture citations round through the CLI (`choir texture create`, 00:07 UTC,
trajectory 150b7de5…). At 00:28 to 00:29 the round stopped: engineering
was blocked, the capsule was revoked, and management settled the round
waiting on the requester. FP0's watcher polled only for a candidate
awaiting approval, so it reported nothing for 40 minutes. The owner
noticed first, from the Texture document on their phone (see
[capsule-cannot-restore-a-path-from-its-base-2026-10-27.md](capsule-cannot-restore-a-path-from-its-base-2026-10-27.md)).

## The problem

Nothing tells the requester that its round has stopped. Some pieces exist:

- `choir texture watch <doc>` streams the document's durable events as
  JSONL;
- `choir lifecycle events` reads reducer events after a cursor.

But "this round now needs you" lives only as prose in a turn ("waiting:
Await …"). An agent outside the computer has to poll, then guess from the
wording. Silence looks the same as progress.

## Owner direction (2026-10-27, in session)

"We should note this as a problem, but recognize that the solution is a
deeper integration of pre-commitment records and PSA. We have to think
about this more deeply."

## Why precommitment records are the frame, not a notification channel

A notification channel (a typed attention event, a `wait` command, a
webhook) would answer "how do I hear about it". It would not answer "what
did I expect, and what do I learn when it does not happen".
Precommitment records ([precommitment-records.md](../precommitment-records.md);
status in
[design/psa-precommitment-icrl-status-2026-10-24.md](../design/psa-precommitment-icrl-status-2026-10-24.md))
already hold that shape:

- **The requester's side.** The requester commits to an expectation when
  it submits. For example: "a frozen candidate with these checks by T,
  p=0.7". The round stopping is then a settlement of that commitment,
  false, scored. The feedback is that settlement, addressed to the
  committer. Calibration level 2 (`/api/precommit`, `choir precommit`)
  exists for agents outside the desks, and nobody uses it routinely.
- **The round's side.** Engineering already makes rule-settled
  precommitments in its capsule (Freeze, Verify, the assignment's end).
  Management's "waiting on the owner" is itself a claim about who must act
  next, and it was wrong here: the base tree was the recovery source.
  Recorded as a commitment, that claim could be settled and scored.
- **ICRL.** A settled-false commitment fed back to the committer's next
  context is the in-context learning loop the status document finds open
  for every desk. It is open for agents outside the computer too.

## Open questions for the design

1. **Commitments at submit time.** Should a round's submit carry the
   requester's precommitment (expected outcome, deadline, probability), so
   that a stop or a missed deadline settles it?
2. **Delivery.** How does a settlement reach a committer that is outside
   the computer and may be offline: a stream it subscribes to, MCP, the
   owner's app notifications (Autoradio)? Delivery should be a projection
   of the settled record, never a second state authority.
3. **Desk claims as commitments.** Which of a desk's claims should be
   commitments that rules can settle? Candidates: "blocked", "waiting on
   the owner", "frozen", "verified". Management's claim that the owner had
   to supply a recovery source would have settled false.
4. **One record for several watchers.** How do the requester, the owner,
   and progressive delivery's ring controller (which reads open, unsettled
   expectations as a guard signal) read the same record without each
   building a watcher?
5. **Silence as a settlement.** Is a missed deadline itself a settlement
   ("expected by T, no outcome")? That would make silence impossible to
   mistake for progress.

## Interim practice (FP0)

Tail the round's trajectory turns and capsule fates, and do not watch only
for a frozen candidate. Consider recording a `choir precommit` for each
round FP0 starts.
