# AutoPaper redesign (station SW) — draft for owner review

Date: 2026-10-09. Status: **draft; decisions marked OPEN are the owner's.**
Station: `SW-world-wire-interface` in the
[app-dev metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md)
(design only; its output is constraints on Gates 1–2). Inputs:

- *Choir — Canonical Project Context* (2026-10-08, owner-supplied);
- *Prospective Self-Alignment* working paper (2026-10-08, owner-supplied);
- [10-05 re-architecture](world-wire-rearchitecture-2026-10-05.md);
- [mission stack Phase 5](world-wire-mission-stack-2026-09-22.md);
- the v6 owner decisions (2026-10-09);
- [Store B inventory](evidence/world-wire-store-b-inventory-2026-10-09.md);
- [precommitment records memo](<Precommitment Records — Engineering Memo.md>).

Owner, 2026-10-09: "lets do our world wire redesign before we commit to the
plan" (the plan for the old Store B data), then "review these docs first"
(the canonical context and the PSA paper).

## 1. What we are building

**AutoPaper** is a maintained, research-intensive automatic newspaper with a
customized article interface (canonical context §5.2):

- It ingests sources continuously and keeps a knowledge graph of
  source-linked claims.
- Its articles are **living documents**. Texture revises them as evidence
  arrives, and corrections are ordinary forward revisions.
- **Published articles are free to read** at ordinary serving cost. The
  public paper is the demo anyone can read without a computer or a model.
- **Personalized investigations and customized articles are paid.** They
  run on the reader's own computer, as metered inference.
- **Precommitment records are the mechanism.** An investigation commits
  before it reads, a claim carries its uncertainty and a resolution spec,
  and publishing is a material action with an authorization gate (PSA paper
  §2, §6). Media is the proving ground because claims resolve and stay
  inspectable (PSA §7.1).
- The first customers are prosumer writers and researchers who publish and
  build a track record. The platform side comes later: white-label papers
  for organizations on private plus public data.

Status discipline (canonical context §4.5, §12): PSA learning, oversight
and reputation effects are research hypotheses. This design records the
evidence they need; it does not claim them.

## 2. Names

- **AutoPaper** is the product (settled in the canonical vocabulary).
  "Universal Wire" is retired from the UI. The desktop app becomes AutoPaper.
- "World Wire", "Universal Wire" and "sourcecycled" are retired (owner,
  2026-10-09). Everything is AutoPaper or a derivation. The shared-claims
  layer needs no product name.
- Code: per canonical context §12.4, existing identifiers are not renamed
  for their own sake. New code is named `autopaper`. The
  `universal-wire-platform` identifiers go away with the code they belong to
  (the corpusd endpoints, `sourcecycled`), which is an engineering reason.
- Code keeps the term `precommitment records`; PSA is the theory.

## 3. Why the first attempt failed (short)

Receipts are in the 10-05 doc and the June attempt report:

- **Wrong plane:** ingestion and processing ran as host daemons beside the
  control plane. They had no computer, no tape and no capsule.
- **Shared store:** one Dolt store held 8.8M objects, 2.4M items and ~12M
  log rows. Its history alone grew to 104 G, with no declared retention.
- **Opaque queues:** processor/reconciler requests failed as 502s in a
  journal, which nothing could supervise.
- **Read path through the store:** the app read stories from corpusd and
  Store B. When that connection broke, the articles didn't load.

## 4. Architecture

```text
Primary AutoPaper computer (ordinary persistent computer, platform-owned)
  tape + object graph + artifacts        <- the only state; no Store B
  management desk   attention, cadence, budget per item (the spend dial)
  research desk     observe sources (deterministic tool modules: fetch,
                    parse, dedup); extract claims; corroborate/contradict;
                    precommit before investigations and reads, resolve after
  engineering desk  writes and fixes source adapters in capsules
  Texture desk      writes, revises and publishes articles
        |
        | publish: precommit -> authorize -> project one exact head
        v
  public projection (static snapshot served by the host; never
  fate-shares the live computer)    -> free reading on choir.news
        |
        | governed query interface (read-only, metered)
        v
Reader's own computer (paid)
  personalized investigation / customized article
  reads the paper's public claims; adopts the ones it cites onto its own tape
```

