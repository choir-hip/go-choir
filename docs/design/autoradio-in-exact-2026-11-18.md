# Moving Autoradio's logic into Exact: a design

Status: proposed, 2026-11-18; reviewed by two seats the same day, both
"repair, the frame is right". The section "After review" at the end
governs where it differs from the text above it. Written by an Opus design pass from the pinned
Exact source (`mobile/.exact-src` at de212e671) and `lane/autoradio-integ20`;
nothing in it was run. Every claim about Exact cites the file it was read
from. Station 0's four tests (section 5) decide whether the order stands.
The rule it serves is in CLAUDE.md, Product: "The phone app is Exact; only
voice is native". Where the app stands today:
[the problem record](../problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md).

## The recommendation

Move the server client and sign-in first, then the run sheet and the player's decisions, then background play. Android shows real data after the first station and plays the radio after the second. Native code shrinks to a "deck", a queue of prepared audio with callbacks, plus the voice.

Three facts read from Exact's source shape the plan:

- **On iPhone, shared code keeps running with the screen locked while our audio plays.** Exact marks the page frozen only for "Background without active audio eligible for background execution" (`docs/contract-for-agents.md:4929`). Read, not yet run on a phone.
- **On Android it does not.** The page freezes when the activity stops unless a foreground-service claim is held (`:4926`). Exact has two kinds of service, capture and call (`host/android/.../ForegroundService.kt:30,33`), with types microphone, phone call and screen projection (`:216-218`). There is no media-playback kind. The `audio` element stops when the activity stops (`ExactActivity.kt:601`), and so does Exact's Android speech (`packages/speech-synthesis/android/SpeechSynthesis.kt:15`). Background radio on Android needs a small patch to Exact, carried in `mobile/exact-patches`.
- **There is almost no TypeScript to build on.** `mobile/AutoradioLab/app.ts` on `lane/autoradio-integ20` is 11 lines, and no other `.ts` file exists under `mobile/` on any branch I checked. The problem record's "8 files, 4,884 lines" does not match the branch. The shared layer will be new code written from the Swift and the fixtures.

## 1. What Exact's shared layer can carry

"Read" means read in the pinned source at the cited line. Nothing was run.

| Need | What Exact gives (where) | iPhone / Android | Missing |
|---|---|---|---|
| HTTP with our headers | `fetch`, `Headers`, abort, deadlines (`docs/reference.md:2084`, `:2545`) | Both | No `FormData` (`:2084`); takes multipart is built as bytes |
| Large upload and download | `exactBodyFrom` streams from disk; `exactBodyTo` writes to a file, up to 1 GiB (`:2084`) | Both | None |
| Event stream | `fetch` with `exactStream` reads server-sent events (`:2605-2633`); long poll `exactHeld` (`:2579`) | iOS stated; Android not named | A stream sent as a POST (our ask route, `ChoirClient.swift:337`) is not shown; the stream closes when its resource leaves the view |
| Structured data | `storage.sqlite` (bundled SQLite 3.54.1), `storage.fs` (`:2324-2345`) | Both | Native code must not open the shared side's databases (`:2342`) |
| Encrypted at rest | Not found for app storage on Apple. Android encrypts app files until first unlock (`host/android/README.md:374`) | Android only, by the platform | Unknown whether Exact's iOS files get the file protection our mail store sets today (`ContentStore.swift:90`) |
| Sign-in key | Secret store: Keychain on Apple, Keystore-wrapped on Android (`reference.md:5159-5167`) | Both | None |
| Background timers | Timers belong to an "answer" (`:2227-2236`); background wakes last 30 s and run no timers (`docs/recipes/background-work.md:5,57`) | See above | No free-running loop; shared code acts when an event or reply arrives |
| Hashing | SHA-256/384/512 and ECDSA P-256; anything else is refused (`:2090`) | Both | No HMAC or AES; our Swift uses only SHA-256 |
| XML, EPUB, PDF | `DecompressionStream("deflate-raw")` (`:2087`) unzips an EPUB | Both | No XML parser and no PDF text found |
| Local search, embeddings | SQLite | Both | FTS5 unverified (`js/Cargo.toml:54`); embedding models stay native |
| Share, file picker | System pickers (`:3920-3947`); `share(...)` (our `native-modules.md:199`); Android share sheet (`ExactActivity.kt:624`) | Both | None |
| Driving native audio | Package calls, `changed` meaning "ask again", `emit` (our `native-modules.md:69-102`); Android packages are Kotlin classes (`README.md:798-806`) | iOS proven by the spike | Delay unmeasured: fine at entry boundaries, so gapless joins stay native |

## 2. Where the line falls

