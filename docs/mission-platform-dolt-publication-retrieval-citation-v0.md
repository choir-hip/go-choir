# MissionGradient: Platform Dolt Publication Retrieval Citation v0

Status: active/landing
Date: 2026-05-16
Operator: Codex directly in the repo
Research input:
[platform-dolt-publication-retrieval-citation-research-2026-05-16.md](platform-dolt-publication-retrieval-citation-research-2026-05-16.md)

## Real Artifact

The artifact is a deployed staging platform service backed by platform Dolt that
can publish one selected private VText revision into immutable platform-visible
records, render it through a public route, generate retrieval source/span
manifests, and record at least one citation candidate or accepted citation edge.

Implementation note: v0 is the `platformd` service plus a separate
localhost-only `dolt sql-server` primary on Node B. The product write entrypoint
is the authenticated proxy route `POST /api/platform/vtext/publications`; the
public read route is `/pub/vtext/...`.

The platform Dolt deployment is a separate `dolt sql-server` service, not an
embedded Dolt workspace inside the platform application process. The intended
end-state is a beefy platform database service: single write primary,
direct-to-standby replication for high availability, read replicas and remotes
or backups for scale and disaster recovery, and Choir-owned service APIs for all
write admission.

This is not a generic public page, not a schema sketch, not a vector search demo,
and not a CHIPS prototype. Those are projections. The real artifact is the first
trust-domain bridge from private user-computer Dolt into platform Dolt.

The platform service must make these facts first-class:

- publication proposal and author consent;
- source private document/revision identity without leaking private-only
  content or paths;
- immutable public publication version;
- content-addressed public artifact manifest/blob refs;
- public route to the immutable version;
- retrieval source/span identity derived from the published version;
- citation candidate or accepted citation edge anchored to exact source spans;
- provenance, verifier/review, rollback, and supersession records.

Platform Dolt is the ledger and queryable index for platform-visible facts. It
is not the live message bus, the private computer store, a blob store, a vector
database, or a direct browser API.

## Invariants

- Staging is the acceptance environment for behavior-changing proof:
  `https://draft.choir-ip.com`.
- Per-user embedded Dolt remains the private mutable user-computer ledger.
- Platform Dolt receives only selected platform-visible projections, manifests,
  refs, and evidence.
- Platform Dolt runs as a separate server/process boundary. User-computer Dolt
  is embedded; platform Dolt is not.
- Platform Dolt has one authoritative write primary. Do not build the product
  around multi-primary OLTP assumptions.
- A private VText can continue changing after publication; the public version
  remains immutable.
- Publication does not grant platform services write access to a user's private
  live document.
- Redaction/projection, when present, creates a new public projection hash; it
  does not expose private content by reference.
- Large bytes live in content/blob storage with Dolt metadata, not opaque Dolt
  rows.
- Retrieval/vector/FTS indexes are derived caches. Canonical retrieval identity
  lives in platform Dolt rows: source, version, selector, hash, index manifest.
- Citation edges are typed and stateful. They are not decorative URL lists.
- Citation edges attach to exact version/span refs where possible.
- Browser-public APIs do not connect to Dolt directly and do not call internal
  or test-only routes as acceptance proof.
- Every platform write records actor, authority, consent or review state,
  source trace/run where available, and rollback/supersession semantics.
- Do not implement CHIPS, wallets, staking, token billing, public citation
  scoring, paywalls, or broad governance in this mission.
- Node B tracked files are not edited directly as a deployment shortcut.
- Behavior-changing commits follow the landing loop:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

## Value Criterion

Minimize divergence between private authored meaning and platform-visible public
memory while preserving privacy, consent, exact version identity, provenance,
retrieval usefulness, citation auditability, and rollback.

The loss function penalizes:

- platform rows that point at mutable private state instead of immutable
  publication artifacts;
- public routes that cannot be traced to exact source revision/projection
  hashes;
- citations that are generated text decorations rather than typed source-span
  edges;
- retrieval indexes that become canonical without rebuildable source manifests;
- platform service writes without consent/review/verifier provenance;
- private trace, prompt, path, unpublished revision, or source leakage;
- schema hardening before citation/retrieval/provenance semantics are tested;
- local-only proof for platform publication behavior.

## Quality Gradient

Expected quality: `solid`.

A solid outcome:

- has a clear platform Dolt service boundary and schema migration path;
- stores publication, route, artifact, retrieval, citation, provenance, consent,
  review, and rollback facts in typed tables;
- keeps blob bytes and derived indexes outside Dolt while recording their
  content-addressed manifests;
- preserves existing VText/private computer APIs;
- adds focused unit/integration tests for the platform store/service;
- proves the product path on staging and inspects platform Dolt state;
- documents residual risks and next realism axes.

Substandard work:

- copying private VText rows wholesale into a platform DB;
- publishing mutable document heads rather than selected immutable versions;
- claiming "citation support" because the rendered page has links;
- storing embeddings or media blobs as canonical Dolt rows;
- using platform Dolt as a message bus;
- writing browser-public internal/test-only routes for acceptance;
- building CHIPS/ranking before public edge quality is queryable.

