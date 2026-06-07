# News, Voice, And Autoradio Forward Plan

**Date:** 2026-06-06  
**Status:** planning doc / forward-looking architecture  
**Context:** Choir Base/Desktop execution is paused until the active
source-system streamlining work concludes. Use this window to sharpen the
news/source/voice/Autoradio plan and current docs.

## Document Role

This is the current forward-looking planning surface while source-system work is
active. It is not an implementation mission and should not be treated as a
checklist. Its job is to make the next mission family coherent: source/news
state, voice provider routing, Automatic Radio, private workflow radio, and the
screenless projection of the automatic computer.

When execution starts, author a fresh MissionGradient from current code and
staging state.

## Decision

Do not start Choir Base/Desktop implementation while source-system cleanup owns
the active CI/CD and staging lane. Instead, use the near-term planning window to
prepare the next product surface:

```text
source system
-> researcher source retrieval + web expansion
-> news/front-page app
-> source-grounded VText issues/briefs
-> newsletter/email projection
-> TTS readback
-> voice input
-> AI DJ / Autoradio queue
-> watch/mobile screenless control
```

Realtime voice models are not required for v0. Batch or nearline speech-to-text
and text-to-speech are enough to start.

The core Autoradio invention is **continuous relevance-walk playback over a
participation continuum**. Once the user starts a station from a broad or narrow
prompt, Choir should keep finding and performing new relevant material
indefinitely until the user interrupts, redirects, pauses, speaks, records,
publishes, or changes the station. The system should not yield after one answer
and force the user to immediately invent the next prompt.

User speech is content, not merely control input. A two-second command, a
two-minute take, a 45-minute monologue, or a four-hour self-recorded episode all
belong on the same continuum. Choir should be able to ingest user speech as a
durable artifact, preserve the full recording/transcript, optionally edit or
segment it, publish it, and make it retrievable by future agents and other
users according to policy.

## North Star

Choir's high-level product object is the **automatic computer**: an AI-native
cloud operating system. The automatic computer has:

```text
runtime and agents
source/news awareness
private/personal/domain data
VText and artifact memory
apps and publications
mail and notifications
media and voice surfaces
desktop/mobile projections
```

Once that computer has live knowledge of the public world plus the user's
private work context, it can be wrapped in a screenless voice interface without
becoming audio-only. Voice is one projection of the automatic computer.
Desktop, mobile web, VText, email, News, and File Provider are other
projections.

For a legal user, the private context is matter files, legal research, client
communications, drafts, citations, and privileged notes. For another domain, it
is that domain's private data and workflows. The product pattern is the same:

```text
automatic computer
+ public/current world model
+ private domain context
-> useful work
-> visual and screenless interaction
-> durable artifacts and publications
```

Mobile strategy follows platform reality:

- Mobile web remains the broad visual/mobile desktop surface.
- Native mobile apps should focus on allowed, high-value surfaces: screenless
  radio, voice capture/playback, notifications, share/open flows, and File
  Provider/Files-style integration where permitted.
- The full deep computer/desktop remains cloud/server-side or desktop-native,
  not an iOS/Android app-store-violating embedded software environment.

Desktop strategy:

- Web desktop remains the canonical product surface.
- Wails/native-looking Mac app can package the web desktop plus local services,
  File Provider, and eventually local computer capacity where feasible.

Automatic Radio is therefore not a side app. It is the screenless operating
surface of the automatic computer.

## Current Repo State

### Source / News

Current code-state note:

- [news-system-current-state-and-improvements-2026-06-06.md](news-system-current-state-and-improvements-2026-06-06.md)
  records the current source/news implementation and code-review findings.
  Treat its improvement ideas as rough input, not as accepted mission scope.

Existing docs already identify the target shape:

- [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)
  says the news system should land as a source ledger and issue manifest
  substrate, not as a standalone newspaper app bolted beside VText.
