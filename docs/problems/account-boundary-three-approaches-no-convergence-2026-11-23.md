# The phone's account boundary: three approaches, twelve rounds, no landing

2026-11-23. A structural assessment under the operating contract's rule 7
(three rounds without convergence: stop patching, change the approach). It is
the third. The first two changed the approach and were each followed by more
rounds, so this one questions where the boundary is drawn, not how.

Status: `lane/account-partition-s1` (head `ed772eee1`) is frozen. Nothing in
it lands as it stands.

## Problem

"The phone must never show one account's private data to another." The work
to make that true has had about twelve review rounds and four recorded failed
drill runs across three approaches, and no version has completed its own
drills. Main still signs out inside the running process with the old wipe.

## Evidence

The three approaches:

1. **A seal** (rounds 1 to 6, 11-21 to 11-22). Wipe at the tap, inside the
   process, and make late writers fail. Every round found a writer or a path
   the seal did not cover.
2. **The process ends; the next launch wipes** (rounds 7 to 11). First with
   two deciders (native and shared code), then with one native verdict.
   Round 9 found six new combinations, none a regression. Round 11 was
   accepted with fixes by one seat and its drill failed on a simulator
   Keychain write (-25300) whose cause was never proven.
3. **The key in the framework's store; the verdict in shared code** (11-22,
   `8ae1e0aa0` and four commits after). Never reviewed. Three drill runs
   found three defects in facts about the framework that 1,166 passing tests
   had hidden: the store answers only in an answer's first turn; a dropped
   answer dies mid-launch; native was never built in the launches that opened
   the account.

The findings grouped by root, with approximate counts from the review outputs
of rounds 5 to 11 and the records for the rest:

| Root | Count | Approach 1 | Approach 2 | Approach 3 |
| --- | --- | --- | --- | --- |
| A writer outlives the account (a late write, a cached path, a poll in flight, the deck) | 9 | created | reduced | as 2 |
| A second party reads the key or mark and decides | 10 | none | created, then cut to one | native's `ready` and held key remain a second party |
| State unreadable before the phone's first unlock | 4 | none | created | open: absent and not-yet-readable both read as null, and the verdict resolves it by waiting, which fails open |
| The wipe is incomplete (the listing, swallowed errors, shared roots, native stores) | 7 | created | partly removed | the sweep is shared code; native forgets only its own files |
| Tests or proofs touch real stores | 3 | created | fixed | none |
| The page or a doc promises something unproven | 6 | none | created | unreviewed |
| The drill itself was wrong | 8 | drills never ran | one | several |
| A framework fact learned only from a drill | 3 | none | one | created |

Each approach removed one seam and opened another. Reviews kept finding new
combinations, not repeats.

What is true on main today: three native pushes (the radio set, the edition
mirror, page text) carry an account generation and are refused when stale.
The deck, retrieval answers, takes, onboarding and one busy flag are not
stamped; a reload can admit an old queued push; a failed boot is logged and
not retried
([record](native-pushes-without-an-account-generation-2026-11-23.md)).
Nothing has been shown to land wrongly.

The owner's words, 2026-11-21: "hmm, is maybe the design a problem? should we
really be going through the account gate so much?"; "we should allow users to
be signed in to multiple accounts, as we do on choir.news. but that is a
separate thing from signing in and out without disrupting service"; "i never
asked for signout without disrupting service". 2026-11-22: "signing in again
is fine".

## Belief

The lead's reading, at about 0.75: the non-convergence is a property of where
the boundary is drawn. One process holds two runtimes (native Swift and the
shared code's JavaScript) over one set of files, and all three approaches ask
that process to become a different account's process, or to stop being one,
by its own code. Every finding sits at a seam between the two runtimes or
between how long the process lives and how long its files live.

Four things no mechanism inside the process can guarantee:

- **Ending the process.** An app has no sanctioned way to quit itself, so
  "one account per process" rests on the person closing the app. Until then
  native still holds the old account's audio session, voice context, held
  pages, the deck and the key in memory.
- **Data at rest between the tap and the next launch.**
- **Before first unlock**, the app cannot tell "no key" from "cannot read
  yet".
- **A reload of shared code under a live native process.** The developer
  menu does this today; whether a production path does is not verified.

## The candidates the operating system enforces

Not designs. Facts for and against, and what is not known.

- **One install per account.** The build already has variants with their own
  bundle id, data container and Keychain scope ("Autoradio 1" is one).
  Sign-out is deleting the app. Against: variants are fixed at build time; the
  sign-in return scheme is per app id; free signing lasts seven days. It does
  not give the owner several accounts in one app.
- **Process exit, then rename the whole container aside at launch before any
  store opens**, and delete it later. One atomic rename replaces
  list-and-verify. Against: still app code at launch, still subject to first
  unlock; whether native can run before the framework opens its stores is not
  known.
- **Encrypt each account's data under a key the Keychain holds; sign-out
  deletes the key.** One atomic act, in process, no file listing, and it is
  the only candidate that fits several accounts held at once. The mail seal
  is the precedent in this repo. Against: the framework writes its own stores
  and no facility for encrypted storage in it was found; hand-built
  cryptography in shared code is refused by standing rule, so it would be
  native.

## Decision (the lead, default until the owner rules)

1. No fourth in-process mechanism. The lane stays frozen as the record of
   what was tried; its drills and its test isolation are worth keeping.
2. What the product can honestly claim today is **one account per install**,
   and the owner's phone has one account. That is the conservative default.
3. Before any build, three facts are read, not guessed: what the framework
   stores and where, and whether a native step can run before it; the file
   protection class of its data before first unlock, on iOS and Android;
   whether any production path reloads shared code under live native code.
4. The choice among the three candidates is the owner's, because it decides
   whether several accounts in one app is a goal. Their words on 11-21 say it
   is, which points at the third candidate.

## Remaining error

- Rounds 1 to 4 are known from the earlier record, not from review outputs.
  The fifth failed drill run could not be found; four have evidence.
- Approach 3 was never reviewed, so "it would not have converged" is an
  inference from its drill failures, not a finding.
- "Sign-out may stop the app" is recorded as a paraphrase in the design
  document; the owner's own sentence was not found.
- The two earlier assessments and the design document
  (`docs/design/account-boundary-2026-11-21.md` as amended) exist only on the
  lane, not on main.
- The lane is based on the first train; main has since merged the second,
  which still carries the old wipe.
