# The deck: the native audio surface under shared code

Station X4 of the Exact cutover. Read this before changing `mobile/AutoradioKit/Sources/AutoradioKit/Deck.swift`,
`mobile/AutoradioLab/deck*.ts`, `mobile/exact-package/interface/deck.json` or the Kotlin deck. Design:
[autoradio-in-exact-2026-11-18.md](../../design/autoradio-in-exact-2026-11-18.md), "The deck applies named rules" and "The journal
is the evidence". Status of what exists (2026-11-21): shared code commands the deck. `mobile/AutoradioLab/deck-native.ts` is the real `DeckPort`: the five names over the app module's one `deck` op (`modules/apple/ModelsEntry.swift` -> `AutoradioModule+Deck.swift` `deckCall`), because only a contract can call a package's names and shared code has to; `radio-runtime.ts` hands it items and reads its journal (the Swift run sheet player is deleted). The Kotlin deck does not exist.

## What it is

Shared code (TypeScript) decides what plays next, what counts as heard and what is posted. The deck plays what it is handed. It
applies a short list of named rules itself, because shared code can be frozen when the deck needs an answer (an iPhone lets shared
code run only while audio is sounding). It keeps one state for why it is silent, and a journal of what started and ended.

## Names (all in `interface/deck.json`; shapes in `mobile/AutoradioLab/deck-types.contract`; types in `deck.ts`)

| Name | Kind | Does |
|---|---|---|
| `deckState` | source, live | `DeckState`: `why`, `item`, `block`, `seconds`, `duration`, `held` (ids), `seq`, `acked`, `journal` (the lines not yet acknowledged) |
| `deckPlay` | send | play this item now; what was playing ends `replaced` |
| `deckEnqueue` | send | hold this item after the playing one at place `at` (0 is next; -1 or absent is the end); the held items from `at` on are dropped first; an id already held is removed first; at most 3 are held |
| `deckAct` | send | `op` is `pause`, `resume`, `stop`, `skip`, `seekTo`, `seekBy` or `trim`; `value` as the op needs |
| `deckAck` | send | the journal lines up to `through` are posted; the deck forgets them |

An item is scalars only (the gate refuses a name that takes an opaque payload): `id`, `kind` (`speech` or `audio`), `title`, `show`,
`document`, `paragraphs` (the text, a blank line between paragraphs), `url`, `start`, `end` (0: to the end), `block`, `seconds`
(where to begin), `requiresPreviousHeard`.

## The rules the deck applies by itself

1. An item with `requiresPreviousHeard` plays only if the item before it ended `finished` with sound rendered; otherwise it leaves the
   held list with no journal line (shared code sees `held` shorten and removes it from its sheet as an orphaned transition).
2. When the audio session cannot be taken, the item stays on air with no outcome (state `held_session`) and is tried again at 1, 2, 4,
   8, 15 s, the last repeating, and at once when the system says the session is back.
3. The listener's pause and a route loss are not undone by a timer. Only the system's "interruption ended" resumes, and only a pause
   the system made.
4. Two items in a row that made no sound stop the deck (state `stopped`).
5. The deck plays on through the held items with shared code not running.

Not the deck's: barge-in and one voice at a time (the engine and `SessionCoordinator`, native to native); what plays next; what is
posted.

## The journal

Ordered lines, durable (a file readable after the first unlock, so it is written with the screen locked), numbered from 1 with the
number stored beside the lines so it never repeats across a relaunch. `type` is `started`, `sounding` (sound reached the output),
`ended`, `remote` (a lock screen or headset command) or `system` (interruption, route loss, session held or back, engine lost). An
`ended` line says how: `finished`, `skipped`, `replaced`, `stopped`, `failed`, with the device's time, the block and seconds
reached, the last block, and `rendered`. `rendered` is true only when sound reached the output; `finished` with `rendered` false is
written `failed`. A launch that finds an item open (the app was killed) closes it first, as `failed` "interrupted by a restart",
before the run sheet is restored. (Today the verdict the run sheet posts is decided by the Swift player's own `played` flag; the journal's
`rendered` is what shared code will decide by once it posts, and the two are checked against each other by the fixtures.) A journal file that
cannot be read (a launch before the first unlock) is never overwritten: lines made meanwhile are held and numbered after it once it reads;
`seq` and `acked` live in a small file of their own so numbers never repeat. A failed save is counted and logged, and the whole journal is
written again at the next event. The journal keeps the newest 2000 lines; past that the oldest go in a tenth at a time and a `system` line
`dropped` says how many. Sign-out wipes the whole deck (`Deck.wipe()`, registered in `AccountWipe` as "deck"): the item on air, the held items, the reason and the journal; an engine start that resolves after the wipe is stopped again. A journal file that reads and cannot be decoded is moved to `deck-journal.bad`, a `system` line `journal-corrupt` says so, and the numbers file carries on; lines at or below the acknowledged number are pruned when the file is read. If the launch found the file unreadable, the item a kill left open is closed (as `failed`, "interrupted by a restart") as soon as it reads.

## What shared code does with it

`deck-ops.ts` builds the `aired` ops from the journal (heard only with sound; the deck's own time in `ended.at`; ids from the line so a
resend counts nothing twice; a start never before the end of the item before it), posts them in batches of at most 64 ops and 256 KB
through `deck-post.ts` (`authedJSON`), and acknowledges only what the server answered. `heldPlan` and `syncHeld` keep the held items in
the sheet's order. The server contract is `internal/runsheet/apply.go` and `internal/agentcore/api_autoradio_settle.go`; the fixtures in
`mobile/fixtures/deck` are applied by the Go reducer (`internal/runsheet/deck_fixtures_test.go`), built from in TypeScript
(`deck-ops.test.ts`) and read by the Swift twin of the ended-line reading (`DeckTests`).

## Android

Kotlin implements the same five names with Media3 (audio) and `TextToSpeech` (speech) behind a media session. Station X6 adds the
media-playback service. Until then it plays only while the app is on screen.

## Not yet proven

Whether a journal line can cause an op to be posted on a locked iPhone with no touch (station X0's test). The design survives either
answer: ops are posted when shared code next runs, with the journal's times. Everything about sound, the lock screen and interruptions
is for ears (see the lane report).
