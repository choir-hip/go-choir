# Autoradio: the notebook and takes (2026-11-17)

Standing: governing | kind: decided | settled-by: owner rulings 2026-11-17 (line 122) | note: supersedes parts of design/autoradio-learning-engine-2026-11-14.md

Design for the next build of the Autoradio learning loop, from the owner's
rulings of 2026-11-17. It supersedes parts of
[autoradio-learning-engine-2026-11-14.md](autoradio-learning-engine-2026-11-14.md)
(section 1 names which; a note at the top of that file says the same). The
theory is [learning-engine.md](../learning-engine.md), and its words are used
here: forecast, outcome, lesson, trust, the notebook. The goal file is
[choir-autoradio-learning-loop-2026-11-18.md](../definitions/choir-autoradio-learning-loop-2026-11-18.md)
(stations L1 to L9).

Tags: **[code]** read at `origin/main` 3c887d1b3 (file:line; lines drift);
**[lane]** on a branch, not on main; **[new]** decided here. Stations L1 to
L3 (outcomes on the server sheet, settlement by rule, the scorecard and
ledger route) are being built on `lane/learn-settle` and are taken as given.

## 1. What the rulings change

| 14 November design | Now |
|---|---|
| 3.1 cause classes `edition` (numbered paragraphs) and `instruction:note` | Replaced by lines of the notebook (section 4). |
| 4.1 to 4.2, 12.3 to 12.4: a separate `POST /api/autoradio/heard`, air events, `finished`/`skipped` classes, `abandoned`, `stopped late` | Dropped. The run sheet op log is the air log; the three outcomes of section 5 settle. |
| 5, guard 1: the DJ never sees a count | The DJ sees each notebook line's count (2.2). Theory section 6 puts "lessons for the role, each with its count" in the prompt. It still never sees an overall finish rate, a streak or a band table. |
| 6.2 to 6.3: lessons as `autoradio.lesson` records; a notebook route | The notebook is one Texture document (section 2). The ledger rows stay Autoradio records; the lesson text lives in the document. |
| 8: the edition is a separate stated document | The notebook absorbs it: "What you asked for" is the edition. |
| 12.1 to 12.7 literals that name the above | Superseded where they name the above; the claim, tag and id grammars that L1 built stand. |

Everything else in the 14 November design stands: one claim per content
segment, the reserved namespace, fences, rule 1 at the generic route.

## 2. The notebook: one document per listener **[new]**

The notebook is what Autoradio has been told and has learned about one
listener. It is one Texture document. The DJ reads it as prompt text; the
listener reads and edits it as prose. It is the only home of a lesson.

| Question | Decision |
|---|---|
| Name for the listener | **the notebook** (goal D2). The theory already names "the notebook" as the listener-facing home of lessons (learning-engine section 11). |
| Title and how it is found | Title `My notebook`, found by title exactly as the edition is today: `editiondoc.Title` becomes `My notebook` and `IsEditionTitle` also accepts `My edition`, so an existing edition document is adopted, not duplicated **[code: `internal/editiondoc/editiondoc.go:12,16`]**. `GET /api/texture/edition` keeps its path and answer **[code: `internal/textureowner/edition_owner.go:27,37`]**; the phone already calls it **[code: `mobile/AutoradioKit/Sources/AutoradioKit/TextureReaderSource.swift:234`]**. |
| Who creates it | Texture, at the first notebook write (section 3), when none is found. Today nothing creates it **[code: `Interview.swift:838` `UnbuiltEditionWriter` throws]**. |
| The topic list | Moves in. The phone's onboarding topics become the first lines of "What you asked for", written once by the phone through the ordinary owner revision route (an `AuthorUser` revision). After that the phone's local topic file is not read by anything. |
| `edition_text` on the programme request | Deleted. The server reads the notebook itself (section 4). The phone sends none today **[code: `Choir/RadioProgramme.swift:362-364`]**. |

### 2.1 Sections

```markdown
# My notebook

## What you asked for
- Mornings: world news first, then science.
- "Less sport." (you said, 17 Nov, during "Leeds sack manager")
- Never play takes by voice-3fa2c1.

## What Autoradio has learned
- Long policy pieces before 8 am get skipped; short ones are heard. [L7 · heard 9 of 11]
- You hear podcast clips through when the clip quotes a scientist. [L9 · heard 4 of 4]

## What Autoradio is trying
- One longer science piece after the headlines. [T2 · heard 1 of 3]

## Recent steering
- More like this: "The new Webb image" (17 Nov 08:12)
- Less like this: "Transfer window latest" (17 Nov 08:20)
```

