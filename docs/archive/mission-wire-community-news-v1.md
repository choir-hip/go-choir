# MissionGradient: Universal Wire (Wire Community News)

Date: 2026-06-09 (v1; supersedes v0 after the first run). **Product name
(2026-06-10):** **Universal Wire** — formerly Universal Wire / Universal Wire in
user-facing copy. See
[universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md).

## Goal String

```text
/goal Run docs/mission-wire-community-news-v1.md as MissionGradient.
```

## Objective

Land **Universal Wire** as the public source-to-VText news instance of the Choir
Community Cloud (Universal Wire deployment).

Requirements contract:
[choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md).

## Operator Decisions (2026-06-09)

- **Platform computer migration:** in-scope for v1 (see Deployment Scope and
  architecture note below). Do not treat host `sandbox-m1` as the long-term
  platform computer.
- **Operator disk recovery:** prune guest caches first; Dolt compaction and
  proactive `data.img` monitoring are follow-on missions (not this one).
- **Prompt-initiated articles:** remove from edition and purge like StoryGraph /
  SourceMaxx remnants — not audit-and-keep.
- **Hacker News:** satisfied by existing RSS feeds (`rss:hn_best`, etc.); no
  separate Phase A row or dedicated adapter required.
- **Publication gate:** fully automatic newspaper on **Universal Wire (Community
  Cloud)** — ingest, process, write, publish, reconcile/update with **no
  operator approval gate** on the publish path. Private Wire instances may
  configure human gating per deployment policy. Personalization (user
  subscriptions, per-user rewrites with same sources) and email/newsletter
  agent are **tabled**, not v1 scope.
- **Canonical mission doc:** `mission-wire-community-news-v1.md` (autonomous-
  ingestion v1 is archived; see `mission-wire-autonomous-ingestion-v1.md`).
- **Platform computer uptime:** Universal Wire platform computer is **always-on**
  (100% uptime target); scale out to multiple or larger VMs later.
- **Cutover policy:** hard cutovers acceptable — no external users; negotiate
  boundaries explicitly between platform computer Dolt, corpusd, and future
  platform vector store.
- **Source fetch credentials:** MTProto (Telegram), ATProto, and similar adapter
  configs are **post-core** — after ingestion → processor → VText → auto-publish
  is proven on RSS/GDELT paths first.
- **Fetch ledger store:** SQLite on host for v1 (`sourcecycled.db`); Postgres or
  replicated store only when multi-host HA requires it. Platform-level **Qdrant**
  (or equivalent) is a follow-on for embeddings/search — not primary provenance.

## Data Store Boundaries (v1)

Three layers — do not collapse them:

```text
sourcecycled (host) — SQLite sourcecycled.db
  WHAT: source registry mirror, poll cursors, fetch/item ledger, adapter ops
  NOT: edition graph, article prose, publication routes

platform computer VM — embedded Dolt (universal-wire-platform)
  WHAT: Wire.vtext, Article/*.vtext, agent notebooks, processor evidence,
        transclusion refs to source items (by ID via sourcecycled API)
  NOT: raw RSS bodies as canonical truth without artifact rows; public routes

corpusd (host) — separate Dolt sql-server primary
  WHAT: published snapshot projections, slugs/routes, access policy,
        sanitized bundles for /pub/vtext and proxy read APIs
  NOT: live private editing; never write authority over platform-computer VTexts
```

**Publish flow:** processors/VText agents commit on platform-computer Dolt →
automatic publication step posts **selected public projection** to corpusd →
proxy/browser reads corpusd for signed-out and cross-user surfaces.

**Future platform vector DB (Qdrant):** embeddings and similarity search over
ingested source spans and/or article chunks. Indexes are caches; Dolt VTexts +
source artifact ledger remain provenance truth. Deploy after core pipeline works
(architecture accepted; v1 does not require Qdrant online).

