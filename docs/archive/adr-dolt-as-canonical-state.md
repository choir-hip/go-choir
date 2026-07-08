# ADR: Dolt As Canonical Product State

**Status:** accepted direction
**Date:** 2026-05-14

## Context

The old root `TODOS.md` asked whether Choir should evaluate a SQLite -> Dolt
hard cutover after stabilization. The current architecture direction is now
clearer:

- `vtext` versions, appagent state, evidence, run memory, traces, prompts,
  publication staging, artifact metadata, and promotion records need versioned
  semantic history.
- Users should have persistent computers whose state can diverge from the
  platform baseline.
- Platform/public state needs provenance, publication, citation, route,
  capacity, and compute-accounting records.
- Hot runtime facts, auth/session compatibility, and caches may still need
  narrower stores.

The problem is not "replace every SQLite table immediately." The problem is
choosing the canonical product-state ledger so future work does not deepen the
wrong boundary.

## Decision

Dolt is the default canonical store for durable product state.

Choir has two Dolt layers:

1. Per-user embedded Dolt owns private computer/appagent state.
2. Platform Dolt owns platform-visible state.

SQLite may remain for narrow hot runtime, auth/session, cache, local
compatibility, or transitional implementation roles when explicitly justified.
New durable product truth should not be added to SQLite by default.

## Implementation Note: Runtime Cutover

As of 2026-05-15, the sandbox runtime/control product tables for runs, events,
Trace, run memory, continuations, app change/adoption state, run acceptances,
browser sessions, researcher findings, worker updates, and desktop state are
opened in the same per-user embedded Dolt workspace that already owns VText
state.

The current filesystem path convention still uses the old runtime path as a
marker and legacy-import source, but the canonical writer is the embedded Dolt
workspace derived from that path. Existing non-empty legacy SQLite runtime files
are imported once when the Dolt runtime tables are empty and are left in place as
rollback inputs during cutover.

Host auth/session and vmctl stores remain SQLite until the platform Dolt layer
and routing/promotion records are designed separately.

## Per-User Embedded Dolt Owns

- `vtext` documents and versions;
- appagent state;
- prompts and policies;
- theme records and user preferences when they belong to the user's computer;
- local trajectories and run memory;
- Trace/provenance summaries where private to the user;
- researcher findings and evidence metadata;
- file metadata for user files whose bytes live in blob/content storage;
- personal promotion records.

## Platform Dolt Owns

- account/user/tenant metadata where platform-visible;
- VM/computer lifecycle, capacity, and routing records where durable;
- platform VM pool records;
- publication records;
- public artifact metadata;
- citation graph state;
- public verifier and promotion records;
- compute/accounting records;
- later CHIPS-compatible state.

Platform Dolt is a ledger, not the network. Cross-VM and cross-computer work
should use direct transport or relays for live delivery, then write compact
durable facts for recovery, audit, provenance, publication, citation, routing,
and compute accounting.

## Filesystem And Blob Boundary

The filesystem is not one thing:

- source files under repos are source/build state;
- uploaded files and generated media are blob/content state;
- runtime caches and temp files are machine state;
- Dolt-backed documents may have filesystem aliases, but canonical semantic
  state belongs in Dolt.

Large files should live in content-addressed blob storage with Dolt/artifact
metadata rather than inside ad hoc SQLite rows or opaque VM snapshots.

## Consequences

This ADR implies:

- Prefer Dolt schemas or typed artifact records for new durable product facts.
- Use migrations and compatibility layers only when needed; avoid speculative
  all-at-once rewrites.
- Keep SQLite tables when they are clearly hot runtime, cache, auth/session, or
  transitional implementation detail.
- Make promotion operate over typed ledgers: Dolt commits/branches,
  source/build deltas, blob hashes, artifact graph records, verifier results,
  app packages, agent packages, and route-switch certificates.
- Do not treat opaque VM state as a semantic merge artifact.

## Migration Policy

Move state when the receiving Dolt model is ready and the product path benefits:

1. Stabilize behavior first.
2. Name the state owner.
3. Define the Dolt schema or typed artifact shape.
4. Write a verifier or invariant check for the migration boundary.
5. Cut over the product path.
6. Delete the stale source of truth.

The repo is still early enough that hard cutovers can be acceptable for narrow
areas, but they should be scoped and verified.

## Related Docs

- [runtime-invariants.md](../runtime-invariants.md)
- [current-architecture.md](../current-architecture.md)
- [computer-ontology.md](../computer-ontology.md)
- [old-docs-review-2026-06-06.md](old-docs-review-2026-06-06.md)
