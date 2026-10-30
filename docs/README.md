# Choir Documentation

This is the map of the docs tree, not a second doctrine and not a list of every
file. It was rewritten in the 2026-10-14 prune (what was removed and why:
[`archive/PRUNED-2026-10-14.md`](archive/PRUNED-2026-10-14.md); the plan:
[`reports/docs-pruning-plan-2026-10-14.md`](archive/reports/docs-pruning-plan-2026-10-14.md)).

Choir documentation has four lanes:

1. **Semantics** — durable terms, invariants, and authority boundaries.
2. **ACTIVE** — the currently confirmed work surface (`NOW.md` is a retired stub).
3. **Records** — dated problems, evidence receipts, reports, goal files: what
   happened and why, written first and cited by the work that follows.
4. **History** — `archive/`, opt-in, never current authority.

Do not infer current authority from a document's existence, filename, or archive
location. `choir-doctrine.md` is the apex doctrine and `AGENTS.md` (also
`CLAUDE.md`) is the operating contract.

## If you are new (people)

Read in this order, about an hour in total:

1. [`../README.md`](../README.md) — what Choir is and how to run it.
2. [`reports/choir-state-and-vision-2026-10-14.md`](archive/reports/choir-state-and-vision-2026-10-14.md)
   — the long state-and-vision report (built, designed, planned, business).
   [`reports/strategy-takeaways-2026-10-14.md`](archive/reports/strategy-takeaways-2026-10-14.md)
   and [`reports/owner-notes-2026-10-14.md`](reports/owner-notes-2026-10-14.md)
   are the latest owner direction.
3. [`choir-vision.md`](choir-vision.md) — the automatic computer, in two pages.
4. [`precommitment-records.md`](precommitment-records.md) — the core mechanism
   (precommitment records and prospective self-alignment).
5. [`learn/`](learn/README.md) — the teaching series, one document per substrate.

## Default reading packet (agents)

Read these ten documents or views before broad architecture or product work.
They are the whole default packet; everything else is on demand.

1. [`README.md`](../README.md) — human and developer entry point.
2. [`AGENTS.md`](../AGENTS.md) — operating contract and mutation/evidence
   ceremony.
3. [`choir-doctrine.md`](choir-doctrine.md) — apex doctrine.
4. [`semantic-registry.md`](semantic-registry.md) — compact, non-overriding
   map of the doctrine's stable semantics.
5. [`NOW.md`](NOW.md) — retired 2026-09-08; routes to live owners.
6. [`ACTIVE.md`](ACTIVE.md) — confirmed active work and work-state caveats.
7. [`computer-ontology.md`](computer-ontology.md) — persistent computer,
   candidate, promotion, and rollback contract.
8. [`runtime-invariants.md`](runtime-invariants.md) — runtime authority and
   causality contract.
9. [`texture-agentic-invariants-2026-06-13.md`](texture-agentic-invariants-2026-06-13.md)
   — canonical artifact contract.
10. [`source-external-data-publication.md`](source-external-data-publication.md)
    — source, provenance, and publication contract.

Views point to source authority; none can override doctrine or a domain contract.
Read the relevant contract before touching its protected surface.

## Reading by task

- **Writing or running a mission (`/goal`)**: `skills/throughline/SKILL.md`
  (the only authoring discipline), [`standing-questions.md`](standing-questions.md)
  (the pre-flight set), [`memo-problem-documentation-first.md`](memo-problem-documentation-first.md),
  and a worked example under [`definitions/`](definitions/) (for instance
  `choir-pd1-persistent-research-desk-2026-10-11.md`). Lineage skills are in
  [`archive/skills/`](archive/skills/).
- **Changing agent/desk behavior**: [`agent-product-doctrine.md`](agent-product-doctrine.md),
  [`choir-prompting-invariants.md`](choir-prompting-invariants.md),
  [`desk-rlm-rectification-plan-2026-09-23.md`](archive/desk-rlm-rectification-plan-2026-09-23.md)
  (desk ontology, sections 1-10), and the settled design decisions in
  [`re-engineering-decisions-2026-10-12.md`](re-engineering-decisions-2026-10-12.md).
- **Architecture and state**: [`current-architecture.md`](current-architecture.md),
  [`platform-os-app-state.md`](platform-os-app-state.md) (a platform ledger, dated),
  [`state-homes-inventory-2026-10-08.md`](operational-invariants-register-2026-10-08.md),
  [`operational-invariants-register-2026-10-08.md`](operational-invariants-register-2026-10-08.md)
  (draft invariants awaiting owner promotion), [`restart-subsystem-design-2026-10-13.md`](restart-subsystem-design-2026-10-13.md),
  [`model-pools-design-2026-10-12.md`](model-pools-design-2026-10-12.md),
  [`texture-live-supervision-architecture.md`](texture-live-supervision-architecture.md),
  [`texture-acceptance-suite.md`](archive/texture-acceptance-suite.md).
