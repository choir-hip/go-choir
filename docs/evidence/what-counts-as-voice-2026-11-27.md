# What counts as voice: a list for the owner

Standing: undecided | kind: wanted | settled-by: nothing yet; the owner's ruling on decision D2 of docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md | note: built from the audit of 2026-11-24, which is an estimate by reading and was never measured

The rule is that only voice is native. What "finished" means depends on
which files are called voice, and two readings exist.

| Reading | What it calls not voice | Size |
| --- | --- | --- |
| The gate (`scripts/check-autoradio-native-boundary`, with `scripts/autoradio-native-voice-list.txt`) | every native file off the list | 34 files, 3,375 lines (measured 2026-11-27) |
| [The audit of 2026-11-24](exact-conversion-audit-2026-11-24.md) | the same, plus the logic inside files on the list | of 47 listed files matched by name (10,958 lines then), 5,083 lines stay and about 5,875 could move (its estimate) |

So the mission as written, which drives the gate's count to zero, would
leave roughly 5,900 lines of logic in Swift that the audit says shared
code could carry: the phases of a turn, the choice of model and voice, the
order of voice setup, the catalogues. Moving those is a larger job than
the 3,375 lines and touches the voice turn itself, where a mistake is
heard.

The options (decision D2):

1. **The list as it is** (the default). Finished means the gate reads zero. The 5,900 lines stay Swift until a later mission.
2. **The audit's boundary.** The files below are split, the list is narrowed in the same change, and finished means the gate reads zero against the narrower list. Larger, and red in places.
3. **File by file.** The owner marks rows below.

A middle course I would take without a ruling, and have not: finish
against the list as it is, then take the rows below that the audit says
move whole (no split), since those carry no audio code at all.

## The files on the voice list that the audit says hold movable logic

Line counts are the audit's, from 2026-11-24. Matching was by file name,
so entries the list gives as a directory are not in this table.

| File | Lines (2026-11-24) | Audit: lines that stay | Audit's verdict |
| --- | --- | --- | --- |
| TurnEngine | 789 | 200 | split (200 stay): preroll, frame append, VAD feed, speak and claim calls stay; phases, System1 to holder to desk to voice, and outcomes go to `voice-turn.ts` |
| VoiceSetup | 649 | 200 | split (200 stay): FluidAudio fetch, Core ML compile and proof run stay; steps, picks and words go to `voice-setup.ts` |
| Configurator | 401 | 0 | moves: `voice-config.ts`; needs only native `deviceFacts` |
| LabRuntime | 487 | 90 | split (90 stay): resident memory and engine construction stay; config store to `voice-config.ts` |
| LocalAck | 397 | 80 | split (80 stay): llama.cpp call and non-verbal cue stay; prompt, splitter, history and holder go to `local-ack.ts` |
| RemoteAudioPlayer | 274 | 0 | moves: Exact `audio` element (audio.md 147-156). Lacks: playback rate to 3x, ended and interruption events |
| Deck | 601 | 350 | split (350 stay): engine drive, journal file, lock-screen reflex, session retry stay; rules 1 and 4 and the held list move to `deck.ts` after the locked-phone proof |
| KnownModels | 251 | 0 | moves: server-fetched model-list data |
| Choir/ClipLocator | 349 | 100 | split (100 stay): AVFoundation byte-range decode stays; search to `clips.ts` |
| HandsFree | 336 | 110 | split (110 stay): per-frame ingest, VAD feed and session partials stay; policy driving, commit, barge-in, visibility and speaker call go to `hands-free.ts` |
| VoiceChoice | 265 | 90 | split (90 stay): SpeakingVoice first-sound fallback stays; catalog, choice and readiness go to `voice-choice.ts` |
| Choir/ChoirDesk | 173 | 0 | moves: delete or `desk-client.ts` |
| Core | 246 | 80 | split (80 stay): registry building native stages; config types to `voice-config.ts` |
| Events | 236 | 70 | split (70 stay): host clock stamped on audio stays; log and types to `voice-turn.ts` |
| Stages | 316 | 150 | split (150 stay): Transcriber, Synth, VAD and Embedder protocols; System1, Desk, Holder, Addressing and Yield move |
| BasicStages | 273 | 110 | split (110 stay): EnergyVAD and RouteMic stay; addressing, yield thresholds and fakes go to `voice-turn.ts` |
| VoiceSampler | 159 | 0 | moves: `voice-choice.ts` |
| ModelCandidates | 150 | 0 | moves: `voice-config.ts` data |
| TurnTimeline | 148 | 0 | moves: `voice-turn.ts` |
| Choir/AutoradioDesk | 221 | 90 | split (90 stay): reading the POSTed stream until gaps.md 36 closes; rest to `desk-client.ts` |
| RetrievalFacts | 130 | 0 | moves: JS calls `retrieval-run.ts` directly |
| KeptTurns | 178 | 60 | split (60 stay): writing mic.caf stays; prune, list and context.json go to `kept-turns.ts` |
| ModelOffer | 113 | 0 | moves: `voice-setup.ts` |
| ModelStore | 277 | 170 | split (170 stay): file SHA-256 of large GGUF, load and unload stay; specs and ledger go to `voice-config.ts` |
| Choir/ChoirSystem1 | 83 | 0 | moves: `api.ts`; plain POST |
| HandsFreeSwitch | 73 | 0 | moves: `hands-free.ts`; native gets `micOpen(bool)` |
| DeskContext | 63 | 0 | moves: `voice-turn.ts` |
| MicCheck | 62 | 0 | moves: `onboarding.ts` |
| FixtureTurn | 58 | 0 | moves: app asset read by path |
| LiveConfigBackend | 42 | 0 | moves: with Configurator |
| FixtureTurnAudio | 13 | 0 | moves: goes with the asset |
| FixtureClip | 12 | 0 | moves: app asset read by path |
