# The sentence follow in shared code: a plan

Standing: built | kind: happened | scope: in part | settled-by: S1, S0 and S2+S3 are built (docs/changes/2026-11-28-readers-aim-in-shared-code.md, docs/changes/2026-11-28-sentence-follow-in-shared-code.md); S4 is not | note: built and tested (bun, Swift, Kotlin JVM, the contract build); no sentence timing was heard on a device, and the iPhone compile of the touched app-side Swift and the Android build are the landing's

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
identity and says WHERE in the entry's text native is, in the text shared code
handed over, not in native's own cut. Nothing is asked of native a second time,
and nothing is stamped on arrival by shared code.

```
place = {
  entry:    string   // the deck item id native is speaking (the segment id it was given)
  digest:   string   // length and a 32-bit FNV-1a of the entry's `paragraphs` string AS NATIVE RECEIVED IT, over UTF-16 code units, e.g. "5123:9f3a07c1"; made once when the item starts
  start:    number   // UTF-16 offset in that string where the unit being spoken begins
  length:   number   // its length in UTF-16 code units
  seconds:  number   // media seconds rendered into the unit (not wall time; not scaled by speed)
  total:    number   // media seconds the unit's audio lasts; 0 when not known yet
}
```

Shared code makes the entry's digest from the string it handed to `deckPlay`
and lights a sentence only when `place.digest` equals it. It then takes
`text.slice(start, start + length)` from ITS OWN string. It never searches
for native's unit in its text, never sees native's words, and does not care
how either platform cuts units. `placeSeq` and `at` are dropped: nothing reads
them (F15 explains why ordering by them was rejected anyway).

Which side should own the units? Shared code owns the text (it writes
`DeckItem.paragraphs`). Native owns the cut because the cut is the voice
engine's own bound: iPhone `ChoirText.speechChars = 280` after flattening
newlines (ChoirProgramme.swift:9, RadioProgramme.swift:186-192), Android
`splitSpeech` parts "within the engine's one-utterance bound"
(DeckMediaEngine.kt:306-314). If shared code did the splitting and handed
units, it would have to know each engine's bound and the deck item would
change shape on both platforms (interface/deck.json `paragraphs`), and the
spoken chunking, which the voice owns, would move out of native. The range
report keeps the voice's cut where it is and makes the two splitters
irrelevant to shared code: only a range into a string both sides hold, checked
by a digest.

Each native must therefore carry, for every unit it makes, its range in the
received string. That is new bookkeeping on both: iPhone `SegmentPrep.prepare`
trims, flattens newlines and splits, so it must track source offsets as it
goes (RadioProgramme.swift:182-195; `ProgrammeBlock` gains `start`, `length`);
Android `splitSpeech` returns ranges as well as strings (:306-314;
`SpeechChunk` gains them). A unit's range is a fact native made when it cut,
not something recomputed at read time.

### What exists today, per platform

| Fact | iPhone | Android |
|---|---|---|
| item on air | `Deck.snapshot().item` Deck.swift:585-591 | `DeckController.snapshot()` DeckController.kt:45-49 |
| block | `spokenReport().block`, the SERVER block (ChoirPlayback.swift:19-31) | `DeckPosition.block`, the paragraph of the chunk (DeckMediaEngine.kt:416-423) |
| seconds | the words before this block at the speaking rate PLUS the rendered seconds in it (entry-wide, partly an estimate; ChoirPlayback.swift:26-30) | `exo.currentPosition` of the CURRENT chunk (per chunk; DeckMediaEngine.kt:418) |
| rendered position in the unit | `RenderPosition{index,fraction,seconds,total}` (Playhead.swift:62-70), from `audio.programmePosition(tag:)` (LiveAudio.swift:528-533), tagged by block index | none; `exo.currentPosition` and `exo.duration` (DeckMediaEngine.kt:418, :424) can give it |
| the unit's range in the received text | none; `ProgrammeBlock` has no source offset (RadioProgramme.swift:190) | none; `SpeechChunk` has `block` and `text` (DeckMediaEngine.kt:440) |
| unit index and count | `blockIndex` of `ProgrammePlayer` (LiveAudio.swift:689) and the document's blocks | `exo.currentMediaItemIndex`; the chunk list grows while synthesis runs (:249-274), so a count is not known early |
| sequence | `DeckSnapshot.seq` is the JOURNAL's number: it does not move when the unit does | same |
| device time | `radioFacts.at` only (AutoradioModule+Bar.swift:119); not in `deckState` | none |

What is missing on Android, all of it: the unit's range and the digest, the
rendered seconds in the unit and its length as a clean pair,
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