## Homotopy Parameters

Increase realism continuously along these axes:

- one selected VText revision -> selected range/edition -> redacted projection;
- one author approval -> owner plus reviewer/platform approval;
- one public route -> handle/slug plus redirect/retraction/supersession;
- one text artifact -> rendered HTML/PDF/media artifact manifests;
- one retrieval span -> many chunk/span selectors with index manifests;
- one citation candidate -> verified/disputed/retracted citation lifecycle;
- single platform Dolt primary -> primary plus standby replication -> read
  replicas/backups/remotes;
- local schema proof -> runtime integration -> deployed staging proof;
- direct platform store inspection -> product API/public reader proof plus Dolt
  commit/branch evidence;
- no external sources -> external canonical/via refs and retrieval timestamps;
- export-level provenance -> publication-level acceptance record.

At low resolution, the platform service may use one Dolt database with staged
rows on `main`. At higher resolution, proposals may use Dolt branches and merges
when branch isolation creates real review value.

## Belief State

Current belief:

- per-user embedded Dolt is now the correct private ledger;
- platform/public state needs a separate Dolt SQL-server service/database, not
  embedded Dolt inside `platformd`;
- Dolt supports direct-to-standby replication and remote-based read replication,
  but does not provide a documented multi-primary OLTP cluster; Choir should
  design for a single write primary with product-level proposal/merge semantics;
- publication is the first platform-Dolt forcing function;
- retrieval and citation identity must be designed with publication because
  public artifacts become the retrieval corpus;
- a narrow VText publication vertical slice is the safest first implementation;
- CHIPS and citation scoring should be deferred until the graph records quality,
  provenance, and review signals.

Evidence:

- [mission-embedded-dolt-runtime-migration-v0.md](mission-embedded-dolt-runtime-migration-v0.md)
- [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md)
- [publication-path-skeleton-2026-05-12.md](publication-path-skeleton-2026-05-12.md)
- [platform-dolt-publication-retrieval-citation-research-2026-05-16.md](platform-dolt-publication-retrieval-citation-research-2026-05-16.md)
- Dolt version-control and branch docs;
- W3C PROV, Web Annotation, DataCite, IPFS, OCI, C2PA, W3C VC, SLSA, and
  Self-RAG references summarized in the research report.
- Dolt server replication/configuration/backups docs summarized in the research
  report.

Main uncertainties:

- whether v0 configures direct-to-standby replication immediately or starts with
  primary-only plus backups and a clear HA follow-up;
- how much account/identity metadata should be mirrored into platform Dolt in
  v0;
- how to compute stable VText source revision/projection hashes from the
  current VText schema;
- the minimal selector model for source spans in VText content;
- the first verifier policy for a citation edge;
- where to store public blobs/artifacts on staging for the first slice.

Highest-impact uncertainty:

Can a selected private VText revision be projected into platform Dolt as an
immutable public artifact with enough provenance and retrieval/citation identity
to support future search/radio/economics, without leaking private state or
overbuilding the public product?

Next observation:

Inventory current auth/proxy/runtime routes and Node B service topology to add
a platform `dolt sql-server` plus `platformd` service with the smallest
operationally real config: data dir, config dir, users/grants, metrics, backup
refs, and optional standby.

## Receding-Horizon Control

Work in short Codex control intervals.

At each interval:

1. name the boundary being changed;
2. predict the observable evidence;
3. make the smallest coherent change;
4. run focused tests;
5. update belief state if observations surprise the mission;
6. continue, narrow, branch, rollback, or stop.

Initial mutation radius:

- add platform store/service scaffolding and tests;
- add a separate platform Dolt SQL-server service boundary, even if v0 starts
  with one primary only;
- do not alter the existing private VText editing path except to add a publish
  proposal endpoint or client call;
- do not alter auth/session semantics beyond required service authorization;
- do not alter vmctl/computer lifecycle;
- do not implement CHIPS/ranking/paywalls.

Widen scope only after the selected-revision publication path is proven.

## Dense Feedback Channels

Use feedback that reveals local error:

- schema tests for platform Dolt migrations;
- service tests for publication proposal, publish, route lookup, retrieval span,
  and citation edge operations;
- privacy tests that private prompt/path/unpublished content does not enter
  platform rows or rendered public output;
- hash tests for source revision, projection, content, and artifact manifests;
- product API tests for selected VText publication;
- public route browser/Playwright proof;
- platform Dolt inspection showing expected rows and commit identity;
- staging health commit identity;
- RunAcceptanceRecord synthesis at publication-level if enough evidence exists.

## Evidence Ledger

Every final claim must name evidence:

```text
claim
evidence source
command or observation
artifact path
result
uncertainty/caveat
promotion relevance
```

