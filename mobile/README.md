# Autoradio lab (`mobile/`)

Design: `docs/design/autoradio-lab-2026-11-10.md`. This directory holds `AutoradioKit`
(a host-agnostic Swift package, iOS 18+ / macOS 15+), the `autoradio-replay` command, and
(later) the Exact host app `AutoradioLab`. Mutation class: green (new tree, nothing in the platform runs it).

## Layout

- `AutoradioKit/` Swift package: events, stage protocols, registry, configuration, turn engine, replay, Apple-framework stages.
- `AutoradioKit/Sources/autoradio-replay/` the Mac command.
- `fixtures/` kept turns (tracked) used by the proof.
- `scripts/` `xe.sh` (run a command with the Xcode toolchain outside the Nix shell), `proof-replay.sh`.

## Ways the pieces can fail (written before the code, per the repo testing rule)

### Turn engine
1. Two turns overlap (second speech starts while the first is acknowledging): the engine must not resolve stages twice or interleave one record's events with another's. Rule: a new speech during acknowledging/holding is queued as `spokeAgain` and starts the next turn only after the phase returns to `playing`; the button always wins and ends holding.
2. A configuration change lands mid-turn: stages are resolved at the start of the turn and frozen; the change applies to the next turn. The record names the configuration that ran.
3. A stage throws or never returns (no transcript, desk silent): every await on a stage has a deadline; failure emits `error`, the turn still ends, the radio resumes. Never silence, never stuck.
4. Candidate speech turns out not to be a turn (noise, a cough, too few words): playback must return to normal and no record of an addressed turn is made.
5. Addressed but no speech (button tapped by mistake): `noSpeechTimeout` closes the turn.
6. Crash/restart: open turns are not resumed (repo rule); nothing is persisted mid-turn except the audio already written.
7. Clock: stamps come from one clock; in replay time is virtual and moves with the audio, stage durations are wall time added to the start stamp.

### Yield policy
1. Dip never returns (speech ended, state stuck at dipped): the policy is a pure function of the situation and the engine applies it every frame, so a silent frame with `notATurn` or no addressing returns to `play`.
2. Pause flaps on and off as the VAD toggles: once paused, stay paused until the engine says `notATurn` or the turn is over; speech seconds reset only when the VAD ends a speech.
3. A long dictation never pauses because the VAD hangover splits it: `speechSeconds` counts from the first speech start of the candidate segment, not the last VAD burst.
4. A cough pauses a programme: dip only after `dipAfter`, pause only when addressed or after `pauseAfter`; a segment that is not a turn resumes.
5. Dip lowers the reference signal too: the reference is what plays, so the engine records the level changes as `yielded` events.

### Replay
1. A different configuration changes the number of turns found (button mode with no button recorded): replay injects a tap at the first energy onset when the kept turn has no recorded inputs and the configuration is button-addressed, and says so in a `note` event.
2. Time runs ahead of the engine (fast mode): the bus delivers each frame to the engine in lockstep, so stamps follow the audio.
3. Stage timings are meaningless in virtual time: measured as wall time and added to the stamp.
4. The kept audio is not 16 kHz mono: it is converted on read.
5. The reference file is missing or a different length: it is optional and padded with silence.
6. A replay mutates the kept turn: it never writes into the turn folder; `--out` is separate.
7. Apple recognition needs authorization on the Mac: the command embeds usage strings; a missing authorization is an `error` event and a non-zero exit in the proof.

## What the listener does (first-run path)

The app is called Autoradio and opens to the radio. No stage names, ids or jargon on this path; the Lab and Trace live behind the last tab, "Lab".

