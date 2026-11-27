# The radio's sentence follow in shared code: parked after four review rounds

Standing: governing | kind: happened | settled-by: four reviews by seat 1 (Gemini 4 Hydrogen) of origin/lane/bar-facts, the last on 2da2c0351, 2026-11-27 | note: the lane is kept, not landed; nothing in it was run on a device

Station E5 of
[finishing the move to Exact](../definitions/choir-finish-the-move-to-exact-2026-11-27.md)
tried to move, from Swift into shared code, the logic for which sentence
the radio is saying and which block of a reader page that is. The change
is on `origin/lane/bar-facts` (three commits, head `2da2c0351`). It would
take non-voice Swift from 3,127 lines to 2,951. It is not on main.

## What holds

- 19 fields of `RadioInfo` that no shared code reads are deleted across
  the boundary. This part was never disputed in review.
- The sentence splitter and the weighting are in `radio-follow.ts` with
  the Swift tests' cases ported, and seven behavioural tests of the path
  that lights a sentence, each shown to fail under a mutation.
- Sign-out clears what shared code holds. The reviewer closed this.
- `bun test` 1858 pass; the contract builds; `test web` fails only the two
  cases that fail on main.

## What does not

Two races were still open at the fourth review:

1. **Native reads the document and the position under two different
   locks.** `ProgrammePlayer.place` releases its lock before
   `BarTracker.blockOnAir` asks the position source, and a position
   carries a block index and no document. A document replaced in between
   gives one document's words with the other's place.
2. **The entry is stamped on arrival, in shared code.** Native does not
   know which run-sheet entry it is speaking for. If the entry on air
   changes to another entry of the same story while a read is in flight,
   the old words are stamped with the new entry and pass the identity
   check.

Both last about one pulse (a second) and light a wrong sentence; neither
loses or leaks anything. They are still the kind of defect this change
introduced rather than inherited: the Swift it replaces held all of this
behind one lock.

Also unexplained on the lane's head: one failure in
`DownloadDoorTests.testW5...` (the same class passes on main, 34 tests, and
the lane touches no download code), and the whole Swift suite ran only with
`MailSealTests` skipped, because those Keychain tests hung twice at 0% CPU
(they hung once on another lane the same day, and passed in other runs).

## Why it is parked and not patched again

Each round closed what was found and opened something new: round 2's
caching protocol, which the lead asked for, was the worst of it and was
deleted in round 3. The operating contract says to stop after three rounds
without convergence and change the approach.

The pattern in rounds 2 to 4 is one thing: the facts needed to light a
sentence (which entry, which document, which block, where in it) are owned
by two sides, and every fix moved the seam without removing it.

## The approach to take when it is picked up

- Native answers with ONE value read under ONE lock, and that value
  carries its own identity: the position source must return the document
  id with the block index, so words and place cannot come from different
  documents.
- Shared code tells native which entry it handed over, and native returns
  that entry id with the answer. Shared code stamps nothing on arrival.
- Land the uncontested part first and alone: the 19 dead fields.

## Reviews

The four review outputs are in the session's scratch area and are not
kept. Their findings are the content of this file and of the lane's
change-log entry.
