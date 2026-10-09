# Autopaper design (station SW) — draft v3

Date: 2026-10-09. Status: **rough draft, co-designed with the owner.**
Station: `SW-world-wire-interface` in the
[app-dev metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md).
Gate 3 (Autopaper) ships as an MVP in October, after Gate 1 (today) and
Gate 2 (tomorrow).

Inputs:
- *Choir — Canonical Project Context* (2026-10-08);
- [Prospective Self-Alignment](prospective-self-alignment-2026-10-09.md);
- [Store B inventory](evidence/world-wire-store-b-inventory-2026-10-09.md);
- [precommitment records memo](<Precommitment Records — Engineering Memo.md>).

v1 and v2 of this draft are superseded (git history). v2 specified a staged
pipeline, per-task model routing and fixed workflows; the owner rejected
that as too much structure.

## Owner brief (2026-10-09, verbatim excerpts)

> pull content (rss, telegram/mtproto, atproto, gdelt, other osint,
> eventually private data feeds) as much and as fast as we can process it
> [...] stick to our four core desks, think about how we incorporate
> precommitment records, and how we use inference efficiently since we are
> budget limited [...] we ingest and store the content in the OG, then we
> transclude it into textures.. and transclude textures into textures.
> [...] we dont adjudicate just show what the world indicates

> publication will be fully autonomous. what is publication really? it just
> means the article is accessible via permalink, a texture in the autoputer
> that appears to logged out users. we will have more policy over email
> distribution. and manual publication is still manual [...] the whole point
> of autopaper is to automate publishing.

> we will often be updating existing textures, not just writing new ones.
> [...] we are happy when the facts change. for them a correction is
> embarrassing.

> the bigger the story, the more the inference, models, data, perspectives,
> etc, that we apply. so its not about routing really. the same story will
> often have multiple textures from different models, and textures can
> change models...and they can transclude, and communicate through the
> precommitment records protocol. almost total chaos, you get me? the less
> structure the better! bitter lesson.

> we dont need to increase our data feeds or focus on any beats yet. we
> just need to get gates 1 2 and 3 mvp. viable meaning no shortcuts. not
> taking on tech debt that causes more problems.

> only the texture agent can edit a texture. so when there is new evidence
> sent to a texture, that desk gets woken up and makes a revision. it may
> call a researcher, management, and/or engineering for help before
> revising and yielding

> on transclusion, we should transclude specific versions, but have a
> subtle indicator that new revisions is available for the transcluded
> texture. that way, the system is stable and intelligible. [...] i think
> we should start from zero content actually. easier to say no backwards
> compatibility. [...] budget, we shouldnt worry about that until we are
> operating live. we can put that in after we track our resource usage.

## 0. Why the Universal Wire app failed, and what not to repeat

Owner: "do we know why world wire app couldnt connect to corpus store? if
we dont know we might replicate that rotten bug we couldnt squash in june
and july."

**What we know (June–July).** The
July attempt report (deleted from the tree; read it with
`git show d10ec47b:docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`) found no single bug. It found six wrong cornerstones that
made the paper "0% retentive": stories did appear several times, but only
in windows that the next deploy closed. Their status today:

| July cause | What it was | Status 2026-10-09 |
|---|---|---|
| C1 | The guest's embedded Dolt runs on **one connection**; health, Texture, graph reads and background work all queue on it | **Still open**: `configureEmbeddedDoltDB` sets one connection (`internal/store/texture.go:399`) |
| C2 | Legacy migration ran on every boot, under a readiness deadline | Fixed: the boot backfill is gone |
| C3 | Slow treated as dead; reads could boot or recover a VM | Partly addressed (recovery admission, faster boot); not re-verified for this workload |
| C4 | Processor lifecycle split across five authorities; "at most once ever" dedup | Moot: the processor and reconciler are deleted |
| C5 | Agent completion was narrative, not artifact-verified | Partly addressed (run acceptance work); must hold for Texture revisions |
| C6 | Every read of the app went proxy → vmctl → live guest → that one connection | Fixed: reads go proxy → corpusd → the host store, never the guest |

**What we do not know (September–October).** After the store split, the
app read from corpusd and the corpus store. There is no trace evidence of
a connection failure in that period: the proxy journal on Node B starts on
October 4 and logs no stories errors after that. What the evidence does
show:
- The processing pipeline was dead (127 of about 211,000 processor
  requests completed, per the October 5 problem doc). No new stories were
  published after June 30, so at best the app showed stale June stories.
