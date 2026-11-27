---
date: 2026-11-27
title: the programme types native code no longer reads are gone
class: orange
---
What: Swift loses the parts of the programme and run sheet types that only a test helper ever built: `RunSheetForecast`, `ProgrammeClipRef`, `IncomingBetween`, the clip, between, take, forecast, entry and claim fields of `IncomingSegment`, the clip branch of `SegmentPrep.prepare`, `ProgrammeSegment.clip` and `blockCountMismatch`, `SegmentKinds.isKnown`, and the Swift `StretchDecoder` test helper with the tests of it (the decoder, the clip segment, the forecast decode, the count mismatch). What stays in `Choir/RadioProgramme.swift` is what `ChoirApp.speakSegment` and the player use: `SplitBlocks`, `SegmentPrep`, the segment and document types. `SavedAnswerHolder`, `Conversation`, `EditionSetup`, `Takes` and the rest of `RunSheetTypes` change nothing: each is read by the voice turn, the deck or the take recorder.
Why: the app is Exact and only voice is native; the desk's stretch is decoded by `programme-model.ts`, and the forecast, clip and between fields are decided there. The Swift copy of them was never called by the app.
Proof: bun test 1834 pass (two tests added to `programme.test.ts`: the clip line with no clip dropped by the decoder, and the claim-only, full and absent forecast); each fails when its rule is broken. `swift test` 826 executed, 2 skipped, 0 failures (837 before; 11 tests of the deleted helper left, 3 title, cite and append tests rewritten on `IncomingSegment`). The native boundary gate went from 3182 to 3127 lines. A device must still show: a spoken entry speaks and the bar names its title.
Replaced: the Swift decoder helper and its tests, duplicates of `programme.test.ts`.
Undo: revert the commit.
