# World Wire / Autopaper Rearchitecture — Analysis for Director Review

**Status:** analysis + open decisions. Authored for the metamission director
agent; not a committed design. Owner framing recorded verbatim from
2026-10-05 session.

## Owner's product definition (ground truth)

- "An automatic newspaper": process global OSINT into an **object graph**;
  articles use the **same Texture architecture** as user documents so the
  autopaper updates articles as new information arrives.
- **Not one autopaper — a platform for others' autopapers.** Tenants integrate
  private data and distribute internally, privately, paywalled, or publicly.
- **The primary autopaper is the basis for white-label autopapers** — "the
  economy of scale that makes it a useful platform."
- Sequencing: "some WW decisions should be deferred — the metamission has
  important prerequisites, and single-guest-mode Choir working is essential.
  Best done *while designing* the platform/autopaper, since it is the true
  application of Choir, the purpose of all this infra."
- The failed first attempt used **processor + reconciler**; whether that is
  the right factorization of responsibilities vs RLM desks is explicitly open.

## Current implementation (what exists)

```text
sourcecycled (host daemon)
  -> fetches 211 sources on cycles -> corpus.fetches/items/ingestion_events
  -> queues processor_requests   (item batch -> structured claims; DEAD: 2,582 dispatch_failed)
  -> queues reconciler_requests  (dedup/corroborate/merge; same dead path)
  -> corpusd serves the corpus store (dolt sql-server, Store B, 99 GB)
       -> publication_* / provenance_* / og_objects (8.5M) / choir.news reads
```

- Processor = "take fetched items, extract structured claims" (LLM-ish;
  `processor_key`, `verticals`, `regions`, `continuity_ref`, `prompt`).
- Reconciler = "dedupe/corroborate/merge claims across items+processors"
  (`scope`, `source_item_ids`, `processor_request_ids`, `prompt`).
- Both are queue-table → runtime-dispatch → 502 loops today. The pipeline's
  *semantic* shape is fetch→extract→reconcile→publish; its *runtime* shape is
  a host daemon writing to one shared unisolated store.

## Why it fails as-is (substrate vs factorization)

The functions are not obviously wrong; the **substrate** is:

1. **Wrong plane.** Ingestion+processing run as host daemons next to platform
   control-plane services. `computer-ontology.md`: "platform-level semantic
   work belongs to a platform computer, user-level semantic work belongs to a
   user computer." WW semantic work is on neither — it's a host process with
   no capsule isolation, no ComputerID, no event tape.
2. **Store entanglement.** One corpus-dolt carries 8.5M WW og_objects AND the
   narrow platform control tables (event-head CAS, lifecycle, route slots).
   WW's write volume + dolt's compaction churn now fate-share with platform
   authority. The 15-day CPU burn lands on the same sql-server that must CAS
   computer event heads.
3. **No tenant model.** One shared corpus can't do private-data integration or
   per-autopaper isolation — the white-label requirement is structurally
   absent, not just unimplemented.
4. **No supervision surface.** Processor/reconciler queues are opaque SQL
   tables; failures surface as 502s in a journal, not as commitments on a
   tape a Texture desk could supervise.
5. **Unbounded state.** No lifecycle on fetches/items/bodies; 99 GB and
   growing with no declared retention (this is what surfaced it under S0).

## Mapping to desk ontology (candidate factorization)

Not a decision — a factorization proposal for review, using the existing desk
names rather than inventing a WW-specific vocabulary:

| WW function | Failed impl | Candidate home | Why |
|---|---|---|---|
| Source fetch + version capture | sourcecycled cycle | **Observation workload on a platform computer** (capsule-bound fetch adapters + scheduler) | Mechanical, deterministic-ish; wants capsule isolation + a real event tape; is shared infra, so *platform* computer not a user computer |
| Item → structured claims | `processor_requests` | **research desk** (read-only world-evidence authority) | Claim extraction + corroboration is evidence work; per-source-version input, claim objects + provenance edges output |
| Dedup/corroborate/merge | `reconciler_requests` | **research desk, same cast family** | Reconcile is claims-work over the same evidence; folding it into one desk keeps the claim/provenance graph coherent; split only if load demands |
| What to cover (attention) | implicit in cycle config | **management desk** (autopaper's own) | Editorial attention policy = coherence decisions for that computer |
| Write/update articles | `publication_*` + prompts | **Texture desk** (canonical doc authority) | Articles are Texture documents; update-on-new-info is exactly the existing revision/commitment machinery + `resolves` precommitment records |
| Publish to distribution modes | host publication tables | **engineering capsule** (mutation) + route/projection for public projection | Public/internal/private/paywalled = projection variants of one canonical doc; public serve path must not fate-share the live computer (already in 5b) |

Open factorization question (owner-flagged): extraction vs reconciliation as
**one research cast or two**. Evidence needed: whether reconcile reads only
claims (then it's a second stage of the same desk) or needs fetch/item access
research shouldn't have. Defer to the WW slice of a station that can inspect
real cast inputs.

## Target architecture sketch (for review)

```text
Platform computer "wire-observer" (cloud-owned, Community Cloud)
  - capsule fetch adapters + source scheduler (replaces sourcecycled)
  - research desk: claims/corroboration over source versions
  - the SHARED public claims graph lives here (the economy-of-scale asset)

User computers "autopaper-N" (per tenant)
  - private source adapters (tenant's private data feeds)
  - own desks: management (attention), research (private claims), Texture (articles)
  - consumes shared public claims graph + private claims; merges in ITS OWN og
  - distribution: public route / internal / private / (paywalled, later)

White-label mechanics
  - primary autopaper = the first user computer of this shape
  - white label = org-template export of that computer (S10: scrubbed template
    + private-source adapter config + policy), installed per tenant
  - this is ALREADY the metamission's mechanism, not new invention
```

This reframes WW from "a service beside the platform" to "**the first real
application running on the computer substrate**" — which is what the owner
means by "the purpose of all this infra."

## Integration with the metamission (dependency map)

WW deferrals and which station owns them:

| WW need | Metamission station that produces it | Status (2026-10-05) |
|---|---|---|
| A computer that runs real workloads durably | S2 layering + SO ops substrate | S2 contract slices landed, at acceptance; SO slices 1-2 landed |
| Always-on feel / wake for scheduled ingest | S3 residency tiers (hot / warm-idle / cold + trigger wake) | gated by SO |
| Capsule workload isolation + recorded egress for fetch adapters | S4 capsule open world (requires the full S1 floor) | pending |
| Live supervision of the editorial pipeline | S5 live preview + supervision (Texture transclusion) | pending |
| Desk-driven editorial pipeline | RLM desks (record-native since S0m) | partially live |
| Publication / update-on-new-info | precommitment records (`resolves`) + Texture desks | Phase 2 |
| Code/app distribution between autopapers | S8 source publication | pending |
| Forked sibling autopapers (editions, experiments) | S9 forks and fleets | pending |
| Org-template white-label | S10 org templates | pending |
| Fleet-wide fixes across divergent autopapers | S11 mainline + security push | pending |
| Tenant private data boundaries | S1 security floor (S1a landed) + S9 data classes | partial |
| **Shared public claims graph read by tenant computers** | **none — no station produces cross-computer DATA subscription** (S8 is code publication) | **gap** |

(Station numbers corrected 2026-10-05: forks are S9, org templates S10;
SO is the ops-substrate station, not S0.)

Conclusion for the director: **WW rearchitecture = the application-level
validation of the entire metamission.** Nearly every station lands a
prerequisite. The right move is likely a new station file (or a Phase-5
revision) that tracks "autopaper as first tenant application" as the consuming
conjecture, with WW decisions deferred to per-station evidence rather than
decided up front.

## Decisions explicitly deferred (NOT made here)

1. Processor/reconciler factorization — keep/replace/fold into research desk.
2. Whether the shared public claims graph stays in corpus-dolt, moves into a
   platform computer's og, or splits (shared claims in platform computer,
   per-autopaper overlays in user computers).
3. Whether existing 99 GB corpus migrates, archives, or is declared
   first-attempt residue (ties to the GC hazard in the problem doc).
4. Whether sourcecycled stays stopped pending rearchitecture, or is
   re-enabled as a stopgap to keep choir.news fed.
5. Host tooling additions (dolt CLI, jq, e2fsprogs, sysstat/iotop in
   node-b.nix) — small, orthogonal, listed for whoever owns the host module.
6. Where the editorial multisupervision / publication transaction lives
   relative to Phase-4 supervision workbench.

## Residual risks if deferred entirely

- corpus-dolt keeps burning >1 core + 10 GB RSS and growing ~100 GB-class
  store; Node B disk headroom stays pressured (S0 fire context).
- Platform event-head CAS continues to fate-share with a WW store.
- Every month of delay grows the migration/archive decision's cost.


## Director notes (2026-10-05)

- **The framing is accepted.** WW/autopaper is recorded in the metamission
  as the consuming application, not a station. Each station's finish
  should be checked against "could an autopaper computer run on this?".
- **One missing primitive: cross-computer data subscription.** The target
  sketch needs tenant autopaper computers to read a shared public claims
  graph owned by a platform computer. The metamission publishes *code*
  (S8) but has no data-publication/subscription primitive: authority,
  privacy class, revocation, and how a subscriber merges a foreign claim
  into its own og. That is the first WW-specific design to settle, and it
  is the natural successor station after S8.
- **Storage lesson to carry forward.** The 99 GB came from undeclared
  retention. Raw fetches and items should be ephemeral or bounded working
  data, with claims and provenance as the durable class. The
  rearchitecture must declare a retention class per object kind before
  ingestion restarts.
- **The factorization question stays parked.** The evidence to decide it
  (does reconcile need item access, or only claims?) can be gathered from
  the frozen corpus read-only, without restarting ingestion.
- **Containment now (in SO, per the problem doc's director review):**
  - durable sourcecycled disable;
  - the og GC wrong-store fix;
  - WW data frozen and retained;
  - one CPU observation.
