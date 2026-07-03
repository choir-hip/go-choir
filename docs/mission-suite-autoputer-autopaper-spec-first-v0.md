# Mission Suite: Autoputer / Autopaper — Spec-First Substrate Redesign

**Status:** scoping / paradoc  
**Date:** 2026-07-03  
**Predecessor:** `docs/mission-autoputer-before-autopaper-v0.md`  
**Harness:** Parallax + Mission Gradient, orchestrated via subagent suite  
**Budget:** 15-25 passes across the suite, tonight  
**Mutation class:** Red/Black — touches runtime substrate, VM lifecycle, TLA+ specs, protected surfaces (Texture, promotion, wire, VM lifecycle)  
**Protected surfaces:** Texture canonical writes, corpusd sync contract, source entity graph, promotion/rollback, VM lifecycle, auth/session, gateway/provider calls

---

## Executive Thesis

The current system is a stack of migrations that each outran their specification:

- The **actor runtime** was built and partially wired, but the old `internal/runtime` package still lives as a zombie business-logic library.
- The **object graph** migration changed the data model, but the specs still speak in old vocabulary.
- The **Universal Wire** was patched on top of a borked substrate, while a formal redesign of the wire pipeline already exists in `specs/wire_pipeline.tla`.
- The **autoputer** (persistent computer) is failing to boot because the service still called `sandbox` tries to start the old runtime.

The missing step is not more code. The missing step is a **redefinition of the TLA+ specification** so that it describes the system as it is now — actor runtime + object graph + autoputer + capsules + wire — and then driving the code to match the spec.

> **TLA+ is the spec. The compiled, verified spec. We center it.**

This mission suite redesigns the specs first, then executes the code migrations that make the code conform to the new specs. The specs are not a verification afterthought; they are the source of truth for what the system does.

---

## Central Conjecture

```text
If we rewrite the TLA+ specs to model the current architecture
(actor runtime + object graph + autoputer + capsules + wire),
and then refactor the code so that the Go implementation is a
mechanical refinement of those specs, then the autoputer will boot
 cleanly, the wire pipeline will publish end-to-end, and the system
will be ready for scale-up.
```

The deeper goal (G): a self-describing, model-checked automatic computer that runs an automatic newspaper. The spec is the contract; the code is the implementation.

---

## Current Spec Inventory (and Staleness)

| Spec | File | Status | Staleness |
|------|------|--------|-----------|
| **actor_protocol.tla** | `specs/actor_protocol.tla` | Likely current | Matches `internal/actor` single-process mailbox model. Needs review for object-graph integration. |
| **actor_protocol_xvm.tla** | `specs/actor_protocol_xvm.tla` | Likely stale | Cross-VM model assumes active/candidate computer split. Needs update for autoputer + Nucleus capsule boundaries. |
| **wire_pipeline.tla** | `specs/wire_pipeline.tla` | Ahead of code | Models publication trajectories, but the Go code still uses the old in-run processor model. Needs to become the driving spec for the wire redesign. |
| **promotion_protocol.tla** | `specs/promotion_protocol.tla` | Violates code | Already proves current code violates two intended invariants. Needs to drive the promotion protocol fix or be updated if the intended protocol changed. |

The specs exist. The specs are model-checked. The specs are not aligned with the running code. The mission suite fixes that by making the specs the design document and the code the refinement.

---

## Mission Suite Architecture

The suite has one **central spec mission** and three **implementation missions**. The spec mission writes the truth; the implementation missions refine the code to match it.

### Mission S: TLA+ Spec Redesign (the center)

**Goal:** Rewrite the TLA+ specifications to describe the system as it is now and as it will be after the migration.

**Deliverables:**
- `specs/actor_protocol_og.tla` — actor runtime with object-graph state (Dolt-backed objects and edges as the durable log).
- `specs/actor_protocol_xvm_capsule.tla` — cross-VM + cross-capsule message protocol. Agents live in the autoputer; capsules are ephemeral effect chambers. Nucleus capsule boundaries are part of the model.
- `specs/wire_pipeline_og.tla` — wire pipeline on object-graph trajectories and editions. The spec drives the wire redesign; the Go code is a refinement.
- `specs/promotion_protocol_og.tla` — candidate promotion with object-graph atomic flip and rollback.
- `specs/autoputer_lifecycle.tla` — autoputer VM boot, health, recovery, and hibernation. Explains why the current VM fails to bind to 8085.
- Updated `specs/README.md` — plain-language guide to the new specs and their layering.

**Invariants:**
- Every spec must be model-checked by TLC in CI.
- Every spec change must be written before the corresponding code change.
- No spec may be weakened to make the current code pass.

