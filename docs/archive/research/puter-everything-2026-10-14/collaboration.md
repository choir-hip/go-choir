# Collaboration group: teamchat, email, calendar, meetings, tasks, projects, wiki, contacts

Reviewer scope: `src/{teamchat,email,calendar,meetings,tasks,projects,wiki,contacts}` in
the shallow clone (read only, nothing executed). Choir comparison against
`frontend/src/lib/MailApp.svelte`, `CalendarApp.svelte`, `TextureEditor.svelte`,
`internal/maild`, and `internal/objectgraph` (`object.go`, `registry.go`).

## Group-wide observations (read first)

- **Stack is uniform**: vanilla JavaScript (no framework), Vite, Vitest, and
  Puter.js as the only backend (`puter.kv`, `puter.fs`, `puter.workers`,
  `puter.peer`, `puter.ai`, `puter.ui` AppConnection). Runtime dependencies are
  almost nil: `lucide` (ISC) everywhere, `yjs` + `lib0` (MIT) in wiki,
  `@mediapipe/tasks-vision` (Apache-2.0), `@sapphi-red/web-noise-suppressor`
  (MIT; GTCRN model) and `@fontsource-variable/inter` (OFL-1.1) in meetings.
  **No non-MIT/GPL/AGPL-incompatible dependency found.** Calendar's
  `src/js/windows-zones.js` is a copy of CLDR `windowsZones.xml` (Unicode
  license, permissive, needs attribution if copied).
- **Everything is coupled to Puter.** Storage, identity, server-side code
  (per-owner "Puter Workers"), P2P media and email all go through the Puter
  SDK. None of the UI code ports to Choir; the portable assets are the
  **pure-function `lib/` modules** (parsers, recurrence, threading, data models)
  and the **design ideas**.
- **Generated, but not slop.** Every app shows the signature of long agentic
  iteration: README prose explaining every edge case at length, tests named
  `*-pass12.test.js` .. `*-pass27.test.js` (teamchat has 167 test files),
  "hardening" test files. But the code is careful (allowlist sanitizers,
  ReDoS-aware regexes, constant-time token compare, no `kv.list` scans,
  surrogate-safe line folding), and the pure modules are well tested. The
  quality is far above "vibe slop" for teamchat, email, calendar, wiki,
  contacts; tasks and projects are competent but ordinary.
- **Puter already discovered "app agents" in miniature.** Calendar and
  Contacts expose a versioned app-to-app RPC over `AppConnection` with a
  per-caller method allowlist (`calendar/src/js/api/server.js`,
  `contacts/src/api/{protocol,server,methods}.js`), and Contacts serves the
  same method table as an **MCP server** (`contacts/src/api/mcp.js`,
  `tools.js`). This is the closest thing in the group to the Choir vision and
  the most transferable idea.
- **Telemetry**: calendar, contacts (and others) load Plausible from
  `plausible.io` in `index.html` and call it from `src/analytics.js`. Strip
  if anything is ever lifted.
- **Security note**: calendar `src/js/mailbox.js:135` instantiates its own
  bundled mail handler with `new Function(source)`. Source is the app's own
  `?raw` import, not remote, so not exploitable as shipped, but it is the kind
  of thing not to copy.

Sizes (JS source excluding tests / test lines / CSS):

| app | src JS | tests | CSS | deps |
|---|---|---|---|---|
| teamchat | 48.9k | 68.0k | 3.3k | lucide |
| email | 15.0k | 16.4k | 2.8k | lucide |
| calendar | 29.0k | 31.9k | 4.1k | none |
| meetings | 13.6k | 10.8k | 3.1k | mediapipe, noise-suppressor, inter |
| tasks | 7.7k | 3.1k | 2.2k | lucide |
| projects | 17.8k | 5.6k | 3.6k | lucide |
| wiki | 12.9k | 2.5k | 1.6k | yjs, lucide |
| contacts | 7.7k | 4.9k | 1.8k | none |

### Object-graph conventions used in point 4 below

