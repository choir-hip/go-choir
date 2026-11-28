---
date: 2026-11-28
title: The iPhone app compiles again
class: orange
---

What: `AutoradioModule+Deck.swift` calls `Deck.shared.boot()` on the main thread, the way the line after it already reaches the deck.

Why: the Android deck landing ce79438a8 added the call from a function that is not on the main actor, which Swift refuses. No iPhone app could be built from main for 31 hours, and no check noticed ([the record](../problems/main-broke-the-iphone-app-build-for-a-day-and-no-check-saw-it-2026-11-28.md)).

Proof: the simulator build (`build-lab.sh ios`, lab-one) failed at 00aedd0bc with that error and completed with this change. Not run on a simulator or a phone.

Replaced: the direct call.

Undo: revert this commit; the app then does not compile.
