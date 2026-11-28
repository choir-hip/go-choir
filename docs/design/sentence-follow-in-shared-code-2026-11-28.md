# The sentence follow in shared code: a plan

Standing: undecided | kind: wanted | settled-by: none yet; a plan for review | note: read from code on main (aaad08a1d) and from the parked lane; nothing here was run, and no sentence timing was heard on a device

Station E5 of [finishing the move to Exact](../definitions/choir-finish-the-move-to-exact-2026-11-27.md).
The parked attempt is `origin/lane/bar-facts` and
[its record](../evidence/bar-facts-lane-parked-2026-11-27.md). Owner,
2026-11-28: "get everything that can be exact into exact, so the difference
between iOS and android is minimized".

## What "the follow" is, and who uses it today

Three things read the sentence the voice is saying. All three get it from
Swift, so Android has none of them.

1. The reader lights a sentence on the page in front (`rfollow`, reader.contract:146).
   Swift: `ReaderFeed.feed` (ReaderFollow.swift:70-82) asks `ReaderFollow.set`
   (:26-31, `locate` :34-47), which matches the spoken sentence against
   `HeldPages` (the page shared code handed over).
2. The Autoradio page shows "the sentence now" and a progress line
   (`radioPageFacts`, radio-bridge.ts:617-626, asks `radioFacts peek` for
   `sentences`, `sentenceIndex`, `progress`; radio-model.ts:200-217).
3. The bar's progress (`barFacts.blockIndex`, `blockCount`, `blockFraction`,
   AutoradioModule+Bar.swift:104; bar.ts:71).

Facts that matter for the design, each read from the code:

- Shared code already hands native the entry's words. `DeckItem.paragraphs`
  (interface/deck.json) is what `deckPlay` carries. Native splits it into
  "units" (blocks of at most 280 characters, RadioProgramme.swift:186-192,
  ChoirProgramme.swift:9) and speaks them one at a time through the programme
  player (ChoirPlayback.swift:12-16, `speakSegment` ChoirApp.swift:167-176).
  The entry id is kept as the segment id.
- The only native consumer of `RadioInfo.sentences`, `sentenceIndex` and
  `progress` besides the three above is nothing. The contracts read only
  `title`, `playing`, `level`, `speedValue` and `micLevel` from `radioView`
  (bar.contract:115-121, :215-222; mark.contract:159).
- `ReaderAim` has no native reader at all (grep: only AccountWipe.swift:69 and
  the two ops, AutoradioModule+Reader.swift:95-106). Its comment says "the
  voice turn reads it"; no code does. It is shared-code state held in Swift.

## (a) What native MUST report, and nothing more

One value, read once, in the same answer as `deckState`. It carries its own
identity and its own words. Nothing is asked of native a second time, and
nothing is stamped on arrival by shared code.

```
place = {
  entry:      string   // the deck item id this place is in (native's own: the segment id it was given)
  unit:       number   // index of the speech unit being spoken, from 0
  words:      string   // the exact words of that unit, as native speaks them
  seconds:    number   // media seconds rendered into the unit (not wall time; not scaled by speed)
  total:      number   // media seconds the unit's audio lasts; 0 when not known yet
  placeSeq:   number   // native bumps it each time the unit changes or a seek, skip or restart happens
  at:         number   // device time in ms since 1970 when native read it (the basis `radioFacts.at` already uses)
}
```

Five lines for the report: which entry, which unit and its words, the seconds
into it and its length, a counter that moves when the place jumps, and the
device time. It is added to `DeckState` (interface/deck.json) as scalar
fields, because the gate admits scalars only.

Why `words` is in the report and not left to shared code to look up: the units
are native's own cut of the entry (two splitters exist, one per platform), and
a revision can replace blocks while they play (`ProgrammeDocument.applying`,
Stages.swift:48). If the report names its words, a place and its words cannot
come from two documents. This is the first fix the parked record asked for.

### What exists today, per platform

