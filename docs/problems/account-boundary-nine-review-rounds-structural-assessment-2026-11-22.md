# The account boundary has been sent back nine times: structural assessment (2026-11-22)

Status: open. Written under the operating contract's rule 7 (three rounds
without convergence: stop patching, change the approach). Candidate
`3f640d366` on `lane/account-partition-s1` is not landed.

## What happened

Nine review rounds. Rounds 5 to 7 found crashes and early clearing of the
sign-out mark. Round 8 found a launch that opened the account without asking
native. The shape was then changed to two rules (one gate, one clearing
point). Round 9 (seat 2, on `3f640d366`) confirms those two holes are closed
and finds six new ones, none of them a regression:

1. A gated answer does not re-check the key, so an account stays open if the
   key changes under it.
2. A Keychain fault on a launch with no mark shows the "unlock" card.
3. The fault card promises that reinstalling clears it; not proven.
4. The poll for native's launch answer never stops.
5. A sign-in poll already in flight can store a key while a mark exists.
6. Native answers "no mark" for good if the mark file is not readable before
   the first unlock; the account then opens with the mark still there.

## Why it does not converge

Every round finds a new combination, not a repeat. The boundary is decided in
two places from five facts that each change on their own clock: the mark, the
key, the phone's lock, which host is running, and time. Shared code re-reads
the key and re-derives the state whenever it is asked (`settle`, `peek`,
`enter`, `keyFact`, `wipeFact`), and native decides some of the same things
at launch. Two deciders over five facts give more combinations than a review
can enumerate, and each fix adds a state (`changed`, `fault`, `notready`,
`locked`, `timeout`, `nomodule`).

## The change of approach: one verdict, made once

A process belongs to one account for its whole life.

- **Native makes one launch verdict, once, and only when the phone's
  protected data is available.** Until then the answer is "not ready". The
  verdict is one of: `anonymous`, `account(digest)`, `blocked(why)`. Making it
  includes finishing a pending sign-out. Native keeps the verdict for the life
  of the process and answers every later ask with the same value.
- **Shared code holds the verdict. It never reads the key or the mark to
  decide anything.** `keyFact`, the key check in `settle`, `peek`'s re-check
  and the `changed` ending are deleted.
- **Two transitions, both one-way, both made by the code that performs
  them:** `anonymous → account(digest)` when sign-in stores a key (native
  refuses to store one in any other state, checked immediately before the
  store), and `account → ended` on sign-out. Nothing else changes the state
  in a running process. A key that changes by any other route is not looked
  for, because nothing in the app can cause it.