- [mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md)
  defines the clean standalone shape: registry, polite adapters, fetch audit,
  immutable source items, clusters, issue manifests, CLI/API/WebSocket, exports.
- [source-external-data-publication.md](source-external-data-publication.md)
  is the governing contract for source artifacts, publication, transclusion,
  and export.

Current code exists but is partial:

- `cmd/sourcecycled/main.go` runs a daemon with `/internal/source-service`
  health/search/item endpoints.
- `configs/sources.json` is the current source registry.
- `internal/sources` contains RSS, GDELT, Telegram, and source types.
- `internal/cycle` contains storage and synthesis scaffolding.
- `internal/runtime/tools_research.go` can call source-service search when
  `SOURCE_SERVICE_BASE_URL`, `SOURCE_SERVICE_URL`, or `SOURCECYCLED_API_URL`
  is configured.
- VText recognizes `source_service_item:<id>` references and can preserve them
  as source entities. The missing layer is a product workflow and News app, not
  the entire representation path.

Known gaps from prior docs remain important:

- sourcecycled is a useful v0 daemon, not the final product path;
- scheduling is still fixed at a global 15-minute loop even though sources have
  per-source cadence metadata;
- dedupe is process-local before storage and should not be treated as durable
  restart-proof "new item" accounting yet;
- issue synthesis does not yet prove exact rendered citation to source item
  mapping;
- there is no prominent News/Newspaper app surface;
- dedup/fetch/cycle/cluster/source-policy ledgers need hardening;
- source-service and VText source entities need a clearer product-level
  workflow boundary.

### Podcast / Radio Seed

The 2026-05-13 podcast/radio brief proof already proved a narrow but important
invariant; its durable lesson is now retained in
[old-docs-review-2026-06-06.md](old-docs-review-2026-06-06.md):

```text
podcast RSS feed
-> durable ContentItem
-> Podcast app listen path
-> VText radio brief
```

This is the correct topology. Autoradio should not be a separate audio bot. It
should be a traversal of VText/source/media artifacts.

### Existing Voice UI Hint

`frontend/src/lib/PromptSurface.svelte` already has an agent audio input button
surface. Treat that as a UI affordance seed, not proof that the voice pipeline
exists.

## Product Shape

### News / Newspaper App

The next visible product surface should be a prominent News/Newspaper app:

```text
source registry
-> fetch ledger
-> source items
-> researcher source_search hit
-> live web expansion/check
-> clusters/events
-> issue manifest
-> front page
-> story VTexts
-> source cards/transclusions
-> newsletter/email digest
-> radio queue
```

The News app should feel like a working front page, not a source-admin table,
but it must avoid the oracle model. Choir Global Wire should not merely
pronounce the synthesized truth. Its core story unit should expose the range of
claims, positions, evidence, uncertainty, and changes over time.

It should show:

- top stories;
- source confidence and gaps;
- why the story matters;
- what changed;
- contested claims;
- who claims what and how those claims differ;
- how the claim range changed across prior issues;
- exact lead sources plus a weighted source/context manifest;
- related VTexts/media;
- "listen" and "email digest" actions.

The ideal story object is closer to:

```text
story
-> claim set
-> source positions
-> supporting/refuting/qualifying evidence
-> uncertainty and missing evidence
-> weighted source/context manifest
-> timeline of claim/confidence changes
-> editorial synthesis
-> VText/story/publication refs
-> radio traversal edges
```

Editorial voice is allowed, but it must sit on top of visible evidence topology
and multiperspectival coverage.

The source model should not hide "background" material. A story may show only a
few lead citations inline, but every material source or context packet that
shaped the article should remain inspectable with role and weight:

```text
lead citation: directly supports this claim/sentence
supporting context: shaped framing, chronology, entities, or priors
contrary context: disputes or narrows the claim
correction/update source: changes prior issue state
ambient corpus context: summarized as counts/classes/recency/selection reason
```

This keeps the page readable without pretending only two cited sources informed
the article.