Required final evidence for behavior-changing completion:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- deployed acceptance command and result;
- published private VText source doc/revision id;
- platform publication/proposal/version ids;
- public route URL;
- platform Dolt rows for publication, artifact manifest, route, retrieval span,
  citation candidate/edge, consent/review, and rollback/supersession;
- proof that the private VText can still change without changing the public
  version;
- proof that private-only refs did not leak into public rows/output;
- rollback target and rollback method;
- residual risks and next realism axis.

## Forbidden Shortcuts

- Do not satisfy the mission by docs only.
- Do not publish a mutable private document head.
- Do not dump the whole user embedded Dolt workspace into platform Dolt.
- Do not make platform Dolt the live actor mailbox or worker message bus.
- Do not store raw large media, embeddings, or vector index blobs as canonical
  Dolt rows.
- Do not use browser-public internal or test-only routes as acceptance proof.
- Do not rely on local-only proof for platform publication behavior.
- Do not implement CHIPS, public citation scoring, paywalls, or marketplace
  mechanics in this mission.
- Do not edit tracked files directly on Node B.
- Do not hide schema uncertainty behind a generic `metadata_json` for query-
  critical citation/provenance/route facts.

## Rollback Policy

Git rollback:

- every behavior-changing patch is committed cleanly;
- previous deployed SHA remains the immediate platform rollback target;
- if staging regresses, revert/redeploy through normal CI/CD.

Platform state rollback:

- publication rollback hides, retracts, redirects, or supersedes platform rows;
- do not rewrite private source revision history;
- preserve publication/proposal evidence unless legal/privacy policy requires
  erasure;
- keep platform Dolt branch/commit refs sufficient to inspect and reverse the
  published route.

Operational rollback:

- if platform service deployment fails, disable the new route/API while leaving
  private user computers untouched;
- if public blob storage fails, mark proposal blocked and preserve diagnostics;
- if citation verification fails, keep candidate/disputed state instead of
  promoting the edge.

Diagnostic stop:

- before stopping unsuccessfully, use cognitive transforms to examine the
  blocker from privacy/security, information-theoretic, mechanism-design, and
  product-operation perspectives;
- stop only when the blocker is precise enough to define the next safe probe.

Successful stop:

- after first correctness, do one quality pass for schema clarity, duplicate
  path removal, tests, docs, and residual-risk clarity;
- before stopping successfully, try to increase quality along the strongest
  next realism axis that does not expand the mission identity.

## Learning Side-Channel

Tactical learning:

- update implementation notes, tests, and final report.

Target-level learning:

- update this mission doc if service topology or schema boundaries change, such
  as primary-only first versus primary-plus-standby first.

Invariant-level learning:

- stop and escalate before changing private/public ledger separation, consent
  semantics, publication immutability, platform Dolt as ledger-not-network, or
  citation as typed version/span edge.

Project artifacts that should receive learnings:

- this mission document;
- [platform-dolt-publication-retrieval-citation-research-2026-05-16.md](platform-dolt-publication-retrieval-citation-research-2026-05-16.md);
- [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md);
- [current-architecture.md](current-architecture.md);
- [runtime-invariants.md](runtime-invariants.md);
- [publication-path-skeleton-2026-05-12.md](publication-path-skeleton-2026-05-12.md).

## Stopping Condition

The mission is complete only when:

- a platform Dolt-backed service exists on staging;
- platform Dolt is running as a separate SQL-server service, not embedded inside
  the application process;
- one selected private VText revision can be proposed and published into
  immutable platform-visible rows;
- public route rendering serves the immutable published version;
- platform rows include artifact manifest/blob refs, provenance, consent/review,
  retrieval source/span manifest, and at least one citation candidate or accepted
  edge;
- the source private VText can receive a later private revision without changing
  the published version;
- private-only refs/content do not appear in public output or platform-visible
  rows;
- local tests cover platform store/service boundaries;
- behavior-changing changes are committed and pushed to `origin/main`;
- CI and staging deploy are green for the pushed SHA;
- `draft.choir-ip.com/health` reports the deployed SHA;
- deployed product/API or Playwright acceptance passes through public product
  routes, not internal/test-only routes;
- read-only Node B/platform Dolt inspection confirms the ledger shape;
- rollback refs and residual risks are named.

If full public route rendering plus citation edge is too large in one run, a
partial result is acceptable only if it publishes an immutable selected revision
into platform Dolt without private leakage, records consent/provenance/rollback,
and leaves retrieval/citation as the sharply defined next realism axis.

## Goal String

```text
Run docs/mission-platform-dolt-publication-retrieval-citation-v0.md as a Codex-operated MissionGradient mission: build the first platform Dolt SQL-server service plus publication/retrieval/citation substrate so one selected private VText revision can be published into immutable platform-visible records, rendered through a public route, indexed as retrieval source/span manifests, and linked to at least one citation candidate or accepted citation edge, while preserving private-computer boundaries, single-primary platform write discipline, consent, rollback, staging proof, and no CHIPS/ranking/paywall scope.
```
