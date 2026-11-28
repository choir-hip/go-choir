# The deck's `seconds` means different things on the iPhone and on Android

Standing: governing | kind: happened | settled-by: a reading of origin/main aaad08a1d on 2026-11-28 | note: read from code, not run; no device was asked

Problem. `DeckState.seconds` and the `seconds` of every journal line are one
field in one shape (interface/deck.json) and shared code reads them as one
thing. The two native decks fill it with different quantities.

Evidence.

- iPhone: `Deck.position()` for a spoken item returns `spokenReport()`
  (Deck.swift:577-581), which is the seconds into the whole ENTRY: the words of
  the blocks before the current one at the speaking rate (an estimate, 2.6 words
  a second times the rate) plus the rendered seconds in the current block
  (ChoirPlayback.swift:19-31). The snapshot (Deck.swift:585-591) and the
  `ended` lines (Deck.swift:408, :434) carry it.
- Android: `DeckMediaEngine.position()` returns `exo.currentPosition / 1000`
  for a spoken item (DeckMediaEngine.kt:416-423). ExoPlayer holds one media item
  per speech chunk, so this is the seconds into the CURRENT CHUNK and it starts
  again at each chunk. `DeckController.snapshot()` (DeckController.kt:45-49)
  and the journal lines (DeckController.kt:241, :369, :383) carry it.
- For remote audio both give the seconds into the episode, so the field agrees
  there.

Belief. A spoken entry's `seconds` is monotone and entry-wide on the iPhone,
and falls back to near zero at every chunk on Android. Anything that compares
`seconds` across the two, or reads a spoken entry's `seconds` as progress,
(the `ended` line's "seconds reached", a resume, a position label) is right on
one and wrong on the other. The block field has the same split: the iPhone's
`block` is the server block, Android's is the chunk's paragraph.

Remaining error. Which consumers in shared code depend on it is not traced
here. `radio-runtime.ts` reads `block` and `seconds` from journal lines; whether
it resumes or posts from them for spoken entries was not read. The sentence
follow plan (docs/design/sentence-follow-in-shared-code-2026-11-28.md) does not
use these fields; it defines its own `seconds` inside a unit.

Next. Name the one meaning in deck.md and interface/deck.json, then make both
decks meet it, or give the spoken entry's place its own fields as the follow
plan does.