Source Service is the retrieval basis, not the retrieval ceiling. When a
researcher hits a relevant sourcecycled story, item, or cluster, the normal
path should also run `web_search` or another live external expansion path,
unless the task's evidence policy forbids external lookup. Sourcecycled gives
Choir durable owned source identity and a retrieval prior; web search catches
what the ledger missed, checks freshness, and broadens the claim range. The
result should record which claims came from the source ledger and which came
from live expansion.

### Newsletter / Email Projection

Email should be a projection of issue manifests, not a separate authoring stack:

```text
issue manifest
-> per-user digest policy
-> VText/email body
-> source references
-> approval or scheduled send
-> maild delivery
```

This connects the news system to existing maild/email work without making email
the source of truth.

Customized newsletters probably belong in the product set, but they should be
implemented as a news/story -> email-agent projection. The email agent drafts
and sends through the existing review/approval model; it does not own source
ranking, ingestion, provenance, or claim state.

### Autoradio

Autoradio is a queue, performance, production, notification, and contribution
layer over artifacts:

```text
source_search / source ledger story
-> live web expansion/check
-> weighted story/source manifest
VText segment
-> narrated summary
-> source excerpt
-> podcast/audio/video clip
-> contextual bridge
-> next item
```

The AI DJ does not need to stream speech directly at first. It can compose a
structured run sheet:

```text
beat_id
kind: narration | source_quote | podcast_clip | video_clip | transition | prompt
source_refs
script_text
media_ref
start/end offsets
voice_policy
duration_estimate
```

Then TTS renders narration and the player interleaves existing media.

The AI DJ inherits the same non-oracle constraint as the newspaper. Radio
segments should be able to say "here is the dominant claim, here is the
counterclaim, here is what changed, here is what remains unproven" rather than
flattening a contested topic into one authoritative answer.

Radio traversal should use the weighted source/context manifest too: lead
citations are good for quick readouts, while supporting, contrary, and update
sources give the DJ places to go deeper, broaden, correct, or shift
perspective.

The first continuous retrieval version can be simple: keep walking from the
current story to adjacent source items, related web-search results, source
updates, VText briefs, podcasts, videos, and user/private artifacts under the
same provenance and access-policy model. Audio playback can start by reading
stories and sources aloud, then interleaving podcast/video/audio sources when
the queue item already has playable media.

Automatic Radio combines four surfaces:

```text
listener surface:
  human-made podcasts/videos, VTexts, source excerpts, AI narration,
  clips, summaries, and transitions

production studio:
  user records takes, monologues, interviews, client updates, or commentary;
  AI DJ edits, segments, titles, mixes, and packages them

control pane:
  user directs the AI DJ: go deeper, broaden, clip that, summarize for client,
  cite sources, make this privileged/private, publish this, hold that

notification layer:
  AI DJ can interrupt playback/readout for priority events such as email,
  long-running agent completion, urgent source update, client reply, calendar
  event, or verification failure
```

The listener hears a fluid program. The operator sees and controls the same
program as structured text, queue items, source refs, clips, transcripts,
commands, and notifications.

### Participation Continuum

Autoradio should support every point between passive listening and active
publishing:

```text
passive listening for hours
-> occasional controls: pause, skip, go deeper, go broader, change topic
-> short spoken reactions
-> multi-minute user takes
-> guided interview / co-host mode
-> long monologue / self-recorded episode
-> edited segments
-> full publication
-> future retrieval by other users/agents
```

This changes the ontology of voice:

```text
voice command: changes station behavior
voice annotation: attaches a reaction or note to current item
voice take: becomes a citeable user artifact
voice episode: becomes long-form media and VText source material
voice correction: updates station memory or source interpretation
voice publication: becomes public or shared content
```

Prompts are also content. Even when a prompt is operational, it can carry a
perspective, claim, question, objection, or private context that should be
available to the user's future computer if policy permits.

The default should be conservative:

- short controls are processed as commands unless the user asks to save them;
- substantive speech can be offered as "save as note/take";
- long recordings are preserved as originals before any editing;
- derived clips and transcripts point back to the original recording;
- publication requires explicit user intent;
- private user speech is never used as public source material without policy.

User speech artifact path:

```text
audio recording
-> immutable media artifact
-> transcript artifact
-> speaker/time segments
-> optional VText draft
-> optional edited clips
-> optional publication
-> retrieval index with access policy
```

This turns Autoradio into a bidirectional medium. Sometimes the user listens
like a long podcast. Sometimes the user becomes the podcast. Most sessions sit
between those poles.

### Private Workflow Radio

Autoradio is not only a public news or consumer podcast surface. It must support
private internal workflows where most content cannot leave the tenant or
matter/project boundary.

Example: legal workflow radio.

```text
lawyer station
  -> private matter sources
  -> research agent updates
  -> cited legal VTexts
  -> AI DJ briefings
  -> lawyer spoken direction and takes
  -> client-safe short audio update
  -> client voice notes
  -> return notes to lawyer as transcript/source artifacts
```

In this mode, Automatic Radio is a conversational workflow interface:

- agents run in the background;
- the DJ reports meaningful progress and blockers;
- the lawyer speaks desired framing, questions, or objections;
- the system converts speech into notes, VText drafts, and task updates;
- a client-safe audio/text artifact can be produced from privileged internal
  work only through explicit policy;
- client replies become durable source artifacts scoped to the matter.

Private workflow requirements:

- every audio segment has access policy and provenance;
- internal/private/privileged/client-shareable states are distinct;
- source-grounded claims preserve citations through audio and text projections;
- AI DJ notifications respect urgency and privilege;
- publication/share steps require explicit user action;
- private client/law-firm material is never used as public radio content.

### Text And Voice Equivalence

Voice and written text are two projections of the same underlying artifact
graph. The product should not force a hard modality split.

```text
spoken take <-> transcript <-> VText note
AI narration script <-> generated audio
podcast/video clip <-> transcript segment <-> source excerpt
voice command <-> structured control event
radio queue <-> visual timeline / article / digest
notification readout <-> inbox/task/status item
```

Every audio behavior should have a visual/text representation:

- current segment and source refs;
- queue/frontier;
- transcript and generated script;
- user takes and saved notes;
- notifications and why they interrupted;
- available controls;
- publication/share status;
- privacy/access state.

This matters for users who prefer reading, for legal/professional workflows, for
accessibility, and for verification. Audio is the performance; text/VText is
the durable score and audit surface.

### Continuous Relevance-Walk Playback

The hard technical problem is not speaking one VText. It is maintaining an
endless, high-quality traversal over a topic space:

```text
seed prompt
-> topic frame
-> retrieval frontier
-> candidate artifacts/sources/media
-> relevance + novelty + depth scoring
-> run sheet segment
-> playback
-> user interruption/control signal
-> updated frontier
-> next segment
```

The station should work for both broad and narrow prompts:

- "Tell me what is going on in the world today."
- "Explain Apple File Provider."
- "Follow AI regulation in Europe."
- "Catch me up on my unread source queue."
- "Go deep on this VText and its sources."

The traversal should be able to move along two axes:

```text
breadth: adjacent topics, contrasting sources, related current events,
        prior context, who disagrees, what changed elsewhere

depth: primary sources, background explainers, technical details,
       source excerpts, long-form VTexts, podcasts/videos, historical context
```

The product behavior should feel like:

```text
Choir keeps going until interrupted.
The user steers or contributes by speaking, not by constantly re-prompting.
```

This requires a frontier model, not just a queue:

- consumed items;
- candidate items;
- unexplored branches;
- depth obligations;
- freshness obligations;
- source diversity;
- novelty budget;
- user corrections and skips;
- user speech artifacts and permissions;
- session memory;
- station policy.