**Conjectures to test:**
- C-S1: The actor_protocol_og spec can be model-checked and holds under the current object-graph semantics. (UNDECIDED)
- C-S2: The wire_pipeline_og spec captures the sourcecycled → processor → Texture → publish → edition flow. (UNDECIDED)
- C-S3: The autoputer_lifecycle spec reproduces the current boot failure as a counterexample. (UNDECIDED)
- C-S4: The promotion_protocol_og spec still flags the current code's violations. (UNDECIDED)

---

### Mission A: Actor Runtime Defactoring (refines actor_protocol_og)

**Goal:** Delete the old `internal/runtime/runtime.go` and make the actor runtime the sole business + concurrency substrate.

**Depends on:** Mission S's `actor_protocol_og.tla` to define the exact state/transition model.

**Deliverables:**
- Move `ResolvedLLMConfigFromMetadata` and `MaxInteractiveOutputTokensForSelection` from `internal/runtime` to `internal/provideriface` (completes State 2).
- Move `NewAPIHandler` and `RegisterRoutes` from `internal/runtime/api.go` to a new `internal/apihandler` package (completes State 3).
- Move stub provider and config defaults to `internal/provideriface`.
- Refactor `internal/actorruntime` so it does not embed `*runtime.Runtime` for business logic.
- Extract remaining business logic from `internal/runtime/runtime.go` into `internal/runengine` or directly into `internal/actorruntime`.
- Delete `internal/runtime/runtime.go`, `tools_coagent.go`, `channel_store.go`.
- `cmd/sandbox/main.go` imports no `internal/runtime`.

**Conjectures:**
- C-A1: `internal/runtime` package can be deleted. (UNDECIDED)
- C-A2: `cmd/sandbox/main.go` builds using only actor runtime + extracted helpers. (UNDECIDED)
- C-A3: Actor runtime tests pass under `-race`. (UNDECIDED)

---

### Mission B: Wire Pipeline Redesign (refines wire_pipeline_og)

**Goal:** Move the Universal Wire pipeline from the old in-run processor model to the object-graph trajectory model defined in the spec.

**Depends on:** Mission A (actor runtime is the sole substrate) and Mission S's `wire_pipeline_og.tla`.

**Deliverables:**
- Create `internal/wire/` package from `wire_*.go` files in `internal/runtime`.
- Decouple wire logic from old runtime types; use actor runtime `Update` messages and object-graph operations.
- Implement sourcecycled → processor → Texture → publish → edition as durable trajectory state.
- Wire the processor agent to spawn Texture agents via actor runtime, not via in-run goroutines.
- Verify with fake providers and fake clock; then verify on staging.

**Conjectures:**
- C-B1: Wire pipeline compiles and unit tests pass with fake providers. (UNDECIDED)
- C-B2: Wire pipeline produces a real article on staging. (UNDECIDED)
- C-B3: Every fetched item's story eventually settles (matches spec liveness). (UNDECIDED)

---

### Mission C: Autoputer Solidification (refines actor_protocol_xvm_capsule + autoputer_lifecycle)

**Goal:** Rename the service from `sandbox` to `autoputer`, make the VM boot cleanly, and install the Nucleus capsule foundation.

**Depends on:** Mission A (the binary no longer depends on old runtime), but rename/packaging can start in parallel.

**Deliverables:**
- Rename `cmd/sandbox` → `cmd/autoputer`, `internal/sandbox` → `internal/autoputer`, `nix/sandbox-vm.nix` → `nix/autoputer-vm.nix`.
- Update env vars and flake.nix.
- Add Nucleus flake input and include Nucleus binary in the autoputer VM image.
- Define `internal/capsule/` package with `CapsuleRunner` interface and Nucleus backend.
- Add `/internal/capsule/*` endpoints to the autoputer runtime.
- Add `bash_in_capsule` tool behind opt-in flag.
- Deploy autoputer VM to staging; verify health on port 8085.

**Conjectures:**
- C-C1: Autoputer VM image builds with Nucleus included. (UNDECIDED)
- C-C2: Autoputer VM boots and binds to port 8085 on staging. (UNDECIDED)
- C-C3: Nucleus can launch a strict-agent capsule inside the autoputer VM. (UNDECIDED)

---

### Mission D: CI / Verification Guard (runs continuously)

**Goal:** Keep CI green on every commit from missions A/B/C/S and verify the TLA+ specs remain model-checked.

**Deliverables:**
- Update CI to run TLC on all specs.
- Ensure race detector tests pass after every substrate change.
- Monitor staging deploys and health.
- Document TLA+ review findings.

**Conjectures:**
- C-D1: CI passes after each mission commit. (TESTING)
- C-D2: TLA+ specs model-check in CI. (UNDECIDED)
- C-D3: Race detector model is correctly scoped. (SUPPORTED from predecessor)

