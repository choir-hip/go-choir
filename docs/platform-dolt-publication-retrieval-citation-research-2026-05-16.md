# Platform Dolt, Publication, Retrieval, And Citation Research

Date: 2026-05-16
Status: research/design input for the next MissionGradient
Operator: Codex

## Executive Summary

Choir is ready to design the platform Dolt layer because the per-user embedded
Dolt layer now owns private user-computer runtime/control and VText state. The
next substrate should not start as "publish a page." It should start as a
platform-visible ledger that records selected immutable private refs, public
artifact manifests, citation/provenance edges, retrieval eligibility, consent,
review, and rollback.

The central design constraint:

```text
Private user computers own mutable private truth.
Platform Dolt owns selected public/platform-visible facts.
Publication is a consented projection from the first ledger into the second.
Retrieval and citations operate over exact published refs, not mutable titles.
```

The first implementation slice should therefore be a narrow but real platform
publication service:

- a Dolt SQL service/database for platform-visible records;
- a service API that mediates all writes;
- selected VText revision publication into immutable public artifact/version
  records;
- content-addressed artifact blobs plus Dolt metadata;
- retrieval-manifest rows for chunk/span identity, not vector blobs as
  canonical state;
- citation candidate and accepted citation edge tables anchored to exact
  published version/span refs;
- consent/review/provenance events sufficient to support later promotion,
  search, radio, and CHIPS/citation economics.

Do not implement CHIPS, public citation scoring, paywalls, or open-ended
community governance in this mission. Preserve the state shapes that make those
later systems possible.

## Current Choir Ground Truth

Completed substrate:

- Per-user embedded Dolt now stores private VText plus runtime/control product
  state in one workspace per user computer.
- The accepted staging run proved VText, Trace, desktop state, worker
  delegation, and run acceptances through product APIs. Later cleanup replaced
  the old patchset promotion-candidate path with AppChangePackage/adoption.
- Node B disk inspection showed fresh active and worker computers with zero-byte
  `/state` markers and Dolt state under `/state.vtext/vtext/.dolt`; no
  `/state-wal` or `/state-shm` runtime SQLite pair existed for those computers.
- Host auth/session and vmctl/routing state are not part of the per-user
  embedded Dolt ledger.

Implication:

The next semantic split is not inside the user computer. It is between private
computer state and shared platform/public state. Platform Dolt should now own
the shared ledger for publication, public route identity, citation graph,
retrieval-visible artifacts, verifier/promoter records, and later compute
accounting.

## Cognitive Transform Pass

Current uncertainty or obstacle:

The naive platform-Dolt mission could harden a schema too early by copying
private VText rows into a public database and calling that "publication." That
would miss consent, exact version identity, redaction/projection, retrieval
reuse, citation economy, and later promotion governance.

Selected transforms:

1. Depth extraction: publication is not rendering; it is a trust-domain
   transition with durable identity and rollback semantics.
2. Information-theoretic lens: a citation is valuable when it reduces future
   uncertainty about a claim, artifact, or source path. Citation count alone is
   a lossy and gameable proxy.
3. Security/adversarial lens: every useful public edge becomes an attack
   surface for privacy leakage, reputation manipulation, spam, and laundering
   unsupported claims through citation-shaped decoration.
4. Mechanism-design lens: the platform should record consent, provenance,
   verification, and reuse before it prices or ranks anything.
5. Audience translation: the author UI should feel like "publish this version"
   and "show what supports this claim," while the backend records exact
   immutable refs, spans, manifests, and attestation events.

Route-changing insights:

- The first artifact should be a publication ledger and exact-ref graph, not a
  public reader alone.
- Retrieval rows should point to source version/span/chunk identity; embeddings
  and search indexes are derived caches that can be rebuilt.
- Citation edges should have lifecycle states: candidate, asserted, verified,
  disputed, retracted, superseded. This prevents the first citation graph from
  becoming permanent spam.
- Redaction must create a new public projection hash. It must not publish a
  private revision id and hope access control hides the omitted parts.
- Platform Dolt can use branches for proposals, but the service API must own
  branch/session discipline so clients do not accidentally mutate the wrong
  branch.

Changed plan:

- Implementation: start with platform publication service and schema; do not
  start with CHIPS, ranking, or a broad public search UI.
- Verifier/evidence: prove selected private revision -> immutable public
  version -> retrieval manifest -> citation candidate -> accepted edge.
