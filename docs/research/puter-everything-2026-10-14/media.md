# Media group review: video-studio, audio-studio, 3d

Reviewer group: media. Clone: `scratchpad/puter-everything/repo/src/{video-studio,audio-studio,3d}`.
Nothing was executed. All paths below are relative to `repo/src/` unless they start with
`internal/` or `docs/` (those are in Choir, `/Users/wiz/go-choir`).

Headline: this group is **not** vibe slop. video-studio and audio-studio are the
strongest code I would expect in this repo: framework-free JS with pure model
modules, many tests, and real engines (WebGL2 compositor, WebCodecs pipeline via
Mediabunny, a 21k-line Rust DSP crate). 3d is very broad (142k lines of TS, a
Blender clone) with thin tests, and is breadth-first generated code. All three
share one house style (identical README voice, the same `check-licenses.mjs` gate,
long explanatory headers, zero TODOs). This points to one strong code-generation
pipeline, not hand-written code. The good parts are good, though. The model
layers are clean and tested.

---

## 1. video-studio (CapCut-class editor). The lead example

### 1.1 What it is, stack, size

- A multi-track browser video editor: timeline, multiple timelines per project,
  compound and multicam clips, keyframes, 92 effects, 56 transitions, 48 filters,
  captions and transcript editing, MediaPipe-based retouch and background
  removal, export, review sharing, and an **LLM edit assistant**
  (`README.md`).
- Stack: plain JavaScript ES modules with no framework, built with Vite (`package.json`).
  - **mediabunny 1.61.3** (MPL-2.0). It does demux, WebCodecs decode and encode,
    and muxing to MP4, MOV, WebM, Ogg and FLAC. MPL-2.0 is file-level copyleft. It
    is fine as an unmodified dependency, but any patch to its files must be
    published. Flag it, but it is not blocking.
  - @mediapipe/tasks-vision (Apache-2.0). Four MediaPipe model binaries are
    committed in `video-studio/src/media/models/` (about 26k "lines", several MB).
  - lucide (ISC), @fontsource fonts (OFL).
  - A tiny Rust/WASM DSP crate: `video-studio/wasm/dsp/src/lib.rs`, 161 lines
    (peaks, mixing, resampling, limiting).
- **No ffmpeg.wasm.** Rendering and export use WebCodecs through Mediabunny
  (`engine/exporter.js:12-16` imports `Output`, `CanvasSource`,
  `AudioSampleSource`, `VideoSampleSink`).
- Size: 431 files. About 86k lines of JS in `src/` (excluding the model
  binaries), about 20k lines of tests in 107 files, and **1,372 test cases**
  (vitest + jsdom).
- Layout:
  - `src/model/` (40 files): pure data and edit functions.
  - `src/engine/` (38): scene evaluation, the WebGL2 compositor, the exporter,
    effects.
  - `src/media/` (42): analysis, AI and Puter calls, proxies.
  - `src/ui/` (about 120): DOM.
  - `src/storage/`: library, backends, zip bundle.
  - `worker/`: a Puter serverless worker for review sharing.
- Puter coupling is narrow: 21 files call `puter.*`, mostly `puter.ai.chat`,
  `puter.ai.speech2txt`, `puter.ai.txt2img`, `puter.fs.*`, `puter.kv.*` and
  auth. Storage is already behind a backend abstraction
  (`storage/backends.js`, with OPFS as the fallback).

### 1.2 Quality

Real engine, not a thin UI. The evidence:

- **Edit model** (`model/timeline.js`, 1,139 lines). About 80 exported pure
  functions over the project: `addClip`, `moveClip`, `trimClip`, `splitClip`,
  `splitAt`, `deleteClips`, `trimToPlayhead`, `setSpeed`, `setSpeedCurve`,
  `applySpeedRamp`, `freezeFrame`, `detachAudio`, `setTransition`, `removeRange`,
  `groupClips`, markers, track ops and keyframe ops.
  - Header contract: "They mutate the project passed in (the store snapshots it
    for undo before calling) and never touch the DOM, so they are covered
    directly by unit tests."
  - `removeRange` (line 1101) is a correct ripple delete across all unlocked
    tracks. It splits clips at both edges and shifts markers, the in/out range
    and the transcript words. This is the primitive behind transcript-based
    editing.
- **Document** (`model/project.js`). Plain JSON with **integer microsecond
  times**. Assets are keyed by id, and media is stored content-addressed as
  `media/<hash>.<ext>` (`storage/library.js` header). The clip types are video,
  image, audio, text, sticker, avatar, effect, filter, adjust and compound.
  Every optional field has a `normalize*` function, so loading is defensive.