**SQLite boundary (v1):** host `sourcecycled.db` is acceptable for the fetch
ledger while a single host runs `sourcecycled`. Platform-computer processors
and VText agents must read source items only through the sourcecycled HTTP API,
never by opening SQLite directly — preserves a clean cutover to Postgres or
replicated ledger later.

## Cognitive Transform Review (2026-06-09)

Current uncertainty: whether the v1 plan is a real automatic newspaper topology
or a repackaged prompt/proxy path with prettier docs.

Selected transforms:

1. **Depth extraction (automatic newspaper)** — banal version removes an approval
   button; deep version makes provenance + deletion + version semantics the
   verifier. Load-bearing variable: every published revision must trace to a
   fetch event no test/prompt/seed created.
2. **Commutative diagram (publish path)** — two paths must agree: edition truth
   on platform-computer Dolt and public projection on corpusd. Today
   `HandleVTextPublication` is user-JWT proxy publish; Wire auto-publish needs
   a **platform-internal** publish step (platform computer → corpusd).
3. **Gradient hacking** — mission passes if one pretty RSS story appears while
   legacy seeds, prompt articles, or StoryGraph shims remain in agent context.
   Verifier must include negative prompt check **and** Deletion Ledger grep-clean.
4. **Failure mode (always-on platform VM)** — operator PROBLEM 0 is disk-full on
   a user computer; platform computer needs its own disk budget, reclaim policy,
   and monitoring so Universal Wire does not repeat the same failure mode.
5. **Value of information / curriculum** — highest-information Phase A proof is
   **one RSS (+ GDELT) end-to-end chain** with full IDs before parallel adapter
   work. Telegram/MTProto is post-core, not a Phase A gate.

Route-changing insights:

- Auto-publish is **not implemented** for Wire; it must be an explicit slice
  (platform-internal projection to corpusd after VText revision), not assumed.
- Telegram HTML scraping stays forbidden, but **Telegram API proof is deferred**
  until RSS/GDELT auto-publish chain works (MTProto credentials post-core).
- Slice 0.5 platform VM must prove dispatch lands on platform sandbox URL, not
  host `sandbox-m1`.

Changed plan:

- **Implementation:** PROBLEM 0 → Slice 0 → Slice 0.5 → Slice 1 → RSS+GDELT
  curriculum → platform auto-publish → then MTProto/Telegram/Qdrant.
- **Verifier/evidence:** per-class matrix row requires fetch_id → ingestion event
  → processor run → VText revision → corpusd publication ref; prompt negative
  proof on every row.
- **Scope:** Phase A completion = RSS/Atom, GDELT, HN-via-RSS — not Telegram.
- **Stopping condition:** do not claim Phase A complete with Telegram stub,
  proxy-only publish, or host-sandbox platform authority.

Next high-information action: PROBLEM 0 operator disk prune, then Slice 0
deletion including prompt-initiated edition article.

## Blocking Gate: Operator Primary Computer (PROBLEM 0)

Wire mission work on the operator account (`yusefnathanson@me.com`) is blocked
until the operator primary computer boots with VText history intact. As of
2026-06-09, the published primary VM (`vm-5b0c1bef…`) fails because the guest
`data.img` is full (~7.8G/7.8G); sandbox startup logs `No space left on device`
when creating `/mnt/persistent/runtime/.sandbox-next`. A blank account
(`a@b.com`, `vm-d067e51c…`, ~327M disk) boots on the same deploy.

See `docs/incident-vm-bootstrap-stale-route-2026-06-09.md` (Sixth Finding).
Recovery (snapshot, cache prune or disk expand, serialized refresh) precedes
operator-scoped Wire acceptance proof. Platform-scoped ingestion work may
proceed on a different computer only when the operator explicitly scopes it
that way in writing.

## Required Launch Context

The operator may start this mission in a fresh thread using only the goalstring.
Therefore the worker must begin by reading this mission document and the
requirements contract above before making behavior changes.

Required context reads:

- [AGENTS.md](../../AGENTS.md) for repo operating contract, staging proof, and
  problem-documentation-first rules;
- [glossary.md](glossary.md) for current Community Cloud, Private Cloud,
  platform computer, user computer, candidate computer, and Wire vocabulary;
- [computer-ontology.md](../computer-ontology.md) for host/computer/candidate
  lineage and promotion boundaries;
- [wire-news-system-learning-saga-2026-06-09.md](wire-news-system-learning-saga-2026-06-09.md)
  for the news failure history and the platform-level realization;
- [choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md)
  for product/architecture requirements (includes invariant 21 / Activation);
- [mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md)
  for v0 evidence ledger and honest remaining-error field.

Do not treat old Universal Wire, StoryGraph, source-maxxing, source-ledger, or
Style.vtext-control documents as current requirements unless this mission or the
Wire spec explicitly mines them as historical failure evidence.

At mission start, run `git status --short` and classify dirty paths. The docs
created for this Wire ontology/mission are intentional mission context. Preserve
unrelated user/agent work. Do not overwrite or revert dirty files unless the
mission explicitly owns them and the diff has been read.

Before the first behavior-changing code commit, perform the repo-required
problem-documentation-first step: document the current fake/legacy Wire problem,
evidence, belief-state update, and remaining error field in the mission report
or a focused problem checkpoint doc. The fix commit(s) come after that
documentation checkpoint.

The shipped product should show live-updated important news from many public
sources in Wire by rendering a published edition VText that transcludes
VText-agent-authored articles/reports. Those VTexts transclude real source
artifacts. Platform processors and reconcilers run under Community Cloud
platform-computer authority. Userland personalization is designed but not
required to ship in this mission unless it falls out naturally.

This mission tables newsletter/email delivery, Autoradio, TTS/STT, Qdrant
**deployment**, deterministic clustering, native mobile, and automatic capital.
(Qdrant as platform vector store is architecturally accepted post-core.)

## Platform Computer Requirements

Universal Wire platform computer (`universal-wire-platform`):

- **Warmness:** always-on (100% uptime target); not hibernated like typical user
  primaries. Scale to multiple or larger VMs later.
- **Disk:** provision headroom above user-computer defaults; guest reclaim must
  include tool/build caches — Dolt growth monitoring is a follow-on mission but
  platform VM must not share the operator disk-full failure mode.
- **Authority:** sole writer of Wire VTexts and edition graph on embedded Dolt.

## Deployment Scope

Universal Wire runs on an **always-on Community Cloud platform computer**
(`universal-wire-platform`). Hard cutover from v0 paths is acceptable.

Document and execute migration for:

- `sourcecycled` stays on host — fetch + ledger + dispatch only;
- platform computer VM owns all Wire VTexts and agent semantic state (embedded Dolt);
- `SOURCE_SERVICE_RUNTIME_BASE_URL` must target platform-computer sandbox, not
  host `sandbox-m1`;
- auto-publish from platform computer → corpusd via **platform-internal**
  publish (not user JWT proxy path); no operator approval gate;
- public Wire app reads published edition via corpusd/proxy, not private Dolt.

Slice 0.5 (platform-computer migration) has its own evidence row before Slice 1
claims ingestion proof on platform authority.

## Real Artifact

The artifact is not a dashboard and not a legacy graph object.

The real artifact is:

```text
Community Cloud source artifacts
-> platform processor/reconciler/researcher notes and requests
-> VText-agent-authored Article/Report.vtexts
-> Wire.vtext public edition
-> Wire app renderer over the edition VText graph
```

The app may use indexes for speed. Indexes are rebuildable caches over VTexts
and source artifacts.

## Value Criterion

Minimize divergence between the public news product and the Wire/VText-native
ontology while increasing source breadth, source depth, article quality, update
freshness, and readable newspaper presentation.

Loss increases when:

- hardcoded mock/seed stories appear as product stories;
- source labels masquerade as full sources;
- Wire owns article prose outside VText;
- platform and user-computer ownership are blurred;
- legacy StoryGraph, source-maxxing, source-ledger, source-network rename
  shims, Global-Wire-as-ontology, or style-control ontology remains visible or
  authoritative;
- rankings are fake deterministic placeholders;
- article VTexts contain outlines/status/source manifests instead of
  publishable prose;
- source transclusions fail to open into source artifacts;
- update/version propagation silently changes meaning;
- tests protect old fake behavior.

## Quality Bar

Quality level: excellent.

The standard is:

- make it work: live source intake, real source artifacts, VText-authored
  articles/reports, real edition VText, product-path proof;
- make it nice: clean readable newspaper typography, no detritus, source-rich
  prose, strong update semantics.

## Hard Invariants

1. Wire is reusable source-to-VText infrastructure.
2. Universal Wire is platform-level work owned by a Choir Community Cloud
   platform computer, not a user-computer feature.
3. Private Wire reuse must remain possible; do not build one-off public-news
   code that cannot run in a Private Choir Cloud over private sources.
4. Personalization belongs in user computers and creates user-owned VTexts,
   forks, notes, alerts, preferences, and style.vtexts.
5. Articles/reports and editions are VTexts.
6. Article/report/edition writing and revision is owned by VText agents.
7. Processors and reconcilers do not write canonical VText prose directly.
8. Processors, reconcilers, researchers, supers, and coding agents may write
   durable notes/evidence/messages in their computer scope and request VText
   work.
9. External sources are durable source artifacts/ContentItems, not forced
   VTexts.
10. Sources are transcluded into VTexts through native source systems.
11. Related VTexts are transcluded VTexts.
12. Public/private source visibility and egress policy are preserved.
13. Transclusions preserve version semantics: pinned, live, or
    live-with-review.
14. Indexes are caches and must be rebuildable from VTexts/source artifacts.
15. Excise and delete legacy StoryGraph, SourceMaxx/source-maxxing/source-maxx,
    source-ledger, source-network rename shims, seed source neighborhoods,
    source chronology/search detritus, style-control panels, durable-storygraph
    labels, and hardcoded three-story fallback behavior from active product
    behavior, APIs, runtime/store types, tests, active docs, and user-visible
    copy.
16. Telegram ingestion uses proper Telegram API paths. Public preview HTML
    scraping is not an accepted fallback.
17. No hardcoded source trust tiers.
18. Wire app works in Future Noir, Carbon Kintsugi, and London Salmon with
    OS-wide theme only.
19. Staging proof on `https://choir.news` is required for behavior-changing
    completion.
20. Wire stories are activated by source ingestion events, never by human
    input. The prompt bar/Command prompt never creates, triggers, or supplies
    a Wire story. Human surfaces near Wire do editorial supervision only.
    Prompt-initiated story creation is a defect wherever found, including in
    paths built by the previous run.
21. "Product-path proof" means: initiated by a real ingestion event, observed
    through product surfaces. A prompt-initiated run is a debugging harness at
    best and is never acceptance evidence.
22. Acceptance provenance must be complete: every source artifact in an
    acceptance proof traces to a real external fetch event with URL/endpoint,
    retrieval timestamp, and fetcher run id that no test, seed, fixture, or
    prompt created. If the trace cannot be shown, the proof is invalid; say
    so rather than substituting.
23. Deletion is evidentiary, not declarative. A legacy ontology counts as
    deleted only when the Deletion Ledger (see Evidence Requirements) shows,
    per named symbol: grep-clean code and tests, dropped runtime/store types
    and routes, purged stored data on staging, and removed read-compat shims.
    Suppressing legacy data from one view, excluding it from one endpoint,
    keeping a reader "backward-compatible," or investing in fixtures that
    keep legacy route tests meaningful are all non-deletion. Routes and their
    tests are deleted together.
24. Source fetch ledger access: platform-computer runtimes read ingested items
    only via sourcecycled HTTP API, never by opening `sourcecycled.db`.