The DJ should periodically replenish the frontier from Choir-native artifacts
and the web/source system. As Choir accumulates more private/public artifacts,
the frontier should shift from web-first to Choir-memory-first while still using
the web for freshness and gaps.

Failure modes to de-risk:

- looping over the same sources or claims;
- drifting away from the seed topic without user intent;
- staying too narrow and becoming repetitive;
- going broad in a shallow listicle way;
- hallucinating continuity when retrieval is weak;
- playing long media clips without enough context;
- yielding dead air or asking the user what to do next;
- hiding uncertainty rather than saying "I need fresher sources";
- ignoring user interrupts, skips, or "go deeper" commands.
- treating substantive user speech as disposable command text;
- publishing or indexing private speech without explicit policy;
- losing the original recording after clipping or summarizing;
- failing to distinguish user correction from user opinion;
- over-editing user speech into agent voice.
- notification spam that destroys listening flow;
- failing to distinguish control-pane direction from publishable content;
- leaking private workflow material into a client-safe or public artifact;
- making audio-only claims that cannot be inspected visually;
- producing client updates without source/provenance policy.

Minimum v0 proof:

```text
Given one seed prompt, generate and play at least 20 minutes of non-repeating
source-grounded segments by repeatedly retrieving, scoring, scripting, and
rendering the next segment, while accepting interrupt controls such as pause,
skip, go deeper, go broader, explain source, and change topic.
```

The proof can use batch STT/TTS and chunked generation. It does not need
low-latency full-duplex realtime audio.

Minimum contribution proof:

```text
During playback, accept a spoken user take, preserve the original audio,
transcribe it, attach it to the current station context, and offer "save as
private note", "draft as VText", or "publish/share" as explicit follow-up paths.
```

Minimum workflow-radio proof:

```text
Given a private matter/project with source artifacts and a background agent
update, produce a short private briefing, interrupt playback with the agent
update, accept a spoken user direction, and generate both an audio reply and a
visual/text transcript with source refs and access labels.
```

## Voice Strategy

### Routing Principle

Voice should be provider-routed by task, privacy, latency, hardware, and battery:

```text
desktop plugged in + Apple Silicon:
  prefer local STT/TTS where quality is good enough

mobile/watch:
  prefer server-side STT/TTS for battery and model size unless native Apple APIs
  are clearly sufficient

server without GPU:
  use hosted APIs for high-quality TTS/STT, or CPU-friendly local models for
  offline/batch jobs

private/sensitive content:
  prefer on-device or self-hosted routing when available
```

Realtime voice-to-voice is out of scope for v0. Use record/transcribe,
compose/respond, synthesize/play.

### Speech-To-Text Candidates

Primary candidates:

- **OpenAI API:** use current audio transcription models as high-quality hosted
  fallback. OpenAI has current speech-to-text and text-to-speech API models and
  also announced GPT-Realtime-Whisper for streaming, but v0 does not need
  realtime.
- **Whisper / whisper.cpp:** best local baseline, especially on Apple Silicon
  with Metal/Core ML acceleration. Good for desktop and offline batch tests.
- **faster-whisper / CTranslate2:** strong CPU/GPU server option with int8
  quantization; useful if a CPU-only Node B path is acceptable for batch or
  nearline jobs.
- **WhisperKit:** Apple/on-device ASR path worth testing for iOS/macOS native
  apps.
- **Moonshine:** promising low-latency/on-device ASR family for edge/mobile
  voice commands and short dictation.
- **Apple SFSpeechRecognizer:** native API fallback. It supports checking
  `supportsOnDeviceRecognition`, but Apple documents network usage, limits, and
  a one-minute recognition limit. Treat it as a device capability, not the whole
  strategy.
- **Deepgram Nova-3 / Flux:** hosted STT benchmark candidate for server/mobile
  routing if cost/quality/latency beat OpenAI for our use.

Recommended v0 tests:

