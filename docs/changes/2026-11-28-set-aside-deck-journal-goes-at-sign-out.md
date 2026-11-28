---
date: 2026-11-28
title: a set-aside deck journal goes at sign-out
class: red
---
What: `Deck.wipe()` (via `DeckJournal.wipe`) also removes `deck-journal.bad`, the one name `setAside` moves a corrupt journal to. `scripts/native-stores.txt` and the native-stores inventory now rule that entry `wiped:deck`.

Why: a journal that failed to decode was set aside, and the sign-out wipe removed only `deck-journal.json` and `deck-journal.seq.json`. The set-aside file holds the previous account's deck lines and stayed for the next account.

Proof: the new test `DeckTests.test19b_ASetAsideJournalGoesAtSignOut` (a corrupt journal through the real `setAside` path, then `AccountWipe.run`) was seen failing before the fix and passing after it. `python3 -I scripts/check-native-stores` and `scripts/check-entries` pass. Nothing ran on a device.

Replaced: a wipe that left the set-aside journal on the phone.

Undo: revert this commit.
