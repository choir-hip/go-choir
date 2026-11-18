# Autoradio: the notebook and takes (2026-11-17)

Design for the next build of the Autoradio learning loop, from the owner's
rulings of 2026-11-17. It supersedes parts of
[autoradio-learning-engine-2026-11-14.md](autoradio-learning-engine-2026-11-14.md)
(the note at the top of that file names which). The theory is
[learning-engine.md](../learning-engine.md); its words are used here:
forecast, outcome, lesson, trust, the notebook. The goal file is
[choir-autoradio-learning-loop-2026-11-18.md](../definitions/choir-autoradio-learning-loop-2026-11-18.md).

Tags: **[code]** read at `origin/main` 3c887d1b3 (file:line; lines drift);
**[lane]** on a branch, not on main; **[new]** decided here.

This first commit holds only section 11 (takes), so the server and phone
builders can start. The other sections follow in the next commit.

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
**[code: `platform_publish.go:191`]**; the take route must also accept the
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
   `ended.outcome` settles the claim by the rule (section 7). Nothing new.
6. **The report.** When a forecast on a take settles `heard` or `skipped`,
   the listener's computer posts `{outcome, claim_id}` to the take's outcome
   route through the proxy, under its own computer credential. A settlement
   with no verdict (section 7) is not reported.

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
| New publisher | Record `new: no outcomes yet`. The phone puts takes into candidates newest first, so a new take is offered; the DJ forecasts it on the transcript alone. |
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
