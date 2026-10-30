# HeyPuter/everything, Reviewed for Choir App Agents

Status 2026-10-30: the Dolt-versioned object graph language below is stale: the object graph is PostgreSQL (shared graph database) and SQLite (guest). Design source for the new data system: [the data system frame](../../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-14. Owner request: review https://github.com/HeyPuter/everything
(MIT, 50 apps, "the world's software, rebuilt") with many sub-agents, not
to act yet, but for the app rebuilds and the long-term vision of **app
agents**. Eight reviewers read the code (never ran it) by group; their
reports are in this directory, with the shared brief in `00-brief.md`.

## The owner's vision, as briefed

- An app (spreadsheet, CAD, video editor) is a **window in the Choir
  desktop with its own desk**: one agent or a nested tree of sub-agents
  that works in the background and talks to the other desks (research,
  Texture, engineering) to fulfil requests and to ask for work.
- The user can **directly manipulate** it (edit a cell, trim a clip), it
  can **operate itself** from natural language, and an **external agent**
  can drive it.
- Its state lives in Choir's **Dolt-versioned object graph**, shared with
  other apps inside the computer, and shared across computers through the
  **Autopaper object graph** on the host.
- Later: any window can go full screen and look like an ordinary web app,
  with a Choir account as its login.
- Not a one-to-one copy: much of the repo was expected to be "vibe slop".

## The verdict on the repo

It is machine-generated at scale, but mostly **not slop**. Most apps have
a pure domain layer with hundreds to over a thousand tests; the weak parts
are huge single UI files (12-15k lines) and no shared code between apps
(five spreadsheet-import parsers, four calendar-file writers). Every app
depends on Puter's unpinned `puter.js` for login, storage, AI and peer
connections, so **no app ports as is**. The value is in engines, data
models, file-format code, test corpora and a few agent patterns.

## What to take, ranked

| rank | what | from | why |
|---|---|---|---|
| 1 | Spreadsheet formula engine (~550 functions, LET/LAMBDA, dynamic arrays, no browser dependency) **and its Excel-recorded test fixtures** | office: `spreadsheet/src/core/formula`, `tests/fixtures/reference-*.json` | the owner's lead app-agent example; the fixtures may be the repo's most durable asset; needs a dependency graph added |
| 2 | Video edit model and renderer (integer-microsecond JSON project, ~80 pure timeline functions, one `evaluate(project, t)` for preview and export, WebCodecs export) | media: `video-studio` | the owner's agentic video-editing example; needs a serializable op log |
| 3 | Parametric CAD (OpenCascade via replicad, JSON feature history with formula dimensions and stable topology names, runs headless) | engineering: `parametric` | an agent-native CAD model; LGPL kernel kept as a separate module; exclude the SolidWorks importer |
| 4 | Double-entry posting engine (integer cents, 17 document types that always balance, frozen past periods, real reports) | finance: `accounting/src/lib` | a ledger on Dolt is the clearest fit for versioned state; invoicing, expense, POS and inventory become windows over one ledger |
| 5 | Calendar recurrence and `.ics` engines | collaboration: `calendar` | Choir's Calendar can only display `.ics`; it cannot edit events or handle real recurrence |
| 6 | Audio DSP crate (Rust, ~21k lines: loudness, resampling, FLAC, noise and click removal) | media: `audio-studio` | one shared media engine for video, audio and Autoradio |
| 7 | Email threading, MIME and an allow-list HTML sanitizer | collaboration: `email` | Choir Mail lacks threading and strips only a short block list |
| 8 | PPTX read/edit/write and SVG slide rendering | office: `presentation/lib/pptx` | slides as a Texture output |
| 9 | Format libraries: DXF (cad), Gerber/Excellon and KiCad (circuits), glTF/FBX/USD/OBJ/STL/PLY (3d), OFX bank import, a small PDF writer and XLSX reader, PDF stamping (signatures) | several | libraries, not apps |
| 10 | Rules language for conditions, actions and response targets | business: `help-desk/shared/rules.js` | triggers and views in any app desk |

Documents and notes fold into Texture (a docx bridge, not a new app). Most
business and vertical apps are generic CRUD: their value is schemas.

## The app-agent pattern the reviews converge on

1. **One verb registry per app**, served to the app's own agent, to other
   desks and to external agents (as MCP), with a per-caller allow-list and
   errors returned as ordinary results the model can act on
   (collaboration: `contacts/src/api`, `calendar/src/js/api`).
2. **Self-operation through a checked plan.** Photo studio's assistant
   searches a catalog of about 870 commands, validates its plan against the
   catalog, runs a request as one undo step, opens a prefilled dialog for
   anything that spends money, and blocks destructive commands. Video,
   spreadsheet and presentation assistants have 22-23 validated tools each.
   These are verb sets for the desks.
3. **Direct manipulation and the agent emit the same ops** (an op log, not
   opaque closures), applied as conditional batch writes to versioned
   objects: `apply_ops(doc, base_rev, ops[])`.
4. **State as objects.** One generic head-plus-revision kind for app
   documents (modelled on `texture_document` / `texture_revision`), with a
   revision per accepted edit, not per keystroke. Heavy data (images, audio
   blocks, strokes) in content-hash blobs, not Dolt rows.
5. **Shared kinds across apps** instead of per-app stores: `choir.party`
   (the same person is stored eight ways across the business apps today),
   `choir.ledger_txn`, `choir.calendar_event`, `choir.catalog_item`,
   `choir.project`, `choir.database` (typed tables over objects), one
   `choir.game`; existing kinds for captures, messages, transcripts,
   recordings, attestations and provenance.
6. **Variants are revision lineages, not Dolt branches.** A Dolt branch
   forks the whole computer's database and is already used for computer
   candidates, and the object graph has no branch support today. Both the
   engineering and media reviewers reached this independently.
7. **Published vs private:** live documents and personal data stay private;
   frozen exports, templates, help articles, clause libraries, specs and
   records both parties agree to publish go to the Autopaper object graph,
   with `cites` edges to their sources.

## Problems to design for

- **Erasure versus versioned history.** Dolt keeps history forever, which
  conflicts with deleting personal data (hiring retention, help-desk
  deletion). Per-person encryption keys or a history-rewrite path are
  needed before personal data enters the versioned graph.
- **Scheduling needs an always-on computer.** Reminders, inbox polling and
  nudges in these apps run only while a tab is open; Choir's persistent
  computer fixes that.
- **Money as floats** in five finance apps, and device sync that can
  corrupt data; Dolt's conditional writes replace their sync journals.
- **Untrusted file parsers in the page** (PDF, PostScript, TIFF, PSD in
  vector-studio): sandbox them. **Arbitrary user scripts** with full app
  access (audio-studio, 3d): external agents drive apps only through the
  verb registry. **Hand-rolled signature verification** (pdf, signatures):
  don't trust the verdicts without an audit. **Arbitrary HTTP** (api-client):
  red class, through the gateway.

## Licences and provenance

No AGPL anywhere. To keep separate or exclude:
- LGPL: OpenCascade wasm (parametric), lamejs (MP3 export in
  audio-studio); keep as separate, replaceable modules.
- MPL-2.0: Mediabunny (fine unmodified); some build and test tools.
- CC BY-SA 3.0: chess piece images. GPL-2.0 with font exception:
  Liberation Sans in the PDF build.
- **Exclude** parametric's SolidWorks importer: it bundles a Parasolid
  schema of Siemens origin relabelled MIT.
- **Rebuild from source** any committed prebuilt wasm (circuits, canvas
  apps) before use; it cannot be checked against its source.
- Apache-2.0 dependencies (SheetJS, pdf.js, tesseract.js) need NOTICE files.

## Also noted

- The whiteboard is 574 MB because 561 MB is committed coding-agent
  scratch output; its source is 4.3 MB.
- `assets` has no source code; `building-design` is two icons.
- `meetings` is a Google Meet clone (full-mesh WebRTC, no SFU, no
  recording or transcription), dependent on Puter's signaling and TURN.

## Horizon notes from the same conversation (not acted on)

**Streaming, recording and calls.** YouTube, X and Twitch ingest RTMP;
a browser can't send RTMP, so the browser sends WebRTC (the WHIP standard)
to a Choir media service, which forwards RTMP to each platform at once
(multistreaming). With H.264 from the browser the service only re-packages
video and converts Opus to AAC, about one core or less per stream.
Recording is the same service writing the stream to a file that becomes a
capture, which a video app agent can cut into clips. Phone browsers
(Safari included) can stream camera and mic; iOS cannot share the screen,
and streaming stops when the screen locks. Calls: WebRTC media is peer to
peer, but needs signaling (Choir's API), STUN and a TURN relay (for the
10-20% of connections that cannot go direct); beyond about four people an
SFU forwards each stream once. Candidates: Pion (Go, MIT), LiveKit
(Apache-2.0), MediaMTX (Go, MIT). The media service runs on the host, shared
like the gateway; stream keys are platform-held secrets. A meeting agent's
value is notes, decisions and action items, which Puter's app lacks.

**Apps from a cache, not the image.** App packages are content-addressed
and fetched on demand from a host cache (the same pattern as the capsule
Go module proxy, distributed by station LU); frontend code loads only
when its window opens; an app's agent-side processes run only while its
desk is active and go dormant when idle. An unused app costs host disk
and nothing in the guest.

## Suggested first rebuilds (when the owner chooses)

1. A **spreadsheet desk** on the formula engine plus a dependency graph,
   cells as versioned objects that can cite captures and research.
2. A **video desk** on the video-studio model with an op log, clips as
   captures, renders published with provenance (pairs with streaming and
   recording).
3. An **editable calendar** on the recurrence and `.ics` engines.
4. A **ledger** (accounting engine ported to Go on Dolt) under invoices and
   expenses.

Each needs the shared pieces first: the verb registry, the op log, the
generic app-document object kind and `choir.party`.