Rules (settled by owner decisions unless marked):

1. **Four desks only.** Deterministic steps (fetch, parse, dedup, schedule)
   are desk tool modules, never separate actors or host daemons.
2. **One computer per paper.** No host-side ingestion service and no shared
   corpus store. `sourcecycled`, the paper tables in corpusd and Store B are
   deleted at the end of the migration.
3. **Budget, not architecture, limits ingest.** Cost per item is a
   management dial with a declared default. Free readers never trigger
   inference; only paid work and the paper's own budget do.
4. **Shared data is adopted objects, never Dolt sync** (§6).
5. **Publishing is a material action.** Texture precommits (claims,
   uncertainty, expected corrections, consequences), the owner authorizes
   in the supervision surface, and the projection binds one head. Delegated
   autopublish comes later, per paper, under a policy.

## 5. Data model, records and retention

Object kinds (W0 set plus records):

- `source`: an adapter config (RSS, Telegram, API, private feed);
- `source_version`: one immutable observation, with its body in content
  storage;
- `claim`: carries uncertainty and a resolution spec (what evidence would
  settle it, and by when);
- `entity`, `thread`;
- `article`: the Texture document; publication is a projection of one head;
- `precommitment` (existing records work): commitments around
  investigations, reads and publishes, resolved later; nested.

Edges: `cites`, `asserts`, `about`, `corroborates`, `contradicts`,
`supersedes`, `transcludes`, `resolves`.

**Resolution and track records.** A new `source_version` that bears on an
open claim wakes it: the mission stack's impact propagation, the same
minted-continuation primitive as the wake obligations. The resolution is
recorded, and the article revises. Per-source and per-desk track records are
derived views over resolved records. They feed the reputation hypothesis
(PSA §7, H4) and are never edited by hand.

| Class | Kinds | Retention (proposal; OPEN on numbers) |
|---|---|---|
| Observation | `source_version` bodies | bounded window, default 30 days, then body dropped and the object kept as a stub (hash, URL, time). Bodies cited by a claim or a record are pinned. |
| Knowledge | `claim`, `entity`, `thread`, edges | durable; superseded, never deleted for correctness |
| Records | `precommitment` and resolutions | durable and immutable (the evidence PSA depends on) |
| Article | Texture documents and revisions | durable (canonical) |
| Publication | public projections | durable per published revision |
| Operations | fetch attempts, cycle logs | not stored as rows: tape events and counters only |

That is the O10 bound the old store lacked: growth is in knowledge and
records, not in logs or raw bodies.

## 6. Reading across computers (launch needs part of it)

- **Read (launch):** a reader's computer queries the paper's public claims
  through a governed, metered, read-only interface. This is the Agent API
  shape (canonical context §1) pointed at Choir's own paper. Nothing private
  crosses.
- **Adopt on cite (launch):** when a personalized document cites a paper
  claim, the reader's computer adopts that object. It appends an adoption
  event carrying the origin computer, signature, privacy class and the
  publisher's head. The citation stays stable if the paper later supersedes
  the claim; the reader sees the supersession and can follow it.
- **Publish and subscribe between papers (later):** tenant papers adopting
  each other's claims, revocation through `supersedes`/retraction, and
  privacy classes (public, tenant-internal, private) all reuse the same
  adoption event.

## 7. Restart, schedules and the no-auto-resume rule (OPEN)

The paper polls on a schedule, so it needs **durable timer obligations**
("observe source S at T"). The owner rule says a restart never resumes
interrupted work on its own. Proposal: after a restart the schedule stays
**paused** until management (later) or the owner (now) resumes it. A missed
poll is recorded, not replayed. Open claims waiting for resolution are not
lost; they wake on the next observation after resume.

## 8. Constraints SW places on Gates 1–2