1. Opens the app. Not signed in: a plain screen, "Sign in to hear the paper and talk to your computer", and one button, Sign in.
2. Taps Sign in. The browser opens the approval page (device flow); the app shows the short code in large type and "Approve this sign-in in your browser". If the browser did not open, an "Open the approval page" button. The code and a Cancel are the only other controls.
3. Approves in the browser (passkey) and comes back. The app has stored the key (Keychain), found the listener's computer, and shows the Player.
4. The Player has loaded the newest story from choir.news. It shows the story title, the passage being read, "3 of 12", a large Play button, and below it a list of stories.
5. Taps Play. The first tap starts audio (the microphone permission prompt appears here, once). The passage is read aloud, block by block.
6. Taps a story in the list: it loads and plays from its start.
7. Holds the big "Hold to talk" button and speaks; the programme dips then pauses; the screen shows what the radio heard. On release the radio says it is on it, then the revised text from the listener's computer is read from the point that changed.
8. If anything fails, one plain sentence appears on the Player ("Could not reach choir.news. Check your connection.", "Your sign-in is no longer accepted. Please sign in again.", "Your computer is busy right now."). Nothing falls back to a stub silently. The details are in the Lab tab, Trace.
9. Sign out is in the Lab tab, under Account. It removes the key from the phone and asks the server to revoke it.
10. The Lab tab (stages, parameters, configurations, kept turns, replay, Trace) is the old lab, unchanged. The default configuration there is named "listener" (choir-front, choir-system1, choir-desk, Apple speech and voice, button to talk); the stubs and the fixture programme stay selectable.

### Choir client stages (`choir-system1`, `choir-front`, `choir-desk`, sign-in)

Written before the code. Base URL defaults to `https://choir.news`; the key is a `choir_sk_` bearer from a `KeyStore`. The key is never put in an event, a turn record, a log line, an error message or the UI.

**Client and sign-in**
1. No key stored: every authenticated call throws `ChoirError.notSignedIn` before any request; the stage fails for the turn, the engine falls back.
2. 401 (key revoked or wrong): `ChoirError.unauthorized`; the key is kept (a retry may be a transient proxy answer) but the UI shows "sign in again" after the first 401, and Sign out clears it.
3. 403 (key lacks a scope): `ChoirError.forbidden`, reported with the path; nothing retried.
4. Timeout / no network / TLS failure: `ChoirError.transport`; every request has a deadline (`timeoutSeconds`), none waits forever.
5. 5xx: `ChoirError.server(status, message)`; message is the server's `error` string only.
6. Body is not the expected JSON: `ChoirError.decode`.
7. Device flow: `authorization_pending` keeps polling at `interval`; `slow_down` adds 5 s; `access_denied`, `expired_token`, `invalid_grant` end the flow with that cause; the whole flow gives up at `expires_in`. A second Sign in while one is pending cancels the first. The browser cannot be opened: the code and URL stay visible in the UI so the listener can type them. The key arrives once; if storing it fails the flow reports failure and the key is dropped from memory.
8. Sign out with no network: the local key is deleted regardless; the server revoke is best effort and says so.

**System 1 (`choir-system1`)**
1. 429 with Retry-After: `ChoirError.rateLimited(retryAfter)`; the stage fails for this turn. No retry, no client-side limiter (owner rule).
2. Slow answer: the route cuts at 4 s with 502; the engine's own `system1Deadline` (2 s) usually fires first and the turn acknowledges without answers. Both are recorded as errors, never as silence.
3. 503 "no decision model bound": surfaced as `server(503, ...)`.
4. 400 from the route's limits (more than 12 questions, 8 choices, state over 48 KiB, instruction over 2000 bytes): the stage clips the state and the question list before sending, so a 400 means a client bug and is reported verbatim.
5. A question comes back with `error`, or with a choice not offered: that question is left out of the answers (the holder treats a missing answer as "no"), the others stand. All questions errored: the stage throws.
6. Two calls at once: refused client-side with `ChoirError.busy` (one call per turn; never concurrent).
7. Cancelled mid-flight (the engine deadline): the URLSession task is cancelled; no half-recorded answers.

**Programme (`choir-front`)**
1. Empty front page (no stories): throws `ChoirError.empty("front page")`; the player keeps the fixture programme.
2. Story with no text (projected-text publication with an empty body): skipped for auto-pick; an explicit pick of such a story fails with a message.
3. `resolve` 404 (story unpublished since the list): the next story is tried when auto-picking; an explicit pick fails.
4. Very long story: blocks are split at sentence ends to at most ~600 characters, and the story is cut at `maxBlocks` (default 80) with a note.
5. A story with only citations, headings or lists: markdown marks are stripped; a block that is empty after stripping is dropped.
6. Front page paging cursor invalid: the list stops at the page already read.
7. Cached copy: none. Every load asks the server (the paper changes).