| Section | Writer | What it holds | Bound |
|---|---|---|---|
| What you asked for | the listener (typed), or Texture writing the listener's own words verbatim (spoken steering, mutes, corrections; section 6) | instructions, authority 1 | not cut by any desk (2.3) |
| What Autoradio has learned | the DJ proposes, Texture writes | lessons, each a generalisation over many settled rows | 12 lines |
| What Autoradio is trying | the same | hypotheses the DJ is testing | 3 lines |
| Recent steering | Texture, from the phone's taps | the last 10 more/less taps | 10 lines; older taps stay in the ledger |

### 2.2 Line grammar

One Markdown bullet per line. A person reads it as prose; a program parses
the trailing tag.

| Line | Grammar | Ref (for `caused_by` and cites) |
|---|---|---|
| Lesson | `- <text> [L<n> · heard <h> of <m>]` | `lesson:L<n>` |
| Trying | `- <text> [T<n> · heard <h> of <m>]` | `try:T<n>` |
| The listener's | `- <text>` with no tag, in "What you asked for" | `mine:<first 8 hex of sha256 of the trimmed text>` |
| Steering | `- More like this: "<title>" (<date time>)` or `Less like this: ...` | `tap:<sheet entry id>` |

- `<text>` is one line, at most 200 characters, no `[` followed by `L`, `T`
  and a digit.
- `heard h of m`: of the settled forecasts whose `caused_by` was this line,
  how many were heard through. Computed by the Autoradio handler from the
  ledger (rule 1: the DJ never writes it), refreshed whenever Texture writes
  a revision, never more often. A line starts at `heard 0 of 0`.
- The line's evidence (its cites and the forecasts it caused) is in the
  ledger, not the document: the lesson's `autoradio.lesson` record holds its
  cites (at most 5 representative settled rows), what it replaced, and the
  revision that wrote it. Tapping a line on the phone shows those (section 7).
- A `mine:` ref changes when the listener edits the line; old forecasts keep
  the old ref and the ledger keeps the revision id that resolved it.

### 2.3 Edits and strikes by the listener

A revision's author tells a listener's edit from a desk's: `AuthorUser`
versus `AuthorAppAgent` with `AuthorLabel` `app:autoradio`
**[code: `internal/types/texture.go:35,42,121,127`]**. Before each notebook
write, the Autoradio handler compares the newest desk revision with every
later `AuthorUser` revision:

| The listener | Meaning | Recorded |
|---|---|---|
| deletes a lesson or trying line | struck | `autoradio.strike` with the line's id and cites **[code: kind exists, `internal/types/autoradio_records.go:33`]** |
| changes a lesson's text | struck, and the new words are the listener's | strike, and Texture moves the new text, verbatim, to "What you asked for" with `(you corrected L7)` |
| adds a line anywhere | an instruction | nothing: the document is the record |
| edits or deletes their own line | their word | nothing |

A struck line is never written again from the same evidence: a proposal
whose every cite is a cite of a struck line is refused (the 14 November
validator, 3.2, kept). The struck line's tally is frozen at the strike.

### 2.4 Size **[new]**

Owner, 2026-11-17: "less than 5 pages, ideally 1 page".

- **Target about 400 words** (one page). **Hard cap 1,000 words** for the
  desk-written sections together (about two pages), counted by Texture at
  write time. A desk revision that would pass it is refused.
- **Never a line per segment or per forecast.** Per-segment rows live only in
  the ledger.
- **Most outcomes confirm.** A confirmation raises `heard h of m` and adds no
  text (learning-engine section 5).
- **A new line must earn its place.** When "learned" holds 12 lines, a
  proposal must replace or merge (it names the lines it displaces); an add
  with no displacement is refused. The writer is told the budget and the
  current lines in the call (section 3).
- **What is cut first:** a trying line older than 20 stretches with `m` at
  least 5 is retired by rule; then the DJ's merges. Listener lines are never
  cut.
- **Listener lines are never auto-deleted.** When "What you asked for"
  passes 15 lines, the DJ may propose one consolidation; the phone shows it
  as a suggestion, and it is applied only when the listener taps Yes (an
  `AuthorUser` revision). Declined, the lines stay.

### 2.5 What is not in it

No hidden fields, no HTML comments, no front matter, no scores beyond each
line's `heard h of m`. Everything else held about the listener is in the
ledger, which the listener reads (section 7). Nothing is held only outside
both.