- Scope: one VText revision and one citation candidate path; no app package or
  source/build promotion merge yet.
- Stopping condition: deployed staging can publish one private VText revision to
  platform Dolt, render it publicly, inspect platform Dolt rows, and verify no
  private-only refs leaked.

Next high-information action:

Write a MissionGradient that builds the platform Dolt service and first
publication/citation vertical slice, with schema exploration before hardening.

## External References And Lessons

### Dolt Service Topology

Dolt is a SQL database with Git-like version control. Its SQL server exposes
branches, commit graph, diffs, merges, history queries, system tables, and
stored procedures. This supports the platform ledger because publication and
promotion proposals are naturally branch/diff/merge-shaped rather than just
append-only rows.

Deployment decision, updated 2026-05-16:

The platform layer should use a separate `dolt sql-server` deployment, not an
embedded Dolt workspace owned by the application process. The per-user runtime
uses embedded Dolt because each user computer owns a private local ledger. The
platform ledger is different: it should grow into a long-lived service with
separate operations, monitoring, backups, replicas, branch permissions, and
eventual failover.

Dolt has native server replication, but it is not a multi-primary OLTP cluster.
The useful end-state is:

```text
platformd/API services
  -> MySQL protocol
  -> platform Dolt SQL primary
  -> direct-to-standby Dolt replication for HA
  -> read replicas / remotes / backups for scale and disaster recovery
```

The write topology should remain single-primary. Choir's platform service owns
write admission, proposal state, verifier contracts, branch/commit discipline,
and conflict semantics. Dolt branches help isolate and review platform state,
but they do not replace a product-level consent and conflict policy.

Sources:

- Dolt version-control features:
  https://docs.dolthub.com/sql-reference/version-control
- Dolt branches:
  https://docs.dolthub.com/sql-reference/version-control/branches
- Dolt remotes:
  https://docs.dolthub.com/concepts/dolt/git/remotes
- Dolt server replication and direct-to-standby cluster replication:
  https://docs.dolthub.com/sql-reference/server/replication
- Dolt SQL server configuration, including metrics, remotesapi, and cluster
  config: https://docs.dolthub.com/sql-reference/server/configuration
- Dolt SQL server backups:
  https://docs.dolthub.com/sql-reference/server/backups

Design consequences for Choir:

- Use platform Dolt as a service database, not as a browser-accessible database.
- Writes should go through a platform service that chooses branch, transaction,
  commit author, verifier metadata, and rollback refs.
- Do not plan around multi-primary writes. If the platform eventually shards or
  federates, shard by product boundary or use explicit merge workflows rather
  than pretending Dolt is an automatic conflict-free multi-writer OLTP cluster.
- Use direct-to-standby replication for high availability once the service has a
  real write path. Dolt's docs describe primary and standby roles, role epochs,
  controlled failover via `dolt_assume_cluster_role`, and automatic protection
  against split-brain primary misconfiguration.
- Treat Dolt users/grants, branch-control metadata, config files, remotes, and
  backups as part of the platform service's operational surface, not incidental
  local files.
- Branches are useful for proposal isolation, but Dolt branch-head metadata has
  different rollback semantics than table data. Preserve explicit proposal rows
  and rollback refs; do not rely on branch names as the only record.
- Use revision-qualified reads and commit hashes for public artifacts. Mutable
  names such as slugs and handles should route to immutable versions.
- Use Dolt remotes for backup/replication and later platform-to-platform
  sync, not as a substitute for product-level consent and publication policy.

### Provenance Model

W3C PROV defines provenance as information about entities, activities, and
people involved in producing a thing, used to assess quality, reliability, or
trustworthiness. That maps directly onto Choir's model: VText revisions,
sources, agents, worker runs, verifier contracts, publication events, and
citations are not just metadata; they are the substrate of future trust.

Source:

- W3C PROV overview: https://www.w3.org/TR/prov-overview/

Design consequences for Choir:

- Keep normalized provenance primitives even if public APIs expose friendlier
  names:
  - entity: VText revision, artifact blob, claim, source span, source delta,
    generated media, public version;
  - activity: edit, retrieve, synthesize, verify, publish, retract, supersede,
    cite, transclude;
  - agent: human user, appagent, researcher, super, worker computer, verifier,
    platform service.
- Public artifacts should carry enough provenance to answer "who/what produced
  this, from what, under which verifier and consent?"
- Do not collapse provenance into a single JSON column only. Keep JSON for
  flexible details, but make the query-critical edges typed.