The 181 names come from 19 interface files (143 names; `mark.json` is empty) plus 38 that only `exact.json` declares (23 `lab*`, 15 `choir*`).

| Group | Native for good (voice) | Native for now, and what releases it | Shared |
|---|---|---|---|
| `lab*` (23) | All | | |
| `choir*` (15) | Listen, ask, talk, hands-free | Play and pause: the deck | Auth, sign in and out, refresh, stories |
| voice (4), talk (5) | Voices, speaker profile | | `conversationInfo`, `answerAction` |
| onboarding (20) | Microphone, hearing, talk, demo say | | The other 15 |
| radio (2), bar (4), queue (3) | | The deck (Android background) | Page models, jump, speed |
| takes (2) | The recorder | | List, publish |
| books (10) | | PDF import (no PDF text in Exact) | The rest; EPUB in TypeScript |
| podcasts (8), joins (2) | | Play goes through the deck | Feeds, subscriptions |
| mail (10) | | Store encryption, until test B passes | The rest |
| reader, files, textedit, paper, cards, spine, settings, notebook (73) | | | All |

**The run sheet player.** The Swift already has the seam: `RunSheetPlayer` decides, and a `PlaybackEngine` protocol makes the sound (`RunSheetPlayer.swift:7-8,22-58`).

Everything that decides moves to shared code: the reducer, the store in SQLite, the stream, the writer, the on-air decisions, and an outbox of ops not yet posted.