- **Store** (`model/store.js`). `update(label, fn, {coalesce})` snapshots the
  project with `structuredClone` before each edit. After the edit it runs the
  invariant passes (`cleanupTransitions`, `resolveOverlaps`, `pruneTracks`).
  `beginBatch`/`endBatch` fold several edits into one undo step. Compound clips
  open as nested stores, each with its own history. Undo is a snapshot stack
  with a 200-step limit (`model/history.js`).
- **Scene** (`engine/scene.js`). `evaluate(project, t)` is a pure function from
  the project and a time to an ordered layer list (transitions, effects,
  filters, adjust layers, media layers with keyed transforms). The header says:
  "shared by the live player and the exporter". That is the right architecture:
  one evaluator, two sinks.
- **Compositor** (`engine/compositor.js`, `engine/shaders.js`). WebGL2, with
  per-layer grading, LUTs, curves, blend modes, transition shaders and effect
  passes. It accepts `HTMLCanvasElement` or `OffscreenCanvas`.
- **Exporter** (`engine/exporter.js`). Renders frame by frame, not in real time,
  with a dedicated compositor on an OffscreenCanvas. Video sources are decoded
  with Mediabunny `VideoSampleSink`, and the sound is mixed offline
  (`engine/audio-mix.js`). Output goes to `CanvasSource` and `AudioSampleSource`
  in an MP4, MOV or WebM `Output`, with H.264, HEVC, VP9 or AV1 where
  `canEncodeVideo` allows. There is a capability probe (`detectCapabilities`),
  size and bitrate estimates, and an async generator `renderFrames(...)`
  (line 769). Audio-only export writes WAV, M4A, Ogg or FLAC.
- **Preview decode** (`engine/frame-source.js`). Media elements give hardware
  playback. Mediabunny `CanvasSink` gives frame-exact frames while seeking,
  with coalesced requests.
- **Proxies** (`media/proxy.js`, `workers/proxy-worker.js`). 720p proxies are
  made in a worker and written into OPFS.

Weaknesses:

- **Edits are opaque closures.** `store.update(label, fn)` records only a
  before-snapshot and a label. There is no serializable operation log, so you
  cannot replay, rebase or audit an edit except as a whole-document diff.
- Coalescing depends on wall-clock time (`performance.now()`, a 1,200 ms
  window). Selection and playhead are store state that the assistant reads
  ("selected"), which is ambient state an external agent cannot rely on.
- **Rendering needs a browser tab.** The compositor is GLSL. A server-side or
  headless export would need Chromium with WebGL2 (GPU or SwiftShader), or a
  re-implementation of every shader in ffmpeg filters, which would not match.
- A very large feature surface (retouch makeup shades, avatars, 41 animations)
  that is mostly catalogue breadth. That is not worth porting early.
- Everything AI goes through `puter.ai.*`: STT, chat, image generation, TTS.

### 1.3 The edit assistant: already an app-agent prototype

`media/edit-assistant.js` (804 lines) and `ui/assistant-actions.js` are the most
relevant code in the whole group for Choir. The design:

- **About 22 tools as OpenAI-style function schemas**, each with a
  `check(args, state)` validator. Validation clamps values, resolves ids and
  returns a short reason on error. The tools:
  - `split`, `delete_clips`, `remove_time_range`, `trim_clip`, `move_clip`,
    `duplicate_clips`, `set_speed`
  - `add_text`, `edit_text`
  - `set_filter`, `add_effect`, `set_transition`, `set_animation`
  - `set_audio`, `adjust_color`, `set_transform`
  - `add_captions`, `remove_silences`
  - `add_marker`, `select_clips`, `set_aspect_ratio`, `move_playhead`
- The model sees **seconds**, and the project stores microseconds. Clip targets
  are ids or **clip words**: `selected`, `all`, `all_video`, `all_text`,
  `all_captions`, `all_audio`, `base_track`, `first`, `last`
  (`resolveClips`, `CLIP_WORDS`). Locked tracks and clips of the wrong type are
  rejected with counts.
- `projectBrief(state)` is a compact text rendering of the timeline: one line per
  clip with its id, type, time span and salient attributes. It is capped at 160
  clips, and long caption tracks are summarized. `catalogBrief()` lists the
  valid catalog ids.
- `askForEdits` tries native tool calls first. If there are none, it parses a
  JSON action list, and then it retries in JSON-only mode.
- All calls of one request become **one undo step** labelled
  `Assistant: <request>`. Each call is re-validated against the project as it
  stands at that moment, so later calls see earlier results.
- `summarize(results)` produces a note kept in the chat history. The note
  includes the ids that were touched, so "make it red instead" resolves.
- There are 28 test cases in `tests/edit-assistant.test.js`.

There is no external control surface. `window.vs` exists only in dev builds
(`main.js:250`), so no outside agent can drive it today. Choir supplies that part.

