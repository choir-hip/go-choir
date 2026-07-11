# Choir Autoputer: External-CLI Operability Before Choir-in-Choir

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-autoputer-cli-operability-2026-07-11.md
```

Read this document as executable semantic authority for making the autoputer
(the persistent Choir computer) fully operable and self-developable by an
**external** agent through the choir CLI, before any internal agent
(co-super) is given the same controls. It restores the deleted
autoputer-before-autopaper doctrine as a standing dictum and sequences the
work the 2026-07-11 post-mortem showed is prerequisite to every product
mission.

## Standing Dictum (owner, restated 2026-07-11)

**Autoputer before autopaper.** The automatic computer, with working
self-development, precedes automatic publication. The canonical sequence
(owner, reversed/confirmed 2026-07-11) is:

1. **Audited computer works** — computers constructed as functions of state
   (PC-5 kernel + acceptance matrix, capsule/candidate materialization, real
   promotion per PC-4). The wire-store conformance mission is the first
   slice: deleting boot-time migration and evicting wire state is what makes
   "computer = f(code, state)" well-defined at all. Candidate computers are
   capsules over substrate-independent audited computers, not VMs (owner,
   2026-07-08).
2. **Choir-CLI autoputer** — an external agent (e.g. Claude in a harness)
   operates and self-develops the computer through the choir CLI alone.
3. **Choir-in-choir autoputer** — co-supers using the choir CLI under
   contained keys.
4. Only then: **autopaper** editorial ambitions on top.

A thin slice of CLI observability (computer status/generation/receipts) is
pulled into step 1 so the audited-computer staging proofs are CLI-visible
rather than another SSH forensics campaign — dogfooding the exact surface
step 2 requires.

Twelve autopaper attempts failed by shortcutting this sequence
(`docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`).
`specs/autoputer_lifecycle.tla` still cites the deleted
`docs/mission-autoputer-before-autopaper-v0.md`; this Definition is that
doctrine's successor.

## Source Authority Order

1. This Definition.
2. `docs/definitions/choir-wire-store-conformance-2026-07-11.md` (in-flight
   prerequisite: two-store taxonomy, D-WIRE conformance, legacy-migration
   deletion, route-ledger demotion to a table).
3. `AGENTS.md`, `docs/choir-doctrine.md`, `docs/computer-ontology.md`,
   `docs/agent-product-doctrine.md`.
4. `docs/definitions/choir-product-completion-2026-07-10.md` (PC-3 key
   coherence, PC-4 promotion truth gate).
5. Observed source: `cmd/choir` (current verbs: wire, trajectories,
   trajectory, texture, search, run, api-key, version), `internal/vmctl`,
   `internal/vmmanager`, `internal/runtime`, deploy workflow scripts.
6. `docs/NOW.md` implementation-status ceilings.

## The Test That Defines "Working Autoputer"

An external agent holding only a scoped API key and the choir CLI — no SSH,
no journalctl, no GitHub Actions access — can:

1. Ask whether its computer is ready, healthy, and which
   code/artifact generation is serving, and get the truth.
2. Start a run, poll one truthful status, and on completion fetch the run's
   required artifact (not a narration of it).
3. Propose a code/config change as a package, materialize a candidate
   computer, run verification, promote it (durable route flip with receipt),
   observe the new generation serving, and roll it back — entirely through
   CLI verbs.
4. Diagnose a failed boot, run, or promotion from CLI-visible evidence
   (receipts, redacted lifecycle events, bounded logs), without host shell
   access.

When Claude can do all four against staging, the autoputer works. When a
co-super can do all four under a key that cannot escalate, choir-in-choir
is open.

## Gap Inventory (grounded 2026-07-11)

- **G1. No computer lifecycle surface in the CLI.** No verbs or product API
  for computer status/health/generation/restart-history. All diagnosis to
  date has required SSH on Node B — a surface neither external agents nor
  co-supers will have. (Enables test items 1 and 4.)
- **G2. Boot/readiness not bounded.** Covered largely by the wire-store
  mission (migration deletion); remaining: vmctl must distinguish slow from
  dead (probe budgets from measured startup), and recovery must be
  generation-guarded (a stale ensure killed a healthy newer generation on
  2026-07-10).
- **G3. Run status lies.** Five disagreeing projections (run state,
  trajectory, processor-resolution, sourcecycled ledger, admission counter);
  blocked/passivated/completed-with-live-trajectory each froze the 12-hour
  run. One capacity/completion authority, surfaced by `choir run status`.
  (Post-mortem cornerstone C4.)
- **G4. Completion is narrative.** Runs report success without their
  required artifact existing (reconciler evidence, 2026-07-11). Completion
  must be artifact-verified and the artifact CLI-fetchable. (Cornerstone C5.)
- **G5. Promotion is not real activation.** Adoption/lineage records exist,
  but promotion can report success without served-route mutation (PC-4;
  NOW.md "not real served-code activation"). Self-development requires the
  real thing: package → candidate → evidence → route-slot CAS + immutable
  receipt → rollback. The route-slot record is a **table on the corpusd
  sql-server with vmctl as sole writer** (per the route-ledger demotion in
  the wire-store mission), not a third store. CLI verbs: package, candidate,
  verify, promote, rollback, receipts.
- **G6. Deploy/verify receipts are untrustworthy.** The verifier failed at
  least six correct deploys on 2026-07-10/11 (2s probe budget, 60s window
  vs. real startup; workflow-SHA vs artifact-SHA conflation partially
  repaired). An operating agent must trust a receipt without re-deriving it
  from journals.
- **G7. Key scoping is not choir-in-choir safe.** Reachable API-key scope
  escalation (2026-07-10 audit) and PC-3 open. When co-supers hold CLI keys,
  key scope is the inter-agent security boundary. Also: CLI hard-coded 30s
  client timeout vs proxy's 60s bound yields false failures for agent
  operators — timeout coherence is part of the operator contract.

## Execution Order

1. **Phase 0 — audited computer groundwork:** wire-store conformance mission
   completes (bounded boot, two-store taxonomy, migration deleted — the
   computer's state function becomes well-defined), plus the thin observe
   slice of G1 (computer status/generation/receipts in the CLI).
2. **Phase 1 — audited computer proven:** the PC-5 kernel transaction is
   built and its acceptance matrix passes; candidate materialization and
   equivalence evidence (`internal/computerversion` + its observation CLIs)
   are exercised against staging computers, observed through the Phase 0 CLI
   slice. Exit: a computer is constructed as a function of state and its
   audit evidence is CLI-visible.
3. **Phase 2 — observe + receipts:** remainder of G1 + G6. External Claude
   can watch a computer live and trust deploy receipts.
4. **Phase 3 — run truth:** G3 + G4. External Claude can run work and trust
   the answer.
5. **Phase 4 — self-development:** G5 (+ residual G2), consuming Phase 1:
   promotion flips routes between audited ComputerVersions with receipts and
   rollback, via CLI only.
6. **Phase 5 — containment:** G7. Keys that cannot escalate; then
   choir-in-choir opens (co-supers get scoped keys and the same four-item
   test).
7. **After all of it:** autopaper editorial (reconciler), per the dictum.

Each phase's acceptance is the corresponding item of the four-item external
operator test, executed by an external agent on staging and recorded with
dated evidence. A phase proven only by unit tests or by SSH observation is
not complete.

## Non-Purpose

- No autopaper/editorial work rides along.
- No new services; no capsule wiring; no rename ceremonies.
- No internal-agent (co-super) key issuance before Phase D.
- The route-slot table is built in Phase C for promotion; it must not grow
  into a general control store.

## Supersession Record

- Restores: the deleted autoputer-before-autopaper doctrine (cited by
  `specs/autoputer_lifecycle.tla`).
- Depends on: `choir-wire-store-conformance-2026-07-11.md`.
- Consumes: post-mortem cornerstones C2–C5 from
  `choir-autopaper-activation-attempt-report-2026-07-11.md` and the
  follow-on missions named there (this Definition is their execution order).