```text
1. 15-second prompt-bar dictation.
2. 2-minute VText voice note.
3. 30-minute podcast episode transcription.
4. Noisy mobile recording.
5. Source quote with names/acronyms.
```

Measure:

- word error rate by rough human review;
- latency to first usable text;
- total wall time;
- CPU/RAM/battery on device where possible;
- cost per hour for hosted providers;
- punctuation/formatting quality;
- failure behavior.

### Text-To-Speech Candidates

Primary candidates:

- **Apple AVSpeechSynthesizer:** native, on-device, no server processing per
  Apple docs. Good first mobile/desktop fallback and low-cost readback.
- **OpenAI TTS:** good hosted baseline with `gpt-4o-mini-tts`/TTS models for
  high-quality narration.
- **ElevenLabs:** high-quality expressive hosted TTS, strong for polished radio
  voice and long-form narration, but cost/latency/vendor dependency must be
  measured.
- **Cartesia Sonic:** low-latency hosted TTS candidate with expressive controls.
- **Piper:** CPU-friendly local TTS for cheap/offline utility voice.
- **Kokoro ONNX:** promising local/browser/CPU TTS candidate for high quality
  at small model size.
- **Coqui XTTS / successors:** useful for experiments, but voice cloning and
  licensing need care.

Recommended v0 tests:

```text
1. Read one VText paragraph.
2. Read a 5-minute VText section.
3. Render a 20-minute issue digest.
4. Speak source citations and URLs gracefully.
5. Alternate narration with podcast/audio clip metadata.
```

Measure:

- time to first audio;
- total generation speed;
- intelligibility at 1.2x/1.5x playback;
- long-form consistency;
- pronunciation of names/acronyms;
- licensing/commercial use constraints;
- cost per hour;
- local CPU/battery impact.

## Suggested Near-Term Documentation Work

1. Keep
   [news-system-current-state-and-improvements-2026-06-06.md](news-system-current-state-and-improvements-2026-06-06.md)
   current as the source/news code-state note until a mission replaces it.
2. Create a News/Newspaper app spec that treats front page, issue manifests,
   story VTexts, newsletter projection, and radio queue as one product path.
3. Create a voice provider matrix with STT/TTS candidates, local/server/mobile
   routing, and test clips.
4. Create an Autoradio v0 spec with run sheet schema, DJ role, source policy,
   TTS policy, media clip policy, and watch/mobile controls.
5. Audit old docs later and archive/delete stale plans once these current docs
   are accepted.

## Defer Until Source Work Concludes

- Choir Base implementation.
- Wails desktop implementation.
- File Provider implementation.
- Apple Virtualization/vmctl implementation.
- Node B Base deployment.

These remain important, but they should be de-risked as the active main task,
not squeezed around source-system landing.

## Sources

- OpenAI audio API: https://platform.openai.com/docs/api-reference/audio/create
- OpenAI next-generation audio models: https://openai.com/index/introducing-our-next-generation-audio-models/
- OpenAI voice intelligence models: https://openai.com/index/advancing-voice-intelligence-with-new-models-in-the-api/
- Apple Speech framework: https://developer.apple.com/documentation/speech/sfspeechrecognizer
- Apple speech synthesis: https://developer.apple.com/documentation/AVFoundation/speech-synthesis
- Deepgram Nova-3: https://deepgram.com/learn/introducing-nova-3-speech-to-text-api
- Deepgram STT product: https://deepgram.com/product/speech-to-text
- ElevenLabs TTS API: https://elevenlabs.io/text-to-speech-api
- Cartesia docs: https://docs.cartesia.ai/
- whisper.cpp: https://github.com/ggml-org/whisper.cpp
- faster-whisper: https://github.com/SYSTRAN/faster-whisper
- WhisperKit paper: https://arxiv.org/abs/2507.10860
- Moonshine: https://github.com/moonshine-ai/moonshine
- Piper TTS: https://github.com/OHF-voice/piper1-gpl