- The store was under constant ingest load: more than one core, 10 GB of
  memory, and the October 1 out-of-memory cluster on the same host.
- The final data was internally consistent for the stories endpoint: all
  209 active routes resolve to a version (offline check on the old repo,
  2026-10-09).
- There were **two route authorities**: 209 route objects in the object
  graph against 926 rows in the `public_routes` table.
- The endpoint fails as a whole if any single route is broken, and it
  makes several store queries per route.

The exact September–October failure is therefore **unknown**. The old
store is offline (reset at 12:33Z), so it cannot be reproduced live.

**Constraints this design takes on, so the known causes cannot recur:**
1. **High-volume ingest never shares the guest's single embedded
   connection with Texture and health (C1).** How (real concurrency in the
   store, or a separate handle and store for captures) is W1's first
   engineering decision, made in Gate 1 terms, not worked around.
2. One authority per fact: one route record, one revision chain, one
   publication state.
3. Reads of published Textures never touch the paper's computer (C6 stays
   fixed).
4. A list read degrades per item (one bad entry is skipped and logged), and
   costs a bounded number of queries.
5. A Texture revision counts only when the revision exists (C5).

## 1. Principles

1. **Show what the world indicates; do not adjudicate.** Textures present
   what is reported, by whom, and where reports differ. Verifying means
   checking provenance and attribution, not ruling on truth.
2. **As little structure as possible.** The design fixes only what the
   substrate must guarantee: where content lands, who may edit a Texture,
   how desks wake and talk, what publishing means, and what is stored.
   Everything else is left to the agents: stages, story boundaries, claim
   schemas, which model writes what, and how many Textures cover a story.
   General methods that scale with compute beat hand-built workflows (the
   bitter lesson).
3. **Spend follows story size.** The bigger the story, the more inference,
   models, data and perspectives go into it. Management decides that
   allocation. There is no per-task routing table.
4. **Many Textures, many models.** A story can have several Textures
   written by different models, and a Texture can change models over its
   life. Textures transclude each other and talk to each other through
   precommitment records. The paper is a population of living documents,
   not a pipeline.
5. **Living Textures.** The paper mostly revises existing Textures. New
   facts are welcome, and knowledge compounds in place.
6. **Only the Texture desk edits a Texture.** New evidence delivered to a
   Texture wakes the Texture desk, which revises and yields. Before
   revising it may call research, management or engineering. Nothing
   changes a Texture's content except a Texture revision.
7. **Publishing is automatic.** It means a Texture has a permalink and
   logged-out readers can see it.
8. **Viable means no shortcuts.** The MVP is small, but each piece is the
   real mechanism, not a stopgap that must be torn out later.

## 2. What the substrate guarantees

### Content lands in the object graph

- Research polls sources through its in-cell module. The polling code
  already exists in `internal/sources` (RSS, Telegram web preview, GDELT,
  Polymarket), and it is enough for the MVP.
- Each fetched item becomes a `choir.web_capture` object in the paper's
  object graph. That kind already exists: URL, canonical URL, title, fetch
  time, and the content and extracted text as content blobs. The fields
  the feeds add (published time, language, source) extend its metadata
  schema rather than creating a new kind.
- Agents may write any further objects they find useful (stories, claims,
  entities, discrepancies, translations). New kinds are added only when
  an existing one does not fit.

### Ontology: existing types only (owner: "so we dont create new types... unless we need to")

