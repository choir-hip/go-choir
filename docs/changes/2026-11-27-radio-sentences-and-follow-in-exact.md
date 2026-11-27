---
date: 2026-11-27
title: the radio's sentences, progress and reader follow-along are shared code; the radio view across the boundary is five facts
class: orange
---
What: native `radioFacts` now hands over facts only: the spoken words of the block on air (`spoken`), its number and the count (`block`, `blocks`) and the audio's place in it (`blockFraction`). The sentence split, the sentence the place falls in, the programme progress and the block and sentence of the page in front that the radio is saying are made in `mobile/AutoradioLab/radio-follow.ts` (cases in `radio-follow.test.ts`), on the one splitter the reader uses. `readerFollow` is a shared source (`radio-bridge.ts`), asked with the radio's own pulse, and no longer a native op; the hand-over of the page to the voice no longer carries sentences. `RadioInfo` across the boundary shrinks from 24 fields to the 5 shared code reads (title, playing, level, speedValue, micLevel). Swift deleted: `BlockClock` and the play-time estimate (the audio position is always set in production), `BarTracker.sentences/sentenceIndex/speedName/clockProgress`, `ReaderFollow` (a three-line shell stays only because `AccountWipe.swift` still names it), `ReaderFeed`, `HeldPages.normalised/all/body`, `PlaybackSpeed.name`, and the tests of only those.

Why: the phone app is Exact and only voice is native; these were logic (names, sentence splitting, progress arithmetic, matching words to blocks) computed in Swift for shared code to read back. What stays native is plain fact: the microphone level, the speed the player is at, the audio's own position, the document only native holds, the engine's events.

Proof: `bun test` in `mobile/AutoradioLab`, `bun exact.mjs contract build app.contract`, `bun exact.mjs test web`, `swift test` of `mobile/AutoradioKit` through the lock, and the native boundary and entry gates, with the numbers in the landing report. Three of the new behaviours were broken on purpose and their tests failed. Not proved: a device build. A device must show the reader lighting the sentence the radio is saying (it now follows the radio's per-second pulse, not each redraw), the same sentence on the radio page, the bar's progress and the mark breathing with the microphone.

Replaced: the Swift sentence split, sentence index, progress, `ReaderFollow`/`ReaderFeed` and the native `readerFollow` op. The radio's sentences are now cut by the reader's regex splitter, not the platform's, so a split may differ from before at an odd abbreviation.

Undo: revert this commit.