Choir's object graph (`internal/objectgraph/object.go`) gives each object a
canonical id `obj:<kind>:<owner>:<suffix>`, a content hash, `Body` +
`Metadata`, `Tombstone`, `SupersededBy`, and typed `Edge`s; the registry
(`registry.go`) already registers kinds this group can reuse:
`choir.channel_message`, `choir.inbox_delivery`, `choir.content_item`,
`choir.work_item`, `choir.texture_document` / `choir.texture_revision`,
`choir.web_capture`, `choir.media_item`, `choir.audio_recording`,
`choir.transcript`, `choir.publication`, `choir.consent_record`,
`choir.review_record`, `choir.artifact_blob`, and edges `cites`,
`references`, `contains`, `derived_from`, `has_media`, `has_transcript`,
`supersedes`, `owns`. Per the Autopaper design (`docs/autopaper-sw-design-2026-10-09.md`
§2, "existing types only"), new kinds are proposed below only where nothing
fits. Embedded Dolt per computer gives history, diff, and branches for free;
the **Autopaper object graph** (global Store B via corpusd) holds what is
published. Collaboration data is overwhelmingly **private by default**; the
default publication rule for this group is: nothing about other people
(messages, addresses, attendees) is published unless the owner publishes a
derived artifact (a Texture, a public booking page, a public wiki page).

---

## 1. teamchat (Slack clone)

**What / stack.** Workspaces, public/private channels, threads, DMs, guests,
reactions, mentions, pins, saved-for-later, scheduled send and reminders,
search, link unfurls, push, WebRTC "huddles", shared channels between two
workspaces, export/import/transfer, incoming webhooks and bot tokens.
Client `src/` (vanilla JS, `src/lib/store.js` 6.7k lines, `src/ui/*`),
server `worker/index.js` (10.2k lines in one file) + `worker/{search,connect,
events,signal,push,unfurl,email}.js`, shared pure modules `src/shared/*`.
Deps: lucide only.

**Quality.** The most heavily engineered app in the group, and the most
obviously agent-iterated (README is 1,292 lines; tests named by "pass").
Real substance: a per-owner backend model (every workspace owner deploys
their own Puter Worker, members call it with their session; identity never
trusted from bodies), a storage layout that never scans (`tc:conv:{c}:seq`
counter + 50-message block documents, `src/shared/blocks.js`), an inverted
search index sharded by month and fed by a log (`worker/search.js`), a
change-signal ring for realtime (`src/shared/ring.js`, `events.js`),
signed attestations for cross-workspace shared channels (`worker/connect.js`),
SHA-256-hashed integration tokens with constant-time compare and per-token
rate limits. The 10k-line single worker file is a maintainability smell;
the whole design is shaped by Puter KV limits (400 KB values, no listing),
which Choir does not have.

**Worth taking.** No code wholesale. Ideas: (a) the per-owner server is
the same shape as Choir's per-owner computer, which validates hosting a
team's chat on its owner's computer; (b) bot/webhook tokens with scopes per
channel (`tc:bot`, `worker/index.js` `cleanIntegration`) are a ready-made
model for "external agent posts into a channel"; (c) the export/import/
transfer protocol (`/api/ws/:ws/export`, `transfer/*`) for moving a shared
space between owners; (d) the safe on-demand syntax highlighter
(`src/lib/highlight.js`, escape-first, small rule lists) is liftable as is.

