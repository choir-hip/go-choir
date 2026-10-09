# Autopaper redesign (station SW) — draft for owner review

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

**Autopaper** is a maintained, research-intensive automatic newspaper with a
customized article interface (canonical context §5.2):

- It ingests sources continuously and keeps a knowledge graph of
  source-linked claims.
- Its articles are **living documents**. Texture revises them as evidence
  arrives, and corrections are ordinary forward revisions.
- **Published articles are free to read** at ordinary serving cost. The
  public paper is the demo anyone can read without a computer or a model.
- **Personalized investigations and customized articles are paid.** They
  run on the reader's own computer, as metered inference, and read
  published material from the host-level store like everyone else.
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

- **Autopaper** is the product. Product names are never camel case:
  Autopaper (or autopaper), Autoputer, Autoradio (owner, 2026-10-09; the
  canonical context will be corrected). "Universal Wire" is retired from the
  UI. The desktop app becomes Autopaper.
- "World Wire", "Universal Wire" and "sourcecycled" are retired (owner,
  2026-10-09). Everything is Autopaper or a derivation. The shared-claims
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

Two levels of Dolt, by design (owner, 2026-10-09: "the whole point of the
architecture is to have a dolt db embedded in vm and at the host level, for
global access for publishing and reading"):

- **In each computer:** an embedded Dolt holds that computer's working state
  (sources, observations, claims, records, Texture documents).
- **At the host:** one Dolt (the corpus store, served by corpusd) is the
  global publication store. Every computer publishes into it, and every
  reader (public pages, the Autopaper app, other computers) reads from it.
  There is no computer-to-computer path.

```text
Primary Autopaper computer (ordinary persistent computer, platform-owned)
  embedded Dolt: tape + object graph + artifacts (working state)
  management desk   attention, cadence, budget per item (the spend dial)
  research desk     observe sources (deterministic tool modules: fetch,
                    parse, dedup); extract claims; corroborate/contradict;
                    precommit before investigations and reads, resolve after
  engineering desk  writes and fixes source adapters in capsules
  Texture desk      writes, revises and publishes articles
        |
        | publish: precommit -> authorize -> publication transaction
        v
Host-level Dolt (corpus store, corpusd)   <- global publish + read
  published article revisions, published claims + provenance, routes
        |                         |
        v                         v
  public pages / Autopaper app    reader's own computer (paid):
  (free; never touch the live     personalized investigation reads
   paper computer)                published material from here
```

Rules (settled by owner decisions unless marked):

1. **Four desks only.** Deterministic steps (fetch, parse, dedup, schedule)
   are desk tool modules, never separate actors or host daemons.
2. **One computer per paper; the host store holds only what is
   published.** No host-side ingestion service: `sourcecycled` is deleted.
   Raw observations, fetch logs and working claims stay in the paper's
   embedded Dolt. The host store holds published material only.
3. **Budget, not architecture, limits ingest.** Cost per item is a
   management dial with a declared default. Free readers never trigger
   inference; only paid work and the paper's own budget do.
4. **Publishing is a transaction into the host store, never Dolt branch
   sync** (§6).
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

## 6. Publishing and reading through the host store

- **Publish:** a computer sends a publication transaction to corpusd: one
  exact Texture head, the claims it cites with their provenance, the
  precommitment, the authorization receipt, and the route. corpusd writes it
  to the host store. A correction is a new transaction that supersedes the
  old revision, which stays readable.
- **Read:** public pages, the Autopaper app and any computer read published
  material from the host store through corpusd. Reading never wakes or
  touches the publishing computer, so a paper restart never takes the paper
  offline.
- **Cite:** a document on any computer cites a published object by its
  host-store identity and revision. The citation stays valid when the
  object is superseded, and the reader can see the newer revision.
- **Bound:** the host store's own history needs a declared bound (O10). The
  old store reached 104 G of history from ingestion churn; publications are
  far smaller, but the bound is still stated (the same retention decision
  as Store A).

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
| SH (Gate 1) | retention classes per object kind; precommitment records immutable and durable; observation bodies are reconstructible or explicitly lossy |
| S1 (Gate 2) | capsule egress for source adapters (recorded network access) |
| S5/S6 (Gate 2) | publish = precommit + owner authorization + a publication transaction binding one head into the host store |
| Texture contract | an article is an ordinary Texture document; revision on new evidence uses the same revision/commitment path as user documents |
| Platform | the host store (corpusd) accepts publication transactions and serves all reads; reads never depend on the publishing computer; the host store's history has a declared bound |

## 9. The old Store B data (proposal; OPEN)

The fresh, empty corpus store (since the 12:33Z reset) **is** the host-level
store of this design. So the plan is to migrate what is published back into
it and keep it lean:

| Data | Proposal |
|---|---|
| All 633 publications with their chain (versions, reviews, provenance, retrieval manifests, citations, artifacts) and the 926 routes | migrate into the host store from the history-free dump, so every URL works again (148 are users' work, 38 the owner's) |
| The 185 platform Texture documents (545 revisions) | migrate into the host store with the publications |
| 211 sources | import as the primary Autopaper's initial source list (its embedded Dolt) |
| 2.4M items, 8.8M objects | not in the host store (working data, not published). Keep in the dump; import into the paper's embedded Dolt at W1 only if useful, or use as an offline evaluation set for claim extraction. Scores against them are retrospective, not PSA evidence (canonical context §12.9). |
| ~12M log rows | archive only (inside the dump) |
| The 104 G old repo | delete once the migrated tables' row counts match the dump; keep the compressed dump as the archive |

## 10. Build order (sketch)

- **W1:** the primary Autopaper computer: source adapters (RSS/Telegram),
  observation with retention, the timer obligation, and the rename.
- **W2:** claims, entities and threads by the research desk, with
  precommitment records around investigations and reads.
- **W3:** articles as Texture documents, revision on new evidence, and
  publish (precommit, authorize, project).
- **W4:** free reading surfaces (the Autopaper app and public pages)
  reading the host store.
- **W5:** paid personalized investigations on readers' computers, citing
  published material from the host store.
- **Later:** white-label papers publishing into the same host store, and
  Autoradio on the same published material.

## 11. Open decisions for the owner

1. ~~Sequencing.~~ **Decided (owner, 2026-10-09):** Gate 1 today, Gate 2
   tomorrow, Gate 3 (Autopaper) in October. The v6 order holds and the
   October launch target holds.
2. ~~"World Wire" as an internal name.~~ **Decided (owner, 2026-10-09):**
   retire "World Wire", "Universal Wire" and "sourcecycled"; everything is
   Autopaper or a derivation. The rename is a TODO, not done now (§12).
3. The primary paper's public name and URL shape on choir.news.
4. ~~Cross-computer reading.~~ **Decided (owner, 2026-10-09):** none.
   Publishing and reading go through the host-level Dolt; restore the old
   publications into it (§9).
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
  primary Autopaper computer replaces it, over an in-place rename.
- **sourcecycled** is slated for deletion (§4 rule 2). Delete rather than
  rename, after W1 moves any reusable source adapters into desk tool
  modules. The host env file `/var/lib/go-choir/corpus-dsn.env` also
  carries `SOURCECYCLED_DOLT_DSN`.
- **Routes:** `/api/universal-wire/stories` and the corpusd
  `/internal/platform/universal-wire/*` endpoints are renamed or replaced
  when the Autopaper app gets its routes in W4. The host store stays.
- **Docs:** rename current documents and roadmap gate names. Dated
  receipts and `docs/archive/` keep their text, with a glossary line in the
  doctrine mapping the former names to Autopaper.