### 1.4 Worth taking for Choir

**Take (MIT, framework-free, lift almost verbatim):**

1. `model/project.js`, `model/timeline.js`, `model/keyframes.js`,
   `model/speed-curve.js`, `model/compound.js`, `model/timelines.js`, `lib/time.js`.
   This is the edit algebra. Port it as the reducer layer and keep its tests.
2. `engine/scene.js`, `engine/compositor.js`, `engine/shaders.js`,
   `engine/exporter.js`, `engine/frame-source.js`, `engine/audio-mix.js`, plus
   `mediabunny` as a dependency. This is the render path for preview and export.
3. `media/edit-assistant.js`. Treat it as the **reference verb schema and
   validator design** for app agents: units the LLM can handle, entity "words",
   clamp-and-explain validation, one batch per request, and a summary note with
   the ids it touched. Rewrite it into the Choir op vocabulary below.
4. `media/transcript.js` (filler and pause detection, word ranges to cut
   ranges), `media/highlights.js` (transcript to strict-JSON highlight picks,
   validated), and the caption grouping in `media/transcribe.js`. These are
   small and high-value.
5. `storage/bundle.js` and `storage/zip.js` (a project plus its media as a
   ZIP). These are useful for import and export interop.

**Leave:** the UI (`src/ui/`, roughly 50k lines, which Choir would redo in
Svelte on `AppSurface`), the retouch, makeup and avatars stack, the Puter review
worker (`worker/`), and the long effect catalogues. Effects can come later as
data.

### 1.5 App-agent design: "Cut" (video desk)

**State as typed objects in the Choir object graph.** The object graph
(`internal/objectgraph/object.go`, `registry.go`) gives:

- `Object{CanonicalID, ObjectKind, OwnerID, ComputerID, VersionID, ContentHash, Body, Metadata, SupersededBy}`
  and typed `Edge`s;
- content-hash or external-key identity, and a `Versioned` flag;
- `PutBatchConditional` compare-and-write (`store.go`), on a Dolt store.

The registry already has `choir.media_item`, `choir.audio_recording`,
`choir.transcript`, `choir.web_capture`, `choir.artifact_manifest`,
`choir.artifact_blob`, `choir.provenance_activity`, `choir.review_record` and
`choir.consent_record`. It has the edges `derived_from`, `has_media`,
`has_transcript`, `references`, `cites`, `revision_parent`, `was_generated_by`
and `transcludes`. Under the owner's "existing types unless we need to" rule
(`docs/autopaper-sw-design-2026-10-09.md`), the mapping is as follows.

| Video-studio concept | Object graph mapping | Fit |
|---|---|---|
| Imported file, recording, screen capture | `choir.media_item` (content-hash identity; the bytes as `choir.artifact_blob`). Edges: `captured_from` to a `choir.web_capture` when it came from the web, and `has_transcript` to `choir.transcript`. | Clean. Video-studio already hashes media (`media/<hash>.<ext>`). |
| **Clip = a capture** | A clip is a *time-span reference into a capture*: `{media_item, in, out}` plus look. It lives inside the cut revision body. When anything outside the cut cites it (an Autopaper story quotes 0:42 to 0:51 of an interview, or a Texture doc embeds it), it is **promoted** to its own span object. That object follows the `choir.retrieval_span` pattern (`contains_span`), with a `derived_from` edge to the media item. | Mostly clean. Making every clip an object would turn each trim into a many-object write. Promote on citation instead. |
| Transcript (`project.transcript.words`) | `choir.transcript` (exists), linked to the media_item, not stored inside the project. Word-to-time maps make transcript ranges citable by research and Texture. | Clean, and better than today, where words live in the project and are rewritten by `removeRange`. Keep the source transcript immutable. The cut's transcript is a projection through the EDL. |
| **The edit decision list** (project tracks, clips, markers, settings) | A versioned head object plus immutable revisions. This follows the `choir.texture_document` (external key) / `choir.texture_revision` (content hash, `Versioned`, `revision_parent` edge) pattern. Either add one generic pair for all app agents (`choir.app_document` / `choir.app_revision`, with `metadata.app = "cut"`), or a `choir.video_cut` pair. Each revision body is the normalized project JSON, and each revision commits with its op batch (below). | The project JSON maps 1:1. It is already a pure, normalized JSON document. One generic app-document pair for every app agent is preferable to a kind per app. |
| Undo history (200 snapshot stack) | Free. The revision chain *is* history. Undo is a forward revision whose body equals an older one, so it is content-hash identical and dedupes. | The in-memory History can stay as a client-side optimistic layer, or it can be deleted. |
| Multiple timelines (`project.timelines`), "duplicate to try an alternative edit" | **Variants as branches.** In this proposal a variant is a new head whose first revision has a `derived_from` edge to the parent revision. That is a branch at object level. Merging a variant back means applying its op log onto the other head. **Caveat:** Dolt `DOLT_BRANCH` is database-wide. Choir uses it for candidate isolation (`internal/computerversion/dolt_branch_isolation_pinned_test.go`), so per-edit Dolt branches would fork the whole computer's state. Do app branches at object level and let Dolt commits supply the history underneath. | Clean at object level. Not at Dolt level. |
| Compound clip (`sequence.tracks` nested) | Either inline (as now), or a `transcludes` edge to another cut's revision. The second makes "insert my intro cut into every short" a live reference. | Good fit and a real upgrade. |
| Export or render | `choir.artifact_manifest` plus a blob for the output file. A `choir.provenance_activity` (render) has `was_generated_by` and `derived_from` edges to the exact cut revision and the render settings. This gives reproducibility for free: the same revision and settings produce the same manifest. | Clean. |
| Review comments (Puter worker + KV today) | `choir.review_record` (exists), with a `reviews` edge to the cut revision and a time or range in the metadata. | Clean, and it deletes the whole `worker/` subsystem. |
| People in footage (retouch, background removal, dubbing a voice) | `choir.consent_record` before publication. | The existing kind fits. |