### Web Annotation And Exact Span Identity

W3C Web Annotation is useful because citations often target a part of a resource
rather than the whole artifact. It also distinguishes canonical identity from
the location where a copy was obtained.

Source:

- W3C Web Annotation Data Model: https://www.w3.org/TR/annotation-model/

Design consequences for Choir:

- Citation edges should support selectors/spans, not only document-level refs.
- Store both canonical identity and "via" retrieval/discovery location when
  importing external sources.
- A citation target should resolve to:
  - artifact/version id;
  - selector type;
  - selector payload;
  - content hash or source revision hash;
  - observed/retrieved timestamp;
  - optional external canonical/via IRIs.

### DataCite And Scholarly Relation Types

DataCite's metadata schema exists for citation and retrieval. Its
`relatedIdentifier` relation types distinguish citations, references,
supplements, versions, and other relationships. Choir should not copy the whole
scholarly stack, but it should learn from relation-typed citation edges.

Sources:

- DataCite Metadata Schema: https://schema.datacite.org/
- DataCite citation/reference guidance:
  https://support.datacite.org/docs/contributing-citations-and-references

Design consequences for Choir:

- A public edge is not always "cites." Needed initial relation types:
  - `cites`;
  - `references`;
  - `supports_claim`;
  - `uses_source`;
  - `transcludes`;
  - `derived_from`;
  - `summarizes`;
  - `contradicts`;
  - `corrects`;
  - `supersedes`;
  - `requires`;
  - `is_version_of`;
  - `is_redaction_of`.
- Keep a mapping layer to DataCite/Crossref-style relation types where useful,
  but preserve Choir-specific semantics for agentic synthesis and candidate
  worlds.

### Content Addressing And Artifact Manifests

IPFS CIDs demonstrate a useful invariant: content identity should derive from
content, not location. OCI image manifests show a pragmatic artifact-manifest
shape: media types, descriptors, digests, layers, annotations, and optional
subject relationships.

Sources:

- IPFS content addressing: https://docs.ipfs.tech/concepts/content-addressing/
- OCI image manifest spec:
  https://github.com/opencontainers/image-spec/blob/main/manifest.md

Design consequences for Choir:

- Store large bytes in content/blob storage, not Dolt rows.
- Platform Dolt rows should point to artifacts by digest, media type, byte size,
  storage ref, and manifest id.
- A public VText version can be an artifact manifest:
  - content text hash;
  - rendered HTML/PDF/media hashes;
  - source revision hash;
  - citation manifest hash;
  - provenance event ids;
  - license/visibility.
- This structure lets retrieval and radio consume stable artifacts without
  binding to mutable filesystem paths.

### C2PA, Verifiable Credentials, And Attestations

C2PA's content provenance model emphasizes manifests, assertions, signatures,
ingredients, and hard content bindings. W3C Verifiable Credentials provide a
general model for machine-verifiable claims from an issuer about a subject.
SLSA focuses on verifying an artifact against provenance and configured roots
of trust.

Sources:

- C2PA technical specification:
  https://spec.c2pa.org/specifications/specifications/1.0/specs/C2PA_Specification.html
- W3C Verifiable Credentials Data Model:
  https://www.w3.org/TR/vc-data-model/
- SLSA verifying artifacts:
  https://slsa.dev/spec/v1.1/verifying-artifacts

Design consequences for Choir:

- Publication and promotion should produce verifier-readable attestations.
- The first version can use internal signed/hashed records rather than adopting
  full C2PA/VC/SLSA envelopes immediately.
- Keep fields compatible with later envelopes:
  - issuer/verifier identity;
  - subject artifact digest;
  - claim type;
  - evidence refs;
  - verification method/root of trust;
  - validity/revocation/supersession state.
- Source/build package adoption should use SLSA-like provenance concepts.
  VText publication should use a lighter content/provenance manifest with
  content binding and author consent.

### GitHub Pull Requests And Review UX

GitHub pull requests are proposals, reviews, comments, approvals, request
changes, required reviewers, and status checks around a branch diff. Choir
should borrow this consent/review shape for source/build promotion, but VText
publication and citation edges are not merely code diffs.

Source:

- GitHub PR reviews:
  https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews

Design consequences for Choir:

- Promotions need typed review, not one global "approve" button.
- VText publication review asks: owner consent, public projection, citations,
  private refs, license, route.
