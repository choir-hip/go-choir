# The Autoradio desk as DJ and interface: the programme script (2026-11-11)

Status: design, nothing built. Mutation class of this document: green.
Rollback: revert. Read-only: no code changed, no computer acted on, no model
called. Companions:
[autoradio-desk-fast-turns-2026-11-11.md](autoradio-desk-fast-turns-2026-11-11.md)
(the turn path; its slice 1 is this design's slice 1),
[autoradio-app-ui-2026-11-10.md](autoradio-app-ui-2026-11-10.md) (rulings 1
to 26), [autoradio-turns-2026-11-10.md](autoradio-turns-2026-11-10.md), the
goal file
[choir-autoradio-pass1-2026-11-10.md](../definitions/choir-autoradio-pass1-2026-11-10.md),
and the archived
[programming engine design](../archive/reports/autoradio-programming-engine-2026-10-14.md)
(October; much of its stream model survives here, cut down).

Evidence marks: **[code]** read in this repository at `25d6d85cd` with a
path; **[doc]** stated in a dated Choir document; **[assumed]** reasoned and
not verified; **[est]** a number derived from stated assumptions.

## The ruling this serves

Owner, 2026-11-11, verbatim: "So there needs to be a new desk, an auto radio
desk, which is the interface even to texture, right? Because texture is
showing generally individual articles, whereas auto radio is going to be
writing a script which spans over many different articles. And one of the
problems with the current interface is very much like listen to an article,
but the point of auto radio is you open the app and it starts playing, and
it's not playing any specific article, it's playing an AI DJ, which can mix
together content from articles, from podcasts, from your emails, and more.
And you can interrupt at any time, and then you can interact with any of
your different apps or desks."

Earlier the same day: "Voice prompts should go to autoradio desk which
should have a fast model that responds quickly. And the immediate responses
should be generated from a local on-device model so they are relevant,
timely, and more intelligent." And, on the turn pool: "We should actually
use River DeepSeek rather than Bedrock Haiku for this" (fast-turns note,
closing paragraph).

## The design in ten lines

1. The programme is the Autoradio desk's own data: a **run sheet**, written
   in **stretches** of about twenty minutes, each stretch an ordered list of
   cited **segments**. Texture is not its home.
2. The desk is the run sheet's sole custodian. It reads the paper (public),
   the listener's edition (Texture, by lease), mail headers (Mail, by lease)
   and podcast metadata, and writes only its own kinds.
3. **One streamed model call writes one whole stretch** (ten to fifteen
   segments). Segments stream to the phone as each is finished, so the first
   one plays in seconds and the call count stays near three per listening
   hour.
4. A second kind, the **air log**, records what the phone actually played,
   to the sentence. It is the only authority for "heard", and it feeds
   dedupe, resume and scoring.
5. **Open and it plays**: the phone plays, in order of what is available,
   the unheard rest of its cached stretch, or a headline run it builds from
   the cached public front page, while the computer wakes and streams a
   fresh stretch. The first two seconds depend on nothing but the phone.
6. A standing owner intent, the **station**, writes a stretch ahead before
   the listener's stated listening times. The intent survives a crash; a
   stretch being written when the crash happens does not.
7. **Interruption is the way in**: the on-device model acknowledges at
   once, the fast turn answers (slice 1), and work for another desk goes out
   as a record (Cast or Ask). The result comes back as a segment or as a page
   on top of the deck.
8. The desk never writes Texture, mail, files or settings. It asks the
   custodian, or it hands the phone a page to open, which is the owner's own
   act.
9. The first version mixes truthfully: told stories across several paper
   articles, headline runs, mail headers, podcast new-episode notes, station
   idents and answers. It plays no podcast audio until there is episode
   intake and a rights decision.
10. Cost is about two cents per listening hour at River DeepSeek rates
    [est], by construction rather than by any cap.

## 0. What the thing is

I used four lenses from
[the transform portfolio](../../skills/cognitive-transform-portfolio/SKILL.md)
before designing anything: object, scale separation, fate-sharing and single
writer. Each one changed the design.

**Object.** "The programme" names three different things that the current
code folds together.

| Thing | Whose | Changes how often | Today |
|---|---|---|---|
| What the listener wants: topics, less-of, sources, places, when and how long | the owner's standing intent, in prose | rarely | the "My edition" Texture, unwritten in practice [code: `internal/editiondoc/editiondoc.go:12`; the phone's `UnbuiltEditionWriter` throws, `mobile/AutoradioKit/Sources/AutoradioKit/Interview.swift:838`] |
| What the DJ plans to say next | the desk's bet | every twenty minutes, and after some interruptions | nothing; the phone plays one story [code: `ChoirFrontProgramme.load()`, `Choir/ChoirProgramme.swift:113`] |
| What was actually heard | a fact the phone observes | every sentence | partly, in a local `signals.jsonl` on the phone [code: agent survey of `mobile/`], never sent |

Each gets its own home: the edition stays a Texture (owner prose), the plan
is the run sheet (the desk's kind), and the heard record is the air log (the
desk's kind, written from the phone's reports). This answers standing
question 5 (one authority per fact). The run sheet never says what was heard,
and the air log never says what was planned.

**Scale separation.** There are three speeds, and each runs in a different
place.

| Speed | Budget | Where | Job |
|---|---|---|---|
| fast | under 300 ms | phone | acknowledge, duck, pause, resume, open a page, play the cached stretch, build a headline run from cached data |
| turn | about 2 s to first word | desk, one streamed call (slice 1) | answer about what is playing; hand work to another desk |
| programme | seconds to first segment, about 30 s for a stretch | desk, one streamed call per stretch | write the next twenty minutes |

No step on a slower layer may block a faster one. That is why the cold start
and the acknowledgement live on the phone.

**Fate-sharing** (standing question 7). If "open and it plays" needed the
listener's computer, it would share fate with the proxy, vmctl, a VM wake
(about 10 s from cold, [doc] archived engine design 7.4) and the model pool.
So the first audio after opening never waits on the computer.