**The op log, the missing piece.** The agent-relevant EDL state is the op
batches between revisions, more than the snapshot. A sketch:

```json
{ "kind": "cut.op_batch", "cut": "obj:app_document:...", "base_rev": "sha256:...",
  "actor": {"type": "user|cut_desk|external", "id": "..."},
  "label": "Assistant: trim the first 2 seconds",
  "ops": [
    {"op": "remove_time_range", "start_us": 0, "end_us": 2000000},
    {"op": "set_filter", "clips": ["all_video"], "filter": "warm", "intensity": 80}
  ],
  "result_rev": "sha256:...", "rejected": [{"op": 1, "why": "..."}] }
```

How the log works:

- Ops are the edit-assistant tools, plus the remaining pure `timeline.js`
  functions (insert or overwrite from a media_item span, slip and slide, ripple
  trim, link and unlink audio, keyframe set, track ops).
- A batch carries `base_rev` and is written with `PutBatchConditional`
  (`ExpectedContentHash` on the head). The user's drag and the agent's batch
  then serialize cleanly. On conflict the desk re-validates the ops against the
  new head, the same way `assistant-actions.js` already re-checks each call
  against the current project, and retries.
- Direct manipulation produces the same ops. One timeline drag is one
  coalesced op. "Edits are closures" goes away, and every edit becomes auditable
  and replayable.
- The op batch is a semantic act and belongs on the commitment ledger tape.
  The revision is the object.

**Direct-manipulation actions** (the UI must emit ops for each): drag and move
across tracks, trim handles (ripple and roll), split at playhead, ripple delete,
slip and slide, speed, keyframe drag, transform gizmo on the player, text edit
in place, select transcript words and delete, set in/out, add marker, drag from
the media bin, apply a preset by drag. Selection and playhead stay
**ephemeral** per-window UI state (`choir.desktop_app_instance`), not part of
the cut.

**Verbs the Cut desk exposes to other desks and external agents:**

- `inspect(cut, rev?)`: the `projectBrief` text plus structured JSON.
- `apply_ops(cut, base_rev, ops[], label)`: returns the new revision, with
  per-op done or skipped reasons.
- `edit(cut, natural_language)`: the assistant loop, one batch per request.
- `preview_frame(cut, rev, t)` and `contact_sheet(cut, rev, times[])`: images,
  so an agent can **see** its edit. The assistant cannot do this today.
- `render(cut, rev, range?, settings)`: returns an artifact manifest. This is
  long-running background work.
- `branch(cut, from_rev, name)` and `merge(variant, into)`.
- Analysis jobs that write objects back: `transcribe(media_item)`,
  `find_silences`, `find_scene_changes`, `find_highlights(n, length)` (each
  highlight becomes a new variant cut), `captions(style, lang)`,
  `reframe(aspect)` (a new variant), `dub(lang)`.
- `cite_span(cut, rev, t0, t1)`: promotes a span object for outside citation.

**Desks it would call:**

- **Research**: "find b-roll for this sentence" (returns web_captures and
  media_items with licence metadata), and fact-check claims in a transcript.
  Research is read-only, which fits.
- **Texture**: script-to-video. A Texture document is the script, and the cut
  links to the revision it implements. Texture can also own published show
  notes or an article that embeds cited spans. Texture stays the sole writer of
  canonical *documents*. The cut is an app object, not a Texture document, and
  this boundary should be stated explicitly.
- **Engineering**: new effects or shaders, or a custom render step, built in a
  capsule and then adopted as catalog entries.