---

## Dependency Graph

```
Mission S (Spec Redesign)
    ├──► Mission A (Actor Defactoring) ──► Mission B (Wire Redesign)
    │                                          │
    ├──► Mission C (Autoputer Solidification) ◄┘
    │       (rename/packaging parallel; full verify after B)
    │
    └──► Mission D (CI/Verification Guard) — runs across all
```

Mission S is the center. It produces the specs. Missions A, B, and C refine the code to match. Mission D verifies both specs and code continuously.

---

## Spec-First Workflow

Every implementation change must follow this sequence:

1. **Spec change first** — update the TLA+ module to reflect the new behavior.
2. **Model check** — run TLC; confirm the spec still holds or deliberately accepts a new counterexample as a known limitation.
3. **Code change second** — refactor the Go code to match the spec.
4. **Build/test** — `go build ./...`, `go test -race ./...`, Nix build.
5. **Staging proof** — deploy and verify the behavior on `choir.news`.
6. **Ledger update** — record the spec change, the code change, and the evidence.

This is the spec-centered version of the Landing Loop.

---

## Invariants / Qualities / Domain Ramp

- **I:** Do not change a spec to make the current code pass. If the code violates the spec, either fix the code or explicitly weaken the spec with a documented rationale.
- **I:** Do not delete TLA+ specs without replacing them with a more precise spec.
- **I:** Do not touch Texture core (O1-O3 canonical writes), corpusd sync contract, or source entity graph structure without a spec update.
- **Q:** Every spec must be model-checked in CI.
- **Q:** Every mission commit must keep CI green.
- **D:** Spec → model check → code refactor → build/test → staging proof → scale-up ready.

---

## Variant (Conjecture Descent)

V = count of undecided conjectures across the suite.

Initial: 4 (S) + 3 (A) + 3 (B) + 3 (C) + 3 (D) = **16 conjectures**.
Target: 0.

Each pass must decide at least one conjecture or discover a new conjecture with evidence. A pass that only adds code without changing the spec or deciding a conjecture is not descent.

---

## Open Decisions (Need Owner Input)

1. **Spec naming:** Should we keep the old specs as `*_v1.tla` and write new `*_og.tla` files, or overwrite in place? Suggest: keep old specs archived as `*_v1.tla`, write new specs as `*.tla`.
2. **TLA+ tooling in CI:** Current CI runs `TLA+ Model Check (specs/)`. Is the command correct? Should we add TLC config for each new spec?
3. **Nucleus version:** Pin a specific rev of `github:sig-id/nucleus` or follow main? Suggest pin for reproducibility.
4. **Staging persistent state:** The VM may fail because Dolt state on the persistent volume is incompatible. Reset or migrate? This is destructive.
5. **Wire redesign scope:** Should Mission B include the full sourcecycled → processor → Texture → publish flow, or focus on the processor → Texture → publish part and leave sourcecycled alone for now?
6. **Autoputer rename first?** Should Mission C rename happen before Mission A, or after? The rename can be done in parallel if we keep the binary working, but final verification must wait for Mission A.
7. **Promotion protocol:** Should we fix the current promotion code to match `promotion_protocol.tla`, or update the spec if the intended protocol changed?

---

## Orchestration Plan (Tonight)

I act as orchestrator, spawning subagents for each mission track.

### Pass 1 (now)
- Spawn Mission S subagent: audit existing specs, design the new `autoputer_lifecycle.tla` and `actor_protocol_og.tla` modules.
- Spawn Mission A subagent: extract the two model-policy helper functions (completes State 2).
- Spawn Mission D subagent: verify current CI command and TLA+ CI setup.

### Pass 2
- Merge Mission A helper extraction into main.
- Spawn Mission A subagent: extract API handlers (State 3 completion).
- Spawn Mission S subagent: draft `actor_protocol_og.tla`.
- Spawn Mission C subagent: begin rename scaffolding (flake.nix package names, env vars).

### Pass 3+
- Continue extraction/deletion in Mission A.
- Model-check new specs in Mission S.
- Begin wire pipeline extraction in Mission B once Mission A is far enough.
- Keep Mission D running as guard.

Every subagent reports conjecture status, evidence, and proposed next move. I coordinate handoffs and verify builds.

---

## Ledger

Ledger file: `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md`

## Version / Lineage

- Predecessor: `docs/mission-autoputer-before-autopaper-v0.md`
- Sibling: `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md`
- Sibling: `docs/naming-rectification-2026-06-27.md`
- Sibling: `docs/archive/handoff-hybrid-computer-capsule-architecture-2026-06-10.md`
- Sibling: `specs/README.md` (to be updated by Mission S)
- Successor: `docs/mission-universal-wire-stabilization-v2.md` (autopaper publishing)