**Single writer.** The script spans many articles and mixes in private mail.
If it were a Texture, then either Texture would write it (a slow agent loop,
10 to 455 s per revision in the measured turns [doc: fast-turns 1.4]) or the
Autoradio desk would write Texture data, which the ruling forbids ("the
interface even to texture"). Neither works, so the script needs its own kind
and its own custodian.

This reverses one line of the 2026-11-10 turns note ("the radio is a Texture
document played aloud ... the reserved run sheet kind may prove
unnecessary"). Today's ruling supersedes it. Texture shows individual
articles, and the DJ's script is not an article. What survives from that note
holds: lasting changes (to the edition, to a story's document) still land in
Texture, by asking Texture, and a turn is still kept on the record where it
was said.

## 1. The objects

### 1.1 Kinds and custody

| Kind | Custodian (sole reader and writer) | Holds | Status |
|---|---|---|---|
| run sheet | `desk:autoradio` | stretches of segments: the plan | the kind is **reserved** as `choir.autoradio_run_sheet`: Dolt store, content-hash identity, versioned, owner "autoradio (reserved, no writer yet)", writer `system:autoradio` [code: `internal/objectgraph/registry.go:28,84`; `writer.go:68`; `internal/kernelservice/kernelservice.go:117`] |
| air log | `desk:autoradio` | what the phone played: segment, sentences heard, how it ended | new |
| turn | `desk:autoradio` | a listener's turn and its answer | proposed in fast-turns slice 1 as `choir.app.autoradio.turn` [doc] |
| station | owner intent, with owner edits allowed; the desk reads it | on or off; listening windows; the grant that lets the desk spend on write-ahead | new; same shape as `choir.source_schedule` [code: `internal/store/source_schedule.go`] |

Naming. The fast-turns note puts the desk's kinds in the package namespace
(`choir.app.autoradio.*`). Doctrine I27 puts a kind's schema with the package
that declares it [doc: `docs/choir-doctrine.md:432-466`]. So I recommend
declaring `choir.app.autoradio.run_sheet` and `.air_log` in the `autoradio`
package, and deleting the reservation of `choir.autoradio_run_sheet` in the
same change. The reservation has no writer, so a rename should have nothing
to migrate [assumed: no rows exist; check on staging]. Standing question 3
applies to the deletion: the reservation is cited in `objectgraph/registry.go`,
`writer.go`, `kernelservice.go`, `vocabmigrate/families.go:143` and their
tests, and every one of those citers must be repaired in the same commit.
The other three reserved Autoradio kinds (`choir.media_item`,
`choir.audio_recording`, `choir.transcript`) stay reserved until podcast
intake exists (section 5).

### 1.2 A stretch

A stretch is one run sheet object: about twenty minutes of programme,
written by one model call. Its fields:

| Field | Meaning |
|---|---|
| `stretch_id` | deterministic: station id plus ordinal |
| `follows` | the previous stretch, and the segment where this one takes over (`supersedes_from`), when it replaces an unheard remainder |
| `written_for` | the cause: `open`, `extend`, `window` (the station's write-ahead), `turn` (an interruption changed direction) or `result` (a hand-off result needs placing) |
| `inputs` | content hashes of the frame: edition revision, candidate set, heard summary, mail header set, pending results |
| `segments[]` | in order (1.3) |
| `fresh_until` | the earliest expiry of any segment |
| `model`, `timings` | which model wrote it; request, first segment, done |
| `prediction` | the stretch's precommitment record id (section 4.4) |

A stretch is committed when its call completes. Segments reach the phone
before that, as each is finished (section 2.3). If the client disconnects,
the call continues and the stretch still commits, because a write-ahead is
useful later. A turn behaves differently: it cancels on disconnect [doc:
fast-turns slice 1 failure list]. If the computer crashes mid-stream, the
round closes as "interrupted by a restart" and nothing resumes. Whatever the
phone already played reaches the tape through the air log, which carries each
segment's text hash and cites (section 3.4).

### 1.3 A segment

| Field | Meaning |
|---|---|
| `segment_id` | stretch id plus index |
| `kind` | the table below |
| `title` | a short line for now-playing and the lock screen; never a raw address (walk ruling) |
| `text` | plain speakable prose, in paragraphs; the phone turns each paragraph into a block |
| `cites[]` | `{ref, role, label}`, where `role` is `subject`, `quoted` or `mentioned`, and `label` is what the screen shows |
| `est_seconds` | words divided by the speaking rate; the phone corrects it with real playback |
| `expires_at` | for example when a cited story gets a new version, or mail is answered |
| `cut_at` | where an interruption may cut: `sentence` for AI speech; `segment` for real audio, later |
| `urgency` | `now`, `next` (the next boundary) or `whenever`; only results and alerts are ever `now` |

Segment kinds for the first version:

| Kind | What it is | Cites | Example |
|---|---|---|---|
| `story` | a told story spanning several articles, each named as it is used | two to five `pub:` refs | "Three of today's stories meet at the same port ..." |
| `headlines` | a run of four to six one-line headlines | one `pub:` ref per line | "Also on the paper: ..." |
| `update` | what changed in a story the listener already heard | `pub:` at the new version, and the version heard | "The Nancy protest story has moved on since this morning ..." |
| `mail` | a briefing on new mail, from headers only | `mail:` refs | "Two new messages: your accountant about the form, and ..." |
| `podcast_note` | a new episode of a followed show, from its title and show notes | `podcast:` refs | "A new episode of a show you follow came out this morning ..." |
| `answer` | the reply to a turn, placed in the programme when it arrives late | the turn record, plus whatever it cites | "You asked about the harbour. Research found ..." |
| `result` | a hand-off came back, with a page on the deck | the Report record, and the `texture:` or `mail:` page | "Your edition now leans towards climate; it's on your screen." |
| `bridge` | a short transition, under ten seconds | none | "Closer to home, ..." |
| `ident` | a station break: time, date, what is coming | none | "It's 7:40 on Tuesday. Coming up: ..." |

Later kinds, which the schema leaves room for: `podcast_clip` and
`podcast_episode` (real audio, section 5), `desk` (the state of the
listener's computer), `alert`.

This design differs from the October engine design on one point. That
design said "a segment is never a mixture of sources". The owner's DJ "can
mix together content from articles", so a `story` segment cites several
articles. Attribution stays strict: every article is named aloud as it is
used ("the paper's story on ..."), and every spoken claim must trace to a
cite in its own segment. Real audio, when it comes, is still always its own
segment, so a human voice is never confused with the DJ's.

### 1.4 Addresses

| Ref | Resolves to | Who reads it, and how |
|---|---|---|
| `pub:<route>@v<n>#<manifest>` | a published story at a pinned version in the Autopaper object graph | public; the phone resolves it today through `/api/platform/publications/resolve` [code: `internal/proxy/platform_public.go:94`]. Whether the guest can read the front list and bundles directly is **not determined**; maild and frontimportance read `/internal/platform/publications/front` on the host [code: `internal/maild/digest.go:107`; `internal/frontimportance/scorer.go:40`]. |
| `texture:<doc>@<rev>` | the listener's own document | by a per-task, by-ref read lease from Texture (the shape of `mail.read`, fast-turns slice 3); until that exists, the phone sends the text as owner-provided data |
| `mail:<message ref>` | one message in the owner's mailbox | by lease from the Mail custodian (section 5.2) |
| `podcast:<subscription>/<episode guid>` | an episode of a followed show | needs episode intake, which does not exist (section 5.1) |
| `record:<id>` | a precommitment record (a turn, a Report) | the desk's own, or one addressed to it |

### 1.5 How the phone plays it

- **Text to on-device speech.** Each paragraph of a segment becomes one
  `.speech` block in the existing `ProgrammeDocument` [code: `Stages.swift:8,29`].
  The player synthesises a whole block before it plays [code:
  `LiveAudio.swift` `playBlock`, about line 490], so the first block of every
  segment is kept to one sentence. That keeps the time to first audio short.
  The voice is whatever voice setup loaded: today the default is
  `AVSpeechSynthesizer` [code: `ChoirApp.swift:12`; `AppleStages.swift:174-203`],
  with Kokoro and others registered but not the default
  [code: `AutoradioModels/ModelHub.swift:44-48`].
- **Real audio.** Not feasible now. The `.clip` block plays only local files
  and fixtures, and resolving an http URL returns nil, "fetch to a cache
  first" [code: `LiveAudio.swift` end, `FixtureResources.resolve`]. There is
  no `AVPlayer` in the app. Playing a publisher's enclosure needs two things:
  a streaming player node in the audio graph (phone work, orange), and an
  episode intake that knows the enclosure URLs (server work, section 5).

## 2. Open the app and it starts playing

### 2.1 The first two seconds: the phone alone

When the app opens, it starts playing. The audio comes from the first rung
of this ladder that is available:

| Rung | Condition | What plays | Depends on |
|---|---|---|---|
| 1 | a cached stretch with unheard segments still before `fresh_until` | a one-line `ident` from the on-device model ("Morning. Picking up where we left off."), then the next unheard segment | the phone |
| 2 | no fresh stretch, but `paper-front.json` is cached [code: `Layout/PaperSource.swift:30`] | an `ident`, then a `headlines` run built on the phone from the cached front list (title and dek of the top stories by importance, filtered by what the air log shows was heard) | the phone |
| 3 | nothing is cached (a first install) | the public front list is fetched (no sign-in, no computer), then rung 2 | the public platform route only |

Rungs 1 and 2 put sound out in under two seconds [assumed: AVSpeech starts
on a one-sentence block in a few hundred ms; the voice lane's measurements
settle it]. A headline run built on the phone is honest. It reads the
paper's own headlines and deks, it says so ("From Choir's paper"), and it
makes no claim the paper did not make.

Today the player starts paused (`userPaused = true`, engine not started), and
sound begins only on play, on talk, or on `choirStartRadio` [code:
`AutoradioModule.swift:116`; `Spine.swift:128`]. Slice 2 changes this to play
on open. There is no setting to turn it off ("No switches"). The pause control
is visible on the bar (ruling 21), and one tap stops it.

### 2.2 The next ten seconds: the computer joins

While rung 1 or 2 plays, the phone calls the desk:

`POST /api/autoradio/programme` (response `text/event-stream`), on the
listener's computer, authenticated the way `/api/system1/judgments` is
[code: `internal/agentcore/api_system1.go`].

| Request field | Meaning |
|---|---|
| `request_id` | idempotency key; a replay returns the committed stretch without a second call |
| `cause` | `open`, `extend`, `turn` or `result` |
| `after` | the last stretch and segment the phone holds, and the last one heard |
| `heard` | air log events not yet sent (section 3.4) |
| `local_time`, `tz` | for the `ident` and for time-of-day pacing |
| `edition_text`, `interview_notes` | slice 2 only: owner-provided data the phone already holds; slice 5 replaces them with a Texture lease |

| Event | Meaning |
|---|---|
| `stretch` | the stretch header: id, `follows`, `written_for` |
| `segment` | one finished segment (1.3), sent as soon as its last line is complete |
| `done` | committed: run sheet version, model, timings |
| `error` | one plain spoken line, plus the reason; the phone keeps playing what it has |

If the station wrote a stretch ahead and it is still fresh, the response is
that stretch, served from the store with no model call. Otherwise the desk
writes one. The phone appends each `segment` to its `ProgrammeDocument` after
the rung-1 or rung-2 material. That material ends at a segment boundary, so
the hand-over is a plain `next`.

### 2.3 One call writes a stretch

The frame, ordered so the provider's prompt cache reuses its prefix across
stretches:

1. the desk's static prompt: the voice of the station, the segment kinds,
   the output format, and the attribution rules;
2. the edition (the listener's prose), and the listener's standing signals
   ("more of this" and less-of; the app records these today in
   `signals.jsonl`);
3. the heard summary: cite refs and versions heard in the last 48 hours, and
   the last stretch's titles;
4. the candidate set: about 30 paper stories (title, dek, section,
   importance, version, and the text of the top 10 to 15), new mail headers,
   new podcast episode notes, and pending results;
5. last, the cause and the local time.

The output is one JSON object per line, one line per segment. The server
validates each line as it completes:

- the kind is known;
- every `ref` in `cites` is in the frame's candidate set;
- the text is non-empty;
- the length is within the kind's range.

A valid line is sent at once as a `segment` event. An invalid line is
dropped and noted in the stretch's `timings`. A run of three invalid lines
ends the call; the phone keeps what it has and the next `extend` tries again.
This is the same streaming and sentence work as slice 1
(`gatewayruntime.Provider` text streaming), with line-level rather than
sentence-level framing.

**Size** [est]. Speech runs at about 150 words a minute, so twenty minutes is
about 3,000 words, or about 4,000 output tokens. The input is about 20 to
30 thousand tokens, of which the first 5 to 8 thousand are a stable prefix.
At the measured 135 to 255 tok/s of comparable models [doc: fast-turns 1.5],
the first segment arrives in a few seconds and the stretch in 15 to 30 s.
River's speed is unmeasured.

### 2.4 Extending while it plays

The phone asks for the next stretch (`cause: extend`) when less than **eight
minutes** of unheard programme remains. That is long enough to cover a cold
computer, a pool walk and a full stretch call with time to spare [est].
There is one request at a time. The phone does not ask again until the
earlier stretch is done or has failed. That is the caller being correct, not
a limit.

### 2.5 Write-ahead: the station

The station is a standing owner intent. Turning it on is an owner act on the
tape: the first time the listener starts the radio after onboarding, or by
saying so. It holds:

- whether the station is on;
- listening windows, taken from the edition's "when do you usually listen"
  (the interview's question 6 [doc: app-ui section 2]);
- the spending grant (section 4.3).

The station writes one stretch, about **ten minutes before each listening
window**, unless a fresh unheard stretch already exists. With no windows,
the station writes nothing ahead, and the open path (2.1 and 2.2) does all
the work. That bounds speculative spend at one stretch per window, and each
pre-written stretch is about half a cent [est].

The mechanism reuses the source schedule's substrate: a durable wake armed
into a mailbox, carrying a generation, re-armed after each tick, with a sweep
for overdue wakes [code: `armSourcePoll`, `HandleSourcePollDue`,
`internal/agentcore/source_schedule.go:61,201,545-606`]. Whether that wake
code is generic enough to reuse as it stands, or needs extracting, is
**not determined**.

**Restarts** (CLAUDE.md; the same rule as source schedules [code:
`recoverSourceSchedulesAtBoot`, `source_schedule.go:405,478-500`]):

- The station survives any restart.
- A planned restart with its marker [code:
  `internal/agentcore/planned_restart.go:128,192`] may redeliver a pending
  window wake once.
- A crash closes an in-flight stretch write as "interrupted by a restart".
  The station stays on, with a new generation, and the next window or the
  next open writes afresh. Nothing resumes the half-written stretch.
- If two consecutive crash boots interrupt a station write, the station is
  closed and the listener is told on the next open ("I stopped writing ahead
  after two crashes; say 'start the station' to turn it back on").

**Freshness.** A stretch is fresh until its earliest segment `expires_at`, and
for at most three hours for news [est; to be tuned from the air log]. A
`story` segment also expires when a cited story gets a newer version. When
it hands a stored stretch to the phone, the desk drops expired segments and
writes anew if fewer than three remain.

### 2.6 Personalising, and not repeating

The inputs, in this order of authority (the October engine design's order,
kept [doc: archived 2.3]):

1. the listener's instructions, spoken in a turn or written in the edition;
2. their stated aspirations, from the edition;
3. importance and urgency, from the paper's importance score [code:
   `internal/frontimportance/scorer.go:41`] and the mail trust status;
4. what they heard and did: the air log, "more of this", skips, cuts.

Two further inputs are time of day (the `ident`, and pacing: a shorter
`headlines` run at 7:40 on a weekday than on a Sunday afternoon) and the
listening length from the edition.

**Dedupe is deterministic, before the model sees anything.** The candidate
builder drops every `pub:` ref whose current version the air log shows was
heard to its end, or past half of it. A story with a newer version than the
one heard stays in the set, marked "heard v3, now v5", and the model writes
it as an `update`. Mail heard in a `mail` segment is dropped the same way,
and so are episodes already noted. The model is also shown the last
stretch's titles, so its bridges do not repeat themselves.

## 3. Interruption is the way into everything

### 3.1 The sequence

| t | Where | What |
|---|---|---|
| 0 | phone | VAD; the programme dips, then pauses [code: dip-then-pause yield, `TurnEngine.swift`] |
| about 0.3 s | phone | **acknowledgement from the on-device model** (ruling 23). Its frame is the listener's words, the segment's title and cites, and the sentence that was cut. Today the phone speaks fixed holder lines, and its only local model is Qwen3-0.6B used as a classifier [code: `TurnEngine.swift:489-540`; `AutoradioModels/LlamaSystem1.swift`]. Foundation Models is not used anywhere in the app [code]. |
| about 0.2 s | listener's computer | System 1 (Jev), one call with the turn's questions [code: `ChoirSystem1.swift:68`], plus one new question (3.3) |
| about 2 s | desk | the fast turn's first spoken sentence (fast-turns slice 1, `POST /api/autoradio/turns`) |
| same stream | desk | slice 4: one `go` cell after the text, `choir.Cast` or `choir.Ask` to the desk that owns the work, and a `handoff` event |
| later | desk | the Report comes back and becomes a `result` segment, or a page on the deck (3.2) |
| after the answer | phone | resume, skip or new stretch (3.3) |

The turn request carries the programme position, which the phone already
captures. `DeskTurn` has `interrupted`, `spokenText` (whole words actually
rendered) and `cutBlockIndex` [code: `Stages.swift:93`; filled at
`TurnEngine.swift:451` from `ProgrammePlayer.spoken()`]. Slice 2 adds
`stretch_id` and `segment_id`, so a turn names the segment it cut and the
cites in front of the listener. The turn route's `page` field then means
"the segment being played, and the page on top of the deck".

### 3.2 Hand-offs: Autoradio asks, the custodian acts

| The listener says | Record (never a call) | The DJ says, in one sentence | The result comes back as |
|---|---|---|---|
| "Change my edition, less sport" | Ask to `texture:<edition doc>` | "I've asked for that change to your edition." | a `result` segment, and the edition page on the deck |
| "Make this into a document", "write me a brief on this" | Ask to `texture:<new>` with the segment's cites as sources | "I've asked Texture to write that up; it'll be on your deck." | a page on the deck when the revision lands |
| "Edit that story's document" | Ask to `texture:<doc>`; Texture decides | "Asked." | a page |
| "Find out more about X" | Cast to `research:<turn>` with hypotheses [doc: precommitment-records 4.4] | "Research is looking; I'll work it in." | a `result` segment placed by the next stretch, early |
| "Read me that email", "what did she say?" | a `mail.read` lease for that one ref [code: `internal/maildesk/maildesk.go:97,140`] | the answer, spoken from the body under the lease | an `answer` |
| "Reply to her: yes, Friday works" | Ask to the Mail desk for a draft | "Draft's ready; approve it on your phone." | the draft page on the deck. Sending stays an owner act: a key cannot send [code: `internal/proxy/email.go:30-35,76-95`] |
| "Open my notes on the harbour", "show my files" | none: navigation is the phone's | "Here." | the phone resolves and opens the page with the owner's key (`GET /api/texture/documents`, `GET /api/files`) |
| "Turn the station off", "change the voice", settings | none: settings are owner acts | "Done", from the phone | the phone performs the owner act itself; the desk is only told |
| "Approve that", "spend" | Escalate to the owner | "That needs you; it's on your screen." | an approval page |

**What "the interface even to Texture" means for custody.** The Autoradio
desk is where the listener's words enter, and the programme is where results
come out. It holds no authority over any other desk's data:

- Texture is the sole writer of revisions [doc: precommitment-records 4.1].
  Any desk may address `texture:<doc>` for a same-owner document, and
  Texture decides what to do with the delivery [doc: 4.3].
- The desk reads private documents and mail only by lease.
- Navigation and owner acts belong to the phone, because the phone holds the
  owner's key and the desk does not.

The desk's addressing entries (which desks it may address, and which may
address it) are new policy, and that change is red.

### 3.3 Where the programme resumes

The owner's rule from 2026-11-10 holds: after a turn, "in general, it's not
resuming the podcast" [doc: turns note]. So resuming is a decision, and the
decision costs no model call. It is one more typed question in the turn's
existing System 1 call [doc: turns note, "early structure lives in
questions"]:

> After answering, should the radio go back to what it was saying, move on
> to the next segment, or change direction? Options: `resume`, `next`,
> `redirect`.

| Answer | The phone does |
|---|---|
| `resume` | after the answer, an on-device bridge line ("Back to the port story."), then replay from the **start of the cut sentence** (from `spokenText` and `cutBlockIndex`) |
| `next` | a bridge, then the next segment |
| `redirect` | a bridge ("Let's go there."), then `POST /api/autoradio/programme` with `cause: turn` and the turn record. The new stretch `supersedes_from` the cut segment, and the rest of the old stretch is never played. That is one extra script call, and only when direction changes. |

If Jev is late or unsure (score under its threshold), the phone takes
`resume`. That is the conservative choice, and the next stretch can still
change course.

### 3.4 The air log: what was actually heard

The phone already knows where a cut fell, to the word. In slice 3 it reports
playback as events, batched and idempotent:

`POST /api/autoradio/heard`, a batch of `{event_id, stretch_id, segment_id,
text_sha256, cites, started_at, ended_at, last_block, last_sentence, outcome}`,
where `outcome` is one of `finished`, `skipped`, `cut` (a turn), `paused` or
`left` (the app closed).

The batch rides on the next programme request, or is sent at the end of a
session. It is buffered in `signals.jsonl` while the phone is offline. The
desk writes each event once under its `event_id`, so a replay mints nothing.
The air log is owner-private attention data. It never leaves the computer,
and it is never aggregated across listeners [doc: archived 2.8].

## 4. Models and cost

### 4.1 Which call does what

| Call | Role and pool | Per | Latency need | Stream |
|---|---|---|---|---|
| acknowledgement, resume bridge, cold-start `ident` | the phone's model (today Qwen3-0.6B via llama.cpp, or a template; Apple Foundation Models untested here [doc: `docs/reference/ON-DEVICE-MODELS.md`]) | turn, open | under 300 ms | n/a |
| System 1 questions | Jev, on the listener's computer | turn | about 0.1 to 0.3 s | no |
| turn answer | role `autoradio`, pool led by River DeepSeek V4.1 Flash (owner, 2026-11-11) | turn | about 2 s to first sentence | yes |
| stretch | new role `autoradio_programme`: River DeepSeek V4.1 Flash, then Synthetic DeepSeek V4.1 Flash 1017, then devin `swe-2-flash` | about 20 minutes of programme | first segment in a few seconds | yes |

The stretch call wants quality, not speed. It is still streamed, because a
cold open waits on its first segment. I propose the same model the owner
chose for turns. A stronger writer is a spending question for the owner
(section 7).

**Batching.** One call already writes ten to fifteen segments. That is the
batching that matters. A provider batch API (half price, hours of latency)
was rejected: the sums are a fraction of a cent per stretch, and a
write-ahead that lands hours late is useless.

### 4.2 Volume and cost per listening hour [est]

Assumptions: 150 words a minute; one listening hour is three stretches
(three script calls) and ten turns; each stretch is 25 thousand tokens in
(5 thousand cached) and 4 thousand out; each turn is 4 thousand in and 220
out; Jev is about 1.5 thousand tokens a turn. Prices are from
`internal/llmcost/pricing.go` [code]: River DeepSeek V4.1 Flash $0.12 in,
$0.0024 cached, $0.48 out per million tokens; Synthetic $0.60, $0.03, $1.20;
Jev $0.042 in; Sonnet 4.6 on Bedrock $3, $0.30, $15.

| Item | Calls an hour | River | Synthetic | Sonnet 4.6 (if chosen for stretches) |
|---|---|---|---|---|
| stretches | 3 | $0.013 | $0.051 | $0.36 |
| turns | 10 | $0.006 | $0.027 | (turns stay on River) $0.006 |
| Jev | 10 | $0.0006 | $0.0006 | $0.0006 |
| redirects (extra stretches) | about 1 | $0.004 | $0.017 | $0.12 |
| **total** | about 24 | **about $0.02** | **about $0.10** | **about $0.49** |
| station write-ahead not heard | at most 1 per window | about $0.004 a window | about $0.017 a window | about $0.12 a window |

The phone's models cost nothing to run. TTS is on the device (the goal file
excludes a server voice).

**The call count stays small by construction, not by limit:**

- one call per stretch, never per segment;
- extension starts only below eight minutes of unheard programme, with one
  request in flight;
- a redirect costs a call only when System 1 says direction changed;
- results are placed by the next stretch call, not by a call of their own;
- `resume` and `next` decisions come from System 1;
- write-ahead runs only in stated windows;
- replays are idempotent.

There is no rate cap, quota or spacing anywhere (CLAUDE.md, "No limits on
our own model calls"). Volume is logged per role, so a spike is seen.

### 4.3 Where spending is an owner act

- **Turning the station on is the grant** for unattended write-ahead spend.
  It is recorded on the tape as the owner's act, with the model and a
  per-day figure to watch.
- **The pool order** is set by the owner's ruling for turns. Using the same
  pool for stretches follows from it. Any paid model placed ahead of it
  (Sonnet, Haiku) is a new owner decision.
- All River spend counts against the gateway's platform-wide River budget,
  $10 a day by default [code: `internal/gateway/paid_budget.go:67-73`]. That
  cap is the owner's own spending cap. The station adds to it, so the owner
  should know Autoradio is now a River consumer beside self-development and
  the default pool's tail.

### 4.4 Precommitments

Every stretch is a precommitment by the desk (the goal file: "each run sheet
choice is a precommitment record with a stated probability"). Committing one
record per segment would be ten records a stretch, so it is one record per
stretch, with one typed question per segment: "heard through, p=…". The air
log settles each question by rule:

- `finished` settles yes;
- `skipped` settles no;
- `cut` followed by `resume` settles nothing, and the question stays open;
- `left` (the app closed) settles as `interrupted`, not wrong.

The desk sees its own settled outcomes when it writes the next stretch,
under the acting-pack boundary [doc: precommitment-records 4.7]. This is
slice 6. It needs two new settlement rule kinds, which the goal file already
names as lane D's work.

## 5. Podcasts and mail, honestly

### 5.1 Podcasts

**What exists** [code]:

- Podcast subscriptions are records of kind `choir.podcast_subscription`,
  holding the feed URL, title, author, artwork and the last fetch time
  (`internal/types/podcast.go:8`; `internal/store/podcast.go`).
- There are routes to subscribe, refresh and search (`/api/podcast/*`;
  `internal/agentcore/podcast.go:72,117,152`).
- A refresh imports the feed URL as one content item through
  `content.ImportURL` (`podcast.go:217-247`).

**What is missing** [code]:

- No episode parsing and no RSS enclosures.
- No audio download and no transcripts. `choir.transcript` and
  `choir.audio_recording` are reserved with no writer, and Texture media
  references default their transcript to "unavailable"
  (`internal/textureowner/texture_media_sources.go:25-30,178-181`).

Whether the imported content item keeps the feed's episode titles and
descriptions in its text is **not determined**.

**What the first version can mix truthfully.** If the imported feed text
holds the episode list, the DJ can say a new episode exists and what its show
notes say, attributed by show and publisher, as a `podcast_note` segment.
Nothing else, and only if that holds. If it does not hold, `podcast_note`
waits for episode intake.

| Step | Needs | Class | Rights |
|---|---|---|---|
| a. `podcast_note` from show notes | episode intake: parse each followed feed's items into a per-episode record (guid, title, published, duration, enclosure URL, notes), owned by the custodian of subscriptions | orange | the publisher's own metadata, attributed; no audio |
| b. `podcast_episode`: play the whole episode from the publisher's URL, introduced by name | the phone's streaming player node; enclosure URLs from step a | orange (phone) | podcast-player behaviour: stream from the publisher's host, never re-host. Owner decision (section 7). |
| c. `podcast_clip`: a minute range chosen to match a story | transcripts (the reserved `choir.transcript`, on-device or host transcription), time anchors | red where it adds a host transcription service | **needs a rights decision**: clipping and storing transcripts of third-party audio |

No third-party audio or transcript text goes in the repo at any step. Test
fixtures use our own recordings or synthetic audio.

### 5.2 Mail

**What exists** [code]:

- The Mail desk (`app:mail`, writer `desk:mail`) is the custodian of the
  grant `mail.read` and writes no kinds yet
  (`internal/maildesk/maildesk.go:1-22`; `internal/deskmanifest/manifest.go:51`).
- Messages live in maild's `email_messages` table (`internal/maild/store.go:680-701`).
- Another desk reads mail like this:
  1. it calls `RequestRead`, and the request becomes a scored precommitment
     to the custodian;
  2. the rule allows **exactly one message per task**, in the owner's
     mailbox, with `mail.read` named in the asking desk's ceiling;
  3. `Read` then serves that one message, labelled `owner_private:mail`
     (`maildesk.go:38-48,97,140`; `internal/lease/grants.go:236`).

**What is missing.** A briefing needs the headers of the last day's messages
(sender, subject, time, trust), and `mail.read` deliberately grants one
message per task. Two options:

| Option | Shape | For | Against |
|---|---|---|---|
| A. A new typed grant, `mail.headers` | one lease per stretch call, scoped to messages received since a time, giving `{ref, from_display, subject, received_at, trust_status}`, never bodies, labelled `owner_private:mail` | a small, deterministic extension of the grant table and the custodian service; no model on the Mail side | a new grant is a red change (lease, privacy) |
| B. Cast to `mail:<owner>`: "brief the radio on new mail" | the Mail desk writes the briefing in its Report, and Autoradio places it | the purest custody: the custodian summarises its own data | needs the Mail desk to run a model loop that answers casts; whether it does is **not determined** |

**Choice: A for the first version.** B stays possible when the Mail desk has
its own loop. Bodies are read only when the listener asks about one message,
through the existing `mail.read`, one ref per turn. The `mail` segment's text
is derived owner-private material: it carries the `owner_private:mail` label
in the run sheet, and the publish gate refuses anything that cites it. Mail
segments are never written ahead by the station before the mail grant is in
place (slice 3).

## 6. The phone

The app's model changes from "the loaded story" to "the programme". This is
cut to what three app lanes can absorb in one round: the engine and player,
the bar and pages, and voice with the on-device model.

| Lane | Change | Size |
|---|---|---|
| engine and player | `ProgrammeDocument` gains `segments: [{id, kind, title, cites, firstBlock, blockCount, stretchID}]` (additive, decodes as empty). This is the only model change, because blocks stay `.speech`. A `RadioProgramme` stage replaces `ChoirFrontProgramme`: it runs the cold-start ladder (2.1), streams `/api/autoradio/programme`, and appends segments. Play on open. Extend below eight minutes. Report the air log. Act on the `resume`, `next` and `redirect` answer. | about 2 days |
| bar and pages | Now-playing shows the segment's title and kind, and its sources as chips (`label`), never raw refs. Tapping a source opens that page on top of the deck (`pub:`, `texture:`, `mail:`; ruling 25). The lock screen title is the segment title, and the artist is "Autoradio" with the kind. The PROGRAMME list in the player sheet lists segments, not blocks. The reader's "Play from here" becomes **"Ask the DJ to read this"** (next table). | about 1 to 2 days |
| voice and on-device | The acknowledgement comes from the on-device model, with the segment's title, cites and cut sentence in its frame (ruling 23), replacing fixed lines. Also the resume bridge line and the cold-start `ident`. The `resume`, `next`, `redirect` question is added to the System 1 set. | about 1 to 2 days |

**"Ask the DJ to read this."** Reading a page aloud word for word is not a
model's job, because the phone already has the text. So the action:

1. inserts a `read` segment at the next boundary: an on-device line ("Here's
   the paper's story on ..."), then the page's paragraphs as blocks, citing
   the page's ref;
2. reports it in the air log, so the desk knows it was heard and does not
   retell it.

It costs no model call. The existing mail "Read it to me" [code:
`mail.contract:164`; `speakText`, `LiveAudio.swift` about 455-480] becomes
the same action.

What does not change: the deck, the reader, sign-in, voice setup and the
interview. The interview remains a programme of its own (`InterviewProgramme`
[code: `Interview.swift:738`]). Its end should write "My edition", but that
writer is still unbuilt [code: `Interview.swift:838`]. Slice 2 therefore
also accepts the interview notes directly (2.2).

## 7. Slices

Each slice lands on its own, carries `Ring-Expedite`, and is proved on
staging against d@e only.

### Slice 1: the fast answer (in flight, fast-turns note)

Unchanged, with one addition: the turn request accepts the optional
`programme: {stretch_id, segment_id}` field now, so slice 2 needs no change
to the route.

### Slice 2: open and it plays a mixed programme (two days)

**The roughest honest version.** You open the app and within two seconds it
plays a headline run from the cached front page. Within about ten seconds a
DJ stretch written for you takes over: told stories across several articles,
headlines, idents and bridges, about twenty minutes long, with sources on
screen. It extends itself as it plays. Mail and podcasts are not in it yet,
and the DJ does not pretend otherwise.

Server:

- the package registers the run sheet kind, and the reservation is deleted
  with all its citers;
- the role `autoradio_programme` and its pool;
- `POST /api/autoradio/programme` (SSE), with one streamed call per stretch
  and line-level validation;
- the candidate builder over the front list.

The guest reads the front list itself if it can. The first hour of the slice
settles this, using the host route maild already uses. If it cannot, the
phone sends the candidate set (title, dek, route, version, and the text of
the top ten) as data, as slice 1 sends page text. The edition and interview
notes arrive as owner-provided data.

Phone: the engine and player lane's work in section 6 except the air log, and
the bar lane's now-playing and source chips.

| | |
|---|---|
| Class | **red**: gateway and provider calls (a new streaming role), tape and evidence (the run sheet kind gets its first writer; the deletion of a reserved kind), auth and session (a device-key route) |
| Protected surfaces untouched | Texture canonical writes (none), vmctl, deploy routing, checkpoint projection |
| Review | two seats (Gemini 3.8 flash through `agy`, then Sonnet); the author is not one |
| Failure list, before code | no auth 401; unknown field 400; replay of `request_id` returns the committed stretch with no second call; first `segment` is flushed before the call ends (a delaying fake provider); a cite outside the candidate set drops the line; three invalid lines end the call cleanly; client disconnect lets the stretch commit; crash mid-call closes the round "interrupted by a restart" and does not resume; dedupe drops a story heard at its current version and keeps one with a newer version as `update`; the reserved kind's deletion leaves no citer (a grep test) |
| Deployed proof | after CI and deploy, verify the deployed commit. Run a script from a host off Node B that opens a programme ten times against d@e and prints, per run: first byte, first `segment`, `done`, segment kinds, every cite resolved against the public resolve route, and the model. Pass: first segment at most 6 s median from a warm computer; every cite resolves; at least two kinds besides `ident` per stretch; no Texture document created. Then on the iPhone: open the app cold with the computer asleep, record audio, and show that sound starts within 2 s and a desk stretch takes over without a gap. Artifacts: the table and the recording. |
| Precommit before landing (calibration ledger) | the proxy streams the programme SSE unbuffered, 85%; first segment at most 6 s median warm, 70%; at most 1 in 10 stretches has an invalid line, 60% |
| Rollback | revert. The route, role and kind registration go together, and stretches already written stay on the tape as history. The app falls back to the cached front page. |
| Expedite | `Ring-Expedite: the radio plays one article, not a programme` |

### Slice 3: mail headers (one day; can run in parallel with slice 2)

The `mail.headers` grant (option A, 5.2): the Mail custodian serves
`{ref, from_display, subject, received_at, trust_status}` since a time, one
lease per stretch call. The candidate builder adds new headers, and the desk
writes `mail` segments.

- **Class:** red (lease grant, owner-private data).
- **Protected surfaces:** leases and the Mail custodian, tape.
- **Review:** two seats.
- **Failure list:** no `mail.headers` in the ceiling, refused; bodies never
  in the answer; quarantined mail excluded; the label is carried into the
  segment; the publish gate refuses a document citing a `mail` segment; one
  lease per run.
- **Deployed proof:** send two test messages to d@e, open the programme, and
  hear a `mail` segment citing both refs, with the lease record on the tape.
- **Rollback:** revert. The grant disappears, and mail segments stop
  appearing.

### Slice 4: the air log and the resume decision (one to two days)

- The air log kind and `POST /api/autoradio/heard`.
- Dedupe from the air log, instead of the phone's summary.
- The System 1 resume question; `redirect` as a stretch with
  `supersedes_from`.
- **Class:** orange (a new owner-private kind, a route, a question).
- **Review:** one seat.
- **Proof:** listen to two segments, skip one, cut one, reopen the app, and
  show that the next stretch omits what was heard and the air log shows four
  events.
- **Rollback:** revert.

### Slice 5: hand-offs and results (fast-turns slices 2 and 3, here)

- The `go` cell with Cast and Ask.
- Addressing policy entries.
- The by-ref Texture read lease, which replaces the edition text sent by the
  phone.
- `result` segments placed by the next stretch, and pages pushed to the deck.
- **Class:** red (desk authority, leases).
- **Review:** two seats.
- **Proof:** say "less sport in my edition" while playing; Texture writes the
  revision, the edition page comes to the top of the deck, and the next
  stretch carries less sport.

### Slice 6: the station and scoring

- The station kind, write-ahead in windows, and restart handling.
- One precommitment per stretch, settled by the air log.
- The desk reads its settled outcomes.
- **Class:** orange for the station, yellow for the scoring views. The
  station's grant is an owner act.
- **Proof:** set a window ten minutes out; a stretch is written before it;
  crash the guest once mid-write on d@e, and the station survives with the
  stretch closed as interrupted; two crashes close the station.

### Slice 7: podcasts, step a, then b

Episode intake and `podcast_note`, then whole-episode playback after the
owner rules.

## 8. Options rejected

- **The programme as a Texture document** (the 2026-11-10 turns note). The
  ruling separates them: Texture shows articles, the DJ writes scripts. It
  would also put the slow revision path in the programme loop, and either
  make Texture write every stretch (two agent loops per stretch) or let
  Autoradio write Texture data.
- **No persistence: stream and forget.** No dedupe, no resume after a
  restart, no scoring, nothing on the tape. Against doctrine.
- **One call per segment.** Ten times the calls, worse coherence across
  segments ("closer to home" needs to know what came before), and a frame
  re-sent each time.
- **Writing the whole day ahead overnight.** Rejected by the owner in October
  ("responsiveness beats precomputation"), and most of it would be stale or
  never heard.
- **A phone-only scheduler over public stories.** Fast, but no mail, no
  records, no desk, and every front end would have to repeat it.
- **Shared per-story scripts written once at the platform** (the October
  design). A good later saving at scale. But the DJ's told stories span
  articles per listener, and it adds a platform writer. Not first.
- **Server-rendered audio.** The goal file excludes a server voice.
- **A provider batch API for write-ahead.** Hours of latency to save a
  fraction of a cent.
- **Two AI hosts, or banter.** The October ruling was "no banter, no fake
  hosts" (paraphrased there). The DJ is one voice, with a personality in its
  phrasing and no second host. The owner can overrule this.
- **The fast turn rewrites the rest of the programme.** That would put a
  second, long output into the 2-second path. Instead the turn answers, and
  System 1 decides whether a new stretch is needed.
- **Reusing `mail.read` in a loop for a briefing.** It is one message per
  task by design. Looping it would be working around a custodian rule.
- **A setting to disable auto-play.** A switch. Pause is one visible tap.

## 9. Questions that are the owner's

1. **The station's spending grant.** Turning the station on lets the desk
   spend River money writing ahead before your listening windows, about half
   a cent a window and about two cents a listening hour [est], inside the
   $10 a day River cap shared with self-development. Is the first "start the
   radio" after onboarding the right owner act to grant it, or should it be
   asked in words? Default if no answer: the first start grants it, the
   grant is stated aloud once, and it is recorded on the tape.
2. **A stronger writer for stretches?** River DeepSeek (about $0.02 an hour)
   is the default, following your turn ruling. Sonnet 4.6 would be about
   $0.49 an hour. Default: River, and judge by ear.
3. **Podcast audio.** Playing a followed show's whole episode from the
   publisher's own URL is ordinary podcast-player behaviour. Clips and stored
   transcripts of third-party audio need a rights decision. Default: notes
   only now; whole episodes after your yes; no clips until a rights review.

The retention of turns and the air log joins fast-turns question 2 (how long
listening records are kept). It is not asked again here.

## 10. What I could not determine

- Whether the guest can read the public front list and publication bundles
  itself, or only through the phone. maild and frontimportance read a host
  route; I did not trace a guest path.
- Whether `content.ImportURL` of a feed keeps the episode list and show notes
  in its text.
- Whether the Mail desk runs a model loop that can answer a Cast (option B).
- Whether the source schedule's durable wake is generic enough to reuse as
  the station's tick without extraction.
- River DeepSeek's time to first token and throughput. Only Synthetic's
  DeepSeek was measured (135 tok/s, whole call) [doc: fast-turns 1.5].
- Whether the reserved run sheet kind has any rows on staging. I assumed
  none.
- How long AVSpeech, or the loaded voice, takes to first audio on a
  one-sentence block on the floor device. This is the voice lane's
  measurement.
- Whether Apple's on-device model is available to the app, and how well it
  writes acknowledgements. The app does not use it today.
- The listener's real wake time from hibernation on staging today. The 10 s
  figure is October's estimate.
