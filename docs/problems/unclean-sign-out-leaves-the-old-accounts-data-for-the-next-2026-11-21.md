# A sign-out that does not finish cleanly leaves the old account's data for the next account

Recorded 2026-11-21T04:04Z by the lead, from seat 2's sixth review of the takes lane. On
main today (`mobile/AutoradioLab/account.ts`). Not fixed; the fix is its own
red change (lane `account-settle-memo`), not part of the takes move.

## Problem

Shared code settles the account once at launch and remembers the answer
(`settled`, read before any page's source runs, `app.ts:135`). A sign-out
that ends other than cleanly leaves that remembered answer in place. If a
second account then signs in without a relaunch, the settle step answers
"nothing to do", the first account's shared files and stamp remain, and the
second account's pages read them.

## Evidence

The reviewer ran the real modules with the test phone's own failure switches:

- sign-out times out after the key is deleted (`account.ts:244`): result
  `done:false, failed:''`, marker and stamp still the first account's;
- native throws or reports not done (`:246-251`);
- the late continuation (`:240`) returns on any present key without checking
  whose, so a second sign-in in the window suppresses the late wipe;
- a failed re-settle is itself remembered (`settleAccount`, `:266`).

In each: key = B, `settleAccount` returns the launch answer, action
"nothing", and `app:/data/takes.json` and `podcasts-library.json` of A are
still present.

## Belief

Reachable only when the native half of a sign-out hangs or throws after the
key delete, and the app is not relaunched before the next sign-in. Rare, but
it is private data crossing accounts, which the product forbids outright.
The app is not launched and has one user, so nothing has leaked.

## Remaining error

Whether the account page offers sign-in while a sign-out is still pending
was read, not run. The same review notes `Conversation.begin` has no
generation guard (a late question could be added after a sign-out); not
reproduced.

## Next

Tests for the four exits first, then: forget the remembered answer on every
unclean result, forget it when a settle reports a failure, and make the late
continuation check whose key it sees. Red: two seats, proof on the simulator
drill.