- **Autoradio and Autopaper**: an Autopaper story or Autoradio run sheet requests
  a video segment. The cut links back with `derived_from` to the
  `choir.autoradio_run_sheet` or the story cluster.

**Publish versus private:**

- *Published to the Autopaper object graph:* the rendered artifact manifest,
  its provenance activity, and the cut revision it came from. A publishable EDL
  means another computer can adopt it and re-cut it, and the remix lineage is
  just `derived_from`. Also published: cited spans, transcripts of published
  media, and the source media_items the owner chooses to release (or only their
  hashes and captured_from web_captures, when the media is third-party).
- *Private:* raw footage, proxies (these stay in the browser or OPFS cache
  only, as today), unpublished variants, the op log with its actor details,
  review records, retouch and face data, and consent records (publish only an
  attestation).

**Why it is better as an agent than as a plain app:**

- Long jobs run in the background on a persistent computer: transcribe, find
  highlights, make 5 shorts, caption, dub, render.
- Variants are cheap and auditable.
- The agent can verify its own work with frames.
- Other desks can request video ("cut a 30 s vertical from today's top story")
  and cite exact spans.
- Every edit, human or agent, is one replayable op stream.

**Rendering placement (a design decision to record):** preview and export stay
in the browser tab (WebCodecs plus WebGL2), because that is where the GPU and
the shaders are. Agent-initiated renders while no window is open need either a
headless Chromium on the computer, or a server-side path: ffmpeg in the guest,
for a restricted op subset of cuts, fades, text and basic grade. Recommend:
start with "render requires an open Cut window (or a headless page) and runs
`exporter.js`". Add a minimal ffmpeg path only if needed. Do not port the
shaders to ffmpeg.

### 1.6 Rebuild priority

**High.** It is the owner's lead example. The model, scene, compositor and
exporter are MIT, framework-free and tested, so they can be lifted. The
assistant is already a working app-agent verb schema.

Rough effort, Choir way:

- Svelte timeline and player on `AppSurface`, plus the ported model, engine and
  exporter, plus the op log, object-graph persistence and the Cut desk verbs
  (inspect, apply_ops, edit, preview_frame, render, branch): about 3 to 5 weeks
  for one strong agent-engineer.
- Transcript editing, captions and highlights: plus 1 to 2 weeks.
- Effect, retouch and avatar parity: open-ended, so skip it.

---

## 2. audio-studio (Adobe Audition-class editor)

### 2.1 What it is, stack, size

- A waveform plus multitrack audio editor: sample-accurate editing, a spectral
  display with brush and healing tools, a multitrack session with buses, sends,
  automation, punch and loop recording, an effects rack, restoration (noise
  print, NMF "sound remover", click and pop, hum, de-reverb), loudness analysis
  and matching, batch processing, MIDI control surfaces, and JS automation
  scripts (`README.md`).
- Stack: vanilla JS with Vite. Dependencies: `lucide` (ISC) and
  **`@breezystack/lamejs` 1.2.7, which is LGPL-3.0**.
  - It is lazy-loaded only for MP3 export (`src/export/mp3.js:33`).
  - It has an explicit exception in `tools/check-licenses.mjs`.
  - It is acceptable as a separately loaded chunk, but it is the one non-MIT,
    non-GPL, non-AGPL-family runtime dependency in this group. Flag it. Choir
    could drop MP3 export, or use the browser `MediaRecorder` and WebCodecs
    AAC and Opus instead.
- The DSP is a **Rust crate of about 21k lines** (`wasm/dsp/src`). It contains:
  - FFT, spectrogram, peaks, statistics, resampling (block and streaming), a
    limiter, **EBU R128 loudness** (`loudness.rs`), pitch detection, and a
    **FLAC encoder** (`flac.rs`, 1,229 lines);
  - `fx/` with delay, dynamics (including multiband), filter, modulation, pitch
    (`shifter.rs`, `offline.rs`), restore (`nmf.rs`, `click.rs`,
    `spectral.rs`), reverb (`convolution.rs`), special (`mastering.rs`,
    `shaper.rs`) and stereo.
  - The built `.wasm` is committed.
- Size: 499 files, about 74k lines of JS, 117 test files (16k lines) with
  **1,035 test cases**.
- Puter coupling sits behind `src/storage/puter.js` (wrapper calls such as
  `puter.readFile` and `puter.writeFile`), about 10 files.

### 2.2 Quality

Real engines:

- **`src/audio/channel.js` is a piece table over immutable `Float32Array`
  blocks.** Cut, paste and delete only rearrange references, so undo
  snapshots are almost free and multi-hour files edit without copying.
  Processing writes new blocks only for the affected range. This is an
  excellent design.
