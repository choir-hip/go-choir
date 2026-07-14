# Choir Autoputer: External-CLI Operability Before Choir-in-Choir

## Subordinate Invocation Semantics

This document supplies the external-operator, self-development, and containment
contracts for:

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-14.md
```

Do not invoke it as a separate spine. The active mission sequences runtime
dissolution, audited-computer proof, observation, run truth, self-development,
and contained Choir-in-Choir authority under one orchestrator and one canonical
state capsule.

## Standing Dictum (owner, restated 2026-07-11)

**Autoputer before autopaper.** The automatic computer, with working
self-development, precedes automatic publication. The canonical sequence
(owner, reversed/confirmed 2026-07-11) is:

1. **Audited computer works** — mission R2 explicitly unpauses PC-5 pre-wiring
   rows 572–581 plus the computer-ontology Candidate Contract for
   `ComputerVersion(CodeRef, ArtifactProgramRef)` materialization. PC-5
   post-gate service ownership remains paused; R5 alone proves real promotion.
   Wire conformance first removes VM-local Wire state so
   `computer = f(code, state)` is well-defined. Candidate computers are
   capsules over substrate-independent audited computers, not VMs.
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
The deleted `docs/mission-autoputer-before-autopaper-v0.md` is historical
evidence. The active mission is now the executable successor to that doctrine;
this document remains its operator-contract specification.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-2026-07-14.md`.
2. This subordinate operator-contract Definition within R2/R3/R5/R6 scope.
3. `docs/definitions/choir-wire-store-conformance-2026-07-11.md`.
4. `AGENTS.md`, `docs/choir-doctrine.md`, `docs/computer-ontology.md`,
   `docs/agent-product-doctrine.md`.
5. `docs/definitions/choir-product-completion-2026-07-10.md`.
6. Observed source: `cmd/choir`, `internal/vmctl`, `internal/vmmanager`,
   extracted runtime/app boundaries, and deploy workflow scripts.
7. `docs/NOW.md` implementation-status ceilings.

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

## Sequence Context And Owned Phase Mapping

The active mission owns execution order. Settled Deploy and Wire receipts plus
R1 runtime extinction are prerequisite context, not phases owned by this
subordinate contract:

1. **Settled receipts:** Deploy restoration and Wire-store conformance.
2. **R1:** runtime dissolution to directory extinction; this prerequisite
   prevents audited-computer/operator work from extending the god package or
   creating compatibility facades.
3. **R2 / former Phase 1:** audited computer proven.
4. **R3 / former Phase 2:** observation and trustworthy receipts.
5. **R4 / former Phase 3:** run truth, consuming this document's operator test.
6. **R5 / former Phase 4:** self-development.
7. **R6 / former Phase 5:** contained credentials and Choir-in-Choir.

Autopaper editorial remains a successor mission after R7. Acceptance is the
corresponding external/co-super operator test on staging, referenced from the
active mission capsule. Unit tests or SSH observation alone cannot complete a
phase.

## Introspection Contract (safe limit for no-SSH debugging)

The observable set is derived empirically, then bounded by authority scope:

- **Demand side:** every SSH observation the 2026-07-10/11 post-mortem run
  actually needed is a candidate CLI observable — restart counts, lifecycle
  transitions with reasons, guest boot phase markers, health latency, run/
  trajectory truth, queue depth and capacity, migration progress, deploy/
  activation receipts, edition state. The attempt report's evidence ledger is
  the requirements document.
- **The safe limit is authority-scoped, not detail-scoped.** Within its own
  computer, a key may see high-fidelity facts. It may never see: host
  journald/systemd or shell, raw SQL, provider secrets, host network/tap
  topology, the existence or state of other computers or owners, or
  unredacted proxy/platform internals.
- **Receipts and states, not shells.** All introspection is read-only, typed,
  owner-scoped projection with bounded cardinality (pagination, time
  windows). No query language, no eval, no tail -f on host logs.
- **Introspection must not consume the substrate it observes.** Diagnostics
  read from projected/receipt state (control tables on the corpusd server,
  emitted lifecycle events), never live-query a computer's contended store —
  the 2026-07-11 evidence shows a diagnostic that takes a turn on the store
  becomes the outage it is measuring.
- **Escalation ladder instead of shell:** anything receipts cannot answer is
  served by a `diagnose` verb that has the computer produce a redacted
  capture bundle as a fetchable artifact. Host SSH remains platform-operator
  break-glass only and is never part of the product or agent contract.

## Non-Purpose

- No autopaper/editorial work rides along.
- No new services; no capsule wiring; no rename ceremonies.
- No co-super key issuance before mission R6.
- The route-slot table established for R5 promotion must not grow into a
  general control store.

## Supersession Record

- Preserves the external-operator and containment contracts from the restored
  autoputer-before-autopaper doctrine.
- Is subordinate to
  `docs/definitions/choir-autoputer-completion-2026-07-14.md`.
- Consumes post-mortem cornerstones C2–C5 from
  `choir-autopaper-activation-attempt-report-2026-07-11.md`; the active mission,
  not this document, owns execution order and resumption.
