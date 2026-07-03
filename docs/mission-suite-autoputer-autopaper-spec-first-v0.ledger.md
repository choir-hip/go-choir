# Mission Suite Ledger: Autoputer / Autopaper — Spec-First Substrate Redesign

## Pass 0 — 2026-07-03 05:15 EDT

**Conjecture:** The substrate migrations (actor runtime, object graph) outran their TLA+ specifications. The next step is to center the TLA+ spec, rewrite the specs to describe the current architecture, and then drive the code to match the specs.

**Move:** construct (create spec-first suite paradoc, inventory current specs, define missions and dependencies)

**Expected ΔV:** 16 new conjectures proposed (suite variant)

**Actual ΔV:** 16/16 conjectures proposed

**Definitions recorded:**
- D1: TLA+ specs are the source of truth, not a verification afterthought.
- D2: The existing specs are either stale, ahead of code, or proving current code wrong.
- D3: Code changes must follow spec changes (spec-first workflow).
- D4: The actor runtime is the correct concurrency substrate; the old runtime must be deleted.
- D5: The autoputer rename and Nucleus capsule integration are parallel work streams.

**Conjectures proposed:**
Mission S (Spec Redesign):
- C-S1: actor_protocol_og spec holds under current object-graph semantics. (UNDECIDED)
- C-S2: wire_pipeline_og spec captures the wire pipeline flow. (UNDECIDED)
- C-S3: autoputer_lifecycle spec reproduces the current boot failure. (UNDECIDED)
- C-S4: promotion_protocol_og still flags current code violations. (UNDECIDED)

Mission A (Actor Defactoring):
- C-A1: internal/runtime package can be deleted. (UNDECIDED)
- C-A2: cmd/sandbox/main.go builds using only actor runtime + extracted helpers. (UNDECIDED)
- C-A3: Actor runtime tests pass under -race. (UNDECIDED)

Mission B (Wire Redesign):
- C-B1: Wire pipeline compiles and unit tests pass with fake providers. (UNDECIDED)
- C-B2: Wire pipeline produces a real article on staging. (UNDECIDED)
- C-B3: Every fetched item's story eventually settles. (UNDECIDED)

Mission C (Autoputer Solidification):
- C-C1: Autoputer VM image builds with Nucleus included. (UNDECIDED)
- C-C2: Autoputer VM boots and binds to port 8085 on staging. (UNDECIDED)
- C-C3: Nucleus can launch a strict-agent capsule inside the autoputer VM. (UNDECIDED)

Mission D (CI/Verification Guard):
- C-D1: CI passes after each mission commit. (TESTING)
- C-D2: TLA+ specs model-check in CI. (UNDECIDED)
- C-D3: Race detector model is correctly scoped. (SUPPORTED from predecessor)

**Evidence:**
- Existing specs: `specs/actor_protocol.tla`, `actor_protocol_xvm.tla`, `wire_pipeline.tla`, `promotion_protocol.tla`
- `specs/README.md` documents the spec layering and current violations
- `promotion_protocol.tla` already proves current code violates `NoStaleCommit` and `ApprovalGate`
- `wire_pipeline.tla` models publication trajectories that the Go code has not yet adopted
- `cmd/sandbox/main.go` still imports `internal/runtime` for non-concurrency helpers
- Only 5 production files still import `internal/runtime`

**Open decisions needing owner input:**
1. Spec naming: keep old specs as `*_v1.tla` or overwrite in place?
2. TLA+ CI tooling: is the current command correct? Add per-spec TLC configs?
3. Nucleus version: pin rev or follow main?
4. Staging persistent state: reset or migrate? (destructive)
5. Wire redesign scope: full sourcecycled → publish or processor → publish subset?
6. Autoputer rename first or after Mission A?
7. Promotion protocol: fix code to match spec or update spec?

**Next:** Commit the suite paradoc, then spawn subagents for Mission S (spec audit), Mission A (helper extraction), and Mission D (CI/TLA+ verification). Get owner decisions on open questions in parallel.