The native core is a **deck**: a queue of items, each "speak these paragraphs as document D" or "play this URL from start to end". It plays items back to back with no gap, reports started, ended, failed and position, handles interruptions and Now Playing, and keeps playing if shared code stops. Ended events go to a journal file that shared code reads and posts when it runs again; fixture 34 already covers a late on-air report. Shared code hands the deck up to a dozen entries ahead (`HOLD_AHEAD` 12 in `radio-runtime.ts`); the deck holds at most 100 (`DECK_HELD_MAX`, the sheet's bound).

**First Android build, at minimum:** a Kotlin deck of about 500 lines using Android's `TextToSpeech` and Media3 ExoPlayer. Exact already uses Media3, and packages may declare Maven libraries (`README.md:846-851`). No models and no microphone. It plays only while on screen until station 3.

## 3. What is reused

- **Fixtures:** Go runs `mobile/fixtures/runsheet/*.json` (`internal/runsheet/runsheet_test.go:13`). Exact's apps test with `bun test` (`apps/shelf/app.test.ts:4`; `reference.md:2225`). A TypeScript test globs the same folders (and `mobile/fixtures/takes/routes.json` on main), so a new fixture must pass both.
- **CI:** Bun is in neither `flake.nix` nor CI, so station 2 adds pinned Bun 1.4.2 and a CI step. Go cannot run TypeScript without adding a JavaScript engine, which I would not do. The Swift twin is deleted in the same change: two copies, down from three.
- **Swift tests (1,237 counted on integ20):**
  - about 290 stay with the voice (TurnPolicy, LocalAck, Talk, VoiceChoice, HandsFree and smaller ones);
  - about 800 carry product rules and are ported to Bun with a fake deck: run sheet and player about 230, settings and setup about 150, files and reader about 100, takes and learning about 90, client and content about 75, podcasts about 70, search 54, mail 22;
  - about 145 die with the code: Swift view-model and layout tests, plus Repair tests whose scenarios are not product rules.

## 4. The order

The rules hold throughout: no switches, each station deletes the Swift for its area, and a station lands with its proof. After station 2, stations 4 to 9 can run as parallel lanes, one Mac build at a time.

**Station 0 (half a day):** install the Android toolchain (`host/android/README.md:9-15`), build Exact's `hello`, then our app with no Kotlin, and run tests A to D. Nothing moves. It also shows what Android does with a name that has no implementation, which the source does not state outside the agent (`:834`).

**Station 1, the client and sign-in.**
- **Moves:** a TypeScript client with our headers and back-off; device-flow sign-in; the key in the secret store; computer discovery; sign-out wipe; paper front; account and computer settings. Native callers get the identity from one call and keep it in memory only.
- **Deletes:** `ChoirClient`, `ChoirSignIn`, `Accounts`, `ProofKey`, most of `AccountWipe`, the paper source, and 13 names.
- **Size:** about 1,800 Swift lines out; 1,600 TypeScript and test lines in.
- **Proof:** the owner signs in once more; the paper shows the staging front; Android shows the real paper. The old Keychain item stays, so a revert is safe.

**Station 2, the run sheet and the decisions.**
- **Moves:** the reducer (Go and Bun on the fixtures), store, stream, writer, outbox and on-air decisions. The deck goes in front of today's Swift engine; the Kotlin deck v0 is added.
- **Deletes:** `RunSheet*`, the decision half of `RunSheetPlayer`, `RadioPage`, `DJQueueAdapter`, `QueueView`, and the radio, bar and queue names.
- **Size:** about 3,200 Swift lines out; 3,700 TypeScript, Kotlin and test lines in.
- **Proof:** the four `radio-page/tour-*.steps` on the simulator, then a locked hour on the owner's phone with aired ops on staging matching what was heard. Android plays with the system voice on screen.
- **Revert:** possible, because the server holds the sheet.

**Station 3, background play.**
- **Moves:** a media-playback service kind added to Exact (about 40 lines beside `ForegroundService.Capture`, offered upstream); the Kotlin deck runs in it with a media session; the decision parts of `Playhead`, `SegmentTracker`, `PlaybackContinuity` and `OpeningCache` move to shared code; the deck gets its journal.
- **Size:** about 900 Swift lines out; 900 TypeScript and Kotlin lines in.
- **Proof:** a locked hour on both phones, with ops landing on staging.
- **Irreversible:** we carry an Exact fork until upstream takes the patch.

**The rest:**

- **4, podcasts and clips:** feeds in TypeScript, downloads to files; about 2,000 lines out.
- **5, files, reader and books:** PDF text from the server, deleting PDFKit; about 3,000 lines out.
- **6, mail:** about 1,200 lines out. **Irreversible:** the phone's mail store is wiped and rebuilt.
- **7, content store and search ranking:** about 2,000 lines out.
- **8, takes, notebook and learning:** about 1,300 lines out.
- **9, cards, settings, themes, onboarding and demo data as JSON:** about 10,000 lines out.
- **10:** what remains in `AutoradioKit` is voice.

**First for Android:** the client and sign-in. The radio needs the run sheet from the server, which needs the client, and station 1 risks the least of the owner's daily iPhone use. Android plays one station later.

## 5. What could make this the wrong plan

**A. Shared code does not run, or does not hear events, on a locked iPhone.**
- Test (30 minutes): on the phone, a deck whose `ended` event posts a test op. Lock the screen and watch the server log.
- Fallback: the deck plays a longer queue alone and journals; ops arrive late.

**B. Exact's iOS files are not encrypted at rest.**
- Test (20 minutes): write with `storage.fs`, then read the file's protection class from a native call.
- Fallback: a native seal and open pair, with the key in the Keychain or Keystore, or the mail store stays native.

**C. No event stream with our headers, or no POST stream, on Android.**
- Test (30 minutes): open `/api/autoradio/sheet/stream` on staging from an emulator.
- Fallback: the long poll, and for the ask route, a GET stream keyed by an id that a POST created.

**D. The old Android phone runs Android 9 or older.**
- Exact needs Android 10 at the least (`README.md:22,1547`).
- Test (2 minutes): `adb shell getprop ro.build.version.release`.
- Fallback: an emulator.

## 6. Build times

**Where the time goes** (`mobile/scripts/build-lab.sh`; our `build-and-ship.md`, `testing-and-debugging.md`):
1. Model libraries: built once, about 10 minutes for each FluidAudio slice, then cached.
2. Copying the kit into the package and generating the interface.
3. Exact's Rust host: 5 min 37 s cold, 220 s in a new worktree.
4. Swift: 48 s for Exact's host; 128 s for all Swift in a new worktree.
5. The contract bake, signing and install.

**Measured on warm builds:** a Swift change about 60 s, a TypeScript-only change about 57 s, a contract-only change 12 to 40 s.

**What the move changes:**
- **The Swift is not why warm builds are slow.** The fixed cost is Exact's pipeline.
- **Saves:** perhaps tens of seconds on cold and Swift-touching builds (inferred, not measured); the copy-and-generate step for most names; and logic tests, which run in seconds under Bun.
- **Does not change:** the 15-minute cold build, the Rust host, the model libraries or the 12 GiB build tree.
- **Adds:** an Android build (Gradle, NDK, Rust for Android). Whether it can run on the Linux build VM instead of the Mac is untested.

## After review (2026-11-18)

Two seats read this design and the mission file against the Swift it
replaces and the server (seat 1, a Sonnet reviewer who also checked the
Exact citations; seat 2, DeepSeek). Both returned REPAIR with the frame
upheld. What changes:

### The deck applies named rules; it does not "decide nothing"

Shared code may be frozen when the deck needs an answer, so the deck carries
these rules itself, and only these. They are data-driven where they can be:
shared code states the rule on the item, the deck applies it.

| Rule | Why it cannot wait for shared code | Today's code |
|---|---|---|
| A transition item plays only if the item before it was heard to its end | a skip on the lock screen must not play a line that refers to what was skipped | `RunSheetPlayer.nextToAir` |
| When the audio session cannot be taken, the item stays on air with no outcome and the deck retries at 1, 2, 4, 8, 15 s | shared timers exist only inside an answer | `RunSheetPlayer` hold, `SessionCoordinator` |
| Interruption: resume only on the system's word or the listener's press; route loss pauses and never falls to the speaker | native audio policy | `PlaybackContinuity` |
| Barge-in: the voice lowers or pauses the deck and the deck reports the block and position it stopped at | only the engine knows the remainder | `rememberPlace`, `spokenRemainder` |
| Two items in a row that made no sound stop the deck | a dead engine must not burn the queue | `RunSheetPlayer` |
| One voice at a time: the voice owns the deck's level | enforced native to native | `SessionCoordinator` |

The deck holds one state for "why it is silent" (playing, paused by the
listener, paused by the system, held for the session, held for the engine,
stopped, empty), not a set of flags. Three review rounds on the Swift repair
of 2026-11-18 each found a new pair of flags that disagreed.

### The journal is the evidence

The deck writes an ordered journal: for each item, started and ended, with
the device's time, the block and position reached, and whether audio was
rendered (samples reached the output, not "the utterance finished"). The
journal survives a kill. Shared code reads it before it restores the run
sheet (today a restart closes the on-air entry as failed before anything
else runs, which would lose a verdict) and posts each op with the journal's
own time, since the server judges lateness on the op's `ended.at`. An item
with no rendered audio is never posted as heard.

### What shared code may assume on a locked phone

On iPhone shared code stays live only while audio is actually sounding
(`docs/contract-for-agents.md:4929`: "an idle playback category grants
nothing"). In a gap between items, while a block is being synthesised, or
after a pause, it can freeze and its stream closes; a stream belongs to a
resource in a view and nothing reconnects it but an action. So: the deck
keeps the session through gaps and holds two or three items; the run sheet's
feed is designed as a long poll first (`exactLongPoll`, host-managed
back-off) and a stream second; and whether a deck event can cause an op to
be posted with no touch is unproven. Station X0's locked-phone test decides
it. If it cannot, ops are posted when shared code next runs, with the
journal's times, and anything past the server's lapse is no verdict.

### Order

The deck comes before the decisions move. The Swift player is first made to
drive the engine through the deck (journal, rules, one state), and that
passes a locked hour on the owner's phone. Only then do the decisions move
to shared code. The owner's daily phone never runs a new decision layer
over an unproven deck.

### Sign-in

Station X3 moves sign-in, accounts and the proof key. It does not delete
`ChoirClient`: twenty-seven Swift files use it until their own stations, so
it is fed the identity by one call and goes at the end. Three things are
decided before X3 starts: the secret store's behaviour on a locked phone
(today's key is readable after first unlock; a locked hour must still post),
that shared code is not updated over the air while it holds the key
(`deploy.store` stays `"0"`), and one sign-out wipe across both languages
with the generation passed to shared code. The old Keychain item is deleted
at the move, not kept for a revert.

### Honest outcomes on Android

An emulator with no sound cannot produce `heard`. The emulator proves the
transport and the reducer: a muted deck yields no verdict. The first `heard`
from Android needs a person listening.

### What each station also carries

What local state it moves and what is fetched again (run sheet ops pending,
podcast subscriptions and positions, book positions, settings, voice, speed
and theme, the kept opening); which Swift tests are ported, which stay with
the voice, which die, by file. `ClipFingerprint` is signal processing: it
stays native and is named in the voice list. The Mac app builds from the
same commit and is checked at the last station.

### Where data may go (owner, 2026-11-18)

Autoradio is an interface into the listener's own Choir computer. Nothing
the phone holds (mail, an imported file, a book) is withheld from that
computer or its desks. An earlier note in the review of this design said a
PDF's text could not come from the server because local files must not go
there; no such rule exists, and step 5's plan ("PDF text from the server,
deleting PDFKit") stands. Where logic runs and where data may go are
separate questions.

### The gate

Lines and names can be gamed by one generic call. The gate runs the pages on
the web host with a fake native side that answers only the voice list, scans
every shipped source including the app target, refuses an interface entry
that takes an opaque payload, requires the interface to equal the voice
list at the end, and carries a planted violation it must catch.

The file-by-file map of the Swift tests, and where each rule goes, is
[autoradio-swift-test-map-2026-11-18.md](autoradio-swift-test-map-2026-11-18.md).
The gate's voice-and-deck list is `scripts/autoradio-native-voice-list.txt`.