| Fact | iPhone | Android |
|---|---|---|
| item on air | `Deck.snapshot().item` Deck.swift:585-591 | `DeckController.snapshot()` DeckController.kt:45-49 |
| block | `spokenReport().block`, the SERVER block (ChoirPlayback.swift:19-31) | `DeckPosition.block`, the paragraph of the chunk (DeckMediaEngine.kt:416-423) |
| seconds | the words before this block at the speaking rate PLUS the rendered seconds in it (entry-wide, partly an estimate; ChoirPlayback.swift:26-30) | `exo.currentPosition` of the CURRENT chunk (per chunk; DeckMediaEngine.kt:418) |
| rendered position in the unit | `RenderPosition{index,fraction,seconds,total}` (Playhead.swift:62-70), from `audio.programmePosition(tag:)` (LiveAudio.swift:528-533), tagged by block index | none; `exo.currentPosition` and `exo.duration` (DeckMediaEngine.kt:418, :424) can give it |
| the unit's words | `document.blocks[i].spokenText` (LiveAudio.swift:1098) | `SpeechChunk.text` (DeckMediaEngine.kt:306-314, :440) |
| unit index and count | `blockIndex` of `ProgrammePlayer` (LiveAudio.swift:689) and the document's blocks | `exo.currentMediaItemIndex`; the chunk list grows while synthesis runs (:249-274), so a count is not known early |
| sequence | `DeckSnapshot.seq` is the JOURNAL's number: it does not move when the unit does | same |
| device time | `radioFacts.at` only (AutoradioModule+Bar.swift:119); not in `deckState` | none |

What is missing on Android, all of it: the unit's words and index, the
rendered seconds in the unit and its length as a clean pair, `placeSeq`, `at`,
and every op this follow rests on: `radioView`, `radioSpeed`, `barFacts`,
`radioFacts`, `radioSet`, `readerFollow`, `readerAim`, `readerAimSet` and
`pageText` answer "NotSupportedError" (AutoradioModule.kt:97). Android serves
only `readerOpenExternal`, the model ops and the six deck names (:63-96).

A finding on the way, to be recorded as a problem when this is picked up: the
deck's `seconds` means two different things. On the iPhone it is entry-wide and
part estimate; on Android it is the seconds into the current chunk, which
restarts at each chunk. `ended` journal lines carry it on both
(`Deck.swift:408-434`, DeckController.kt:369-383). Read from code, not run. The
new `place.seconds` is defined once and does not reuse either.

How each platform meets the interface:

- iPhone: add one function on `ProgrammePlayer` that reads document id, block
  index, the segment id of that block, the block's words and the rate under the
  one `st.read` that already guards them (LiveAudio.swift:669-689), then asks
  the audio for the position tagged with that index, then reads the block index
  once more. If it moved, try again, twice at most, then report `total 0`.
  Today `spokenReport` reads `document` and `blockIndex` as two separate reads
  (ChoirPlayback.swift:20) and `renderPosition` as a third: that is the parked
  attempt's first race.
- Android: one function on `DeckMediaEngine`, run on the main looper where
  ExoPlayer and `speechChunks` already live (the synthesis scope is
  `Dispatchers.Main.immediate`, AutoradioModule.kt:56): index, chunk text,
  `currentPosition`, `duration`, in one call. No lock exists to disagree.

## (b) What shared code computes

New file `mobile/AutoradioLab/radio-follow.ts`. Every function is pure: inputs
in, value out, no module state, no cache, no "have". The parked lane's
splitter, weighting and `locate` are the starting point (the lane's
radio-follow.ts, read, not applied), with the report above in place of its
`SpokenSnapshot`.

1. `sentencesOf(words)`: split with the reader's own rule (`sentenceEnds`,
   reader-parse.ts), so the radio's sentence and the page's are cut the same
   way. Swift used ICU `.bySentences` (BarState.swift:152-158); the two rules
   can disagree on "Dr. Smith", "3.5" and a closing quote.
