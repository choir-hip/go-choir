---
date: 2026-11-28
title: on Android a set-aside deck journal goes at sign-out
class: red
---
What: `DeckJournal.wipe()` in `mobile/exact-package/android/DeckJournal.kt` also removes `deck-journal.bad`, the one name `setAside` moves a corrupt journal to. It mirrors the iOS fix (824cc0011).

Why: a journal that failed to decode was set aside, and the sign-out wipe removed only the journal and its numbers file. The set-aside file holds the previous account's deck lines and stayed for the next account.

Proof: the new JVM test `DeckJournalTest.aSetAsideJournalGoesAtSignOut` (a corrupt journal through the real `setAside` path, then `wipe()`) was seen failing before the fix (21 tests, 1 failed) and passing after it. Run with `./gradlew -p scripts/android-deck-tests test` under JDK 21. Nothing ran on a device.

Replaced: a wipe that left the set-aside journal on the phone.

Undo: revert this commit.