**App-agent design.**
- *State as objects.* `choir.channel_message` already exists (used today for
  desk-to-desk channels, `internal/store/graph_store.go`) and fits human
  messages too: body = text, metadata = {conversation, author, seq, thread
  root, mentions, attachments}. Edits are `supersedes` edges to a new
  version (history for free, matches teamchat's "edit text sits beside the
  original"); deletes are tombstones. Proposed new kinds:
  `choir.conversation` (channel/DM/thread, external key, versioned: name,
  topic, members, policies) and `choir.reaction` only if per-reaction
  provenance matters (otherwise reactions fold into message metadata).
  Activity inbox entries reuse `choir.inbox_delivery`; attachments are
  `choir.artifact_blob` / `choir.media_item` with `has_media` edges.
  Read cursors and drafts are per-member UI state, not objects.
- *Links.* A message `references` a Texture, a `choir.web_capture` (link
  unfurl = a capture, so the preview is the same object research already
  stores), a `choir.work_item` ("make this a task"), a calendar event.
  A thread `derived_from` into a Texture when summarized.
- *Versioning.* Dolt history gives edit history and moderation audit; a
  branch is not useful for chat.
- *Published vs private.* Private by default (it is other people's speech).
  Publishable only as a derived artifact: an owner-approved Texture summary,
  or a public channel explicitly marked public (`choir.public_route`), with
  `choir.consent_record` per participant.
- *Direct manipulation.* Post, edit, react, reply in thread, pin, save,
  schedule, search, jump to message.
- *Desk verbs.* `post(conv, text, refs)`, `summarize(conv|thread, since)`,
  `catch_me_up(since)`, `extract_commitments(thread)` -> `choir.work_item`s
  on the ledger, `draft_reply(msg)` (owner approves), `answer_from(conv,
  question)` with research, `schedule(msg, at)`, `watch(conv, predicate)`.
- *Calls.* Research (answer questions with cited captures), Texture
  (summaries, meeting notes, decisions into documents), management (turn
  "can you do X" into work items), engineering (a bug report thread opens a
  capsule mission).
- *Why agent.* Chat is the place where commitments are made informally; a
  desk that lifts them onto the commitment ledger is the Choir-specific win
  that Slack lacks. Also: the desks themselves already talk in channels, so
  one chat surface could show human and desk conversation together.

**Priority: medium.** High value only once Choir has multi-person computers
(guests); for a single owner, the desk-channel view already exists. Effort:
2-3 weeks for a single-computer chat over `choir.channel_message` with
threads, mentions, attachments; multi-member auth is the hard part.

---

## 2. email (Puter Mail)

**What / stack.** Mail client over Puter's mailbox (raw RFC 822 files at
`/<user>/.mail/objects/<date>/<uuidv7>--<subject>`), sending through a
`useremailtx` endpoint. Browser-side MIME parsing and composition. Folders
are directories (inbox, sent, archive, trash), drafts in localStorage +
IndexedDB, rich-text composer via `execCommand`. `src/ui/app.js` (5.2k
lines), `src/lib/*` pure modules. Deps: lucide; Playwright for browser tests
and `benchmarks/`.

**Quality.** Strong pure engines with real RFC care: `src/lib/mime.js` (960
lines: RFC 2047 encoded words, RFC 2231 continuations, quoted-printable,
charset decoding, multipart walking), `compose.js` (MIME builder, header
folding, `multipart/alternative`, reply-all recipient rules, References
trimmed to 20), `thread.js` (Gmail-style conversation grouping with
localized `Re:`/`Fwd:` prefixes, mailing-list tags, and an explicitly
ReDoS-safe regex), `sanitize.js` (allowlist HTML sanitizer for a sandboxed
iframe), `richtext.js` (HTML <-> text/plain with structure kept),
`contacts.js` (autocomplete ranked "written-to > written-by", from the header
index, no address book), `calendar.js` (ICS parse of invites, keeps sender
TZID), format=flowed (RFC 3676) decode. Tests round-trip composed messages
through the parser. UI is a 5k-line file.

**Compare with Choir Mail.** Choir's Mail (`MailApp.svelte`, 3.4k lines over
`internal/maild`: Resend send, webhook ingest, aliases, drafts, approval
email, a quarantine folder) is server-side and already more Choir-shaped
(approval-gated send is the agent pattern). What Choir lacks and Puter has:
**no conversation threading** in Choir's UI; Choir's sanitizer
(`MailApp.svelte:1163`) is a **regex blocklist** (strips form/iframe/object/
embed/base) relying on the iframe sandbox, versus Puter's allowlist; no
recipient autocomplete; no undo-send window; no ICS invite rendering; no
format=flowed. Gain: port `thread.js` grouping rules and `sanitize.js`'s
allowlist (to Go in `maild`, or to the frontend), the ranked autocomplete
idea, and the invite parsing.

