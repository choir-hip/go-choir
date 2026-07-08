# Universal Wire — Autonomous Publish Security Review (Slice 3b)

Date: 2026-06-10  
Status: **Pre-implementation gate** for platform-internal publish  
Blocks: Workstream (e) staging proofs that claim live public newspaper

Related:

- [universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md)
- [mission-wire-community-news-v1.md](mission-wire-community-news-v1.md) (Slice 3b, evidence matrix)
- `internal/proxy/platform_publish.go` (owner JWT path, today)
- `internal/platform/handlers.go` (`/internal/platform/publications/vtext`)
- `internal/runtime/wire_publication.go` (edition transclusion only — **not** corpusd)

---

## Scope

This review covers **autonomous Wire publish**: after an eligible canonical
`edit_vtext` on the `universal-wire-platform` computer, the runtime must project
a **public corpusd publication** without a human operator gate (Community
Cloud policy). It does **not** re-litigate the owner JWT proxy publish path
except where shared corpusd behavior matters.

---

## Threat model

| Threat | Impact | Likelihood without controls |
|--------|--------|------------------------------|
| **T1. Non-article revision published** (input/brief/seed, empty, placeholder) | Public junk or internal handoff text on choir.news | Medium — LLM + handoff churn |
| **T2. Wrong owner published** (user doc, wrong platform doc) | Cross-tenant or wrong-corpus leak | Low–medium if publish API trusts body fields |
| **T3. Model-prior article without ingestion provenance** | Untraceable “news” violating mission contract | Medium — VText can draft from priors |
| **T4. Unsafe source entity projection** (private content_item snapshots in public bundle) | Reader-visible leak of non-public source text | Medium — metadata carries `source_entities` |
| **T5. Policy override via revision metadata** (`access_policy` / `route_policy` in metadata) | Intended public wire article published with wrong visibility | Low today (defaults public); autonomous must not widen |
| **T6. Spoofed internal publish caller** | Arbitrary `owner_id` + content → corpusd | Low on Node B if corpusd stays localhost-only; **high** if port exposed or header-only auth |
| **T7. Edition index without platform record** (current code) | Wire UI shows stories that are not publicly published | **Current** — edition transclusion alone |
| **T8. Prompt-bar / user path triggers autonomous publish** | User drafts become wire front page | Low if policy is owner + run-intent gated |
| **T9. Replay / duplicate publish storms** | Retrieval spam, reconciler noise, storage cost | Medium — publish-then-correct is allowed |
| **T10. Compromised VText run publishes harmful content** | Public misinformation (policy choice: publish-then-correct) | Inherent product risk — mitigate with provenance + correction loop |

**Security goal:** autonomous publish is a **narrow, programmatic capability**
invoked only from the runtime publication-policy hook after re-reading Dolt state.
It is **not** a new agent tool and **not** a browser-public API.

---

## Current controls (what we already have)

### User owner publish (proxy → corpusd)

- JWT authentication (`validateAccessJWT`).
- Re-reads doc + revision from **user sandbox**; enforces `owner_id` match.
- Strips `X-Internal-Caller` from client requests before sandbox forward
  (`internal/proxy/handlers.go`).
- Optional explicit `access_policy` / `export_policy` on request; forwards to
  corpusd.
- Source metadata enrichment with publication-safety gates
  (`enrichVTextPublicationMetadata`, `sourcecontract` reader snapshot states).

### corpusd internal API

- Routes under `/internal/platform/*` require `X-Internal-Caller: true`.
- `PublishVText` builds immutable publication ledger (consent/review/attestation
  rows, retrieval spans, rollback ref).
- Default access policy is **public** unless overridden
  (`defaultPublicationAccessPolicy`).
- corpusd binds **127.0.0.1** by default (`internal/server/server.go`).

### Wire publication policy (runtime, partial)

`wireCanonicalRevisionEligibleForPublication` requires:

- Run owner = `universal-wire-platform`
- Wire article revision run (`universal_wire_*_article_revision`)
- `source: edit_vtext`, canonical role, ingestion cycle provenance
- Non-empty content; not seed/brief heuristics

**Gap:** eligible publish today only updates **edition VText** in embedded Dolt;
it does **not** call corpusd.

### Agent capability enforcement

Processor/reconciler registries **exclude** `edit_vtext` (`TestInstallDefaultAgentToolsProfiles`).
Prompts should describe **delegation**, not forbidden tools (registry is the contract).

---

## Required security architecture for Slice 3b

### 1. Single choke point (no tool, no public route)

```text
edit_vtext success (platform wire article run)
  -> publishPolicy.EvaluateAndPublish(ctx, docID, revisionID, runID)
       -> reload doc + revision from Dolt (do not trust tool args alone)
       -> eligibility guards (existing + tests)
       -> build platform.PublishVTextRequest (server-side only)
       -> call corpusd (host-local trusted caller)
       -> on success: edition transclusion + debounced reconciler event
```

**Invariant:** nothing browser-public may call corpusd publish for Wire.

### 2. Trusted caller to corpusd

`X-Internal-Caller: true` alone is **not** a security boundary if any host
process or misbound listener can reach corpusd.

**Minimum for v1 (Node B):**

- corpusd remains **127.0.0.1** only (already default).
- Only **proxy** or a **dedicated host-side publisher** may call
  `/internal/platform/publications/vtext`.
- Platform VM sandbox runtime must **not** call corpusd directly with a
  spoofable header; it should call a **host-mediated** path (e.g. proxy internal
  route or sidecar) that checks publication policy **before** forwarding.

**Stretch (document, not blocking v1 if localhost holds):**

- HMAC/mTLS internal credential per caller class (`owner_proxy` vs `wire_policy`).
- corpusd rejects publish unless `publication_kind` + caller credential match.

### 3. Server-side request shaping (autonomous path)

The autonomous builder must **set**, not inherit blindly:

| Field | Rule |
|-------|------|
| `owner_id` | Always `universal-wire-platform` (env default) |
| `requested_by` | Fixed service principal, e.g. `wire_publication_policy` — **not** model-chosen |
| `source_doc_id` / `source_revision_id` | From re-loaded Dolt revision |
| `title` / `content` / `citations` / `metadata` | From re-loaded revision |
| `access_policy` | **Force** public wire policy server-side; **ignore** revision `access_policy` / `route_policy` overrides |
| `export_policy` | Explicit wire defaults (or inherit platform defaults consciously) |
| `source_trace_id` | Wire run id + revision id for audit |

**Provenance metadata** must include at minimum:

```json
{
  "publication_kind": "universal_wire_autonomous",
  "revision_role": "canonical",
  "source_network_cycle_id": "...",
  "ingestion_handoff_cycle_id": "...",
  "processor_key": "...",
  "wire_run_id": "...",
  "wire_request_intent": "universal_wire_processor_article_revision"
}
```

corpusd provenance should record `authority: wire_autonomous_publish_v1`, not
`owner_publish_v0`.

### 4. Eligibility guards (authorization logic, not prompts)

Keep and test all of:

- `revision_role: input` → **never** publish
- Non-platform owner → **never** autonomous
- Missing wire provenance (`source_network_cycle_id` / handoff cycle) → **never**
- Seed/brief content heuristics → **never**
- Non-wire `request_intent` runs → **never**

**Additional guards for slice 3b:**

- Require `artifact_kind: article_revision` (or equivalent canonical marker).
- Require at least one **cited** `source_entities` entry **or** explicit
  `source_item_ids` / ingestion lineage (configurable strictness for Phase A).
- Reject if `rights_scope` on any source entity is non-public when snapshot would
  embed reader text (reuse `sourcecontract` publication-safe checks from proxy).

### 5. Source projection safety

Autonomous path must reuse the same enrichment/safety pipeline as owner publish:

- `enrichVTextPublicationMetadata` logic (or shared package) before corpusd.
- No raw `reader_snapshot` for blocked / non-public content items.
- Transclusion display modes respect `sourcecontract` publication rules.

**Do not** duplicate a weaker enrichment path inside runtime.

### 6. Ordering: corpusd before edition index

Today edition transclusion can make a story appear in
`/api/universal-wire/stories` **without** a corpusd record.

**Required order:**

1. corpusd publish succeeds (returns `route_path`, `publication_version_id`)
2. persist publication ref on revision metadata or durable publish ledger
3. edition transclusion
4. debounced reconciler

On corpusd failure: **no** edition transclusion, **no** debouncer fire.

### 7. Idempotency and publish-then-correct

Multiple corpusd publications per doc revision are **allowed** (corrections).
Still:

- Record `source_revision_hash` from corpusd response on success.
- Edition transclusion should be idempotent per `doc_id` (already checks `included`).
- Debouncer should key on publish events, not raw edits.

### 8. Rate and abuse limits (v1 lightweight)

- Per-doc cooldown or “already published this revision hash” short-circuit optional.
- Global rate limit on autonomous publishes per hour (platform computer policy)
  to bound runaway VText loops.

---

## corpusd hardening backlog (shared with owner path)

| Issue | Risk | Recommendation |
|-------|------|----------------|
| `PublishVText` trusts `RequestedBy` in body | Audit spoofing | Validate against caller class server-side on internal route |
| Auto `consent` + `review` = granted/approve | Compliance theater | OK for v1 wire if provenance distinguishes autonomous vs owner |
| Default public access/export | Sensitive owner docs | Wire autonomous **overrides** to public; owner path already accepts explicit policy |
| No authz on `owner_id` in internal API | Cross-owner publish if caller compromised | Add `publication_kind` enum + allowlist of owner_ids per kind |

---

## Negative proofs required for (e)

Staging acceptance **must** include:

1. Canonical platform wire article → corpusd `route_path` resolvable via proxy
2. Input/seed revision → **no** corpusd row, **no** edition line
3. User-owned doc `edit_vtext` on platform VM → **no** autonomous publish
4. Prompt-bar conductor route → **no** ingestion event → **no** wire publish
5. Processor spawn researcher → denied (registry)
6. Reconciler spawn researcher / vtext without `channel_id` → denied
7. Published revision metadata carries ingestion cycle id traceable to fetch ledger
8. Attempt to set `access_policy: private` in revision metadata → autonomous publish still **public** (forced server-side)

Evidence per Phase A row (mission v1):  
`fetch_id → ingestion_event → processor_run → vtext_revision → corpusd_publication_ref`  
plus negative prompt check.

---

## Recommended implementation sequence

1. **This doc** (checkpoint) ✓
2. Extract shared `buildPlatformPublishRequest` + enrichment from proxy into
   `internal/platformpublish` or reuse from proxy with runtime caller
3. Add `publishPolicy.PublishWireArticleIfEligible` calling corpusd via
   **host-trusted** HTTP client (proxy internal route preferred)
4. Reorder `maybeAutonomousPublishWireArticle`: corpusd → edition → debouncer
5. Tests: unit eligibility + integration with fake corpusd + negative proofs
6. Node B: platform VM + corpusd + staging matrix row with publication ref

---

## Residual risks (accepted for Community Cloud v1)

- **Publish-then-correct:** harmful or mistaken articles may go public briefly;
  mitigated by reconciler + correction VText wakes, not pre-publish human gate.
- **LLM quality:** not a traditional infosec boundary; provenance and source
  handles are the verifier.
- **corpusd localhost trust:** any host root process can publish; acceptable on
  single-tenant Node B with hardened deploy, revisit for multi-tenant.

---

## Belief state

Autonomous Wire publish is **not live** until corpusd projection lands with the
controls above. Edition-only transclusion is an internal editorial index, not
publication. **(e) staging proofs should fail** if `corpusd_publication_ref` is
missing from the evidence chain.