- iPhone: add one function on `ProgrammePlayer` that reads, under the one
  `st.read` that already guards them (LiveAudio.swift:669-689), the segment id,
  the digest, the unit's range, the block index and a DOCUMENT GENERATION (a
  counter in `State`, bumped by `setDocument` and `applying`, LiveAudio.swift:750
  and :1115). The playhead records the generation it was scheduled under, and
  `programmePosition` returns it with the position (LiveAudio.swift:528-533,
  Playhead.swift:62-70). The place is dropped (`entry ""`) when the playhead's
  generation or tag differs from the one read, and the read is retried twice
  before dropping. A tag of the block index alone is not enough: a revision
  that replaces the document at the SAME index passes it (review finding 2).
  Today `spokenReport` reads `document` and `blockIndex` as two reads
  (ChoirPlayback.swift:20) and `renderPosition` as a third: the parked attempt's
  first race.
- Android: one function on `DeckMediaEngine`, run on the main looper where
  ExoPlayer and `speechChunks` live (AutoradioModule.kt:56): the chunk index
  (bounded, `coerceIn`, and an empty list gives no place), the chunk's range,
  `currentPosition`, and `duration` with `C.TIME_UNSET` (a large negative
  number) mapped to 0 (the guard at DeckMediaEngine.kt:424 is the model).
  The chunks and the player are on one thread and there is no document that
  can be replaced under it, so no generation is needed; the digest is made once
  when the item starts.

## (b) What shared code computes

New file `mobile/AutoradioLab/radio-follow.ts`. Every function is pure: inputs
in, value out, no module state, no cache, no "have". The parked lane's
splitter, weighting and `locate` are the starting point (the lane's
radio-follow.ts, read, not applied), with the report above in place of its
`SpokenSnapshot`.

The rule for every function and resource below: **every value the pure
function reads is in its input list.** No function reads a module variable, the
clock or the sheet from inside. Where a value comes from shared state (the
sheet, the text handed to the deck), the resource passes it as an argument.

1. `sentencesOf(text)`: split with the reader's own rule (`sentenceEnds`,
   reader-parse.ts), so the radio's sentence and the page's are cut the same
   way. Swift used ICU `.bySentences` (BarState.swift:152-158); the two rules
   can disagree on "Dr. Smith", "3.5" and a closing quote.
2. `sentenceAt(sentences, fraction)`: by share of characters, fraction capped
   at 0.99 so the last sentence stays lit until the unit changes (BarState.swift
   failure 2). `fraction = total > 0 ? seconds / total : 0`.
3. `unitOf(place, handed)`: `handed = { id, text }` is what shared code passed
   to `deckPlay` for the entry on air (the runtime records it at that call;
   it is replaced at each call and its digest is checked, so it needs no
   invalidation). Returns `text.slice(start, start + length)` only when
   `place.entry == handed.id`, `place.digest == digestOf(handed.text)`, and
   `0 <= start`, `length > 0`, `start + length <= text.length`. Anything else is
   none. This is where duplicates and live revisions are caught: the digest
   differs when the text does, so a unit is never found by searching.
4. `owns(ds, place, onAir)`: true only when `place.entry == ds.item == onAir.id`
   and `ds.why == "playing"` and the on-air entry has an address.
5. `locate(sentence, blocks)`: the block and sentence of the page that contain
   the sentence (the parked `locate`, 6-character floor, first match).
