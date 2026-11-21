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

## Two builds, both sent back by both seats; the lane is stopped for a design (2026-11-21T05:32Z)

Lane `account-settle-memo`. Nothing of it is on main.

1. `eb0604d12` (three guards that reset the remembered answer at exits).
   Seat 2: one guard's retry wipes the current account's data on every page
   answer. Seat 1: three more paths where the answer stays remembered.
2. `579ceb2db` (the answer remembered only for the observed state; a list of
   stores still to wipe kept in the stamp). Seat 2: `app.ts:135-139` discards
   the settle result, so when a wipe or the stamp write fails the page source
   answers anyway and a listed store is readable by the next account. Seat 1:
   the same fail-open, plus a failed marker removal remembered as clean, two
   settles racing through an account change, and an unknown name in the list
   looping forever.

Belief. Both builds worked on what settle remembers. Neither changed what a
page source does when settle has not finished cleanly, and that is where the
data crosses: the gate fails open. The questions a design has to answer
before any code: (a) what a page source is allowed to read while the account
state is not known clean, and what the person sees then; (b) who is the one
writer of the account state, so two settles cannot race; (c) what is durable
and in what order, so a crash at any point restarts into a state that
refuses rather than reads; (d) how native stores (`HeldPages`, `TakeMirror`
and the rest of the wipe list) take the same boundary. The reviews are in
the session scratchpad (`review-acct/r1`, `seat1`, `r2`, `seat1b`).

Remaining error: the defect is still on main. It needs a sign-out whose
native half hangs or throws, then a second account without a relaunch; the
app has one user and is not launched.

Next: a written design (the account boundary in shared code), reviewed
before code, then tests from its table.

## Landing 1 built, not landed (2026-11-21)

Lane `account-boundary-l1`. Shared code and contracts only, from `docs/design/account-boundary-2026-11-21.md`:
the gate in `app.ts` is default-refuse with a named exempt list; the boundary is derived from the key and the
record before every gated answer (`account.ts`); one writer wipes, then removes the record; the marker file and
the remembered settle answer are gone. Seen: the whole `bun test`, the contract build and `test web` pass in the
lane. Not seen: the simulator drills D1 to D4 (written into `proof-real.sh`, not run), two review seats, a
device (the Exact error code on a data-protected file before first unlock is unread: design section 10). The
defect stays open for the native side until landings 2 and 3; Remaining error: a background Clear's deadline
timer may be cut when the answer that started it ends (Exact timers end with the answer, EXACT.md); a Clear that
hangs in native then holds the one writer until the host drops the call. Next: two seats, then the drills.

Recorded from seat 1's review of landing 1 (not fixed here; landings 2 and 3): native `wipe` answers `done:true`
unconditionally and sends no `failed` and reads no generation (`ModelsEntry.swift`); native stores swallow removal
errors; a launch-time wipe with key B also runs `MailSeal().forget()`; native read sources stay ungated until
landing 3; a theme installed from account A's Texture document stays visible to B (owner decision 4).
Fixed in landing 1 after the review: the proof's record checks are taken after their own drive; the epoch is
`state|wipes` so sign-in re-asks the pages; a throwing key read is LOCKED with words and buttons; the shared half of a
Clear has a deadline (`timing.sharedMs`) and counts as failed; the card's Adopt write is stated in the design.

Fixed in landing 1 after seat 2's review (fe027e0fc): a late read no longer plants the old account's data in a module
cache (`sameAccount()` in `api.ts`, one helper; files recents, podcast library and cached feeds, clips, mail folder,
alias and saved drafts, demo leftovers' write); the proof's D3 plants in a file `reset_state` leaves; a Clear the card
has seen running for six reads is abandoned on a fresh chain (no clock is read); `survivors` stats `content.db-wal`
and `-shm`; Texture-installed themes are wiped; the card never waits on a running job; sign-out forgets a refused key.