- **One blocked card.** It says what is true ("Autoradio cannot read its
  sign-in on this phone. Unlock the phone and open Autoradio again.") and
  promises nothing unproven. The reason is logged, not shown as different
  cards.
- **The ask for the verdict is one in-flight call with a deadline inside
  it.** No loop outlives its deadline.

Against round 9: 1 cannot occur (there is no second key read); 2 and 3 are
one card; 4 is the single bounded ask; 5 is the check before the store; 6 is
the protected-data condition.

## What this costs

Signing in while signed in to another account stays impossible without
closing the app (owner, 2026-11-22: sign-out may stop the app). A key removed
by something outside the app is noticed at the next launch, not before.

## Proof it needs before landing

The existing drills 1 to 6, and two new ones: a launch with a mark before the
first unlock; and a test that every account-gated answer refuses in every
state except `account`. Then seat 1 and seat 2 on the frozen candidate.

## Remaining error

Whether Exact lets a native call hold its reply until the verdict exists, or
shared code must ask again, is not known (read the Exact reference page).
Whether protected-data availability can be read at the point Exact loads the
module is not known. Native stores that are not partitioned by account
(the speaker profile) remain as recorded for station 2.

## What was built (2026-11-22, lane `account-partition-s1`)

The shape above, with these decisions made while building it:

- **One verdict, native's** (`AccountWipe.decide`, answered by the `verdict` op in `ModelsEntry`'s `call`: only native can meet it, it needs the Keychain and the phone's protected-data state). `anonymous`, `account(digest)`, `blocked(why)`, then `ended` after a sign-out. Shared code (`api.ts` `askVerdict`) asks, holds it, and never reads the key or the mark.
- **Not ready** is answered at once and never cached when ANY of three facts fails, behind one seam (`AccountWipe.Readiness`): UIKit's `isProtectedDataAvailable` (main thread), a Keychain read that is not `errSecInteractionNotAllowed`, and the mark read by reading the FILE (metadata can be visible before first unlock while contents are not). Each is driven separately by a test.
- **Exact cannot hold a native reply past the turn budget** (`docs/reference/exact/internals.md`: an overrun fails the resource and it is asked again at 250 ms, then 1 s), so native answers `notready` instantly and the contract's re-ask drives it: one native call per ask, none once the verdict is known.
- **Rule B reverts**: native removes the mark once ITS OWN listing (the account folder, the old paths, the extra paths and the shared account roots from Exact's own `context.data`, `.cache`, `.temporary`) is clean. The pre-partition top-level leftovers (`app:/data/takes.json`, `mail`, `paper-front.json`, `content.db*`, `vectors`, `exports`, `app:/cache/mail-*` and the other entries of `LEFTOVERS` in `account.ts`) are not the signed-out account's data: they are files of builds before the partition, no current store writes them, and shared code sweeps them at EVERY launch, signed out or not. That sweep finishes before any world opens, the anonymous one included, and a failure is `blocked`.
- **T1** is `AccountWipe.store`: the verdict must be `anonymous` under the lock that stores; a failed store leaves it `anonymous`; success makes it `account(digest)` in the same critical section. **T2** is `AccountWipe.end()` at the start of `ChoirSignIn.signOut`, so a sign-in poll already in flight cannot store a key.
- **Deleted**: `keyFact`/`keyState`/`KeyFact`, the key check in `settle`, `peek`'s re-check, the `changed` ending and its card, the fault/locked/unlock/wipe card split (one `blocked` card), the unbounded `wipeFact` poll, `leftover`/`finishMark`, the `nomodule` timer branches, and (in this commit, dead code found while staying under the native ceiling) unreferenced functions and enums in `RunSheetEntries`, `PodcastClips`, `VoicePreparation`, `PlaybackSupport`, `Moment`, `PaperModel`, `Learning`.
- **Unproven until a real phone**: the true before-first-unlock case (owner check, drill 2d), `isProtectedDataAvailable` against Exact's file-protection class, and the `context.data/ending` mapping (drill 2c).

## Round 12 (2026-11-22): what changed and why

The decision of round 9 (one launch verdict, made once by native) held for two rounds and produced the same class of defect: native had to decide, from the Keychain and a protected-data fact, what shared code needed to know, and every seam between them (the readiness facts, the lock, `notready` never cached, the sign-in's store, the proof key's import ordering) was a place for a race. The drill of round 11 then failed on the simulator's Keychain write (-25300), a fact no unit test could have shown.

The owner found the frame change: **the key belongs to Exact's store** (`secret.keep`), which shared code reads synchronously and writes in one call. With the key there, the verdict is a pure function of the store, two markers and the sweep, made in the one place that already held the invariant (`account.ts`); native holds nothing and decides nothing. Of the round 9 to 11 findings: the readiness seam, protected-data check, `notready` caching, T1/T2 lock, the proof key's import race, the unconfigured-process and mid-decision-lock cases, the bounded revoke semaphore, `signOutWithoutModule` and the blocked-versus-fault split are **gone by construction** (the code does not exist); the shared-side findings (inert chains, the gate, the world watcher, the sweep before any world) are **kept as built**; **still open**: the file-protection class of `app:/data` before first unlock, Android, `holdKey` as a residual until station 2, and the drills, which have not run on this design. See `docs/design/account-boundary-2026-11-21.md`, "Round 12".