| Thing | Existing type | Where |
|---|---|---|
| A feed or outlet to poll | `sources.Source` (configuration read by research's polling code) | `internal/sources/types.go` |
| A fetched item | `choir.web_capture`, with bodies as content blobs | `internal/objectgraph/web_capture.go` |
| A cited source inside a Texture | `choir.source_entity` and `choir.source_ref` | `internal/store/texture_source_graph.go` |
| An article | a Texture document and its revisions | the Texture store |
| A commitment, report or resolution between desks | `choir.commitment_record` (precommitment records) | `internal/types/commitment.go` |
| Polling, fetching, parsing | functions in research's in-cell module, not a type | — |

"Adapter" and "report" in earlier drafts of this design were informal
words, not types. They are dropped.

### Storage that cannot repeat the 104 G problem

- **The paper's computer** (embedded Dolt plus content storage) holds
  working state. Capture bodies live in content storage, not Dolt rows.
  High-volume tables stay out of Dolt history or are committed on a
  bounded cadence (to be settled in SH; Dolt's ignored-table mechanism is
  the first candidate). Textures and records are versioned.
- **The host-level store** (the corpus Dolt behind corpusd) holds published
  Textures and their revisions, and serves every logged-out read. Its
  history has a declared bound.
- Retention: capture bodies have a bounded window (OPEN: 30 days), then
  shrink to a stub. Bodies cited by a Texture or a record are pinned.

### Textures: one writer, woken by evidence

- Only the Texture desk writes a Texture revision.
- Evidence reaches a Texture as a delivery (a capture, another Texture's
  record, a research result). The delivery wakes the Texture desk for that
  Texture. It may call research, management or engineering, waits for
  what it needs, then revises and yields.
- **Transclusion pins a specific revision.** A Texture transcludes a graph
  object or another Texture at an exact version, so what a reader sees is
  stable and intelligible. When the transcluded item has a newer revision,
  the transclusion shows a subtle indicator ("newer revision available").
  Moving the pin forward is itself a Texture revision, made by the Texture
  desk.
- A Texture's model is a property of its activation and can change from
  one revision to the next. Several Textures can cover one story.

### Desks talk through precommitment records

- When a desk asks for something, or expects something, it commits a
  record: what it expects, how sure it is, and what would resolve it.
  Examples:
  - management, before spending on a story: how big it will get;
  - a Texture, before revising: what the next evidence will show;
  - research, before a search: what it will find.
- Records addressed to a Texture or a desk are deliveries that wake it.
  That is the whole communication protocol between Textures and desks.
- Resolutions arrive as the world moves: later reports, later revisions,
  search results. Where a count or a revision answers the question,
  resolution needs no model call.
- What accumulates: calibration per desk, model and Texture, and track
  records for sources (how early they reported, how often other sources
  later contradicted them). These are shown as data, never as verdicts,
  and they feed back into where management spends.

### Publishing

- Publishing is a visibility state: a permalink, readable when logged out.
  Every later revision appears at the same permalink, and the history stays
  readable.
- Autopaper's Textures publish automatically, with no human step.
- Users can still publish any Texture they own by hand.
- Email distribution is a separate channel with its own policy, owned by
  management. That is where sending is gated.

## 3. The four desks

- **Management:** attention and spend. It decides how much inference, how
  many models and how many Textures each story gets, scaling with story
  size. It also owns the source schedule and email policy, and it scores
  records (Jev by default).
- **Research:** polls sources, searches the web sparingly
  (rate limits and cost), checks provenance, writes graph objects and
  resolves records. Its authority is read-only toward the world.
- **Engineering:** fixes polling code and other module functions in
  capsules, when cast.
- **Texture:** the only writer of Textures. It is woken by evidence,
  revises and yields, and publishes automatically.

## 4. Inference and resources

Spend scales with story size, and nothing else is fixed. The levers are
management's: how many models, how many Textures, how much research and
search a story gets. The cheap defaults that need no workflow:

- Polling and resolution by counting cost no model calls.
- Texture revisions happen only when evidence arrives.
- A small story gets one Texture and few calls. A big one gets many
  Textures, many models and more research.

No budget is set before the paper runs live (owner). The MVP **tracks**
resource usage per story, Texture, revision and desk: tokens, model, time,
storage. The budget mechanism is added once that data exists.

## 5. Constraints on Gates 1–2

| Gate / station | Constraint |
|---|---|
| SL (Gate 1) | a delivery to a Texture durably wakes the Texture desk; durable timer obligations for the source schedule, paused after a restart (owner rule) |
| SH (Gate 1) | capture bodies in content storage; bounded history for high-volume tables; retention classes; records immutable |
| Store (Gate 1) | high-volume capture writes do not share the guest's single embedded Dolt connection with Texture and health (July C1) |
| Texture contract (Gate 1) | sole writer; woken by deliveries; may call the other desks before revising; model per activation; several Textures per story; transclusion pinned to a revision, with a newer-revision indicator |
| S5/S6 (Gate 2) | publishing is a visibility state agents can set; published revisions reach the host store; logged-out reads never touch the paper's computer |
| Platform | the host store holds published Textures and serves reads, with a declared history bound; email is a separate, policy-gated channel |

## 6. MVP build order (Gate 3, October)

No new feeds and no beats. Each step lands through the Landing Loop with
deployed acceptance.

- **W1 Content in the graph:**
  - decide and build how captures avoid the single embedded connection
    (§0 constraint 1);
  - the primary Autopaper computer, starting from zero content;
  - research's polling over the existing source code, writing
    `choir.web_capture` objects with bodies in content storage and bounded
    history;
  - the source schedule as timer obligations;
  - resource usage tracking from the first poll.
- **W2 Living Textures:**
  - management notices stories in the incoming reports and opens or
    feeds Textures;
  - the Texture desk revises on delivered evidence, calling the other
    desks as needed;
  - records flow between desks and Textures and resolve.
- **W3 Readers:**
  - automatic publishing;
  - the Autopaper app and public permalinks reading from the host store.
  Email distribution comes after the MVP.

Later, not in the MVP: polling for AT Protocol and MTProto, other feeds,
private feeds, beats (Taiwan first, then Asian-language social media).

## 7. The old Store B data: start from zero

Owner: "i think we should start from zero content actually. easier to say
no backwards compatibility."

- Nothing is migrated. The corpus store behind corpusd stays as the
  host-level store and starts empty (reset at 12:33Z).
- The June publications (633, of which 148 are users' work) and their
  URLs are not restored.
- The history-free dump (23 GB of SQL, finished 13:15Z) is the only archive
  of the old data. Compressing it and keeping a copy on node-a is cheap.
- The 104 G old repo can be deleted: the dump holds its current state.
  This is irreversible and waits for the owner's go-ahead.
- The source list for the MVP is chosen fresh. The old 211 sources remain
  readable in the dump if useful.

## 8. Decisions

Decided on 2026-10-09:
- **Sequencing:** Gate 1 today, Gate 2 tomorrow, Gate 3 MVP in October.
- **Names:** Autopaper, Autoputer and Autoradio, never camel case. "World
  Wire", "Universal Wire" and "sourcecycled" are retired; the rename is a
  TODO (§9).
- **Storage:** each computer embeds Dolt; the host-level Dolt behind
  corpusd is the global publish and read store. No cross-computer reading.
- **Old data:** start from zero content; no backwards compatibility.
- **Transclusion:** pinned to a revision, with a subtle newer-revision
  indicator.
- **Budget:** none until live; track resource usage first.
- **Ontology:** existing types only (§2); no new kinds unless needed.
- **Publishing:** automatic for Autopaper; manual for users; email
  separately gated.
- **Structure:** minimal. Spend scales with story size; many Textures and
  models per story; records are the protocol.
- **Editing:** only the Texture desk edits a Texture, woken by evidence.
- **MVP scope:** existing feeds, no beats, no shortcuts.

Open:
1. Capture body retention window (proposal: 30 days, cited bodies pinned).
2. Email distribution policy, after the MVP.
3. The MVP source list.
4. Deleting the 104 G old repo (irreversible; the dump holds its state).

## 9. TODO: retire the old names (owner, 2026-10-09; not done now)

"lets get rid of world wire, universal wire, and sourcecycled name.
everything can be autopaper or some derivation", then "dont rename it now,
just mark that as a todo."

Footprint measured 2026-10-09:

| Name | Code (non-Markdown) | Markdown files |
|---|---|---|
| Universal Wire | 65 files, 558 occurrences | 86 |
| sourcecycled | 33 files, 115 occurrences | 92 |
| World Wire | 6 files, 13 occurrences | 115 |

Notes for whoever does it:

- **Persisted identity, not just symbols.** `universal-wire-platform`
  (owner), `vm-universal-wire-platform` and
  `computer-universal-wire-platform` are stored in vmctl ownership on Node
  B. vmctl ensures that platform computer by these constants
  (`internal/vmctl/platform_computer.go`, `cmd/vmctl/main.go`,
  `internal/vmctl/handlers.go`), and the proxy authorizes by them
  (`internal/proxy/guest_authority.go`, `handlers.go`). Renaming the
  constants without a migration would orphan the existing platform
  computer and could start a new one. Prefer retiring it in W1, when the
  primary Autopaper computer replaces it, over an in-place rename.
- **sourcecycled** is slated for deletion. Delete rather than rename,
  after W1 moves the polling code in `internal/sources` into research's
  in-cell module. The host env file `/var/lib/go-choir/corpus-dsn.env` also
  carries `SOURCECYCLED_DOLT_DSN`.
- **Routes:** `/api/universal-wire/stories` and the corpusd
  `/internal/platform/universal-wire/*` endpoints are renamed or replaced
  when the Autopaper app gets its routes in W3. The host store stays.
- **Docs:** rename current documents and roadmap gate names. Dated
  receipts and `docs/archive/` keep their text, with a glossary line in the
  doctrine mapping the former names to Autopaper.