## 3. Who writes **[new]**

### 3.1 The DJ proposes, in the stretch call

The student writes the lesson (learning-engine section 5). No separate call:
the `extend` call that writes the next entries is shown the marked cases and
the notebook, and may add notebook lines in the same response. An `open`
call writes none. The stable prefix (the notebook) is cached; marked cases
are volatile.

The model writes, after its entries:

```json
{"kind":"notebook","op":"add","section":"learned","text":"...","cites":["calib:autoradio-dj:..."],"replaces":["L3","L5"]}
{"kind":"notebook","op":"retire","line":"T2","cites":["..."]}
{"kind":"notebook","op":"steer","turn":"<turn_id>","quote":"less sport"}
```

| Check | Failure |
|---|---|
| `section` is `learned` or `trying`; text one line, at most 200 characters | drop |
| every cite is a settled forecast shown to this call as a marked case (claim id) | drop |
| at least 2 cites for `add` | drop |
| `replaces` names live lines of that section; required when the section is full | drop |
| every cite already on a struck line | drop |
| `steer`: `quote` is a verbatim substring of that turn's request words, which this call lists | drop |
| a line id sent by the model | ignored; the handler mints `L<n>`, `T<n>` |

No text in the prompt says which way to move a number. The prompt states the
budget plainly: "The learned section holds 12 lines and has 12. A new line
must replace or merge lines, named in `replaces`."

### 3.2 The hand-off to Texture

Texture is the only writer of the document. Today the one desk-to-Texture
path is `EnsureTextureHandoff`, gated to the front-door desk, which seeds a
revision and then starts a Texture lifecycle in which Texture's model writes
**[code: `internal/textureowner/texture_handoff.go:94`; `coagent_route.go:38,214`]**.
That path does not fit: Texture's model would rewrite the DJ's words, and
the lesson would no longer be the student's.

The smallest new grant **[new]**:

1. The Autoradio handler appends one record `autoradio.notebook_patch`
   (owner-private, labelled like the other kinds), addressed to desk
   `texture`: the base revision id, the validated ops, the minted ids, the
   counts it computed, and its forecast that Texture applies it.
2. It calls one Texture function, `textureowner.ApplyNotebookPatch(record)`,
   in process. Texture checks the base revision is still current (else it
   answers `stale` and the handler rebuilds the patch on the next call),
   applies the ops to the parsed document, enforces 2.4, renders, and writes
   one revision with `AuthorAppAgent`, `AuthorLabel` `app:autoradio` and the
   patch record id in its metadata. No model call. No lifecycle.
3. Texture's answer (applied, stale, refused with a reason) is a settlement of
   the patch record's forecast by rule.