2. `sentenceAt(sentences, fraction)`: by share of characters, fraction capped
   at 0.99 so the last sentence stays lit until the unit changes (BarState.swift
   failure 2). `fraction = total > 0 ? seconds / total : 0`.
3. `owns(place, ds, onAir)`: true only when `place.entry == ds.item == onAir.id`,
   `ds.why == "playing"` and the sheet's on-air entry has an address. Three
   equal ids, two of them native's own.
4. `locate(sentence, blocks)`: the block and sentence of the page that contain
   the sentence (the parked `locate`, 6-character floor, first match).
5. `followOf(ds, onAir, rdoc)`: the reader's `{address, blockId, sentence}` or
   none. Used by reader.contract through a resource
   `rfollow = readerFollow(ds) with rdoc.address, rdoc.version` (a `with`
   source receives both lists in order and a change to either asks again, so a
   new `ds` each radio tick, or an edited page, asks again).
6. `nowOf(ds)`: the radio page's `sentence` line, replacing
   `radioFacts.sentences` and `sentenceIndex`.
7. `progressOf(place, itemWords)`: the entry's progress, computed from the
   words shared code itself handed over (characters before the unit plus the
   weight inside it) over all the characters. It needs no unit count from
   native, which Android cannot give early.

The reader's aim is separate and needs no native report. It is a place in
shared state (see slice S1).

Not cached: nothing holds the last sentence, the last place or a page. Each
answer is a function of the arguments of that ask. This is the rule that
removes the protocol the parked lane needed (`have`).

## (c) The two old races, and their fate

Race 1, "two locks". Native read the document under the player's lock and
the position under another, and a position had a block index and no document.
Fate: gone on the iPhone by the one read (a), and its remaining window is
named: the audio's own lock is taken after the player's. The block index is
re-read afterwards and the read retried twice; if still moving, `total` is 0 and
shared code lights the unit's first sentence. The words and the unit stay from
one read, so a wrong document's words cannot appear. The remaining effect is a
sentence one or two early for at most a tick. Android has no second lock.

