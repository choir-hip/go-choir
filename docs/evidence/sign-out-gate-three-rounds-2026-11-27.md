# The sign-out order fix: three review rounds without convergence, and the change of approach

Standing: governing | kind: happened | settled-by: three reviews each by seat 1 (Gemini 4 Hydrogen, diff only) and a fresh Sonnet whole-tree reviewer of lane sign-out-order, the last on f56dbd470, 2026-11-27 | note: nothing in the lane has landed or run on a device

Station E6.3 of
[the move to Exact](../definitions/choir-finish-the-move-to-exact-2026-11-27.md)
fixes the first item of
[the sign-out problem](../problems/phone-sign-out-leaves-the-microphone-open-past-the-key-2026-11-27.md):
on main, sign-out deletes the account key while the microphone may be open
and a voice turn may be in flight. The contract says three rounds without
convergence end the patching. This is the assessment it asks for.

## What happened

| Round | Head | What it built | Why it was sent back |
| --- | --- | --- | --- |
| 1 | b7a48d19f | Sign-out closes hands-free, the turn and the microphone, then deletes the key; a gate refuses new turns | A take's give-back turned hands-free on after the close; the gate was checked only at turn entry; the gate's own checks raced |
| 2 | cd96c19ab | One epoch; checks at every stage; the gate reopens only for a key whose identity differs; sign-out deletes only the key it began with | A take could start during sign-out; the "only the key it began with" rule (the lead's steer) left the old account's data readable by the new one and the old key unrevoked; checks not atomic; an old turn could be sent under a new account's key; the gate could stick shut on a second sign-out |
| 3 | f56dbd470 | Identity and the supersede rule removed; one flag and one epoch under one lock; requests carry their turn's epoch; takes refuse | The native op shuts the gate before it knows a sign-out will run. Two early returns (no module; a superseded key) never run the sequence, so nothing reopens it: after sign out, sign in, a stale sign-out op leaves every turn, hands-free and take refused until the app restarts. Verified in `ModelsEntry.swift:53-57`. Worse than main |

Each round closed what the last review found and each review found something
new of the same kind.

## What the three rounds have in common

Every blocking finding after round 1 is one defect in different places: **the
gate is a latch that one piece of code sets and a different piece of code is
trusted to clear.** Round 1 set it in sign-out and cleared it by reading the
key store. Round 2 cleared it by comparing identities. Round 3 set it in the
native op and cleared it at the end of a task that the op does not always
start. A latch with two owners sticks or opens early whenever a path between
them was not thought of, and review finds those paths one at a time.

The second common cause is the lead's: twice the lead added requirements
during review (the supersede rule in round 2; "shut synchronously where the op
arrives" in round 3) and each produced the next blocking finding. The change
grew from 12 files and 632 lines to 13 files and 908.

## What is not in doubt

Reviewed three times and never faulted:

- the order: hands-free off, the turn cancelled, the microphone closed, a
  bounded wait for shared code, then the key deleted;
- the hands-free switch refusing under its own lock;
- the enrolment microphone tap cleared at sign-out;
- a take refusing to start while a sign-out runs;
- a request carrying the epoch of the turn that made it.

## The change of approach

The gate stops being a latch. **"A sign-out is running" is the fact that the
sign-out task exists**, read under the lock that guards the task slot. Nothing
sets a flag and nothing clears one:

- the task is created synchronously, under the app's lock, by one function,
  and that function is the only thing that bumps the epoch;
- the slot is emptied by the task's own last statement, under the same lock;
- the native op decides whether a sign-out will run (module present, key not
  superseded: both synchronous reads) before it calls that function, so the
  gate cannot shut for a sign-out that never starts.

A path that does not start the task cannot shut the gate, and a task that ends
cannot leave it shut. The class of defect the three rounds found is removed,
not patched.

Everything else in the lane is kept as reviewed. No requirement is added.

## What happens next

One build on this approach, then both seats. If either sends it back for a
defect of the same kind, the lane is parked as it is, the order fix is cut
down to the five parts listed above with the gate removed, and the question
goes to the owner with this record. The defect on main is unchanged meanwhile;
the app has no users.

## Not settled by any round

- Whether anything restarts the radio after the fallback close at sign-out.
  The builder says the next play or question does and names a test; the
  whole-tree reviewer could not confirm it. The next review is asked for the
  call site.
- A sign-in that lands while a sign-out is running is destroyed with it. This
  is main's behaviour, recorded in the problem record, and waits on E6.
- `MailSealTests.testK3TwoFirstUsesAtOnceShareOneKey` hung at no CPU again and
  was skipped in round 3's whole-suite run. The cause is not known.