6. `followOf(ds, onAir, handed, rdoc)` gives the reader's `{address, blockId,
   sentence}` or none. Its resource lists all of them:
   `rfollow = readerFollow(ds, onAir.id, onAir.address, handed.id, digestOf(handed.text)) with rdoc.address, rdoc.version`.
   Exact asks again when any argument or any `with` value changes (the source
   receives both lists in order; a change to either asks again,
   contract-grammar.md:657-660). So an on-air change that arrives before
   `ds` ticks changes `onAir.id` and asks again at once, and the answer is
   none because `ds.item` (old) is not `onAir.id` (new). That was review
   finding 3.
7. `nowOf(ds, onAir, handed)`: the radio page's `sentence` line, replacing
   `radioFacts.sentences` and `sentenceIndex`; asked with the same inputs.
8. `progressOf(place, handed)`: the entry's progress is
   `(start + fraction * length) / text.length`, from the range and shared
   code's own text. No unit count and no search for "characters before the
   unit" is needed (review finding 1). It is also the answer to Android not
   knowing its chunk count early.

Every resource in this plan against the rule: `rfollow` (6): inputs listed.
Radio page sentence (7): `radioPageFacts` already passes the sheet through
the radio runtime; the plan adds `ds` and `handed` to its asks, and its key
already carries `posKey`. Bar progress (S4): `barInfo(bf, brun, ...)` gains
`ds` and `handed`. `raim` (S1): a function of the mutation's arguments and the
account generation (bar.ts `closedIds` precedent): the generation is an input
(`signedOutTick`), as in `barInfo`'s key.

The reader's aim is separate and needs no native report. It is shared state
(slice S1).

Not cached: nothing holds the last sentence, the last place or a page. Each
answer is a function of the arguments of that ask. This is the rule that
removes the protocol the parked lane needed (`have`).

## (c) The two old races, and their fate

Race 1, "two locks". Native read the document under the player's lock and
the position under another, and a position had a block index and no document.
Fate: gone on the iPhone by the one read (a). The remaining window, between
the player's lock and the audio's, is closed by the document generation: the
playhead carries the generation it was scheduled under, and a place whose
playhead generation or block tag differs from the one read is dropped, after two
retries. A revision that swaps the document at the same block index (review
finding 2) changes the generation and is dropped; a block-index tag alone would
have passed it. The cost is nothing lit for a tick, never old range with new
timing. Android has no second lock and no replaceable document.

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
| F9 | A revision replaces the document while a unit plays, at the same index or another. | Swift test in ProgrammePlayerTests: the one read returns a place whose range and generation agree, or none; a swap between the player read and the position read gives none (Swift, through the lock, not bun) |
| F10 | The sentence appears in two blocks. | pins first-match; documents it |
| F11 | The two splitters disagree. | the Swift tests' cases for `sentences` ported, plus "Dr. Smith went home.", "It cost 3.5 dollars.", a closing quote, an ellipsis, no stop at all, only spaces |
| F12 | The audio runs longer than the estimate: the last sentence must stay lit, not vanish. | fraction 1, 2, NaN and negative are clamped; last sentence at 0.99 |
| F13 | A unit that begins in the middle of a sentence (the 280-character cut). | `locate` of the second unit's first sentence finds the page sentence that holds its head |
| F14 | Words under 6 characters, empty words, a non-story entry (opening, answer, take: `airAddressOf` is empty). | none, for each |
| F15 | A report that arrives late, out of order. Exact lets go the older answer of the same resource (answers-let-go.md), so one answer is for one ask. The report has no sequence number or time: ordering by them would need memory and an invalidation, and nothing would read them. | a test that the module exports no mutable state (it imports nothing that holds any) |
| F16 | Sign-out: nothing is held, so nothing to clear. `rfollow` is a function of `ds`, and `ds` is wiped by `deckWipe`. | after `deckWipe` state (`item ""`) the follow is none |
| F17 | Android: the unit's audio is not made yet, so `total 0`. | covered by F2; Kotlin test in `DeckControllerTest` if one exists for the chunk list (not found in this lane; to be built) |
| F19 | The entry's text holds the same sentence twice, or was revised after `deckPlay` (the digest differs). | `unitOf` by range: a duplicate sentence is lit at its range, not its first match in the text; a changed text gives none; `start+length` past the end gives none |
| F20 | The same-index document swap on iPhone (review finding 2). | Swift test, F9's second half |
| F21 | The on-air entry changes before `ds` ticks (finding 3). | `followOf` with new `onAir.id` and old `ds.item` gives none; contract text test that the resource's argument list holds `onAir.id`, `onAir.address`, the handed id and digest, and its `with` holds the page's address and version |
| F22 | Android `exo.duration` is `C.TIME_UNSET` (negative), or `speechChunks` is empty or the index is past its end. | Kotlin unit test of the place function: negative duration gives `total 0`, an empty list gives `entry ""`, an index past the end is bounded; shared side: negative `total` is treated as 0 |
| F23 | Asleep then awake with the on-air entry unchanged: shared code is frozen, the last answer stays until the next ask. Nothing is shown while hidden. Bound: `posKey` moves the beat the moment the page is visible and the app in front, so the first ask is on that beat and the stale sentence can show for at most one answer time. It cannot show another entry's, because every id is checked on each ask. | the contract text test for `posKey` in the resource's inputs (radio-position.test.ts P7's pattern); the on-device part is in (f) |
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

**S0. Before S2: shrink the boundary.** `RadioInfo` in interface/bar.json (and
exact.json) still lists `progress`, `sentenceIndex` and `sentences`; no contract
reads them (`ri.` is read only for `title`, `level`, `playing`, `speedValue`,
`micLevel`: bar.contract:115-121, :215-222, mark.contract:159, members-bar.contract:38-57).
Three unread fields after f0509f6a7. Drop them from the shape and from the
Swift struct's encoding; `radioFacts` keeps its own copies until S2+S3. Small,
removes Swift (about 4 lines), no new behaviour. Whether Exact refuses extra
keys in an answer is to be read from the pin before this is built.

**S2+S3, one change: the report, and both consumers switched.** There is no S2
that leaves the old and new computations of the same sentence side by side
(CLAUDE.md, no switches). The change adds `place` to `DeckState` on both
platforms and `radio-follow.ts`; the reader's `rfollow` and the radio page's
sentence and progress read from it; and in the same change the old path is
deleted: `ReaderFeed`, `ReaderFollow`, the `sentences` fields of `radioFacts`
and the `radioFacts peek` call (radio-bridge.ts:618-619). The native lines it
adds (about 40 Swift, 30 Kotlin, 120 TypeScript) and the lines it removes are
in one review, so the weight of the change is clear. If one seat finds that too
large, the fallback is: first change adds the report and its tests and NO
consumer (dead but not duplicated), second switches both consumers at once.
The report-only first change duplicates nothing, because nothing yet reads
the new fields.

**The deletions of S2+S3.** `rfollow` becomes `followOf(ds, onAir, handed, rdoc)`.
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

## What was built (2026-11-28)

S1, S0 and S2+S3 are built; S4 (the bar's block facts) is not. Differences from the plan above, each a detail and each decided while building:

- S0 took the three fields off the wire and out of the Swift struct's encoding (`CodingKeys`) and left the values held natively until S2+S3 deleted them: `radioFacts` and `ReaderFeed` still read them until then.
- The page's blocks are arguments, not a `with` on a version. `ReaderDoc` has no `version`; `rfollow = readerFollowShared(ds, rair, rdoc.address, <block ids>, <block texts>)` passes what `followOf` reads, so no value is read from outside the input list. `rair` is a new resource, `radioAir(rpulse.tick, posKey)`: the on-air entry's id and page address, and the id and text last handed to the deck (`RadioRuntime.handed`).
- The radio page shows its sentence and progress while the deck plays or the listener has paused (the place stands); the reader lights a sentence only while it plays (parity with Swift). The station's own words (no page address) show on the page and light nothing in the reader.
- iPhone ranges: `SegmentPrep.prepare` cuts as before and the ranges are found afterwards by `DeckDigest.ranges`, matching each unit to the received string white space to white space, made once when the segment is prepared and stored on the segment. A unit that cannot be found gives no ranges and so no place. Tracking offsets through the trim, newline flattening and the ICU sentence cut was the alternative; matching afterwards touches none of those. Android makes its ranges as it cuts (`SpeechRanges`).
- The page position is by occurrence, not search: the k-th copy in the entry of the sentence at the reported offset is the k-th copy on the page, when both have the same number of copies (`locateAt`); else nothing. An earlier build searched by text and lit the earlier of a repeated sentence; a second tried a longest-common-run alignment and could jump back to an earlier copy; review caught both.
- The reader's page numbers sentences with `sentenceEnds` over each block's text, as `reader-parse.ts` does; a block over the reader's size is one sentence.
- `radioFacts` lost `sentences`, `sentenceIndex`, `progress` and its `peek` argument (the page facts no longer ask native for anything); `HeldBlock.sentences` and the `sentences` of the `pageText` hand-over are gone.

Not done and listed for a device: see (f), and for Android that `exo.duration` is known for the chunk on air and that `currentMediaItemIndex` moves with the sound.

## What I would do first

S1 (landed or landing alone), then S0, then S2+S3 as one change once two seats
agree on the report. The report's shape is the part to review before any of it
is built.

## Review of 2026-11-28 and what changed

One seat reviewed this plan. Verdict: build S1 after changes; fix three
blocking defects in the plan before S2. Each is fixed above.

1. `progressOf` needed "characters before the unit", which neither the unit
   index nor the words give (the two platforms cut differently; duplicates and
   revisions break a search). Now: native reports the unit's UTF-16 RANGE in
   the entry's text as it received it, with a length-and-FNV digest of that
   whole text; shared code lights only when its own handed text has the same
   digest and slices its own text. Native keeps the cut (the voice's bound);
   shared code never searches. See (a) and (b) 3 and 8.
2. The same-index document swap passed the retry, which re-checked only the
   block index. Now: a document generation in the player's state and on the
   playhead; a place whose generation differs is dropped. See (a) and (c).
3. `readerFollow(ds) with rdoc...` omitted the on-air entry. Now the rule "every
   value the pure function reads is in its input list" is stated in (b), the
   resource lists the on-air id and address and the handed id and digest, and
   each resource of the plan is checked against it.

Also changed: `placeSeq` and `at` are gone from the report (nothing read them);
failure items F19 to F23 are added (duplicates and revisions, the same-index
swap, on-air changing before `ds` ticks, `C.TIME_UNSET` and chunk bounds,
asleep-then-awake with the same entry on air and what bounds it); S2 and S3 are
one change so no two computations of the sentence run side by side; S0 shrinks
the `RadioInfo` boundary first; and the deck's two meanings of `seconds` have
their own record:
[deck-seconds-mean-different-things-on-iphone-and-android-2026-11-28](../problems/deck-seconds-mean-different-things-on-iphone-and-android-2026-11-28.md).