- Source/build review asks: patch diff, tests, build identity, verifier
  contracts, rollback.
- Citation review asks: does this edge accurately relate this claim/span to
  this source/span?

### Retrieval And Citation Accuracy

RAG reduces some hallucination risk by bringing retrieved knowledge into
generation, but retrieval itself is not enough. Self-RAG research highlights
adaptive retrieval and reflection over retrieved passages, including gains in
factuality and citation accuracy for long-form generation.

Source:

- Self-RAG paper: https://arxiv.org/abs/2310.11511

Design consequences for Choir:

- Retrieval should produce candidate evidence with IDs, versions, spans, and
  freshness metadata before generation.
- A generated/published output should select citation edges from a constrained
  registry of retrieval candidates, not invent citations after the text exists.
- Store retrieval manifests for durable published outputs:
  - query/objective;
  - retrieval index version;
  - selected source refs;
  - rejected/conflicting refs where relevant;
  - generated claim ids;
  - citation edges used.
- Do not make every ephemeral search a permanent public citation. Public edges
  should require publication or explicit assertion.

## Proposed Platform Dolt Boundary

Platform Dolt should own platform-visible facts:

- account and public identity metadata where platform-visible;
- public handles, routes, slugs, redirects, custom-domain bindings;
- platform computer lifecycle/capacity/routing records where durable;
- publication proposals and publication records;
- public artifact metadata and manifests;
- citation candidates and accepted citation graph edges;
- retrieval manifests and index-source manifests;
- public verifier, review, promotion, and rollback records;
- compute/model attribution records needed for later accounting;
- later CHIPS-compatible state.

Platform Dolt should not own:

- private mutable VText drafts;
- private user computer runtime traces unless selected for publication or
  support evidence;
- hot agent-to-agent messaging;
- raw model streams;
- vector index binary storage;
- large blobs;
- CI/deploy execution itself;
- auth credential secrets.

Platform Dolt is the ledger and queryable index. It is not the network, the
message bus, the blob store, or the whole search engine.

## Proposed Service Shape

Working name: `platformd`.

Responsibilities:

- own the connection to platform Dolt SQL server;
- apply migrations and schema version checks;
- expose internal platform APIs for publication, citation, retrieval manifests,
  public routing, and promotion records;
- expose browser-public read APIs only where privacy-safe;
- mediate every write through authz, consent, verifier, and branch/commit
  policy;
- write platform Dolt commits with meaningful authorship and trace refs;
- publish events for public cache/index refresh.

Initial deployment:

- run on Node B staging as a normal platform service;
- run a separate platform `dolt sql-server` process and have `platformd` connect
  over the MySQL protocol;
- use one database initially, with table prefixes/namespaces rather than many
  databases;
- configure metrics, users/grants, branch permissions, and a backed-up
  `config.yaml` from the start;
- after the first staging proof, add backups via point-in-time block snapshots
  where available and/or Dolt backups/remotes;
- after the first real write path is stable, add direct-to-standby replication
  rather than trying to retrofit HA around an embedded process.

Branch policy:

- `main`: accepted platform-visible state;
- `proposal/publication/<publication_id>`: multi-row proposed publication state
  if branch isolation is useful;
- `proposal/citation/<citation_id>`: optional later branch for complex citation
  disputes;
- simple first slice may use `main` plus `state='staged'` rows, as long as the
  accepted public route only reads `state='published'`.

The service, not clients, chooses branches and commits. Browser code should
never connect to Dolt directly.

## Candidate Schema Families

This is deliberately conceptual. The mission should refine and test exact DDL.

### Identity And Route

```text
platform_subjects
  subject_id
  subject_kind         -- user, org, agent, service, publication, artifact
  display_name
  canonical_uri
  created_at
  updated_at

public_handles
  handle
  owner_subject_id
  state                -- reserved, active, suspended, released
  created_at
  updated_at

public_routes
  route_id
  handle
  route_path
  target_kind          -- publication, artifact, collection, profile
  target_id
  target_version_id
  state                -- active, redirected, hidden, retracted
  created_at
  updated_at
```

### Publication

```text
publication_proposals
  proposal_id
  owner_id
  source_computer_id
  source_doc_id
  source_revision_id
  source_revision_hash
  projection_hash
  title
  visibility
  license
  state                -- draft, staged, approved, published, blocked
  created_by
  created_trace_id
  created_at
  updated_at

publications
  publication_id
  owner_id
  handle
  slug
  title
  state                -- published, hidden, retracted, superseded
  latest_version_id
  created_at
  updated_at

publication_versions
  publication_version_id
  publication_id
  proposal_id
  edition_label
  source_doc_id
  source_revision_id
  source_revision_hash
  projection_hash
  content_hash
  artifact_manifest_id
  published_at
  supersedes_version_id
```

