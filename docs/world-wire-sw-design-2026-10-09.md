# World Wire redesign (station SW) — draft for owner review

Date: 2026-10-09. Status: **draft; decisions marked OPEN are the owner's.**
Station: `SW-world-wire-interface` in the
[app-dev metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md)
(design only; its output is constraints on Gates 1–2). Inputs:

- [10-05 re-architecture](world-wire-rearchitecture-2026-10-05.md);
- [mission stack Phase 5](world-wire-mission-stack-2026-09-22.md);
- the v6 owner decisions (2026-10-09);
- [Store B inventory](evidence/world-wire-store-b-inventory-2026-10-09.md);
- [precommitment records memo](<Precommitment Records — Engineering Memo.md>).

Owner, 2026-10-09: "lets do our world wire redesign before we commit to the
plan" (the plan for the old Store B data).

## 1. What we are building

An **automatic newspaper**. Choir observes global sources, turns what they
report into claims on an object graph, and keeps **living articles**. These
are Texture documents that revise themselves as new evidence arrives. It is
also **a platform for other people's papers**: tenants add private sources
and publish internally, privately, paywalled or publicly. Choir's own paper
is the first of these and the template for white-label papers.

The mechanism is the commitment ledger made public. An article's claims are
commitments with provenance. A later source either corroborates or
contradicts them, and a correction is an ordinary forward revision.

## 2. Names (OPEN)

"Universal Wire" is retired. Proposal: two names for two layers.

| Name | What it is |
|---|---|
| **Autopaper** | the application: one computer running the four desks to keep a paper (sources, claims, articles, publication). Choir's own paper is the *primary autopaper*; a tenant's is another autopaper. |
| **World Wire** | the shared network: the public claims (with provenance) that autopapers publish and other autopapers adopt. |