- `src/model/session-edits.js` holds pure state-to-state functions for the
  multitrack session ("never mutates its arguments"), with a fully documented
  state shape (frames at the session rate, dB, envelopes, fades with curve
  shapes, buses, sends and automation modes).
- `src/engine/mixcore.js` is shared between the AudioWorklet (realtime) and
  offline render, which is the same one-evaluator, two-sinks discipline as
  video.
- `src/commands/registry.js` is a **command registry of about 340 command ids**
  (`id`, `label`, `shortcut`, `enabled(app)`, `run(app, args)`). Every menu,
  shortcut and button runs a command by id, and there is a watcher hook.
- `src/commands/scripts.js` runs user JS with full app access through
  `new AsyncFunction('app','run','log','files', source)` after a confirm
  dialog. It is a local-file macro feature. It is acceptable as is, but **do
  not** carry this pattern into Choir. An external agent must go through
  commands, not eval.
- The NMF sound remover (`fx/restore/nmf.rs`) has a careful header: supervised
  KL-NMF, soft masks, WOLA resynthesis. This is real DSP, not a stub.
- The "Sound Assistant" (`src/sound-assistant/`) is an Audition "Essential
  Sound" clone. It maps audio types (dialogue, music, sfx, ambience) to effect
  chains, loudness targets, ducking keyframes and stretch. It is pure model plus
  ops, not an LLM.

Weaknesses: huge UI and docking surface (`editor/multitrack-view.js`, 1,962
lines; `ui/preferences.js`, 1,064 lines), and an Audition-faithful breadth that
Choir does not need.

### 2.3 Worth taking

1. **The Rust DSP crate** (`wasm/dsp`): loudness (R128), resampling, FLAC
   encoding, the NMF and spectral denoiser, click repair, convolution reverb, the
   limiter and mastering. This is the best reusable engine in the group, and it
   could also power video-studio's audio, since video-studio's own crate is
   only 161 lines. It also compiles to native Rust (`crate-type = ["cdylib",
   "rlib"]`), so a guest-side Go service could call it through a small
   CLI/FFI for agent-side processing with no browser.
2. `src/audio/channel.js`, the piece-table sample storage.
3. `src/model/session-edits.js`, the multitrack state shape and pure edits.
4. The pattern of `src/commands/registry.js`, where command ids with args are
   the agent verb surface. It is the same idea as Blender operators in 3d.
5. The Sound Assistant model (`sound-assistant/model.js`), as data: presets for
   podcast voice, broadcast loudness targets and ducking.

### 2.4 App-agent design ("Mix" desk)

**Objects:**

- **Recordings and source files**: `choir.audio_recording` / `choir.media_item`
  (content hash), with transcripts as `choir.transcript` (both kinds exist).
- **Waveform edits.** The piece table maps naturally onto content-addressed
  blobs. Each immutable block (65,536 frames) becomes a `choir.artifact_blob`.
  An edited file is then a small revision body listing `(block_hash, offset,
  length)` pieces, and versioning is nearly free in storage, because unchanged
  blocks dedupe. This is the cleanest object fit in the whole group.
- **Multitrack session**: an app-document head plus revisions (the same generic
  pair as Cut). The body is the session state from `session-edits.js`, and
  clips reference recordings by object id instead of pool `sourceId`. The media
  pool (today "not part of the undo state") becomes edges.
- **Effect chains and presets**: small content-hash objects. They are
  shareable, and they are publishable as presets on the Autopaper object graph.
- **Analysis results** (loudness, statistics, click and pop reports): evidence
  objects (`choir.agent_evidence`) linked to the recording revision.
- **Variants**: object-level branches (alternative mixes or masters), the same
  as video.

**Links:**

- An Autoradio run sheet (`choir.autoradio_run_sheet`, which exists) has a
  `derived_from` edge from the produced session. A podcast episode's mixdown is
  an artifact plus a publication.
- A Cut's audio track can reference the same recordings.
- A Texture transcript or document can cite time spans.

**Verbs:**

- `inspect`, `apply_ops` (the session edits),
  `process(recording_rev, range, effect_chain)`,
  `measure_loudness`/`match_loudness(target_lufs)`,
  `denoise(learn_from_range)`, `remove_silences`, `duck(music, against=dialogue)`,
  `mixdown(session_rev, format)`, `align_to_transcript`;
- direct manipulation of selection, fades, envelopes, clip moves and the
  spectral brush, all emitting the same ops.

**Desks it calls:**

- Research: licensed music or SFX lookup.
- Texture: scripts and show notes.
- Autoradio: production requests.

**Publish versus private:**

- Published: final masters (artifact plus provenance), presets, and transcripts
  of published shows.
- Private: raw recordings, takes, noise prints and learned sound models (these
  can be voice-identifying), and session history.