### Artifact Manifests

```text
artifact_manifests
  artifact_manifest_id
  subject_kind
  subject_id
  media_type
  manifest_hash
  manifest_json
  created_at

artifact_blobs
  blob_id
  artifact_manifest_id
  content_hash
  hash_algorithm
  media_type
  byte_size
  storage_ref
  created_at
```

### Provenance And Consent

```text
provenance_entities
  entity_id
  entity_kind
  content_hash
  canonical_uri
  metadata_json

provenance_activities
  activity_id
  activity_kind
  trace_id
  run_id
  started_at
  ended_at
  metadata_json

provenance_agents
  agent_ref_id
  agent_kind
  subject_id
  model
  provider
  vm_id
  metadata_json

provenance_edges
  edge_id
  edge_kind            -- used, generated, was_attributed_to, verified_by
  from_id
  to_id
  activity_id
  metadata_json

consent_records
  consent_id
  subject_id
  target_kind
  target_id
  action
  state                -- granted, revoked, expired
  evidence_ref
  created_at
```

### Retrieval

```text
retrieval_sources
  source_id
  source_kind          -- publication_version, artifact, external_url
  canonical_uri
  content_hash
  license
  visibility
  state
  created_at

retrieval_spans
  span_id
  source_id
  source_version_id
  selector_kind        -- text_quote, byte_range, xpath, css, media_time
  selector_json
  text_hash
  chunk_hash
  token_count
  metadata_json

retrieval_index_manifests
  index_manifest_id
  source_scope
  index_kind           -- lexical, vector, hybrid
  index_version
  generated_from_commit
  storage_ref
  created_at

retrieval_manifests
  retrieval_manifest_id
  output_kind
  output_id
  query_or_objective_hash
  index_manifest_id
  selected_refs_json
  rejected_refs_json
  created_at
```

Embeddings and search indexes may live outside Dolt. Dolt stores the exact
source/version/chunk identities and index manifests needed to rebuild or audit
them.

### Claims And Citations

```text
claims
  claim_id
  publication_version_id
  claim_text_hash
  claim_text
  selector_json
  state                -- draft, asserted, verified, disputed, retracted
  created_by
  created_at

citation_edges
  citation_id
  from_kind            -- claim, publication_version, span, artifact
  from_id
  from_selector_json
  to_kind              -- publication_version, span, artifact, external_ref
  to_id
  to_selector_json
  relation_type
  state                -- candidate, asserted, verified, disputed, retracted
  proposed_by
  accepted_by
  evidence_ref
  confidence
  created_at
  updated_at
```

### Verification And Review

```text
review_records
  review_id
  target_kind
  target_id
  reviewer_subject_id
  decision             -- comment, approve, request_changes, reject
  body
  created_at

verifier_attestations
  attestation_id
  target_kind
  target_id
  verifier_id
  verifier_kind
  result               -- passed, failed, inconclusive
  subject_digest
  predicate_type
  evidence_json
  created_at

rollback_refs
  rollback_id
  target_kind
  target_id
  rollback_kind
  ref
  expires_at
  created_at
```

## Publication Flow

First vertical slice:

```text
private VText revision
  -> user selects publish
  -> sandbox computes source revision hash, projection hash, citations/artifacts
  -> platform service validates owner/session/source revision
  -> optional redaction creates a public projection hash
  -> platform stores artifact manifest and content blob refs
  -> platform creates publication proposal
  -> user approval records consent
  -> platform creates immutable publication version
  -> route points to the published version
  -> retrieval source/span rows become eligible
  -> citation candidates are stored but not automatically trusted
```

Acceptance should prove:

- the user's private document can continue changing after publication;
- the public version remains stable;
- the public route resolves by handle/slug to the immutable version;
- platform Dolt has proposal, publication, artifact, provenance, consent, and
  route rows;
- private-only references are absent from platform rows and rendered output.

## Retrieval And Citation Flow

First vertical slice:

```text
published version
  -> chunk/span extraction
  -> retrieval source/span rows
  -> optional external search/vector index manifest
  -> VText synthesis retrieves exact spans
  -> output creates claim ids
  -> citation candidates link claims to spans
  -> verifier checks cited text supports claim
  -> accepted citation edge becomes public
```