- **Autopaper (the automatic newspaper)**: [`autopaper-sw-design-2026-10-09.md`](autopaper-sw-design-2026-10-09.md),
  [`autopaper-app-design-2026-10-11.md`](autopaper-sw-design-2026-10-09.md),
  the Store A/B split in [`runbooks/platform-dolt-authority-split-2026-09-18.md`](archive/runbooks/platform-dolt-authority-split-2026-09-18.md).
- **Security and the VM boundary**: [`vmctl-360-review-2026-10-10.md`](vmctl-360-review-2026-10-10.md),
  [`reports/computer-isolation-security-review-2026-10-14.md`](archive/reports/computer-isolation-security-review-2026-10-14.md).
- **Why and lineage**: [`signal-is-sparse-not-the-learner-2026-08-01.md`](signal-is-sparse-not-the-learner-2026-08-01.md)
  (the foundational argument), [`why-texture-2026-06-15.md`](why-texture-2026-06-15.md),
  [`memo-per-computer-frontend-2026-08-13.md`](memo-per-computer-frontend-2026-08-13.md),
  [`desk-system-harness-architecture-2026-10-01.md`](desk-system-harness-architecture-2026-10-01.md),
  [`node-a-shared-host.md`](node-a-shared-host.md),
  [`mission-residues.md`](mission-residues.md),
  [`memo-diagonal-media-strategy-2026-09-17.md`](memo-diagonal-media-strategy-2026-09-17.md)
  (business origin; superseded by the strategy takeaways), and the superseded
  roadmap [`world-wire-mission-stack-2026-09-22.md`](world-wire-mission-stack-2026-09-22.md).
- **Enforcement artifacts**: [`heresy-detectors.md`](heresy-detectors.md)
  (detector manifest read by `scripts/check-heresies.sh`),
  [`conjecture-assertion-ledger-2026-06.md`](conjecture-assertion-ledger-2026-06.md)
  (assertion register read by `cmd/doccheck`).

## Authority map

- `choir-doctrine.md` is apex doctrine; `choir-vision.md` is the product
  north star and defers to doctrine.
- `semantic-registry.md` is a compact derived map; domain contracts govern only
  their stated scope and cannot override doctrine.
- `NOW.md` is retired; it routes deployment-health questions to direct observation
  and work-state questions to `ACTIVE.md`.
- `ACTIVE.md` is the curated work view. A promoted working goal file is the
  sole current authority root; completed goals remain evidence.
- `mission-graph.yaml` is discovery metadata. Goal files own their state; the
  graph and Git history are not executable authority.
- `archive/` is searchable historical material, never current doctrine,
  executable mission authority, or part of the default packet. Verify retained
  claims against current doctrine, contracts, goal files, and observed state.
- `doc-authority-manifest.yaml` is navigation metadata, not doctrine or a
  historical catalog.

## Directory lanes

| directory | what it holds | rule of thumb |
|---|---|---|
| `definitions/` | throughline goal files (in flight and recent) | the goal file owns its state; landed ones move to `archive/definitions/` |
| `problems/` | dated problem records, written before the fix | keep while open or recent; delete old resolved ones |
| `evidence/` | acceptance receipts and probe outputs | keep only what a kept doc, script or goal file cites |
| `reports/` | design reports, station letters, owner direction | the 10-13/14 set is current; older ones are pruned when closed |
| `research/` | dated research bundles | kept while they inform an open decision |
| `learn/` | teaching series | current |
| `briefings/` | read-aloud system briefings for the owner ([how](briefings/README.md)) | keep; each settles the previous one's predicted questions |
| `runbooks/` | operational procedures | keep working procedures only |
| `deck/`, `design/`, `legal/` | pitch, showreel, policies | latest variants only |
| `archive/` | historical corpus | opt-in; see `archive/README.md` |

## Operational helper: gateway ChatGPT OAuth

The gateway's ChatGPT OAuth file is refreshed locally and copied to Node B by
[`nix/deploy-provider-creds.sh`](../nix/deploy-provider-creds.sh). The helper
does not refresh OAuth itself. Refresh/login with the local Codex/ChatGPT client
first, then run:

```sh
./nix/deploy-provider-creds.sh node-b
```

The default source is `~/.codex/auth.json`; override it with
`CODEX_AUTH_PATH`. The helper copies the JSON to
`/var/lib/go-choir/codex-auth.json`, sets `CHATGPT_AUTH_PATH`, regenerates the
gateway provider environment from the configured credential sources, and
restarts `go-choir-gateway`. It is therefore a broad credential deployment,
not a ChatGPT-only operation. Never print or commit the auth JSON.

Verify without exposing secrets:

```sh
sha256sum ~/.codex/auth.json
ssh node-b sha256sum /var/lib/go-choir/codex-auth.json
ssh node-b systemctl is-active go-choir-gateway
ssh node-b journalctl -u go-choir-gateway -n 50 --no-pager
```

When a semantic, current-state, or active-work claim changes, update its source
authority and this compact view together. Do not add another orientation page.