25. Auto-publish safety: with no human approval gate, prompt-initiated articles,
    seeded stories, and legacy graph rows are **contamination** — they enter
    agent context and must be hard-deleted in Slice 0 before auto-publish is
    enabled, not audited and kept.

## Current Belief State

Evidence from code/doc review on 2026-06-09 (see
[mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md)):

Lessons from the v0 run:

- The v0 run completed front-page honesty work (removed hardcoded frontend
  preview stories and read-time story seeding on the active stories path) but
  did not satisfy invariant 23: legacy store seed helpers, routes, tests,
  shims, and staging seed rows survive.
- Lacking a specified activation mechanism, the v0 run invented a prompt-bar
  trigger to produce its proof, then spent most of its cycles repairing that
  probe chain. The spec now defines activation explicitly (spec invariant 21);
  mission invariants 20-22 exist to prevent this recurrence.
- The one published article ("The Computer Science Degree Isn't Dead") was
  prompt-initiated over a Source Service top result of undemonstrated fetch
  provenance. It is not acceptance evidence. Its provenance must be audited.
- Prompt-path infrastructure that is entry-point-agnostic (compaction encoding,
  provider fallback, MiMo policy, edition transclusion gate) may stay.
  Prompt-initiated story-creation paths are removed or demoted to clearly
  marked debugging harnesses outside the product path.
- The v0 run suppressed legacy ontology instead of deleting it per the Deletion
  Ledger scope below.

Current codebase (post-v0, pre-v1):

- `configs/sources.json` contains 211 configured sources: 137 RSS, 73 Telegram,
  1 GDELT.
- `frontend/src/lib/UniversalWireApp.svelte` no longer hardcodes preview stories;
  it fetches `/api/universal-wire/stories` and renders honest empty state when
  the edition index is empty.
- `internal/store/universal_wire.go` still contains `defaultUniversalWireStories`,
  `universalWireSeedState`, and `ensureDefaultUniversalWireStories` (used by tests
  and legacy paths, not the v0-honest front-page read path).
- `internal/runtime/universal_wire.go` still mixes edition-VText index responses
  with legacy graph/fallback behavior on some routes.
- RSS ingestion stores feed summaries/excerpts, not consistently full article
  bodies.
- Telegram ingestion uses `internal/sources/telegram.go` HTML preview scraping
  (`t.me/s/…`); proper Telegram API is not implemented.
- GDELT uses one GKG source as metadata firehose.
- Hacker News is ingested today via RSS feeds (`rss:hn_best`, etc.), not a
  dedicated HN adapter. Phase A accepts either a dedicated adapter or a
  documented, proven RSS path — but not an unproven stand-in.
- No `IngestionEvent` contract or ingestion-only processor dispatch exists yet.
- Backend routes still expose graph-candidate, style-source, publication,
  newsletter, autoradio, and SourceMaxx-compat surfaces.

Highest-impact uncertainty:

- Platform-internal auto-publish path (platform computer → corpusd) is not
  proven to exist; user JWT proxy publish is a different topology.
- Where Universal Wire platform-computer authority lands in the current
  deployment (see Deployment Scope).
- The cleanest hard cutover from legacy graph/fallback to edition-VText truth
  without preserving fake compatibility behavior.

## Execution order (post Slice 2 checkpoint 2026-06-10)

After Slice 2 adapters landed, **architecture checkpoint (a)** records the
feed-forward activation graph. Implement workstreams **sequentially** — do not
start (c) before (b) is grep-clean, etc. Full matrix:
[universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md).

```text
(a) Architecture checkpoint + mission/spec amendments (docs)     [in progress]
(b) Workstream 1 — Deletion Ledger (replace SourceMaxx spine, then delete)
(c) Universal Wire rename/migration (edition alias, routes, copy; no redirects)
(d) Workstream 2 — Activation graph (Slice 3 dispatch + negative proofs)
(e) Staging acceptance — ingestion chain + fork/claim loop + Slice 4 matrix
```

