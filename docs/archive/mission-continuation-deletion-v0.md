# Mission M4 — Continuation Deletion (cutover step 5; remove the old control surface) — v0

Source: `docs/mission-portfolio-2026-06-11.md` §M4 and the dependency graph
(`M3 lifecycle cutover ──► M4 continuation deletion ──► M5 wire on settlement`).
Program: `docs/choir-rearchitecture-durable-actors-2026-06-11.md`. Predecessors:
M1 trajectory model (settled 2026-06-12), M2 messaging cutover (settled
2026-06-13), M3 lifecycle cutover (`docs/mission-lifecycle-cutover-v0.md`,
working). Successor: M5 wire on settlement
(`docs/mission-wire-on-settlement-v0.md`, open_handoff, deferred behind this
mission). Graph node: `m4-continuation-deletion` in `docs/mission-graph.yaml`.

This paradoc is compiled ahead of execution to make the Path A route concrete.
It is **gated on M3 settlement**: do not start deletion construction until the
M3 lifecycle proof has settled or named its blocker, because M3's settlement
names which run rows, active-run queries, and legacy parent-run fields remain
audit-only — and M4 deletes the layer above them.

## Why this mission exists (the route insight)

The durable-actor cutover replaced run-tree control with trajectory/work-item
causality (M1) and one typed update/wake primitive (M2). The old
`RunContinuation` surface — record, store table, `/api/continuations/*` API,
trace projection, and `continuation-level` acceptance — is the **old half of a
dual model that is still on disk and in the wire**. AGENTS.md already marks
`continuation-level` as transitional H008/H014 residue that "M4 must delete or
explicitly shim."

Observed pressure supporting the mission conjecture: the spine has paid four
M3.x recovery gates (M3.1–M3.4) plus repeated Texture product-loop repairs,
all keeping the prompt-bar→Texture loop alive during a half-finished cutover.
M3's own learning state names the likely root cause: **"no old/new dual model
may survive settlement; stale prompt/tool surfaces are blockers, not cleanup."**
The conjecture is that deleting the continuation control surface removes the
dual-model tax that keeps re-opening the product loop, rather than being mere
hygiene after the fact.

## Parallax State

status: planned (gated on M3 settlement; compiled 2026-06-17 as the Path A
design artifact, not yet under execution)

**mission conjecture:** if the `RunContinuation` record, store table, public
`/api/continuations/*` surface, trace projection, and `continuation-level`
acceptance are deleted or reduced to an explicitly-named audit-only shim — with
every live caller re-pointed at trajectory/work-item settlement — then the
deeper rearchitecture goal advances: causality is single-model (durable
trajectories/work items only), the recurring Texture/product-loop regressions
lose their dual-model cause, and M5's wire-on-settlement gate runs against a
clean spine.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary. M4 is the deletion that
makes "durable causality replaces parent/child control" true in code, not just
in the new path.

**witness/spec (A/S):** the witness is a diff that removes the continuation
control surface and leaves a single causality model. Concretely, on current
code (2026-06-17), the surface to retire is:
- `internal/runtime/continuation.go` (~293 lines) + `continuation_test.go`;
- `internal/store/continuations.go` + `continuations_test.go`, and the
  `run_continuations` DDL/migration in `internal/store/store.go` /
  `internal/store/migration.go`;
- `types.RunContinuationRecord` in `internal/types/task.go` and the
  `continuation-level` constant in `internal/types/acceptance.go`;
- the `/api/continuations/` route, `ListRunContinuationsBySource`, and
  `runContinuationListResponse`/`Continuation*` response fields in
  `internal/runtime/api.go`;
- continuation references in `internal/runtime/api_trace.go`,
  `internal/runtime/api_compaction_eval.go`,
  `internal/runtime/tools_product_api.go`, and
  `internal/runtime/run_acceptance.go`;
- any `/api/continuations/*` reference in `frontend/`;
- the `continuation-level` run-acceptance level (re-point at trajectory/
  work-item settlement evidence, or delete with an explicit shim per AGENTS.md).
The spec is "single causality model": after the diff, no live path reads or
writes continuation records to decide work; trajectory/work-item settlement is
the only causality oracle.

**invariants / qualities / domain ramp (I/Q/D):**
- I: no new nouns; do not replace continuation with another bespoke control
  record. Re-point to existing trajectory/work-item APIs only.
- I: deletion must not strand in-flight work. Any live continuation consumer
  must have a trajectory/work-item equivalent proven before its continuation
  path is removed.