The same function serves the reads Autoradio needs (the current text, and the
revisions since the last desk revision for 2.3): a read grant from the
custodian, not a direct store read. This is red: a Texture canonical write
and a new desk-to-desk grant. `commit.texture_revision` stays with the
Texture desk alone **[code: `internal/deskmanifest/manifest.go:375`]**; the
Autoradio manifest gains only two narrow grants, `notebook.patch` (ask Texture
to apply a patch to this listener's notebook) and `notebook.read`.

### 3.3 What the ledger holds for a line

`autoradio.lesson` (the kind exists; this is its first writer): id, section,
text as written, cites, `replaces`, the patch record and revision ids. A
retire or strike appends a record; nothing is rewritten.

## 4. How the DJ reads it **[new]**

| Part of the input | Holds | Stable or volatile |
|---|---|---|
| Instructions | turns since the newest notebook revision, as today | stable **[code: `api_autoradio_programme_input.go:79-96,141-147`]** |
| **The notebook** | the whole document, each line with its ref shown (`lesson:L7`, `mine:3fa2c1d0`), one fence | stable; it changes only at a revision |
| Candidates | as today | stable |
| Marked cases (L3) | the last 40 settled forecasts: title, kind, `p_heard`, `caused_by`, outcome, claim id | volatile |
| Ahead, cause, time | as today | volatile |

The notebook replaces the `lessons` and `edition` sections
**[code: `api_autoradio_programme_input.go:148-161`]**.

**`caused_by` points at a line.** The resolver accepts `lesson:L<n>`,
`try:T<n>`, `mine:<8 hex>`, `tap:<entry id>` against the lines in this call's
notebook, plus `instruction:turn/<id>`, `importance` and `explore` as today
**[code: `api_autoradio_programme_lines.go:383-424`]**. The claim tags gain
`line:<ref>` (within 64 bytes). When the forecast settles, the line's
`heard h of m` moves at the next revision. That is how a lesson carries a
forecast and is scored (learning-engine section 5).

**Candidate order.** Unchanged: the phone sends the front list in the paper's
order, and the DJ chooses from 40 with the notebook in view. Re-ranking
candidates by the notebook before the call would need another model call or
an index; not now.

## 5. Outcomes (L1 to L3, as given)

Owner, 2026-11-17: "you will listen to a segment in full, the alternative
being you could skip it, or you could close the app entirely".

| `aired` op `ended.outcome` **[code: `docs/reference/AUTORADIO-SHEET.md`, air reports]** | Verdict |
|---|---|
| `finished` | heard through: true |
| `skipped`, `cut` | false |
| `left` (sign-out, session end), `failed` | no verdict; recorded with `position_seconds` |
| no op (crash, app killed) | no verdict; the forecast stays open; never synthesised as `left` |

The rule reads `ended.outcome`, never the entry `state` (a `left` entry is
applied with state `heard`).

## 6. Steering **[new]**

| Act | Becomes | Path |
|---|---|---|
| A more or less tap on the entry on air | a line in "Recent steering" with the entry's title and time; ref `tap:<entry id>` | the phone posts it as an `autoradio.note` record (`POST /api/autoradio/steer {entry_id, direction}`); Texture appends the line at the next notebook write, or at once if none is due within a minute |
| Spoken: "more of this", "less sport" | the listener's words, quoted, in "What you asked for", with the entry on air: `"Less sport." (you said, 17 Nov, during "<title>")` | the turn is recorded as today; the next `extend` call proposes a `steer` op quoting it verbatim (3.1); the DJ chooses which turns are steering, never the words |
| Typed | the listener's own line | they edit the notebook on the phone |
| Mute a publisher | `Never play takes by <handle>.` in "What you asked for" | the steer route with `direction: mute` (11.7) |

The referent (the entry on air) is read from the server sheet at the time of
the act. A listener line outranks a lesson: the prompt says so and the
notebook puts it first.

## 7. The listener's page **[new]**

One page on the phone, from the Autoradio tab:

1. **The notebook**, editable in place (an owner revision through the
   existing Texture routes).
2. **The scorecard** (L3's route): forecasts made, settled, heard through,
   Brier, by segment kind and cause.
3. **Behind a line**: tapping a lesson or trying line shows its ledger rows:
   the cites, then every forecast whose `caused_by` was that line, each with
   title, `p_heard` and outcome. These rows are never in the document.

**Rule 2 check.** Every record kind held about the listener, and where they
read it:

| Record kind **[code: `autoradio_records.go:23-37`]** | Read on |
|---|---|
| `autoradio.turn` (their words and the reply) | ledger route |
| `autoradio.request`, `stretch_start`, `segment`, `stretch` | ledger route (the stretch and its text) |
| `autoradio.forecast`, `autoradio.settlement` | scorecard and ledger route |
| `autoradio.sheet_op`, `sheet_snapshot` (the air log) | ledger route; the sheet route |
| `autoradio.lesson`, `strike`, `notebook_patch` | behind a line; ledger route |
| `autoradio.note` (taps, steering) | the notebook (recent steering) and ledger route |
| `autoradio.take` and outcome reports on others' takes (11) | my takes; ledger route |
| `autoradio.air`, `conflict`, `clear` | no writer after this design; L3 lists them if any exist |
| model call traces (the assembled prompt) | owner trace surfaces; named on the page as held elsewhere |

## 8. Trust for one listener **[new]**

One concrete hand-over now: **promotion**. When a lesson's line reaches `m` of at
least 10, the page offers "Make this
something you asked for?" Yes moves its text, in the listener's name, to
"What you asked for" (an `AuthorUser` revision). A learned guess becomes a
stated aim only by the listener's word (14 November design, section 8).

Deferred, honestly: wider room for `trying` lines, write-ahead and a
standing station. Each spends more of the listener's attention or the
owner's money and waits for a record worth reading.

## 9. Order of building

| Station (goal file) | Depends on | Parallel with | Class |
|---|---|---|---|
| L1 to L3 outcomes, settlement, scorecard and ledger (`lane/learn-settle`) | none | L7 phone, L8 | red, red, orange |
| L4 the notebook: title change, `ApplyNotebookPatch`, the patch record, strike detection, the `notebook` lines and validators (sections 2, 3) | L2 | L7 | red |
| L5 the DJ reads it: input, `caused_by` to lines, `line:` tags, counts (section 4) | L4 | L6 | red |
| L6 the phone: the page, steer route and taps, topics moved in, crash not sent as `left` (sections 6, 7) | L3, L4 | L5 | orange |
| L7 takes (section 11): phone recording first; server routes and kinds; the `take` line and sheet kind | L2 for settlement only | everything | red (server), orange (phone) |

The drill proof of the whole loop: on d@e, a scripted stretch with three
entries finished and two skipped settles five forecasts; the next `extend`
call proposes a line citing those claim ids; Texture writes a revision of
d@e's notebook with `AuthorLabel` `app:autoradio`; the following call's input
holds that revision.

## 10. Tension with the theory and custody

| Point | Resolution taken |
|---|---|
| Theory: "The student writes it." Custody: Texture writes canonical documents. | The DJ writes the words; Texture applies them without a model (3.2). Texture holds the pen, not the voice. |
| 14 November guard: the DJ sees no counts. Theory section 6: lessons go in the prompt with their count. | Theory wins: per-line counts are shown. No overall rate, streak or band table. |
| 14 November must-preserve: the air log "never leaves the computer and is never aggregated across listeners". Takes need outcomes to leave. | Only the outcome of a forecast on someone else's take leaves, as one hashed-key row (11.6), and the listener's ledger says it was sent. Nothing about their own listening leaves. |
| The listener's "Recent steering" taps are cut at 10 lines, against "instructions are never auto-deleted". | A tap is a signal, not an instruction; every tap stays in the ledger and lessons cite them. Spoken and typed words are instructions and are never cut. |
| Rule 1b for takes: the platform cannot read the listener's tape, so it trusts the listener's computer that a settled forecast exists. | Accepted for invited testers: one outcome per listener per take bounds the damage. A signed settlement receipt is later work. |

## 11. Takes: first build

A take is a listener's recorded voice, published so other listeners' DJs can
play it. It is the same engine pointed outward: another listener's DJ
forecasts `p_heard` on the take, that listener's act is the outcome, and the
publisher's record is the trust stroke across listeners. Owner, 2026-11-17:
"you as a user could earn distribution on your takes if another user's AI DJ
predicts that user will listen to your take in full."

It cannot be tried with real listeners yet. On staging, the drill computer
d@e stands in as the second listener, for plumbing only (11.8).

### 11.1 What exists to reuse

| Need | Existing piece |
|---|---|
| Recording | The app runs one `AVAudioEngine` with a mic tap at 16 kHz mono and has no save-to-file path **[code: `mobile/AutoradioKit/Sources/AutoradioKit/LiveAudio.swift:35,44`]** |
| Transcript | On-device `SFSpeechRecognizer` (on-device required) and `SpeechAnalyzer` on iOS 26 **[code: `AppleStages.swift:23-37,101-104`]** |
| Publishing from a computer through the proxy | The opinion path: the proxy names the user from the credential, fetches the object from the user's computer, posts to a corpusd internal route; corpusd holds the daily bound and the byline **[code: `internal/proxy/platform_publish.go:180` `HandleOpinionPublication`, `:262` `publishTextureRevision`, `:848`; `internal/platform/handlers.go:163`; daily bound `publication_opinion.go:26`]** |
| A byline without a name or email | `opinionAuthorLabel`, a stable handle from the user id **[code: `platform_publish.go:169`]** |
| A public list the phone reads into candidates | `GET /api/platform/podcast-clips` **[code: `internal/proxy/podcast_clips.go:29,51`; dispatch `handlers.go:993`]** |
| Candidates | Built by the phone from the front list **[lane: `origin/lane/autoradio-integ8` `mobile/AutoradioKit/Sources/AutoradioKit/Choir/StretchCandidates.swift:68,93`]**. The request carries `candidates[].clips` **[code: `internal/agentcore/api_autoradio_programme.go:109-131`]**. On main the phone sends no candidates **[code: `Choir/RadioProgramme.swift:362-364`]** |
| A played item that is not DJ text | `podcast_clip`: the model names a clip id from the request; it carries no forecast today **[code: `api_autoradio_programme_lines.go:268-295`]** |
| Object kinds on the platform | corpusd publication kinds in the object graph registry **[code: `internal/objectgraph/registry.go:195-222`]** |
| Platform byte store | file CAS, keyed by computer and sha256 **[code: `internal/platform/file_cas_blobs.go:33,48`]** |
| Telling drill computers apart | `TestComputer`, `OperatorProofComputer`; d@e is listed as the operator proof computer **[code: `internal/computertiers/computertiers.go:160,180`; `computer-tiers.json:37-41`]** |
| Operator removal | the operator yank path **[code: `internal/proxy/handlers.go`, `platformOperatorYankPath`]** |
| Mute, block | none **[code: grep]** |

### 11.2 Recording on the phone **[new]**

| Step | Behaviour |
|---|---|
| Start | A record button on the Autoradio page. Pressing it pauses the radio, records the entry on air as `responds_to` (11.4), and starts recording. |
| Stop | Press again, or the length bound ends it. |
| Bounds | At least 3 s, at most 60 s. The 60 s stop is automatic and says so. |
| Review | Play back, re-record (the previous recording is deleted), or discard. Nothing leaves the phone before Publish. |
| Transcript | Made on the device from the recorded file with the existing recogniser, shown before Publish. It is **not editable**: the DJ reads the transcript and the listener hears the audio, so an edited transcript could describe audio it does not match. Re-record instead. A take whose transcript comes back empty cannot be published. |
| Publish | One tap, with the line "Your voice will be offered to other Autoradio listeners. You can delete it at any time." |
| Delete | From the listener's list of their takes, at any time after publishing (11.7). |
| Audio session | `playAndRecord` while recording; back to playback after. Recording uses `AVAudioRecorder` to a file; the live turn tap stays as it is. |

Format: AAC-LC in an `.m4a` file, mono, 24 kHz, 32 kbit/s. Sixty seconds is
about 240 KB; the hard size bound is 512 KB **[new]**.

### 11.3 Where the bytes live **[new]**

1. **The listener's computer holds the original.** The phone posts the take
   to its own computer: `POST /api/autoradio/takes` (multipart: the `.m4a`,
   the transcript, `responds_to`, `duration_seconds`, `sha256`). The computer
   stores the bytes as a file and an owner-private tape record
   `autoradio.take` (label `autoradio_take`, beside the kinds of
   `internal/types/autoradio_records.go:32`). It answers the `take_id`
   (`take-` and 16 hex digits, minted by the computer).
2. **Publishing copies it to the platform**, the way an opinion is published:
   the phone calls `POST /api/platform/takes {take_id}` on the proxy; the
   proxy names the user from the credential, fetches the take from the user's
   computer, and posts it to a new corpusd route
   `POST /internal/platform/takes`. corpusd writes the take object and pins
   the bytes in the file CAS under the publisher's computer id.
3. **Public delivery** is from corpusd, never from the publisher's computer,
   so playing a take does not wake that computer (standing question 7).

Credential: a signed-in account's session or its device-login key; an
owner-wide key held by an agent is refused (a headless agent does not publish
a voice). The opinion route takes a browser session only
**[code: `platform_publish.go:192`]**; the take route must also accept the
phone's device key, so the builder reads `internal/auth/device_login.go:400`
for how to tell a device-login key from another key, and records the answer
in the route's failure list.

### 11.4 The published object **[new]**

Two new kinds in the object graph registry, owner corpusd, beside the
publication kinds **[code: `registry.go:195-222`]**:

| Kind | Identity | Fields |
|---|---|---|
| `choir.take` | external key, the `take_id` | `take_id`; `publisher` (the byline `opinionAuthorLabel`-style handle, prefix `voice-`); `publisher_key` (a hash of the owner id, never the id or email); `published_at`; `duration_seconds`; `sha256`; `bytes`; `transcript` (at most 2,000 bytes); `responds_to` (`{route, title}` of the story on air when recording started, or absent); `state` (`live`, `deleted`, `yanked`) |
| `choir.take_outcome` | content hash | `take_id`; `listener_key` (a hash of the listener's computer id under a platform secret); `outcome` (`heard`, `skipped`); `at`; `drill` (bool) |

`responds_to` keeps provenance: a take made while a story is on air cites
that story. A take with no entry on air has none.

Routes on the proxy, answered from corpusd, signed-in callers only for the
first build (11.9):

| Route | Answers |
|---|---|
| `POST /api/platform/takes` | publish one take of the caller's (11.3) |
| `GET /api/platform/takes?limit=50` | the newest live takes of the last 7 days: every field of `choir.take` except `publisher_key`, plus `audio_url` and the publisher's record summary (11.6) |
| `GET /api/platform/takes/{take_id}/audio` | the `.m4a` bytes, `Content-Type: audio/mp4`; 404 once deleted or yanked |
| `DELETE /api/platform/takes/{take_id}` | the publisher deletes their own take |
| `GET /api/platform/takes/record?publisher=<handle>` | the publisher's whole record (11.6) |
| `POST /api/platform/takes/{take_id}/outcome` | a listener's computer reports that listener's outcome (11.6) |

### 11.5 How a take reaches another listener's DJ **[new]**

1. **Candidates.** The phone reads `GET /api/platform/takes`, drops its own
   takes (by `publisher` equal to its own handle) and takes by muted
   publishers (11.7), and adds at most 3 takes to the request as candidates
   with `kind: "take"`: `{kind, take_id, publisher, title, transcript,
   duration_seconds, responds_to, record}`. `title` is the first line of the
   transcript cut to 120 bytes. The take counts toward the 40-candidate bound.
   The audio address stays on the phone, as for clips.
2. **The line.** The model writes
   `{"kind":"take","take":"<take_id>","title":"...","text":"<lead-in>","p_heard":0.6,"caused_by":"..."}`.
   Lead-in at most 400 bytes. Validators as `podcast_clip` (the id is one of
   the request's takes; once per stretch) **plus** the forecast fields as a
   story: `p_heard` in [0, 1], `caused_by` resolves. A take line without a
   forecast is dropped.
3. **The forecast.** One claim per take segment, the same constructor as a
   story **[code: `internal/calibration/autoradio.go:190-213`]**, with
   `project` = `take` and the tag `take:<take_id>`.
4. **On the sheet.** A new entry kind `take` beside `clip`
   **[code: `internal/runsheet/runsheet.go:63-72`]** in the Go reducer and its
   Swift twin, with `forecast` set like a story entry. The phone plays the
   lead-in by voice, then the take's audio from `audio_url`.
5. **Settlement.** Exactly as a story entry: the phone's `aired` op with
   `ended.outcome` settles the claim by the rule (section 5). Nothing new.
6. **The report.** When a forecast on a take settles `heard` or `skipped`,
   the listener's computer posts `{outcome, claim_id}` to the take's outcome
   route through the proxy, under its own computer credential. A settlement
   with no verdict (section 5) is not reported.

The prompt section that names `take` is appended only when the request lists
a take, as the clip section is **[code: `AUTORADIO-PROGRAMME.md`, test C7]**.

### 11.6 The publisher's record **[new]**

The record is the trust stroke across listeners: a readable public record
that the next listener's DJ reads before forecasting.

| Question | Answer |
|---|---|
| What is aggregated | Per publisher and per take: takes offered (forecasts made, reported with the outcome), heard through, skipped, and the number of distinct listeners. Counts by distinct listener, so one listener can move a publisher's record by at most one outcome per take. |
| Who computes it | corpusd, from `choir.take_outcome` rows. Neither the publisher nor the listener writes the count. |
| Rule 1 | Only another listener's device act settles. corpusd refuses an outcome whose reporting owner is the publisher (a listener's own take played back to them never counts), refuses a second outcome from the same `listener_key` for the same take (the first stands), and refuses a take that is deleted or yanked. The outcome comes from that listener's sheet op, written under their device key, settled on their computer by the rule. |
| Rule 2, publisher | The publisher reads every row about their takes: each outcome with its time and `drill` flag, under an opaque listener key. Nothing about them is held elsewhere on the platform. |
| Rule 2, listener | The listener's ledger shows, on the forecast row of a take, "reported to the publisher's record: skipped". The platform holds only the hashed key and the outcome. A skip of one take is one count on that take, not a file about the listener or a judgement of the publisher beyond that take. |
| New publisher | Record `new: no outcomes yet`. A take with fewer than 3 counted outcomes is a trial (goal D5): the phone always includes it among the 3 take candidates while it is in the list, and the DJ forecasts it on the transcript alone, cause `explore` when nothing else decided it. |
| What the DJ sees | The record summary in the candidate line: "heard through by 7 of 11 listeners over 5 takes". It is evidence about the publisher, stated plainly, not the DJ's own record. |
| Drill | Outcomes from a computer listed in `test_computers` or `operator_proof_computers` are stored with `drill: true`, shown to the publisher in a separate drill line, and never counted. |

**Earning distribution.** There is no ranking switch. A publisher earns
distribution when listeners' DJs, reading the transcript and the record,
forecast that their listener will hear the take through and so put it in the
stretch. The record makes that forecast better informed. Ordering the takes
list by record is deferred (11.10).

### 11.7 Delete, mute, bounds **[new]**

| What | Bound |
|---|---|
| Delete | The publisher deletes a take on the phone: the platform object goes to `deleted`, the bytes are unpinned and the audio route answers 404; the computer deletes its file and appends a delete record. Outcome rows stay as counts (they are about the take's past plays, and the publisher can still read them). |
| Mute | A listener mutes a publisher from the entry on air or the takes list. The mute is a listener line in the notebook ("Never play takes by voice-3fa2c1."), so it is readable and editable like any instruction; the phone also filters muted publishers out of candidates. |
| Who can publish | A signed-in account with a computer. No take is offered to anyone until its publisher is signed in. |
| Rate | At most 10 takes published per publisher per 24 hours (corpusd, like `OpinionDailyLimit`). A bound on what strangers can cause, not on our own model calls. |
| Size | 3 to 60 s; at most 512 KB; `audio/mp4` only; corpusd checks the sha256 and the duration header. |
| Outcome reports | One per listener per take; only from a computer whose own sheet holds a settled forecast for that take (corpusd checks the `claim_id` shape; it cannot read the listener's tape). |
| Operator | The yank path removes a take (`yanked`). |

**Deliberately missing, and unsafe beyond invited testers:** no review before
a take is offered; no abuse report button; no screening of audio or
transcript; no age gate; no block that stops a publisher hearing a listener
(there is nothing to hear back). The list and audio routes are signed-in
only for this reason. Opening takes to strangers needs at least a report
button with an operator queue and a published policy. That is an owner act.

**Rights.** A take is the publisher's own voice and words. No third-party
audio: recording pauses the radio, and the Publish screen says a take with
music or another person's recording will be removed. The platform holds a
copy only to deliver it; delete removes it.

### 11.8 Drill data

d@e is the second listener for plumbing. A script on d@e reads the takes
list, asks its programme route for a stretch with a take candidate, and
writes sheet ops for the take entry. Every outcome it reports is
`drill: true`, appears on the publisher's record as drill and never counts.

### 11.9 What tonight builds, and what waits

| Builds now, without other users | Waits |
|---|---|
| Phone: record, review, re-record, transcript, publish, delete, my takes list, mute (11.2, 11.7) | Real second listeners (TestFlight testers) |
| Computer: `POST /api/autoradio/takes`, the `autoradio.take` record and file | Ordering the list by record |
| Proxy and corpusd: the six routes, the two kinds, the bounds, drill marking (11.4, 11.6, 11.7) | Abuse report, operator queue, policy (owner act) |
| Programme: the `take` candidate, line, forecast, prompt section; sheet kind `take` in Go and Swift (11.5) | Takes answering takes (a take whose `responds_to` is a take) |
| d@e drill: one take published from the owner's account, played and skipped on d@e, its record showing two drill outcomes and zero counted | Any non-drill number on a record |

### 11.10 Open questions, each with its default

| Question | Default taken |
|---|---|
| Is a take's record per take or per publisher? | Both are served; the DJ sees the publisher summary. |
| May a publisher see which story a listener was hearing? | No. The outcome row carries no context. |
| Should the takes list be ordered by record? | No, newest first, until there are real listeners. |
| Does a mute travel to other devices? | Yes, because it is a notebook line. |
| Should the publisher's display name be chosen? | No, a derived handle (`voice-` and 6 hex) until accounts have names. |

## 12. Open questions, each with the default taken

| Question | Default |
|---|---|
| Does closing the app count against a segment? (goal D1) | No verdict, with position recorded (section 5). |
| The name (goal D2) | the notebook, title `My notebook`; `My edition` adopted. |
| How many settled rows before a lesson helps? | Two cites is a floor against invention, not a finding. |
| Does a one-page notebook change the DJ's picks? | The conjecture K1 (claim C120) measures it. |
| Should taps become instructions? | No; signals in "Recent steering", cited by lessons. |
| Should the DJ's lesson writing be its own call? | No, the `extend` call carries it. If the stretch call cannot do both well (entries dropped or lessons never valid over 50 calls), a separate call earns its place; the evidence is the drop reasons in the stretch index. |
| Does Autopaper read the notebook? | Not in this mission; "What you asked for" is the junction when it does. |
| Should a lesson's count include forecasts from before a model swap? | Yes. A lesson that stops holding after a swap was not a generalisation (learning-engine section 5). |
