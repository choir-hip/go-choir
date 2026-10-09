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

- Source adapters write each item as a `report` object in the paper's
  object graph: source, time, URL, language and a content-addressed body.
  The adapters are deterministic tool modules, and the existing adapters
  (RSS, Telegram web preview, GDELT, Polymarket in `internal/sources`) are
  enough for the MVP.
- Agents may write any further objects they find useful (stories, claims,
  entities, discrepancies, translations) through the open kind registry.
  The design does not require them.

### Storage that cannot repeat the 104 G problem

- **The paper's computer** (embedded Dolt plus content storage) holds
  working state. Report bodies live in content storage, not Dolt rows.
  High-volume tables stay out of Dolt history or are committed on a
  bounded cadence (to be settled in SH; Dolt's ignored-table mechanism is
  the first candidate). Textures and records are versioned.
- **The host-level store** (the corpus Dolt behind corpusd) holds published
  Textures and their revisions, and serves every logged-out read. Its
  history has a declared bound.
- Retention: report bodies have a bounded window (OPEN: 30 days), then
  shrink to a stub. Bodies cited by a Texture or a record are pinned.

### Textures: one writer, woken by evidence

- Only the Texture desk writes a Texture revision.
- Evidence reaches a Texture as a delivery (a report, another Texture's
  record, a research result). The delivery wakes the Texture desk for that
  Texture. It may call research, management or engineering, waits for
  what it needs, then revises and yields.
- Transclusion is how Textures reference graph objects and each other. A
  transcluded item changes what a reader sees only through a Texture
  revision.
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
- **Research:** observes through the adapters, searches the web sparingly
  (rate limits and cost), checks provenance, writes graph objects and
  resolves records. Its authority is read-only toward the world.
- **Engineering:** fixes adapters and tool modules in capsules, when cast.
- **Texture:** the only writer of Textures. It is woken by evidence,
  revises and yields, and publishes automatically.

## 4. Inference

Spend scales with story size and nothing else is fixed. The levers are
management's: how many models, how many Textures, how much research and
search a story gets. The cheap defaults that need no workflow:

- Adapters and resolution by counting cost no model calls.
- Texture revisions happen only when evidence arrives.
- A small story gets one Texture and few calls. A big one gets many
  Textures, many models and more research.

Cost per story and per revision is measured and shown to management. It
is not designed in advance.

## 5. Constraints on Gates 1–2

| Gate / station | Constraint |
|---|---|
| SL (Gate 1) | a delivery to a Texture durably wakes the Texture desk; durable timer obligations for the source schedule, paused after a restart (owner rule) |
| SH (Gate 1) | report bodies in content storage; bounded history for high-volume tables; retention classes; records immutable |
| Texture contract (Gate 1) | sole writer; woken by deliveries; may call the other desks before revising; model per activation; several Textures per story; transclusion of graph objects and Textures |
| S5/S6 (Gate 2) | publishing is a visibility state agents can set; published revisions reach the host store; logged-out reads never touch the paper's computer |
| Platform | the host store holds published Textures and serves reads, with a declared history bound; email is a separate, policy-gated channel |

## 6. MVP build order (Gate 3, October)

No new feeds and no beats. Each step lands through the Landing Loop with
deployed acceptance.

- **W1 Content in the graph:**
  - the primary Autopaper computer;
  - the existing adapters as research tool modules, writing reports with
    bodies in content storage and bounded history;
  - the source schedule as timer obligations;
  - the June publications restored into the host store.
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

Later, not in the MVP: AT Protocol and MTProto adapters, other feeds,
private feeds, beats (Taiwan first, then Asian-language social media).

## 7. The old Store B data (proposal)

The corpus store behind corpusd stays as the host-level store. It was
reset empty at 12:33Z. The history-free dump finished at 13:15Z (23 GB of
SQL).

| Data | Proposal |
|---|---|
| All 633 publications with their chain and the 926 routes | migrate into the host store so the URLs work again (148 are users' work, 38 the owner's) |
| The 185 platform Texture documents (545 revisions) | migrate into the host store |
| 211 sources | import into the primary Autopaper as its initial source list |
| 2.4M items, 8.8M objects | keep in the dump; not part of the MVP |
| ~12M log rows | archive only (inside the dump) |
| The 104 G old repo | delete once the migrated tables' row counts match the dump; keep the compressed dump |

## 8. Decisions

Decided on 2026-10-09:
- **Sequencing:** Gate 1 today, Gate 2 tomorrow, Gate 3 MVP in October.
- **Names:** Autopaper, Autoputer and Autoradio, never camel case. "World
  Wire", "Universal Wire" and "sourcecycled" are retired; the rename is a
  TODO (§9).
- **Storage:** each computer embeds Dolt; the host-level Dolt behind
  corpusd is the global publish and read store. No cross-computer reading.
- **Publishing:** automatic for Autopaper; manual for users; email
  separately gated.
- **Structure:** minimal. Spend scales with story size; many Textures and
  models per story; records are the protocol.
- **Editing:** only the Texture desk edits a Texture, woken by evidence.
- **MVP scope:** existing feeds, no beats, no shortcuts.

Open:
1. Starting daily budget.
2. Report body retention window (proposal: 30 days, cited bodies pinned).
3. Email distribution policy, after the MVP.

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
  after W1 moves the adapters in `internal/sources` into research tool
  modules. The host env file `/var/lib/go-choir/corpus-dsn.env` also
  carries `SOURCECYCLED_DOLT_DSN`.
- **Routes:** `/api/universal-wire/stories` and the corpusd
  `/internal/platform/universal-wire/*` endpoints are renamed or replaced
  when the Autopaper app gets its routes in W3. The host store stays.
- **Docs:** rename current documents and roadmap gate names. Dated
  receipts and `docs/archive/` keep their text, with a glossary line in the
  doctrine mapping the former names to Autopaper.