**Better as an agent:** batch restoration and leveling of a whole show to a
target LUFS while the owner is away, and producing Autoradio segments
end-to-end.

### 2.5 Rebuild priority

**Medium.**

- Take the DSP crate now, as a shared media engine for Cut and for Autoradio
  production on the guest.
- A full Audition-class editor is not a short-term need. A slim "Mix" view
  (multitrack session, loudness, denoise) over the same object model is.
- Effort: about 1 week to vendor and wrap the DSP crate (browser WASM plus a
  native CLI), and about 3 weeks for a slim multitrack Mix app with desk verbs.

---

## 3. 3d (a Blender clone)

### 3.1 What it is, stack, size

- A from-scratch TypeScript Blender workalike: modeling, sculpt, UV, shader,
  geometry and compositor nodes, animation and rigging, physics, a path tracer,
  a video sequencer, motion tracking, Grease Pencil, `bpy`-style scripting in JS
  and in Python via Pyodide, and importers and exporters for glTF/GLB, FBX,
  OBJ, STL, PLY and USD. It also imports Collada, Alembic and **.blend
  2.80–5.x** (`README.md`, `ARCHITECTURE.md`).
- Stack: three.js 0.186 (MIT), three-mesh-bvh, three-bvh-csg and
  three-gpu-pathtracer (all MIT, by gkjohnson), xatlas-web (MIT), Rapier3d
  (Apache-2.0), fzstd (MIT), opentype.js (MIT), and Pyodide (MPL-2.0, loaded
  at runtime **from cdn.jsdelivr.net**). Licenses are documented in
  `THIRD_PARTY_NOTICES.md`.
- Generated tables of Blender node and property names
  (`nodes/shader/data.gen.ts`, `io/blend/nodeprops.gen.ts`) are flagged there
  "for legal review" as interface facts. That is reasonable, but note it given
  Blender's GPL.
- No Puter coupling at all (one `puter.com` string).
- Size: about 533 files and **142k lines of TS**. There are only 55 test files
  (8.7k lines, **466 cases**), including 3 Playwright specs. There are **576
  registered operators** (`idname: 'mesh.subdivide'`, and so on).

### 3.2 Quality

- **Breadth over depth.** The test-to-code ratio is about 6% by lines, versus
  about 23% for video. There are zero TODO or FIXME markers in 142k lines, and
  975 lines over 180 characters. This is consistent with bulk generation.
- The architecture is coherent:
  - an RNA-named document of data-block tables keyed by id (`data/types.ts`);
  - copy-on-write typed arrays that are immutable after an undo push
    (`data/cow.ts`);
  - a depsgraph (`eval/depsgraph.ts`);
  - Blender-style operators with `poll`, `exec` and `invoke`, deterministic in
    their props so that "Adjust Last Operation" can re-run them.
- I did not check the numeric correctness of individual tools (bevel, boolean,
  NLA). Treat feature claims as unproven beyond the tested areas.
- Real, valuable pieces:
  - The **interchange IR** (`io/formats/ir.ts`, plus `to-document.ts`): a
    neutral scene representation with axis and unit conversion and BAKE versus
    ROOT modes. Readers and writers for glTF, FBX (binary), USD (USDA and USDC),
    OBJ, STL and PLY total about 11k lines, with tests against fixtures and
    `gltf-validator`.
  - A generic **.blend SDNA reader** (`io/blend/reader.ts`, `container.ts`)
    with decompression limits.
- Security notes:
  - The scripting runner uses `new Function` and a `with`/`eval` sloppy path
    (`scripting/runner.ts:181, 233`).
  - Pyodide is fetched from a CDN at runtime. Choir would need to vendor it or
    block it.

### 3.3 Worth taking

1. The `io/formats/` IR plus importers and exporters (glTF, FBX, USD, OBJ, STL,
   PLY). These are worth it on their own, even for a Files previewer or CAD
   interop. Coordinate with the CAD and parametric reviewers.
2. The **operator registry pattern** (`app/app.ts`, `registerOperators`):
   `idname` plus a typed `props` schema plus `poll` plus deterministic `exec`
   plus an automatic undo step. This is the cleanest agent verb surface in the
   group. `bpy.ops.<module>.<op>(props)` is exactly how an LLM should drive a 3D
   app, and `scripting/api.ts` already maps it.
3. `data/cow.ts` (copy-on-write typed arrays) as a pattern.
4. Everything else (the editors, the vdom, sculpt, physics, the path tracer) is
   not needed short-term.

### 3.4 App-agent design ("Scene" desk)

**Objects:**

