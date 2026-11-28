---
date: 2026-11-28
title: The sentence follow is shared code
class: orange
---

What: (first slice, S0) `RadioInfo` no longer carries `progress`, `sentenceIndex` and `sentences` across the boundary. They are gone from `interface/bar.json` and `exact.json` and from the Swift struct's encoding (`CodingKeys`); the struct still holds them natively, because `radioFacts` and `ReaderFeed` read them until the follow moves (the next change).

Why: nothing in shared code reads them from `radioView` (the contracts read `title`, `playing`, `level`, `speedValue`, `micLevel`). The second slice of the sentence follow plan ([the plan](../design/sentence-follow-in-shared-code-2026-11-28.md)).

Proof: `radio-info-fields.test.ts` holds the shape to the five fields (red against the old `bar.json`, green after); `swift build`. Not proved: on a phone. Non-voice native lines do not fall in this change (3006 to 3006: the fields are still held natively); they fall in the next one.

Replaced: three fields on the wire.

Undo: revert the commit.