**Desk (`choir-desk`)**
1. No key / 401 / 403: the stream throws at once; the engine records the error and the radio resumes.
2. The programme being played is a published story, not the listener's document: the first turn forks it (`POST /api/texture/fork` with the listener's raw words as `text`); later turns use the prompt bar with that document in `focus`. The fork's `client_request_id` is the turn id, so a retry replays rather than forking twice.
3. The programme is not a Choir story (fixture) or has no known document: the unfocused prompt bar routes the words; if the answer carries no `doc_id` the stage ends with an error naming that (server gap).
4. 409 command conflict / stale head: reported; no automatic retry with another id.
5. The stream (`GET .../stream?format=json&after=`) drops, 5xx, or returns garbage: polled with a doubling pause; three consecutive failures end the stream with the error. A 409 `cursor_expired` restarts from 0 and de-duplicates by revision id.
6. A revision that is not the desk's (the listener's own revision, or a version number at or below the listener's) is ignored; only `author_kind == appagent` above the listener's version is delivered.
7. Trajectory terminal with no agent revision: the stream ends with a note, no revision (the engine's hold timeout also bounds this).
8. The document changed underneath (someone edited between the listener's words and the revision): the revision is whatever head the desk produced; the stage delivers its text from the first paragraph that differs from what was playing, but never earlier than the block after the listener's position.
9. Revision text identical to what is playing: nothing delivered.
10. Engine cancels (hold timeout, new speech): the polling task ends on cancellation; no request is left running.

**When a desk turn is over** (found by the first staging run: a mixed turn got its revision at 38 s and then "did not finish within 90 s"). The stage polls the document's stream, and a live document's stream never closes, so waiting for a terminal state waited for the stage timeout. A desk turn is over at the first of:
1. the document's trajectory reports `work_state: terminal` after at least one desk revision;
2. `settleSeconds` (default 4) pass after the last desk revision with no newer one (a revision that changes nothing to speak counts as the desk's answer);
3. `timeoutSeconds` (default 110; an error if the desk wrote nothing).
The engine's own hold timeout (120 s) bounds it from outside. Tests: `testDeskTurnEndsAfterQuietWhenStreamNeverSettles`, `testRevisionThatChangesNothingStillEndsTheTurn`.

**Computer discovery is not on the turn path.** The first staging run showed two `GET /api/compute/status` calls per turn and a System 1 stage of 1.7 to 2.2 s although the call itself failed in 617 ms: System 1 and the desk start together, each found no stored computer id and each asked. Now: discovery is single-flight, and it happens at sign-in, at app start (`ChoirApp.refresh`) and before a replay (`ChoirEnvironment.warm`), so a turn makes exactly one System 1 request (test `testWarmThenTurnPathMakesNoDiscoveryCall`). A failed System 1 call falls back at once: the stage throws, the engine goes on to the acknowledgement. The engine used to log "no answer within 2.0s" for a thrown error too, which hid the cause; it now says that only when the deadline really passed.

**System 1 questions carry `criteria`**: every `choice` question sends one line per choice (the route needs the keys to equal the choices; the defaults in `Question.defaults` have written descriptions, other questions get a plain "The answer to the question is X." line).

**What the acknowledgement waits on** (from the engine code and the figures in the first staging run; I have not seen the records themselves, so the split of the 2.7 to 4.4 s is partly inferred). `TurnEngine.runPipeline` starts the desk at once but does not start the acknowledgement until the System 1 stage has returned or failed (`withDeadline(system1Deadline)`), because the acknowledgement line is chosen from its answers. Before the turn: final transcription (Apple speech, up to `stageDeadline`). So ack = transcript final + System 1 (1.7 to 2.2 s in those runs, of which discovery was most) + the Voice stage's first audio (`voice.firstAudio`, on-device speech synthesis, the largest variable in the mixed turn at 4.4 s). Measured floor with no network at all (fixed System 1, stub desk, `proof-replay.sh` today): ack 2.2 to 2.3 s, so about 2.2 s of every ack is transcription plus first synthesized audio before any Choir call is added; the staging figures are that plus the System 1 stage. With discovery removed the System 1 term should fall to its request time (about 0.6 s for a failure). Next lever, not done here (engine behaviour): start the acknowledgement without waiting for System 1 and let the answers choose only the holding lines.

### Server gaps (what the client needs that today's routes do not give)

No Go code was changed. Each line is a route, the missing piece and what the app needs.

1. **Block-addressed prompt** (`POST /api/prompt-bar`, `POST /api/texture/fork`): a Texture focus has no position (`focus.selection` is dropped on that path, `prompt_bar.go:245`; no `block_id`). The app needs "revise from paragraph N". Workaround in the app: the listener's raw words come first, followed by one line naming the paragraph number, the title and the first 300 characters of the paragraph being heard. Turn it off with the desk's `placeInPrompt` parameter.
2. **Which revision answers my turn**: the prompt bar's receipt names the listener's own revision but the stream's `version` events do not say which request a revision answers. The app uses "author_kind is appagent and version_number is above the listener's revision". Wanted: `request_id`/`command_id` on the `version` event (the field exists on lifecycle events; it is empty on the version projection for a turn commit) so a revision can be matched to the turn.
3. **Unfocused prompt-bar names no document**: the 202 for a routed submit carries `doc_id` only when the conductor opened a Texture in the same call; otherwise the app polls `/api/prompt-bar/submissions/{id}` for `decision.doc_id` (about 12 tries). When the conductor routes to a package desk or answers in chat there is no document to follow and the desk stage ends with a "server gap" error. Wanted: a status field `result_ref` that is always set once the submission is settled.
4. **A published story's audio and time spans**: `body_doc` has no speech or clip block and publication refuses embeds, so the app speaks the paragraphs with the on-device voice. Wanted: a run-sheet block (design lane).
5. **Device key**: never expires, has no refresh, carries `owner:acts` and `admin` scopes, and is owner-wide, so every request must send `X-Choir-Computer`. The app discovers the id with `GET /api/compute/status` (an owner-wide key may read it unnamed; `computers[]`, first `interactive` with a `computer_id`, preferring `current`) and stores it beside the key. Wanted: `computer_id` in the `/auth/device/token` response (saves a call and a failure mode: a new account has no computer until the first status read creates it, and the app then says "try again in a moment"); narrower scopes at `begin`.
6. **System 1 deadline**: the engine gives System 1 2 s (`EngineLimits.system1Deadline`) while the route's own deadline is 4 s and a Jev call is usually 300 ms to a few seconds. A slow call is cancelled by the engine and the turn acknowledges without answers. The client default timeout is 5 s; raising the engine limit is the audio/engine lane's call.
7. **429 Retry-After** is always `1`; the client reports the failure for the turn and does not wait (owner rule).
8. **Sign-out revocation** needs `DELETE /auth/api-keys/{id}` with `manage:keys` or `admin` (a device key has both); the id comes from the token response (`key_id`). If the call fails the key is still deleted locally and the Lab tab says it was not revoked.

## The kit in one page

Everything a stage concludes is a `LabEvent` on one `EventLog` (monotonic host-clock stamps; audio frames are not events). A turn is kept as a `TurnRecord`: the folded events, the `LabConfiguration` that ran, device model, OS, app build, and the outcome (`heardThrough`, `spokeAgain`, `skipped`, `corrected`, `unknown`).

**Stage protocols** (`Sources/AutoradioKit/Stages.swift`), one per `StageKind`:

| Kind | Protocol | Shape |
| --- | --- | --- |
| `micSource` | `MicSource` | `inputPreference` (built-in or current route) |
| `addressing` | `Addressing` | `decide(_ AddressInput, now) -> AddressDecision`; `holdsTurnOpen`; `noSpeechTimeout` |
| `speechBounds` | `SpeechBounds` | `feed(frame) -> BoundsUpdate`, `reset()`, `speechSeconds(now)` |
| `speakerCheck` | `SpeakerCheck` | `isListener(frames) async -> Bool` |
| `transcriber` | `Transcriber` | `begin() -> TranscriptionSession` (`append`, `partials`, `finish()`, `cancel()`) |
| `yieldPolicy` | `YieldPolicy` | pure `target(for: YieldSituation) -> PlayLevel` (play, dip, pause) |
| `system1` | `System1` | `answer(System1Input{context, listenerWords, questions}) -> [System1Answer{option, score, ms}]` |
| `holder` | `Holder` | `lines(for: HolderInput) -> AsyncStream<String>` |
| `voice` | `Voice` | `speak(AsyncStream<String>) -> AsyncThrowingStream<AudioFrame>` |
| `desk` | `Desk` | `submit(DeskTurn) -> AsyncThrowingStream<Revision>` |
| `programme` | `Programme` | `load() -> ProgrammeDocument` (blocks: speech, clip, listenerPassage) |

Every implementation is a class conforming to `Stage` (`static stageID`, `static descriptor: StageDescriptor` with parameters, defaults, ranges, needs). To add a model:

```swift
registry.register(.transcriber, MyParakeet.self) { params in MyParakeet(params) }   // uses stageID and descriptor
```

`StageRegistry.builtIn()` registers lane K's stages; wave B builders add theirs after calling it (or inside `registerAppleStages`-style functions of their own). `LabConfiguration` is `{name, stages: {kind: {id, params}}}` JSON; the `TurnEngine` resolves a turn's stages at the start of the turn, so a change applies to the next turn.

`ModelStore` (fetch on demand to Application Support, pinned by SHA-256, resumable, unload what the configuration does not need) ships with no models registered.

Audio: stages never touch AVAudioSession. The host supplies an `AudioSessionOwner` (`claim(.listening|.talking, ...)`); every claim is logged as an `audioSession` event. The Exact app adapts Exact's claims; the replay command uses `NullSessionOwner`.

### Desk contract (not implemented; the server lane supplies it)

`ChoirDesk.submit` should POST the turn (`turnID`, `programmeID`, `blockIndex`, `listenerWords`) as a prompt at a place in the document and stream `Revision` values `{id, fromBlock, blocks, keepRest, isFinal}` back. No endpoint is invented here. `RemoteSystem1` posts `{context, listener_words, questions:[{id,text,options}]}` with `Authorization: Bearer <Keychain service news.choir.autoradio, account system1>` and expects `{answers:[{question_id, option, score}]}`.

## Running

```sh
mobile/scripts/proof-replay.sh                       # Mac end-to-end proof; exits non-zero on failure
mobile/AutoradioKit/.build/release/autoradio-replay --list-stages
mobile/AutoradioKit/.build/release/autoradio-replay --turn mobile/fixtures/turns/plain --config mobile/fixtures/configs/open.json
python3 -I mobile/scripts/make-fixtures.py           # regenerate fixtures (macOS: say, afconvert)
```

All Xcode/Swift commands run outside the Nix shell through `mobile/scripts/xe.sh`.

## The host app: Exact

`AutoradioLab/` is an Exact app (Contract screens: Player, Lab, Trace; no SwiftUI, no UIKit views of ours). AutoradioKit is attached as the Exact package `@choir/autoradio` (`exact-package/`): `scripts/sync-exact-package.sh` copies the kit sources in and writes `exact-package/apple/exact.json`; `scripts/gen-exact-interface.py` writes the interface.

Pinned Exact: `expo/exact` at `76cd50f9e863129b3fc75db1673d6ad4bbf79258`, checked out at `mobile/.exact-src` (gitignored), Bun 1.4.2 at `~/.bun-1.4.2`. Commands: `mobile/scripts/exact.sh ios` (simulator), `mobile/scripts/exact.sh ios --device --phone 00008120-00114DE80A90C01E`.

Signing: the free provisioning team is `QSGU6VRJL6` ("Yusef Nathanson (Personal Team)"); `N9HKM55LWY` is only the certificate's person id. Exact needs an installed profile for `news.choir.autoradio.lab`; `scripts/make-profile.sh` obtains one with a stub Xcode project and `-allowProvisioningUpdates`.