- I: an audit-only shim is acceptable only if settlement names it explicitly
  and proves it is not a control/authority oracle (mirrors M3's shim rule).
- I: `continuation-level` acceptance must not be silently weakened; either
  re-point it at trajectory settlement evidence or delete it with the explicit
  AGENTS.md shim, and do not introduce new `continuation-level` claims.
- Q: deletion is grep-clean for the target symbols in product code (the M2/M3
  "no dual model survives settlement" bar), with any residue named.
- D ramp: (1) inventory + caller map + unit/example coverage of the
  trajectory/work-item replacement for each caller; (2) delete behind green
  package suites locally; (3) staging deploy identity; (4) deployed product
  proof that the paths formerly served by continuation (trace, run-acceptance,
  product-api) still work via trajectory/work-item settlement.

**variant (ranking function) V:** count remaining continuation control reads
and surfaces that must reach zero or named-shim:
1. live (non-test) call sites of `RunContinuation*` that decide work;
2. public surface elements (`/api/continuations/*`, response fields) still
   exposed;
3. `continuation-level` acceptance still defined without a trajectory re-point
   or named shim;
4. store DDL/migration for `run_continuations` still creating/maintaining the
   table without a deprecation note;
5. driving conjecture (does deletion remove the Texture regression cause?)
   still undecided.
Initial V≈5 (to be re-measured at execution start, since M3 settlement may
already remove some readers). Move selection prefers the largest ΔV per budget;
expect to batch the mechanical deletions once the caller map is proven.

**budget:** one focused mission once M3 settles: a caller-map pass, a deletion
batch with full touched-package suites, one independent review, and a staging
deploy + deployed product proof. If the caller map reveals a live consumer
without a trajectory equivalent, that is a blocker to document first (Problem
Documentation First), not to paper over with a kept continuation path.

**authority / bounds:** mutation class `red` — touches public API surface,
trace/evidence projection, run-acceptance levels, and store schema. Before code:
name conjecture delta, protected surfaces, admissible evidence, rollback path,
and heresy delta. Repo changes on a branch or main per the landing loop;
behavior settlement requires commit→push→CI→deploy identity→deployed proof in
this document.

**mutation class / protected surfaces:** red. Protected: Trace/evidence
projection, run-acceptance synthesis, public product API, store schema/migration.

**evidence packet:** focused tests for each re-pointed caller;
`nix develop -c scripts/go-test-runtime-shards`; `nix develop -c go test
./internal/store ./internal/types`; `go vet` both tags; independent prover over
the accumulated diff; push/CI/Node B deploy with `/health` identity; deployed
product proof that trace, run-acceptance, and product-api paths work without
continuation; rollback ref = pre-deletion SHA.

**heresy delta:** discovered: H001 parent/child runtime vocabulary/control
residue is carried forward from M3.4 as a discovered heresy. M4 repairs the
continuation slice of the dual-model heresy (H008/H014). Introduced: none
accepted; replacing continuation with a new bespoke record would be an
introduced heresy.

**position / live conjectures / open edges:**
- Inventory is mapped (see witness/spec); caller-vs-test split not yet
  separated into "decides work" vs "audit-only".
- C1 (testing at execution): every live continuation reader has a
  trajectory/work-item equivalent already shipped by M1/M2/M3.
- C2 (undecided): deleting continuation measurably reduces Texture
  product-loop regressions — falsifier is a post-deletion Texture regression
  with the same dual-model signature.
- Edge/M5: `continuation-level` acceptance is also referenced by M5 wire proof
  framing; coordinate the re-point so M5's evidence gate does not depend on a
  deleted level.
- Edge/sequencing: gated on M3 settlement; starting before M3 risks deleting a
  surface M3 still names as an audit shim.

**next move:** (after M3 settles) compile the caller map: split every
`RunContinuation*` reference into "decides work" vs "audit/projection only",
and for each "decides work" site name the trajectory/work-item API that
replaces it. Record the map in the ledger, re-measure V, then batch deletions.
Do not delete any path whose replacement is unproven.

**ledger file:** `docs/mission-continuation-deletion-v0.ledger.md`.

**version / lineage:** v0 compiled 2026-06-17 as the Path A design artifact.
Predecessor M3 (`docs/mission-lifecycle-cutover-v0.md`); successor M5
(`docs/mission-wire-on-settlement-v0.md`).

**learning state:** retained here until execution; promote outward only if the
deletion changes shared assertions (e.g. confirms the dual-model-causes-
regression conjecture) or the run-acceptance level taxonomy.

**settlement:** settled when the continuation control surface is deleted or
named as an explicit audit-only shim, every live caller is proven on
trajectory/work-item settlement, target-symbol grep is clean in product code,
and deployed product proof shows trace/run-acceptance/product-api intact —
with C2 (regression-cause) accepted-and-named with a next discriminator if not
yet decided.

## Suggested Goal String

```text
Use Parallax on docs/mission-continuation-deletion-v0.md (M4, spine cutover
step 5). Source program: docs/mission-portfolio-2026-06-11.md §M4 and
docs/choir-rearchitecture-durable-actors-2026-06-11.md. GATE: do not start
deletion until M3 (docs/mission-lifecycle-cutover-v0.md) has settled or named
its blocker. Goal: delete the RunContinuation control surface (record, store
table + migration, /api/continuations/* API, trace projection,
continuation-level acceptance) or reduce it to an explicitly-named audit-only
shim, re-pointing every live caller at trajectory/work-item settlement.
Invariants: no new nouns/control records; no stranded in-flight work; do not
silently weaken continuation-level acceptance (re-point at trajectory settlement
or delete with the AGENTS.md shim); grep-clean target symbols in product code.
Variant V = live RunContinuation work-deciding call sites + exposed public
surface elements + undefined-re-point acceptance level + undeprecated store DDL
+ undecided regression-cause conjecture (initial V≈5; re-measure at start).
Budget: one focused mission — caller map, deletion batch with full touched-
package suites, independent review, staging deploy + deployed product proof.
Mutation class red (public API, trace/evidence, run-acceptance, store schema):
name conjecture delta, protected surfaces, admissible evidence, rollback path,
heresy delta before code; apply Problem Documentation First to any new blocker.
First move: compile the caller map (decides-work vs audit-only) for every
RunContinuation* reference and name the trajectory/work-item replacement per
work-deciding site; record in ledger; re-measure V; then batch. Ledger:
docs/mission-continuation-deletion-v0.ledger.md. Settlement: surface deleted or
named-shim, callers proven on settlement, grep clean, deployed proof intact.
No claim outruns its evidence class; no self-checked proofs; no fake islands.
```
