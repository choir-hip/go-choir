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


## Round 2: the frame was wrong, not the guards (2026-11-21)

The first change (`eb0604d12`) reset the remembered answer at four exits. Seat 2 and seat 1 each sent it back with different blocking findings (seat 2: F1 to F4; seat 1: S1-1 to S1-4; recorded below),
so the exits are too many to enumerate, and a guard at one of them was itself a defect (F1). The frame changes. **A remembered answer is valid only for the state it was computed from, and the
settle step is idempotent.** No exit remembers anything.

### The design

1. **The memo is keyed on what settle reads.** `settleAccount` observes a tuple, the key (absent, unreadable, or present with its digest), the stamp file's content, whether the marker exists, the account
   generation and the wipe counter, and returns the remembered answer only if the tuple is the one a CLEAN settle left. Anything else recomputes. A failed settle remembers nothing; a settle that throws
   remembers nothing (so there is no permanently rejected memo, F4, S1-4).
2. **A sign-out in flight is a state.** While `accountSignOut` is running, `settleAccount` waits for it to return and then observes again (S1-3). A page source therefore cannot read A's stores while B's
   key is already in and the native half is still running; the wait is bounded by the sign-out's own deadline (30 s).
3. **Settle is idempotent: the stamp says whose data the stores hold, and a list inside the stamp file says which stores still hold the PREVIOUS account's.** An account change writes the stamp = the new
   key together with the list of every store (and the native half, and any path a wipe left behind) BEFORE it wipes anything, and strikes a name off as it clears. A retry (a transient native failure, a
   broken store) wipes only what is listed, so the new account's data in the stores that already cleared is never wiped again (F1). Stamp file format: `<digest>` or `<digest>` and a second line of
   comma-separated names. If the list cannot be written nothing is wiped (fail closed).
4. **Cost.** One native key read, one stat and one small file read per source answer (the tuple), where the memo used to cost nothing; a wipe runs only when the tuple differs from a clean one, and then
   only for what is listed.

### The table: every state of (key, stamp, marker, sign-out in flight)

Notation: `A` and `B` are two accounts; the stamp is the digest of the account whose data the shared stores hold. `key A, stamp A` and `key B, stamp B` are "same"; `key A, stamp B` and `key B, stamp A` are
"other" (an account change; the two are symmetric). Rows with the sign-out in flight differ only in that settle first waits.