- A scene is an app-document head plus revisions. The document's data-block
  tables (`objects`, `meshes`, `materials`, `node_groups`, `actions`, `images`)
  map cleanly onto **one object per data-block**. Their references (`ob.data`
  points to a mesh id, material slots, node-group usage) become `references`
  edges. The heavy typed arrays (positions, indices, uvs) become
  content-hashed `choir.artifact_blob`s, the same idea as the COW arrays and the
  `.p3d` binary section (`io/native.ts`).
- Versioning then gives per-data-block history, and identical meshes dedupe.
- A material or node group used in many scenes is one object, and it is
  **publishable on the Autopaper object graph as an asset library** (materials,
  node groups, models with licence and provenance).
- Variants (an alternative design) are object-level branches. The 3D
  sequencer's strips could reference Cut objects rather than duplicate the
  video model.
- **Where it does not map cleanly:** edit-mode state (the BMesh-like
  `MeshEdit`), modal tools, and the depsgraph's evaluated meshes. These are
  derived or ephemeral and must stay out of the graph. Physics caches are
  large, derived blobs that should be recomputed or cached, not versioned.

**Links:** a CAD or parametric part (another group's app) or an invoice line
references a model object. A Cut clip can be a render artifact of a scene
revision (`was_generated_by`). Research can attach reference images
(web_captures) to a scene.

**Verbs:**

- `inspect(scene)` (an outliner-style brief);
- `run_op(idname, props)` and `apply_ops([...])`;
- `import(file)` and `export(rev, format)`;
- `render(rev, camera, settings)`, which returns an artifact;
- `preview(rev, view)`, an image the agent uses for self-checks;
- `branch` and `merge`.

**Publish versus private:** published assets and renders go to the Autopaper
object graph with provenance. Work-in-progress scenes and history stay private.

**Better as an agent:** procedural and parametric generation from language
("a 3 m shelf with 5 shelves, oak"), batch format conversion, and render queues.

### 3.5 Rebuild priority

**Low** for the app, because it is far from Choir's near-term product.
**Medium** for lifting `io/formats` as an interchange library and the operator
registry as a pattern.

Effort: a minimal glTF viewer with an agent-driven scene document is about 2 to
3 weeks. Parity is not a goal.

---

## Group summary

**Top picks:**

1. **video-studio, high.** Lift `model/` (project, timeline, keyframes,
   speed-curve, compound), `engine/` (scene, compositor, shaders, exporter,
   frame-source, audio-mix) and `media/edit-assistant.js` as the verb-schema
   template. The rendering pipeline is **WebCodecs via Mediabunny, not
   ffmpeg.wasm**.
2. **The audio-studio Rust DSP crate, medium-high.** It is the shared media
   engine for Cut, Mix and Autoradio: R128 loudness, resampling, FLAC, NMF
   denoise, click repair, convolution reverb and mastering. It builds as both
   WASM and an rlib.
3. **3d `io/formats` IR plus the operator-registry pattern, medium as a
   library.** The app itself is low priority.

**Reusable pieces across the three apps (one design for all app agents):**

- **One evaluator, two sinks.** A pure `evaluate(doc, t)` feeds both live
  preview and offline export (video `engine/scene.js`, audio
  `engine/mixcore.js`).
- **Verb registry = agent surface.** Video's validated tool schemas, audio's
  command registry and 3d's operators converge on the same thing: an id, typed
  args, a validator or `poll`, a deterministic exec, and one undo step per
  batch. Choir should define this once for all app desks, as
  `apply_ops(doc, base_rev, ops[], label)` with conditional writes, and make
  direct manipulation emit the same ops.
- **Content-addressed immutable data.** Video hashes its media, audio's piece
  table uses immutable blocks, and 3d uses COW typed arrays. All three map onto
  `choir.media_item` and `choir.artifact_blob` plus small revision bodies.
- **A generic app-document head/revision kind pair**, modelled on
  `texture_document`/`texture_revision` with `revision_parent` and
  `derived_from` edges. Variants are object-level branches. Dolt branches are
  database-wide and reserved for computer candidates.

**Alarms:**

- Licensing:
  - lamejs is **LGPL-3.0** (audio, lazy MP3 export).
  - mediabunny is **MPL-2.0** (file-level copyleft). Fine unmodified.
  - Pyodide is loaded at runtime from jsdelivr (3d).
  - Blender-derived name tables in 3d are self-flagged for legal review.
- Security: audio (`commands/scripts.js`) and 3d (`scripting/runner.ts`) run
  arbitrary user JS with full app access through `new Function` and `eval`. Do
  not carry this pattern over. External agents should drive apps only through
  the op and verb registry.
- Quality: 3d is 142k lines with about 6% test coverage by lines, and is
  probably bulk-generated. Video and audio are well tested (1,372 and 1,035
  cases).
- Architecture: video-studio's edits are opaque closures with snapshot undo.
  Choir must add the serializable op log. That is the main design work, not a
  port.
