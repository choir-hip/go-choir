# Where a voice turn's time goes, and the Autoradio desk (2026-11-11)

Status: design and investigation, nothing built. Mutation class of this
document: green. Rollback: revert. Read-only: no code was changed and no
computer was acted on. No run was started (the four real turns below were
enough), so no fresh timed ask was made on the test computer.

Why this exists. After the first phone walk the owner said a voice prompt is
"super duper slow to respond" and ruled (ruling 23 of
[autoradio-app-ui-2026-11-10.md](autoradio-app-ui-2026-11-10.md)): voice
prompts go to an Autoradio desk with a fast model; the immediate response is
made on the phone by a local model (another agent's lane). This note is the
server side: what the time is spent on today, and the desk that replaces it.

This note also carries the problem record that CLAUDE.md asks for before a
fix ("Problems come first"): section 1 is the evidence, section 2 the belief
and the remaining error.

## 0. Method and what is measured

- Sources: the app source on origin/main (`mobile/AutoradioKit`), the server
  source, the gateway journal on Node B for the test computer d@e
  (`go-choir-gateway`, metadata lines only: provider, model, tokens,
  timestamps to the second), and the durable ledger of d@e read with the
  `choir` CLI and d@e's own read key (`choir run list`, `choir texture
  history`, `choir lifecycle events`, `choir policy get`). Nothing under
  `/var/lib/go-choir/secrets` was read. The owner's computer and the paper
  were not touched.
- Tags: **M** measured (a log line or ledger row with a timestamp), **C** read
  from code, **I** inferred (stated with its reason).
- Resolution: gateway log lines are to the second; ledger rows (revision and
  event `created_at`) are to the millisecond. A call duration taken from the
  gateway log is therefore good to about one second. Every model call in these
  traces was non-streamed (`stream=false`, **M**), so no time to first token
  exists anywhere in the data. That number is unmeasured and is the first
  thing slice 1 must emit.

## 1. Where the time goes

### 1.1 The app side (C)

`LabRuntime.ask(text)` calls `TurnEngine.typedTurn`, which runs the same
pipeline as a spoken turn (`runPipelineBody`, `TurnEngine.swift`). On one turn
it starts, at once and beside each other:

1. the **desk stage** (`ChoirDeskStage`, `Choir/ChoirDesk.swift`), which sends
   the words to the listener's computer and waits for a document revision;
2. **System 1** (`ChoirSystem1`): one `POST /api/system1/judgments` with every
   question of the turn (Jev, 2 s deadline in the engine); it writes no text;
3. the **holding lines** (fixed stock lines spoken by `AVSpeech`) until a
   revision arrives, with a 120 s hold timeout and a 110 s desk timeout.

The desk stage's request depends on the turn:

| Case | Request |
|---|---|
| First turn on a published story (no document yet) | `POST /api/texture/fork` with the words, the place in the story as prose, and `source` = publication version id + manifest hash |
| Later turn on a document the fork made | `POST /api/prompt-bar` with `focus` = Texture + the document id |
| No story in front | `POST /api/prompt-bar` unfocused (routed) |

The answer comes back by **polling**: `GET /api/texture/documents/{id}/stream
?format=json&after=<cursor>` every 1.5 s (`pollSeconds`), then
`GET /api/texture/revisions/{id}` for each new `version` event. Only a revision
with `author_kind == appagent` and a version above the listener's own is
taken. The revision body is the **whole document**; the app diffs it against
the paragraphs it already has and speaks from the first paragraph that
differs (`revision(id:content:...)`). Speech of the real answer therefore
cannot start before a complete revision is committed on the server and the next
poll sees it. There is no token stream to the phone anywhere on this path.

### 1.2 The server side (C)

| Hop | What | Model | Source |
|---|---|---|---|
| 1 | phone to `choir.news` proxy to the listener's computer | none | proxy, vmctl |
| 2 | `HandlePromptBar` / `HandleTexturePublicationFork` | none | `internal/textureowner/prompt_bar.go` |
| 3 (unfocused only) | the prompt router: one Jev call over the registered desks' `accepts` descriptions; a route record is frozen; failure takes the default (Texture) | Jev (System 1), no text | `internal/promptroute`, `internal/agentcore/prompt_router.go` |
| 4 (unfocused only) | a conductor run is created and completes with no model | none | `submitPromptBar` |
| 5 | a **new Texture document** is created (or the focused one revised) and the owner revision is committed; the Texture desk is woken with intent `apply_owner_revision` | none | `reviseDocument`, `EnsureTextureHandoff` |
| 6 | the Texture desk's **agent loop**: the model writes Go cells (`tools=1`), each cell result is a further call; the system prompt is 15,337 characters; on d@e the role runs at `reasoning=high` | pool `default`, which leads with devin `swe-2-flash`, then 8 fallbacks | `internal/modelpolicy`, `internal/modelcatalog/catalog.go` (`Pools`) |
| 7 | when the desk judges it needs evidence, it casts to the research desk and ends its turn with a "waiting" commit; research runs its own loop (23,464-character prompt, web search and fetch) and reports back; Texture wakes again and writes the answer revision | same pool | ledger, section 1.3 |
| 8 | the revision is committed; the document stream gains a `version` event | none | `texture_turn_committed` |
| 9 | the phone polls (hop 1 again, twice) | none | `ChoirDeskStage.run` |

Every model call in hops 6 and 7 goes through `CallWithTools`, which sets
`Stream: false` unconditionally (`internal/gatewayruntime/provider.go:179`,
`call` forces it again). The gateway can stream (`handleStreamingInference`,
`internal/gateway/handlers.go`, SSE, and all nine providers implement
`Stream`) and the guest provider has a streaming method (`stream`, used by
`Execute`), but the desk loop never uses them. The prompt-bar response itself
carries no answer: it is a 202 with ids.

### 1.3 Four real turns (M unless marked)

All four were asked by the app on d@e (the user revision text carries the
app's own phrasing, "The listener was hearing paragraph 1 of 6 of ..."). I
found **no** app-shaped asks on d@e between 02:00 and 05:40 UTC on 2026-11-11:
the model traffic in that window is management, Texture-integration and
engineering rebase work (`integrate_execution_findings`, rebase runs). The
four asks that exist are two at 23:26 UTC on 2026-11-10 and two at 07:02 and
07:03 UTC on 2026-11-11. Revision times are from `choir texture history`;
event times from `choir lifecycle events`; model rows from the gateway
journal.

**Turn A** (2026-11-10, "Please tell me more about the harbor in the park",
unfocused, routed to Texture by Jev; command `autoradio-turn-2cad921b`).

| Time (UTC) | Hop | Model | Duration |
|---|---|---|---|
| 23:26:21 | two Jev calls (2 questions 819 bytes, 1 question 1,317 bytes), upstream 132 and 94 ms; the larger is the router, the smaller the app's System 1 (I: by size) | Jev | 0.1 s each |
| 23:26:21.67 | conductor run created, route record `judge` to texture, completes with no model | none | under 1 s |
| 23:26:22.07 | owner revision committed in a new document | none | |
| 23:26:22.59 | Texture run created | none | 0.5 s after the revision |
| 23:26:22 to :24 | the loop's first call is refused or skipped by 9 pool members in turn: devin (quota), opencode-go iceman (429, then circuit open), beam-free (parked, three tries), openrouter flint and nemotron (quota), chatgpt luna (quota) | none answered | about 2 s |
| 23:26:24 to :34 | one call on bedrock `claude-haiku-5-5`: 9,002 in (5,752 cached), 1,642 out, text empty (the output is a cell holding the revision) | haiku 5.5 | **10 s** |
| 23:26:34.31 | first appagent revision committed | | |

Ask to first answer revision, server side: **12.2 s** from the owner revision
(about 12.6 s from the conductor run). Research (woken at 23:26:34) added
revisions at 23:27:01, 23:27:10 and 23:27:30.

**Turn B** (the same words, 20 s later; command `autoradio-turn-45d1c17a`).

| Time (UTC) | Hop | Model | Duration |
|---|---|---|---|
| 23:26:42 | two Jev calls (upstream 114 and 88 ms), conductor run created (:42.74) | Jev | 0.1 s |
| 23:26:43.09 | owner revision committed | | |
| 23:26:43.51 | Texture run created | | |
| 23:26:43 to :48 | call 1, bedrock haiku 5.5, devin refused first: 17,310 in, 1,006 out | haiku | 5 s |
| 23:26:49 to :50 | call 2 (devin refused first), 321 out | haiku | 1 s |
| 23:26:51 to :52 | call 3, 163 out | haiku | 1 s |
| 23:26:53.79 | first appagent revision committed | | |

Ask to first answer revision: **10.7 s**, three sequential calls.

**Turn C** (2026-11-11 07:02:09, "What is the top story on the front page
right now?", asked in a story about school protests in Nancy; first turn on a
published story, so a fork; trajectory `e1748683`).

| Time (UTC) | Event | Since ask |
|---|---|---|
| 07:02:09.56 | trajectory started; fork creates the document and two owner revisions (:09.56, :09.68). One Jev call, 3 questions, upstream 84 ms (the app's System 1; no router on the fork path) | 0 |
| 07:02:10 to 07:03:18 | Texture loop on devin `swe-2-flash`, `reasoning=high`: calls of about 31 s (7,904 in, 2,728 out), 16 s (1,704 out), 3 s, and more (I: attribution of individual calls is blurred by Turn D running at the same time) | |
| 07:03:18.54 | `texture_turn_committed`: **"waiting on research to identify today's top front-page stories"**. No revision is written, so the phone sees nothing | 69 s |
| 07:03:18 to 07:09:53 | research run `7674d5dc`: web search and page fetches on `swe-2-flash`, then the opencode-go `iceman` model | |
| 07:09:45.09 | `texture_turn_committed` "answer": the first appagent revision, the answer to the listener | **455.5 s** |
| 07:25:17 to 07:31:45 | six more commits that repair unsupported citations in the answer | 16 to 22 min |

**Turn D** (07:03:02, a second question of the same kind on another document;
trajectory `34651b86`): "waiting" commit at 07:04:39.97 (97.6 s), answer at
07:09:52.71 (**410 s**). Across Turns C and D together the computer sent 96
model requests (89 succeeded) in the 7 minutes 44 seconds from 07:02:09 to
07:09:53, about 1.84 million input tokens and 76,700 output tokens on the
successes; 58 requests carried the Texture prompt and 38 the research prompt
(**M**, gateway journal; computer-wide, and the computer was otherwise idle
except for background Texture integration).

### 1.4 Totals

| Measure | Turn A | Turn B | Turn C | Turn D |
|---|---|---|---|---|
| Ask to first server token | not measured (no streaming anywhere) | same | same | same |
| Ask to first appagent revision committed (**M**) | 12.2 s | 10.7 s | 455.5 s | 410 s |
| Revision committed to the phone seeing it (**I**) | mean 0.75 s, up to 1.5 s (poll interval) plus two GETs, 0.2 to 0.5 s | same | same | same |
| Phone sees it to first spoken word of it (**I**) | on-device speech start, outside this note | | | |

Turns C and D exceed the app's own 110 s desk timeout (`timeoutSeconds` 110,
and the engine's 120 s hold), so on the phone these two would have ended with
"the desk wrote no revision in 110 s" while the desk was still working (I: from
the code; the phone's own log was not available). The owner's complaint is
this: a question about the page is answered by an edit of a document by an
agent that researches.

### 1.5 Model speed in the gateway journal (M, whole-call wall time)

Pairs of "inference request" and "inference succeeded" on the same computer
and provider with no other call of that provider in flight, 2026-11-10 12:00 to
2026-11-11 12:06 UTC, all computers, all `stream=false`. Inputs are large
(10 to 35 thousand tokens) because they are agent-loop prompts, so these
numbers overstate a short grounded answer, and TTFT cannot be derived.

| Provider and model | Calls | Output at most 250 tokens | 250 to 1,000 | Throughput, 800+ out |
|---|---|---|---|---|
| bedrock `claude-haiku-5-5` | 167 | median 2 s, p90 2 s (n=23) | 3 s | 255 tok/s |
| opencode-go `beam-free` | 181 | median 1 s, p90 2 s (n=61) | 3 s | 187 tok/s |
| synthetic DeepSeek V4.1 Flash 1017 | 402 | median 2 s, p90 3 s (n=85) | 5 s | 135 tok/s |
| devin `swe-2-flash` | 136 | median 2 s, p90 4 s (n=81) | 5 s | 113 tok/s |
| opencode-go `iceman-contributor-free` | 166 | median 4 s, p90 6 s (n=27) | 12 s | 57 tok/s |

Reliability, from the same journal (2026-11-11 00:00 to 12:06 UTC): of 20,581
devin requests, 1,532 were refused with "quota exhausted" (paced), 7.4%; the
journal also holds 1,546 "circuit open" refusals and 38 "every OpenCode key is
parked". Each refusal is a hop the pool walks before an answer (Turn A walked
nine). Jev, by comparison, answered in 84 to 183 ms upstream across these
turns, but it writes no text.

### 1.6 The cause (the biggest, with its evidence)

**A question about the page is handled as a document revision by the Texture
desk's agent loop.** The ledger shows it directly: Turns C and D end in a
"waiting on research" commit and then an answer 410 to 455 s after the ask;
Turns A and B, which needed no wait, still take 11 to 12 s because the answer
is a Go cell that carries a rewritten document, produced by a non-streamed
call (10 s for 1,642 output tokens in Turn A), after a walk through dead pool
members (2 s), with each ask also creating a new private document. Polling
adds under 2 s and the Jev calls under 0.2 s: they are not the problem.

Ranked by what they cost a short question about the page:

1. the research wait and the multi-call loop: 410 to 455 s (C, D) (**M**);
2. the revision-as-answer: one non-streamed call whose output is the whole
   document, 5 to 10 s (A, B) (**M**);
3. the pool walk past refused members: about 2 s on a bad moment (A) (**M**);
4. the poll plus two GETs: 0.75 to 2 s (**I**);
5. everything else (Jev, conductor, document creation): about 1 s (**M**).

## 2. Belief and remaining error

Belief (about 85%): moving the listener's question off the revision path and
onto one streamed model call with the page in the request removes items 1, 2
and 4, and a dedicated pool removes item 3. Remaining error: TTFT of any
model on a 2 to 3 thousand token frame is not measured; the proxy and vmctl
hops are not logged, so I cannot say what they cost or whether the proxy
flushes an SSE body promptly (it already carries the Texture SSE stream, which
makes it likely); the owner's phone asks were not located (section 7).

## 3. The Autoradio desk

### 3.1 What it is and what it owns

A **package desk**, registered the way Mail is: package `autoradio`, desk
`app:autoradio`, writer `desk:autoradio`, an in-tree first-party package under
a reserved slug (`firstPartyAppSlugs` in `internal/deskmanifest/manifest.go`;
manifest in `internal/appdesk/firstparty.json`; prompt in
`internal/appdesk/firstparty_prompts/autoradio.md`, as the drill desk does).
It is not a special desk and has no exemption (I26).

It owns one data kind, `choir.app.autoradio.turn`: the listener's turn about
a page (the words, the page address and a hash of the text it was given, the
place heard, the spoken answer, the hand-offs it made, the model that
answered, and the hop timings). It has sole read and write of that kind.
It reads the page in front as data (taint class `untrusted_input`: page text
is world content and is labelled so in the frame; the desk can hold no
privileged authority). It writes no Texture revision, does no research and
runs no engineering.

Its job: a short spoken answer to the listener's turn about the page in front,
grounded in the page text and what the paper holds about it, in the voice of
the radio; or, when the turn needs real work, an honest short statement of
what it will do and a record to the desk that does it.

### 3.2 What it hands to other desks, and what it says then

| The turn needs | Hand-off (a record, never a call) | What the listener hears |
|---|---|---|
| Evidence the page and the paper do not hold | a Cast to the research desk for this turn, with the question and the page address, and a stated hypothesis and probability | "I don't have that in front of me. Research is looking; I'll work it into the programme." |
| A change to the listener's edition or a story's document | an Ask to `texture:<doc>`; Texture, the sole writer, decides and writes the revision | "I've asked for that change; you'll hear it when it lands." |
| A thing only the owner can do (send mail, spend) | an Escalate to the owner, as the doctrine has it; no act | a plain statement that it needs the listener's approval |

Records ride the ledger (precommitment-records.md, 4.1 to 4.3): the desk's
reply to the listener's turn is itself a Report that settles the owner-origin
request record; each hand-off is a Cast or Ask with a prediction ("research
finds this within five minutes", probability), scored when the listener next
acts, as the turns note already specifies. Work that lands later reaches the
phone through the path that exists today: the Texture revision of the
edition, followed by `ChoirDeskStage`. That slow lane stays as it is; it is
now the hand-off lane, not the answer lane.

This reconciles [autoradio-turns-2026-11-10.md](autoradio-turns-2026-11-10.md)
("the radio is a document played aloud; speaking is prompting the document;
no routes") with ruling 23. The document remains the place where lasting
changes land; the answer to a spoken question is no longer a revision of it.
See open question 4.

## 4. The request path

One new route on the listener's computer, modelled on
`POST /api/system1/judgments` (`internal/agentcore/api_system1.go`: a signed-in
client on a device key, in-guest, one call):

`POST /api/autoradio/turns` (response `text/event-stream`)

Request (unknown fields refused, as the prompt bar does):

| Field | Meaning |
|---|---|
| `turn_id` | the client's command id; the idempotency key |
| `words` | what the listener said or typed, verbatim |
| `page.address` | the page in front, as the app holds it: `pub:<route>` with version and manifest hash, or `texture:<doc>@<revision>` |
| `page.title`, `page.text`, `page.text_sha256` | slice 1: the text the phone is showing (data, at most 48 KB, as System 1's state bound); slice 3 replaces `text` by server-side resolution of the address |
| `heard` | block index and the last 300 characters spoken (what the cut-in interrupted) |
| `programme` | the edition document id when there is one |

Events: `speech` (a sentence, in order), `handoff` (desk, record id, one line),
`done` (turn record id, model, per-hop timings), `error` (a plain spoken
fallback line plus the reason).

Server steps and their budgets:

1. authenticate the device key, validate, replay if `turn_id` was answered (no
   second model call): 20 ms;
2. commit the owner-origin request record to `app:autoradio` (the prediction
   goes on the tape before the answer): 50 ms;
3. build the frame, ordered for the provider prompt cache: static desk prompt,
   edition context, page text, heard, then the words last: 10 ms;
4. resolve the model for role `autoradio` and call it **once, streamed**, with
   no tools in slice 1 (the answer is the model's text), `max_tokens` about
   220, reasoning at most `low`;
5. split the text stream into sentences and write each as a `speech` event as
   it completes, flushing at once;
6. on completion commit the Report record (the answer, model, timings) and
   send `done`.

Hops this skips: the router and its Jev call, the conductor run, document
creation, the Texture wake and its 15 K-token prompt, the cell loop, the
research wait and the poll plus two GETs. A simple question about the page in
front costs **one** model call.

Speak first, then act (slice 2). The same streamed response may carry text
and then one cell (the desk's single `go` tool: `choir.Cast(...)`). Text
deltas stream to the listener first; the cell runs after, which is how a
hand-off costs no spoken time. No legacy tool call is added: the hand-off is
the in-cell module surface the other desks already use.

## 5. The model, and the budget

A new pool `autoradio` in `modelcatalog.Pools`, and a role `autoradio`
resolved from the computer's policy like every other (`[roles.autoradio]`):

| Order | Member | Why |
|---|---|---|
| 1 | bedrock `claude-haiku-5-5` | fastest observed whole-call (median 2 s, p90 2 s on 10 to 35 K-token prompts; 255 tok/s), no bedrock refusal appears in the 2026-11-11 failure counts, already routed |
| 2 | devin `swe-2-flash` | 2 s median, free until 2026-12-04, but 7.4% quota refusals |
| 3 | synthetic DeepSeek V4.1 Flash 1017 | 2 s median, 135 tok/s |
| 4 | opencode-go `beam-free` | 1 s median but parked often; last resort |

The order is not the default pool's (free first) because the cost of a
refusal here is audible silence, not a retried agent step. It puts a paid
model first, which is the owner's call (open question 1). A refusal before the
first chunk comes back as the typed quota answer, which the streaming handler
already writes as an HTTP error before any SSE (`handleStreamingInference`,
the `!wrote` branch), so the caller walks to the next member without the
listener hearing it. The walk reuses `modelpolicy.Manager.RoleChain` and the
availability record so a known-dead member is skipped, not retried.

Latency budget, end of speech to the first spoken words of the real answer.
The target is about two seconds; TTFT and the proxy hop are the two unknowns,
so these are budgets, not measurements:

| Hop | Budget | Basis |
|---|---|---|
| final transcript on the phone | 0.2 to 0.4 s | on-device lane, not this note |
| request upload, warm TLS, 10 to 30 KB | 0.1 to 0.2 s | I |
| proxy and vmctl route to the guest | at most 0.1 s | not measured; must be emitted |
| route steps 1 to 3 | 0.1 s | I |
| model first token on a 2 to 3 K-token frame, cache warm | 0.5 to 0.9 s | not measured; assumption from whole-call times |
| first sentence, 12 to 20 tokens | 0.1 s | derived from 255 tok/s |
| `speech` event to the phone | 0.1 s | I |
| on-device TTS first audio | 0.2 to 0.3 s | on-device lane |
| total | **1.5 to 2.2 s** | |

What is achievable: a median near two seconds on a warm computer and a clear
connection is plausible; three seconds is the safe claim; the tail (p90) is
unknown until TTFT is measured. A cold computer, a cache miss on a long page
or a pool walk will exceed it, and the phone's local line covers that gap by
design.

## 6. Streaming to the phone

SSE over the existing proxy (the Texture document stream already crosses it).
The phone reads the body with `URLSession.bytes(for:)` line by line; each
`speech` event becomes one speech block handed to the engine as a revision with
the rest of the programme kept (`keepRest`), so `onRevision` and the playback
control already work unchanged. The new app stage replaces `ChoirDeskStage` for
the turn's answer; the old stage remains the slow lane for hand-off results.
A WebSocket (barge-in on the same connection) is a later option, not needed
for one-way speech.

Server side, `gatewayruntime.Provider` gets a text-streaming method beside
`CallWithTools` (the existing `stream` helper does the work; no gateway
change is needed, the gateway honors `stream: true` today). Sentences are cut
on terminal punctuation followed by a space, never inside an abbreviation or a
number, with a forced flush at 200 characters and at end of stream.

## 7. Slices

### Slice 1: the fast answer (about a day)

Land: the package registration (manifest with no `accepts`, so the web prompt
router is not changed), the prompt, the pool and role, `POST
/api/autoradio/turns` with the single streamed call, the request and Report
records, sentence streaming, per-hop timing in `done` and in the log, and a
rerunnable timing script. No hand-off, no tools, the page text comes from the
client. The app's stage swap is the app lane's change and ships with it.

- Mutation class: **red**. Protected surfaces: gateway and provider calls (a
  new streaming caller), tape and evidence (a new record kind), auth and
  session (a device-key route). Not touched: Texture canonical writes (the
  desk writes none), vmctl, deploy routing, checkpoint projection.
  Review: two seats; the author is not one.
- Ships on. No switch, no allowlist. The route is additive, so old app builds
  keep working through the prompt bar.
- Conjecture delta: a voice turn about the page can be answered by one
  streamed call with the first sentence audible in about two seconds, without
  the document path.
- Failure list first (tests written before code): no auth is 401; a foreign
  key is refused; empty words 400; oversize page 413; unknown field 400; the
  same `turn_id` replays the recorded answer with no second model call; the
  first `speech` event is flushed before the model finishes (a fake provider
  that delays); a quota refusal before the first chunk moves to the next
  member and the listener sees no error; all members refused yields one spoken
  fallback line and a settled "failed" record; client disconnect cancels the
  model call and settles the record "interrupted"; page text containing
  instructions stays data (the frame labels it world untrusted, the desk has
  no hand-off verb in slice 1); the request record is on the tape before the
  call and the Report after; a crash mid-turn closes the round as "interrupted
  by a restart" and never resumes it; the `autoradio` role resolves to the
  `autoradio` pool and a computer's policy overrides it; the manifest
  registers through the one path and a third-party package under the reserved
  slug is refused; the sentence splitter on abbreviations, numerals, forced
  flush and an unterminated final fragment.
- Deployed proof (staging, d@e only): after CI and the staging deploy,
  verify the deployed commit, then run the timing script ten times from a host
  off Node B against `choir.news`. It prints, per turn: request sent, first
  byte, first `speech` event, `done`, plus the server's per-hop timings and the
  model that answered, and it leaves the table as an artifact. Pass: median
  ask to first `speech` event at most 2.5 s over ten asks, no turn above 6 s,
  every turn has a request and a Report record, no ask creates a Texture
  document. Also read the gateway journal for `stream=true` on the
  `autoradio` role's calls.
- Claims to precommit in the calibration ledger before landing (the ledger is
  not edited by this note): proxy delivers SSE unbuffered, 85%; median ask to
  first sentence at most 2.5 s on d@e, 60%; haiku TTFT on the 3 K-token frame
  at most 1 s median, 65%; no quota refusal reaches a listener in ten turns,
  80%.
- Rollback: revert the commit(s); the route, pool, role and registration
  disappear together; records already written stay on the tape as history.
  The Texture path is untouched, so old builds are unaffected.
- Expedite: the landing commit carries `Ring-Expedite: voice turns take 10 s
  to 7 min through the Texture revise path`.

### Later slices

| Slice | Content | Class |
|---|---|---|
| 2 | speak first, then act: the single `go` cell in the same streamed response; `choir.Cast` to research and Ask to `texture:<doc>` with predictions; the `handoff` event; the spoken commitment | red (records, desk authority) |
| 3 | page by address: the server resolves publications by route, version and manifest hash, and private Texture pages through a per-task, by-ref, owner-bound read lease from the Texture custodian (the shape of `mail.read`); the client sends no text; the prompt prefix is the cache key | red (lease grant) |
| 4 | edition context: headlines, the programme's blocks, material the desk prepared when it built the edition; prefix-stable so the cache stays warm | orange |
| 5 | scoring: each turn's prediction settles on what the listener does next; the pool order is tuned from settled records, not from this note | yellow |
| 6 | the app retires the fork-per-ask path for questions; the fork stays for "make this a document" | orange |

## 8. Options rejected

- **Make the Texture desk faster** (lower reasoning, a smaller model, a
  shorter prompt). The role is the sole canonical writer of revisions; a
  question is not a revision. Even at best it keeps the document per ask
  (Turns A to D made four documents), the whole-document output and the
  research wait. A substrate fix is to stop sending questions there.
- **Stream the Texture revision's text.** The revision is the output of a
  cell, not text deltas, and a partial revision is not a canonical one.
- **Route the question through research.** Research took 6.6 minutes in
  Turns C and D.
- **Use Jev (System 1) to answer.** It returns choices and scores, no text.
- **Let the phone call a model directly.** Provider credentials stay
  server-owned, and the turn would leave no record on the computer.
- **A fast path inside the conductor or the prompt bar** as a special case.
  That is a special desk by another name (I26). The desk is registered like
  every other.
- **Free-first pool for this role.** 7.4% of devin calls were refused and the
  OpenCode keys park; a refusal here is silence.
- **Hedged concurrent calls to two providers** to cut the tail. It doubles
  spend per turn; the rule is to fix the caller, and concurrency is never
  plan A. Revisit only with a measured tail.
- **WebSocket now.** One-way speech does not need it; SSE already crosses the
  proxy.
- **Faster polling** (0.2 s) of the document stream. It saves under a second
  of a path that costs 10 to 455 s.

## 9. Open questions that are the owner's

1. **A paid model first.** The `autoradio` pool leads with Bedrock Haiku 5.5,
   which spends money per turn, ahead of the free members the default pool
   prefers. Owner act: approve it, with a spend figure to watch. Default if
   no answer: ship haiku first for d@e only through the computer's policy
   file (the owner-edit path), and report the measured cost.
2. **Retention of spoken turns.** Doctrine wants every turn on the ledger;
   these are the listener's words and the page they were on, owner-private.
   Confirm they are kept as owner-private records for scoring, and for how
   long.
3. **Where a hand-off lands.** Into the listener's edition document (my
   default, matching the turns note) or into a new document per hand-off (as
   asks do today).
4. **The reconciliation in 3.2.** The 2026-11-10 statement was that speaking
   is prompting the document. Ruling 23 sends voice to a desk. I read the desk
   as answering and the document as receiving lasting changes; confirm.

Not the owner's, decided here: SSE over WebSocket; the sentence splitter;
the role and pool names; the record fields.

## 10. What I could not determine

- The owner's phone asks of the morning. I did not find the owner's computer
  id (the journal names VMs, and the Node B proxy keeps no per-request log), I
  did not look further, and I did not touch that computer. No timeline of an
  owner turn is in this note.
- The 02:00 to 05:40 UTC simulator asks on d@e: none are in the ledger or the
  journal as app-shaped asks (section 1.3). The four turns above are the
  complete app traffic I could find on d@e for 2026-11-10 23:00 to
  2026-11-11 07:35.
- Time to first token of any model: every call is `stream=false` and the
  journal is to the second.
- The cost of the proxy and vmctl hops, and whether the proxy flushes SSE
  without delay on this path.
- The phone's own timings (when the ask left, when the revision was seen, when
  speech began). "Answer to the client sees it" is inferred from the 1.5 s
  poll in the code.
- Per-call attribution inside Turns C and D, which overlapped.
- Whether a policy file section `[roles.app:autoradio]` resolves for a package
  desk (`modelpolicy.NormalizeRole` consults the desk registry's profile flag);
  slice 1 must settle it with a test, and the role may need to be named
  `autoradio` instead.
- Whether the guest can resolve a publication by address itself (slice 3); the
  phone does today through the public platform route.