**App-agent design.**
- *State as objects.* Proposed `choir.email_message` (content-hash
  identity; body = raw RFC 822 as `choir.artifact_blob`, metadata = parsed
  headers, folder, flags) or reuse `choir.content_item` with a
  `content_kind: email` metadata field if the "existing types first" rule is
  strict. Threads: `choir.email_thread` (external key = root Message-ID) with
  `contains` edges, or derived on read (Puter derives them; that keeps the
  graph smaller). Drafts are versioned objects (history = undo across
  sessions; branch = "try a softer version" of a reply). Attachments are
  `choir.artifact_blob` with `has_media`.
- *Links.* Message `references` contact (`choir.person`, see contacts),
  calendar event (an invite becomes a `choir.calendar_event` linked
  `derived_from` the message), `choir.work_item` (an ask in an email), a
  Texture (the email cites or attaches a document). Newsletters can be
  ingested as `choir.web_capture` so research can use them.
- *Published vs private.* Always private. Never published to the Autopaper
  object graph; a newsletter capture could be published only if it is
  itself public content (link to the canonical URL capture instead).
- *Direct manipulation.* Read, reply, forward, archive, flag, compose,
  attach from Files, approve agent-drafted sends.
- *Desk verbs.* `triage(folder)` (label, archive, surface), `draft_reply(msg,
  intent)` -> draft awaiting owner approval (Choir already has approval
  email), `send(draft)` gated, `extract(msg)` -> event/contact/work item,
  `summarize_thread`, `watch(sender|query)` -> wake a desk.
- *Calls.* Research (verify claims, look up senders), Texture (turn a thread
  into a document), calendar desk (invites), contacts desk.
- *Why agent.* Mail is the canonical "agent does the drudgery, human
  approves the send" surface, and Choir already has the approval rail.

**Priority: medium (as engine imports into existing Mail).** Threading and
the allowlist sanitizer are a few days each; a full app-agent mail desk is
2-4 weeks on top of maild.

---

## 3. calendar

**What / stack.** Month/week/day/4-day/year/list views, drag/resize,
recurrence with this/following/all edits, reminders, holidays computed from
rules, DST markers, ICS + JSON import/export, emailed invitations with
self-serve RSVP (per-guest links served by a per-owner Puter Worker,
`src/workers/rsvp-worker.js`), Calendly-style booking pages
(`src/js/booking.js`, `public/book.html`), meeting links, reading invites
from the mailbox (`mail-intake.js`), and an app-to-app API. No runtime deps.