## Phased Route

### Phase A — Core machinery (required before Phase B)

1. **Slice 0 (excision):** Deletion Ledger — **reopened 2026-06-10** because
   `BuildSourceMaxxHandoff` and related symbols remain on the ingestion path.
   Replace handoff/dispatch with neutral vocabulary, then grep-clean delete per
   symbol, drop types/routes with tests, purge staging seed rows and
   **prompt-initiated edition articles**, remove read-compat shims after purge.
   **Workstream (b).**
2. **Slice 0.5 (platform VM):** always-on `universal-wire-platform` computer;
   repoint `SOURCE_SERVICE_RUNTIME_BASE_URL` to platform-computer sandbox;
   evidence row: dispatch + Dolt owner binding, not host `sandbox-m1`.
3. **Slice 1:** Ingestion event contract; processor dispatch only from ingestion
   events; test that prompt-bar submission cannot produce an ingestion event.
   **Landed** (behavior on legacy handoff names until Workstream 1).
4. **Slice 2 (curriculum — RSS/GDELT first):** Adapters writing the same artifact
   + event shape:
   - RSS/Atom with conditional GET and full readability import where allowed;
   - GDELT broadened beyond the single GKG source;
   - Hacker News via proven RSS path (`rss:hn_best`, etc.) — counts as HN row.
   **Post-core (not Phase A gates):** Telegram via MTProto/API (delete HTML
   scraping when replacement lands); ATProto; Qdrant.
   **Landed** (deploy evidence in mission report).
5. **Slice 3 (Workstream 2 / (d)):** Feed-forward activation graph:
   ingestion event → processor (**vtext spawn only**) → VText autoregressive
   loop (researcher/super via VText) → Article VTexts with native source
   transclusions → autonomous publish → debounced reconciler → VText wake on
   edition `universal-wire/Wire.vtext` (after migration (c)).
6. **Slice 3b:** Autonomous platform publish to corpusd on Community Cloud —
   **no operator gate**; procedural guards (article-before-edition, fidelity)
   are load-bearing acceptance, not human approval. Folded into Workstream 2
   staging proof (e), not a separate operator workflow.
7. **Slice 4 (part of (e)):** Phase A staging evidence matrix (RSS/Atom, GDELT,
   HN-via-RSS) plus negative proofs (prompt bar cannot create Wire stories;
   reconciler cannot `edit_vtext`; processor cannot spawn researcher).

### Phase A-post-core (after Slice 4 proven)

- Telegram MTProto/API adapter (replace HTML scraping).
- ATProto adapter (when configured).

### Phase B — Transcript media (opens only when every Phase A row is proven)

- Podcast feeds (RSS enclosures; persist audio; transcript as processor step).
- YouTube channels/videos through proper API paths (persist media reference and
  transcript; index by transcript).

Same ingestion event contract and per-class proof requirements. Do not start
Phase B early.

### Phase C — Out of scope for this mission (design for, do not build)

- Open-web page watch over configured URLs with change-detection. Design the
  event contract and source-artifact fields so this slots in without rework.

## Forbidden Shortcuts

(See v0 mission report and spec invariant 21. Same as amended Downloads v1,
with these codebase-specific additions:)

- Do not claim v0 already deleted legacy ontology; invariant 23 is the bar.
- Do not treat existing `rss:hn_best` ingestion as proof of Phase A HN row
  until a dedicated matrix proof is recorded with full provenance chain.
- Do not repair or extend prompt-initiated story paths to make progress.

## Evidence Requirements

Completion requires named evidence for:

- source registry count and source classes after expansion;
- per-source-class ingestion-triggered proof for **Phase A rows only** (RSS/Atom,
  GDELT; Hacker News via proven RSS path), one row each, with full ID chain,
  corpusd publication ref, and negative prompt check;