| Gate / station | Constraint |
|---|---|
| SL (Gate 1) | a durable timer obligation kind with an explicit post-restart state (`paused`), owned by management; the impact-propagation wake (new evidence wakes open claims) uses the same obligation substrate |
| SH (Gate 1) | retention classes per object kind; precommitment records immutable and durable; a home for adopted foreign-origin objects; observation bodies are reconstructible or explicitly lossy |
| S1 (Gate 2) | capsule egress for source adapters (recorded network access); privacy class on objects |
| S5/S6 (Gate 2) | publish = precommit + owner authorization + projection of one head; public projection served without the live computer |
| Texture contract | an article is an ordinary Texture document; revision on new evidence uses the same revision/commitment path as user documents |
| Platform | static public projection outside Store B; a governed, metered read interface to a computer's public objects |

## 9. The old Store B data (proposal; OPEN)

With this design there is **no Store B** in the end state, so there is no
need to rebuild one. Per class:

| Data | Proposal |
|---|---|
| Whole store (current state) | keep the history-free dump, compressed, as a cold archive on Node B and node-a. Delete the 104 G repo. |
| The 148 user publications (38 the owner's, ~110 from 12 accounts) | restore as frozen public pages (static snapshots) so their URLs work. These are the prosumer writers the product is for; their authors can republish from their computers later. |
| The paper's own 485 publications and 185 articles (June attempt) | archive only; the primary AutoPaper starts fresh |
| 211 sources | import as the primary AutoPaper's initial source list |
| 2.4M items, 8.8M objects | archive only; old news is not seed material. Use them as an offline evaluation set for claim extraction (the 10-05 director note). They are not prospective evidence: any claim scored against them is retrospective (canonical context §12.9). |
| ~12M log rows | archive only (inside the dump) |

## 10. Build order (sketch)

- **W1:** the primary AutoPaper computer: source adapters (RSS/Telegram),
  observation with retention, the timer obligation, and the rename.
- **W2:** claims, entities and threads by the research desk, with
  precommitment records around investigations and reads.
- **W3:** articles as Texture documents, revision on new evidence, and
  publish (precommit, authorize, project).
- **W4:** free reading surfaces (the AutoPaper app and public pages) plus
  restored user publications.
- **W5:** the governed read interface and adopt-on-cite for paid
  personalized investigations.
- **Later:** paper-to-paper publish and subscribe, white-label templates,
  AutoRadio on the same claims.

## 11. Open decisions for the owner

1. ~~Sequencing.~~ **Decided (owner, 2026-10-09):** Gate 1 today, Gate 2
   tomorrow, Gate 3 (AutoPaper) in October. The v6 order holds and the
   October launch target holds.
2. ~~"World Wire" as an internal name.~~ **Decided (owner, 2026-10-09):**
   retire "World Wire", "Universal Wire" and "sourcecycled"; everything is
   AutoPaper or a derivation. The rename is a TODO, not done now (§12).
3. The primary paper's public name and URL shape on choir.news.
4. User publications from June: restore as frozen pages, or retire?
5. Observation retention default (proposal: 30 days, cited bodies pinned).
6. Schedules after restart: paused until resumed (proposal)?
7. First edition scope: general news first, then verticals (AI, Taiwan and
   geopolitics, semiconductors, internal democracy) per the 10-01 decision.
   Still right?
8. Who resolves claims: the research desk against later sources only, or
   also an independent scorer (canonical context §13, "who provides the
   independent resolution")? Proposal: research resolves, a second model
   scores, and disagreements are flagged, per the memo's scorer interface.

## 12. TODO: retire the old names (owner, 2026-10-09; not done now)

"lets get rid of world wire, universal wire, and sourcecycled name.
everything can be autopaper or some derivation" — then "dont rename it now,
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
  primary AutoPaper computer replaces it, over an in-place rename.
- **sourcecycled** is slated for deletion (§4 rule 2). Delete rather than
  rename, after W1 moves any reusable source adapters into desk tool
  modules. The host env file `/var/lib/go-choir/corpus-dsn.env` also
  carries `SOURCECYCLED_DOLT_DSN`.
- **Routes:** `/api/universal-wire/stories` and the corpusd
  `/internal/platform/universal-wire/*` endpoints go away with Store B
  (§9); the AutoPaper app gets new routes in W4.
- **Docs:** rename current documents and roadmap gate names. Dated
  receipts and `docs/archive/` keep their text, with a glossary line in the
  doctrine mapping the former names to AutoPaper.