Race 2, "stamped on arrival". Shared code gave the answer the id of whatever
entry was on air when the answer came back, so a read begun for entry A and
finished after entry B (a sibling of the same story) took B's id. Fate: gone.
Shared code stamps nothing. The entry id is native's, from the segment it
is speaking; the check is equality of three ids (`place.entry`, `ds.item`, the
sheet's on-air id). A late answer for A against B on air fails the equality
and lights nothing. Entry ids are minted unique (`mintId`), so two entries of
the same story differ.

Remaining, said plainly:

- One tick (about one second) of nothing lit at every change of entry. That is
  the safe direction.
- `place` and `ds.item` are two reads in one data answer, not one lock. Their
  disagreement is caught by the equality, not prevented.
- The sentence chosen is an estimate from audio share by characters. That was
  true of the Swift too. It can be a sentence early or late, and cannot be
  proved without ears.
- `locate` takes the first block that contains the sentence, so a sentence that
  appears in two blocks lights the first. No memory means no tie-break. Listed
  as F10 below, with a test that pins the limit.

## (d) The failure list

Tests are `bun test` in `AutoradioLab` unless a Swift or Kotlin test is named.
Write them first, see them fail, then the code.

| # | Way it can be wrong | Test |
|---|---|---|
| F1 | A seek back inside the unit: the sentence goes back. That is correct (the report is truth); no hysteresis must hide it. | `sentenceAt` at 0.9 then 0.1 gives last then first; no module state read |
| F2 | A speed change: the fraction is media time over media time, so the speed does not matter; a shown lag of up to one tick. | same report at two speeds gives the same sentence; `total 0` gives the first sentence and no throw |
| F3 | Skip (Next): the old place arrives with the new `ds.item`. | `owns` is false when `place.entry != ds.item`; nothing lit |
| F4 | The entry changes mid-sentence to ANOTHER entry of the same story (race 2). | `place.entry = A`, `ds.item = A`, `onAir.id = B` gives none; and `ds.item = B` with place A gives none |
| F5 | Pause: the follow must not stay lit if it was cleared before. Swift cleared when not playing (BarState.swift:186 `playing`). Parity kept. | `ds.why = paused_listener` gives none; `held_session` gives none |
| F6 | Background and return with shared code asleep: nothing is replayed, no events missed, because the state is a sample. A frozen answer may show once on return. | the same arguments give the same value twice (purity); an old `ds` against a changed `onAir` gives none; the resource's key includes `posKey` so the first visible beat asks (checked in spine.contract by a test that reads the contract text, as radio-position.test.ts P7 does) |
| F7 | A report for an item no longer playing, or `ds.item == ""` (stopped, wiped). | `owns` false; `place.entry == ""` false |
| F8 | The page is edited, restored or reloaded while playing. | `followOf` on the new blocks: sentence moved gives the new index; sentence deleted gives none, not the old block id; the resource re-asks on `rdoc.version` (contract text test) |
| F9 | The unit's words are replaced by a revision while it plays. | Swift test in ProgrammePlayerTests: the one read returns the new words with the new index or the old with the old, never a mix (Swift, through the lock, not bun) |
| F10 | The sentence appears in two blocks. | pins first-match; documents it |
| F11 | The two splitters disagree. | the Swift tests' cases for `sentences` ported, plus "Dr. Smith went home.", "It cost 3.5 dollars.", a closing quote, an ellipsis, no stop at all, only spaces |
| F12 | The audio runs longer than the estimate: the last sentence must stay lit, not vanish. | fraction 1, 2, NaN and negative are clamped; last sentence at 0.99 |
| F13 | A unit that begins in the middle of a sentence (the 280-character cut). | `locate` of the second unit's first sentence finds the page sentence that holds its head |
| F14 | Words under 6 characters, empty words, a non-story entry (opening, answer, take: `airAddressOf` is empty). | none, for each |
| F15 | A report that arrives late, out of order. Exact lets go the older answer of the same resource (answers-let-go.md), so one answer is for one ask. `placeSeq` and `at` are carried for the log and for the failure record; shared code does not order by them, because that would need memory and an invalidation. | a test that the module exports no mutable state (it imports nothing that holds any) |
| F16 | Sign-out: nothing is held, so nothing to clear. `rfollow` is a function of `ds`, and `ds` is wiped by `deckWipe`. | after `deckWipe` state (`item ""`) the follow is none |
| F17 | Android: the unit's audio is not made yet, so `total 0`. | covered by F2; Kotlin test in `DeckControllerTest` if one exists for the chunk list (not found in this lane; to be built) |
| F18 | The retry in the one read (iPhone) gives up. | Swift test: a position tagged for another block gives `total 0` and the SAME words and unit |

## (e) The slices

Each can land alone. The rule "no switches" applies: a slice never leaves two
paths. S1 is not part of the follow itself; it is the smallest piece that
shares its files and removes Swift.

**S1. The reader's aim moves to shared code.** The aim is a document or a
block the bar's prompt is aimed at. No native code reads it. Put it in shared
state keyed on the account generation (the pattern of `dismissAnswer`, bar.ts,
and `answerDismissShared`, app.ts:230), serve `readerAim` and `readerAimSet`
from app.ts, `refreshes raim` on the mutation.
Swift removed: `ReaderAimInfo` and `ReaderAim` (ReaderFollow.swift:57-72, 17
lines), the two ops and `onChange` (AutoradioModule+Reader.swift:95, 98, 100-107,
about 10 lines), the wipe's call (AccountWipe.swift:69, part of one line);
interface/reader.json and exact.json entries. Ops made unnecessary: `readerAim`,
`readerAimSet`. Proof is in `bun test` (the memory, the sign-out) plus the
contract build and `exact test web` for the wiring.

**S2. The place report, and the radio page's sentence.** Add `place` to
`DeckState` on both platforms (the one read, (a)); add `radio-follow.ts`; the
radio page's `sentence` and `progress` come from it; delete `sentences`,
`sentenceIndex`, `progress` from `radioFacts` (AutoradioModule+Bar.swift:121) and
the `radioFacts peek` call in `radioPageFacts` (radio-bridge.ts:618-619).
New: about 30 Swift lines, about 25 Kotlin lines, about 120 TypeScript.
Removed: about 3 Swift lines. Honest: S2 adds native lines. The Swift it makes
dead is deleted in S3. Android gets the Autoradio page's sentence line for the
first time.

**S3. The reader's follow.** `rfollow` becomes `followOf(ds, onAir, rdoc)`.
Swift removed: the rest of ReaderFollow.swift (`ReaderFollowInfo`,
`ReaderFollow`, `ReaderFeed`, lines 1-55 and 70-83, 66 lines, the file goes
whole), `BarTracker.sentences` and `sentenceIndex` (BarState.swift:152-169, 18
lines), the sentence part of `radioInfo` (:183-190, 8 lines),
`RadioInfo.sentenceIndex`, `sentences` and `progress` (3 lines) and the
`ReaderFeed.feed` call (AutoradioModule+Bar.swift:67), the ops `readerFollow`
(+Reader.swift:94, 97) and the `sentences` field of `HeldBlock` and the
`sentences` the reader sends with `pageText` (HeldPages.swift:11-12, :34;
reader.ts:144). Around 100 lines. Ops made unnecessary: `readerFollow`. After
S1 and S3, `ReaderFollow.swift` is gone (83 lines). `HeldPages` stays, smaller,
only for the voice turn's page text (`pageTextProvider`, AutoradioModule+Spine.swift:44)
until the voice owns its page text.

**S4. The bar's facts.** `barFacts` stops carrying `blockIndex`, `blockCount`,
`blockFraction`; `bar.ts` takes the progress from `progressOf`. Swift removed:
`BlockClock` (BarState.swift:31-43), `positionSource` (:64) and its wiring
(LabRuntime.swift:136), `blockFraction` and `clockProgress` (:133-150), the
words-per-second estimate, about 45 lines. `BarState.swift` goes from 192 to
about 120: what stays is the engine's events, the microphone level and the
speed, which are voice. Ops made unnecessary: none whole; `barFacts` is
smaller. Android needs `barFacts` still for phase and transcript (voice facts);
that is a different slice and not this plan.

**Not in this chain:** `SourceLink.swift` (23 lines) is the web page view's
URL router; it is used by the native `web-page` view and the Android copy is
in AutoradioModule.kt:40-58. It is not blocked by the follow.
`radioView`'s `level`, `micLevel`, `speedValue` and `radioSpeed` are voice and
speed; they need their own small voice-facts op that Android implements, and
that is the remaining way `radioView` and `radioSpeed` stop being "not
supported" on Android.

## (f) What cannot be verified without a device

- That the characters-by-share estimate lights the right sentence for a real
  voice on each platform. It was never measured on either (the Swift had the
  same estimate).
- Android: that `exo.duration` is known for the chunk on air, and that
  `currentMediaItemIndex` changes when the sound does and not before. Also
  what `total 0` looks like while the first chunks are made.
- The lag of one tick (one second) at the phone's real read rate; whether that
  reads as late on a short sentence.
- That the read in `deckState` at one second each costs nothing the phone
  would notice (it adds a text copy of at most 280 characters).
- A locked or backgrounded iPhone: whether `deckState` is asked at all, and
  what the first visible beat shows (F6).
- Whether the retry in the iPhone read ever gives up in real use.
- That the contract's `with` re-asks on a page edit as the grammar says. The
  text of the grammar is read (contract-grammar.md:657-660); the behaviour is
  for `exact test web` and then a phone.

## What I would do first

S1, then S2 and S3 together if two seats agree on the report above. S1 is
small, removes Swift and needs no new native report; its tests run in `bun
test`. S2 and S3 add native code on two platforms that cannot be compiled or
run from here, so they wait for the plan to be reviewed. The report's shape is
the part to review before any of it is built.
