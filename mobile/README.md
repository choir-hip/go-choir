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

## Programme player (station P1): ways it can fail (written before the code)

Design: `docs/design/autoradio-dj-programme-2026-11-11.md` section 6; vocabulary: `docs/design/autoradio-learning-engine-2026-11-14.md` 12.3.
Fixtures: `mobile/fixtures/learning/` (from the server builder, `lane/arl-l1a`). Tests: `ProgrammePlayerTests.swift`; the numbers below are in the test names.

### Reading the stretch
1. A network chunk ends inside a line (JSON or SSE): nothing is parsed until the line is complete; the line ending is LF, CRLF or a lone CR.
2. Blank lines, SSE comments (`: autoradio`), unknown event names (`stretch`, `handoff`), a line that is not JSON, a JSON value that is not an object: ignored and counted, never a crash, never a gap in the programme.
3. A line the phone cannot speak (unknown `kind`, text empty or only blanks) is dropped. The server is the validator of everything else (`p_heard`, `caused_by`, cites in the candidate set): the phone does not judge those, so a fixture line that is server-invalid but speakable is queued.
4. A title with a newline, a control character, or over 200 bytes becomes one clean line; no title falls back to a plain label of the kind. A raw ref is never a title or a chip label.
5. `cites` as plain strings (the fixture) or as `{ref, role, label}`: a chip exists only for a cite with a label; one ref is listed once.
6. A line with no `stretch_id` or `segment_id` (the fixture has none) gets local ids; a segment id seen twice is appended once.
7. `done` ends the stretch; `error` is logged and keeps what is queued; a stream that ends with no `done` keeps what it has.

### Blocks
8. `SplitBlocks` is mirrored exactly (blocks.json): CRLF and lone CR are line endings, a whitespace-only line is blank, leading, trailing and many blank lines vanish, a single newline joins, empty text has no blocks.
9. The server's `block_count` is kept and is the count reported; the phone computes a count only when none was sent (fixtures). A sent count of zero or less is treated as not sent. A differing count is noted in the segment (`blockCountMismatch`) and the reported `lastBlock` is clamped inside the server's count.
10. A paragraph over 600 characters is spoken as several playback units; every unit maps to the one server block it came from, so the reported `last_block` is the server's index, never a unit index.

### The document
11. A document saved before this change has no `segments`: it decodes as empty and plays as one article.
12. A revision applied to a segmented document keeps every remaining segment's range right (inserted blocks shift later ranges; a segment cut through by a replacement is dropped).
13. A segment appended to a document whose id changed meanwhile (the listener asked for something else) is refused, not mixed in.

### The player
14. A segment arriving while one plays does not restart, skip or repeat the block now playing.
15. A segment arriving after the programme ran dry starts at its first block when not paused; while the listener has paused, it queues and makes no sound.
16. Skip goes to the first block of the next segment; on the last segment it does nothing (it never wraps and never reports a skip that did not happen). Previous goes to the start of this segment, or to the one before when already at its start. A jump to a segment outside the list does nothing.
17. A block that did not play (the voice is missing or failed) is not a finish: the segment gets `failed`, never `finished`.
18. Observers (station P2): `segmentBegan`, `blockBegan(lastBlock)` once per server block (not once per unit), `finished` only when the last unit of the last server block played to its end, `skipped` only for a forward skip, `rewound` for a backward jump, `superseded` when the document is replaced mid-segment, `paused` and `resumed` for the listener, `cut` and `cutEnded` when the engine yields to a turn. No event is emitted twice for one transition.
19. Now playing shows the segment title and kind and the labelled sources, never raw refs; the lock-screen title is the segment title and the artist is "Autoradio" with the kind; the player's programme list is segments, not blocks, once a document has segments.

### The stage
20. No stretch (route 404 or 503, 401, a transport failure, a stream with no speakable segment): the front-page article plays exactly as before. A stretch that fails after some segments keeps what it has and is not retried by itself.
21. A reload (sign-in, a story picked) cancels the follower of the old stretch; a late segment of the old stretch never lands in the new document (13).

### What P1 is, where it lives, and what it leaves to P2

