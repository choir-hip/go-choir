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

## The shape of the solution (owner, 2026-10-27, refining the first draft)

"It wouldn't really be for you as the external agent to make a
pre-commitment. That may be, but it would really be that the engineering
would be, yes, it would be making an event on the tape, but the event on the
tape would also be resolving the pre-commitment as basically a failure. And
that is what would be sent as a notification to the auto radio app."

So the commitment belongs to the desk doing the work, not to the
requester:

- **The commitment.** Engineering precommits to its outcome when it takes
  the assignment, for example "a frozen candidate that passes these checks".
  It already makes rule-settled precommitments in its capsule (Freeze,
  Verify, the assignment's end; see
  [design/psa-precommitment-icrl-status-2026-10-24.md](../archive/design/psa-precommitment-icrl-status-2026-10-24.md)).
- **The settlement.** When the work stops, engineering's event on the tape
  (blocked, capsule revoked) also resolves that precommitment as a failure.
  The stop and its settlement are one fact, not two.
- **The notification.** The settlement is what reaches the owner, as a
  notification in the Autoradio app. Delivery is a projection of the
  settled record, never a second state authority. An outside agent that
  started the round reads the same settlement, but its own precommitment
  is optional, not the mechanism.
- **ICRL.** A settled failure in the desk's next context is the in-context
  learning loop the status document finds open (precommitment-records.md
  §1.2).

## Open questions for the design

1. **Whose commitment: Texture's or engineering's?** (Owner, 2026-10-27:
   "is it engineering's pre-commitment or is it textual [Texture's]? I'm
   not quite sure.") There are two layers:
   - Texture's commitment to the owner, the round's promise ("this change,
     proved, frozen for approval"). The round opened in the owner's
     Texture document, and Texture renders the commitment ledger as the
     supervision surface.
   - Engineering's commitment to management, its assignment in one
     capsule.

   Engineering's stop settles its own commitment as a failure. It does not
   by itself fail Texture's, because management may retry in a fresh
   capsule (it did on 10-27). The open questions are which settlement
   notifies the owner, or both with different weight, and which events
   settle each one: Freeze, blocked, revoked, Verify, the owner's decision.
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
for a frozen candidate.