| # | key | stamp | marker | in flight | settle does | a page source may read | how it heals | findings |
|---|---|---|---|---|---|---|---|---|
| 1 | absent | none | present | no | finish: native sign-out again (idempotent), then every shared wipe; the stamp and the marker are removed only when all of it succeeded | nothing: the call waits for the wipe and the page shows signed out | the next settle (a source answer, a relaunch) repeats finish until clean | O1 O2 S1-2 F3 |
| 2 | absent | none | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 3 | absent | none | absent | no | nothing (no account, no data, nothing unfinished) | nothing: the stores are empty | n/a | S1-1 (the state a clean sign-out leaves) |
| 4 | absent | none | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 5 | absent | A | present | no | finish: native sign-out again (idempotent), then every shared wipe; the stamp and the marker are removed only when all of it succeeded | nothing: the call waits for the wipe and the page shows signed out | the next settle (a source answer, a relaunch) repeats finish until clean | O1 O2 S1-2 F3 |
| 6 | absent | A | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 7 | absent | A | absent | no | finish: native sign-out again (idempotent), then every shared wipe; the stamp and the marker are removed only when all of it succeeded | nothing: the call waits for the wipe and the page shows signed out | the next settle (a source answer, a relaunch) repeats finish until clean | O1 O2 S1-2 F3 |
| 8 | absent | A | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 9 | absent | B | present | no | finish: native sign-out again (idempotent), then every shared wipe; the stamp and the marker are removed only when all of it succeeded | nothing: the call waits for the wipe and the page shows signed out | the next settle (a source answer, a relaunch) repeats finish until clean | O1 O2 S1-2 F3 |
| 10 | absent | B | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 11 | absent | B | absent | no | finish: native sign-out again (idempotent), then every shared wipe; the stamp and the marker are removed only when all of it succeeded | nothing: the call waits for the wipe and the page shows signed out | the next settle (a source answer, a relaunch) repeats finish until clean | O1 O2 S1-2 F3 |
| 12 | absent | B | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 13 | A | none | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then adopt: write the stamp = this key; no wipe | the stores hold no other account's data (a first launch, or after a clean sign-out) | none needed | S1-1 |
| 14 | A | none | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 15 | A | none | absent | no | adopt: write the stamp = this key; no wipe | the stores hold no other account's data (a first launch, or after a clean sign-out) | none needed | S1-1 |
| 16 | A | none | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 17 | A | A | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then nothing; if the stamp file lists unfinished stores, resume: wipe ONLY those (and the native half), never the stores that already cleared | this account's own data (a listed store may still hold the previous account's: the page is told its name) | each settle retries only what is listed, until the list is empty | F1 F2 O4 |
| 18 | A | A | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 19 | A | A | absent | no | nothing; if the stamp file lists unfinished stores, resume: wipe ONLY those (and the native half), never the stores that already cleared | this account's own data (a listed store may still hold the previous account's: the page is told its name) | each settle retries only what is listed, until the list is empty | F1 F2 O4 |
| 20 | A | A | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 21 | A | B | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then account change: write the stamp = this key AND the list of every store (and the native half) FIRST, wipe each and strike it off the list as it clears | only cleared stores (the new account's); a listed store may still hold the old account's data and the page names it | the next settle resumes the list only; the new account's data in cleared stores is never wiped again | F1 F2 O1 O2 O3 O4 S1-1 S1-2 |
| 22 | A | B | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 23 | A | B | absent | no | account change: write the stamp = this key AND the list of every store (and the native half) FIRST, wipe each and strike it off the list as it clears | only cleared stores (the new account's); a listed store may still hold the old account's data and the page names it | the next settle resumes the list only; the new account's data in cleared stores is never wiped again | F1 F2 O1 O2 O3 O4 S1-1 S1-2 |
| 24 | A | B | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 25 | B | none | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then adopt: write the stamp = this key; no wipe | the stores hold no other account's data (a first launch, or after a clean sign-out) | none needed | S1-1 |
| 26 | B | none | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 27 | B | none | absent | no | adopt: write the stamp = this key; no wipe | the stores hold no other account's data (a first launch, or after a clean sign-out) | none needed | S1-1 |
| 28 | B | none | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 29 | B | A | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then account change: write the stamp = this key AND the list of every store (and the native half) FIRST, wipe each and strike it off the list as it clears | only cleared stores (the new account's); a listed store may still hold the old account's data and the page names it | the next settle resumes the list only; the new account's data in cleared stores is never wiped again | F1 F2 O1 O2 O3 O4 S1-1 S1-2 |
| 30 | B | A | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 31 | B | A | absent | no | account change: write the stamp = this key AND the list of every store (and the native half) FIRST, wipe each and strike it off the list as it clears | only cleared stores (the new account's); a listed store may still hold the old account's data and the page names it | the next settle resumes the list only; the new account's data in cleared stores is never wiped again | F1 F2 O1 O2 O3 O4 S1-1 S1-2 |
| 32 | B | A | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 33 | B | B | present | no | clear the marker (the native half never deleted the key: nothing was signed out), then nothing; if the stamp file lists unfinished stores, resume: wipe ONLY those (and the native half), never the stores that already cleared | this account's own data (a listed store may still hold the previous account's: the page is told its name) | each settle retries only what is listed, until the list is empty | F1 F2 O4 |
| 34 | B | B | present | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |
| 35 | B | B | absent | no | nothing; if the stamp file lists unfinished stores, resume: wipe ONLY those (and the native half), never the stores that already cleared | this account's own data (a listed store may still hold the previous account's: the page is told its name) | each settle retries only what is listed, until the list is empty | F1 F2 O4 |
| 36 | B | B | absent | yes | WAITS for the sign-out in flight to return (its deadline bounds it), then observes again and acts as the row of the state it left | nothing: no source answers until the sign-out has returned | the sign-out's own result and the next settle | S1-3 (and the rows it leaves) |

Rows outside the tuple (faults), each with its test:

| # | fault | what happens | findings |
|---|---|---|---|
| X1 | a sign-out times out after the key delete, then B signs in (no relaunch) | the tuple differs (key B, stamp A, marker pending): clear-marker, account change; A's stores are wiped, the stamp is B's, the marker is gone | O1 |
| X2 | the native sign-out throws after the key delete, or answers "not done" | the marker stays or goes by the answer, the tuple differs at B's sign-in: as X1 | O2 |
| X3 | the late answer of a timed-out sign-out arrives with B's key present | it wipes nothing (B's data is B's); the next settle decides by the stamp: account change if A's data is still there, nothing if it is not. With the key absent it finishes the wipe and, if a store fails, leaves the stamp so the next settle finishes it | O3 S1-2 |
| X4 | a store fails to clear in an account change, or the native wipe returns false | the stamp is already B's, the list names the failed store or the native half; B writes; the next settle wipes only the listed names; B's data in the other stores survives | F1 F2 O4 |
| X5 | a clean sign-out whose stamp or marker removal fails | folded into `failed`, so the answer is not clean and nothing is remembered; the leftover stamp A with the key absent is finished by the next settle | F3 |
| X6 | `reconcile` or the old-copy cleanup throws | nothing is remembered; the next source answer settles again | F4 S1-4 |
| X7 | a clean sign-out of A writes no stamp for B (the memo used to be kept), B signs in, B writes, B signs out uncleanly, C signs in | B is stamped at its first settle (adopt), so C's sign-in is an account change that wipes B's data | S1-1 |
| X8 | settle is asked while signed out (a `signedOutTick`), then B signs in | the tuple has changed (key B): not the signed-out answer | S1-1 S1-2 |
| X9 | two sign-outs overlap | the second waits for the first (one in flight at a time); each leaves its own marker token, and a late answer touches only its own | seat 2 table |

### Findings of both reviews, mapped

- Original (the lead's record above): O1 timeout, O2 throw or not-done, O3 the late continuation does not check whose key, O4 a failed re-settle is remembered. Rows: X1 X2 X3 X4.
- Seat 2, review 1: F1 the retry wipes the current account's data on every source answer (reproduced): X4 and the "same, list" rows; F2 the tests hide F1 (test 6 never checks B's data survives, test 5 is
  misdescribed): the table tests assert the data survives and the account state at each step; F3 a clean sign-out ignores a failed stamp or marker removal: X5; F4 a throwing reconcile leaves a rejected memo: X6.
- Seat 1: S1-1 a clean sign-out or a signed-out settle leaves a memo with no stamp, a later unclean sign-out leaks: X7 X8; S1-2 a late continuation with the key absent leaves the memo when its wipe
  fails, and `signedOutTick` can re-memoise before the key delete: X3, and the tuple (the key delete changes it); S1-3 the memo stays during the 30 s window: the in-flight rows; S1-4 a rejected memo
  sticks: X6.