- Telegram/MTProto and ATProto rows are **post-core**, not Phase A completion;
- provenance audit of the v0 published article;
- Deletion Ledger per named legacy symbol (see Downloads v1 list — unchanged);
- staging deploy SHA identity and acceptance proof;
- operator-computer recovery evidence when operator-scoped proof is required.

## Anti-Goodhart Checks

(Same as amended Downloads v1 — acceptance on prompt-initiated runs, untraceable
sources, or surviving legacy symbols fails the mission regardless of UI polish.)

## Dense Feedback / Rollback / Mission Report

Same discipline as v0. Maintain
[mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md).
PDF copy to `~/Library/Mobile Documents/com~apple~CloudDocs/mission reports/` at
mission end.

## Run Checkpoint And Resumption State

status: checkpoint_incomplete — **architecture checkpoint (a) 2026-06-10**;
next code workstream is **(b) Deletion Ledger**.

blocking gate:

- ~~Operator primary computer boot blocked (guest disk full).~~ **Resolved
  2026-06-09:** guest cache prune + refresh; sandbox `ready` on
  `vm-5b0c1bef…` (see incident doc Seventh Finding).
- **SourceMaxx spine on ingestion path** — must complete Workstream (b) before
  rename (c) and activation graph (d).

current artifact state:

- Front-page read path is honest-empty when no edition articles exist; v0
  removed frontend hardcoded preview stories and read-time story seeding on
  the active stories endpoint.
- **Slice 0 not complete:** `BuildSourceMaxxHandoff`, per-cycle reconciler
  dispatch, and legacy routes/types survive — see topology checkpoint.
- Staging VText purge for prompt-initiated edition articles done locally
  (2026-06-09); deploy evidence recorded separately.
- **Slice 1 landed:** ingestion events + processor dispatch from sourcecycled
  (still named SourceMaxx in code until Workstream 1).
- **Slice 2 landed:** RSS conditional GET + reader import; GDELT export stream
  (`7877e0aa` deploy evidence).
- No proven staging path: ingestion → processor → VText → publish → reconciler
  → correction-request → VText revision (Workstream 2 + (e)).
- Telegram HTML scraping still active; Telegram/MTProto adapter post-core.
- No platform-internal auto-publish path proven end-to-end.
- Platform-computer deployment binding not proven (dispatch still targets host
  `sandbox-m1` per `nix/node-b.nix`).

what shipped (v0):

- Front-page honesty slice, edition transclusion gate, compaction/provider
  repairs — see mission report evidence ledger.

unproven:

- Deletion Ledger grep-clean (Workstream **b**).
- Universal Wire rename with zero `universal-wire` references (Workstream **c**).
- Feed-forward activation graph and negative proofs (Workstream **d**).
- Staging proofs: ingestion chain + fork/claim loop + Slice 4 matrix (**e**).
- Platform-computer deployment binding documented and proven (Slice 0.5).

next step:

1. ~~Resolve PROBLEM 0~~ **Done 2026-06-09.**
2. ~~Slice 1 ingestion events~~ **Done.**
3. ~~Slice 2 RSS/GDELT adapters~~ **Done** (`7877e0aa`).
4. ~~Architecture checkpoint (a)~~ **Done 2026-06-10** —
   [universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md).
5. **Workstream (b):** Deletion Ledger — replace then delete SourceMaxx spine.
6. **Workstream (c):** Universal Wire rename/migration.
7. **Workstream (d):** Activation graph (Slice 3).
8. **Workstream (e):** Staging acceptance + Slice 4 matrix.
9. Slice 0.5 platform VM binding (may parallelize after (b) if unblocked).

## Related Documents

- [universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md) —
  activation graph, workstream order, negative proofs.
- [mission-wire-community-news-v0.md](mission-wire-community-news-v0.md) —
  superseded by this document for active work.
- [mission-wire-autonomous-ingestion-v1.md](mission-wire-autonomous-ingestion-v1.md) —
  archived; lessons folded into this document.
