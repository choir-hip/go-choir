# Autoradio Programming Engine: Design

Date: 2026-10-14 (late evening). Status: design only. No product code was
written. Mutation class of this note: **green** (docs only). What it
proposes is mostly **orange** (a new app-agent desk, new product APIs, a
new client) with **red** edges (anything that sends mail, publishes a
take, approves self-development, or changes Texture's schema), classed
where each appears (§7.8).

Companion to `docs/reports/autoradio-mobile-app-plan-2026-10-14.md` (the
mobile plan). That plan decides the client stack; this note designs the
part the owner called the heart of it: the engine that decides what plays
next, and how a listener talks to it.

Evidence labels: **code** (read in this repository at `c3c9b4cf8`, with a
path), **doc** (a dated Choir document), **web** (a third-party page cited
by URL, fetched 2026-10-14), **UNVERIFIED** (not confirmed from a primary
source; test on the device), **inference** (reasoning, not tested),
**estimate** (a number derived from stated assumptions).

## The brief

The owner, 2026-10-14 about 21:40 UTC (mobile plan, "Owner rulings"),
quoted in full because it is the specification:

> Autoradio is entirely voice-driven; with every visual element removed it
> should work on an Apple Watch. Want the state of your computer? Say so.
> Turn it on without saying anything and it plays whatever is most
> intelligent to play: it learns the mix of media you like and weighs the
> importance and urgency of topics for you.
>
> The core mechanism: Autoradio traverses the user's personal object graph
> and the global Autopaper object graph to produce a continuous stream of
> audio — segments and full episodes of media made by and for humans
> (podcasts, videos), AI-voiced updates from written media (Autopaper),
> updates from the user's agentic email, and updates on the state of the
> user's work from the other desks. The user can interrupt at any time to
> ask or redirect; it learns from usage and follows up on topics of
> interest (e.g. a new Machine Learning Street Talk episode plays when it
> appears for someone interested in AI and tech business strategy).
>
> Interaction model: not conversational turn-taking. Long monologues in
> (the owner may speak for ten minutes), long listening out (10, 30, 80
> minutes), with interruption whenever wanted; passive listening or active
> talking, including performing — a take that may be sent to other people,
> optionally edited automatically before it goes out. Execution is
> everything.

Also binding, from the same rulings and earlier in the day:

- Takes are captured on the server, as private captures on the owner's own
  computer (ruling 6).
- The email app becomes a full desk app agent with its own RLM; every paid
  account gets an agentic address; **by default it sends nothing without
  approval**; it is included in Autoradio. External mailboxes are deferred.
- "I want my AI to be clearly sounding like AI, and then we're going to be
  mixing in human voices — that contrast is so much better."
  "Competitors that keep you listening to an AI all day are a worse
  experience." (doc: `docs/reports/pricing-and-economics-notes-2026-10-14.md`)
- "I hate thumbs up and thumbs down." Optimize importance, with attention
  as a weak signal; attention data stays on the reader's computer
  (doc: `docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`).
- Each app agent is its own RLM desk; one app-agent desk can have several
  front ends (doc: `docs/reports/owner-notes-2026-10-14.md` §5).
- "Auto radio quality is very execution dependent. I don't want slop."
  (doc: `docs/reports/choir-state-and-vision-2026-10-14.md` §2.11)

## Bottom line

1. **The programme is a rolling, versioned run sheet of typed segments,**
   not a playlist and not a chat. Ten segment kinds cover the brief:
   human media (whole episodes or clips, from the publisher's own audio),
   story updates and developing-event updates (AI voice, from Autopaper
   story versions and the claims ledger), mail updates, desk updates (work
   state), takes (your own and teammates'), answers (replies to what you
   said), alerts, and the connective tissue (bridges and idents). Every
   segment carries duration, source refs, provenance, importance, urgency,
   freshness, novelty for this listener, entities and topics, and the
   engine's own prediction of whether you will finish and value it (§1).
2. **Two loops, two places.** A slow **planner** on the user's computer,
   the **Autoradio desk** (an app-agent RLM desk, per owner note §5),
   builds a two-to-three-hour *horizon* of scored, scripted candidates,
   with a precommitment on each. A fast **scheduler** on the phone (and
   watch) picks the next segment at every boundary from that horizon plus
   public candidates, handles interrupts and plays offline. The phone
   never invents authority; the computer never sits in the audio path
   (§2.1, §7.2).
3. **Scoring is importance-first, engagement-weak.** The score is the
   predicted *value* of a segment for this listener now, combined with an
   importance floor (things that matter play even if they are not "fun"),
   urgency, freshness and novelty, under hour-clock constraints borrowed
   from radio's format clock: a target mix of media kinds, fatigue and
   repetition limits, a human-voice share, and the listener's time budget
   (§2.3, §2.4).
4. **Preemption is rationed.** Four urgency levels; only level 0 ("now")
   cuts in mid-segment, at a sentence or word boundary, after an earcon
   and a haptic. Every preemption is itself a prediction ("you will want
   this interruption") that the listener's reaction settles, and the
   engine has a budget of interruptions per hour (§2.5).
5. **Learning is PSA, not a recommender bolted on.** Each scheduled
   segment commits a typed prediction (finish, value, act) before it
   plays; implicit signals (skip and when, interrupt, replay, listen-
   through, voice acts such as "follow this", takes about the segment)
   settle it by rule; Brier scores and calibration per segment kind are
   views; the planner's context pack carries its own settled record. All
   of it lives on the user's computer; aggregates over private listening
   are never computed (§2.6, §2.7).
6. **Voice is not turn-taking. The programme is the reply channel.** You
   talk; the engine acknowledges with an earcon or one short line and
   answers by *inserting segments into the programme*, not by holding a
   conversation. Commands ("skip", "follow this") run on the device in
   under 300 ms. Questions, directives and takes go to the computer and
   come back as segments (§3).
7. **Listening modes:** push-to-talk by default (screen, mini-player,
   Action button, watch double tap, headphone gesture opt-in), a **talk
   session** ("open mic" for a monologue of any length, ended by silence
   or by saying so), and a wake word later. Always-listening is not the
   default: battery, bystanders and App Review all argue against it (§3.2).
8. **Barge-in works by ducking first and pausing second.** Voice
   processing (Apple's echo canceller) keeps the programme out of the
   microphone; voice activity ducks the programme within about 150 ms and
   pauses it at the next word boundary once speech is confirmed. Human
   audio rewinds a few seconds on resume; AI speech resumes at the
   interrupted sentence (§3.3).
9. **Monologues are first-class.** Ten minutes of speech is captured
   whole, transcribed in overlapping chunks while you speak (Parakeet TDT
   v3 with a custom vocabulary drawn from your object graph, so "Choir" is
   not heard as "Quire"), segmented into commands, questions, directives
   and takes, and routed. Consequential actions read back names and
   targets; approvals and sends never complete on voice alone (§3.4-§3.8).
10. **Takes and performances** are recorded as private captures on your
    computer, auto-edited non-destructively (filler and silence cuts from
    word timestamps, loudness to -16 LUFS, a title, a summary and the
    segment it answers), and routed to keep, team, Opinion draft,
    directive or email reply. Nothing leaves your computer without your
    approval. Your take stays in your voice: no AI re-voicing and no
    cloning (§4).
11. **Watch first means audio first.** Everything in this design works
    with no screen: voice in, audio out, a small haptic vocabulary, double
    tap to talk. The watch plays pre-rendered assets and records takes; it
    leaves transcription to the phone or the computer (§6).
12. **Cost is small and mostly shared** (estimate): model calls for
    planning, scripts and summaries are about $0.03-0.15 per listening
    hour per user; TTS on the device is free; story-update scripts are
    written once per story version and shared by every listener; human
    audio streams from the publisher. Data is the publisher audio (about
    30-60 MB an hour); AI segments travel as text (§8.2).
13. **Prior art** shows the gap. Spotify DJ, Google Daily Listen,
    NotebookLM's interactive mode, Huxe and Spotify's personal podcasts
    all generate or curate one-way audio and treat a voice as a request
    box or a chat. Huxe shut down in May 2026, the day after Spotify
    shipped the same feature. None of them mixes human media with AI
    updates from a provenance graph, or reports the state of your own
    agents, or turns your monologue into a routed take, or keeps a scored
    record of its own predictions about you (§9).
14. **The system briefing is a programme type of its own** (owner
    direction, §5): a 10-80 minute session that traverses the listener's
    own tape (goals, landed and pending work, open decisions, residuals,
    precommitment records), presents pathways as typed alternatives with
    probabilities and named predictors (PSA surfaced to the human), and
    explains concepts at the listener's level and just past it. The
    learner model is itself a precommitment record, settled by the
    listener's own questions and monologues; in teams each listener's
    model stays private on their own computer; monologues feed back as
    intent records. It is the expensive programme (about $0.30-0.60 per
    30 minutes, estimate) and the owner's highest-value one.
15. **Build order:** the M0 client ships the scheduler with public
    candidates and on-device commands; M1 adds the Autoradio desk v0 and
    desk-update segments ("what is my computer doing?"); M2 adds
    monologues, takes routing and per-segment precommitments; M3 adds the
    watch, podcasts as first-class sources and the learned mix. Teammates'
    takes and published audio Opinions wait for contributions and accounts
    (§10).

---

## 1. The programme model

### 1.1 Stream, segments, horizon

```text
  time ─────────────────────────────────────────────────────────────────▶
  ┌──────────┬───────────┬──────┬───────────────────────┬──────┬────────┐
  │ ident    │ story upd │bridge│ human: MLST episode   │ desk │ mail   │ ...
  │ AI 0:04  │ AI 1:10   │AI 0:06│ Human 1:12:40 (whole) │AI 0:40│AI 0:30 │
  └──────────┴───────────┴──────┴───────────────────────┴──────┴────────┘
  ▲ now playing  ▲ next N prefetched on device   ▲ horizon (2-3 h) from planner
                                               ▲ alert may preempt (level 0 only)
```

- **Segment:** the unit of playback, scheduling, prediction and signal.
  One audio item with one attribution: an episode, a clip, one AI-voiced
  update, one take. A segment is never a mixture of sources; a story
  update that mentions a podcast clip is two segments.
- **Programme:** the ordered sequence actually played. It is a projection
  of what the scheduler chose, recorded as events (§1.5).
- **Horizon:** the planner's ranked candidate set for the next two to
  three hours, with scripts written for AI segments and assets resolved for
  human ones. It is a *candidate pool with a suggested order*, not a fixed
  schedule; the phone may reorder within it (§2.1).
- **Run sheet:** the persisted horizon plus what played. The object kind
  already exists and is reserved for this: `choir.autoradio_run_sheet`
  (code: `internal/objectgraph/registry.go:76`, Dolt-backed, versioned,
  `Reserved: true`, no writer yet).

### 1.2 Segment kinds

| Kind | Source graph | Voice | Typical length | Example | Writer of the content |
|---|---|---|---|---|---|
| `human_episode` | Autopaper object graph (a podcast or video source in the registry, its episode captured as metadata, later a transcript capture) | **Human**, publisher's audio | 20-120 min | A new Machine Learning Street Talk episode, whole | The publisher; Choir only selects and attributes |
| `human_clip` | Same, with a `timestamp_range` into a transcript capture | **Human** | 30 s - 4 min | Two minutes of a Hard Fork episode where the hosts discuss the story just read | The publisher; Choir selects the range |
| `story_update` | Autopaper: a published story at a pinned version; the claims ledger for what changed | **AI** | 40 s - 3 min | "The Taiwan export story moved to version 4: TSMC now says..." | Autopaper's shared pipeline writes the script once per story version (§2.2) |
| `developing_event` | Autopaper: a `choir.world_event` with new claims since the listener last heard it (`develops`, `updates`, `contradicts` edges) | **AI** | 20-60 s | "Update on the Ukrenergo outage: the ministry raised its figure from 10 to 25 regions; Reuters and Ukrainska Pravda both carry it." | Shared, per event delta |
| `mail_update` | Personal: the email desk's inbox records | **AI** | 15-90 s | "Two new messages. Your accountant needs the signed form by Friday; I drafted a reply, it is waiting for you." | The email desk (summary and urgency); never sends |
| `desk_update` | Personal: the supervision projection (approvals, obligations, open and settled commitments, running work) | **AI** | 20-90 s | "Engineering froze the drain-call capsule. All fourteen verifiers passed. It predicted the tests pass at 0.8. It needs your approval on your phone." | The Autoradio desk, from records it reads; the facts are the ledger's |
| `take` | Personal or team: a take capture (yours, or a teammate's shared with you) | **Human** (the speaker's own voice) | 10 s - 10 min | Your colleague's 90-second reply to your take on the Fed story | The speaker |
| `answer` | Either graph, in response to something you said | **AI** (with human clips inside as separate segments) | 10 s - 3 min | "You asked why the drain took four rounds. Management's record says..." | The Autoradio desk, citing records |
| `alert` | Either: something urgent | **AI**, with a distinct earcon | 5-20 s | "Interrupting: engineering's capsule failed and rolled back." | The Autoradio desk; triggered by a desk or email-desk urgency judgment |
| `bridge` / `ident` | None (connective tissue) | **AI** or a sonic ident | 2-10 s | "Next, from Business Daily by BBC World Service." | Template first, model only where a template cannot say it |

The **system briefing** (§5) adds five kinds, used mostly inside that
programme type: `state_brief` (where the project stands, with evidence
labels), `pathways` (typed alternatives with probabilities), `concept` (an
explanation pitched by the learner model), `check` (an optional
invitation to think, answered whenever the listener next talks), and
`playback` (the listener's own earlier take, in their voice). Briefing
segments carry two more metadata fields: `concept_refs` and
`pitched_level`, with the learner predictions in `prediction` (§5.4).

Later kinds, named so the schema leaves room: `box_score_line` (a track
record read as a record, never a verdict; rules in the claims design §8:
spoken only when n meets the bar, scorer named, never a characterization
of a person), `music_bed` (only licensed or original), `team_meeting` (an
asynchronous meeting programme assembled from a team's objects, state and
vision §2.11).

Two kinds the brief named deserve a note:

- **Video.** A video's audio is played only where the terms allow it.
  YouTube's terms forbid splitting audio from video (doc: media and rights
  §4), so a YouTube-only show is a `human_*` segment **only when the same
  show publishes a podcast feed**; otherwise the engine says "there is a
  video on this; it's on your phone" and puts it in the Library. On the
  phone with the screen on, the official embed can play it.
- **Full episodes versus clips.** Owner example: a new MLST episode
  "plays when it appears". The default for a followed show is the whole
  episode, introduced in one line and resumable across days (the existing
  `/api/media/progress` stores position; code:
  `internal/mediastate/handler.go`). Clips are for shows you do not follow,
  attached to a story they illuminate.

### 1.3 Segment metadata

A sketch of the record, written as the planner emits it into the horizon
(JSON for readability; the real type would live in `internal/types`):

```json
{
  "segment_id": "seg:01JA...",
  "kind": "story_update",
  "voice": "ai",
  "duration_ms": 71000,
  "duration_basis": "script_words_at_160wpm",
  "source_refs": [
    {"ref": "publication:taiwan-export-controls@v4", "role": "subject"},
    {"ref": "claim:<extraction>#c12", "role": "cited"}
  ],
  "provenance": {
    "script_by": "autopaper.shared_scripts", "model": "deepseek-v4.1-flash",
    "script_hash": "sha256:...", "story_version": 4, "previous_heard_version": 2
  },
  "asset": {"type": "script", "text_ref": "blob:sha256:...", "lang": "en"},
  "entities": ["wd:Q713418", "wd:Q865"],
  "topics": ["topic:ai-chip-export-controls"],
  "section": "tech_ai",
  "importance": 0.72,
  "urgency": 2,
  "freshness": {"published_at": "2026-10-14T19:02Z", "half_life_h": 12},
  "novelty": 0.81,
  "follow_reason": "followed_event",
  "prediction": {
    "record_id": "commit:autoradio:...",
    "p_finish": 0.74, "p_value": 0.55, "p_act": 0.08,
    "expected_value_score": 0.61
  },
  "attribution_line": "From Choir's paper, story version four.",
  "interruptible_at": "sentence",
  "expires_at": "2026-10-15T07:00Z"
}
```

| Field | Meaning | Who sets it |
|---|---|---|
| `duration_ms` | Exact for audio files (enclosure length or HEAD/feed duration), estimated for scripts (words / speaking rate; corrected after rendering) | planner; device corrects |
| `source_refs` | Object-graph refs with a role; every spoken fact traces to one | planner |
| `provenance` | Who wrote the script, with which model, from which version; for human media, the feed, enclosure URL and publisher | planner |
| `importance` | How much this matters in the world or in your work, independent of you: for public items, Autopaper's front-page importance (LF0 Jev scores today; later the scored importance prediction); for personal items, the materiality of the record (supervision projection) or the email desk's urgency judgment | shared pipeline / desk |
| `urgency` | 0 now, 1 at the next boundary, 2 within the hour, 3 whenever (§2.5) | planner, from source judgments |
| `freshness` | When the underlying thing happened or changed, and its decay half-life by kind (developing events hours; analysis episodes weeks) | planner |
| `novelty` | How much of this is new *to this listener*: 1 minus the overlap with what they already heard (story versions heard, claims already spoken, episodes started). The prototype's "you heard version 2" generalized to claims | planner, from the listening log |
| `entities`, `topics`, `section` | Wikidata-keyed entities, standing topics, the four Autopaper sections | shared pipeline / claims layer |
| `follow_reason` | Why this is a candidate: followed show, followed story or event, followed entity, front-page importance, open work, inbox, explicit request | planner |
| `prediction` | The precommitment (§2.7) | planner |
| `interruptible_at` | Where preemption may cut: `word`, `sentence`, `segment` | by kind (human media: `segment` unless level 0) |
| `expires_at` | After this the segment is stale and must be re-planned (a story update superseded by version 5) | planner |

### 1.4 Transitions, attribution and the AI voice

Rules, in order of importance:

1. **Every human segment is introduced by name, in the AI voice, before it
   plays:** show, publisher, and for clips the episode and the minute. "From
   Business Daily, by BBC World Service, eleven minutes into Tuesday's
   episode." This is the prototype's rule (doc: prototype report) and the
   media plan's attribution requirement.
2. **Every AI segment names what it is reading from:** "Choir's paper,
   version four", "your inbox", "engineering's last report". The listener
   can always ask "where is that from?" and get the refs (§3.7).
3. **The AI voice is one fixed, clearly synthetic voice** with a light,
   constant treatment (the mobile plan §4.2: Kokoro with band-limiting, or
   a deliberately machine voice at M0). A short sonic ident marks the
   switch to a human voice, so the contrast is audible with eyes closed.
4. **Bridges are short and templated.** A bridge says what comes next and
   why it is here, in under ten seconds: "You follow MLST; a new episode
   came out this morning. Seventy-two minutes. Starting it." A model writes
   a bridge only when the *reason* needs words ("this connects to the
   export story you skipped yesterday").
5. **No filler, no banter, no fake hosts.** The owner's principle: the AI
   is the announcer and the reporter, the humans are the conversation. No
   two-host AI banter (NotebookLM's format) and no personality patter
   (Spotify DJ's). Silence between segments is under one second; there is
   no dead air and no padding.
6. **Numbers and names are spoken from structured fields, never
   paraphrased.** A figure comes from the claim's quantity field; a name
   from the entity's label; this is where synthetic voices most often
   mislead (inference).
7. **Loudness is normalized across the programme** to the spoken-word
   target of about -16 LUFS integrated, -1 dBTP (web:
   https://auphonic.com/blog/2019/06/12/the-new-loudness-target-war/;
   Apple's figure via secondary sources, UNVERIFIED from Apple itself), so
   a quiet publisher episode after a TTS segment does not drop away.

Sketch of ninety seconds of programme:

```text
[ident: two-note AI tone]
AI   It's 7:40. Three things changed overnight, then a new MLST.
AI   The Taiwan export story is at version four; you last heard version two.
     TSMC now says the new rules cover its Arizona fab. Reuters and Nikkei
     both carry it; the commerce department has not commented.
AI   Nikkei's tech podcast discussed this yesterday. Two minutes, from
     twenty-three minutes in.
[human ident: warm chime]
HUMAN (publisher audio, 2:00)
AI   Your computer: engineering finished the drain-call work overnight and
     is waiting for your approval. It's on your phone when you want it.
AI   New from Machine Learning Street Talk: an interview on test-time
     compute, seventy-two minutes. Starting it.
[human ident]
HUMAN (whole episode)
```

### 1.5 What goes on the tape

The memo that first sketched the DJ desk had the right shape: "Content
flows OG → queue → output; accountability flows action → tape. The tape is
backwards-looking; the DJ never plays the tape" (doc:
`docs/memo-diagonal-media-strategy-2026-09-17.md`).

- **Planned:** each horizon is a new version of the run sheet object,
  content-addressed; each segment's prediction is a commitment record.
- **Played:** the device reports what played as signal events (§2.6): one
  event per segment start and end, with position and reason. These are
  the listener's private attention data; they settle the predictions and
  live only on the listener's computer.
- **Said:** utterances are captures (voice provenance); what the engine
  did with them (command run, question answered by segment X, directive
  sent as record Y) is on the tape as ordinary records.

---

## 2. Selection and scheduling

### 2.1 Two loops

```text
                    the user's computer (guest)                       phone / watch
  ┌────────────────────────────────────────────────────┐   ┌───────────────────────────────┐
  │ AUTORADIO DESK (RLM, app agent)  — slow loop        │   │ SCHEDULER (Swift) — fast loop │
  │  every 20-60 min, or on a new urgent record, or    │   │  at every segment boundary    │
  │  when the device's horizon runs low:               │   │  and on every interrupt:      │
  │   1 generate candidates (both graphs)   §2.2       │   │   pick next from horizon      │
  │   2 score, write scripts, commit predictions §2.3  ├──▶│   + public candidates scored  │
  │   3 publish horizon (run sheet version)            │   │     with the exported taste   │
  │   4 settle last horizon's predictions   §2.7       │◀──┤     profile                   │
  │  reads: listening log, supervision projection,     │   │   enforce hour clock    §2.4  │
  │  email desk records, followed graph, packs         │   │   preempt (level 0)     §2.5  │
  └────────────────────────────────────────────────────┘   │   log signals; play offline   │
                 ▲ wakes on intent only (§7.5)              └───────────────────────────────┘
  host (Node B): public Autopaper candidates and shared story-update scripts and audio,
  served from Store B without waking any computer; supervision summary (counts, urgent flags)
```

Why two loops (inference, argued in §7.2):

- **The audio path cannot wait for a model, or for a computer to wake.**
  A segment boundary needs the next item in milliseconds. A hibernated
  computer takes seconds to resume and about 10.5 s to cold boot (doc:
  `docs/learn/04-computer-lifecycle.md` §2.3).
- **Planning needs the private graph and judgment.** Deciding that the
  drain-call approval matters more than a sports story, or writing a mail
  summary, needs the user's computer, its records and a model.
- **The listener's computer must not stay awake for the listener's whole
  commute.** A phone holding a socket to the guest keeps it from
  hibernating (doc: mobile plan §8.3). The planner wakes the computer for a
  few minutes per horizon, not for the listening hour.

### 2.2 Candidate generation

Each source contributes candidates with a `follow_reason`. The queries are
shaped so they are seekable by the retrieval ladder's indexes
(doc: `docs/reports/object-graph-retrieval-research-2026-10-14.md` §5),
not window scans.

| Source | Graph | Trigger and query shape | Candidates |
|---|---|---|---|
| Followed shows | Autopaper (podcast sources in the registry) | New episode since the listener's cursor for each followed source: `(source_id, published_at)` | `human_episode` |
| Front-page importance | Autopaper | Top-N stories by importance and depth (LF0/LF1) the listener has not heard at their current version | `story_update` |
| Followed stories and events | Autopaper (claims ledger) | New version of a followed story; new claims on a followed `choir.world_event` since the listener's last heard claim: `(event_id, fetched_at)` | `story_update`, `developing_event` |
| Followed entities and topics | Autopaper | New claims mentioning a followed entity or topic: `(entity_id, fetched_at)`; new episodes whose transcript mentions it (later) | `developing_event`, `human_clip` |
| Clips for a story | Autopaper | Transcript captures whose timed blocks share entities or propositions with the story; ranked by a Jev "illuminates this story?" judgment. Lexical matching is too crude (prototype finding: an AI data-centre story matched an AI-regulation episode on words alone) | `human_clip` |
| Inbox | Personal (email desk) | Email desk records since the cursor, with the desk's urgency and a summary | `mail_update` |
| Work state | Personal (supervision projection) | Approvals awaiting decision, open obligations, newly settled commitments, finished or failed runs, ordered by materiality (doc: `docs/precommitment-records.md` §4.7) | `desk_update`, `alert` |
| Takes | Personal / team | Teammates' takes shared with the listener (LC/LA); replies to the listener's takes | `take` |
| Requests | Either | Anything the listener asked for ("more on this", "what happened with...") becomes candidates with the highest personal weight | `answer` and any kind |
| Exploration | Autopaper | A small share of candidates from outside the listener's profile: high-importance stories in sections they rarely hear, shows adjacent to followed shows (§2.6) | any public kind |

**Shared scripts.** Story-update and developing-event scripts depend on the
story version and the claims delta, not on the listener. They are written
once per (story, version) or (event, claim-window) in Autopaper's shared
pipeline and published beside the story, so every listener's computer
adopts the same script and the host can pre-render it once (§7.1). Only the
personal part ("you last heard version two"; "you follow TSMC") is added
by the listener's planner, as a templated preamble. This is the pricing
notes' amortization ("Autopaper's inference amortizes across users")
applied to audio.

**Personal media is personal.** Mail and desk candidates never leave the
user's computer except as the rendered segment sent to the user's own
devices.

### 2.3 Scoring

For candidate *s*, listener *u*, context *c* (time of day, device, route
such as car or headphones, how long they have been listening, what just
played):

```text
score(s) = V(s | u, c)                        predicted value for this listener now
         × F(freshness, kind)                  decay by half-life per kind
         × N(novelty)                          discount what they already know
         + λ_imp · max(0, importance − floor)  importance lift, independent of taste
         + U(urgency)                          urgency bonus (levels 1-3; level 0 preempts)
         + E(s)                                exploration bonus (bounded share, §2.6)
subject to the hour clock (§2.4)
```

- **V is the learned part.** It predicts value, not clicks: p(finish) times
  the listener's per-kind completion weight, plus p(act) (follow, more on
  this, a take, a save) weighted highest, minus p(early skip). At cold
  start, V is a hand-set prior per kind and section (§2.9). Later it is a
  small per-listener model (logistic regression or gradient-boosted trees
  over a few dozen features: kind, show, section, entities followed,
  length, time of day, position in session, novelty, importance), trained
  on the listener's own settled predictions on their own computer
  (inference: small models suffice for one listener's data; deep sequence
  models such as the skip-prediction challenge entries need millions of
  sessions; web: https://www.aicrowd.com/challenges/spotify-sequential-skip-prediction-challenge).
- **The importance term keeps the station honest.** The owner's objective
  for Autopaper is importance, with attention weighted low. A segment
  whose importance is above the floor gets airtime even if V says the
  listener would rather hear another tech podcast. The floor and λ_imp are
  listener settings in words ("keep me on the big stories" versus "only my
  interests"), not sliders.
- **Personal importance is materiality.** For desk and mail candidates,
  importance is the supervision projection's materiality and the email
  desk's urgency judgment, not a guess by the Autoradio desk.
- **Explicit standing intents dominate learned taste.** "I want to keep up
  with AI and tech business strategy" is recorded as a standing intent on
  the computer and enters as a hard lift on the matching topics and
  shows; learned taste fills the rest. A standing intent survives a crash
  (the restart rule's "standing intents survive; occurrences do not").

### 2.4 The hour clock: mix, fatigue, diversity, time budget

Radio solved "what next" under constraints long before recommenders: the
**format clock**, a template for each hour listing which kind of segment
goes in which slot (web: https://en.wikipedia.org/wiki/Broadcast_clock).
Autoradio keeps the idea and makes the clock learned and personal.

| Constraint | Default (learned per listener) | Why |
|---|---|---|
| Human-voice share | ≥ 50% of listening time | The owner's contrast principle: AI is the announcer, humans are the conversation |
| AI run length | ≤ 4 min of consecutive AI voice before a human segment, a take or a pause point | Fatigue; "keep you listening to an AI all day" is the competitor failure the owner named |
| Personal block | Mail and desk updates batched into one or two "your computer" blocks per hour, except urgency ≤ 1 | Context switching is costly; batching is how humans read mail |
| Same-topic spacing | ≥ 15 min between segments on the same event unless the listener asked | Repetition fatigue |
| Same-show spacing | No two episodes of one show back to back unless binge mode | Diversity |
| Section mix | Proportional to the listener's learned section weights, with the exploration share on top | Private personalization (LF1's section counting, generalized) |
| Time budget | If the listener said "I have 25 minutes" or the context implies it (a calendar is out of scope; a learned commute length is in), fill to the budget and finish whole segments inside it | Long-form shows must not start with three minutes left |
| Long-form slots | A whole episode only when the remaining budget covers at least half of it, or the listener resumes it by habit | Episodes are resumable; starting is a commitment |

**Algorithm (inference):** greedy fill with one-step lookahead. At each
boundary the scheduler takes the highest-scoring candidate that keeps every
constraint satisfiable for the rest of the budget; ties go to the
candidate with the lower `expires_at`. The planner runs the same rules over
the whole horizon to produce the suggested order, so the device usually
just takes the next item. A knapsack or integer program is unnecessary at
the scale of 20-60 segments an hour, and its results would be harder to
explain aloud ("why did you play this?").

### 2.5 Urgency and preemption

| Level | Meaning | Where it cuts in | Who may raise it | Budget |
|---|---|---|---|---|
| 0 **now** | Waiting harms the listener: a self-development operation failed and rolled back, a deadline in the next hour, a teammate's take marked urgent and the listener allowed that | At the next word boundary of AI speech, the next sentence boundary of human speech (from transcript timestamps, or a 1.5 s silence detector) | Desks and the email desk by typed judgment; the Autoradio desk confirms against the listener's policy | ≤ 2 per hour by default; a third becomes level 1 |
| 1 **next** | Should be heard soon: an approval waiting, a followed story's major development | At the end of the current segment; long human segments (over 20 min) are paused at a sentence boundary after 10 min of waiting | Same | ≤ 4 per hour |
| 2 **this hour** | In the next personal block | Scheduled | Planner | none |
| 3 **whenever** | Normal candidate | Scheduled | Planner | none |

- **Earcon and haptic first.** A level-0 cut-in is preceded by a distinct
  two-tone earcon and, on the watch, a distinct haptic; the listener can
  say "later" (or press the talk control twice) to defer it to level 1.
- **Every preemption is a prediction.** The engine commits "the listener
  will engage with this alert (not defer, not skip within 5 s)". Deferred
  or skipped alerts settle as misses, and the source's urgency judgments
  are calibrated against them, per source. A desk whose alerts are
  routinely deferred has its level-0 rights narrowed automatically (to
  level 1) and the change is a record the owner can see.
- **Urgency reaches a listening phone without waking anything.** The
  computer writes urgent counts to the host-side supervision summary when
  the event happens (mobile plan §8.3); the playing phone polls the host
  (cheap, no guest wake) or receives a push. The phone then fetches the
  alert segment, which may wake the computer: that wake is on intent.

### 2.6 Learning from implicit signals

The listener never rates anything. The signals, their meanings and their
traps:

| Signal | Captured | Reading | Trap |
|---|---|---|---|
| Listen-through | segment end reached, or ≥ 90% | positive for value | Passive listening (phone in pocket) also finishes; weight by whether the listener acted at all in the session |
| Early skip (< 10 s) | skip with position | strong negative for this item, weak for kind | Skipped because already heard elsewhere: novelty, not taste. Ask the novelty model first |
| Late skip | skip after 10 s | negative for length or pacing more than topic | |
| Replay / back 30 | rewind | strong positive (or the audio was unclear: check route and noise) | |
| Pause and resume | pause duration | neutral; a long pause on a human episode means "save for later", resume tomorrow | |
| Interrupt to talk about it | a take or question during or right after | **strongest positive for topic**, regardless of agreement | An angry take is engagement, not endorsement; it still means "this matters to me" |
| "Follow this", "more on this" | voice command | explicit positive; creates a standing follow | |
| "Less like this", "mute X" | voice command | explicit negative; mutes a topic, show or source (with an optional spoken reason) | Over-broad mutes: confirm the scope in one line ("Muting crypto stories, not markets. Right?") |
| "Why are you playing this?" | voice question | uncertainty; answered from `follow_reason` and the score terms | Frequent "why" means the explanations are wrong, not the taste |
| Session end | stop, headphones out | the last segment before a stop gets a weak negative, discounted when the stop matches the learned commute end | |

**Explore, exploit, explain.** The closest prior art is Spotify's BaRT, a
contextual bandit that learns satisfaction for (item, explanation, context)
and logs propensities so it can retrain off-policy (web: McInerney et al.,
RecSys 2018, https://patents.google.com/patent/US11301513). Autoradio
borrows three things: a bounded exploration share (default 10% of
candidates, never at level 0-1, never in a time-budgeted slot), logging the
probability with which each candidate was chosen so learning is unbiased,
and **an explanation spoken with every exploratory pick** ("Something
outside your usual: a story the paper ranks highly in World"). It does not
borrow the objective (streams and engagement).

### 2.7 The precommitment loop

The engine is a desk, so it is held to the same rule as every desk: commit
to what you expect before the evidence arrives, then be scored by someone
else (doc: `docs/precommitment-records.md` §1.1, §5.1).

**What is committed.** For each segment placed in a horizon, one
`choir.commitment_record` (act `precommit`, emitter the Autoradio desk,
subject refs the segment and its sources), with typed questions:

| Question | Type | Resolves by (rule scorer) |
|---|---|---|
| Will the listener finish this segment (≥ 90%) if it plays? | yes/no with p | the device's end event |
| Will they skip it in the first 10 s? | yes/no with p | the skip event |
| Will they act on it (follow, more, take, save, reply)? | yes/no with p | voice acts within 10 min referencing the segment |
| Will it play at all in this horizon? | yes/no with p | the played log (measures the planner's ordering against the device's choices) |
| For alerts: will the listener engage rather than defer? | yes/no with p | defer or skip events |

Plus one record per horizon: "the listener will listen at least M minutes
of this horizon and skip at most K segments", the mix-level forecast.

**Settlement and scoring.** Settlement is by rule from the listening log
(the cheapest, most independent scorer in the precommitment design's
order: rules first). The desk never settles its own records. A segment
that never played resolves `unresolved`, not wrong; a session that a crash
or a dead battery ended resolves `interrupted` and is excluded from
calibration (precommitment design §5.4, "platform failure scored as
forecasting error"). Scores are views: Brier per question type, per
segment kind and per time of day; a reliability curve; sharpness beside
calibration, because a planner can look calibrated by predicting 0.6 for
everything (§5.3 there).

**Learning in context.** The Autoradio desk's acting pack carries
observations and discrepancies, not scorer judgments, per the decided
boundary (§4.7 there): "Last week you predicted 0.8 finish for long policy
episodes on weekday mornings; 3 of 11 finished; the listener skipped
after the intro in 6." That is the in-context reinforcement learning the
owner named as PSA's key value, applied to a taste model. Retrieval is by
corrective value: the records most worth showing are the confident misses.

**Why this is better than a black-box recommender** (inference):

- The listener can ask "how well do you know me?" and get a true answer:
  "This week I predicted 71% of what you'd finish; I'm overconfident about
  long interviews." That is a box score of the engine, scoped to one
  listener, shown to that listener only.
- The model of the listener is legible records, so "forget that I listened
  to that" is a deletion the next horizon honours, and a listener can
  read why the engine believes what it believes.
- Drift is visible: a planner that slides toward engagement bait will show
  rising p(finish) accuracy with falling p(act) and falling importance
  share. That is a Goodhart detector, not just a dashboard.

**Guards** (borrowing the design's table, §5.4 there):

| Failure | Guard |
|---|---|
| Optimizing finish rate by playing short, safe, familiar segments | Importance floor and exploration share are constraints, not score terms the planner may trade away; report sharpness and the share of airtime above the importance floor beside accuracy |
| Self-fulfilling predictions ("p(play) = 0.95" because the planner orders it first) | The "will it play" question measures the planner against the device only; it is excluded from taste calibration |
| Engagement as value | p(act) and explicit follows weigh more than finishing; takes count as value regardless of sentiment |
| Learning a scorer's quirks | The scorer is a rule over the device's events; there is no model scorer to game at this level |
| Private data leaking into shared aggregates | Settled predictions over private listening never leave the computer; no cross-user calibration is computed from them (state and vision §9 risk) |

### 2.8 Privacy: where attention lives

- **The listening log, the taste model and the settled predictions live on
  the user's computer** (its tape and object graph). The host sees only
  what any client request reveals (which public assets were fetched).
- **The device holds a copy** for offline scheduling: the exported taste
  profile (§7.3) is the user's own data on the user's own device.
- **Shared Autopaper ranking learns only from public signals** (coverage,
  velocity, the editorial importance judgment), never from listeners'
  private attention. If the owner later wants aggregate listening
  statistics to inform the shared paper, it is an opt-in, differentially
  private contribution, decided separately (open question 6).
- **Teammates do not see each other's listening.** A team programme sees
  only what was shared to the team.

### 2.9 Cold start

1. **Ask once, by voice, in one breath.** First run: "Tell me what you
   want to keep up with, as long as you like." The monologue becomes
   standing intents and seed follows (topics, shows, entities), confirmed
   in one read-back.
2. **Priors.** Front-page importance, the four sections at equal weight,
   human-voice share 50%, a handful of high-quality shows per section from
   a curated seed list (the prototype's five feeds generalized and moved
   into the source registry).
3. **Fast learning from strong signals.** Explicit follows and takes count
   immediately; finish and skip rates move V only after a few sessions.

---

## 3. Voice-driven interaction

### 3.1 The principle: the programme is the reply channel

Turn-taking assistants (Siri, Alexa, and full-duplex chat models such as
Kyutai's Moshi, which models both speakers as parallel streams for a
natural conversation; web: https://arxiv.org/abs/2410.00037) are built to
hold a dialogue. Autoradio is built to hold a **programme**: long audio
out, long speech in, and the two rarely alternate quickly. So:

- The listener speaks for as long as they want; the engine does not reply
  turn by turn.
- The engine acknowledges with an earcon (heard) and, where useful, one
  short line ("Got it: following TSMC." "Checking your computer.").
- The substance of a reply is a **segment inserted into the programme**,
  with the same attribution and provenance as any other. "Answer" is a
  segment kind, not a mode.
- Conversation is possible (ask, hear an answer segment, ask again), but
  it emerges from inserted segments; there is no chat state machine to
  get stuck in.

This is also why NotebookLM's "join" mode, where AI hosts pause, take your
question and resume (web: https://support.google.com/notebooklm/answer/16212820),
is the nearest prior art and still the wrong shape: it is a question box
attached to a generated show, with reported delays and voice glitches on
joining.

### 3.2 Listening modes

| Mode | How it starts and ends | Pros | Cons | Verdict |
|---|---|---|---|---|
| **Push-to-talk** | Hold or tap the Talk control: screen, mini-player, Action button (via App Intents), watch double tap (§6), Control Center. Release or tap to end | Unambiguous, no false triggers, cheap, App Review-friendly | Needs a hand (or a wrist) | **Default** |
| **Talk session ("open mic")** | Start by PTT or by voice ("let me talk"); mic stays open while the programme is paused or ducked; ends after a long silence (default 4 s, longer if the listener has a slow monologue style), or on "that's it" / "back to the programme" | Owner's ten-minute monologues; no holding a button for ten minutes | An open mic hears bystanders | **Default for monologues** |
| **Wake word** | A custom phrase ("Hey Autoradio" or the owner's choice) spotted on device while the programme plays | Hands-free in the car and kitchen | Third-party wake words run in the app's own audio session, so the app must hold `.playAndRecord` and the orange mic indicator the whole time; battery cost; false triggers from the programme itself (AEC helps); Picovoice Porcupine supports custom words and background use on iOS, "controlled by the operating system", commercial licence (web: https://picovoice.ai/docs/faq/porcupine/) | **Later (opt-in)**, after AEC and battery are measured |
| **Always listening** | Mic always open, VAD plus speaker verification decides what is addressed to the engine | Most natural | Privacy for bystanders, the orange indicator constantly on, App Review scrutiny of persistent background recording (web: https://developer.apple.com/forums/thread/776949 reports 2.5.4 rejections and appeals), battery; ambiguity between talking *to* the engine and talking *near* it | **No** |
| **Siri / App Intents** | "Hey Siri, tell Autoradio ..." | Free wake word, works on watch and CarPlay | Siri's turn-taking UI; Siri transcribes, not Parakeet | **Supported as an entry point**, never the main path |

A headphone gesture as Talk is worth an opt-in: Snipd maps a headphone tap
to "save this moment" (web: https://apps.apple.com/app/id1557206126), and a
remote command (for example the "previous track" triple press) could open
a talk session. It overloads a system control, so it stays off by default
(UNVERIFIED which remote commands AirPods expose on the current iOS).

### 3.3 Barge-in: ducking, echo cancellation, resuming

```text
 PLAYING ──VAD onset (mic, AEC on)──▶ DUCKED (-18 dB) ──speech ≥ 300 ms──▶ PAUSED at word/sentence
    ▲                                    │ no speech in 600 ms                 │ (talk session)
    │                                    ▼                                     ▼
    └──────────────── un-duck ◀──────── (false alarm)          RECORDING ── end ──▶ RESUME
                                                                               (AI: interrupted sentence;
                                                                                human: rewind 3-5 s)
```

- **Echo cancellation is the precondition** for barge-in on a speaker
  (phone on a table, car): without it the programme's own speech triggers
  VAD. Apple's voice processing I/O provides echo cancellation, noise
  suppression and gain control, and `AVAudioEngine` enables it on the input
  node; it also ducks other audio, with an "advanced ducking" mode that
  varies by voice activity (web: https://developer.apple.com/videos/play/wwdc2023/10235/).
  Because the programme is played *by the same engine*, the canceller has
  the reference signal (inference: this is the VoIP configuration it is
  designed for). One forum report says ducking starts as soon as the engine
  is initialized (same source's notes; UNVERIFIED), so the M0 spike must
  measure it.
- **With headphones** echo is mostly absent, but Bluetooth input forces the
  hands-free profile and drops playback quality (mobile plan §4.1); prefer
  the phone or watch microphone while AirPods play, and record from AirPods
  only when nothing else is available.
- **Voice activity**: Silero VAD (in FluidAudio) for onset; the Parakeet
  realtime end-of-utterance model (120M, median 160 ms EOU; mobile plan
  §4.1) to detect the end of a short command, so "skip" does not wait for
  four seconds of silence.
- **Ducking before pausing** avoids two failure modes: pausing on every
  cough (false alarms un-duck in 600 ms) and talking over the programme
  (confirmed speech pauses at the next boundary).
- **Resume rules**: AI speech resumes at the start of the interrupted
  sentence (the TTS unit is the sentence; prototype lesson); human audio
  rewinds 3-5 s (or to the start of the transcript sentence when a
  transcript exists); after a monologue longer than two minutes, a
  four-second bridge reminds the listener where they were ("Back to Hard
  Fork, on the chip rules").

### 3.4 Long monologues

```text
mic (16 kHz) ─▶ take file (whole, CAF/m4a, never lost) ─────────────────────────────┐
     │                                                                              │
     ├─▶ streaming EOU 120M ─▶ live partial text (watch/phone caption; commands)    │
     │                                                                              │
     └─▶ chunker: 30-60 s windows, 2 s overlap, cut at VAD silences                 │
              └─▶ Parakeet TDT v3 (batch, custom vocabulary) ─▶ chunk transcripts ──┤
                                                                                    ▼
                            stitch (word timestamps, overlap dedupe) ─▶ full transcript
                                                                                    │
       on device: command grammar pass (skip/follow/mute/...) ─ immediate          │
                                                                                    ▼
       on the computer: Autoradio desk segments the transcript into units ─▶ route (§3.5)
```

- **Transcribe while speaking.** Batch Parakeet per chunk means a
  ten-minute monologue is fully transcribed seconds after it ends instead
  of after a ten-minute pass (inference; depends on the iPhone real-time
  factor, which the M0 spike measures; FluidAudio reports ~150x real time
  on an M5 Pro Mac and no iPhone figure; mobile plan §4.1).
- **Custom vocabulary only works in batch.** FluidAudio's vocabulary
  boosting runs a second, 110M CTC keyword-spotting encoder beside the TDT
  encoder and is "batch mode only (Parakeet TDT)", not supported with the
  streaming EOU model (web: https://docs.fluidinference.com/asr/custom-vocabulary).
  So live partials may say "Quire"; the chunk transcripts say "Choir". The
  command grammar therefore matches on phonetic keys, not spelling (§3.8).
- **Segmentation into units** is a model job on the computer (the
  Autoradio desk; a Jev typed `choice` question per span is the cheap
  classifier, a stronger model for the summary). Units: a span of the
  transcript with a class, a target and a confidence. A monologue commonly
  contains several: "Skip this. Actually, on the Fed story, I think
  they're wrong because... [three minutes] ... and tell engineering to hold
  the drain call until I've read the diff. Also, what happened with the
  Mauritius riots?" is one command, one take, one directive and one
  question.
- **The audio stays whole.** Units cite time ranges into the one take
  capture; nothing is re-recorded or split destructively.

### 3.5 Utterance classes and routing

| Class | Examples | Where decided | Path | Latency target |
|---|---|---|---|---|
| **Command** (playback) | skip, back, pause, louder, faster, "play the whole episode", "stop in 20 minutes" | On device, a closed grammar over the EOU partials | Local | ≤ 300 ms after end of utterance |
| **Command** (preference) | follow this, more on this, less like this, mute crypto, "fewer AI voices" | On device recognizes; computer records the standing intent | Local effect now; record syncs | ≤ 300 ms ack; record on next sync |
| **Question** | "What's my computer doing?", "Why are you playing this?", "Where's that figure from?", "What happened with X?" | Computer (Autoradio desk), except "why this?" and "where from?" which the device answers from segment metadata | Insert an `answer` segment | Ack ≤ 1 s; answer audio ≤ 8 s when the computer is awake, ≤ 20 s from hibernation (inference) |
| **Directive** | "Tell engineering to hold the drain call", "Ask research to dig into the Mauritius story", "Draft a reply saying yes" | Computer: management receives a `directive` record (the existing delivery path; mobile plan §8.4); mail replies go to the email desk as a draft | A record to the desk; a confirmation line now; the outcome later as a `desk_update` | Ack ≤ 1 s; confirmation after read-back |
| **Take** | Opinion, analysis, a reaction: speech about content, not to the engine | Computer: the desk proposes a title, summary and destination | Private capture; a review step (§4) | Draft ready ≤ 30 s after the monologue ends |
| **Intent** | An idea about what the project should do or become: "Autopaper should rank by importance, not churn" | Computer: the desk files an intent record and links it to goals and decisions (§5.7) | Intent record for management; read back at the end of the monologue | Filed ≤ 30 s after the monologue ends |
| **Performance** | A take meant for an audience: "Recording a take for the team", "This is for my Opinion column" | Declared by the speaker, or proposed by the desk and confirmed | Same as take, with the auto-edit pipeline and an approval | Draft ready ≤ 60 s |

Ambiguity rule: when the class is uncertain (p < 0.7 from the classifier),
the default is **take** (nothing happens but a private capture), never
directive. Consequential classes (directive, any send) require the
read-back in §3.8.

### 3.6 Answering by inserting segments

- **Placement.** An answer is urgency 0 if the listener is waiting in a
  talk session (programme paused), otherwise urgency 1: it plays at the
  end of the current segment, introduced by "You asked about...".
- **Long answers are several segments.** "What happened with the
  Mauritius riots?" may become: an AI summary of the story at its current
  version (shared script), a human clip from a podcast that covered it, and
  a one-line "research is looking for more; I'll tell you when it lands"
  if the listener said "dig into it" (a directive to research, whose
  report returns as a later segment).
- **Answers carry their refs**, so "where's that from?" always works.
- **Nothing is answered from nowhere.** If the computer has no record for
  a question about its own state, the answer says so; it does not
  improvise (the doctrine's "review the traces before attributing
  behaviour", applied to narration).

### 3.7 Computer state, redirects, follows and mutes

- **"What's my computer doing?"** → a `desk_update` segment from the
  supervision projection: needs-you items first, then running work, then
  open commitments by materiality, then what settled since the listener
  last asked. Thirty to ninety seconds; "more" expands any item.
- **"Anything need me?"** → only the needs-you list; "approve" is not
  completed by voice (mobile plan §1.4: "never a voice-only approval"); the
  answer is "It's on your phone" plus a notification with the approval
  sheet (Face ID).
- **Redirects:** "Enough politics", "Just tech for the next half hour",
  "Play the whole MLST", "Catch me up on the last two days of the Taiwan
  story". These change the hour clock for a stated period or create an
  `answer` series; they do not change learned taste unless repeated.
- **Follows and mutes** are standing intents with a scope read back in one
  line, and they are listable ("What am I following?") and reversible.

### 3.8 Names, mishearings and confirmation

The owner's example: "Choir" heard as "Quire". Three layers:

1. **Custom vocabulary from the object graph.** The device keeps a
   vocabulary list built by the computer: product names (Choir, Autopaper,
   Autoradio, Autoputer, Texture, Jev), desk names, teammates' names, the
   listener's followed entities (labels and aliases from the claims
   layer's multilingual alias table), followed shows and hosts, and
   recent entities in the programme. Each term carries aliases for known
   mishearings ("Quire", "choir's" → "Choir"). FluidAudio takes exactly
   this shape (terms with aliases, canonical form in output; web: link
   above). The list is capped (a few hundred terms) and ranked by recency
   and the current segment's entities (inference: boosting too many terms
   raises false insertions; tune on the owner's takes).
2. **Phonetic matching for commands.** The on-device grammar matches
   commands and targets by phonetic key (Double Metaphone or a phoneme
   lattice from the G2P the TTS already ships; inference), so "follow
   Quire" resolves to "follow Choir" before any transcript is fixed.
3. **Read-back proportional to consequence.**

| Consequence | Example | Confirmation |
|---|---|---|
| Reversible, private | follow, mute, skip | None; the ack line says what was done ("Following Choir") and "undo" works for 10 s |
| Private record | a take saved, a question asked | None |
| Sends to a desk | a directive | Read back the target desk and the gist; proceed unless the listener says "no" within 3 s |
| Leaves the computer | a take to a teammate, an email reply, an Opinion draft to publish | Read back recipient and title; requires an explicit "send" plus a tap or Face ID on phone or watch (§4.5) |
| Authority | self-development approval, granting a mail policy | Never by voice; phone approval sheet with Face ID |

---

## 4. Takes and performances

### 4.1 Recording

- A take is recorded whole, on the device, at 16 kHz mono for transcription
  and, for performances, at 48 kHz for publication (inference: two taps on
  the same input; check the cost in the spike).
- It is linked to the segment it interrupted (publication and version, or
  the record id), as the prototype already does.
- It uploads to the user's computer as a private capture (owner ruling 6)
  with voice provenance: device, transcriber model and version, the
  interrupted segment as a citation (mobile plan §8.4). Until the computer
  acknowledges, the take stays on the device and is never deleted.

### 4.2 Auto-editing before it goes out

The edit is a **non-destructive edit decision list** over the original
audio, so every cut can be undone and the original is always kept:

| Step | Method | Default |
|---|---|---|
| Trim | Leading and trailing silence (VAD) | on |
| Filler and false starts | Word timestamps from Parakeet: drop "um", "uh", repeated words and restarted phrases, with 40 ms crossfades | on for performances, off for team takes |
| Long pauses | Shorten pauses over 1.2 s to 0.6 s | on |
| Noise and level | Voice processing already suppressed noise at capture; normalize to -16 LUFS, -1 dBTP | on |
| Content edits | Model proposes cuts of tangents or retracted remarks ("scratch that") with reasons; the speaker approves each | proposed, never applied silently |
| Packaging | Title, a one-paragraph summary, chapter marks for long takes, the cited story versions, a transcript with timed blocks | on |

Snipd's lesson is relevant: its users adjust a clip's start and end by
tapping words in the transcript (web: same App Store listing). Autoradio's
review sheet does the same on the phone; on the watch the review is
listening to the edited version at 1.5x.

### 4.3 AI voice for a take?

No, by default, and never a clone. The owner's contrast principle puts human
voices in human segments. A take's transcript can be *published as text*
(an Opinion is a Texture document; owner notes §6: "an Autoradio take can be
transcribed into an Autopaper Opinion"), and a listener who receives a text
Opinion hears it read in the AI voice like any written item, clearly
labelled as AI reading a human's words: "Yusef's Opinion, read by Choir".
The speaker may choose text-only publication if they do not want their
voice heard.

### 4.4 Routing

| Destination | What happens | Depends on | Class |
|---|---|---|---|
| **Keep** | Private capture on the user's computer; searchable; can be cited later | Take capture kind (orange) | orange |
| **Directive** | Units classed as directives become `directive` records to management; the take is cited | Directive records via the API | orange |
| **Team** | A team object adopted by teammates' computers; plays as a `take` segment in their programmes | LC, LA (contributions, accounts); team sharing | orange (red where it crosses computers) |
| **Opinion draft** | Texture writes a draft Opinion citing the stories at the versions heard; the speaker edits and publishes | LC (Opinion section); Texture is the sole writer | red at publication (Texture canonical write, public) |
| **Email reply** | The email desk drafts a reply quoting the take or its summary; it waits for approval | Email desk | red at send |
| **Audio Opinion** (published audio) | The edited take published with the Opinion | LC moderation; the media lawyer's questions on audio op-eds (media plan open question 4) | red |

### 4.5 Consent and approval

- **Nothing leaves the user's computer without the user's explicit
  approval**, matching the email desk's default. "Send" is a voice command
  plus a confirming tap or Face ID; on the watch, a confirming double tap
  while the watch is unlocked on the wrist (open question 3).
- **Policies can be granted later**, the email desk's model: "Team takes
  under two minutes may go without review." A policy is a standing intent,
  listable and revocable.
- **Teammates' takes** were shared by their speakers to a scope; they play
  only inside that scope and are never forwarded without the speaker's
  consent (a forward is a request to the speaker's computer).
- **Bystanders.** Diarization (FluidAudio includes on-device diarization;
  mobile plan §4.1) flags a take with more than one speaker; such a take
  cannot be published or shared until the speaker confirms the other
  voices consented, or the other speakers are cut.
- **Retraction.** A published take can be withdrawn; a team take can be
  recalled from computers that have not played it, and marked withdrawn on
  those that have.

---

## 5. The system briefing: a programme that teaches

### 5.1 The direction

Owner direction, 2026-10-14 (relayed by the coordinating session while
this note was being written; paraphrased, not a verbatim quote): the most
valuable personal use of Autoradio is a **system briefing** while building
something complex like Choir. The owner pours streams of ideas in and
wants depth back: the current state of the system, the possible pathways
forward, and explanations of the concepts involved, pitched at their level
and challenging them just past it. The thesis: the most important thing
superintelligence will do for people is **teach**. Many people will soon
work with systems beyond their own expertise, and a well-calibrated
feedback environment can speed their cognitive and emotional development.
That is social, not one human alone with one AI, because advice lands only
inside a trusted relationship.

The system briefing is therefore a **programme type**, not a segment
kind: a deliberate session of 10 to 80 minutes, started by voice ("brief
me on Choir", "where are we?"), by a standing schedule ("every morning on
my walk"), or proposed by the engine after a long monologue ("You raised
four things; want a briefing on how they touch the open work?"). It runs
inside the same engine and obeys the monologue-first ruling: long audio
out, long speech in, no quiz-show turn-taking.

### 5.2 Shape of a briefing

| Chapter | What it says | Source | Typical length |
|---|---|---|---|
| **Since last time** | What changed since this listener's last briefing: stations landed, decisions made, predictions settled, surprises | The tape since the listener's briefing cursor | 2-5 min |
| **State** | Where each part of the project stands, each claim labelled *built*, *decided, not built*, or *hypothesis* (the precommitment design's labels, spoken) | Records, goal files' current-state cards, code and commits | 5-20 min |
| **Decisions waiting** | Open decisions addressed to this listener, each as typed alternatives (§5.5) | Owner-decision lists, open questions, residuals | 3-10 min |
| **Pathways** | Where the work can go next, as typed alternatives with probabilities and who predicts them (§5.5) | Desks' precommitments, goal files' conjectures, panel outputs | 5-15 min |
| **Concepts** | Explanations of the ideas the state and pathways depend on, chosen by the learner model (§5.4), or on demand ("what's a prolly tree?") | `docs/learn/` primers and glossaries, doctrine, the code | 1-5 min each, inserted where needed |
| **Your turn** | An open-mic invitation; the listener talks as long as they want (§5.7) | The listener | open |
| **Playback** | The listener's own earlier takes, in their own voice, where they bear on the topic: "On Tuesday you said the ranking should optimize importance. Here is what happened since." | Take captures | 0.5-2 min |

Human voices still matter here (the contrast principle): teammates' takes,
the listener's own past takes, and, where one exists, a human clip that
explains a concept better than narration (a conference talk's podcast
audio on Dolt's prolly trees, say).

### 5.3 Traversing the listener's own tape

The briefing reads the project the way the repository's own discipline
says to read it: **the trace before the story**. Sources, in order of
authority:

1. **The tape and object graph**: commitment records (open, settled,
   discrepancies), runs and their fates, obligations and work items,
   approvals, the supervision projection ordered by materiality.
2. **Code and commits**: what actually landed, with SHAs and dates.
3. **Goal and mission documents**: the active goal (for Choir:
   `docs/ACTIVE.md`, `docs/mission-graph.yaml`, the throughline goal files
   and their `now` cards), problem documents (`docs/problems/`), dated
   reports with their owner-decision lists and open questions.
4. **Explanatory documents**: `docs/learn/` and the doctrine, for
   concepts.

Rules that keep it honest (inference, from the repository's own
failure receipts):

- **Prefer the tape and the code over documents**, and say so when they
  disagree. Example (real, already fixed): the vision document said Choir
  is "not a multiplayer app" until the owner corrected it on 14 October
  (doc: owner notes §4); a briefing that week should have said "the
  document says X; the owner's ruling says Y". A disagreement between a
  document and the code is itself worth teaching.
- **Every state claim carries a date and a ref**, so "how do you know?"
  always has an answer, and stale claims age visibly.
- **No behaviour is attributed without trace evidence**: "management
  ignored the instruction" is said only if the ledger shows it; otherwise
  it is said as a hypothesis (CLAUDE.md, "Review the traces before
  attributing behavior").
- **The Autoradio desk reads with observation verbs only.** Every desk
  already has `ReadFile`, `ListDir`, `Context`, `Records` and `Pack`
  (doc: `docs/learn/02-desk-runtime.md` §2.2). Deep questions about the
  world ("what are others doing about agent memory?") are cast to the
  research desk, whose report returns as a later segment.

### 5.4 The learner model is a precommitment record

Before a briefing segment plays, the engine predicts what the listener
understands, and the listener's own words settle it.

**Concepts.** The unit is a concept: a node in a concept graph whose
first inventory already exists in `docs/learn/` (six primers with
glossaries, prerequisites implied by the reading order, and self-check
questions with answer sketches; doc: `docs/learn/README.md`). For another
user's project, the planner builds the inventory from the project's own
documents and code, the same way.

**Levels.** Per listener and concept, a level on a small scale (inference:
five levels are enough to act on): 0 unaware, 1 recognizes the term,
2 can explain it, 3 can apply it to a decision, 4 can critique or extend
it. This is Bayesian knowledge tracing's idea, a probability per knowledge
component updated from the learner's actions (web: Corbett and Anderson
1994, https://link.springer.com/doi/10.1007/BF01099821), with the
learner's own speech as the action.

**What is committed**, per segment that depends on concepts:

| Question | Type | Settles by |
|---|---|---|
| The listener is at level ≥ k on concept C | yes/no with p, per concept | Their questions ("what's a CAS?" settles "no" for level ≥ 1), and their monologues: a Jev typed judgment "does this span use C correctly at level k?" on what they said, blind to the prediction |
| The listener will follow this explanation (no "say that again", no "simpler", no confused question within 2 min) | yes/no with p | Voice acts and replays |
| The listener will engage (a question, a take, a decision) | yes/no with p | Voice acts |
| The explanation was too easy ("I know this", skip) | yes/no with p | Voice acts and skips |

**Using the listener's own words as evidence is the core move.** The
owner already talks for ten minutes at a time; a concept used correctly in
a monologue is stronger evidence than any quiz answer, and it costs the
listener nothing (inference). Optional **check** segments exist ("Before I
go on: why does a crash restart not resume work? Think about it; I'll
come back to it") but they are invitations, never gates, and the answer
can arrive minutes later inside a monologue. The listener can turn them
off.

**Difficulty is calibrated, not guessed.** The target is the zone just
past the listener's current level: explanations pitched so the predicted
probability of following is high but not certain. The "eighty-five
percent rule" (web: Wilson et al. 2019, https://pmc.ncbi.nlm.nih.gov/articles/PMC6831579)
derives an optimal training error near 15% for gradient-descent learners
and offers it as a footing for the zone of proximal development; with
Bjork's *desirable difficulties* (web:
https://bjorklab.psych.ucla.edu/wp-content/uploads/sites/13/2016/04/EBjork_RBjork_2011.pdf;
difficulties help only when the learner has the background to meet them),
that suggests a design target, not a law: pitch each explanation where
p(follow) is about 0.75-0.9, and move up a level when the listener has
followed several in a row. The engine's calibration on these predictions
is scored like any other (§2.7), so "pitched at your level" is a
measurable claim.

**Commands that move the pitch:** "simpler", "deeper", "skip the basics",
"challenge me", "give me the math". Each settles the current prediction
and moves the level estimate.

**Why it matters** (owner's thesis; web: Bloom 1984, "The 2 Sigma
Problem", https://en.wikipedia.org/wiki/Bloom%27s_2_sigma_problem, the
finding that one-to-one tutoring outperforms group instruction by about
two standard deviations): a tutor's advantage is that it knows what the
student knows. A learner model that is a scored, inspectable record is
that advantage made checkable.

### 5.5 Pathways: PSA surfaced to the human

Decisions and pathways are presented as **typed alternatives with
probabilities**, the precommitment record's own shape (hypothesis,
assumptions, alternatives, typed questions with p) spoken aloud:

```text
AI   Pathway decision: how to rank the front page next.
     Three options.
     A, keep the importance judge and add depth. Research predicts the
       one-source-stub leads drop below 1 in 10 within a week: 0.7.
       Research's record on estimates like this: 6 of 9 held.
     B, rank by breadth of independent outlets. Management puts 0.4 on
       this lifting research-heavy stories over important ones; the
       preview on 14 October showed exactly that.
     C, wait for the claims layer's event merging. Lowest risk, slowest:
       engineering estimates three weeks, 0.5.
     The assumption they disagree on is whether breadth measures
     importance. What would you bet?
```

- **Who predicts, and with what record**, is said every time (§5.6).
  Probabilities with no named predictor are not spoken.
- **Mutation class and reversibility** are part of each alternative ("B
  is orange and reverts with a file delete").
- **The listener may commit their own probability** ("I'd say 0.8 on A").
  The listener's forecasts become their own private precommitments,
  settled like the desks' and shown only to them: the human gets a
  calibration record too. This is the learning economy's premise applied
  to the owner (doc: `docs/precommitment-records.md` §1.3) and is opt-in
  (open question 11).
- **The choice is recorded with provenance.** A decision spoken in a
  briefing becomes a decision record citing the briefing segment and the
  alternatives as they stood, which answers the standing question on
  decision provenance (doc: `docs/standing-questions.md`).
- **Unchosen paths are not scored as if they ran.** Their predictions stay
  on the tape; only the parts that later evidence can settle are settled,
  and the briefing says when a counterfactual cannot be checked.

### 5.6 Teams: one briefing, separate learners, trust first-class

- **Shared content, separate models.** State and pathways are rendered
  once for the team; each listener's concept insertions, pitch and
  playback are personal. Each learner model lives on that listener's own
  computer.
- **Learner models are private by default, including from managers.**
  Inference: a learner model a manager can read becomes a performance
  review, and people stop asking the questions that settle it. A listener
  may share a concept's level with the team ("tell the team I've got the
  snapshot path now"); nothing is shared implicitly.
- **"Who knows this?" comes from public work, never private models.** The
  briefing may suggest "Alex wrote the snapshot-resume change; ask for a
  two-minute take" because authorship is on the tape, not because Alex's
  learner model says so.
- **Speakers are named, with their relationship and record.** Every
  segment says who is speaking: which desk, which teammate, which outside
  publisher. A desk's prediction carries its track record on that kind of
  prediction when n meets the bar (the box score rules: never one trust
  number, "n too small" below the bar; doc: claims design §8). A human
  teammate's record is spoken only if they opted in (`actor_scope`
  defaults to private).
- **Advice travels through people where it can.** Because advice lands
  inside trusted relationships, the engine prefers to route a hard point
  through a human the listener works with: it can request a take from a
  teammate (a consented request to their computer) and play it in their
  voice, rather than narrate the same point itself.
- **Encouragement only with evidence.** Language models drift toward
  telling users what they want to hear (web: Sharma et al., "Towards
  Understanding Sycophancy in Language Models",
  https://arxiv.org/abs/2310.13548). The briefing praises only with a
  settled record behind it ("Three weeks ago you asked what a CAS was;
  today you argued for one, correctly"), says plainly when the listener's
  reasoning conflicts with evidence, and never optimizes for how the
  listener feels about the session. Emotional development, in this design,
  means a feedback environment that is honest, consistent and kind in
  tone, not one that flatters.

### 5.7 Monologues as intent capture

The listener's idea streams are the briefing's input as much as its
output. A monologue's units (§3.4) gain one class for this programme:

| Class | Example | What happens |
|---|---|---|
| **Intent** | "Autopaper should rank by importance, not churn"; "the ontology needs a box score per outlet" | An **intent record**, verbatim span plus timestamp, filed for management as owner direction "recorded before any design work. Status: intent", the form this repository already uses (e.g. `docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`). Not a directive: nothing is built from it until management or the owner promotes it |

- **Linked and checked on capture.** The desk links each intent to the
  goals, decisions and records it touches, and flags conflicts with
  settled decisions (the standing question on settled-decision
  conformance): "This conflicts with ruling 5 from this morning; which
  stands?" The conflict is asked in the next briefing, not as an
  interruption.
- **Read back once, at the end.** "I heard five ideas. Two are new
  intents; one refines the ranking intent; one is a question I'll answer
  in the next segment; one sounded like a directive to engineering, and I
  have not sent it. Say 'send it' if you meant it."
- **Fed forward.** The next briefing's "since last time" chapter opens
  with what became of the listener's last intents: promoted, designed,
  built, or still waiting.

### 5.8 Cost, placement and build

- **This is the expensive programme, by design.** A 30-minute briefing is
  about 4,500 spoken words, written from a large context (records, goal
  files, commits, primers). At DeepSeek V4 Pro list prices ($1.32 per
  million input, $3.96 output; code: `internal/llmcost/pricing.go`),
  200-400k input tokens and 10k output is about $0.30-0.60 per briefing,
  less with cached context (estimate). Quality outranks cost here: it is
  the owner's highest-value use, and "execution is everything".
- **Placement.** The Autoradio desk writes the briefing; the research desk
  answers world questions; Texture renders a written companion (the same
  briefing as a Texture document with its refs) for reading later.
- **Build:** a briefing v0 can be built before the phone app: a script
  generated from the tape, `docs/ACTIVE.md`, the mission graph, goal files
  and `docs/learn/`, read by any TTS (the M0 client or the desktop). The
  learner model and check segments come with M2's monologue pipeline; team
  briefings follow accounts and teams (LA) (§10).

---

## 6. Apple Watch and audio-only operation

### 6.1 What works with no screen

| Function | Watch, no phone nearby | Watch with phone | Notes |
|---|---|---|---|
| Play the programme | Yes, via Wi-Fi or cellular: AVPlayer and AVQueuePlayer stream on watchOS since watchOS 6 (web: https://developer.apple.com/videos/play/wwdc2019/716) | Yes | Long-form audio with the `.longFormAudio` policy needs a Bluetooth route, except the built-in speaker on supported models from watchOS 11 (web: https://developer.apple.com/documentation/avfaudio/avaudiosession/activate(options:completionhandler:)) |
| AI segments | Pre-rendered audio assets (server-rendered, §7.1) | Rendered on the phone, streamed or transferred | Running Kokoro on the watch is UNVERIFIED and unlikely to be worth it |
| Talk (record a take) | Yes, watch microphone; upload to the computer | Yes; the phone transcribes | |
| Commands | Small on-watch grammar for playback commands (inference: `SFSpeechRecognizer` or SpeechAnalyzer availability on watchOS is UNVERIFIED; fallback: send audio to the phone or the computer) | Phone's EOU model | Playback commands also map to the crown and double tap |
| Questions, directives | Audio uploaded, transcribed on the computer (Parakeet on the host CPU, inference) | Phone transcribes | Answer arrives as a segment |
| Approvals | No (open question 3) | Phone | |
| Haptics | Yes | Yes | §6.2 |

The watch app's controls when the screen is up: Talk (also the double tap,
assigned with `.handGestureShortcut(.primaryAction)`; double tap is the only
hand gesture apps may respond to; web:
https://developer.apple.com/documentation/watchos-apps/enabling-double-tap),
the Digital Crown for volume, swipe for skip, and the Action button on
Apple Watch Ultra mapped to Talk through App Intents (UNVERIFIED which App
Intents the Action button accepts on the current watchOS).

### 6.2 Haptic and earcon vocabulary

Kept small enough to learn in a day (inference):

| Event | Earcon | Watch haptic |
|---|---|---|
| Heard you (talk started) | short rising tone | one tap |
| Done listening (talk ended, received) | short falling tone | one tap |
| Command done | none (the effect is audible) | none |
| Something for you is queued (urgency 1) | soft two-note | two light taps |
| Interrupting (urgency 0) | distinct three-note, then the line | strong double |
| Needs your phone (approval, send) | the "phone" tone | long buzz |
| Error (didn't catch that, offline) | low tone | three short |

A spoken equivalent exists for each, for the first week or on request.

### 6.3 Watch architecture

- **Thin player, not a second engine.** The watch fetches a short horizon
  (the next 30-60 minutes) from the phone when nearby (WatchConnectivity)
  or from the API when standalone, plays assets, records takes, logs
  signals, and syncs. It runs the same scheduling rules in a reduced form:
  take the next item; honour level-0 alerts from the phone or a push.
- **Public story-update audio is shared**, so the watch can stream it from
  the host without the computer awake. Personal segments (mail, desk) are
  rendered by the phone or the computer and fetched when the watch asks.
- **Formats:** AAC or xHE-AAC per Apple's watch guidance (same WWDC source).

### 6.4 CarPlay later

The CarPlay audio entitlement must be requested from Apple, and iOS 26.4
added a "voice-based conversational apps" category (mobile plan §4.3; web:
https://developer.apple.com/carplay/). The car is where the talk session
and wake word matter most; it comes after the App Store release, with Siri
("Hey Siri, talk to Autoradio") as the first hands-free entry and the
steering-wheel voice button as Siri's. CarPlay's audio app template shows
lists, not custom UI, which suits an audio-only design.

---

## 7. Architecture

### 7.1 What runs where

| Component | Phone | Watch | Host (Node B) | User's computer (guest) |
|---|---|---|---|---|
| Audio engine (one `AVAudioEngine` graph: TTS, human audio, earcons, ducking, voice processing) | ✓ | reduced (AVQueuePlayer) | | |
| ASR: EOU streaming, Parakeet TDT v3 batch, VAD, diarization | ✓ | VAD only | Parakeet CPU fallback for watch uploads and podcast transcripts (inference) | |
| TTS: personal segments | ✓ (AVSpeech at M0, Kokoro later) | | | fallback render for watch |
| TTS: shared story-update segments | renders from script, or fetches audio | fetches audio | ✓ renders once per script version, serves from Store B (inference: Kokoro on CPU; cost in §8.2) | |
| Scheduler (fast loop), hour clock, preemption | ✓ | reduced | | |
| Signal log (buffer) and upload | ✓ | ✓ (via phone or API) | | stored on the tape |
| Prefetch cache: next N segments, horizon | ✓ | short | | |
| Command grammar (local) | ✓ | small | | |
| Autoradio desk: candidates, scoring, scripts for personal segments, utterance segmentation, answers, precommitments, settlement | | | | ✓ |
| Email desk (summaries, urgency, drafts) | | | maild transport (code: `internal/maild`) | ✓ (a desk; to be built) |
| Public candidates (stories, scripts, episodes, clips) | | | ✓ Store B via corpusd, no wake | adopts |
| Supervision summary (counts, urgent flags) | | | ✓ written by the computer on events | writes it |

### 7.2 An Autoradio desk, or a scheduler in the client?

| | Client-only scheduler | Autoradio desk only (server streams the programme) | **Hybrid: desk plans, client schedules** |
|---|---|---|---|
| Latency at boundaries | best | depends on network and wake | best |
| Offline | yes | no | yes, for the cached horizon |
| Private graph (mail, desk state) | needs every source on the phone | natural | natural |
| Scripts and answers (model calls) | from the phone, with credentials on the device | natural, through the gateway | natural |
| PSA (commitments, settlement, packs) | outside the ledger, so outside the doctrine | natural | natural |
| Computer awake time | none | the whole listening session | minutes per horizon |
| Multiple front ends (phone, watch, desktop, CarPlay, a sibling app) | each re-implements it | one | one planner, thin schedulers |
| Owner note §5 ("each app agent is its own RLM desk; one desk can have several front ends") | no | yes | yes |

**Decision: the hybrid.** The Autoradio desk is an app-agent desk with its
own RLM and verb vocabulary (read: listening log, supervision projection,
email desk records, followed graph, public candidates; write: run sheet
versions, segment scripts, its own commitment records, answers; send:
directive records to management on the listener's behalf, drafts to the
email desk; nothing else). It has **no** authority to send mail, publish,
approve or write canonical Texture revisions; those stay with the desks
that own them. The client scheduler is deterministic code with no model,
tested like any client logic (the prototype's `autoradio.ts` is its seed).

The desk's addressing (who may address whom, code:
`internal/agentprofile/agentprofile.go`, `PolicyFor`) needs new entries:
Autoradio may address management (directives) and the email desk (draft
requests); management, the email desk and Texture may address Autoradio
(urgent items, "tell the listener"). Fail-closed by default, as the code
requires. This is station LP's verb registry work (state and vision §2.11).

### 7.3 APIs

All under the versioned product API with an OpenAPI contract (mobile plan
§8.6). Sketch:

| Endpoint | Served by | Wakes the computer? | Purpose |
|---|---|---|---|
| `GET /api/autoradio/horizon?after=<run_sheet_version>` | computer | yes (on intent: the device's horizon is low, or the listener started a session) | The planned horizon: segments with metadata, predictions, asset refs |
| `GET /api/platform/autoradio/public?since=<cursor>&sections=` | host (Store B) | no | Public candidates: story updates with shared scripts and rendered audio refs, new episodes, clips |
| `GET /api/autoradio/assets/{hash}` | host for shared, computer for personal | personal only | Script text or rendered audio, content-addressed, cacheable forever |
| `GET /api/autoradio/taste-profile` | computer | yes | The listener's scoring weights, hour-clock parameters, follows and mutes, vocabulary list; lets the device score public candidates without waking the computer |
| `POST /api/autoradio/signals` | computer | buffered: sent with the next horizon fetch, or flushed at session end | Batched playback events with idempotency keys; settle predictions |
| `POST /api/autoradio/utterances` | computer | yes (on intent) | A transcript (with word timestamps) plus the take capture ref; returns accepted units and the planned insertions |
| `POST /api/captures` (take variant) | computer | yes | The take audio and transcript (mobile plan §8.4) |
| `GET /api/supervision/summary` | host | no | Counts and urgent flags (mobile plan §8.3) |
| Push (APNs) | host relay | no | Level-0 and level-1 items, ids only, no content |

Signals buffered on the device are the listener's own data on the
listener's own device; they are not written to the host while the computer
sleeps (inference: simplest private design; open question 6 covers a
host-side encrypted drop box if buffering proves lossy).

### 7.4 Latency budgets

| Path | Budget | How |
|---|---|---|
| Segment boundary to next audio | ≤ 150 ms (gapless for prefetched) | Next N segments cached and pre-rendered; AI segments rendered ahead by sentence |
| Voice onset to duck | ≤ 150 ms | VAD on device |
| Command end to effect | ≤ 300 ms | EOU (median 160 ms) + local grammar |
| Question ack | ≤ 1 s | Local earcon and templated line |
| Question answer, computer awake | ≤ 8 s to first audio | Desk turn + script + TTS of the first sentence; stream the rest |
| Question answer, computer hibernated | ≤ 20 s (inference) | Snapshot resume (seconds) or cold boot (~10.5 s), then the above; the ack line says "checking your computer" |
| Monologue end to draft take ready | ≤ 30 s (60 s for performances) | Chunked transcription during speech; desk segmentation after |
| Level-0 item to on-air | ≤ 60 s from the source event while listening | Host summary poll (≤ 30 s) or push; fetch; next boundary |

### 7.5 Caching, prefetch and offline

- **Prefetch depth:** the next N segments where N covers 30 minutes or 5
  segments, whichever is more; on Wi-Fi, the whole horizon's AI segments
  (they are text) and the first 10 minutes of each human episode, streamed
  from the publisher's URL into the player's buffer only (no re-hosting,
  no persistent copy; media plan §4; a personal offline download is an
  open legal question, mobile plan §8.5).
- **Offline:** the programme continues from the cached horizon and cached
  public scripts with on-device TTS; personal segments older than their
  `expires_at` are dropped; signals buffer; takes queue.
- **Horizon refresh:** when fewer than 30 minutes remain, at the start of a
  session if the horizon is older than its shortest `expires_at`, or when
  the host summary shows a new urgent item. Each refresh is one computer
  wake. Expected 1-4 wakes per listening hour (estimate).

### 7.6 Battery and data

- **Data (estimate):** publisher audio at 64-128 kbps is about 29-58 MB per
  hour of human audio; AI segments as text are under 100 KB per hour;
  server-rendered AI audio for the watch at about 48 kbps is about 22 MB
  per hour of AI audio.
- **Battery (UNVERIFIED; measure in the spike):** playback alone is cheap;
  the costs are on-device TTS (Kokoro on the Neural Engine, sentence by
  sentence ahead of playback), the open microphone with voice processing
  during a talk session, and Parakeet passes. Push-to-talk keeps the mic
  closed most of the time, which is a second reason it is the default.

### 7.7 Failure and restart behaviour

The repo's restart rule applies directly (CLAUDE.md, "Restarts end work"):

- **A crash on the computer closes the planner's in-flight run**
  ("interrupted by a restart"); the device keeps playing its cached
  horizon and asks again later. Predictions for that horizon resolve
  `interrupted` where they depended on the dead run.
- **The listener's standing intents survive** (follows, mutes, the
  programme being "on"); the occurrence (a planning run, an answer in
  progress) does not resume after a crash. An unanswered question is
  answered with "I lost that when your computer restarted; ask again?"
  rather than silently re-run.
- **The device never depends on the computer to keep playing** (fate-
  sharing: a sleeping or crashed computer degrades the programme to public
  content; it does not stop it).

### 7.8 Mutation classes

| Work | Class | Protected surfaces |
|---|---|---|
| Client scheduler, audio engine, local grammar | orange (client) | none server-side |
| Autoradio desk (new app-agent desk, verbs, addressing policy) | orange; **red** where it adds addressing rules (`PolicyFor`) | desk authority |
| Run sheet writer, segment records, signals ingestion | orange | object graph (reserved kind gets its first writer) |
| Autoradio precommitments and rule settlement | orange (commitment records exist) | run acceptance not touched |
| Shared story-update scripts in Autopaper's pipeline | orange (model routing through the gateway) | gateway calls |
| Host rendering of shared audio | orange | none |
| Take routing to team, Opinion publication, email sends | **red** | Texture canonical writes, outbound mail |
| Approvals from phone or watch | **red** | self-development approvals |
| Push relay | **red** | outbound credentials |

---

## 8. Rights and cost

### 8.1 Rights per segment kind

| Kind | Basis | Rule |
|---|---|---|
| `human_episode` | A podcast player playing the publisher's enclosure from the publisher's host (media plan §4; diagonal memo: "playing podcasts and clips is a podcast player — clear") | Never re-host or proxy; attribute by name before playing; link out; keep the publisher's analytics intact by fetching their URL |
| `human_clip` | Same, seeking into the publisher's file; the clip choice uses Choir's transcript capture (reporting; *Swatch* pattern) | Short, attributed, with the minute; the transcript is not published whole; YouTube-only shows are not split from video (terms) |
| `story_update`, `developing_event` | Choir's own writing from its own captures and verified quotes | The same rules as Autopaper text: verified quote spans, attribution, AI-written labelling per the AI Act Art. 50(4) decision (media plan step 1) |
| `mail_update` | The user's own mail, for the user | Never mixed into shared programmes |
| `desk_update` | The user's own computer | Same |
| `take` (own) | The speaker's own work | Their consent governs every send |
| `take` (teammate) | Shared by its speaker to a scope | Play only within that scope |
| `answer`, `alert`, `bridge` | Choir's own writing | Cite refs |
| Music | None licensed | No music beds unless original or licensed |

Open legal items carried from the media plan's lawyer list: implied
licence for in-app enclosure playback beside our own transcript (its
question 4), and published audio Opinions.

### 8.2 Cost per listening hour (estimate)

Assumptions: an hour of programme is about 30 minutes of human audio and 30
minutes of AI voice and takes; about 25 AI segments; one planning wake per
20-30 minutes; model prices from `internal/llmcost/pricing.go` (DeepSeek
V4.1 Flash $0.30 input / $1.20 output per million tokens; GPT-6 Luna
$0.10 / $0.50; Jev $0.042 input).

| Item | Tokens or units per listening hour | Cost per listener-hour |
|---|---|---|
| Shared story-update scripts (written once per story version) | ~15 scripts × 6k in / 400 out, shared by all listeners of that version | ≈ $0.04 for the first listener; ≈ $0 at scale |
| Personal preambles, mail and desk scripts, bridges | ~10 × 4k in / 250 out | $0.01-0.015 (Flash), $0.005 (Luna) |
| Planner desk turns (candidates, scoring, commitments) | 2-3 turns × 30-40k in (mostly cached) / 3k out | $0.01-0.05 |
| Jev classifications (clip relevance, utterance classes, urgency) | ~200 × 1k in | ≈ $0.01 |
| Monologue processing (10 min of speech) | ~2k tokens of transcript, ~15k in with context | ≈ $0.005-0.01 |
| Email summaries (20 messages) | ~60k in / 3k out | ≈ $0.02 |
| ASR | on device | $0 |
| TTS, personal | on device | $0 |
| TTS, shared, server-rendered | once per script version, CPU | ≈ $0 per listener |
| Computer awake time | a few minutes per hour; a $129/month host serving ~1,200-2,000 users at ~1.5% busy (pricing notes) puts an awake computer-hour well under $0.01 (inference) | < $0.01 |
| **Total** | | **≈ $0.03-0.15 per listener-hour** |

The system briefing (§5) is priced separately: about $0.30-0.60 per
30-minute briefing with the strongest listed model, less with cached
context (estimate, §5.8).

At two hours a day of the ordinary programme that is about $2-9 a month
per listener (estimate),
inside the $35 anchor but not negligible; the levers are shared scripts,
caching, the cheapest model that passes a quality check per segment kind,
and the state and vision note's "tiers by time spent". Human audio costs
Choir nothing to serve: the publisher serves it.

---

## 9. Prior art and how this differs

| Product | What it does | Borrow | Avoid |
|---|---|---|---|
| **Spotify DJ** (2023; voice requests 2025) | A personalized music stream with a synthetic host voice modelled on a real person, commentary written with OpenAI models under heavy editorial oversight; hold the DJ button and speak a request (web: https://www.thurrott.com/?p=279662; https://www.tomsguide.com/entertainment/music-streaming/spotifys-ai-dj-now-takes-requests-heres-how-it-works) | Hold-to-talk on the main control; a host voice that only connects human-made content | A lifelike, human-modelled voice (the owner wants clearly AI); banter as a feature; requests as the only voice act |
| **Spotify Personal Podcasts / Studio** (May 2026) | AI-generated personal podcasts and daily briefings from listening history and connected email, calendar and notes; a CLI for coding agents (web: https://www.macrumors.com/2026/05/08/spotify-personal-podcasts-ai-agent; https://spotify.substack.com/p/spotify-expands-into-ai-audio-with) | Briefings from personal data; agents as producers of audio | One-way generated episodes; an AI talking all hour |
| **NotebookLM Audio Overviews, interactive mode** | Two AI hosts discuss your sources; "Join" lets you ask a question, the hosts answer and resume; English only; delays and voice glitches noted (web: https://support.google.com/notebooklm/answer/16212820) | Answer-then-resume; answers grounded in sources | Two-host AI banter; the question box as the only interaction |
| **Google Daily Listen** (Search Labs, 2025) | A ~5-minute personalized audio overview from Search and Discover activity, with transcript, per-story skip and thumbs (web: https://9to5google.com/2025/01/08/google-discover-daily-listen/) | Per-story skip as a unit; links out to sources | Thumbs; a fixed short episode instead of a stream |
| **Huxe** (2025, shut down 28 May 2026) | Daily briefing from email and calendar, "live stations" on topics, interrupt the hosts to ask (web: https://techcrunch.com/2025/09/23/former-notebooklm-devs-new-app-huxe-taps-audio-to-help-you-with-news-and-research; shutdown: https://techcrunch.com/2026/05/22/audio-generation-app-huxe-founded-by-former-notebooklm-developers-shuts-down/) | Stations as standing topics that refresh; mail in the mix | The lesson: generated audio alone became a commodity the day a platform shipped it. Differentiation must be the graph (provenance, versions, claims), human media, the user's own agents, and two-way takes |
| **Snipd** | AI podcast player: tap headphones to save a moment, AI transcripts, chapters and summaries, clip bounds edited on the transcript (web: https://apps.apple.com/app/id1557206126) | Headphone gesture as an opt-in capture; transcript-based clip editing for takes | |
| **Particle** | AI news reader; "Podcast Clips" matches podcast moments to stories with embeddings and AI-chosen clip bounds, transcript highlighted as spoken; entity pages list a person's podcast appearances (web: https://techcrunch.com/2026/02/23/particles-ai-news-app-listens-to-podcasts-for-interesting-clips-so-you-you-dont-have-to/) | Clips attached to stories; entity-keyed appearances (our claims layer does this by Wikidata id) | Unstated rights basis for clips; Choir plays the publisher's file and records the basis |
| **Artifact** (2023-2024) | Personalized AI news ranking and summaries; wound down because "the market opportunity isn't big enough", with moderation of comments and posts too costly for a small team (web: https://the-decoder.com/ai-curated-news-app-artifact-shuts-down-after-one-year/) | | Personalization alone is not a product; social features need moderation before launch (LC's moderation gate) |
| **ElevenReader / GenFM** | Lifelike TTS of any text; two AI co-hosts generated from a document (web: https://elevenlabs.io/text-reader; https://elevenlabs.io/de/genfm) | Reading any document aloud (Texture documents, PDFs) | Lifelike voices; fake co-hosts |
| **Voice assistants and full-duplex models** (Siri, Alexa; Moshi) | Turn-taking dialogue; Moshi models overlap and interruption with ~200 ms latency (web: https://arxiv.org/abs/2410.00037) | Barge-in engineering, end-of-utterance detection | Conversation as the product; a long monologue does not fit a turn |
| **Tutoring research and intelligent tutors** (Bloom's 2 sigma; knowledge tracing) | One-to-one tutoring beats group instruction by about two standard deviations; knowledge tracing keeps a probability per skill updated from the learner's actions (web: https://en.wikipedia.org/wiki/Bloom%27s_2_sigma_problem; https://link.springer.com/doi/10.1007/BF01099821) | A per-concept probabilistic learner model; pitching by what the learner knows | Quizzes as the main evidence; a monologue is better evidence and costs the listener nothing |
| **Broadcast radio** | The format clock, station idents, back-announcing, traffic and news breaks that preempt (web: https://en.wikipedia.org/wiki/Broadcast_clock) | The clock as a constraint set; idents; preemption discipline | Fixed schedules; ads |

**What none of them does** (the differences that matter):

1. Mix human media and AI updates from **one provenance graph**, with
   story versions and claims, so every spoken fact has a source and the
   update says exactly what changed since you last heard.
2. Report the **state of your own agents** and route your spoken
   directives to them as records.
3. Accept **long monologues** and turn them into routed takes,
   performances, questions and directives.
4. Keep a **scored record of its own predictions about you**, which it
   learns from in context and which you can ask about.
5. Keep your attention data **on your own computer**.
6. **Teach you your own project**: brief you from your own tape, state
   pathways as scored predictions with named predictors, and pitch
   explanations from a learner model that is itself a scored record.

---

## 10. Phased build plan

Aligned with the mobile plan's milestones (§10 there). Estimates in focused
agent-days, as there.

| Phase | Programming engine scope | Depends on | Done when (artifact) | Effort |
|---|---|---|---|---|
| **Spikes** (add to the mobile plan's three) | (d) Voice processing on `AVAudioEngine` with programme playback through the same engine: echo leakage into VAD, ducking behaviour, time to duck; (e) chunked Parakeet with FluidAudio custom vocabulary on 20 of the owner's spoken sentences with "Choir", "Autopaper", "Jev", teammates' names; (f) watch: stream an AAC asset with the built-in speaker and AirPods; record and upload a take | none | Dated evidence notes with numbers and recordings | 2-3 days |
| **M0** (local prototype) | Swift scheduler: hour clock, public candidates from the front API, shared scripts generated on the client for now (labelled prototype), local command grammar, push-to-talk, signals logged locally, segment metadata schema v0 | none beyond M0 | The owner listens for 30 minutes, skips, follows, interrupts; the signal log shows every event; an XCUITest run plays a scripted programme | inside M0's 8-12 days, +3 |
| **M1** (supervision) | Autoradio desk v0: horizon from public plus supervision projection; `desk_update` and `answer` segments; "what's my computer doing?"; host summary for urgent flags; level 0-1 preemption | Supervision feed and host summary (mobile plan §8.3); app-agent desk scaffolding and addressing policy (LP) | The owner asks the state of his computer by voice and hears a correct answer citing records; a failed capsule preempts within 60 s | +8-12 days server, +3 client |
| **M2** (Parakeet takes) | Talk sessions and monologues; chunked transcription; utterance segmentation and routing; takes as private captures with auto-edit; directives to management; per-segment precommitments and rule settlement; calibration view for the listener | Take capture kind; directive records via API; commitment records from an app desk | A ten-minute monologue becomes a take, a directive and a question, each routed correctly; a week of predictions settles with a Brier score per kind | +10-14 days |
| **System briefing v0** (can start before M0 ships; owner's highest-value use, §5) | A briefing script from the tape, `docs/ACTIVE.md`, the mission graph, goal files, open-decision lists and `docs/learn/`; chapters: since last time, state with evidence labels, decisions, pathways with named predictors, concepts on demand; read by the M0 client or the desktop; a written Texture companion | Read access to records and files (observation verbs exist); the Autoradio desk scaffold, or a one-off run while it does not exist | The owner listens to a 30-minute briefing on his walk and finds no claim without a ref; every pathway names its predictor | 4-6 days |
| **System briefing v1** (with M2) | Learner model as precommitments over the `docs/learn/` concept inventory; settlement from monologues by Jev; pitch commands; check segments; intent capture with conflict flags; the owner's own forecasts (opt-in) | M2's monologue pipeline | Over two weeks, the engine's p(follow) is calibrated within 0.1 per level, and the owner's intents appear in management's queue with links | +6-10 days |
| **Team briefings** (later) | One briefing, per-listener learner models, requested teammate takes | LA, team sharing | Two listeners hear one briefing pitched differently; neither model leaves its owner's computer | later |
| **Email desk** (parallel, its own design) | `mail_update` segments; voice reply drafts; nothing sent without approval | The email desk (its own RLM), per owner ruling | The owner hears a new message summarized and dictates a reply that waits as a draft | its own plan |
| **M3** (media, watch) | Podcast sources first-class in the registry (media plan step 3), transcripts as captures (step 9), clip selection by Jev; shared script and audio rendering on the host; the learned V model and exploration; watch app | Media plan steps 3 and 9; host rendering | A new MLST episode plays the morning it appears for a listener who follows AI strategy; the watch runs a 30-minute programme with no phone | +12-18 days |
| **Later** | Teammates' takes and team programmes; audio Opinions; wake word; CarPlay; box score lines; Android | LC, LA, LC moderation, the lawyer's answers, the App Store | | |

**What to do first:** the three new spikes, beside the mobile plan's three.
Then the segment schema and the horizon API as a design note with an
OpenAPI fragment (no code), because the client scheduler, the desk and the
watch all depend on that contract.

### 10.1 Risks

| Risk | Impact | Mitigation |
|---|---|---|
| The AI voice is tiring for long stretches | Listeners leave | Human-voice share ≥ 50%, AI runs ≤ 4 min, short templated bridges (§2.4) |
| Story-update scripts are slop | "I don't want slop" | Shared scripts written from claims deltas with verified quotes; a Jev check per script; the owner listens daily in M0-M2 |
| Echo cancellation fails on speaker playback | Barge-in false triggers | Spike (d); fall back to push-to-talk only on speaker |
| Monologue misclassified as directive | Unwanted actions | Default to take below p 0.7; read-back for directives |
| Planner wakes the computer too often | Compute cost | Taste profile on device for public candidates; horizon of 2-3 h; measure wakes per listening hour |
| Personalization becomes engagement bait | Owner's objective lost | Importance floor and exploration as constraints; Goodhart detector (§2.7) |
| Rights: clips | Takedowns | Publisher-hosted only; attribution; lawyer list |

### 10.2 Open questions for the owner

1. **The desk's name.** "Autoradio desk" (product name) or a role name
   such as "programme desk"? Recommendation: the Autoradio desk, since the
   owner's rule is that each app agent is a desk.
2. **Human-voice share.** Is "at least half the listening time is human
   voices" the right default, or should it be learned from zero?
3. **Approvals and sends on the watch.** May a confirming double tap on an
   unlocked, worn watch approve a send (a take to the team, an email
   reply)? Self-development approvals stay on the phone with Face ID in any
   case. Recommendation: sends yes, approvals no.
4. **Interruption budget.** Two level-0 interruptions an hour by default,
   and which sources may raise level 0 (engineering failures, the email
   desk, teammates)?
5. **Whole episodes.** For followed shows, play whole episodes by default
   (the owner's MLST example) or offer a summary first? Recommendation:
   whole, with a one-line intro, resumable.
6. **Aggregate listening data.** Should the shared Autopaper ranking ever
   learn from listeners' attention (opt-in, privacy-preserving), or never?
   Recommendation: never for now; revisit with a differential-privacy
   design.
7. **The wake word.** Worth an opt-in in the first App Store version, and
   what phrase? It needs a commercial licence (Porcupine) or an open model.
8. **AI reading of teammates' text Opinions.** Acceptable to read a human's
   written Opinion in the AI voice with a clear label, or text only?
9. **Exploration share.** 10% of candidates outside the profile, spoken as
   such: right, or should exploration be silent?
10. **Engine transparency.** Should "how well do you know me?" give the
    listener their own calibration numbers, as recommended (§2.7)?
11. **The listener's own forecasts.** In a briefing, may the owner commit
    probabilities on pathways and get a private calibration record of his
    own (§5.5)? Recommendation: yes, opt-in, never shown to anyone else.
12. **Check segments.** Are invitations to think ("Before I go on: why...?")
    welcome, or should concept evidence come only from what the listener
    says unprompted (§5.4)? Recommendation: on, rare, easy to turn off.
13. **Learner-model sharing in teams.** Confirm the default that no one,
    including a manager, can read another listener's learner model, and
    that sharing a level is an explicit act by its owner (§5.6).
14. **Briefing cadence and depth.** A standing daily briefing (say 20
    minutes each morning), or only on request? And is about $0.30-0.60 per
    30 minutes acceptable for the strongest model (§5.8)?

---

## Sources

Choir (docs and code): `docs/reports/autoradio-mobile-app-plan-2026-10-14.md`
(including "Owner rulings, 2026-10-14 ~21:40 UTC"),
`docs/reports/autoradio-prototype-2026-10-14.md`,
`docs/reports/claims-events-entities-ontology-2026-10-14.md` (§0, §4.2-4.4,
§8), `docs/precommitment-records.md` (§1, §2, §4.7, §5),
`docs/reports/object-graph-retrieval-research-2026-10-14.md` (§0, §2, §5),
`docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`,
`docs/research/autopaper-media-and-rights-2026-10-14/README.md` (§4, §6, §7),
`docs/reports/pricing-and-economics-notes-2026-10-14.md`,
`docs/reports/owner-notes-2026-10-14.md` (§4-§6),
`docs/reports/choir-state-and-vision-2026-10-14.md` (§2.11),
`docs/memo-diagonal-media-strategy-2026-09-17.md`,
`docs/learn/README.md`, `docs/learn/02-desk-runtime.md`,
`docs/learn/04-computer-lifecycle.md`, `docs/standing-questions.md`,
CLAUDE.md (restart rule, trace-before-attribution rule); code:
`internal/objectgraph/registry.go:76`, `internal/mediastate/handler.go`,
`internal/maild/`, `internal/agentprofile/agentprofile.go`,
`internal/llmcost/pricing.go`.

Web (fetched 2026-10-14):

- Spotify DJ launch: https://www.thurrott.com/?p=279662,
  https://www.techradar.com/news/spotify-is-getting-a-new-ai-dj-with-the-help-of-chatgpts-creators;
  voice requests: https://www.tomsguide.com/entertainment/music-streaming/spotifys-ai-dj-now-takes-requests-heres-how-it-works,
  https://www.fastcompany.com/91332823/spotify-ai-dj-voice-request
- Spotify Personal Podcasts: https://www.macrumors.com/2026/05/08/spotify-personal-podcasts-ai-agent,
  https://spotify.substack.com/p/spotify-expands-into-ai-audio-with
- NotebookLM interactive Audio Overviews: https://support.google.com/notebooklm/answer/16212820,
  https://blog.google/technology/google-labs/notebooklm-new-features-december-2024/
- Google Daily Listen: https://9to5google.com/2025/01/08/google-discover-daily-listen/
- Huxe: https://techcrunch.com/2025/09/23/former-notebooklm-devs-new-app-huxe-taps-audio-to-help-you-with-news-and-research,
  https://techcrunch.com/2026/05/22/audio-generation-app-huxe-founded-by-former-notebooklm-developers-shuts-down/
- Snipd: https://apps.apple.com/app/id1557206126
- Particle Podcast Clips: https://techcrunch.com/2026/02/23/particles-ai-news-app-listens-to-podcasts-for-interesting-clips-so-you-you-dont-have-to/
- Artifact: https://the-decoder.com/ai-curated-news-app-artifact-shuts-down-after-one-year/
- ElevenReader and GenFM: https://elevenlabs.io/text-reader, https://elevenlabs.io/de/genfm
- Moshi: https://arxiv.org/abs/2410.00037
- BaRT (explore, exploit, explain): https://patents.google.com/patent/US11301513
- Spotify skip prediction challenge: https://www.aicrowd.com/challenges/spotify-sequential-skip-prediction-challenge
- Broadcast (format) clock: https://en.wikipedia.org/wiki/Broadcast_clock
- Apple voice processing (echo cancellation, ducking): https://developer.apple.com/videos/play/wwdc2023/10235/
- watchOS streaming: https://developer.apple.com/videos/play/wwdc2019/716;
  long-form audio routing: https://developer.apple.com/documentation/avfaudio/avaudiosession/activate(options:completionhandler:);
  double tap: https://developer.apple.com/documentation/watchos-apps/enabling-double-tap
- Background recording and App Review: https://developer.apple.com/forums/thread/776949
- FluidAudio custom vocabulary: https://docs.fluidinference.com/asr/custom-vocabulary
- NVIDIA NeMo word boosting: https://docs.nvidia.com/nemo/speech/latest/asr/asr_customization/word_boosting.html
- Porcupine wake word: https://picovoice.ai/docs/faq/porcupine/
- Podcasting 2.0 namespace (transcript, chapters, soundbite tags): https://github.com/Podcastindex-org/podcast-namespace
- Loudness target: https://auphonic.com/blog/2019/06/12/the-new-loudness-target-war/
- CarPlay: https://developer.apple.com/carplay/
- Eighty-five percent rule (Wilson et al. 2019): https://pmc.ncbi.nlm.nih.gov/articles/PMC6831579
- Desirable difficulties (Bjork and Bjork 2011): https://bjorklab.psych.ucla.edu/wp-content/uploads/sites/13/2016/04/EBjork_RBjork_2011.pdf
- Knowledge tracing (Corbett and Anderson 1994): https://link.springer.com/doi/10.1007/BF01099821
- Bloom's 2 sigma problem: https://en.wikipedia.org/wiki/Bloom%27s_2_sigma_problem
- Sycophancy in language models (Sharma et al. 2023): https://arxiv.org/abs/2310.13548