- `Choir/RadioProgramme.swift`: `SplitBlocks`, `SegmentPrep`, `ProgrammeDocument.append` and the `RadioProgramme` stage (the listener's default programme; it loads nothing: the run sheet decides what plays). The stretch decoder and the route request (`POST /api/autoradio/programme`) are shared code now (`AutoradioLab/programme-model.ts`, `radio-writer.ts`); the Swift `StretchDecoder` lives only in the tests (`Tests/.../StretchDecoder.swift`). `SegmentTracker.swift`: the playback observer. `DemoStretch.swift`: what demo mode answers on the route.
- `ProgrammePlayer`: `appendSegment`, `next`/`previous`/`jumpToSegment` by segment, `playback` (the tracker). `RadioInfo` gains `kind` and `sources`; with segments its title and position are the segment's (the list is gone: the queue is the run sheet's); `player.contract` shows the kind and the source chips.
- For P2: `player.playback.observe { PlaybackEvent }`. `lastBlock` is the server's block index; `blockCount` is the server's count. `finished` means the last playback block played to its end; the server refuses a `finished` whose `last_block` is not `block_count - 1` (a mismatch the segment records as `blockCountMismatch`). P2 owns `seq`, ids, hashes, `started_at`/`ended_at`, the buffer and what to send on each kind.
- Readings taken where the designs were open: the wire shape of a `segment` event is the segment fields of design 1.3 plus `stretch_id`, `segment_id`, `block_count` (nothing pins it until L1); the request body is `request_id`, `cause`, `local_time`, `tz` only (`after`, `heard` and the edition fields are for later stations); the player waits for the first segment before it starts (no time limit but the transport's idle timeout), then appends the rest; the cold-start ladder (design 2.1) and play-on-open are not P1; the first block of a segment is not subdivided to one sentence (only paragraphs over 600 characters are split).

### Review round 1 repairs (tests in `ProgrammePlayerRepairTests.swift`, written first)

- R1 a segment closed as `failed` or `finished` reports nothing more from its later blocks until another segment begins or the document is replaced (a multi-block segment with no voice on block 0 was `failed`, then `segmentBegan`, then `finished`).
- R2 after the programme ran dry, pause, append, play starts at the first unplayed block (it replayed the last old block and reported its segment twice). With nothing new, play still replays the last block.
- R3 the loop ends in one write that also looks at the document, so a segment appended in the window between its last read and its end is played (test hook: `beforeLoopTail`).
- Load token in `ChoirApp.loadProgramme`: a stale open drops its result and a new load cancels the old follower; sign-out invalidates loads in flight. `RadioProgramme.open` re-throws a cancel instead of fetching the article. A first-segment deadline of 8 seconds (the design names none; it covers the request and the first segment) falls back to the article and lets the stretch go; late segments of that stretch are not used.
- A bare `event: done` or `event: error` with no `data:` line is dispatched when the next line or the end of the stream arrives. Titles lose Unicode format characters (zero-width, bidi). `block_count` is a whole JSON number only (not a bool, a fraction or a string). The player view counts segments in a programme.
- `SplitBlocks` works on scalars with Go's `unicode.IsSpace` set, so it matches `calibration.SplitBlocks`. `fixtures/learning/blocks_unicode.json` holds 12 cases whose outputs came from running that Go function; it is a separate file so the L1a lane's `blocks.json` is not edited.
- Left as is: `looksLikeRef` turns a one-word title such as "Note:important" into the kind's label (noted at the function). Some tests still wait with short sleeps and polling.

### Review round 2 repairs (tests in `ProgrammePlayerRepair2Tests.swift`, written first)

- B1 `resumeAt` is cleared by `setDocument`, which `speakText` uses, so a text read after a dry programme starts at its first paragraph.
- S1 the tracker's closed marker holds the unit it closed on and swallows only a later unit of the same segment; a unit at or before it is a seek (previous, rewind, jump back, play at the end) and is reported as a new play (`segmentBegan`, `blockBegan`, `finished`).
- S2 sign-out sets `loading` false. S3 titles lose only bidi controls (U+202A to U+202E, U+2066 to U+2069, U+200E, U+200F, U+061C), U+200B and U+FEFF; ZWJ and ZWNJ stay. The server's title rule (design 3.2: one line, no control character, 200 bytes) is about control characters only, so the phone strips more than the server checks; the two do not disagree, the phone is stricter on what it shows.
- The 8-second first-segment deadline (request and first segment together): after it the article plays and the stretch is let go (the reader is cancelled, nothing is followed). A late programme is not handed over: there is no hand-over in P1 (round 3 removed the one round 2 added). A computer that is slow to wake leaves the listener on the article for that load; the programme comes at the next load.
- Where per-document state lives and dies. `ProgrammePlayer`: `blockIndex` and `resumeAt` are reset by `setDocument` (also reached from `speakText` and the queue); `resumeAt` is also cleared by `start(at:)` and set when the loop ends dry; the queue of texts is cleared by `ChoirApp.signOut` (`clearQueue`). `SegmentTracker`: the open segment and the closed marker are cleared by `documentReplaced()` (from `setDocument`); the marker also by a block of another segment or of no segment, and by a seek (a block at or before the unit it closed on). `ChoirApp`: the load token and `followTask` are replaced by every `loadProgramme` and by `signOut`, which also sets `loading` false.

### Residuals (known, left as they are)

- An `id:` or `retry:` line between `event:` and `data:` flushes the pending event early and the data line dispatches it again (a repeated `done` or `error` is harmless).
- `appendSegment` can see `loop == nil` in the microseconds inside `start(at:)` between clearing the loop and storing the new task; its `start(at: old)` could then skip blocks a just-pressed skip was about to play.
- A stale open (an older load) is not cancelled: it runs to its first segment or the deadline and may fetch the article for nothing; its result is dropped by the load token.
- The slow relaunch (round 1 drive): a temporary probe (not committed) timed the path in the simulator on two launches, one fresh and one with kept storage, in the demo mode of the time: `refresh` to `loadProgramme` returned in 0.04 s, and the request plus first segment took 0.01 s. So the stretch path itself is not slow, and the more than 9 seconds seen earlier did not reproduce; the Mac's load average was in the tens to the nineties then, which is the only explanation left (not proven). Real latency against a computer is unmeasured until L1 deploys the route; a computer slower than the 8-second deadline leaves the listener on the article for that load.
- The deadline covers the request and the first segment, not the article fetch after it. Some tests wait with short sleeps and polling.

## Player controls: speed to 3x, next and previous story, autoplay, the DJ's candidates (ways it can fail, written before the code)

Owner QA round 1, 2026-11-15. Tests: `PlayerControlsTests.swift` (the numbers are in the test names).

### Speed (0.5x to 3x, a slider)
1. A speed outside the range, or NaN, reaches the audio: it clamps to 0.5...3 in the one place (`PlaybackSpeed.clamp`) that the tracker, the player and the audio all call; NaN is 1.
2. The slider sends a value that is not on a step (1.2345): it snaps to 0.05, so the label and the stored value agree.
3. Speed changes mid-sentence: the time-pitch unit sits after the player node, so the position (counted in source frames) does not jump or restart; the fraction never goes backwards.
4. 3x with a voice that cannot go that fast: every voice (the system voice, Kokoro, Pocket, LFM) is rendered to buffers and played through the one time-pitch unit (`LiveAudio.timePitch`, range 1/32 to 32), so the voice's own rate is never used and no voice has a lower ceiling than 3x. The ceiling is the unit's, and the audio's, not the synthesizer's.
5. The chosen speed is forgotten at the next launch: it is saved (`SpeedStore`) and applied at start.
6. A saved speed that is garbage (0, negative, 99, a string) is ignored and the speed is 1.

### Next and previous story
7. Next at the last story does nothing and does not wrap; previous at the first story restarts it.
8. Previous within the first 3 seconds of a story goes to the previous story; later it restarts the current one.
9. The current story is not in the list (opened from search, or a text from mail): the buttons do not guess; a list is only taken for a story that is in it.
10. The list is the paper's article stack order when the story was started from the paper; the front list when no stack was seen; a programme (the DJ) skips by segment.
11. Lock-screen next and previous do the same as the buttons.

### Autoplay
12. A story ends and the next in the list starts by itself; no setting.
13. The reader paused: nothing starts. The reader is talking to the radio (a turn is open): nothing starts over them; it starts when the turn is over, once.
14. The next story fails to load: skip to the one after it, once, and say so; if that fails too, stop and say so.
15. Nothing was played (the voice failed for every block): the story did not "finish"; autoplay does not run through the whole list failing.
16. The last story ends: stop cleanly.
17. A text queued behind the story (mail's "Read it to me") plays before the next story is chosen (the player's queue runs first). Known limit: when the text ends nothing continues the list; a text is not a story of it.
18. The app is in the background: the audio session keeps the app alive; the loop is the same as in the foreground (nothing to build; to check on the phone).

### The DJ's candidates (the programme request)
19. The request carried no `candidates`, so the server's prompt said "write only ident and bridge segments": the listener got station breaks, never a story. The request now carries the front list, ordered by the paper's weight, with the text of the first ten.
20. A candidate the server would refuse (no dek, a route with a space, @ or #, a title over 300 bytes, text over 8 KB, a 41st item) is shaped or dropped on the phone so one bad story does not make the whole request 400.
21. The text of a story cannot be fetched: that candidate goes without text; the request still goes.
22. No candidates at all (the front list did not load): no request is made; the front page's story plays, as before.
23. A stretch ends (the stream is done and the last segment played): one more request with cause `extend`, with the routes already told left out; nothing left to tell: stop.
24. "Start the radio" after onboarding (owner, 2026-11-15) spoke "there's nothing to report now, so I'm just going to let the music play": the request had no candidates, the server's prompt then says "write only ident and bridge segments", and the model wrote a station break. Fixed at the request (19) and by holding breaks until the first story, update or headlines arrives; a stretch of only breaks is not a programme and the paper's story plays instead.
25. Play pressed before any programme was loaded (sign-in just done, the launch load slow or failed) played the Lab's fixture edition ("written only to test the radio"): Play and Start the radio now wait for a load in flight, or load, and never play the fixture or an empty document.
26. No network and nothing cached: the words are "I can't reach the paper right now", never an announcement that there is nothing to report.

## Acknowledgement timing (`ackTiming`): ways it can fail (written before the code)

`LabConfiguration.ackTiming` is `immediate` (default) or `afterSystem1`. Measured on staging, the acknowledgement
came 3.0 s after the turn ended: transcription ~0.8 s, the System 1 stage ~1.4 s (the line was chosen from its
answers), first audio ~1.6 s. With `immediate`, the holder's first line starts as soon as the turn is the listener's,
in parallel with System 1 and the desk; System 1's answers select only what follows.

- **Ack spoken for a turn System 1 rejects.** The listener said something that needs nothing ("asks_for_something":
  "no"), and the radio says "Got it." anyway. The engine must be able to stop the acknowledgement: the holder is
  cancelled (no further lines), the desk is not engaged, the programme carries on, and the turn record says so
  (`ackCancelled`, with whether the ack was already audible).
- **Double speech when the desk answers fast.** The desk revision arrives while the immediate ack or its follow-up
  line is still playing. The revision stops the holding lines (as today), the line in flight finishes, and the
  revised programme starts after it, never over it. One voice at a time.
- **Ack cut mid-word.** Cancelling on a System 1 verdict must not truncate audible speech: if the first audio has
  started, the line in flight finishes and no further line starts; only an ack that has not started is cancelled
  outright.
- **System 1 never answers.** Deadline reached: the immediate ack has already played; the follow-up is the generic
  holding line, and the record carries the System 1 error as today.
- **Replay differs from live.** Both timings are `LabConfiguration` values, so the same kept turn replays under
  either and the two records can be compared.

## What the listener does (first-run path)

The app is called Autoradio and opens to the radio. No stage names, ids or jargon on this path; the Lab and Trace live behind the last tab, "Lab".

1. Opens the app. Not signed in: a plain screen, "Sign in to hear the paper and talk to your computer", and one button, Sign in.
2. Taps Sign in. The browser opens the approval page (device flow); the app shows the short code in large type and "Approve this sign-in in your browser". If the browser did not open, an "Open the approval page" button. The code and a Cancel are the only other controls.
3. Approves in the browser (passkey) and comes back. The app has stored the key (Keychain), found the listener's computer, and shows the Player.
4. The Player has loaded the newest story from choir.news. It shows the story title, the passage being read, "3 of 12", a large Play button, and below it a list of stories.
5. Taps Play. The first tap starts audio (the microphone permission prompt appears here, once). The passage is read aloud, block by block.
6. Taps a story in the list: it loads and plays from its start.
7. Speaks (voice is off until the Voice control is tapped; there is no press-and-hold); the programme dips then pauses; the screen shows what the radio heard. On release the radio says it is on it, then the revised text from the listener's computer is read from the point that changed.
8. If anything fails, one plain sentence appears on the Player ("Could not reach choir.news. Check your connection.", "Your sign-in is no longer accepted. Please sign in again.", "Your computer is busy right now."). Nothing falls back to a stub silently. The details are in the Lab tab, Trace.
9. Sign out is in the Lab tab, under Account. It removes the key from the phone and asks the server to revoke it.
10. The Lab tab (stages, parameters, configurations, kept turns, replay, Trace) is the old lab, unchanged. The default configuration there is named "listener" (choir-front, choir-system1, choir-desk, the listening models chosen from the model list (Silero, Parakeet EOU live words and Fluid Speech Mini for the transcript once they are on the phone; the level detector and Apple's recogniser until then), the saved voice, button to talk); the stubs and the fixture programme stay selectable.

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

**What the acknowledgement waits on** (from the engine code and the figures in the first staging run; I have not seen the records themselves, so the split of the 2.7 to 4.4 s is partly inferred). `TurnEngine.runPipeline` starts the desk at once but does not start the acknowledgement until the System 1 stage has returned or failed (`withDeadline(system1Deadline)`), because the acknowledgement line is chosen from its answers. Before the turn: final transcription (Fluid Speech Mini once the listening models are on the phone, Apple's recogniser until then, up to `stageDeadline`). So ack = transcript final + System 1 (1.7 to 2.2 s in those runs, of which discovery was most) + the Voice stage's first audio (`voice.firstAudio`, on-device speech synthesis, the largest variable in the mixed turn at 4.4 s). Measured floor with no network at all (fixed System 1, stub desk, `proof-replay.sh` today): ack 2.2 to 2.3 s, so about 2.2 s of every ack is transcription plus first synthesized audio before any Choir call is added; the staging figures are that plus the System 1 stage. With discovery removed the System 1 term should fall to its request time (about 0.6 s for a failure). Next lever, not done here (engine behaviour): start the acknowledgement without waiting for System 1 and let the answers choose only the holding lines.

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
## On-device models: failure lists (written before the code)

Applies to every model stage in `AutoradioModels`. Per stage, then what the code does about it.

- Download interrupted or no network: FluidAudio's downloader and `ModelStore.ensure` resume or retry on the next ask; a stage whose model is missing throws, the engine records `error`, the turn still ends and the radio resumes (engine rule 3). `ensure` pins the llama GGUF by SHA-256 and refuses a mismatch.
- Model too big for 6 GB: nothing is bundled, loads are measured (`ModelLoadReport`: bytes fetched, load ms, resident MB before/after, MB after unload) and `ModelStore.unloadAll(except:)` runs before LFM3-Audio loads. If the load throws or the process is killed for memory, the stage stays registered and the error is the record.
- First-load Core ML compile time (LFM3 builds a multi-GB cache; Parakeet and Kokoro compile on first use): the first load is reported separately from the warm one; stage `begin()`/`speak()` are not on the hot path until the model is loaded (`ModelSlot` loads once and shares it).
- Memory pressure and thermal throttling: `labState` already carries resident MB and thermal state; the proof prints MB after unload to show whether Core ML actually gave memory back.
- Wrong sample rate: every model reads 16 kHz mono; `samples16k` resamples frames that are not, voices emit 24 kHz frames labelled as such.
- Two stages want the same model (EOU 160/320/1280 chunk sizes): one `ModelSlot` per manager, shared across stage instances.
- Silero hop is 256 ms: bounds events are quantised to it; documented, not hidden.
- llama.cpp: the option letter must be one token (checked); context is cleared per question (no cross-question leakage); the logits are read at the final prompt position only.
- LFM3-Audio does not fit beside other models: a stage that cannot load reports the exact error; the next step is recorded in the table.

## Third-party dependencies in the Exact app (settled by reading the pinned Exact source and proving it)

Question: how does a third-party Swift/Core ML library get into the Exact app, given an Exact package cannot depend on a local SwiftPM library?

Findings, from `mobile/.exact-src` at `76cd50f9e`:

- An Exact package (`exact.json` `sources.swift`) is compiled by `swiftc` alone (`host/apple/build.mjs` `swiftModuleArgs`): only its listed files, with `-I <packages dir>`. It has no `-F`, no `-I` for a library, no SwiftPM. So a package cannot `import` a third-party module. (A package may declare `native.library` for a Rust-linked dynamic library; that is not a Swift route.)
- The app's own module (`AutoradioLab/modules/apple/*.swift`) is compiled with every `modules/apple/*.xcframework` linked in (`frameworkSlice`): a static library slice gives `-I Headers -L dir -l<name>`, a framework slice gives `-F -framework`. `import FluidAudio` works there. The app module imports the package modules (`-I`), so it can use the kit.
- Exact never embeds a dynamic framework into the app: only static slices are usable (the code says so: "a dynamic library is not linked into the module"). The official llama.cpp release is a dynamic framework with no simulator slice, so it cannot be used as is.
- No SwiftPM dependency, CocoaPod or remote URL is read for an app or package. So: **third-party Swift code reaches the app as a static `.xcframework` in `AutoradioLab/modules/apple/`**, built by `scripts/build-xcframeworks.sh` from a pinned tag (`mobile/.deps`, gitignored), with the Swift modules placed in the slice's `Headers` directory.

How it is wired here:

1. `AutoradioKit` stays dependency-free (it is copied into the package). The models are a second SwiftPM target, `AutoradioModels`, which depends on `AutoradioKit` and on FluidAudio (`exact: 0.18.3`, trait `TTS`) and on the llama.cpp release binary (macOS only). The Mac commands use it through SwiftPM.
2. `scripts/sync-exact-package.sh` copies `Sources/AutoradioModels/*.swift` to `AutoradioLab/modules/apple/Models_*.swift` with `import AutoradioKit` rewritten to `import ExactPkg_autoradio`, copies the xcframeworks beside them, and `ModelsEntry.swift` (tracked) gives the app module its `ExactModule`.
3. The package module cannot import the app module, so `ModelHub.register` is reached by name: the app module defines `@objc(AutoradioModelsRegistrar)`, and `StageRegistry.builtIn()` looks it up with `NSClassFromString` (`ExternalModels.swift`). No registrar class, no models; the app still builds and runs without them.
4. Built with: FluidAudio v0.18.3 by `swift build --triple arm64-apple-ios18.0[-simulator] --sdk <iphone sdk> --disable-default-traits --traits TTS` (about 10 minutes per slice on a loaded Mac) giving `libFluidAudio.a` (14 MB) and `FluidAudio.swiftmodule`; llama.cpp b12259 by its own `build-xcframework.sh ios-sim ios-device`, static libraries merged with `libtool`. `xcodebuild -create-xcframework -headers` refuses a `.swiftmodule` without a `.swiftinterface`, so the module directories are copied into each slice's `Headers` afterwards.

Proof: see "Proof: models" below (simulator build with FluidAudio linked, registrar found, stages listed in the app).

### For EXACT.md (`docs/reference/exact/native-modules.md`, "Packaging our Swift package")

Replace the sentence "whether a package may list files from outside its own directory, or depend on a SwiftPM library, is not established" with: a package may not depend on a SwiftPM library or import any third-party module (it is compiled by `swiftc` with only its own files and `-I`); third-party Swift/Core ML code goes in the app's own module: static `*.xcframework` files in `modules/apple/` are linked into `libexact_modules.dylib` (`host/apple/build.mjs` `frameworkSlice`: static `lib*.a` slice with `-I HeadersPath -L -l`, or a static framework with `-F -framework`; dynamic frameworks are not embedded by the build; that they would then fail to load at launch is inferred, unrun). The app module can import package modules (`-I` of the packages directory); a package reaches app-module code only by runtime lookup (`NSClassFromString`). A `.swiftmodule` placed in a slice's `Headers` is found by `-I`.

## On-device model stages: what is registered and what was measured

`autoradio-replay --list-stages` lists 28 stages (the new ones below). Replay any kept turn with a model stage swapped in:
`autoradio-replay --turn mobile/fixtures/turns/plain --config mobile/fixtures/configs/open.json --set transcriber=parakeet-eou --set speechBounds=silero-vad --set voice=kokoro`.
`mobile/scripts/proof-models.sh` replays the fixture through each stage and prints one line per stage (`LFM3=1` adds the 2.9 GB LFM3-Audio pair; `STAGES=a,b` narrows). Nothing is bundled: each model downloads on first use into the app's Application Support (FluidAudio's cache, or `ModelStore` for the GGUF) and `ModelStore.unload` drops it.

All figures: fixture turn `plain` (3.6 s of speech). "first" is the first load in a process, including the download and Core ML compile where the model was not yet on disk; "warm" is a reload from disk. Memory is process resident size right after load and after `unload` (Core ML maps lazily, so "loaded" under-reports until the first prediction). Mac = iMac21,1 (M1), shared with other builds, so its timings are noisy; the cold runs below were taken under load average above 100.

| Stage id (kind) | Model, size, licence | Cold first load (download) | Warm load | Resident loaded / after unload | Speed | Measured on | Result on the fixture |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `silero-vad` (speechBounds) | Silero VAD v6.2.1 Core ML, about 1 MB, MIT | 8.6 s (1 MB) | 42-70 ms | 27-44 MB / same | 0.2-37 ms per 256 ms hop (cold-loaded machine) | Mac; phone (before the owner's ruling): load 93 ms, 0.6 ms per hop, 106 MB | speech start at 0.9 s (hops of 256 ms) |
| `parakeet-eou` (transcriber, streaming) | Parakeet EOU 120M, chunk 320 ms, 219 MB, NVIDIA Open Model License | 28 s (219 MB) | 168 ms | 247 / 86 MB | RTF 0.05 (Mac); phone: 0.035 | Mac; phone (before the ruling): first load 34 s, 149 MB | "please tell me more about the harbour and the park" |
| `parakeet-tdt` (transcriber, batch) | Parakeet TDT 0.6B v2 (English), about 480 MB, CC-BY-4.0 | not captured (downloaded in an earlier run) | 263 ms | 96 / 114 MB | RTF 0.02-0.03 | Mac | "Please tell me more about the harbour and the park" |
| `llama-qwen3-0.6b` (system1) | Qwen3-0.6B Q8_0 GGUF via llama.cpp b12259, 639 MB, Apache-2.0 (llama.cpp MIT) | 217 s (625 MB) | 0.8-2.4 s | 1050 / 202 MB | 65-71 ms per question, option letter probability from logits | Mac (Metal) | asks_for_something=yes p=0.98; carry_on=yes p=0.61 |
| `kokoro` (voice) | Kokoro 82M ANE v3 via FluidAudio, Apache-2.0 | 104 s | 4.5-22 s | 722 / 520 MB | first audio 253 ms (first sentence), RTF 0.07 | Mac | 6.3 s of audio at 24 kHz |
| `pocket-tts` (voice) | Kyutai PocketTTS 100M via FluidAudio, CC-BY-4.0 | 44 s | 0.4-1.2 s | 1027 / 413 MB | first frame 120-239 ms, RTF 0.5 (streams frames) | Mac | 5.6 s of audio at 24 kHz |
| `lfm3-audio-live`, `lfm3-holder`, `lfm3-audio-tts` | LFM3-Audio-2B Core ML via `LFM3AudioManager`, about 2.9 GB, LFM Open License (read it: commercial use limited by revenue) | 739 s (download of about 2.9 GB plus first Core ML compile; the cache delta was not captured because the library's cache folder is named differently) | not run | 238 MB loaded / 78 after unload (measured before the first prediction; vendor figure 1.9 GB peak) | live ASR RTF 4.97 (17.6 s for 3.6 s of audio, so slower than real time on this loaded M1; vendor: RTFx 10 on an M5 Pro); TTS first audio 2.4 s, RTF 1.65 | Mac only | live ASR: "Please tell me more about the harbour..." (correct); TTS: 5.9 s of audio at 24 kHz |

Library licences: FluidAudio Apache-2.0; llama.cpp MIT. Models' licences are read from the Hugging Face cards (`cardData.license`), not from memory. The LFM3 holder (`lfm3-holder`) and the fixed-lines holder were removed in round 3: the holder is now `local-ack`, the phone's own Qwen3 model thinking aloud (see docs/reference/ON-DEVICE-MODELS.md).

Findings worth acting on:
- FluidAudio's downloader cancels a file as "stalled" when under 1 MB arrives in 120 s, even for a tiny file (`metadata.json`), then retries with back-off. On the phone this made the first `parakeet-eou` load look hung for more than 20 minutes; the second launch finished in 34 s. A first-run download therefore needs a visible progress state, not a spinner.
- `parakeet-eou` is the smallest streaming model with partials; 120M parameters, 219 MB, RTF 0.035 on the A16. TDT 0.6B is more accurate at the end of a turn and 480 MB. For 6 GB, EOU first, TDT second; both fit beside Qwen3-0.6B (1 GB resident on the Mac).
- The Silero events are quantised to 256 ms hops.

## To measure on the phone (owner ruling: nothing is installed or launched on the owner's phone by builders)

Two phone figures (Silero and Parakeet EOU) were taken before that ruling and are labelled above. These are the runs to do, by hand, when the owner agrees. Phone UDID `00008120-00114DE80A90C01E`.

```sh
mobile/scripts/build-lab.sh ios --device --phone 00008120-00114DE80A90C01E      # installs the app with the model stages linked
cd mobile
D="scripts/xe.sh /usr/bin/xcrun devicectl device"
# the proof reads Documents/proof/turn/{mic.caf,context.json} in the app container
$D copy to --device 00008120-00114DE80A90C01E --domain-type appDataContainer --domain-identifier news.choir.autoradio.lab --source fixtures/turns/plain/mic.caf --destination Documents/proof/turn/mic.caf
$D copy to --device 00008120-00114DE80A90C01E --domain-type appDataContainer --domain-identifier news.choir.autoradio.lab --source fixtures/turns/plain/context.json --destination Documents/proof/turn/context.json
# one launch per group (the first launch of a model downloads it; Wi-Fi, screen on). Stderr lines start "autoradio proof:".
$D process launch --device 00008120-00114DE80A90C01E --terminate-existing --console news.choir.autoradio.lab -- -ModelProofStages silero-vad,parakeet-eou,parakeet-tdt
$D process launch --device 00008120-00114DE80A90C01E --terminate-existing --console news.choir.autoradio.lab -- -ModelProofStages kokoro,pocket-tts,llama-qwen3-0.6b
$D process launch --device 00008120-00114DE80A90C01E --terminate-existing --console news.choir.autoradio.lab -- -ModelProofStages lfm3-audio-live,lfm3-audio-tts   # 2.9 GB download; first load builds a multi-GB Core ML cache; keep 30 GB free
$D copy from --device 00008120-00114DE80A90C01E --domain-type appDataContainer --domain-identifier news.choir.autoradio.lab --source Documents/proof/results.json --destination /tmp/phone-results.json
```

Also record, on the phone: thermal state after the LFM3 run (`labState.thermal` shows in the Player header), and whether iOS kills the app during LFM3's first load (jetsam), which is the figure that settles whether it fits beside the other models on 6 GB.

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

### Desk contract

The interface-only `ChoirDesk` ("choir") and the bearer-token `RemoteSystem1` and `RemoteProgramme` stand-ins, with their Keychain helper, are deleted (2026-11-22): the listener's turn goes through the Autoradio desk stage (`Choir/AutoradioDesk.swift`, `POST /api/autoradio/turns`). The lab stand-ins `fixed`, `stub-echo` and `fixture` stay for the Mac replay proof configs (`mobile/fixtures/configs`).

## Running

```sh
mobile/scripts/proof-replay.sh                       # Mac end-to-end proof; exits non-zero on failure
mobile/AutoradioKit/.build/release/autoradio-replay --list-stages
mobile/AutoradioKit/.build/release/autoradio-replay --turn mobile/fixtures/turns/plain --config mobile/fixtures/configs/open.json
python3 -I mobile/scripts/make-fixtures.py           # regenerate fixtures (macOS: say, afconvert)
```

All Xcode/Swift commands run outside the Nix shell through `mobile/scripts/xe.sh`.

## The host app: Exact

`AutoradioLab/` is an Exact app (Contract pages: the radio, paper, reader, notebook, mail, files, podcasts, settings; the Lab and Trace pages were deleted 2026-11-19; no SwiftUI, no UIKit views of ours). AutoradioKit is attached as the Exact package `@choir/autoradio` (`exact-package/`): `scripts/sync-exact-package.sh` copies the kit sources in and writes `exact-package/apple/exact.json`; `scripts/gen-exact-interface.py` writes the interface.

Pinned Exact: `expo/exact` at `f5334750386192245ee76add13a67ab51fb4e2fd` (moved from de212e671 on 2026-11-20, by reading; the first build at it is the lead's: `docs/reference/exact/build-and-ship.md`, "Moving the pin"), checked out at `mobile/.exact-src` (gitignored), Bun 1.4.2 at `~/.bun-1.4.2`. Commands: `mobile/scripts/exact.sh ios` (simulator), `mobile/scripts/exact.sh ios --device --phone 00008120-00114DE80A90C01E`.

Signing: the free provisioning team is `QSGU6VRJL6` ("Yusef Nathanson (Personal Team)"); `N9HKM55LWY` is only the certificate's person id. Exact needs an installed profile for the app's id; `scripts/make-profile.sh` obtains a development one with a stub Xcode project and `-allowProvisioningUpdates` (`TEAM=S47GERGJZ3` for the paid team).

Paid team, 2026-11-23: the owner renewed the developer account and signed in to Xcode. The paid team is `S47GERGJZ3`; a wildcard development profile for it (to 2027-11-23) covers the owner's phone. The second development app ("Autoradio 1" then, `news.choir.autoradio.lab.one`) was moved to it that evening: the same code (`dcc56f2cd`), built in the build tree with `EXACT_IDENTITY=<the new development identity's SHA-1>` and `EXACT_PROFILE=<the wildcard profile's path>` (`exact.sh` now passes both through `env -i`). iOS refuses to install over a copy signed by another team (`MismatchedApplicationIdentifierEntitlement`), so that app alone was removed and installed again; it starts signed out with no models. The first app is still on the free team (`QSGU6VRJL6`, profile to 2026-11-25) and was not touched. Without those two variables Exact may pick the free team's profile while it is still on disk. There is no Apple Distribution identity yet, so nothing can be uploaded until the owner makes one (below).

## The apps: production, and the two development apps on the phone

One `AutoradioLab/app.json`, three identities (owner, 2026-11-23: the name is "Autoradio 1"; ids are in the `news.choir` family). **The production id is `app.id` in `app.json`, one value in one place** (also `id`, which Exact wants equal); everything else reads it through `scripts/app-identity.sh`. If the owner registered another id, change `app.id`, `id`, the default in `app.ts` (`SIGNIN_SCHEME ?? '...'`) and the lab variants' `env.SIGNIN_SCHEME`, then the server's list (below): the bun test `store-build.test.ts` fails until they agree.

| Choice | Id | Name | Sign-in scheme |
| --- | --- | --- | --- |
| none (production) | `news.choir.autoradio` | Autoradio 1 | same as the id |
| `AR_VARIANT=lab` (or `AR_APP_ID=news.choir.autoradio.lab`) | `news.choir.autoradio.lab` | Autoradio | same |
| `AR_VARIANT=lab-one` (or `AR_APP_ID=news.choir.autoradio.lab.one`) | `news.choir.autoradio.lab.one` | Autoradio 1 Dev | same |

The lab apps are Exact **variants** (`variants` in `app.json`: id = `app.id` + `idSuffix`, its own name, `env.SIGNIN_SCHEME`; Exact's reference, "Install a development build beside the release one (variants)"). The lab ids are exactly the ones on the owner's phone, so they stay installable side by side with each other and with the production app. `lab-one` was named "Autoradio 1" until the production app took that name: Exact refuses two apps of one installed name (`scripts/variants.mjs`), so its home-screen name becomes "Autoradio 1 Dev" at its next install (the id, and so its data, is unchanged). `AR_VARIANT=one` is retired and refused (the message lists what is declared). Nothing in the app branches on the identity, and the tracked `app.json` is never rewritten by a build. `AR_APP_NAME` is accepted only as a check. `app-identity.sh id|name|variant|version` reads and checks the choice, `exact.sh` passes it to Exact as `EXACT_VARIANT` and the id as `SIGNIN_SCHEME`, and `proof-common.sh`, `sim-launch.sh` and `make-profile.sh` take their bundle id from it.

**No choice is now the production id, not the lab app.** A build for a phone (`--device`, not `--archive`) must name its development app (`build-lab.sh` refuses otherwise): the production id reaches phones through TestFlight. Simulator builds and the proofs with no choice run the production id on the simulator. The lead's queue scripts outside the repo (`~/.local/state/choir-buildq/device-install.sh`, `side-by-side.sh`, `run-one.sh`) must set `AR_VARIANT=lab` (device-install: the first app) or `AR_VARIANT=lab-one` before they build for the phone.

The lead, through the Mac build queue, in the build tree, the phone connected, unlocked, Developer Mode on:

```sh
# (a) once: a development profile for a new id (the stub installs it; its compile may fail afterwards, that is fine)
AR_VARIANT=lab-one mobile/scripts/make-profile.sh
# (b) build, sign and install beside the other apps (does not touch them)
AR_VARIANT=lab-one mobile/scripts/build-lab.sh ios --device --phone 00008120-00114DE80A90C01E
# optional launch
AR_VARIANT=lab-one mobile/scripts/build-lab.sh ios --device --run --phone 00008120-00114DE80A90C01E
```

Every `devicectl` line that names `--domain-identifier news.choir.autoradio.lab` reaches the first app; use `news.choir.autoradio.lab.one` for the second and `news.choir.autoradio` for the production app.

Shares with the other apps: nothing, by the entitlements Exact writes (read from `host/apple/build.mjs` at the pin, not yet from a built app). The signed `application-identifier` is `<team>.<id>`; Exact adds `keychain-access-groups` and an App Group only for an app with widgets or activities, and this app has neither. Our Keychain code sets no access group, so an item goes in the app's default group, `<team>.<id>`, and the fixed service strings (`news.choir.autoradio.choir`, `.mailseal`, `news.choir.autoradio`) are keys inside each app's own group; they are the same strings in all three apps and need no change. The sandbox containers differ, so preferences, Documents and Application Support differ. The on-device models live in the app container (`Application Support/AutoradioLab/models` and FluidAudio's `Application Support/FluidAudio/Models`), so each app downloads its own (the small language model is 0.64 GB, the embedder 0.31 GB, the optional LFM3 pair more); Exact's build output for each id is also a separate copy on the Mac. Background modes, the audio session and the `AVAudioSession` are per app; two apps playing at once compete for the audio session like any two apps. No URL scheme, universal link, notification category or background task id is declared except the sign-in callback scheme (the app's id, a grant). **The server must list every id:** `internal/auth/device_login.go:53` `DeviceReturnSchemes` holds the two lab ids and must gain `news.choir.autoradio` (red: auth; the Go test `TestDeviceReturnSchemesAreTheAppsInAppJSON` reads `app.json` and fails until it does). The server learns no other bundle id.

Free provisioning team (`QSGU6VRJL6`): a profile lasts 7 days (lane experience, `docs/reference/exact/build-and-ship.md`; not checked against Apple's page here), after which the app stops opening until it is built and installed again; the second id needs its own profile, so it is a second app id on the team. Apple's limits on app ids per week and on apps installed per device: not checked.

Remove one: delete it from the phone (long press, Remove App), or `xcrun devicectl device uninstall app --device 00008120-00114DE80A90C01E news.choir.autoradio.lab.one`. Removing it deletes only its container and Keychain items; the others are untouched.

## The store build: archive and upload (TestFlight)

Station P1 of `docs/definitions/choir-shipping-the-app-2026-11-23.md`. Two scripts, both the lead's, neither run by a builder (`archive-store.sh` needs the build tree and the build lock; `upload-store.sh` is an owner act until the owner delegates it).

**What the owner makes first (Xcode signed in to the paid team `S47GERGJZ3`):** an Apple Distribution certificate (Settings, Accounts, Manage Certificates; its key stays in the login keychain); the explicit App ID `news.choir.autoradio` (done 2026-11-23); an **App Store** provisioning profile for it, downloaded to a file; an App Store Connect API key (Users and Access, Integrations; its `.p8` kept outside the repo, mode 600). Agents are given the certificate's SHA-1 and file paths, never key material.

```sh
# the lead, in the build tree, the build lock held (check-store-signing.py reads only what these two name)
cd /Users/wiz/Dev/go-choir/.lane/trees/qa-mac && git fetch -q && git checkout --detach origin/lane/store-build   # the queue's clean-tree rule applies
EXACT_IDENTITY=<SHA-1 of the Apple Distribution identity> EXACT_PROFILE=<path of the App Store profile> \
  /usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock mobile/scripts/archive-store.sh
# prints: ipa, bundle id, version, build number, ITSAppUsesNonExemptEncryption (read back from the .ipa)
ASC_KEY_ID=<key id> ASC_ISSUER_ID=<issuer id> ASC_KEY_FILE=<path of AuthKey.p8> mobile/scripts/upload-store.sh <the ipa>
```

**Version and build number.** `CFBundleShortVersionString` is `app.json`'s `"version"` ("1.0"), given to Exact as `EXACT_VERSION`; Exact has no other source on iOS. `CFBundleVersion` is `EXACT_BUILD_NUMBER`, set by `archive-store.sh` to whole minutes since 2026-01-01 00:00 UTC (a number of at most 9 digits that rises with every run, so uploading the same commit again is a higher number; two runs in one minute cannot happen under the build lock). `AR_BUILD_NUMBER=<n>` overrides it, for example with App Store Connect's own next number. App Store Connect wants a new build number for every upload of a version; raise `"version"` for a new store version.

**What the archive is.** `AR_BUILD_KIND=store` (only `archive-store.sh` sets it; `build-lab.sh` refuses an archive without it and the kind without an archive) then Exact's `bun exact.mjs ios --device --archive <ipa> --json`: production trust (`deploy.store.ios` is `"0"`, so no update receipt), Swift's release build, signed by `EXACT_IDENTITY` and `EXACT_PROFILE`, no phone. `EXACT_NON_EXEMPT_ENCRYPTION=false` writes `ITSAppUsesNonExemptEncryption = NO` (the app uses the system's HTTPS, Keychain and CryptoKit only). `EXACT_DEV_MENU=0` stamps Exact's four-finger developer menu off (it is on in release builds otherwise). The script then reads the `.ipa` back and fails unless the bundle id, version, build number, export answer, menu stamp and `ExactBuildKind = archive` are as asked, no `ExactDevelopment*` key is present, and a provisioning profile is embedded.

**Developer-only surfaces, and how each is excluded** (a build kind, like debug symbols: no setting the app reads):

| Surface | In a development build | In the store build |
| --- | --- | --- |
| Exact's iOS developer menu and reload (four-finger tap or hold) | on | off: `EXACT_DEV_MENU=0` at build, stamped `ExactDevMenu = false` and read back |
| Exact's development client keys, inspect mode, development trust | `ExactDevelopment*` plist keys, `EXACT_INSPECT` | not written: `--archive` bakes production trust (Exact), checked on read-back |
| Launch-argument measurements (`-ModelProofStages`, `-EmbedTrial 1`: write into Documents) and the embedder's 24-story fixture | staged into the app module | `sync-exact-package.sh` with `AR_BUILD_KIND=store` does not stage `EmbedDeviceTrial.swift` and `EmbedTrialFixture.swift` and drops the two calls from the staged `ModelHub.swift` (the uncalled `ModelProof.runFromLaunchArguments` definition remains in `ModelProof.swift`, which the product's `ModelProof.frames` needs) |
| Lab, Trace, Components pages | none exist (deleted 2026-11-19); `lab:` addresses in `pages.ts` are titles only, `PageHost` has no `lab` kind and shows the not-yet page | same |
| `motion.contract` (motion gallery) | in the tree, imported by nothing | not built |
| Fixtures, demo mode | `typescript.sources.fixtures` is a source directory for tests; demo mode was deleted 2026-11-20 (`demo-leftovers.ts` only cleans what it left) | same |
| Internal hostnames, test accounts | none: `ORIGIN` is `https://choir.news`; no drill account is in the app | same |
| About's Build row (commit, dirty) and Server build | shown | shown (a tester's bug report names the commit); the commit is `build-info.gen.ts`, written by `gen-build-info.sh` |

**What a tester sees, tab by tab** (the launcher's nine places and its More list; read from the contracts, not seen on a phone): Autoradio (the radio page: Play, queue, speed), My notebook, Paper (the front page, stories, the reader), Textures, Files, Mail, Imported documents, Podcasts, Settings (edition, appearance, account and devices, Computer, voice setup, voice and models, what the paper has learned, Activity, About). Signed out, the paper and the radio work; Mail, Files, Textures and the notebook need an account. Version 1's scope (Q2: radio, paper, reader, voice and podcasts; mail, files and takes later) is the owner's: every page above is in the build until the owner says to remove a place. Residual: `NotYetPage` ("... is not built yet") is reachable only for an address kind `PageHost` does not know.

**Info.plist and the privacy manifest.** Microphone, speech and background-audio strings are `AutoradioLab/strings/en.json` (plain wording since 2026-11-23); `UIBackgroundModes = [audio]` is `host.ios.backgroundModes`; no other permission string exists (no camera, photos, location, contacts, Bluetooth, Face ID, tracking). `PrivacyInfo.xcprivacy` declares disk space (E174.1), file timestamps (C617.1), boot time (35F9.1) and user defaults (CA92.1), and Exact merges its own and scans every linked binary, stopping on an undeclared required-reason API; a grep of `AutoradioKit` and the app module finds exactly those four categories. It declares no `NSPrivacyCollectedDataTypes`: that waits for the owner's privacy label (mission station P4). Whether FluidAudio's and llama.cpp's static code use another category: unknown until the archive's scan runs.

## Audio states: ways it can fail (written before the code)

The adapter holds one standing capture claim; the kit asks for `listening` or `talking` through `AudioSessionOwner`.
Policy (`LabConfiguration.audioPolicy`): `switchOnAddress` (default: listening, talking while a turn is open), `alwaysTalking`
(voice processing the whole run, the Mike-plugin shape), `alwaysListening` (phone mic, no voice processing, never switches).

1. The first claim's mode wins in Exact (`AudioSession.swift:238-262`): claiming `voiceChat` while the `default` claim is held changes nothing and the
   log still says "claimed". Rule: a switch is release, then claim, in that order, and the adapter reads `AVAudioSession.mode` afterwards and emits
   it; a mismatch is an `error` event.
2. An empty options list silently means `defaultToSpeaker + allowBluetoothHFP` (`NativeHatches.swift:239-243`): every claim passes an explicit list.
3. `playAndRecord` without `defaultToSpeaker` plays the programme from the earpiece receiver: listening carries `defaultToSpeaker` too.
4. The category change stops the running `AVAudioEngine`: after every switch the engine is restarted, taps reinstalled (voice processing changes the
   input and mixer formats) and the programme node resumed. A programme block that was mid-buffer continues from the frames already rendered, not from
   the start and not skipped.
5. `setVoiceProcessingEnabled` throws or is unsupported (simulator, some routes): `error` event, engine still starts without it.
6. A release leaves no claim for a moment: the session falls to `.ambient` and is deactivated (`restoreAfterCaptures`). The engine must be stopped before
   the release and started after the new claim, never run across the gap.
7. Interruption (a phone call) stops the engine and Exact reactivates the session only when the system recommends resuming: on `interrupted` pause the
   programme and mark the engine down; on `resumed` rebuild and restart; with no `resumed` the app says so in `message` and a tap on Start audio recovers.
8. Route change (AirPods in or out): the engine posts `AVAudioEngineConfigurationChange` and has stopped; restart taps and engine once, debounced, and
   emit the new route. A switch caused by our own claim must not trigger a second restart.
9. Media services reset: every AVAudioEngine and node object is invalid. The engine is rebuilt from scratch; the programme position is kept by block.
10. A switch is slow (20-41 s reported with AirPods on iOS 27.0.1): the claim runs off the main actor's critical path, the turn engine proceeds, a
    watchdog emits `error` at 5 s and the latency is recorded in the turn record (`audioSwitchMs`).
11. Player controls before Start audio: a player node on a stopped engine raises an Objective-C exception. All node calls go through `LiveAudio` and
    do nothing while the engine is down.
12. Under the Exact agent no audio hardware is opened (`context.agent`): the runtime runs a virtual engine, which plays by waiting and takes mic frames
    from a kept turn, so Start audio, Talk and Trace are driven without a device.

## Yielding live: ways it can fail (written before the code)

The `YieldPolicy` is pure; the engine applies it and `ProgrammePlayer.setLevel` drives the nodes (dip = programme node volume 0.25, pause = node pause).

1. Dip changes `programmeNode.volume`, but a restart of the engine (session switch) rebuilds the node's output: the volume is kept in `LiveAudio` and re-applied in `bring`.
2. Pause then `play` replays from the node's queue: after an engine restart the queue is gone, so playback continues from the rendered-frame offset (audio-state item 4).
3. Pause is applied while the user has paused (`userPaused`): resume must not start playing.
4. The yield event and the audible effect disagree (event says dipped, node still at 1.0): the Lab screen shows the node's real volume and audibility (`audio-level`), and the proof reads them while a turn is held.
5. Under the agent there is no node: the virtual engine keeps the same state variables, so the events and the numbers are the same, but nothing proves the speaker. Only the phone does that.

## Background and lock screen: ways it can fail (written before the code)

1. The Info.plist lacks `UIBackgroundModes: audio` or `ExactCapturesInBackground`: the process suspends on lock. Checked in the built plist (below).
2. `playsInBackground` is true only with the `audio` mode and a capture claim with `background: true` (`AudioSession.swift:96`): both claims carry `background: true`, including after a switch.
3. A switch while locked: the release leaves no claim for an instant and the session is deactivated (`restoreAfterCaptures`); iOS can then refuse to reactivate from the background. Unmeasured; on the phone's list.
4. The microphone cannot be opened from the background, so a locked-screen session must already be live: Start audio before locking, never a first claim after.
5. Interruption while locked (a call) with no `resumed`: the programme stays silent. The Lab screen shows engine "down" on unlock.
6. Now Playing and the lock-screen controls belong to Exact's own audio element (`UIBackgroundModes` and the remote command centre are Exact's). Our engine's `MPNowPlayingInfoCenter` and `MPRemoteCommandCenter` calls in `ProgrammePlayer` are not what Exact shows; building that is out of scope here. What it would take: an Exact `audio` element (a hold the owner knows) carrying the programme, or a package-side `MPNowPlayingSession` plus an active remote command target, with the app category `playback` and an activated session; Exact would need to not fight over `MPRemoteCommandCenter`.

## Slim simulators (`scripts/sim-slim.sh`)

A booted stock iOS 27.0 simulator runs about 160 to 180 processes. `simslim` (MobAI-App/simslim, MIT, commit `7e374b3839d49608a681d1f8b9ca0dd83d35189e`, read in full and built with the repo's Go 1.26; it makes no network calls, runs nothing as root, and its only host write for `on` and `off` is the device's launchd disable store, `/private/var/tmp/com.apple.CoreSimulator.SimDevice.<udid>/disabled.plist`) switches off daemons by launchd label. It talks to the simulator only through `xcrun simctl`; it never opens Simulator.app, so Device Hub changes nothing for it. Binary: `~/.local/bin/simslim` (`SIMSLIM=` overrides).

```sh
mobile/scripts/sim-slim.sh autoradio-p2            # slim it: reboots once, offline, about 25 s
mobile/scripts/sim-slim.sh autoradio-p2 --status   # 143/173 managed services off
mobile/scripts/sim-slim.sh autoradio-p2 --off      # back to stock (a clean `simctl erase` also keeps the store: use --off)
```

It refuses `autoradio-lab` unless `--allow-lab` is given. Never run `simslim disk-clean`.

What stays on: category `siri` (speech recognition, `corespeechd`, on-device language and model services, text to speech) and `com.apple.mobileassetd` and `com.apple.accessibility.axassetsd` (speech models and voices arrive as assets). What goes: widgets and wallpaper, search, iCloud and account, store and push, mail and calendar, web sync, family, health and home, photos analysis, news and maps, messaging, connectivity, telemetry, device management, and the rest of "other".

Measured on 2026-11-15 on this Mac (load average 100 to 200 at the time, one simulator, 60 s after boot, `simslim measure`):

| | processes | footprint |
| --- | --- | --- |
| stock, first boot | 178 | 5.82 GB |
| stock, second boot | 162 | not measured |
| slim (siri kept) | 97 | 2.90 GB |
| slim after the app ran, siri kept | 106 | 3.50 GB |
| the same, siri also disabled live | 91 | 2.87 GB |

Proved on a slim simulator: the installed `news.choir.autoradio.lab` launched, its Exact agent socket opened in 2 to 3 s, `tree` returned 45 to 67 nodes, `bar-play` and `bar-mic` claimed the audio session (`PlayAndRecord`, then `VoiceChat`). Not proved: real speech recognition and real text to speech on a slim simulator (the agent runs the virtual engine), the old demo-mode proofs (deleted), background audio. Keep `siri` until those are run. If something breaks, `sim-slim.sh <name> --off`, then report which feature failed; `simslim enable <udid> <category>` turns one category back on without a reboot.

## Driving the app and proving it (`scripts/proof-real.sh`)

```sh
mobile/scripts/setup-exact.sh                  # pinned Exact checkout at mobile/.exact-src, Bun 1.4.2 at ~/.bun-1.4.2 (EXACT_REPO=<local clone> to avoid the network)
export EXACT_SIM=$(mobile/scripts/sim-device.sh)   # a simulator of your own (see below)
mobile/scripts/build-lab.sh ios                # sync the kit into the package, build, install on $EXACT_SIM
mobile/scripts/exact.sh agent ios tree "tap tab-lab" "clock data" tree
mobile/scripts/check-all.sh                    # every check that needs no simulator or native build: generators, strings, wiring, boundary, the Exact loop, the whole Swift package (through the lock)
mobile/scripts/proof-real.sh                   # real data on staging as the drill account; last line PROOF OK or PROOF FAILED at <step>; NOBUILD=1 skips the build
```

**Why "the app never opened its agent socket" (cause, from Exact's source).** Three things, all of them outside our package:

1. *Two drivers on one simulator.* Exact allows one app per bundle id per simulator and every launch uses `simctl launch --terminate-running-process` (`scripts/agent.mjs:529`, `:569`). Another builder's drive of `news.choir.autoradio.lab` on the same simulator kills our running copy: the driver sees "the app hung up (exit code 0)" (the exit is `Agent.exitAfterLanding`, `Agent.swift:140-146`, which runs when the socket closes: it is the other driver's launch, not a crash) or never gets its socket. Fix: `sim-device.sh` creates and boots a simulator of our own; `exact.sh` passes `EXACT_SIM` through its `env -i`.
2. *The socket opens late.* The app binds it only after the first frame is applied and, on the simulator, after the scene is foreground-active (`ExactIOS/main.swift:134-141`, `AgentFrontIOS.swift:29-45`, up to 15 s); the driver gives up after 20 s (`agent.mjs:567-579`). A cold first launch on a machine at load 400 (several builders compiling) misses that; the second launch is warm. `proof-real.sh` retries a refused launch.
3. *A system alert holds the foreground.* The speech recognizer asked for authorization at its first use, which put the iOS permission alert over the app; it stays after the app is killed, and every later launch is then not foreground-active and never opens its socket. `simctl privacy ... grant all` does not cover speech recognition (`simctl privacy` has no speech service). Fix in the kit: while the runtime is virtual (driven by the agent) the transcriber does not ask (`LabEnvironment.promptsAllowed`); restarting the simulator clears a stale alert.

The agent's tree shows `[hidden]` on elements of tabs that are not showing, but they are all in the tree: assertions read text by test id, and the proof orders its checks along the one transcript.

**Virtual audio.** The Exact agent plays nothing and opens no device (`state.sounds.output = "agent"`). Under the agent `labStart` starts `LiveAudio` in virtual mode: the same events, claims (those are real, on the simulator's session), yield levels and playback state, with playback as a timed wait and no `AVAudioEngine`. The mic is whatever is pushed into the bus: the Lab screen's "Inject fixture turn" button presses talk, feeds the bundled kept turn (`FixtureTurn.swift`, from `fixtures/turns/plain`) as 32 ms frames in real time, adds silence, and releases: the whole live engine path runs, and the turn lands in Trace like a spoken one. The same button on a phone injects through the real engine, which is a useful smoke test of the live path with a known utterance.

## Trace export (stale: the Trace page was deleted 2026-11-19; the path below is the old one)

`Export JSON` in Trace writes the turn record to `Documents/AutoradioExports/<id>.json` in the app container, and also copies it to `app:/data/exports/` and offers Exact's `share` hatch (`share(title=, files=[path])`, `Share.swift:1-30`): the system share sheet on a phone; under the agent a hold, never a sheet. Without the sheet, pull it:

```sh
xcrun devicectl device copy from --device 00008120-00114DE80A90C01E --domain-type appDataContainer \
  --domain-identifier news.choir.autoradio.lab --source Documents/AutoradioExports/<id>.json --destination .
# simulator:
cp "$(xcrun simctl get_app_container $EXACT_SIM news.choir.autoradio.lab data)/Documents/AutoradioExports/<id>.json" .
```

## Background and lock screen (verified in the built Info.plist)

`AutoradioLab/.../Autoradio Lab.app/Info.plist` carries `UIBackgroundModes: [audio]` (from `app.json` `host.ios.backgroundModes`), `ExactCapturesInBackground: true` (from the `device.microphone-background` grant in `app.ts`), `ExactAudioSession: playback`, `NSMicrophoneUsageDescription` and `NSSpeechRecognitionUsageDescription` (from `strings/en.json`). Both claims pass `background: true`, so `playsInBackground` (`AudioSession.swift:96`) is true while a claim is held. What is not proved: that the programme keeps playing with the screen locked, and that a listening-to-talking switch while locked is allowed (the release leaves an instant with no claim). Both are on the phone list below.

Now Playing and the lock-screen transport belong to Exact's own audio element. Ours plays through our own `AVAudioEngine`, so the lock screen shows nothing for it, and `ProgrammePlayer`'s `MPNowPlayingInfoCenter`/`MPRemoteCommandCenter` calls are not what Exact displays. To get them: either (a) route the programme through an Exact `audio` element (a hold the owner knows, with Exact's media session), or (b) publish from the package: an `MPNowPlayingInfoCenter` entry plus `MPRemoteCommandCenter` targets, with the session active and the app category `playback`; Exact's owner must be told not to fight it. Not built.

## To measure on the phone

The phone (iPhone 14 Pro Max, `00008120-00114DE80A90C01E`) refused launches earlier; on 2026-11-10 `devicectl device process launch` worked (certificate trusted) and the current build is installed on it (`build-lab.sh ios --device --phone ...`, 5 minutes on a loaded Mac). A drive over the network (`scripts/exact.sh agent ios --device --phone 00008120-00114DE80A90C01E "clock data" "tap tab-player" "tap start" ...`) ended at its second op with "the app left the foreground ... so it is suspended": the phone was locked or showing a permission alert nobody could answer. On a phone the module runs the real engine even under the agent (only the simulator is virtual), so a drive measures the real switches. To measure: unlock the phone, keep it awake, launch the app by hand once and allow the microphone and speech recognition, then run the drive above with `tap tab-lab`, `tap stage-transcriber-scripted`, `tap inject`, and read `audio switch ... in N ms` in the Lab events and `Audio switch N ms` in Trace. Nothing below has been measured on a phone. Build and install: `mobile/scripts/build-lab.sh ios --device --phone 00008120-00114DE80A90C01E`. Collect a turn afterwards with the `devicectl` command above (Trace, Export JSON). Every number is read from the Lab screen (audio state, last switch ms) or the turn record (`audioSwitchMs` in `record.summary`).

1. **Switch latency listening to talking and back.** Lab, pick `Switch on address`, Start audio, hold Talk, release. The record's `audioSwitchMs` is the sum of both switches; the events `audio switch listening -> talking in N ms` give each. Run on the speaker, on wired headphones, on AirPods. Repeat 10 times per route; report median and worst.
2. **The 20 to 41 s route-switch stall reported with AirPods (iOS 27.0.1).** AirPods connected, `Switch on address`, Start audio, hold Talk: the engine restarts onto HFP. Watch for `audio switch ... took N s` (an `error` event fires above 5 s). Compare with `Always talking` (one switch at start only) and `Always listening` (none).
3. **Voice processing's effect on playback quality.** Play the same programme clip in `Always listening` and `Always talking` on the speaker and on AirPods; record the speaker output with a second device, or listen. Note the route in `audio-route` (in/out ports) and `voice processing on/off`.
4. **Background capture.** Start audio, lock the screen, wait 5 minutes: does the programme keep playing, does the mic indicator stay on, does an injected turn (not possible locked) or a spoken turn after unlock still land in Trace? Then lock during a held turn and release.
5. **Yielding on the speaker.** Inject fixture turn (or speak) while the programme plays: confirm by ear that it dips at about 1 s and pauses when the utterance counts; `audio-level` shows the node's real volume.
6. **Interruption and route change.** Phone call during playback: events `audio interrupted`, then `resumed` if the system recommends; engine `up` again. Plug and unplug AirPods during playback: one `audio route` event and one restart.

## For EXACT.md (what the reference pages lack or get wrong; Exact source at 76cd50f9e)

1. **The "open problem: the simulator exit after the claim" in `testing-and-debugging.md` is not an audio bug.** "the app hung up (exit code 0)" is `Agent.exitAfterLanding` (`host/apple/Sources/ExactKit/Agent.swift:140-146`) running because the driver's socket closed; a second driver launching the same bundle id on the same simulator (`--terminate-running-process`, `scripts/agent.mjs:569`) does exactly that. Hypotheses (a) to (c) there are not needed. Give every driver its own simulator (`simctl create`).
2. **When the agent socket opens**: after the first frame is applied (`ExactIOS/main.swift:117-141`, `exactSession(didChange:)`) and after the scene is foreground-active and the window key (`ExactKit/IOS/AgentFrontIOS.swift:29-45`, up to 15 s); the driver's bound is a fixed 20 s (`scripts/agent.mjs:567-579`), not configurable. A cold launch on a loaded machine, or any system alert over the app, misses it ("the app never opened its agent socket"). `simctl privacy grant all` does not grant speech recognition (no such service); an unanswered speech alert survives the app being killed and blocks every later launch until the simulator is restarted.
3. **A package's `context.agent` does not silence the audio session**: under the agent our claim was real (`cat=PlayAndRecord mode=Default in=[MicrophoneBuiltIn] out=[Speaker]` on the simulator, `AudioSession.swift:238`); only Exact's own sounds are replaced (`state.sounds.output = "agent"`). An `AVAudioEngine` we start ourselves under the agent opens the simulator's audio hardware; `AVSpeechSynthesizer.write` on `@MainActor` stalled the main thread on a fresh simulator until the drive's `clock` op timed out at 120 s (`agent.mjs:85`, `REPLY_MS`). The package must branch on `context.agent` for anything that can block the main thread.
4. **Claim options**: the bit order and raw values are in `NativeHatches.swift:236-243` (`mixWithOthers 0x1`, `duckOthers 0x2`, `allowBluetoothHFP 0x4`, `allowBluetoothA2DP 0x20`, `defaultToSpeaker 0x8`, `bluetoothHighQualityRecording 1<<19`, `farFieldInput 1<<18`, `overrideMutedMicrophoneInterruption 0x80`); mode names accepted: `default`, `measurement`, `voiceChat`, `videoChat`, `videoRecording` (an unknown name silently becomes `default`, `NativeHatches.swift:233`). `audio.md`'s listening claim lists `mixWithOthers`; ours omits it on purpose (a radio should take the session) and adds nothing else. `defaultToSpeaker` matters: `playAndRecord` without it plays from the earpiece.
5. **Release-then-claim works and is fast on the simulator**: listening to talking 228 to 349 ms, talking to listening 251 to 349 ms, including the claim call, the engine stop and the engine restart (the kit's `audioSwitch` events); the second claim's `AVAudioSession.mode` reads `VoiceChat` right after. Not a phone number.
6. **Mutation results are options**: `mutation exp as shape X` reads as `option<X>`; a field read is `type-not-a-record`. Use `derive v = match exp { case some(e) => e.field, case none => "" }`. `share(title=, files=[path])` takes a path held in a derive and compiles.
7. **`tree` right after a navigation** carries the note "N requests in flight ... start with `clock data`": always issue `clock data` before reading data-bound text. The tree lists the panels of every tab (hidden ones marked `[hidden]`), so an assertion on text must not depend on which tab shows.
8. **Pointer drive**: `tap <id> down` then `tap up` (the bare `tap up`, not `tap <id> up`) delivers `pointerdown` and `pointerup` to a `Pressable`; `tap up` with a target is refused.
9. **The Exact build log**: a first `ios` build in a new worktree took about 5 minutes (cargo 220 s, swift 128 s); the exit 128 seen once was a transient (a second run succeeded). The driver rebuilds nothing itself when the build is current.

## The Autoradio app spine (lane/ar-spine)

The plan is `docs/design/autoradio-app-ui-2026-11-10.md` (section 6). The spine is the shared skeleton the nine lanes build
on: a page host, OpenPages, the bar and the layers, Card/Layer, the theme loader, `PageSource`/`Signals`, a dispatch
table in the module, per-lane interface fragments. Every lane's files exist already as small stubs: **a lane replaces the
body of its own files and edits no shared file except its own band of the `provide` list in `app.contract`** (and only
if it adds a name a child component must `inject`).

### How the pieces fit

- `AutoradioLab/app.contract` is the root: `members` lines (already wired, one per lane), the `provide` list (a band per
  lane), and the view: sign-in screen, or the page deck (full bleed; `pages.contract` PageDeck, order and drag in `members-stack.contract`), and the glass
  lens bar (`bar.contract`) with the launcher and the player rising from it as sheets. There is no pile view (round 3).
- A **page** is `{address, kind, title}` (`Pages.swift`). `PageHost` in `pages.contract` maps kind to a component, one
  `if` line per kind, already pointing at each lane's component (`PaperPage`, `ReaderPage`, ...). The lane owns the
  component; it takes what it needs with `inject` (the root `provide`s names; children cannot hold resources, so a lane's
  resources and actions live in its `members-<lane>.contract`) and reads its page model with the `pmod` resource (the kit's
  `PageSource`: address to `PageModel`). Open a page with the `openPage(address, title)` action.
- **Dispatch table** (module): `handle("sourceOrSend") { args, reply in ... }` in your `AutoradioModule+<Lane>.swift`
  (`registerX()` is called once by the module). Reach `pages`, `signals`, `pageSources`, `runtime`, `choir`
  through `self`; `touch(["name"])` refreshes a live source.
- **Interface**: your source and send names and shapes go in `exact-package/interface/<lane>.json`
  (`{"shapes": {...}, "names": [{"source": ..., "live": true, "answers": "Shape", "options": {...}, "substitute": {"default": {...}}} | {"send": ..., "answers": "Shape"}]}`).
  `scripts/gen-exact-interface.py` merges the fragments and refuses a duplicate shape or name.
- **Strings**: `AutoradioLab/strings/en.json`, keys prefixed per lane (`paper.*`, `reader.*`, ...); `scripts/check-strings.py` fails on a
  duplicate or an unprefixed key.
- **Themes** are data (plan section 5, schema v3) and shared code (`AutoradioLab/themes.ts`): `mobile/themes/*.json`, imported by `themes-bundled.ts` (a new file needs one line there; `themes.test.ts` checks),
  plus any `*.json` dropped in `app:/data/themes` (read once at launch). The choice is `app:/data/theme-choice.json`; the system scheme and Reduce Motion are `exactViewport()` facts.
  The `th` record (`Theme`) is what views bind: `th.paper`, `th.fg`, `th.accent`, `th.fontRead` (the body face), `th.fontUi`, `th.radiusCard`,
  `th.shadow1..3`, `th.material1..3`, `th.motionFastMs`, `th.springStiffness`... No component names a colour, a font or a radius.
  `Card(level, testId, minHeight)`, `Layer(testId)`, `Surface(level, radius, bleed)`, `PageScroll(id)` are in `ui.contract`.
- **Demo mode is gone** (owner, 2026-11-20: "i never asked for that"). There are no fixtures served to the app, no `AutoradioDemo` flag and no demo session; every proof runs on real data (`scripts/proof-real.sh`) or, for page logic, in bun tests with a canned `fetch`. `Demo.clearLeftovers` (DemoLeftovers.swift) stays once: a phone that was in demo mode loses what demo left on it.
- `PageSource` (`Pages.swift`): `handles(address)` and `load(address) -> PageModel`; register in your `registerX()` with `pageSources.register(...)`
  (the most recently registered wins). `Signals` (`Signals.swift`): `signals.record(SignalEvent(kind:, subject:, detail:))`.

### Test id conventions

`<lane>-<thing>[-<id>]`, lower case, every interactive element has one, ids built from addresses keep the address as is.
Existing: `bar-stack`, `bar` (the glass lens), `bar-mark` (opens the launcher), `bar-field`, `bar-mic` (tap to talk, tap to send), `bar-send`, `bar-play`,
`bar-text` (the now-playing line), `bar-progress`, `page-indicator`, `mark`, `page-deck` (the pan surface), `deck-slot-<address>`, `story-more`, `launcher`,
`launcher-layer`, `launcher-open-<address>` (bring to the top), `launcher-close-<address>` (remove from the deck), `layer-scrim`, `place-<slug>`, `radio-page` (the Autoradio page, the first deck card), the Takes section (`radio-takes`, `takes-record`, `takes-stop`, `takes-review`, `takes-transcript`, `takes-publish`, `takes-mine-<n>`, `takes-muted-<n>`, `radio-take-mute`; tour `fixtures/tours/takes-record-publish.steps`, wire in `Choir/Takes.swift`), the notebook page (`notebook-page`, `notebook-line-<id>`, `notebook-score-<n>`, `notebook-behind`, `notebook-held-<n>`, `radio-notebook`, `place-notebook`; tour `fixtures/tours/notebook-page.steps`, wire in `Choir/Learning.swift`, model in `Notebook.swift`, fixtures `fixtures/learning/`), `paper-story-<route>`, `reader-cite-<anchor>` (lane 3),
`settings-theme-<id>`, `settings-mode-<light|dark|system>`, `settings-components`, `page-title` (every page's heading), plus the
integrator's (`signin`, `lab-talk`, `inject`, `stage-<kind>-<id>`, `trace-*`, ...), unchanged. Keep an id or update `scripts/proof-real.sh` in the same commit.

### File ownership (from the plan, section 6) and the one shared line

| Lane | Owns | The shared edit it may make |
|---|---|---|
| 1 Bar, radio, talk | `bar.contract`, `player.contract`, `members-bar.contract`, `AutoradioModule+Bar.swift`, `Kit/BarState.swift`, `exact-package/interface/bar.json` | its band in `provide` (app.contract) |
| 2 Paper layout | `paper.contract`, `members-paper.contract`, `AutoradioModule+Paper.swift`, `Kit/Layout/*.swift`, `mobile/layouts/default.json` | its band in `provide` |
| 3 Texture reader | `reader.contract`, `textures.contract`, `members-reader.contract`, `AutoradioModule+Reader.swift`, `Kit/TextureDoc.swift`, `Kit/Citations.swift`, `interface/reader.json` | its band in `provide` |
| 4 Cards, launcher, signals | `cards.contract`, `launcher.contract`, `members-cards.contract`, `AutoradioModule+Cards.swift`, `Kit/OpenPagesUI.swift`, `Kit/Signals.swift`, `Kit/Preferences.swift`, `interface/cards.json` | its band in `provide` |
| 5 Files and viewers | `files.contract`, `viewers.contract`, `members-files.contract`, `AutoradioLab/files.ts` and `files-model.ts` (shared code; Swift's Files service is gone), `Kit/Epub.swift`, package views `PDFPage.swift`, `QuickLookPage.swift` | its band in `provide` |
| 6 Mail | `mail.contract`, `members-mail.contract`, `AutoradioModule+Mail.swift`, `Kit/ChoirMail.swift`, package view `MailOriginal.swift`, `interface/mail.json` | its band in `provide` |
| 7 Settings, themes, accounts | `settings.contract`, `activity.contract`, `members-settings.contract`, `AutoradioModule+Settings.swift`, `AutoradioLab/themes.ts`, `mobile/themes/*.json`, `Kit/Accounts.swift`, `interface/settings.json` | its band in `provide` |
| 8 Onboarding and the edition topics | `onboarding.contract`, `members-onboarding.contract`, `AutoradioModule+Onboarding.swift`, `interface/onboarding.json` | its band in `provide` |
| 9 Motion and the mark | `mark.contract`, `mark.ts` (if a canvas is used), `AutoradioModule+Mark.swift`, `interface/mark.json`, motion tokens in the themes | its band in `provide` |
| Integrator | engine, `lab.contract`, `components.contract` (Components and Trace pages), audio session, `Kit/*` engine files | none |
| Spine (done) | `app.contract` view, `spine.contract`, `ui.contract`, `pages.contract`, `AutoradioModule.swift`, `AutoradioModule+Spine.swift`, `Pages.swift`, `scripts/*` | - |

`Kit/` is `mobile/AutoradioKit/Sources/AutoradioKit/`; file names must be unique there (the sync script flattens them). Changing a
page kind's address scheme is a spine change (`PageAddress.kind(of:)`).

### The three spikes (answers, on the simulator, 2026-11-10)

1. **A glass bar over a full-bleed scroll, with the keyboard: YES.** `main viewport-fit="cover" interactive-widget="resizes-content"`, the page a
   `scroll` filling the screen, the bar a `row position="absolute"` at `bottom: calc(env(safe-area-inset-bottom) + 8px)` with
   `backgroundMaterial="glass"` (`bar.contract`). With the field focused the bar rides above the keyboard (screenshot `12a-bar-field-focused.png`
   in the proof's directory). Caveats: (a) a separate glass node *under* a text field covers it (`_UIVisualEffectContentView` swallows the tap), so
   material, fill, radius and shadow go on the node that holds the content; (b) `backgroundMaterial` takes literal arms only: a ternary of
   `"glass"` and `"none"`, not `""`; (c) the Exact agent refuses to tap an element under the bar, and scrolls an element to the bottom edge
   of the scroll frame, which is under the bar: tool pages (Components, Trace) use `ToolScroll`, whose frame ends above the bar.
2. **A card dragged left and closed: YES, with `swipeleft`, not `pan`.** `swipeleft=closePage(address)` on the card: it follows the finger and the action
   runs when the finger lifts past 64 pt (needs `touch-action="pan-y"`); the proof drives it with `tap card-swipe-<address> drag -150 0` under
   `agent ios --touch platform`. `pan` reports the movement since its last event, so a pull (the bar's drag up to the launcher) accumulates `dy` in a state and
   decides in `panrelease`. Round 3 uses that for the page deck (`pageDrag`/`pageRelease`, `touch-action="pan-y"` on the deck so vertical scroll is left to the page). The bar has no drag and no hold any more.
3. **A colour bound from data: YES.** `background-color=th.paper`, `color=th.fg`, `box-shadow=th.shadow2` and `font-family=th.fontRead` (a data string,
   `ui-serif`) all paint from the `theme` source, and a switch (Depth, Newsprint, light, dark) repaints without a rebuild (proof screenshots 06 to 10).
   The data-theme path is therefore the only path: the "compiled themes" fallback is not needed.
   Also spiked: a 3D rotation, `rotate="y 50deg"` with `perspective` on the parent and `transition="rotate 400ms ease"` (settings page,
   `Spike3D`, `settings-spike3d`): draws as a narrower face, YES.

### Exact lessons (from this build)

- A `provide` list on the root and `inject` in children carry resources, states, derives and actions down: a lane's component takes what it needs and
  the page host passes only `address` and `title`. A members file (`members "./x.contract"`, block `members Autoradio`) needs at least one member.
- `use` cycles are refused (`contract-use-cycle`): shared components (`PlainPage`, `NotYetPage`) live in `ui.contract`, which imports nothing of ours.
- A native button's `text` takes no `font-family` (`lower-button-style`); `buttonStyle="glass|bordered|filled"` buttons inherit the root font.
- `swipeleft`/`swiperight` are the card-swipe primitive; `pan` gives deltas. `after(ms, action)` exists only in a `task` (hold-to-talk is `task voiceHold when voiceArmed`).
- Driver: `agent ios --touch platform` is needed for drags; under it a `tap X down`, `clock`, `tap up` sequence is refused, so hold gestures run in a second
  drive without the flag. `screenshot <png> window` shows the real display (keyboard, glass). The first launch of a fresh install can miss the
  agent socket or find the scene not foreground-active: retry. A system permission alert (microphone, speech) steals the foreground until the
  simulator is rebooted; `simctl privacy <udid> grant all <bundle>` works only on a freshly booted simulator.
- Contract build takes 2 s and finds nearly everything; do it before every simulator build. A contract-only rebuild is 12 to 40 s; adding Swift 3 min when cold.
- With a 3-minute cold build and 12-second contract builds, the Mac's load decides: at load above 60 the touch runner does not start.

### Spine decisions a lane must know

- Under the simulator's agent the engine is virtual, and the radio's
  buttons (`choirPlayPause`, `choirTalkDown`, `choirPlay`) start it virtually (`AutoradioModule+Spine.swift`). Speech recognition is not authorized
  in the proof, so a live turn has no transcript; the proof's turn uses the Lab's stub stages (scripted transcriber) through `inject`.
- The sign-in screen (`SignInScreen`, lane 8) shows when `au.signedIn == false`.
- The pile keeps pages in focus order; an older card is covered by the one in front, so a driver switches with the launcher's "Open" chips
  (`launcher-open-<address>`) or the card strip; lane 4 may fan the deck so older cards show.
- OpenPages persist in `Application Support/Autoradio/open-pages.json`; the proof deletes it for a clean start.
- Themes: Depth's dark `onAccent` is dark ink (the plan's white fails the 3:1 contrast gate on the dark accent). The loader enforces the gate.
- There is no Swift view for drawing, layout or motion in the app; the only Swift is the engine, audio session, models and the module's data. Nothing to move to Contract.
- Not done in the spine: keyed enter/exit when the page changes, the Undo pill, swipe right, the launcher's live rows, the TetraMark (a flat
  glyph in `mark.contract`; lane 9 builds it in Contract), Android.

### Proof

`mobile/scripts/proof-real.sh` drives the app on real data (see "Real data" below); the spine's checks (bar, launcher, deck, themes) are its `spine` drive. `mobile/scripts/proof-replay.sh` and `swift test` still pass.

## Lane 3: the Texture reader and the Textures place

Files: `reader.contract`, `textures.contract`, `members-reader.contract`; Kit `TextureDoc.swift` (model, parser), `Citations.swift`,
`TextureMarkdown.swift`, `TextureReaderSource.swift` (loader, Textures list, read-along and aim state);
`AutoradioModule+Reader.swift`; `interface/reader.json`; sample story bodies `fixtures/texture/story-*.json` (read by TextureBodyTests); proof: the read drive of `scripts/proof-real.sh`.

- `pub:<route>` and `texture:<doc>[@rev]` both become a `ReaderDoc` (flat blocks of runs; no body_doc falls back to Markdown). Citations are tappable
  runs (`reader-cite-<anchor>`), cards open in place (`reader-card-<anchor>`), "Open source" opens a sheet with the quote highlighted in its block; a
  Texture transclusion is a nested card with "Open as page". Sources list, More on this, versions sheet, page menu, revising banner (`agent_revision_pending`).
- Read along: lane 1 calls `ReaderFollow.shared.set(address:spoken:)`; matched by text. Aim: `ReaderAim.shared` / `raim` resource (long-press a paragraph, or the menu).
- Experiments with no control now (the demo menu that offered them is gone): side notes (textflow) and Markdown with atom pills.

## Files and the viewers (lane 5)

Files is a place, not an app: `file:/` is the listener's computer's files (folders, breadcrumbs, list and grid, sort by name, modified,
size or kind, search under the folder, recent, new folder, add, rename, delete); `file:/path/name.ext` is one file opened as a page, so it
sits in the pile with everything else. The lane owns `files.contract` (the place and the router), `viewers.contract`,
`members-files.contract` (prefixes `fs` = the place, `fv` = the viewers), `exact-package/apple/AutoradioModule+Files.swift`,
`AutoradioLab/files.ts`, `Kit/Epub.swift` and `fixtures/files/sample-tree.json` (the Epub tests' input, made by `scripts/make-files-fixtures.py`: a PNG, a 3-page PDF,
a 3-chapter EPUB, a WAV, an MP4, CSV, Markdown, text and code, all invented). Proof: the Files steps of `scripts/proof-real.sh`; the page logic is `files.test.ts` with a canned computer.

- Routes (`internal/autoputer/files.go`): `GET /api/files[/{path}]` (an array of `{name, type, size, modified}`), `?disposition=inline` for
  media, `POST` makes a folder, `PUT` writes, `DELETE`, `PATCH {move_to}` renames a regular file and only when the server's move gate allows
  (folders cannot be moved; the page says so).
- A file is brought to `app:/cache/files/<hash>/<safe-name>` (the hash is of path, size and time, so a changed file is a new entry), and the
  viewers play from that path. Per-kind size caps refuse before downloading ("too large to open on this phone").
- Viewers: image (`transformDragFor`: pinch and drag, 1x to 4x, plus Fit, 2x, 4x, Center), video and audio (native controls; the radio is
  paused when media plays and media is paused when the radio plays, through `paused=` and `choirPlayPause`, never by claiming the
  session), PDF (an `iframe` on the local `.pdf` is PDFKit's `PDFView`; page by `#page=N`; search by PDFKit in the module), e-book
  (`Epub.swift`: ZIP central directory, raw deflate through Compression, container, OPF spine, XHTML to blocks, then Markdown for
  `text markup="markdown"`), text, Markdown, code, CSV/TSV, and an honest "cannot preview" page with the file's facts.
- Ways the code can fail are listed at the top of `files-model.ts` and `Epub.swift`; `files.test.ts` and `EpubTests.swift` cover each.

## Integration (lane/ar-app): the nine lanes in one app

`lane/ar-app` is the spine plus lanes 9, 1, 2, 3, 4, 5, 6, 7, 8 merged (`--no-ff`, in that order). One proof drives the whole app:
`scripts/proof-real.sh` (helpers in `scripts/proof-common.sh`) for the app and `scripts/check-all.sh` for everything that needs no simulator (the deleted `proof-app.sh` ran both); the demo-mode whole-app and lane proofs it replaced were deleted on 2026-11-20 with demo mode.

Decisions made at the seams:
- **Generated files** (`exact-package/exact.json`) are never hand-merged: take a side and
  run `gen-exact-interface.py`. The build runs it again.
- **One owner per thing.** Talk mode: Settings (`AppSettings` `talkMode`); the bar and the topics step read the engine's addressing stage that Settings
  applies (the setup no longer forces "open"). Gesture meanings: Settings' `swipeLeft`/`swipeRight`, handed to lane 4's `Preferences`
  (`syncGestureMeanings`), and lane 4's own buttons write back to Settings. "What the paper has learned": `PreferencesLearned`, over lane 4's acts.
- **Signed out** the paper is the root; sign-in is a code screen over the app that appears when `au.status == "pending"` (the paper's Sign in button).
- **Bar**: a page-colour backing under the glass (`th.paper`, 0.82), the aim reads `readerAim` (a block of the open document) and `ob.aim` before setup;
  the mic level (`RadioInfo.micLevel`, from the audio bus) reaches `MarkAt(level=)`; lane 1 feeds `ReaderFollow` with the sentence being spoken.
- **Proof state.** `defaults write` from `simctl spawn` does not override what the app itself has written (theme, mode, talk mode, gestures live in
  the app's own preferences file): `proof-real.sh` removes the app's plist.
- **Rough / open.** Files still renders Markdown and EPUB text with its own `text markup="markdown"` (the reader's `ReaderBody` takes the one `rdoc`
  resource, so sharing it is not a small change). The mic level and
  the reader's read-along are wired (`RadioInfo.micLevel`, `ReaderFeed`) but the simulator's engine is virtual, so neither is seen moving in the proof.
  Engine (round 2, done): playback rate with a real position (`Playhead`), `DeskTurn.interrupted`, and `speakText(address:text:)` (mail's "Read it to me").

## Real data (lane/ar-app-real): the app signed in to a real computer

`scripts/proof-real.sh` runs the app on the simulator, signed in as the test account on staging, and tours every place with real
data (paper, two stories in the reader with "More on this", a played story, Textures list and documents, the edition state, Files, Mail, Settings
account / computer / appearance, a theme loaded from a Texture, the largest document). It asserts real conditions (a title is not empty, no error
banner), not fixture text. `REAL_PROMPT=1` adds one typed question in the bar (a model call on the test computer); it is off by default.
- **Sign-in.** The real sign-in is a browser device flow a script cannot approve. The proof copies a key file (`REAL_KEY_FILE`, default the QA key of the
  test account) with `cp` into the app's `Documents/proof-key`; the simulator build reads it into the Keychain at launch and deletes it
  (`AutoradioKit/ProofKey.swift`, called under `#if targetEnvironment(simulator)` only). The key is never printed; the script fails if a `choir_sk_`
  string appears in the drive output.
- **First run is real.** With no edition on the computer the app opens the "Make it yours" offer once; the proof screenshots it (`real-00`) and skips it.
  The app now asks the computer first (`GET /api/texture/edition`): `{found:false}` is the normal first-run answer; a computer that has an edition gets
  no offer and no card, and the Textures list pins that document.
- **What real data changed** (each with a test in `RealShapeTests`/`TypedTurnTests`): related stories are `links` in the real answer; the front dek comes
  with a space before each stop (tidied); the owner key's `computer_id` is empty in `whoami` (the page names the computer the phone found); the typed Ask
  in the bar was only recorded as a signal and now runs a turn (`TurnEngine.typedTurn`, `LabRuntime.ask`, send `choirAsk`); `touch` could be called
  from a background thread and crashed (`Set.insert`, found in a crash report during the typed-ask drive); long ids and unbroken tokens ran off the card
  (`overflow-wrap="anywhere"`); the Textures page showed the generic "Make your edition" card twice.
- **Known and left.** A typed Ask forks and revises the story the radio has loaded, not the page in front (the desk stage's first turn forks the
  programme); the Textures list shows the newest 100 documents (the route pages with `next_cursor`, not read yet); the document route returns every
  revision body (the largest document on the test computer has 314), which the reader fetches whole.

## Round 3 merge (lane/ar3-app)

- The lens is theme data: a theme's `lens` is `{ "style": "A", "wash": 0 }` (A native glass, B no material with a wash of the page colour,
  C ultra-thin; wash 0 to 1). Absent, it is A and 0. `Theme.lensStyle` / `Theme.lensWash` reach the bar as `th.lensStyle` / `th.lensWash`;
  there is no in-app switch (the launcher's Developer lens rows are gone). Failure list F11 in `AutoradioLab/themes.ts`, tests in `themes.test.ts` (L1-L3).
- Cancel while listening calls `choirTalkCancel` (the engine drops the audio and the words, nothing is sent); Send is the microphone again.
- `aim` (the address of the page in front) goes from the bar to `choirAsk` / `choirTalkDown`, into `LabRuntime.aimAddress`, and from there into the
  Autoradio desk request's `page.address`.
- `RadioInfo.modeLabel` is gone; nothing in the app describes a hold gesture.
- With no theme chosen the app shows Chrome Pastel (light) and Liquid Metal (dark).