The desktop app that reads articles becomes "Autopaper" (or the paper's own
masthead name). Code identifiers follow: `universal-wire-platform` →
`autopaper-primary` (owner id), `/api/universal-wire/stories` →
`/api/autopaper/...`. The rename is mechanical and lands with W1, not before.

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
Autopaper computer (ordinary persistent computer; primary = platform-owned)
  tape + object graph + artifacts        <- the only state; no Store B
  management desk   attention, cadence, budget per item (the spend dial)
  research desk     observe sources (deterministic tool modules: fetch,
                    parse, dedup); extract claims; corroborate/contradict;
                    precommit before reads, resolve after
  engineering desk  writes and fixes source adapters in capsules
  Texture desk      writes, revises and publishes articles
        |
        | publish: a supervised Texture action binding one exact head
        v
  public projection (static snapshot served by the host; never fate-shares
  the live computer)                  -> choir.news/<paper>/<article>
        |
        | claims published to the World Wire (signed objects)
        v
  other autopapers adopt them onto their own tape   (built at tenant #2)
```

Rules (settled by owner decisions unless marked):

1. **Four desks only.** Deterministic steps (fetch, parse, dedup, schedule)
   are desk tool modules, never separate actors or host daemons.
2. **One computer per paper.** No host-side ingestion service and no shared
   corpus store. `sourcecycled`, the World Wire tables in corpusd and Store B
   are deleted at the end of the migration.
3. **Budget, not architecture, limits ingest.** Cost per item is a
   management dial with a declared default.
4. **Shared data is adopted objects, never Dolt sync.** See §6.
5. **Publishing is a Texture action the owner supervises** (S5/S6 surface).
   Policy-delegated autopublish comes later, per paper.

## 5. Data model and retention

Object kinds (from W0, minimal):

- `source`: an adapter config (RSS, Telegram, API, private feed);
- `source_version`: one immutable observation, with its body in content
  storage;
- `claim`, `entity`, `thread`;
- `article`: the Texture document; publication is a projection of one head.

Edges: `cites`, `asserts`, `about`, `corroborates`, `contradicts`,
`supersedes`, `transcludes`.

| Class | Kinds | Retention (proposal; OPEN on numbers) |
|---|---|---|
| Observation | `source_version` bodies | bounded window, default 30 days, then body dropped and the object kept as a stub (hash, URL, time). Bodies a published claim cites are pinned. |
| Knowledge | `claim`, `entity`, `thread`, edges | durable; never deleted for correctness, only superseded |
| Article | Texture documents and revisions | durable (canonical) |
| Publication | public projections | durable per published revision |
| Operations | fetch attempts, cycle logs | not stored as rows: tape events and counters only |

That is the O10 bound the old store lacked: growth is in knowledge, not in
logs or raw bodies.

## 6. World Wire: adopting another paper's claims (interface now, build later)

- A paper publishes claim objects with their provenance subgraph. Each one
  is content-addressed and signed by the publishing computer.
- A subscriber **adopts** an object by appending an adoption event to its
  own tape. The event carries the origin computer, the signature, the privacy
  class and the publisher's head.
- Adopted objects live in the subscriber's object graph under a
  foreign-origin namespace. They are cited like local objects and never
  rewritten.
- **Revocation:** the publisher issues a `supersedes` or retraction object.
  The subscriber's policy (management desk) decides whether to follow it.
- **Privacy class:** public, tenant-internal or private. Nothing private
  leaves a computer. The adoption verb refuses a class the subscriber is not
  entitled to.

v1 has a single paper, so v1 builds none of this. SW only fixes the shape so
that SH and S1 reserve the homes for it (§8).

## 7. Restart, schedules and the no-auto-resume rule (OPEN)

The paper polls on a schedule, so it needs **durable timer obligations**
("observe source S at T"). The owner rule says a restart never resumes
interrupted work on its own. Proposal: a scheduled observation is new work,
not resumed work. After a restart the schedule stays **paused** until
management (later) or the owner (now) resumes it. A missed poll is
recorded, not replayed. This keeps restart a visible failure while the
paper still runs unattended between restarts.

## 8. Constraints SW places on Gates 1–2

| Gate / station | Constraint |
|---|---|
| SL (Gate 1) | a durable timer obligation kind with an explicit post-restart state (`paused`), owned by management |
| SH (Gate 1) | retention classes per object kind (observation / knowledge / article / publication); a home for adopted foreign-origin objects; observation bodies are reconstructible or explicitly lossy |
| S1 (Gate 2) | capsule egress for source adapters (recorded network access); privacy class on objects |
| S5/S6 (Gate 2) | publish = a supervised Texture action binding one head plus provenance manifest; public projection served without the live computer |
| Texture contract | an article is an ordinary Texture document; revision on new evidence uses the same revision/commitment path as user documents |
| Platform | the public projection path exists outside Store B (static snapshots), so Store B can be deleted |

## 9. The old Store B data (proposal; OPEN)

With this design there is **no Store B** in the end state, so there is no
need to rebuild one. Per class:

| Data | Proposal |
|---|---|
| Whole store (current state) | keep the history-free dump, compressed, as a cold archive on Node B and node-a. Delete the 104 G repo. |
| The 148 user publications (38 the owner's, ~110 from 12 accounts) | restore as frozen public pages (static snapshots), so their URLs work again. Their authors can republish from their computers later. **OPEN:** restore, or retire with notice. |
| The wire's own 485 publications and 185 articles (June attempt) | archive only; the primary autopaper starts fresh |
| 211 sources | import as the primary autopaper's initial source list |
| 2.4M items, 8.8M objects | archive only; old news is not seed material. Use them as an offline evaluation set for claim extraction (the 10-05 director note: decide the factorization from the frozen corpus). |
| ~12M log rows | archive only (inside the dump) |

## 10. Build order for Gate 3 (sketch)

- **W1:** the primary autopaper computer. Rename, source adapters
  (RSS/Telegram), observation with retention, and the timer obligation.
- **W2:** claims, entities and threads by the research desk, with
  precommitment records around reads.
- **W3:** articles as Texture documents, revision on new evidence, and a
  supervised publish to a public projection.
- **W4:** the reading surface: the Autopaper app and public pages.
- **W5:** World Wire adoption, built at the second paper.

## 11. Open decisions for the owner

1. Names: Autopaper (app/computer) and World Wire (shared claims network)?
2. The primary paper's public name and URL shape on choir.news.
3. User publications from June: restore as frozen pages, or retire?
4. Observation retention default (proposal: 30 days, cited bodies pinned).
5. Schedules after restart: paused until resumed (proposal), or something
   else?
6. First edition scope: general news first, then verticals (AI, Taiwan and
   geopolitics, semiconductors, internal democracy) per the 10-01 decision.
   Still right?