**Quality.** Real engines, well tested (31.9k test lines):
`src/js/recurrence.js` (series master + `exdates` + `overrides` keyed by
`recurrenceId` = original start; jump-ahead expansion with loop caps; weekly
BYDAY, monthly by date / nth weekday incl. last, yearly, count/until,
WKST), `ics.js` (`toICS`, `parseICS`, `parseITIP` with SEQUENCE, METHOD,
RECURRENCE-ID, RANGE=THISANDFUTURE), `windows-zones.js` (Outlook TZIDs to
IANA), `holidays.js` (Easter computus and rule tables per country),
`booking.js` (`computeBusy`, slot generation with buffers/notice/horizon),
`api/server.js` + `api/methods.js` (grant-scoped RPC). Weakness: **the
model is local-time-only** (recurrence "works in the browser's local time
zone"; ICS emitted as floating time or bare TZID without VTIMEZONE), so a
multi-zone owner or agent scheduling across zones would hit edge cases.
`modal.js` is a 3k-line UI file.

**Compare with Choir Calendar.** Choir's `CalendarApp.svelte` (1.1k lines) is
a **read-only `.ics` file viewer**: a naive RRULE that ignores BYDAY,
BYMONTHDAY, EXDATE and RECURRENCE-ID (`parseRrule` / `nextOccurrence`,
lines 200-260), caps at 100 occurrences, no editing, no events store, no
invites. Gain: an actual calendar (create/edit), a correct recurrence engine
with exceptions, iTIP invite/RSVP handling, booking-slot computation, and the
grant-scoped API shape. This is the largest concrete upgrade in the group.

**App-agent design.**
- *State as objects.* Proposed `choir.calendar_event` (external key = ICS
  UID, versioned): metadata = {title, start, end, tz, all_day, location,
  recurrence rule, organizer, attendees+partstat, reminders, meeting_url},
  body = description. Overrides map cleanly to child objects with
  `derived_from` the master and metadata `recurrence_id` (Puter's
  `overrides` keyed by `recurrenceId` maps 1:1). `choir.calendar` (a
  collection, e.g. personal, subscribed feed) with `contains` edges.
  Booking page = `choir.booking_page` (versioned settings); a booking =
  an event `derived_from` the page. Subscribed external `.ics` feeds can be
  research-polled sources (`choir.source_schedule` exists) whose items land
  as events in a read-only calendar.
- *Links.* Event `references` people (`choir.person`), a Texture (agenda or
  notes), a meeting (`choir.meeting`), `choir.work_item`s due that day, the
  email it came from (`derived_from`). A deadline in a project task can
  project into the calendar without copying (an edge, not a duplicate
  event); this is where shared object state beats Puter's per-app stores
  (tasks, projects and calendar each kept their own `.ics` export).
- *Versioning.* Dolt history = "what did this meeting look like before it
  was moved" and undo; **branches = "propose a reschedule"**: the desk plans
  a week on a branch, the owner reviews the diff and merges.
- *Published vs private.* Private. Publishable: a booking page's free/busy
  slots (only busy intervals, not titles) and explicitly public events
  (an event a Texture announces) via `choir.publication`.
- *Direct manipulation.* Create, drag, resize, edit series/occurrence,
  RSVP, toggle calendars.
- *Desk verbs.* `find_time(people, duration, window)`, `schedule(event)`,
  `reschedule(event, constraint)`, `rsvp(event, status)`, `agenda(day)`,
  `prepare(event)` -> research brief + Texture agenda linked to the event,
  `protect_focus(rule)`, `import_ics(url|file)`.
- *Calls.* Research (prep briefs on attendees and topics), Texture (agenda
  and notes documents), mail desk (invites, RSVPs), contacts desk.
- *Why agent.* Scheduling negotiation and meeting prep are multi-step and
  cross-app; the plain app only stores times.

**Priority: high.** Choir's calendar is a viewer; Puter's `recurrence.js` +
`ics.js` are directly portable (MIT, pure, tested) and a scheduling desk is
an obvious early app agent. Effort: 1-2 weeks for an editable calendar on
object-graph events reusing the engines (port to TS, add real TZ support);
+1-2 weeks for invites/RSVP through maild.

---

## 4. meetings (Google Meet clone)

**What / stack.** Persistent room codes, full-mesh WebRTC over `puter.peer`
with a host star for control traffic and host migration by election
(`src/lib/protocol.js` v4, `src/net/session.js` 2.6k lines), a room directory
Worker (`worker/rooms.js`) holding leases, guests without accounts, screen
share, PiP, chat, reactions, hands, active speaker, background blur and
replacement (MediaPipe selfie segmentation -> WebGL2 compositor with joint
bilateral upsampling, `src/net/background-compositor.js`), GTCRN noise
suppression in an AudioWorklet (`src/net/noise-suppression.js`), and a
home screen listing today's meetings from Calendar.

**Quality.** Competent; real media pipeline work. Full mesh caps practical
size (no SFU). Entirely dependent on Puter's signaling/TURN. No recording,
transcription or captions at all (grep finds no MediaRecorder or
SpeechRecognition).

**Worth taking.** `background-compositor.js` + `background-effect.js`
(blur/replace) and the GTCRN wiring are reusable if Choir builds calls.
The host-migration protocol is a nice idea. Signaling/TURN must be rebuilt.

**App-agent design.**
- *State as objects.* `choir.meeting` (proposed; external key = room code,
  versioned: title, scheduled event link, participants, start/end).
  Recordings are `choir.audio_recording` / `choir.media_item`, transcripts
  are `choir.transcript` with `has_transcript` edges; all three kinds
  already exist for Autoradio. In-call chat lines are `choir.channel_message`
  in a conversation `contains`-linked to the meeting.
- *Links.* Meeting `references` its `choir.calendar_event`; notes are a
  Texture `derived_from` the transcript; action items are `choir.work_item`s
  `derived_from` transcript spans.
- *Published vs private.* Private; transcripts contain other people's
  speech. Publish only an owner-approved Texture of notes, with consent
  records.
- *Direct manipulation.* Join, mute, camera, share, pin.
- *Desk verbs.* `join_as_notetaker(meeting)`, `transcribe`, `summarize`,
  `extract_action_items`, `answer_live(question)` via research.
- *Why agent.* The agent value (notes, decisions, follow-ups) is exactly
  what Puter's app lacks; video plumbing is commodity.

**Priority: low** for building calls (infrastructure-heavy, Puter-coupled);
**medium** for a "meeting notes" agent that ingests recordings from other
tools into transcript/Texture objects. Effort: calls 4+ weeks with SFU/TURN;
notes agent 1 week given existing transcript kinds.

---

## 5. tasks (Things/Todoist clone)

**What / stack.** Today/Upcoming/Lists/Calendar/Logbook, recurrence, tags,
reminders, natural-language quick add (`src/lib/quickparse.js`: "pay rent
friday 9am every month !high #home"), shared lists by replicated snapshots
in each participant's drive with tombstone/newest-wins merge
(`src/lib/collab.js`), `.ics` feed. `src/lib/store.js` 1.9k lines. Small.

**Quality.** Fine, ordinary. The collab replica merge is clever given Puter's
per-user storage, irrelevant for Choir. Quick-add parsing is regex grammar
that an agent does better.

**Worth taking.** Only ideas: quick-add chips that show what was understood
and never drop unrecognized text; overdue-repeat skipping.

**App-agent design.**
- *State as objects.* `choir.work_item` already exists and is the
  commitment substrate; an owner task should be a work item (or a
  commitment record) with metadata {title, due, priority, tags, list,
  recurrence, assignee: owner|desk}. Lists are `choir.collection`-like
  containers (or a tag). This maps cleanly; the interesting part is that the
  **owner's to-dos and the desks' work items become one ledger**, and a
  task can be assigned to a desk.
- *Links.* `derived_from` an email/message/transcript span; `references` a
  Texture or a calendar event; completion evidence `cites` the artifact.
- *Versioning.* History of a task's due-date slips is free and useful
  ("this moved four times").
- *Published vs private.* Private.
- *Desk verbs.* `add(nl_text)`, `plan_day`, `delegate(task, desk)`,
  `complete(task, evidence)`, `review_overdue`.
- *Why agent.* Delegation to desks with evidence-backed completion; a plain
  to-do app cannot do the work.

**Priority: medium, merged with projects** (one work-item surface, not two
apps). Effort: 1-2 weeks as a view over work items.

---

## 6. projects (Trello/Asana-lite)

**What / stack.** Kanban, list, timeline (Gantt-lite), milestones,
assignees, comments, checklists, attachments, saved views, cross-project
"All tasks" and "Today", quick add, markdown briefs, restore points
(daily/weekly snapshots, `src/lib/snapshots.js`), undo/redo, trash,
**Trello JSON importer** (`src/lib/trello.js`) and **CSV importer** with
header detection (`src/lib/csvimport.js`), Markdown/CSV/print export, zip
backup. One JSON document per project in Puter FS; optimistic concurrency
via `puter.fs.stat`.

**Quality.** Competent, tested at the store level (5.6k test lines), mostly
CRUD and views. Restore points and undo are reinventions of what Dolt gives
for free.

**Worth taking.** `trello.js` and `csvimport.js` as importer references
(small, pure); the saved-views model (filter/sort/group as named object);
the project-brief idea (a markdown "what done means" on the project, which
is close to a Choir goal file).

**App-agent design.**
- *State as objects.* Proposed `choir.project` (versioned: name, brief,
  columns, labels, milestones) with `contains` edges to `choir.work_item`s
  (task metadata: column, start, due, priority, assignees, checklist,
  labels). Comments are `choir.channel_message` in a per-task conversation.
  Attachments: `choir.artifact_blob` or a reference to a file/Texture (Puter
  already prefers "linked from your Drive, one copy of the truth", which is
  exactly an edge). The brief can be a Texture document `references`-linked.
  Maps cleanly; Puter's per-project JSON document would be split into
  objects so that tasks can be referenced from mail, chat and calendar.
- *Versioning.* Replaces `snapshots.js`, trash, and undo; **branches =
  "plan B" for a project** (re-plan the timeline on a branch, diff, merge).
- *Published vs private.* Private; a project status page could be
  published as a Texture.
- *Desk verbs.* `plan(goal)` -> project + tasks, `status_report` -> Texture,
  `replan(constraint)` on a branch, `assign(task, desk|person)`,
  `import(trello_json|csv)`.
- *Calls.* Management (coherence, admits engineering work), engineering
  (tasks that are code changes become capsule missions), research.
- *Why agent.* The board becomes the owner's view of the desks' commitment
  ledger rather than a second to-do database.

**Priority: medium** (as the owner-facing view of work items, merged with
tasks). Effort: 2 weeks for board + list over work items; timeline +1 week.

---

## 7. wiki (Notion clone)

**What / stack.** Block editor (each text block is its own contenteditable
bound to a `Y.Text`, `src/editor/editor.js`), flat block list with
fractional order keys and indent levels (`src/data/page.js`,
`src/lib/order.js`), page tree, **databases** whose rows are pages
(`src/data/database.js`: typed props, table/board/list/gallery/calendar
views with filters and sorts), comments anchored to blocks, Markdown/HTML
import/export, live collaboration via Yjs through a per-owner Worker
(`worker/service.js`: edit log + checkpoints + presence + invite roles).
Pinned dependency versions and a license gate at build
(`tools/check-licenses.mjs`). Deps: yjs, lucide.

**Quality.** Good and compact (12.9k lines). Thin tests (2.5k lines) for the
editor's complexity. The data model is the best-designed part.

**Compare with Choir Texture.** Texture is a single-writer canonical
document system (only the Texture desk writes revisions; owner edits go
through it), Markdown blocks, citations and source flows, publication with
permalinks. Wiki offers what Texture lacks: **page trees, databases (typed
rows with views), block-anchored comments, live multi-user editing**. The
Yjs CRDT model conflicts with Texture's single-writer doctrine, so do not
import it; the **database model** (props/views/rows-as-pages) and
**block-anchored comments** are the transferable pieces.

**App-agent design.**
- *State as objects.* Pages are `choir.texture_document` +
  `choir.texture_revision` (existing, versioned; Texture remains sole
  writer). Tree = `contains` edges between documents. Proposed
  `choir.database` (versioned schema: props, views) whose rows are Texture
  documents (as in Puter: "every row is also a page") with row property
  values as metadata on a `choir.database_row` object or an edge to the
  document. Comments = `choir.review_record` (exists) or
  `choir.channel_message` anchored by block id. Images are
  `choir.artifact_blob`.
- *Links.* Database rows `cites` captures (a research database of sources),
  `references` people, events, work items. A database view is the generic
  "table over objects" that other app agents (tasks, contacts, CRM) can
  reuse.
- *Versioning.* Texture revisions already provide history; Dolt branches
  give "draft a restructure of this space".
- *Published vs private.* Pages publish via the existing Texture permalink
  path to the Autopaper object graph; databases publish as a Texture view
  or not at all; comments stay private.
- *Desk verbs.* Texture's own verbs plus `create_database(schema)`,
  `add_row`, `query(db, filter)`, `fill_column(db, prop, research_query)`
  (research fills a column with cited answers), `organize(tree)`.
- *Why agent.* A database whose columns research fills with evidence, and
  whose rows are documents Texture maintains, is something Notion does not
  do.

**Priority: medium.** Databases-as-objects are a high-leverage shared
primitive; the editor itself is not needed (Texture exists). Effort: 2-3
weeks for a database object + table/board views over documents.

---

## 8. contacts

**What / stack.** Address book (multi email/phone/url/address, company,
birthday, notes, tags, favorites), vCard 3.0 import/export
(`src/vcard.js`: line unfolding, quoted-printable, surrogate-safe folding,
TYPE params), "smart add" (paste a signature, `puter.ai.chat` extracts JSON,
`src/ui/smartadd.js`), birthday-to-calendar, **app-to-app API with
per-caller grants and an MCP server** on the same connection
(`src/api/mcp.js`, `tools.js`: `search_contacts`, `list_contacts`,
`suggest_recipients`, `get_contact`, `create_contact`, `update_contact`,
`delete_contact`; `contact://<id>` resources as vCard; tool errors as
`isError` results vs protocol errors as JSON-RPC errors; compact text
rendering alongside `structuredContent`). No deps. Pure `model.js`.

**Quality.** Small, clean, well tested. The MCP design notes are good
practice and directly relevant to external agents driving Choir apps.

**Worth taking.** `vcard.js` (pure, MIT) and `model.js` normalization; the
MCP pattern (same method table served as internal RPC and MCP, grant
decides both; model-fixable errors as results; compact text + structured).

**App-agent design.**
- *State as objects.* Proposed `choir.person` (external key, versioned:
  names, emails, phones, org, tags, notes, birthday). Existing
  `choir.provenance_agent` and `choir.source_entity` are for publication
  provenance and cited entities; a `choir.person` should be able to link
  `references` to a source entity when the contact is also a public figure.
  Interactions are not stored on the person: they are edges from messages,
  emails, events and meetings, so "everything with Ada" is a graph query
  (Puter does this with cross-app RPC calls; the shared graph makes it free).
- *Versioning.* Contact history and merge-duplicates as a reviewed diff.
- *Published vs private.* Strictly private (PII; `internal/pii` exists).
  Never published. Only the owner's own card could be published.
- *Desk verbs.* `find(query)`, `suggest_recipients(prefix)`,
  `upsert_from(text|email|vcard)`, `dedupe`, `brief(person)` (research +
  history), `timeline(person)`.
- *Why agent.* Extraction from mail/signatures and a person-centric graph
  view across all apps.

**Priority: medium-high as an object kind** (every other app here needs
`choir.person`), low as a standalone window. Effort: 3-5 days for the kind,
vCard import, and suggest verb.

---

## Group summary

**Top picks**
1. **Calendar engines** (`calendar/src/js/recurrence.js`, `ics.js`,
   `windows-zones.js`, `booking.js`): biggest concrete upgrade over Choir's
   read-only ICS viewer; pure, MIT, heavily tested. Needs real time-zone
   support added. High priority.
2. **The app-to-app API + MCP pattern** (`contacts/src/api/*`,
   `calendar/src/js/api/*`): one method table, per-caller grants, served
   both as internal RPC and as MCP, with model-fixable errors returned as
   results. This is the template for "an external agent can drive any Choir
   app window".
3. **Email engines into Choir Mail** (`email/src/lib/thread.js`,
   `sanitize.js`, `mime.js`, `contacts.js` ranking): threading and an
   allowlist sanitizer replace Choir's regex blocklist.
4. **Wiki database model** (`wiki/src/data/database.js`): typed props and
   views over rows-that-are-documents, as a shared object primitive that
   Texture, tasks, contacts and research tables could all reuse.
5. **Tasks + projects as one view over `choir.work_item`**: take the
   Trello/CSV importers and saved-views idea; drop their storage, snapshots
   and undo (Dolt provides those).

**Cross-app reusable pieces / shared kinds to add.** `choir.person`,
`choir.calendar_event`, `choir.conversation` (over existing
`choir.channel_message`), `choir.project`, `choir.database`, maybe
`choir.email_message`. With those, the links Puter fakes through background
app launches and duplicated `.ics` exports (tasks, projects and calendar
each export their own feed) become graph edges: message -> work item ->
event -> person -> Texture.

**Alarming.** No license problems (all MIT/ISC/Apache-2.0/OFL; CLDR data
under Unicode license). Plausible analytics scripts in several
`index.html` files. `new Function` in `calendar/src/js/mailbox.js:135`
(own code, not remote). Everything is Puter-coupled, so UI and backends are
not portable. Code is clearly agent-generated through many "passes" but is
careful and tested; the risk is volume (teamchat 49k source lines, one 10k
line worker file), not correctness.
