# The deck: the native audio surface under shared code

Station X4 of the Exact cutover. Read this before changing `mobile/AutoradioKit/Sources/AutoradioKit/Deck.swift`,
`mobile/AutoradioLab/deck*.ts`, `mobile/exact-package/interface/deck.json` or the Kotlin deck. Design:
[autoradio-in-exact-2026-11-18.md](../../design/autoradio-in-exact-2026-11-18.md), "The deck applies named rules" and "The journal
is the evidence". Status of what exists (2026-11-21): shared code commands the deck. `mobile/AutoradioLab/deck-native.ts` is the real `DeckPort`: the five names over the app module's one `deck` op (`modules/apple/ModelsEntry.swift` -> `AutoradioModule+Deck.swift` `deckCall`), because only a contract can call a package's names and shared code has to; `radio-runtime.ts` hands it items and reads its journal (the Swift run sheet player is deleted). The Kotlin deck does not exist.

## What it is

Shared code (TypeScript) decides what plays next, what counts as heard and what is posted. The deck plays what it is handed. It
applies a short list of named rules itself, because shared code can be frozen when the deck needs an answer. It keeps one state for why it
is silent, and a journal of what started and ended.

### Does shared code run in the background while the deck plays? What the source says (read 2026-11-21, not run on a phone)

Exact's `page.frozen` is `AppBackground.now && !backgroundExecutionEligible()` on iOS (`host/apple/Sources/ExactKit/PageFacts.swift:318-323` in the
Exact checkout `mobile/.exact-src`, commit f53347503). `backgroundExecutionEligible()` (`host/apple/Sources/ExactKit/AudioSession.swift:148-157`) is true
when the app declares the `audio` background mode and the audio session is active in a playback or playAndRecord category and one of these holds: a
call, an Exact audio or video element that is running, a background capture claim, or **a `speech-playback` claim**. The app module holds exactly
such a claim while the radio plays (`mobile/exact-package/apple/AutoradioModule.swift:37`, mode `speech-playback`, `background: true`; the claim
is documented at `host/apple/modules/ExactPackage.swift:1077-1083`). So the condition is **a held, active playback claim, not sound being
produced at that moment**: shared code is not `frozen` while the claim is held, even in the gaps between entries. It stops being true when the claim
is released, when the session cannot be taken (`held_session`: a phone call), or when the claim is for a state that is not `.playing` or
`.listening` (see `AutoradioModule.swift:20-26`). Exact's own contract adds that "False does not promise continued execution"
(`docs/contract-for-agents.md:5509-5510`): `frozen: false` is not a promise that iOS will not suspend the process. Whether shared code keeps
running on a locked phone through a whole programme is therefore **unproven**; the owner's locked phone is the only proof.

The deck is built so that it does not depend on it: it holds every ready entry it is given (up to 100, the sheet's own bound) and plays through
them with shared code not running; the lock screen's next, previous, pause and play act on the deck at once (`Deck.remoteNext`, `remotePrevious`,
`listenerPaused`, `resumed`); each is journalled first, with where the item was, and shared code makes the sheet agree when it next runs. What the deck
cannot do alone: an entry whose sound is not in the entry (a paper story, a page, a book, a take or a file, which shared code has to fetch), the
entry before this one (the sheet's heard list), and anything that needs the desk. When the held list runs dry in the background the deck writes a
`system` line `held-dry`, so the gap is on the record.

## Names (all in `interface/deck.json`; shapes in `mobile/AutoradioLab/deck-types.contract`; types in `deck.ts`)

| Name | Kind | Does |
|---|---|---|
| `deckState` | source, live | `DeckState`: `why`, `item`, `block`, `seconds`, `duration`, `held` (ids), `seq`, `acked`, `journal` (the lines not yet acknowledged) |
| `deckPlay` | send | play this item now; what was playing ends `replaced` |
| `deckEnqueue` | send | hold this item after the playing one at place `at` (0 is next; -1 or absent is the end); the held items from `at` on are dropped first; an id already held is removed first; at most 100 are held (the sheet's bound) |
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
5. The deck plays on through the held items with shared code not running. A lock-screen next skips and plays on at once; a previous on audio, or on a spoken
   entry that has gone on (past its first block, or 3 s), starts it again at once; a previous on an entry just begun is left to shared code. Each is journalled first.

Not the deck's: barge-in and one voice at a time (the engine and `SessionCoordinator`, native to native); what plays next; what is
posted.

## The journal

Ordered lines, durable (a file readable after the first unlock, so it is written with the screen locked), numbered from 1 with the
number stored beside the lines so it never repeats across a relaunch. `type` is `started`, `sounding` (sound reached the output),
`ended`, `remote` (a lock screen or headset command) or `system` (interruption, route loss, session held or back, engine lost). An
`ended` line says how: `finished`, `skipped`, `replaced`, `stopped`, `failed`, with the device's time, the block and seconds
reached, the last block, and `rendered`. `rendered` is true only when sound reached the output; `finished` with `rendered` false is
written `failed`. A launch that finds an item open (the app was killed) closes it first, as `failed` "interrupted by a restart",
before the run sheet is restored. Shared code decides the verdict from the line: `radio-runtime.ts` `read` takes a `finished` line with `rendered` as heard and
posts it with the line's own `at`; at launch `recover` reads the lines the process outlived before anything closes the entry on air as interrupted. A journal file that
cannot be read (a launch before the first unlock) is never overwritten: lines made meanwhile are held and numbered after it once it reads;
`seq` and `acked` live in a small file of their own so numbers never repeat. A failed save is counted and logged, and the whole journal is
written again at the next event. The journal keeps the newest 2000 lines; past that the oldest go in a tenth at a time and a `system` line
`dropped` says how many. Sign-out wipes the whole deck (`Deck.wipe()`, registered in `AccountWipe` as "deck"): the item on air, the held items, the reason and the journal; an engine start that resolves after the wipe is stopped again. A journal file that reads and cannot be decoded is moved to `deck-journal.bad`, a `system` line `journal-corrupt` says so, and the numbers file carries on; lines at or below the acknowledged number are pruned when the file is read. If the launch found the file unreadable, the item a kill left open is closed (as `failed`, "interrupted by a restart") as soon as it reads.

## What shared code does with it

The run sheet store (`runsheet-store.ts`) posts the `aired` ops, made by the director from the journal lines (`radio-runtime.ts`); `deck-ops.ts` keeps the held list equal to the
sheet's next entries (`syncHeld`). `deck-ops.ts`'s own op builder and `deck-post.ts` are the first design's and nothing in the app calls them (a residual to delete, with their
fixtures). The first design, still the reference for the rules: `deck-ops.ts` builds the `aired` ops from the journal (heard only with sound; the deck's own time in `ended.at`; ids from the line so a
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