Important distinction:

- retrieval candidate: evidence found;
- citation candidate: evidence selected for a generated claim or artifact;
- asserted citation: author/appagent says the relation is true;
- verified citation: verifier says the relation satisfies a policy;
- public citation edge: platform accepts it into the visible graph.

The first implementation may stop at citation candidates and one manual/agent
accepted citation. Do not build ranking before edge quality is queryable.

## Security And Privacy Invariants

- No private user-computer Dolt branch or revision becomes public by reference
  alone. Publication copies selected immutable content/projection refs.
- Redaction creates a separate public artifact hash and provenance relation to
  the private source; it does not expose private source content.
- Browser-public APIs cannot write platform Dolt directly.
- Every platform write has actor, authority, consent/review state, source trace,
  and rollback/supersession semantics where applicable.
- Search indexes are derived caches. A compromised index must not become a
  canonical citation source.
- Citation edges must be stateful and revocable/supersedable.
- Public retrieval must filter by visibility/license/route state before ranking.
- External sources need canonical/via identifiers and retrieval timestamps.
- Claim/citation verification must be allowed to fail closed: unsupported
  claims can remain uncited or blocked rather than inventing a citation.

## Mechanism Design Notes For The Future Citation Economy

Do not price or rank citations yet, but store enough to avoid repainting later.

Future value signals should distinguish:

- attention: a source was retrieved or viewed;
- use: a source influenced an output;
- citation: a typed relation was asserted;
- support: a verifier found the source supports the claim;
- transformation: the output derived from, summarized, contradicted, corrected,
  or extended the source;
- trust: a reviewer or verifier with known authority accepted the edge;
- reuse: later artifacts depended on the edge or source.

Goodhart risks:

- citation-count farming;
- spam publication to create retrieval-visible surface area;
- laundering unsupported claims through plausible source lists;
- reciprocal citation rings;
- stale but highly cited material overpowering fresher corrections;
- model-generated citations that look academic but do not map to source spans.

Early countermeasures:

- exact source/span identity required for public edges;
- edge lifecycle and verifier state;
- relation types richer than `cites`;
- conflict/correction edges as first-class records;
- source freshness and version metadata in retrieval rows;
- author/platform review visible in the graph;
- no economic scoring until the graph has enough quality signals.

## Recommended Mission Sequence

1. Platform Dolt service and schema design vertical slice.
2. Selected VText revision publication into immutable platform records.
3. Public route/reader for one published version.
4. Retrieval source/span manifest generation for published versions.
5. Citation candidate and accepted citation edge path.
6. Promotion consent layer improvements for source/build packages.
7. Broader publication UX: redaction, editions, retraction, supersession.
8. Search/radio over published retrieval graph.
9. Citation quality scoring and later CHIPS-compatible economics.

This sequence deliberately puts platform Dolt before publication UX polish, and
publication/retrieval/citation identity before economic scoring.

## Open Questions For The Mission

- How much of direct-to-standby replication should v0 configure immediately:
  primary-only plus backups, or primary plus one standby from the start?
- How much of auth/account metadata belongs in platform Dolt now versus mirrored
  from auth SQLite as public subject records?
- Does the first publish action require an explicit preview approval step, or is
  "publish selected revision" enough for staging?
- Should citation candidates be generated during publication, during later
  retrieval/synthesis, or both?
- What is the minimal source-span selector set for VText v0: byte range,
  text quote, line range, or paragraph id?
- Should public route state live in platform Dolt immediately, or remain proxy
  config until publication records are proven?
- How do we represent private-source provenance in public rows without leaking
  private document titles, prompts, trace details, or source paths?
- What is the first verifier policy for a citation edge: exact quote contains,
  semantic support, contradiction detection, or manual approval only?

## Recommended Next Mission

Author and run a MissionGradient for:

```text
Build the first platform Dolt service and publication/retrieval/citation
substrate: publish one selected private VText revision into immutable
platform-visible Dolt records, render it through a public route, generate
retrieval source/span manifests, and record at least one citation candidate or
accepted citation edge without leaking private state or implementing CHIPS.
```

The mission should be staging-first for behavior changes. Local work is
acceptable for schema and service iteration, but acceptance requires commit,
push, CI, staging deploy, `/health` identity, product-path publication proof,
platform Dolt inspection, and rollback refs.
